// Package http provides the compliance service HTTP API.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/screener"
)

// Handlers wires together the compliance repo and screener.
type Handlers struct {
	repo     repo.ComplianceRepo
	screener screener.Screener
	log      *zap.Logger
}

func NewHandlers(r repo.ComplianceRepo, sc screener.Screener, log *zap.Logger) *Handlers {
	return &Handlers{repo: r, screener: sc, log: log}
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

	r.Route("/compliance", func(r chi.Router) {
		r.Post("/screen", h.screen)
		r.Get("/status/{userId}", h.status)
	})

	return r
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /compliance/screen
//
// Body:
//
//	{
//	  "user_id":  "<uuid>",
//	  "name":     "Full Name",
//	  "country":  "TR",          // ISO-3166-1 alpha-2
//	  "order_id": "<uuid>"       // optional, links result to an order
//	}
func (h *Handlers) screen(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID  string `json:"user_id"`
		Name    string `json:"name"`
		Country string `json:"country"`
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}

	userID, err := uuid.Parse(body.UserID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_user_id", "user_id must be a valid UUID")
		return
	}

	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "missing_name", "name is required")
		return
	}
	if !httputil.ValidateName(body.Name, 1, 200) {
		writeErr(w, http.StatusBadRequest, "invalid_name", "name must be 1-200 characters with no control characters")
		return
	}
	if body.Country != "" && !httputil.ValidateCountryCode(body.Country) {
		writeErr(w, http.StatusBadRequest, "invalid_country", "country must be a valid ISO 3166-1 alpha-2 code")
		return
	}

	var orderID *uuid.UUID
	if body.OrderID != "" {
		oid, err := uuid.Parse(body.OrderID)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_order_id", "order_id must be a valid UUID")
			return
		}
		orderID = &oid
	}

	res, err := h.runScreen(r.Context(), userID, body.Name, body.Country, orderID)
	if err != nil {
		h.log.Error("screening failed", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "screening failed")
		return
	}

	writeJSON(w, http.StatusOK, screeningResultResponse(res))
}

// GET /compliance/status/{userId}
func (h *Handlers) status(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "userId")
	userID, err := uuid.Parse(rawID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_user_id", "userId must be a valid UUID")
		return
	}

	state, err := h.repo.StateByUserID(r.Context(), userID)
	if errors.Is(err, repo.ErrNotFound) {
		// No screening on record — treat as clear.
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"user_id":    userID.String(),
			"status":     string(domain.UserClear),
			"updated_at": nil,
		})
		return
	}
	if err != nil {
		h.log.Error("fetch compliance state", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not fetch compliance status")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":    state.UserID.String(),
		"status":     string(state.Status),
		"updated_at": state.UpdatedAt.UTC().Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// runScreen — shared logic used by both HTTP and NATS paths
// ─────────────────────────────────────────────────────────────────────────────

func (h *Handlers) RunScreen(ctx context.Context, userID uuid.UUID, name, country string, orderID *uuid.UUID) (domain.ScreeningResult, error) {
	return h.runScreen(ctx, userID, name, country, orderID)
}

func (h *Handlers) runScreen(ctx context.Context, userID uuid.UUID, name, country string, orderID *uuid.UUID) (domain.ScreeningResult, error) {
	sr, err := h.screener.Screen(ctx, screener.Request{Name: name, Country: country})
	if err != nil {
		return domain.ScreeningResult{}, err
	}

	status := domain.ScreeningApproved
	if !sr.Allowed {
		status = domain.ScreeningRejected
	}

	result := domain.ScreeningResult{
		ID:          uuid.Must(uuid.NewV7()),
		UserID:      userID,
		OrderID:     orderID,
		Status:      status,
		MatchType:   domain.MatchType(sr.MatchType),
		MatchedName: sr.MatchedName,
		Provider:    sr.Provider,
		ScreenedAt:  time.Now().UTC(),
	}

	if h.repo != nil {
		if err := h.repo.SaveResult(ctx, result); err != nil {
			return result, err
		}

		// Update aggregate user status.
		userStatus := domain.UserClear
		if !sr.Allowed {
			userStatus = domain.UserBlocked
		}
		_ = h.repo.UpsertState(ctx, domain.ComplianceState{
			UserID:    userID,
			Status:    userStatus,
			UpdatedAt: result.ScreenedAt,
		})
	}

	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Response helpers
// ─────────────────────────────────────────────────────────────────────────────

func screeningResultResponse(r domain.ScreeningResult) map[string]interface{} {
	m := map[string]interface{}{
		"result_id":    r.ID.String(),
		"user_id":      r.UserID.String(),
		"status":       string(r.Status),
		"match_type":   string(r.MatchType),
		"matched_name": r.MatchedName,
		"provider":     r.Provider,
		"screened_at":  r.ScreenedAt.UTC().Format(time.RFC3339),
	}
	if r.OrderID != nil {
		m["order_id"] = r.OrderID.String()
	}
	return m
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]string{"error": errCode, "message": msg})
}
