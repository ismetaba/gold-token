package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/services/compliance/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/jurisdiction"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/monitor"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/repo"
)

// AdminHandlers wires together the monitoring worker and jurisdiction rule repo
// for admin-only operations.
type AdminHandlers struct {
	monRepo  repo.MonitoringRepo
	ruleRepo jurisdiction.RuleRepo
	worker   *monitor.Worker
	log      *zap.Logger
}

func NewAdminHandlers(
	monRepo repo.MonitoringRepo,
	ruleRepo jurisdiction.RuleRepo,
	worker *monitor.Worker,
	log *zap.Logger,
) *AdminHandlers {
	return &AdminHandlers{
		monRepo:  monRepo,
		ruleRepo: ruleRepo,
		worker:   worker,
		log:      log,
	}
}

// MountAdminRoutes attaches admin routes under r at /compliance/monitoring and
// /compliance/rules. Callers are responsible for applying authentication
// middleware before mounting.
func (h *AdminHandlers) MountAdminRoutes(r chi.Router) {
	r.Route("/compliance/monitoring", func(r chi.Router) {
		r.Get("/", h.listMonitoring)
		r.Post("/run", h.runMonitoring)
	})
	r.Route("/compliance/rules", func(r chi.Router) {
		r.Get("/", h.listRules)
		r.Patch("/{id}", h.updateRule)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Monitoring handlers
// ─────────────────────────────────────────────────────────────────────────────

// GET /compliance/monitoring
//
// Returns users whose next re-screening is due (next_check_at <= now).
func (h *AdminHandlers) listMonitoring(w http.ResponseWriter, r *http.Request) {
	if h.monRepo == nil {
		writeErr(w, http.StatusServiceUnavailable, "no_db", "database not available")
		return
	}

	due, err := h.monRepo.UsersDue(r.Context(), 200)
	if err != nil {
		h.log.Error("list monitoring due", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not list monitoring schedule")
		return
	}

	type row struct {
		UserID        string  `json:"user_id"`
		LastCheckedAt *string `json:"last_checked_at"`
		NextCheckAt   string  `json:"next_check_at"`
		FrequencyDays int     `json:"frequency_days"`
	}

	out := make([]row, 0, len(due))
	for _, s := range due {
		var lastStr *string
		if s.LastCheckedAt != nil {
			t := s.LastCheckedAt.UTC().Format(time.RFC3339)
			lastStr = &t
		}
		out = append(out, row{
			UserID:        s.UserID.String(),
			LastCheckedAt: lastStr,
			NextCheckAt:   s.NextCheckAt.UTC().Format(time.RFC3339),
			FrequencyDays: s.FrequencyDays,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": out, "count": len(out)})
}

// POST /compliance/monitoring/run
//
// Triggers an immediate monitoring pass. Returns counts of users screened and
// alerts published. This endpoint is idempotent (safe to call repeatedly).
func (h *AdminHandlers) runMonitoring(w http.ResponseWriter, r *http.Request) {
	if h.worker == nil {
		writeErr(w, http.StatusServiceUnavailable, "no_worker", "monitoring worker not available")
		return
	}

	if err := h.worker.RunOnce(r.Context()); err != nil {
		h.log.Error("manual monitoring run failed", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "monitoring run failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─────────────────────────────────────────────────────────────────────────────
// Jurisdiction rule handlers
// ─────────────────────────────────────────────────────────────────────────────

// GET /compliance/rules
//
// Returns all jurisdiction rules (active and inactive).
func (h *AdminHandlers) listRules(w http.ResponseWriter, r *http.Request) {
	if h.ruleRepo == nil {
		writeErr(w, http.StatusServiceUnavailable, "no_db", "database not available")
		return
	}

	rules, err := h.ruleRepo.ListAllRules(r.Context())
	if err != nil {
		h.log.Error("list rules", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not list rules")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": ruleResponses(rules),
		"count": len(rules),
	})
}

// PATCH /compliance/rules/{id}
//
// Body:
//
//	{ "active": true, "action": "require_edd" }
func (h *AdminHandlers) updateRule(w http.ResponseWriter, r *http.Request) {
	if h.ruleRepo == nil {
		writeErr(w, http.StatusServiceUnavailable, "no_db", "database not available")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing_id", "rule id is required")
		return
	}

	var body struct {
		Active *bool   `json:"active"`
		Action *string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}

	// Fetch existing rule to apply partial update.
	existing, err := h.ruleRepo.GetRule(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "rule not found")
		return
	}

	active := existing.Active
	if body.Active != nil {
		active = *body.Active
	}
	action := existing.Action
	if body.Action != nil {
		action = *body.Action
	}

	updated, err := h.ruleRepo.UpdateRule(r.Context(), id, active, action)
	if err != nil {
		h.log.Error("update rule", zap.String("id", id), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not update rule")
		return
	}

	writeJSON(w, http.StatusOK, ruleResponse(updated))
}

// ─────────────────────────────────────────────────────────────────────────────
// Response helpers
// ─────────────────────────────────────────────────────────────────────────────

func ruleResponse(r domain.JurisdictionRule) map[string]interface{} {
	m := map[string]interface{}{
		"id":        r.ID.String(),
		"arena":     r.Arena,
		"rule_type": r.RuleType,
		"action":    r.Action,
		"active":    r.Active,
	}
	if r.ThresholdGramsWei != nil {
		m["threshold_grams_wei"] = *r.ThresholdGramsWei
	}
	return m
}

func ruleResponses(rules []domain.JurisdictionRule) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(rules))
	for _, r := range rules {
		out = append(out, ruleResponse(r))
	}
	return out
}
