// Package http provides the wallet service HTTP API.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/wallet/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/wallet/internal/repo"
)

// BalanceReader is the on-chain balance interface.
type BalanceReader interface {
	BalanceOf(ctx context.Context, address string) (*big.Int, error)
}

// ctxKey for the authenticated user ID.
type ctxKey struct{}

// Handlers wires together repos and on-chain reader.
type Handlers struct {
	wallets  repo.WalletRepo
	txs      repo.TxRepo
	chain    BalanceReader
	jwtPub   interface{} // *rsa.PublicKey or nil (local dev — skip auth)
	localDev bool
	log      *zap.Logger
}

// NewHandlers constructs the handler set.
// jwtPublicKeyFile: path to RSA public key PEM; empty = local dev (auth skipped).
func NewHandlers(wallets repo.WalletRepo, txs repo.TxRepo, chain BalanceReader, jwtPublicKeyFile string, log *zap.Logger) (*Handlers, error) {
	h := &Handlers{wallets: wallets, txs: txs, chain: chain, log: log}

	if jwtPublicKeyFile == "" {
		h.localDev = true
		log.Warn("wallet: JWT auth disabled — local dev mode")
		return h, nil
	}

	pem, err := os.ReadFile(jwtPublicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read JWT public key: %w", err)
	}
	pub, err := jwt.ParseRSAPublicKeyFromPEM(pem)
	if err != nil {
		return nil, fmt.Errorf("parse JWT public key: %w", err)
	}
	h.jwtPub = pub
	return h, nil
}

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

	r.Route("/wallet", func(r chi.Router) {
		r.Use(h.requireAuth)
		r.Post("/address", h.createAddress)
		r.Get("/address", h.getAddress)
		r.Get("/balance", h.getBalance)
		r.Get("/transactions", h.listTransactions)
	})

	return r
}

// ─────────────────────────────────────────────────────────────────────────────
// Middleware
// ─────────────────────────────────────────────────────────────────────────────

func (h *Handlers) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.localDev {
			// In local dev, accept X-Dev-User-Id header as the user identity.
			rawID := r.Header.Get("X-Dev-User-Id")
			if rawID == "" {
				rawID = "00000000-0000-0000-0000-000000000001"
			}
			id, err := uuid.Parse(rawID)
			if err != nil {
				writeErr(w, http.StatusBadRequest, "invalid_user_id", "X-Dev-User-Id must be a valid UUID")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
			return
		}

		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "missing_token", "Authorization header required")
			return
		}
		tokenStr := strings.TrimPrefix(hdr, "Bearer ")

		t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return h.jwtPub, nil
		}, jwt.WithIssuer("gold-auth"), jwt.WithExpirationRequired())
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid_token", "access token is invalid or expired")
			return
		}

		claims, ok := t.Claims.(jwt.MapClaims)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "invalid_token", "bad claims")
			return
		}
		tt, _ := claims["token_type"].(string)
		if tt != "access" {
			writeErr(w, http.StatusUnauthorized, "invalid_token", "not an access token")
			return
		}
		subStr, _ := claims["sub"].(string)
		userID, err := uuid.Parse(subStr)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid_token", "invalid sub claim")
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, userID)))
	})
}

func userIDFrom(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(ctxKey{}).(uuid.UUID)
	return v
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /wallet/address — generate or return the authenticated user's Ethereum address.
func (h *Handlers) createAddress(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())

	// Idempotent: return existing wallet if already assigned.
	existing, err := h.wallets.ByUserID(r.Context(), userID)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]string{"address": existing.Address})
		return
	}
	if !errors.Is(err, repo.ErrNotFound) {
		h.log.Error("lookup wallet", zap.String("user_id", userID.String()), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not look up wallet")
		return
	}

	// Generate a new Ethereum address.
	// Production: derive from HD wallet or HSM (CAPA-18).
	priv, err := crypto.GenerateKey()
	if err != nil {
		h.log.Error("generate key", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not generate address")
		return
	}
	addr := crypto.PubkeyToAddress(priv.PublicKey)

	wallet := domain.Wallet{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    userID,
		Address:   addr.Hex(), // checksummed EIP-55
		CreatedAt: time.Now().UTC(),
	}
	if err := h.wallets.Create(r.Context(), wallet); err != nil {
		if errors.Is(err, repo.ErrAlreadyExists) {
			// Race condition — another request won; fetch and return.
			w2, _ := h.wallets.ByUserID(r.Context(), userID)
			writeJSON(w, http.StatusOK, map[string]string{"address": w2.Address})
			return
		}
		h.log.Error("create wallet", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not create wallet")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"address": wallet.Address})
}

// GET /wallet/address — get the authenticated user's Ethereum address.
func (h *Handlers) getAddress(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())

	wallet, err := h.wallets.ByUserID(r.Context(), userID)
	if errors.Is(err, repo.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "no_wallet", "no address assigned; POST /wallet/address first")
		return
	}
	if err != nil {
		h.log.Error("get address", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not fetch wallet")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"address": wallet.Address})
}

// GET /wallet/balance — return on-chain GOLD token balance for the user's address.
func (h *Handlers) getBalance(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())

	wallet, err := h.wallets.ByUserID(r.Context(), userID)
	if errors.Is(err, repo.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "no_wallet", "no address assigned; POST /wallet/address first")
		return
	}
	if err != nil {
		h.log.Error("get balance: lookup wallet", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not fetch wallet")
		return
	}

	if h.chain == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"address":    wallet.Address,
			"balance_wei": "0",
		})
		return
	}

	bal, err := h.chain.BalanceOf(r.Context(), wallet.Address)
	if err != nil {
		h.log.Error("get balance: chain call", zap.String("address", wallet.Address), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "chain_error", "could not fetch on-chain balance")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"address":     wallet.Address,
		"balance_wei": bal.String(),
	})
}

// GET /wallet/transactions?page=1&limit=20 — paginated transaction history.
func (h *Handlers) listTransactions(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())

	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	txs, err := h.txs.ListByUserID(r.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("list transactions", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not fetch transactions")
		return
	}

	type txResp struct {
		ID         string `json:"id"`
		TxHash     string `json:"tx_hash"`
		EventType  string `json:"event_type"`
		AmountWei  string `json:"amount_wei"`
		OccurredAt string `json:"occurred_at"`
	}
	out := make([]txResp, 0, len(txs))
	for _, tx := range txs {
		out = append(out, txResp{
			ID:         tx.ID.String(),
			TxHash:     tx.TxHash,
			EventType:  tx.EventType,
			AmountWei:  tx.AmountWei,
			OccurredAt: tx.OccurredAt.UTC().Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"page":         page,
		"limit":        limit,
		"transactions": out,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
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
