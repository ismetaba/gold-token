// Command pord starts the GOLD Proof-of-Reserve Service.
//
// Responsibilities:
//  1. GET  /por/current       — latest reserve attestation (on-chain + DB)
//  2. GET  /por/history       — paginated attestation history from DB
//  3. GET  /por/ratio         — current reserve ratio
//  4. GET  /por/transparency  — public transparency data
//  5. POST /por/attest        — admin: publish new attestation to ReserveOracle
//  6. POST /por/auditor-verify— auditor: submit verification record
//  7. GET  /health            — liveness probe
//  8. Background sync: polls on-chain attestation count and back-fills DB log
//  9. Auto-attestation worker: scheduled chain sync on configurable cron
// 10. NATS pub: por.attestation.updated on each new publish
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

	porchain "github.com/ismetaba/gold-token/backend/services/por/internal/chain"
	"github.com/ismetaba/gold-token/backend/services/por/internal/config"
	"github.com/ismetaba/gold-token/backend/services/por/internal/domain"
	porhttp "github.com/ismetaba/gold-token/backend/services/por/internal/http"
	"github.com/ismetaba/gold-token/backend/services/por/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/por/internal/worker"
)

func main() {
	log := obs.NewLogger("pord")
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
			return fmt.Errorf("connect db: %w", err)
		}
		defer pool.Close()
	}

	// 2. Event bus
	var bus *pkgevents.Bus
	if cfg.NATSURL != "" {
		var err error
		bus, err = pkgevents.NewBus(cfg.NATSURL, log)
		if err != nil {
			return fmt.Errorf("connect nats: %w", err)
		}
		defer bus.Close()
	}

	// 3. On-chain reader (view-only, no signing)
	var oracleReader *porchain.OracleReader
	if cfg.ChainRPCURL != "" && cfg.ReserveOracleAddr != "" {
		var err error
		oracleReader, err = porchain.NewOracleReader(cfg.ChainRPCURL, cfg.ReserveOracleAddr)
		if err != nil {
			return fmt.Errorf("init oracle reader: %w", err)
		}
		defer oracleReader.Close()
		log.Info("oracle reader connected",
			zap.String("rpc", cfg.ChainRPCURL),
			zap.String("oracle", cfg.ReserveOracleAddr),
		)
	} else {
		log.Warn("oracle reader: no CHAIN_RPC_URL or RESERVE_ORACLE_ADDR — on-chain reads disabled")
	}

	// 4. Token supply reader (for reserve ratio)
	var supplyReader *porchain.TokenSupplyReader
	if cfg.ChainRPCURL != "" && cfg.GoldTokenAddr != "" {
		var err error
		supplyReader, err = porchain.NewTokenSupplyReader(cfg.ChainRPCURL, cfg.GoldTokenAddr)
		if err != nil {
			return fmt.Errorf("init token supply reader: %w", err)
		}
		defer supplyReader.Close()
	}

	// 5. On-chain writer (for admin attest endpoint)
	var oracleWriter *porchain.OracleWriter
	if cfg.ChainRPCURL != "" && cfg.ReserveOracleAddr != "" {
		w, err := buildOracleWriter(ctx, cfg, log)
		if err != nil {
			// Non-fatal in local mode — admin write will return 503.
			log.Warn("oracle writer init failed — admin write path disabled", zap.Error(err))
		} else {
			oracleWriter = w
		}
	}

	// 6. Repos
	var attestRepo repo.AttestationRepo
	var verificationRepo repo.AuditorVerificationRepo
	var autoAttestCfgRepo repo.AutoAttestationConfigRepo
	if pool != nil {
		attestRepo = repo.NewPGAttestationRepo(pool)
		verificationRepo = repo.NewPGAuditorVerificationRepo(pool)
		autoAttestCfgRepo = repo.NewPGAutoAttestationConfigRepo(pool)
	}

	// 7. Background chain sync (backfills DB log from on-chain history)
	syncOnceFunc := func(ctx context.Context) error {
		if oracleReader == nil || attestRepo == nil {
			return nil
		}
		return syncOnce(ctx, oracleReader, attestRepo, bus, log)
	}
	if oracleReader != nil && attestRepo != nil {
		go runChainSync(ctx, oracleReader, attestRepo, bus, cfg.SyncInterval, log)
	}

	// 7b. Auto-attestation worker (scheduled cron-based sync)
	if autoAttestCfgRepo != nil {
		var workerReader worker.OracleReader
		if oracleReader != nil {
			workerReader = oracleReader
		}
		aaWorker := worker.NewAutoAttestationWorker(
			autoAttestCfgRepo,
			workerReader,
			attestRepo,
			syncOnceFunc,
			log,
		)
		go aaWorker.Run(ctx)
	}

	// 8. HTTP server
	var httpReader porhttp.OracleReader
	if oracleReader != nil {
		httpReader = oracleReader
	}
	var httpWriter porhttp.OracleWriter
	if oracleWriter != nil {
		httpWriter = oracleWriter
	}
	var httpSupply porhttp.TokenSupplyReader
	if supplyReader != nil {
		httpSupply = supplyReader
	}

	handlers := porhttp.NewHandlers(
		attestRepo,
		verificationRepo,
		httpReader,
		httpWriter,
		httpSupply,
		bus,
		cfg.AdminToken,
		cfg.AuditorAPIKey,
		log,
	)

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

