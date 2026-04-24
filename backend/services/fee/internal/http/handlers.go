// Package http provides the Fee Management service's HTTP API.
package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/fee/internal/repo"
)

type Handlers struct {
	schedules   repo.ScheduleRepo
	ledger      repo.LedgerRepo
	adminSecret string
	log         *zap.Logger
}

func NewHandlers(schedules repo.ScheduleRepo, ledger repo.LedgerRepo, adminSecret string, log *zap.Logger) *Handlers {
	return &Handlers{schedules: schedules, ledger: ledger, adminSecret: adminSecret, log: log}
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

	rl := httputil.NewRateLimiter(120, time.Minute)
	r.Use(rl.Middleware)

	r.Get("/health", h.health)

	r.Route("/fees", func(r chi.Router) {
		r.Get("/schedule", h.listSchedule)
		r.Get("/calculate", h.calculate)
		r.With(h.requireAdmin).Get("/ledger", h.listLedger)
		r.With(h.requireAdmin).Patch("/schedule/{id}", h.updateSchedule)
	})

	return r
}

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /fees/schedule
func (h *Handlers) listSchedule(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.schedules.ListAll(r.Context())
	if err != nil {
		h.log.Error("list schedules", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.FEE.INTERNAL", "failed to list schedules")
		return
	}

	type schedResp struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		OperationType   string  `json:"operation_type"`
		Arena           string  `json:"arena"`
		TierMinGramsWei string  `json:"tier_min_grams_wei"`
		TierMaxGramsWei *string `json:"tier_max_grams_wei"`
		FeeBPS          int     `json:"fee_bps"`
		MinFeeWei       string  `json:"min_fee_wei"`
		Active          bool    `json:"active"`
	}

	out := make([]schedResp, 0, len(schedules))
	for _, s := range schedules {
		resp := schedResp{
			ID:              s.ID.String(),
			Name:            s.Name,
			OperationType:   s.OperationType,
			Arena:           s.Arena,
			TierMinGramsWei: s.TierMinGramsWei.String(),
			FeeBPS:          s.FeeBPS,
			MinFeeWei:       s.MinFeeWei.String(),
			Active:          s.Active,
		}
		if s.TierMaxGramsWei != nil {
			v := s.TierMaxGramsWei.String()
			resp.TierMaxGramsWei = &v
		}
		out = append(out, resp)
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": out})
}

// GET /fees/calculate?operation_type=mint&amount_grams=10&arena=TR
func (h *Handlers) calculate(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	opType := q.Get("operation_type")
	amountGrams := q.Get("amount_grams")
	arena := q.Get("arena")

	if opType == "" || amountGrams == "" {
		writeError(w, http.StatusBadRequest, "GOLD.FEE.VALIDATION", "operation_type and amount_grams are required")
		return
	}
	if arena == "" {
		arena = "global"
	}

	// Convert grams to wei (multiply by 1e18).
	amountWei := gramsToWei(amountGrams)
	if amountWei == nil || amountWei.Sign() <= 0 {
		writeError(w, http.StatusBadRequest, "GOLD.FEE.VALIDATION", "invalid amount_grams")
		return
	}

	tier, err := h.schedules.FindTier(r.Context(), opType, arena, amountWei)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "GOLD.FEE.001", "no fee schedule found for this operation/arena/amount")
			return
		}
		h.log.Error("find tier", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.FEE.INTERNAL", "failed to find fee tier")
		return
	}

	feeWei := new(big.Int).Mul(amountWei, big.NewInt(int64(tier.FeeBPS)))
	feeWei.Div(feeWei, big.NewInt(10000))
	if tier.MinFeeWei != nil && feeWei.Cmp(tier.MinFeeWei) < 0 {
		feeWei = new(big.Int).Set(tier.MinFeeWei)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"fee_wei":        feeWei.String(),
		"fee_bps":        tier.FeeBPS,
		"amount_wei":     amountWei.String(),
		"operation_type": opType,
		"arena":          arena,
	})
}

// GET /fees/ledger?limit=50&offset=0
func (h *Handlers) listLedger(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	if limit > 200 {
		limit = 200
	}

	entries, err := h.ledger.List(r.Context(), limit, offset)
	if err != nil {
		h.log.Error("list ledger", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.FEE.INTERNAL", "failed to list ledger")
		return
	}

	type ledgerResp struct {
		ID            string  `json:"id"`
		OrderID       string  `json:"order_id"`
		OperationType string  `json:"operation_type"`
		AmountWei     string  `json:"amount_wei"`
		FeeWei        string  `json:"fee_wei"`
		FeeBPS        int     `json:"fee_bps"`
		Arena         string  `json:"arena"`
		Status        string  `json:"status"`
		CollectedAt   *string `json:"collected_at,omitempty"`
		CreatedAt     string  `json:"created_at"`
	}

	out := make([]ledgerResp, 0, len(entries))
	for _, e := range entries {
		resp := ledgerResp{
			ID:            e.ID.String(),
			OrderID:       e.OrderID.String(),
			OperationType: e.OperationType,
			AmountWei:     e.AmountWei.String(),
			FeeWei:        e.FeeWei.String(),
			FeeBPS:        e.FeeBPS,
			Arena:         e.Arena,
			Status:        e.Status,
			CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339),
		}
		if e.CollectedAt != nil {
			t := e.CollectedAt.UTC().Format(time.RFC3339)
			resp.CollectedAt = &t
		}
		out = append(out, resp)
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out, "limit": limit, "offset": offset})
}

// PATCH /fees/schedule/{id}
func (h *Handlers) updateSchedule(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.FEE.VALIDATION", "invalid id")
		return
	}

	var req struct {
		FeeBPS    *int   `json:"fee_bps"`
		MinFeeWei string `json:"min_fee_wei"`
		Active    *bool  `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.FEE.VALIDATION", "invalid request body")
		return
	}

	existing, err := h.schedules.ByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "GOLD.FEE.001", "schedule not found")
			return
		}
		h.log.Error("fetch schedule", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.FEE.INTERNAL", "internal error")
		return
	}

	feeBPS := existing.FeeBPS
	if req.FeeBPS != nil {
		feeBPS = *req.FeeBPS
	}
	minFee := existing.MinFeeWei
	if req.MinFeeWei != "" {
		minFee = new(big.Int)
		if _, ok := minFee.SetString(req.MinFeeWei, 10); !ok {
			writeError(w, http.StatusBadRequest, "GOLD.FEE.VALIDATION", "invalid min_fee_wei")
			return
		}
	}
	active := existing.Active
	if req.Active != nil {
		active = *req.Active
	}

	if err := h.schedules.Update(r.Context(), id, feeBPS, minFee, active); err != nil {
		h.log.Error("update schedule", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.FEE.INTERNAL", "failed to update schedule")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id.String(), "status": "updated"})
}

// ── middleware ───────────────────────────────────────────────────────────────

func (h *Handlers) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-Admin-Secret")
		if subtle.ConstantTimeCompare([]byte(secret), []byte(h.adminSecret)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid admin secret")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── helpers ─────────────────────────────────────────────────────────────────

// gramsToWei converts a decimal grams string to wei (grams * 1e18).
func gramsToWei(grams string) *big.Int {
	parts := strings.Split(grams, ".")
	if len(parts) > 2 {
		return nil
	}

	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}

	// Pad fractional part to 18 digits.
	for len(fracPart) < 18 {
		fracPart += "0"
	}
	fracPart = fracPart[:18]

	combined := intPart + fracPart
	result := new(big.Int)
	if _, ok := result.SetString(combined, 10); !ok {
		return nil
	}
	return result
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
