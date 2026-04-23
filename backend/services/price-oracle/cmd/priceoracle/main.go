// Command priceoracle starts the GOLD Price Oracle Service.
//
// Responsibilities:
//  1. GET /price/current  — latest XAU/USD price in USD per gram
//  2. GET /price/history  — historical prices over a configurable window
//  3. GET /health         — liveness probe
//  4. Fetches XAU/USD from goldapi.io (stub in local mode)
//  5. Caches latest price in-memory; persists history to PostgreSQL
//  6. Publishes price.updated on NATS JetStream on each successful fetch
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
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/config"
	oraclehttp "github.com/ismetaba/gold-token/backend/services/price-oracle/internal/http"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/oracle"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/provider"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/repo"
)

func main() {
	log := obs.NewLogger("priceoracle")
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
	// 1. DB (optional in local mode)
	var pool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		var err error
		pool, err = pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
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
	}

	// 3. Price provider
	var prov provider.Provider
	if cfg.PriceAPIKey != "" {
		prov = provider.NewGoldAPI(cfg.PriceAPIKey)
		log.Info("using goldapi.io price provider")
	} else {
		prov = provider.NewStub()
		log.Warn("GOLD_PRICE_API_KEY not set — using stub price provider (local mode)")
	}

	// 4. Repo (optional in local mode)
	var priceRepo repo.PriceRepo
	if pool != nil {
		priceRepo = repo.NewPGPriceRepo(pool)
	}

	// 5. Oracle
	orc := oracle.New(prov, priceRepo, bus, cfg.RefreshInterval, log)

	// Start the fetch loop in background.
	go orc.Run(ctx)

	// 6. HTTP server
	handlers := oraclehttp.NewHandlers(orc, priceRepo, log)
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
