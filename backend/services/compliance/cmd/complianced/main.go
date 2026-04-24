// Command complianced starts the GOLD Compliance Service.
//
// Responsibilities:
//  1. POST /compliance/screen    — manual sanctions screening for a user
//  2. GET  /compliance/status/:userId — current compliance status
//  3. GET  /health               — liveness probe
//
// Admin routes:
//  4. GET  /compliance/monitoring     — list users due for re-screening
//  5. POST /compliance/monitoring/run — trigger monitoring run
//  6. GET  /compliance/rules          — list jurisdiction rules
//  7. PATCH /compliance/rules/{id}    — update rule
//
// NATS subscriptions:
//   - gold.order.created.v1  → auto-screen user; publish compliance.approved/rejected
//
// NATS publications:
//   - gold.compliance.approved.v1
//   - gold.compliance.rejected.v1
//   - gold.compliance.alert.v1
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
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/jurisdiction"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/monitor"
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

	// 3. Sanctions screener — custom file or embedded default
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

	// 4. PEP screener — custom file or embedded default
	var pepSc screener.PEPScreener
	if cfg.PEPListFile != "" {
		data, err := os.ReadFile(cfg.PEPListFile)
		if err != nil {
			return err
		}
		pepSc, err = screener.NewLocalPEPScreenerFromJSON(data)
		if err != nil {
			return err
		}
		log.Info("PEP list loaded from file", zap.String("path", cfg.PEPListFile))
	} else {
		var err error
		pepSc, err = screener.NewLocalPEPScreener()
		if err != nil {
			return err
		}
		log.Info("using embedded PEP list")
	}

	// 5. Adverse media screener (stub — logs skip notices)
	adverseMedia := screener.NewAdverseMediaStub(log)
	_ = adverseMedia // available for future wiring into runScreen

	// 6. On-chain registry (stub in local/POC mode)
	var registry chain.ComplianceRegistryClient = chain.NewStubRegistryClient()
	_ = registry

	// 7. Repos
	var compRepo repo.ComplianceRepo
	var monRepo repo.MonitoringRepo
	var pepRepo repo.PEPRepo
	var ruleRepo jurisdiction.RuleRepo

	if pool != nil {
		compRepo = repo.NewPGRepo(pool)
		monRepo = repo.NewPGMonitoringRepo(pool)
		pepRepo = repo.NewPGPEPRepo(pool)
		ruleRepo = repo.NewPGJurisdictionRepo(pool)
	}

	// 8. Jurisdiction engine
	var jEngine *jurisdiction.Engine
	if ruleRepo != nil {
		jEngine = jurisdiction.NewEngine(ruleRepo)
	}
	_ = jEngine // wired into event consumer in future iteration

	// 9. Monitoring worker
	var monWorker *monitor.Worker
	if compRepo != nil && monRepo != nil {
		monWorker = monitor.NewWorker(
			compRepo, monRepo, pepRepo,
			sc, pepSc,
			bus,
			monitor.Config{
				Interval:  time.Duration(cfg.MonitoringIntervalSeconds) * time.Second,
				BatchSize: cfg.MonitoringBatchSize,
			},
			log,
		)
		monWorker.Start(ctx)
		log.Info("monitoring worker started",
			zap.Duration("interval", time.Duration(cfg.MonitoringIntervalSeconds)*time.Second),
			zap.Int("batch_size", cfg.MonitoringBatchSize),
		)
	}

	// 10. HTTP handlers
	handlers := comphttp.NewHandlers(compRepo, sc, log)

	var adminHandlers *comphttp.AdminHandlers
	if monRepo != nil || ruleRepo != nil {
		adminHandlers = comphttp.NewAdminHandlers(monRepo, ruleRepo, monWorker, log)
	}

	// 11. NATS consumer — auto-screen on order.created
	if bus != nil {
		cons := compevents.NewConsumer(bus, handlers, log, cfg.NATSStream)
		if err := cons.Start(ctx); err != nil {
			return err
		}
	}

	// 12. HTTP server
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handlers.Routes(cfg.Env, adminHandlers),
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
