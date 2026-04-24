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
	if pool != nil {
		adminUserRepo = repo.NewPGAdminUserRepo(pool)
	}

	handlers := adminhttp.NewHandlers(adminUserRepo, cfg.KYCAdminSecret, log)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handlers.Routes(cfg.Env),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
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
