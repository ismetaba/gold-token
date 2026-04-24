// Command priceoracle starts the GOLD Market Data Service.
//
// Responsibilities:
//  1. GET /price/current?pair=XAU/USD  — latest price for any supported pair
//  2. GET /price/history?pair=XAU/USD&window=24h  — historical prices
//  3. GET /price/candles?pair=XAU/USD&interval=1h&from=...&to=...  — OHLCV candles
//  4. GET /price/ws  — WebSocket endpoint for real-time price updates
//  5. GET /health   — liveness probe
//
// Data sources (active in non-local mode):
//   - goldapi.io   (GOLD_PRICE_API_KEY)
//   - metals-api.com (GOLD_METALS_API_KEY, optional)
//   - stub (local mode / fallback)
//
// Aggregation: median of all successful provider responses per pair.
// Currencies: XAU/USD, XAU/TRY, XAU/EUR, XAU/CHF
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
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/domain"
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

	// 3. Price providers — build the multi-source list.
	var providers []provider.Provider

	if cfg.GoldAPIKey != "" {
		providers = append(providers, provider.NewGoldAPI(cfg.GoldAPIKey))
		log.Info("enabled provider: goldapi.io")
	}
	if cfg.MetalsAPIKey != "" {
		providers = append(providers, provider.NewMetalsAPI(cfg.MetalsAPIKey))
		log.Info("enabled provider: metals-api.com")
	}

	if len(providers) == 0 {
		// Local / test mode: use a single stub that covers all pairs.
		providers = append(providers, provider.NewStub())
		log.Warn("no real price API keys set — using stub provider (local mode)")
	}

	// 4. Repo (optional in local mode)
	var priceRepo repo.PriceRepo
	if pool != nil {
		priceRepo = repo.NewPGPriceRepo(pool)
	}

	// 5. Oracle — drives multi-pair fetch, aggregation, candle building, WebSocket hub.
	orc := oracle.New(
		providers,
		domain.SupportedPairs,
		priceRepo,
		bus,
		cfg.RefreshInterval,
		log,
	)
	go orc.Run(ctx)

	// 6. HTTP server
	handlers := oraclehttp.NewHandlers(orc, priceRepo, log)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handlers.Routes(cfg.Env),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0, // Disable for WebSocket connections (long-lived).
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
