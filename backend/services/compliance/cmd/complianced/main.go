// Command complianced starts the GOLD Compliance Service.
//
// Responsibilities:
//  1. POST /compliance/screen    — manual sanctions screening for a user
//  2. GET  /compliance/status/:userId — current compliance status
//  3. GET  /health               — liveness probe
//
// NATS subscriptions:
//   - gold.order.created.v1  → auto-screen user; publish compliance.approved/rejected
//
// NATS publications:
//   - gold.compliance.approved.v1
//   - gold.compliance.rejected.v1
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
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/chain"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/config"
	compevents "github.com/ismetaba/gold-token/backend/services/compliance/internal/events"
	comphttp "github.com/ismetaba/gold-token/backend/services/compliance/internal/http"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/screener"
)

func main() {
	log := obs.NewLogger("complianced")
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

	// 3. Screener — custom file or embedded default
	var sc screener.Screener
	if cfg.SanctionsListFile != "" {
		data, err := os.ReadFile(cfg.SanctionsListFile)
		if err != nil {
			return err
		}
		sc, err = screener.NewLocalScreenerFromJSON(data)
		if err != nil {
			return err
		}
		log.Info("sanctions list loaded from file", zap.String("path", cfg.SanctionsListFile))
	} else {
		var err error
		sc, err = screener.NewLocalScreener()
		if err != nil {
			return err
		}
		log.Info("using embedded sanctions list")
	}

	// 4. On-chain registry (stub in local/POC mode)
	var registry chain.ComplianceRegistryClient = chain.NewStubRegistryClient()
	_ = registry // wired into future on-chain whitelisting calls

	// 5. Repo
	var compRepo repo.ComplianceRepo
	if pool != nil {
		compRepo = repo.NewPGRepo(pool)
	}

	// 6. HTTP handlers (also used by the NATS consumer)
	handlers := comphttp.NewHandlers(compRepo, sc, log)

	// 7. NATS consumer — auto-screen on order.created
	if bus != nil {
		cons := compevents.NewConsumer(bus, handlers, log, cfg.NATSStream)
		if err := cons.Start(ctx); err != nil {
			return err
		}
	}

	// 8. HTTP server
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
