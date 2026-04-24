// Package http provides the Treasury service's HTTP API.
package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/treasury/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/treasury/internal/repo"
)

// Handlers wires repos and event bus together.
type Handlers struct {
	reserves    repo.ReserveRepo
	settlements repo.SettlementRepo
	recons      repo.ReconciliationRepo
	bus         *pkgevents.Bus
	adminSecret string
	log         *zap.Logger
}

// NewHandlers constructs the handler set.
func NewHandlers(
	reserves repo.ReserveRepo,
	settlements repo.SettlementRepo,
	recons repo.ReconciliationRepo,
	bus *pkgevents.Bus,
	adminSecret string,
	log *zap.Logger,
) *Handlers {
	return &Handlers{
		reserves:    reserves,
		settlements: settlements,
		recons:      recons,
		bus:         bus,
		adminSecret: adminSecret,
		log:         log,
	}
}

// Routes returns the configured chi router.
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

	rl := httputil.NewRateLimiter(120, time.Minute)
	r.Use(rl.Middleware)

	r.Get("/health", h.health)

	r.Route("/treasury", func(r chi.Router) {
		r.With(h.requireAdmin).Get("/reserves", h.listReserves)
		r.With(h.requireAdmin).Get("/settlements", h.listSettlements)
		r.With(h.requireAdmin).Post("/settlements", h.createSettlement)
		r.With(h.requireAdmin).Post("/reconcile", h.reconcile)
	})

	return r
}

// ── middleware ─────────────────────────────────────────────────────────────

