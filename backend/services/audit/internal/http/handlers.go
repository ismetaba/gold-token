// Package http provides the Audit Log service's HTTP API (read-only).
package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/audit/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/audit/internal/repo"
)

// Handlers provides read-only HTTP access to the audit log.
type Handlers struct {
	entries     repo.EntryRepo
	adminSecret string
	log         *zap.Logger
}

// NewHandlers constructs the handler set.
func NewHandlers(entries repo.EntryRepo, adminSecret string, log *zap.Logger) *Handlers {
	return &Handlers{entries: entries, adminSecret: adminSecret, log: log}
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

	r.Route("/audit", func(r chi.Router) {
		r.Use(h.requireAdmin)
		r.Get("/logs", h.listLogs)
		r.Get("/logs/{id}", h.getLog)
	})

	return r
}

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /audit/logs?entity_type=order&entity_id=xxx&actor_id=yyy&from=2026-01-01T00:00:00Z&to=...&limit=50&offset=0
func (h *Handlers) listLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := domain.ListFilter{
		Limit:  queryInt(r, "limit", 50),
		Offset: queryInt(r, "offset", 0),
	}
	if f.Limit > 200 {
		f.Limit = 200
	}

	if v := q.Get("entity_type"); v != "" {
		f.EntityType = &v
	}
	if v := q.Get("entity_id"); v != "" {
		f.EntityID = &v
	}
	if v := q.Get("actor_id"); v != "" {
		f.ActorID = &v
	}
	if v := q.Get("action"); v != "" {
		f.Action = &v
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}

	entries, err := h.entries.List(r.Context(), f)
	if err != nil {
		h.log.Error("list audit entries", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.AUDIT.INTERNAL", "failed to list entries")
		return
	}

	out := make([]entryResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, toEntryResponse(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out, "limit": f.Limit, "offset": f.Offset})
}

// GET /audit/logs/{id}
func (h *Handlers) getLog(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.AUDIT.VALIDATION", "invalid id")
		return
	}

	entry, err := h.entries.ByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "GOLD.AUDIT.001", "entry not found")
			return
		}
		h.log.Error("get audit entry", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.AUDIT.INTERNAL", "failed to get entry")
		return
	}

	writeJSON(w, http.StatusOK, toEntryResponse(entry))
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

// ── response types ──────────────────────────────────────────────────────────

type entryResponse struct {
	ID         string          `json:"id"`
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	ActorID    string          `json:"actor_id"`
	ActorType  string          `json:"actor_type"`
	EntityID   string          `json:"entity_id"`
	EntityType string          `json:"entity_type"`
	Action     string          `json:"action"`
	Metadata   json.RawMessage `json:"metadata"`
	OccurredAt string          `json:"occurred_at"`
	IngestedAt string          `json:"ingested_at"`
}

func toEntryResponse(e domain.Entry) entryResponse {
	return entryResponse{
		ID:         e.ID.String(),
		EventID:    e.EventID.String(),
		EventType:  e.EventType,
		ActorID:    e.ActorID,
		ActorType:  e.ActorType,
		EntityID:   e.EntityID,
		EntityType: e.EntityType,
		Action:     e.Action,
		Metadata:   e.Metadata,
		OccurredAt: e.OccurredAt.UTC().Format(time.RFC3339),
		IngestedAt: e.IngestedAt.UTC().Format(time.RFC3339),
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

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
