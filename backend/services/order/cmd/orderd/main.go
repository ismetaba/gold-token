// Command orderd starts the GOLD Order Service.
//
// Responsibilities:
//  1. POST /orders        — create buy or sell order (idempotent via X-Idempotency-Key)
//  2. GET  /orders        — list user orders, paginated
//  3. GET  /orders/:id    — order detail + status
//  4. GET  /health        — liveness probe
//
// State machine: created → confirmed → processing → completed | failed
//
// Buy orders:  auto-confirm → publish gold.order.ready_to_mint.v1 → mint saga
// Sell orders: auto-confirm → publish gold.burn.requested.v1      → burn saga
//
// Listens on NATS:
//   - gold.mint.executed.v1  → mark order completed
//   - gold.mint.failed.v1    → mark order failed
//   - gold.burn.executed.v1  → mark order completed
//   - gold.burn.failed.v1    → mark order failed
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
	"github.com/ismetaba/gold-token/backend/services/order/internal/config"
	orderevents "github.com/ismetaba/gold-token/backend/services/order/internal/events"
	orderhttp "github.com/ismetaba/gold-token/backend/services/order/internal/http"
	"github.com/ismetaba/gold-token/backend/services/order/internal/repo"
)

func main() {
	log := obs.NewLogger("orderd")
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

	// 3. Repo
	var orderRepo repo.OrderRepo
	if pool != nil {
		orderRepo = repo.NewPGOrderRepo(pool)
	}

	// 4. NATS consumer — update order status on saga outcomes
	if bus != nil && orderRepo != nil {
		cons := orderevents.NewConsumer(bus, orderRepo, log, cfg.NATSStream)
		if err := cons.Start(ctx); err != nil {
			return err
		}
	}

	// 5. HTTP server
	handlers, err := orderhttp.NewHandlers(orderRepo, bus, cfg.NATSStream, cfg.JWTPublicKeyFile, log)
	if err != nil {
		return err
	}
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
