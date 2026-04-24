// Command auditd starts the GOLD Audit Log Service.
//
// Responsibilities:
//  1. Consume ALL domain events and persist as immutable audit trail
//  2. GET /audit/logs  — paginated, filterable audit log query
//  3. GET /audit/logs/{id} — single entry lookup
//  4. GET /health — liveness probe
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

	"github.com/ismetaba/gold-token/backend/services/audit/internal/config"
	audevents "github.com/ismetaba/gold-token/backend/services/audit/internal/events"
	audhttp "github.com/ismetaba/gold-token/backend/services/audit/internal/http"
	"github.com/ismetaba/gold-token/backend/services/audit/internal/repo"
)

func main() {
	log := obs.NewLogger("auditd")
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
	var entryRepo repo.EntryRepo
	if pool != nil {
		entryRepo = repo.NewPGEntryRepo(pool)
	}

	// 4. Event consumer — wildcard listener
	if bus != nil && entryRepo != nil {
		cons := audevents.NewConsumer(bus, entryRepo, log, cfg.NATSStream)
		if err := cons.Start(ctx); err != nil {
			return err
		}
		log.Info("audit event consumer started")
	}

	// 5. HTTP server
	handlers := audhttp.NewHandlers(entryRepo, cfg.AdminSecret, log)
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

	// 6. Graceful shutdown
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
