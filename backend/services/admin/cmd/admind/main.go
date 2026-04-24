// Command admind starts the GOLD Admin API Gateway.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/obs"
	"github.com/ismetaba/gold-token/backend/services/admin/internal/config"
	adminhttp "github.com/ismetaba/gold-token/backend/services/admin/internal/http"
	"github.com/ismetaba/gold-token/backend/services/admin/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/admin/internal/tokens"
)

func main() {
	log := obs.NewLogger("admind")
	defer func() { _ = log.Sync() }()

	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal("config load failed", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log, cfg); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal("service exited with error", zap.Error(err))
	}
}

func run(ctx context.Context, log *zap.Logger, cfg *config.Config) error {
	// Database.
	var pool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		var err error
		pool, err = pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		log.Info("database connected")
	}

	var adminUserRepo repo.AdminUserRepo
	var apiKeyRepo repo.APIKeyRepo
	if pool != nil {
		adminUserRepo = repo.NewPGAdminUserRepo(pool)
		apiKeyRepo = repo.NewPGAPIKeyRepo(pool)
	}

	// Admin JWT manager.
	tm, err := tokens.NewManager(cfg.JWTPrivateKeyFile, cfg.JWTPublicKeyFile)
	if err != nil {
		return err
	}

	// Build upstream proxies for each routed service.
	type proxyDef struct {
		prefix      string
		serviceURL  string
		adminSecret string
	}
	proxyDefs := []proxyDef{
		{"/admin/kyc", cfg.KYCServiceURL, cfg.KYCAdminSecret},
		{"/admin/users", cfg.AuthServiceURL, cfg.AuthAdminSecret},
		{"/admin/orders", cfg.OrderServiceURL, cfg.OrderAdminSecret},
		{"/admin/treasury", cfg.TreasuryServiceURL, cfg.TreasuryAdminSecret},
		{"/admin/vault", cfg.VaultServiceURL, cfg.VaultAdminSecret},
		{"/admin/fees", cfg.FeeServiceURL, cfg.FeeAdminSecret},
		{"/admin/audit", cfg.AuditServiceURL, cfg.AuditAdminSecret},
		{"/admin/compliance", cfg.ComplianceServiceURL, cfg.ComplianceAdminSecret},
	}

	proxies := make(map[string]http.Handler, len(proxyDefs))
	for _, pd := range proxyDefs {
		p, err := adminhttp.NewServiceProxy(pd.serviceURL, pd.prefix, pd.adminSecret, log)
		if err != nil {
			return err
		}
		proxies[pd.prefix] = p
		log.Info("proxy registered", zap.String("prefix", pd.prefix), zap.String("target", pd.serviceURL))
	}

	handlers := adminhttp.NewHandlers(adminUserRepo, apiKeyRepo, tm, cfg.MasterSecret, proxies, log)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handlers.Routes(cfg.Env),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http listen", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	}
}