func (h *Handlers) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-Admin-Secret")
		if subtle.ConstantTimeCompare([]byte(secret), []byte(h.adminSecret)) != 1 {
			writeError(w, http.StatusUnauthorized, "GOLD.TREASURY.UNAUTHORIZED", "invalid admin secret")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── handlers ───────────────────────────────────────────────────────────────

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /treasury/reserves
func (h *Handlers) listReserves(w http.ResponseWriter, r *http.Request) {
	accs, err := h.reserves.List(r.Context())
	if err != nil {
		h.log.Error("list reserves failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.TREASURY.INTERNAL", "failed to list reserves")
		return
	}

	type reserveResp struct {
		ID          string `json:"id"`
		AccountType string `json:"account_type"`
		BalanceWei  string `json:"balance_wei"`
		Currency    string `json:"currency"`
		Arena       string `json:"arena"`
		UpdatedAt   string `json:"updated_at"`
	}
	out := make([]reserveResp, len(accs))
	for i, a := range accs {
		out[i] = reserveResp{
			ID:          a.ID.String(),
			AccountType: string(a.AccountType),
			BalanceWei:  a.BalanceWei.String(),
			Currency:    a.Currency,
			Arena:       a.Arena,
			UpdatedAt:   a.UpdatedAt.UTC().Format(time.RFC3339),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"reserves": out})
}

// GET /treasury/settlements?limit=50&offset=0
func (h *Handlers) listSettlements(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	if limit > 200 {
		limit = 200
	}

	ss, err := h.settlements.List(r.Context(), limit, offset)
	if err != nil {
		h.log.Error("list settlements failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.TREASURY.INTERNAL", "failed to list settlements")
		return
	}

	type settlementResp struct {
		ID             string  `json:"id"`
		SettlementType string  `json:"settlement_type"`
		AccountID      string  `json:"account_id"`
		AmountWei      string  `json:"amount_wei"`
		ReferenceID    string  `json:"reference_id"`
		ReferenceType  string  `json:"reference_type"`
		TxHash         string  `json:"tx_hash"`
		Status         string  `json:"status"`
		SettledAt      *string `json:"settled_at,omitempty"`
		CreatedAt      string  `json:"created_at"`
	}
	out := make([]settlementResp, len(ss))
	for i, s := range ss {
		resp := settlementResp{
			ID:             s.ID.String(),
			SettlementType: string(s.SettlementType),
			AccountID:      s.AccountID.String(),
			AmountWei:      s.AmountWei.String(),
			ReferenceID:    s.ReferenceID.String(),
			ReferenceType:  s.ReferenceType,
			TxHash:         s.TxHash,
			Status:         string(s.Status),
			CreatedAt:      s.CreatedAt.UTC().Format(time.RFC3339),
		}
		if s.SettledAt != nil {
			ts := s.SettledAt.UTC().Format(time.RFC3339)
			resp.SettledAt = &ts
		}
		out[i] = resp
	}
	writeJSON(w, http.StatusOK, map[string]any{"settlements": out, "limit": limit, "offset": offset})
}

// POST /treasury/settlements
func (h *Handlers) createSettlement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SettlementType string `json:"settlement_type"`
		AccountID      string `json:"account_id"`
		AmountWei      string `json:"amount_wei"`
		ReferenceID    string `json:"reference_id"`
		ReferenceType  string `json:"reference_type"`
		TxHash         string `json:"tx_hash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.TREASURY.VALIDATION", "invalid request body")
		return
	}

	sType := domain.SettlementType(req.SettlementType)
	if sType != domain.SettlementCredit && sType != domain.SettlementDebit {
		writeError(w, http.StatusBadRequest, "GOLD.TREASURY.VALIDATION", "settlement_type must be credit or debit")
		return
	}
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.TREASURY.VALIDATION", "invalid account_id")
		return
	}
	refID, err := uuid.Parse(req.ReferenceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.TREASURY.VALIDATION", "invalid reference_id")
		return
	}
	amountWei, ok := new(big.Int).SetString(req.AmountWei, 10)
	if !ok || amountWei.Sign() <= 0 {
		writeError(w, http.StatusBadRequest, "GOLD.TREASURY.VALIDATION", "amount_wei must be a positive integer string")
		return
	}
	if req.ReferenceType == "" {
		writeError(w, http.StatusBadRequest, "GOLD.TREASURY.VALIDATION", "reference_type is required")
		return
	}

	now := time.Now().UTC()
	s := domain.Settlement{
		ID:             uuid.New(),
		SettlementType: sType,
		AccountID:      accountID,
		AmountWei:      amountWei,
		ReferenceID:    refID,
		ReferenceType:  req.ReferenceType,
		TxHash:         req.TxHash,
		Status:         domain.SettlementPending,
		CreatedAt:      now,
	}

	// Apply balance change immediately for manual settlements.
	if sType == domain.SettlementCredit {
		if err := h.reserves.Credit(r.Context(), accountID, amountWei); err != nil {
			h.handleRepoError(w, err, "GOLD.TREASURY.001")
			return
		}
	} else {
		if err := h.reserves.Debit(r.Context(), accountID, amountWei); err != nil {
			h.handleRepoError(w, err, "GOLD.TREASURY.002")
			return
		}
	}

	ts := now
	s.Status = domain.SettlementSettled
	s.SettledAt = &ts
	if err := h.settlements.Create(r.Context(), s); err != nil {
		h.log.Error("create manual settlement failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.TREASURY.INTERNAL", "failed to record settlement")
		return
	}

	h.publishSettlementEvent(r.Context(), s, accountID)

	writeJSON(w, http.StatusCreated, map[string]any{
		"settlement_id": s.ID.String(),
		"status":        string(s.Status),
	})
}

// POST /treasury/reconcile
func (h *Handlers) reconcile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID        string `json:"account_id"`
		ActualBalanceWei string `json:"actual_balance_wei"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.TREASURY.VALIDATION", "invalid request body")
		return
	}

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.TREASURY.VALIDATION", "invalid account_id")
		return
	}
	actualWei, ok := new(big.Int).SetString(req.ActualBalanceWei, 10)
	if !ok || actualWei.Sign() < 0 {
		writeError(w, http.StatusBadRequest, "GOLD.TREASURY.VALIDATION", "actual_balance_wei must be a non-negative integer string")
		return
	}

	acc, err := h.reserves.ByID(r.Context(), accountID)
	if err != nil {
		h.handleRepoError(w, err, "GOLD.TREASURY.001")
		return
	}

	discrepancy := new(big.Int).Sub(actualWei, acc.BalanceWei)
	status := domain.ReconciliationOK
	if discrepancy.Sign() != 0 {
		status = domain.ReconciliationDiscrepancy
	}

	log := domain.ReconciliationLog{
		ID:                 uuid.New(),
		AccountID:          accountID,
		ExpectedBalanceWei: acc.BalanceWei,
		ActualBalanceWei:   actualWei,
		DiscrepancyWei:     discrepancy,
		Status:             status,
		ReconciledAt:       time.Now().UTC(),
	}
	if err := h.recons.Create(r.Context(), log); err != nil {
		h.log.Error("create reconciliation log failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.TREASURY.INTERNAL", "failed to persist reconciliation")
		return
	}

	// Publish reconciled event.
	_ = pkgevents.Publish(r.Context(), h.bus, pkgevents.Envelope[map[string]any]{
		EventType:   pkgevents.SubjTreasuryReconciled,
		AggregateID: accountID.String(),
		Data: map[string]any{
			"reconciliation_id":     log.ID.String(),
			"account_id":            accountID.String(),
			"expected_balance_wei":  acc.BalanceWei.String(),
			"actual_balance_wei":    actualWei.String(),
			"discrepancy_wei":       discrepancy.String(),
			"status":                string(status),
			"reconciled_at":         log.ReconciledAt.Format(time.RFC3339),
		},
	})

	resp := map[string]any{
		"reconciliation_id":    log.ID.String(),
		"account_id":           accountID.String(),
		"expected_balance_wei": acc.BalanceWei.String(),
		"actual_balance_wei":   actualWei.String(),
		"discrepancy_wei":      discrepancy.String(),
		"status":               string(status),
		"reconciled_at":        log.ReconciledAt.Format(time.RFC3339),
	}
	if status == domain.ReconciliationDiscrepancy {
		h.log.Warn("reconciliation discrepancy detected",
			zap.String("account_id", accountID.String()),
			zap.String("discrepancy_wei", discrepancy.String()),
		)
		writeJSON(w, http.StatusOK, resp) // 200 — discrepancy is a business outcome, not an error
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ── helpers ────────────────────────────────────────────────────────────────

func (h *Handlers) handleRepoError(w http.ResponseWriter, err error, notFoundCode string) {
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, notFoundCode, "account not found")
	case errors.Is(err, repo.ErrInsufficientBalance):
		writeError(w, http.StatusUnprocessableEntity, "GOLD.TREASURY.002", "insufficient reserve balance")
	default:
		h.log.Error("repo error", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.TREASURY.INTERNAL", "internal error")
	}
}

func (h *Handlers) publishSettlementEvent(ctx context.Context, s domain.Settlement, accountID uuid.UUID) {
	_ = pkgevents.Publish(ctx, h.bus, pkgevents.Envelope[map[string]any]{
		EventType:   pkgevents.SubjTreasurySettlement,
		AggregateID: accountID.String(),
		Data: map[string]any{
			"settlement_id":   s.ID.String(),
			"account_id":      accountID.String(),
			"settlement_type": string(s.SettlementType),
			"amount_wei":      s.AmountWei.String(),
			"reference_id":    s.ReferenceID.String(),
			"reference_type":  s.ReferenceType,
			"tx_hash":         s.TxHash,
		},
	})
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error":   code,
		"message": message,
	})
}
