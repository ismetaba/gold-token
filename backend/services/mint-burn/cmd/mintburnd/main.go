// Command mintburnd starts the GOLD Mint/Burn Service.
//
// Sorumluluklar:
//   1. NATS'dan `gold.order.ready_to_mint` event'lerini dinle → saga oluştur
//   2. PostgreSQL'deki saga_instances'ı her `StepPollInterval`'de bir ilerlet
//   3. Admin HTTP API sun (/health, /admin/sagas/{id})
package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	pkgchain "github.com/ismetaba/gold-token/backend/pkg/chain"
	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/pkg/obs"
	pkgsigner "github.com/ismetaba/gold-token/backend/pkg/signer"

	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/chain"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/config"
	mbevents "github.com/ismetaba/gold-token/backend/services/mint-burn/internal/events"
	mbhttp "github.com/ismetaba/gold-token/backend/services/mint-burn/internal/http"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/saga"
)

func main() {
	log := obs.NewLogger("mintburnd")
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

	// 3. Chain client — real go-ethereum client when CHAIN_RPC_URL is set;
	//    falls back to StubClient for local/CI runs without a chain node.
	mc, err := buildMintControllerClient(ctx, cfg, log)
	if err != nil {
		return err
	}

	// 4. Repos
	var sagaRepo repo.SagaRepo
	var barRepo repo.BarRepo
	if pool != nil {
		sagaRepo = repo.NewPGSagaRepo(pool)
		barRepo = repo.NewPGBarRepo(pool)
	}

	// 5. Orchestrator
	orch := saga.NewOrchestrator(sagaRepo, barRepo, mc, bus, log, saga.Config{
		ApprovalTimeout:  cfg.ApprovalTimeout,
		StepPollInterval: cfg.StepPollInterval,
		MaxAttempts:      cfg.MaxAttempts,
	})

	// 6. Event consumer
	if bus != nil {
		cons := mbevents.NewConsumer(bus, orch, log, cfg.NATSStream)
		if err := cons.Start(ctx); err != nil {
			return err
		}
	}

	// 7. HTTP server
	handlers := mbhttp.NewHandlers(sagaRepo, log)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handlers.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	// 8. Start saga worker + HTTP server in parallel
	errCh := make(chan error, 2)
	go func() {
		log.Info("http listen", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		log.Info("saga worker started",
			zap.Duration("poll_interval", cfg.StepPollInterval),
			zap.Duration("approval_timeout", cfg.ApprovalTimeout),
		)
		if err := orch.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()

	// 9. Graceful shutdown
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

// buildMintControllerClient returns a real EthMintControllerClient when
// CHAIN_RPC_URL + MINT_CONTROLLER_ADDR are set, otherwise falls back to the
// in-memory StubClient (local / CI without a chain node).
//
// Signer selection is controlled by SIGNER_TYPE (stub|softhsm).
func buildMintControllerClient(ctx context.Context, cfg *config.Config, log *zap.Logger) (chain.MintControllerClient, error) {
	if cfg.ChainRPCURL == "" || cfg.MintCtrlAddr == "" {
		log.Warn("chain client: stub mode — set CHAIN_RPC_URL + MINT_CONTROLLER_ADDR for production")
		return chain.NewStubClient(), nil
	}

	ethClient, err := pkgchain.NewEthClient(cfg.ChainRPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial chain RPC: %w", err)
	}

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch chain ID: %w", err)
	}
	if cfg.ChainID != 0 && chainID.Int64() != cfg.ChainID {
		return nil, fmt.Errorf("chain ID mismatch: expected %d, got %s", cfg.ChainID, chainID)
	}

	// Build the digest-level signer (StubSigner or SoftHSMSigner).
	digSigner, err := pkgsigner.New(pkgsigner.Config{
		Type:              pkgsigner.Type(cfg.SignerType),
		PrivateKeyHex:     cfg.SignerPrivateKey,
		LibPath:           cfg.SoftHSMLib,
		TokenLabel:        cfg.SoftHSMTokenLabel,
		PIN:               cfg.SoftHSMPin,
		KeyLabel:          cfg.SoftHSMKeyLabel,
	})
	if err != nil {
		return nil, fmt.Errorf("init signer (%s): %w", cfg.SignerType, err)
	}

	// Wrap the digest signer into a transaction signer.
	txSigner := pkgchain.NewTxSigner(digSigner, chainID, ethClient.Inner())

	signerAddr := txSigner.Address()
	log.Info("chain signer ready",
		zap.String("type", cfg.SignerType),
		zap.String("address", fmt.Sprintf("0x%x", signerAddr[:])),
	)

	mc, err := chain.NewEthMintControllerClient(cfg.MintCtrlAddr, txSigner, ethClient)
	if err != nil {
		return nil, fmt.Errorf("init mint controller client: %w", err)
	}

	log.Info("chain client: go-ethereum",
		zap.String("rpc", cfg.ChainRPCURL),
		zap.String("chain_id", chainID.String()),
		zap.String("mint_ctrl", cfg.MintCtrlAddr),
	)

	_ = big.NewInt(0) // ensure math/big is used
	return mc, nil
}
