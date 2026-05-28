// Command kycd starts the GOLD KYC Service.
//
// Responsibilities:
//  1. POST /kyc/submit          — authenticated user submits ID document + personal info
//  2. GET  /kyc/status          — authenticated user checks their KYC status
//  3. PATCH /kyc/:id/review     — admin approves or rejects a KYC application
//  4. GET  /health              — liveness probe
//  5. Publish NATS events: kyc.submitted, kyc.approved, kyc.rejected
//
// Status flow: pending → under_review → approved | rejected
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/pkg/obs"
	"github.com/ismetaba/gold-token/backend/pkg/server"
	"github.com/ismetaba/gold-token/backend/services/kyc/internal/config"
	kychttp "github.com/ismetaba/gold-token/backend/services/kyc/internal/http"
	"github.com/ismetaba/gold-token/backend/services/kyc/internal/jwtverify"
	"github.com/ismetaba/gold-token/backend/services/kyc/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/kyc/internal/storage"
)

func main() {
	log := obs.NewLogger("kycd")
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

	// 3. JWT verifier (public-key only; no signing needed)
	verifier, err := jwtverify.New(cfg.JWTPublicKeyFile, cfg.Env)
	if err != nil {
		return err
	}
	if cfg.JWTPublicKeyFile == "" {
		log.Warn("jwt verifier: running in permissive local mode — signatures are NOT verified")
	}

	// 4. Document store
	store, err := storage.NewLocalStore(cfg.StorageDir)
	if err != nil {
		return err
	}
	log.Info("document storage ready", zap.String("dir", cfg.StorageDir))

	// 5. Repos
	var appRepo repo.ApplicationRepo
	if pool != nil {
		appRepo = repo.NewPGRepo(pool)
	}

	// 6. HTTP server
	handlers := kychttp.NewHandlers(appRepo, store, verifier, bus, cfg.AdminSecret, log)
	srv := server.NewHTTPServer(cfg.HTTPAddr, handlers.Routes(cfg.Env), server.Timeouts{ReadHeader: 5 * time.Second, Read: 30 * time.Second, Write: 15 * time.Second})
	return server.Serve(ctx, srv, log, 10*time.Second)
}
