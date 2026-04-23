// Command walletd starts the GOLD Wallet Service.
//
// Responsibilities:
//  1. POST /wallet/address — generate or return user's Ethereum address
//  2. GET  /wallet/address — get user's Ethereum address
//  3. GET  /wallet/balance — on-chain ERC-20 GOLD balance via eth_call
//  4. GET  /wallet/transactions — paginated tx history (event-sourced from NATS)
//  5. GET  /health — liveness probe
//  6. Listens on NATS: mint.executed.v1, burn.executed.v1 → populates transaction_log
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
	"github.com/ismetaba/gold-token/backend/services/wallet/internal/chain"
	"github.com/ismetaba/gold-token/backend/services/wallet/internal/config"
	walletevents "github.com/ismetaba/gold-token/backend/services/wallet/internal/events"
	wallethttp "github.com/ismetaba/gold-token/backend/services/wallet/internal/http"
	"github.com/ismetaba/gold-token/backend/services/wallet/internal/repo"
)

func main() {
	log := obs.NewLogger("walletd")
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

	// 3. On-chain balance reader (optional — skipped in local/no-chain mode)
	var balReader *chain.BalanceReader
	if cfg.ChainRPCURL != "" {
		var err error
		balReader, err = chain.NewBalanceReader(cfg.ChainRPCURL, cfg.GoldTokenAddr)
		if err != nil {
			return err
		}
		defer balReader.Close()
		log.Info("chain reader connected",
			zap.String("rpc", cfg.ChainRPCURL),
			zap.String("token", cfg.GoldTokenAddr),
		)
	} else {
		log.Warn("chain reader: no CHAIN_RPC_URL — balance calls return 0")
	}

	// 4. Repos
	var walletRepo repo.WalletRepo
	var txRepo repo.TxRepo
	if pool != nil {
		walletRepo = repo.NewPGWalletRepo(pool)
		txRepo = repo.NewPGTxRepo(pool)
	}

	// 5. NATS consumer (event-sourced tx log)
	if bus != nil && walletRepo != nil && txRepo != nil {
		cons := walletevents.NewConsumer(bus, walletRepo, txRepo, log, cfg.NATSStream)
		if err := cons.Start(ctx); err != nil {
			return err
		}
	}

	// 6. HTTP server
	var chainReader wallethttp.BalanceReader
	if balReader != nil {
		chainReader = balReader
	}
	handlers, err := wallethttp.NewHandlers(walletRepo, txRepo, chainReader, cfg.JWTPublicKeyFile, log)
	if err != nil {
		return err
	}
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
