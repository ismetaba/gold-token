// Package http provides the PoR service HTTP API.
package http

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	porchain "github.com/ismetaba/gold-token/backend/services/por/internal/chain"
	"github.com/ismetaba/gold-token/backend/services/por/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/por/internal/repo"
)

// OracleReader is the on-chain read interface (subset used by handlers).
type OracleReader interface {
	Latest(ctx context.Context) (porchain.OnChainAttestation, error)
	AttestationCount(ctx context.Context) (uint64, error)
}

// OracleWriter is the on-chain write interface.
type OracleWriter interface {
	Publish(ctx context.Context, req porchain.PublishRequest) (string, error)
}

// TokenSupplyReader reads the total supply of the GOLD ERC-20 token.
type TokenSupplyReader interface {
	TotalSupply(ctx context.Context) (*big.Int, error)
}

// Handlers wires together repos, on-chain reader/writer, and event bus.
type Handlers struct {
	attestations repo.AttestationRepo
	reader       OracleReader        // nil in no-chain mode
	writer       OracleWriter        // nil when admin write disabled
	supply       TokenSupplyReader   // nil in no-chain mode
	bus          *pkgevents.Bus      // nil when NATS disabled
	adminToken   string
	log          *zap.Logger
}

// NewHandlers constructs the handler set.
func NewHandlers(
	attestations repo.AttestationRepo,
	reader OracleReader,
	writer OracleWriter,
	supply TokenSupplyReader,
	bus *pkgevents.Bus,
	adminToken string,
	log *zap.Logger,
) *Handlers {
	return &Handlers{
		attestations: attestations,
		reader:       reader,
		writer:       writer,
		supply:       supply,
		bus:          bus,
		adminToken:   adminToken,
		log:          log,
	}
}

// Routes registers all HTTP routes.
func (h *Handlers) Routes(env string) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	if env == "local" {
		r.Use(httputil.CORSMiddleware(httputil.LocalCORSConfig()))
	} else {
		r.Use(httputil.CORSMiddleware(httputil.DefaultCORSConfig()))
	}

	rl := httputil.NewRateLimiter(60, time.Minute)
	r.Use(rl.Middleware)

	r.Get("/health", h.health)

	r.Route("/por", func(r chi.Router) {
		r.Get("/current", h.current)
		r.Get("/history", h.history)
		r.With(h.requireAdmin).Post("/attest", h.attest)
	})

	return r
}

// ─────────────────────────────────────────────────────────────────────────────
// Middleware
// ─────────────────────────────────────────────────────────────────────────────

func (h *Handlers) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.adminToken == "" {
			writeErr(w, http.StatusServiceUnavailable, "admin_disabled", "admin endpoints are disabled")
			return
		}
		hdr := r.Header.Get("Authorization")
		token := strings.TrimPrefix(hdr, "Bearer ")
		if token == "" {
			token = r.Header.Get("X-Admin-Token")
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(h.adminToken)) != 1 {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "valid admin token required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /por/current — latest reserve attestation
func (h *Handlers) current(w http.ResponseWriter, r *http.Request) {
	// Try on-chain first, fall back to DB.
	if h.reader != nil {
		a, err := h.reader.Latest(r.Context())
		if err != nil {
			h.log.Error("fetch on-chain latest attestation", zap.Error(err))
			writeErr(w, http.StatusInternalServerError, "chain_error", "could not fetch on-chain attestation")
			return
		}
		if a.Timestamp == 0 {
			// No on-chain attestations yet.
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"attestation": nil,
				"message":     "no attestations published yet",
			})
			return
		}

		totalTokensWei, ratio := h.computeRatio(r.Context(), a.TotalGrams)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"attestation": attestationResponse(a, totalTokensWei, ratio),
		})
		return
	}

	// No chain — serve from DB.
	if h.attestations == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"attestation": nil,
			"message":     "service running without chain or database",
		})
		return
	}

	a, err := h.attestations.Latest(r.Context())
	if errors.Is(err, repo.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"attestation": nil,
			"message":     "no attestations recorded yet",
		})
		return
	}
	if err != nil {
		h.log.Error("fetch latest attestation from db", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not fetch attestation")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"attestation": dbAttestationResponse(a),
	})
}

// GET /por/history?page=1&limit=20 — historical attestation list
func (h *Handlers) history(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	if h.attestations == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"page": page, "limit": limit, "attestations": []interface{}{},
		})
		return
	}

	records, err := h.attestations.List(r.Context(), limit, offset)
	if err != nil {
		h.log.Error("list attestations", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not fetch history")
		return
	}

	out := make([]interface{}, 0, len(records))
	for _, a := range records {
		out = append(out, dbAttestationResponse(a))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"page": page, "limit": limit, "attestations": out,
	})
}

