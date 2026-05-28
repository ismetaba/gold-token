// Command feed starts the GOLD Fee Management Service.
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

	"github.com/ismetaba/gold-token/backend/services/fee/internal/config"
	feeevents "github.com/ismetaba/gold-token/backend/services/fee/internal/events"
	feehttp "github.com/ismetaba/gold-token/backend/services/fee/internal/http"
	"github.com/ismetaba/gold-token/backend/services/fee/internal/repo"
)

func main() {
	log := obs.NewLogger("feed")
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

	var bus *pkgevents.Bus
	if cfg.NATSURL != "" {
		var err error
		bus, err = pkgevents.NewBus(cfg.NATSURL, log)
		if err != nil {
			return err
		}
		defer bus.Close()
		log.Info("NATS connected")
	}

	var (
		scheduleRepo repo.ScheduleRepo
		ledgerRepo   repo.LedgerRepo
	)
	if pool != nil {
		scheduleRepo = repo.NewPGScheduleRepo(pool)
		ledgerRepo = repo.NewPGLedgerRepo(pool)
	}

	if bus != nil && scheduleRepo != nil {
		cons := feeevents.NewConsumer(bus, scheduleRepo, ledgerRepo, log, cfg.NATSStream)
		if err := cons.Start(ctx); err != nil {
			return err
		}
		log.Info("fee event consumer started")
	}

	handlers := feehttp.NewHandlers(scheduleRepo, ledgerRepo, cfg.AdminSecret, log)
	srv := server.NewHTTPServer(cfg.HTTPAddr, handlers.Routes(cfg.Env), server.DefaultTimeouts())
	return server.Serve(ctx, srv, log, 10*time.Second)
}
