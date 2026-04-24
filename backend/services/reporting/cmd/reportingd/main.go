// Command reportingd starts the GOLD Reporting Service.
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

	"github.com/ismetaba/gold-token/backend/services/reporting/internal/config"
	repevents "github.com/ismetaba/gold-token/backend/services/reporting/internal/events"
	reportinghttp "github.com/ismetaba/gold-token/backend/services/reporting/internal/http"
	"github.com/ismetaba/gold-token/backend/services/reporting/internal/repo"
)

func main() {
	log := obs.NewLogger("reportingd")
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
	// 1. DB pool (optional in local mode)
	var pool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		var err error
		pool, err = pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		log.Info("database connected")
	} else {
		log.Warn("DATABASE_URL not set — running without persistence (local stub mode)")
	}

	// 2. Event bus (optional in local mode)
	var bus *pkgevents.Bus
	if cfg.NATSURL != "" {
		var err error
		bus, err = pkgevents.NewBus(cfg.NATSURL, log)
		if err != nil {
			return err
		}
		defer bus.Close()
		log.Info("NATS connected", zap.String("url", cfg.NATSURL))
	} else {
		log.Warn("NATS_URL not set — running without event bus (local stub mode)")
	}

	// 3. Repos
	var (
		jobs    repo.ReportJobRepo
		queries repo.QueryRepo
		mat     repo.MaterializedRepo
	)
	if pool != nil {
		jobs = repo.NewPGReportJobRepo(pool)
		queries = repo.NewPGQueryRepo(pool)
		mat = repo.NewPGMaterializedRepo(pool)
	}

	// 4. Event consumer
	if bus != nil && mat != nil {
		cons := repevents.NewConsumer(bus, mat, log, cfg.NATSStream)
		if err := cons.Start(ctx); err != nil {
			return err
		}
		log.Info("event consumer started")
	}

	// 5. HTTP server
	handlers := reportinghttp.NewHandlers(jobs, queries, cfg.AdminSecret, log)
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
