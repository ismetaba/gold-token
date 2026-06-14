// Command vaultd starts the GOLD Vault Integration Service.
//
// Responsibilities:
//  1. POST /vault/bars/ingest        — register new gold bars
//  2. GET  /vault/bars               — query bar inventory
//  3. POST /vault/bars/{serial}/transfer — inter-vault bar transfer
//  4. GET  /vault/audits             — vault audit history
//  5. POST /vault/audits             — record vault audit
//  6. GET  /health                   — liveness probe
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

	"github.com/ismetaba/gold-token/backend/services/vault/internal/config"
	vaultevents "github.com/ismetaba/gold-token/backend/services/vault/internal/events"
	vaulthttp "github.com/ismetaba/gold-token/backend/services/vault/internal/http"
	"github.com/ismetaba/gold-token/backend/services/vault/internal/repo"
)

func main() {
	log := obs.NewLogger("vaultd")
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
	// 1. DB pool
	var pool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		var err error
		pool, err = pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		if err := pool.Ping(ctx); err != nil {
			return err
		}
		log.Info("database connected")
	} else {
		log.Warn("DATABASE_URL not set — running without persistence")
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
		log.Info("NATS connected")
	} else {
		log.Warn("NATS_URL not set — running without event bus")
	}

	// 3. Repos
	var (
		barRepo      repo.BarRepo
		movementRepo repo.MovementRepo
		auditRepo    repo.AuditRepo
		vaultRepo    repo.VaultRepo
	)
	if pool != nil {
		barRepo = repo.NewPGBarRepo(pool)
		movementRepo = repo.NewPGMovementRepo(pool)
		auditRepo = repo.NewPGAuditRepo(pool)
		vaultRepo = repo.NewPGVaultRepo(pool)
	}

	// 4. Event publisher
	pub := vaultevents.NewPublisher(bus, log)

	// 5. HTTP server
	handlers := vaulthttp.NewHandlers(barRepo, movementRepo, auditRepo, vaultRepo, pub, cfg.AdminSecret, log)
	srv := server.NewHTTPServer(cfg.HTTPAddr, handlers.Routes(cfg.Env), server.DefaultTimeouts())
	return server.Serve(ctx, srv, log, 10*time.Second)
}
