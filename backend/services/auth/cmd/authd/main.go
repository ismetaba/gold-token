// Command authd starts the GOLD Auth Service.
//
// Responsibilities:
//  1. POST /auth/register  — email+password → JWT pair
//  2. POST /auth/login     — email+password → JWT pair
//  3. POST /auth/refresh   — refresh token rotation → new JWT pair
//  4. GET  /auth/me        — current user from access token
//  5. GET  /health         — liveness probe
//  6. Publish gold.user.registered.v1 on NATS on successful registration
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

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/pkg/obs"
	"github.com/ismetaba/gold-token/backend/services/auth/internal/config"
	authhttp "github.com/ismetaba/gold-token/backend/services/auth/internal/http"
	"github.com/ismetaba/gold-token/backend/services/auth/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/auth/internal/tokens"
)

func main() {
	log := obs.NewLogger("authd")
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
	// 1. DB
	var pool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		var err error
		pool, err = pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
	}

	// 2. Event bus
	var bus *pkgevents.Bus
	if cfg.NATSURL != "" {
		var err error
		bus, err = pkgevents.NewBus(cfg.NATSURL, log)
		if err != nil {
			return err
		}
		defer bus.Close()
	}

	// 3. Token manager (RS256; generates ephemeral key in local mode)
	accessTTL := time.Duration(cfg.AccessTokenTTL) * time.Second
	refreshTTL := time.Duration(cfg.RefreshTokenTTL) * time.Second
	tm, err := tokens.NewManager(cfg.JWTPrivateKeyFile, cfg.JWTPublicKeyFile, accessTTL, refreshTTL)
	if err != nil {
		return err
	}
	if cfg.JWTPrivateKeyFile == "" {
		log.Warn("token manager: using ephemeral RSA key — not suitable for production")
	}

	// 4. Repos
	var userRepo repo.UserRepo
	if pool != nil {
		userRepo = repo.NewPGUserRepo(pool)
	}

	// 5. HTTP server
	handlers := authhttp.NewHandlers(userRepo, tm, bus, log)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handlers.Routes(),
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