// buildOracleWriter constructs an OracleWriter when the chain is configured.
func buildOracleWriter(ctx context.Context, cfg *config.Config, log *zap.Logger) (*porchain.OracleWriter, error) {
	ethClient, err := pkgchain.NewEthClient(cfg.ChainRPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial chain RPC: %w", err)
	}

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch chain ID: %w", err)
	}
	if cfg.ChainID != 0 && chainID.Int64() != cfg.ChainID {
		return nil, fmt.Errorf("chain ID mismatch: expected %d got %s", cfg.ChainID, chainID)
	}

	digSigner, err := pkgsigner.New(pkgsigner.Config{
		Type:          pkgsigner.Type(cfg.SignerType),
		PrivateKeyHex: cfg.SignerPrivateKey,
		LibPath:       cfg.SoftHSMLib,
		TokenLabel:    cfg.SoftHSMTokenLabel,
		PIN:           cfg.SoftHSMPin,
		KeyLabel:      cfg.SoftHSMKeyLabel,
	})
	if err != nil {
		return nil, fmt.Errorf("init signer (%s): %w", cfg.SignerType, err)
	}

	txSigner := pkgchain.NewTxSigner(digSigner, chainID, ethClient.Inner())
	signerAddr := txSigner.Address()
	log.Info("oracle writer signer ready",
		zap.String("type", cfg.SignerType),
		zap.String("address", fmt.Sprintf("0x%x", signerAddr[:])),
	)

	return porchain.NewOracleWriter(cfg.ReserveOracleAddr, txSigner, ethClient)
}

// runChainSync polls the on-chain attestation count and back-fills the DB log
// for any attestations not yet recorded locally.
func runChainSync(
	ctx context.Context,
	reader *porchain.OracleReader,
	attestRepo repo.AttestationRepo,
	bus *pkgevents.Bus,
	interval time.Duration,
	log *zap.Logger,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := syncOnce(ctx, reader, attestRepo, bus, log); err != nil {
				log.Warn("chain sync tick failed", zap.Error(err))
			}
		}
	}
}

// syncOnce fetches the on-chain attestation count and persists any new entries.
func syncOnce(
	ctx context.Context,
	reader *porchain.OracleReader,
	attestRepo repo.AttestationRepo,
	bus *pkgevents.Bus,
	log *zap.Logger,
) error {
	count, err := reader.AttestationCount(ctx)
	if err != nil {
		return fmt.Errorf("attestation count: %w", err)
	}
	if count == 0 {
		return nil
	}

	for i := uint64(0); i < count; i++ {
		a, err := reader.AttestationAt(ctx, i)
		if err != nil {
			log.Warn("fetch attestation from chain", zap.Uint64("index", i), zap.Error(err))
			continue
		}
		if a.Timestamp == 0 {
			continue
		}

		idx := int64(i)
		record := domain.Attestation{
			ID:            repo.NewID(),
			OnChainIdx:    &idx,
			TimestampSec:  int64(a.Timestamp),
			AsOfSec:       int64(a.AsOf),
			TotalGramsWei: gramsString(a.TotalGrams),
			MerkleRoot:    porchain.Bytes32ToHex(a.MerkleRoot),
			IPFSCid:       porchain.Bytes32ToHex(a.IPFSCid),
			Auditor:       a.Auditor.Hex(),
			RecordedAt:    time.Now().UTC(),
		}

		if err := attestRepo.Create(ctx, record); err != nil {
			log.Warn("persist synced attestation", zap.Int64("idx", idx), zap.Error(err))
			continue
		}

		// Publish NATS update for each newly synced attestation.
		if bus != nil {
			env := pkgevents.Envelope[map[string]interface{}]{
				EventType:   pkgevents.SubjReserveAttestation,
				AggregateID: fmt.Sprintf("chain:%d", i),
				Version:     1,
				Data: map[string]interface{}{
					"on_chain_idx":    idx,
					"timestamp":       a.Timestamp,
					"as_of":           a.AsOf,
					"total_grams_wei": record.TotalGramsWei,
					"merkle_root":     record.MerkleRoot,
					"ipfs_cid":        record.IPFSCid,
					"auditor":         record.Auditor,
				},
			}
			if err := pkgevents.Publish(ctx, bus, env); err != nil {
				log.Warn("publish sync event", zap.Int64("idx", idx), zap.Error(err))
			}
		}

		log.Info("synced attestation from chain",
			zap.Int64("on_chain_idx", idx),
			zap.String("auditor", record.Auditor),
		)
	}
	return nil
}

func gramsString(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}