// POST /por/attest — admin: submit a new attestation to the ReserveOracle contract
//
// Request body:
//
//	{
//	  "timestamp":    1234567890,       // unix seconds; must be <= now
//	  "as_of":        1234567890,       // audit reference date unix seconds
//	  "total_grams_wei": "12000000000000000000000",
//	  "merkle_root":  "0xabc...def",    // bytes32 hex
//	  "ipfs_cid":     "0xabc...def",    // bytes32 hex (IPFS CID encoded)
//	  "auditor":      "0xabc...def",    // auditor Ethereum address
//	  "signature":    "0xabc...def"     // EIP-712 signature by auditor key
//	}
func (h *Handlers) attest(w http.ResponseWriter, r *http.Request) {
	if h.writer == nil {
		writeErr(w, http.StatusServiceUnavailable, "write_disabled",
			"on-chain write path not configured; set CHAIN_RPC_URL and RESERVE_ORACLE_ADDR")
		return
	}

	var req struct {
		Timestamp     int64  `json:"timestamp"`
		AsOf          int64  `json:"as_of"`
		TotalGramsWei string `json:"total_grams_wei"`
		MerkleRoot    string `json:"merkle_root"`
		IPFSCid       string `json:"ipfs_cid"`
		Auditor       string `json:"auditor"`
		Signature     string `json:"signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	if req.Timestamp <= 0 || req.AsOf <= 0 || req.TotalGramsWei == "" ||
		req.MerkleRoot == "" || req.IPFSCid == "" || req.Auditor == "" || req.Signature == "" {
		writeErr(w, http.StatusBadRequest, "missing_fields",
			"timestamp, as_of, total_grams_wei, merkle_root, ipfs_cid, auditor, signature are required")
		return
	}

	// Validate timestamps are within reasonable bounds.
	now := time.Now().Unix()
	if req.Timestamp > now+3600 {
		writeErr(w, http.StatusBadRequest, "invalid_timestamp", "timestamp cannot be more than 1 hour in the future")
		return
	}
	if req.AsOf > now {
		writeErr(w, http.StatusBadRequest, "invalid_as_of", "as_of cannot be in the future")
		return
	}
	const ninetyDays = 90 * 24 * 3600
	if req.AsOf < now-ninetyDays {
		writeErr(w, http.StatusBadRequest, "invalid_as_of", "as_of cannot be more than 90 days in the past")
		return
	}

	totalGrams, ok := new(big.Int).SetString(req.TotalGramsWei, 10)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid_total_grams_wei", "total_grams_wei must be a decimal integer string")
		return
	}

	merkleRoot, err := porchain.HexToBytes32(req.MerkleRoot)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_merkle_root", fmt.Sprintf("merkle_root: %v", err))
		return
	}

	ipfsCid, err := porchain.HexToBytes32(req.IPFSCid)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_ipfs_cid", fmt.Sprintf("ipfs_cid: %v", err))
		return
	}

	if !gethcommon.IsHexAddress(req.Auditor) {
		writeErr(w, http.StatusBadRequest, "invalid_auditor", "auditor must be a 0x-prefixed Ethereum address")
		return
	}

	sigHex := strings.TrimPrefix(req.Signature, "0x")
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_signature", "signature must be hex-encoded bytes")
		return
	}

	publishReq := porchain.PublishRequest{
		Timestamp:  uint64(req.Timestamp),
		AsOf:       uint64(req.AsOf),
		TotalGrams: totalGrams,
		MerkleRoot: merkleRoot,
		IPFSCid:    ipfsCid,
		Auditor:    gethcommon.HexToAddress(req.Auditor),
		Signature:  sig,
	}

	txHash, err := h.writer.Publish(r.Context(), publishReq)
	if err != nil {
		h.log.Error("publish attestation on-chain", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "chain_error", "could not publish attestation on-chain")
		return
	}

	// Persist to DB log.
	if h.attestations != nil {
		record := domain.Attestation{
			ID:            uuid.Must(uuid.NewV7()),
			TimestampSec:  req.Timestamp,
			AsOfSec:       req.AsOf,
			TotalGramsWei: req.TotalGramsWei,
			MerkleRoot:    req.MerkleRoot,
			IPFSCid:       req.IPFSCid,
			Auditor:       req.Auditor,
			TxHash:        txHash,
			RecordedAt:    time.Now().UTC(),
		}
		if err := h.attestations.Create(r.Context(), record); err != nil {
			h.log.Error("persist attestation to db", zap.Error(err))
			// Non-fatal — the on-chain write succeeded.
		}
	}

	// Publish NATS event.
	if h.bus != nil {
		env := pkgevents.Envelope[map[string]interface{}]{
			EventID:     uuid.Must(uuid.NewV7()),
			EventType:   pkgevents.SubjReserveAttestation,
			OccurredAt:  time.Now().UTC(),
			AggregateID: txHash,
			Version:     1,
			Data: map[string]interface{}{
				"timestamp":       req.Timestamp,
				"as_of":           req.AsOf,
				"total_grams_wei": req.TotalGramsWei,
				"merkle_root":     req.MerkleRoot,
				"ipfs_cid":        req.IPFSCid,
				"auditor":         req.Auditor,
				"tx_hash":         txHash,
			},
		}
		if err := pkgevents.Publish(r.Context(), h.bus, env); err != nil {
			h.log.Error("publish attestation event to NATS", zap.Error(err))
			// Non-fatal.
		}
	}

	h.log.Info("attestation published on-chain",
		zap.String("tx_hash", txHash),
		zap.String("auditor", req.Auditor),
		zap.String("total_grams_wei", req.TotalGramsWei),
	)

	writeJSON(w, http.StatusCreated, map[string]string{
		"tx_hash": txHash,
		"status":  "published",
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Response helpers
// ─────────────────────────────────────────────────────────────────────────────

// computeRatio fetches the ERC-20 total supply and returns it along with the
// backing ratio (total_grams_wei / total_supply_wei). Returns ("0", "0") on error.
func (h *Handlers) computeRatio(ctx context.Context, totalGrams *big.Int) (string, string) {
	if h.supply == nil || totalGrams == nil || totalGrams.Sign() == 0 {
		return "0", "0"
	}
	supply, err := h.supply.TotalSupply(ctx)
	if err != nil {
		h.log.Warn("fetch total supply for ratio", zap.Error(err))
		return "0", "0"
	}
	if supply == nil || supply.Sign() == 0 {
		return "0", "0"
	}

	// ratio = totalGrams / supply expressed as a decimal with 6 fractional digits.
	// Both values are in 1e18 (wei) units so units cancel.
	// ratio_e6 = totalGrams * 1e6 / supply
	ratioE6 := new(big.Int).Mul(totalGrams, big.NewInt(1_000_000))
	ratioE6.Div(ratioE6, supply)

	whole := new(big.Int).Div(ratioE6, big.NewInt(1_000_000))
	frac := new(big.Int).Mod(ratioE6, big.NewInt(1_000_000))

	return supply.String(), fmt.Sprintf("%s.%06d", whole.String(), frac.Int64())
}

type currentResponse struct {
	TimestampSec  int64  `json:"timestamp"`
	AsOfSec       int64  `json:"as_of"`
	TotalGramsWei string `json:"total_grams_wei"`
	TotalTokensWei string `json:"total_tokens_wei"`
	Ratio         string `json:"ratio"`
	MerkleRoot    string `json:"merkle_root"`
	IPFSCid       string `json:"ipfs_cid"`
	Auditor       string `json:"auditor"`
}

func attestationResponse(a porchain.OnChainAttestation, totalTokensWei, ratio string) currentResponse {
	grams := ""
	if a.TotalGrams != nil {
		grams = a.TotalGrams.String()
	}
	return currentResponse{
		TimestampSec:   int64(a.Timestamp),
		AsOfSec:        int64(a.AsOf),
		TotalGramsWei:  grams,
		TotalTokensWei: totalTokensWei,
		Ratio:          ratio,
		MerkleRoot:     porchain.Bytes32ToHex(a.MerkleRoot),
		IPFSCid:        porchain.Bytes32ToHex(a.IPFSCid),
		Auditor:        a.Auditor.Hex(),
	}
}

func dbAttestationResponse(a domain.Attestation) map[string]interface{} {
	resp := map[string]interface{}{
		"id":              a.ID.String(),
		"timestamp":       a.TimestampSec,
		"as_of":           a.AsOfSec,
		"total_grams_wei": a.TotalGramsWei,
		"merkle_root":     a.MerkleRoot,
		"ipfs_cid":        a.IPFSCid,
		"auditor":         a.Auditor,
		"recorded_at":     a.RecordedAt.UTC().Format(time.RFC3339),
	}
	if a.OnChainIdx != nil {
		resp["on_chain_idx"] = *a.OnChainIdx
	}
	if a.TxHash != "" {
		resp["tx_hash"] = a.TxHash
	}
	return resp
}

// ─────────────────────────────────────────────────────────────────────────────
// Generic helpers
// ─────────────────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]string{"error": errCode, "message": msg})
}

func queryInt(r *http.Request, key string, def int) int {
	if s := r.URL.Query().Get(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return def
}
