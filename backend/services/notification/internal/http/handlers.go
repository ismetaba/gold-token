package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/notification/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/notification/internal/repo"
)

type ctxKey struct{}

type Handlers struct {
	deliveries repo.DeliveryRepo
	prefs      repo.PreferencesRepo
	verifyFunc func(token string) (uuid.UUID, error)
	log        *zap.Logger
}

func NewHandlers(
	deliveries repo.DeliveryRepo,
	prefs repo.PreferencesRepo,
	verifyFunc func(string) (uuid.UUID, error),
	log *zap.Logger,
) *Handlers {
	return &Handlers{deliveries: deliveries, prefs: prefs, verifyFunc: verifyFunc, log: log}
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

	r.Route("/notifications", func(r chi.Router) {
		r.Use(h.requireAuth)
		r.Get("/", h.list)
		r.Patch("/{id}/read", h.markRead)
		r.Get("/preferences", h.getPreferences)
		r.Post("/preferences", h.setPreferences)
	})

	return r
}

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxKey{}).(uuid.UUID)
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	if limit > 200 {
		limit = 200
	}

	deliveries, err := h.deliveries.ListByUser(r.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("list notifications", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.NOTIFICATION.INTERNAL", "failed to list notifications")
		return
	}

	type deliveryResp struct {
		ID        string  `json:"id"`
		Channel   string  `json:"channel"`
		Subject   string  `json:"subject"`
		Body      string  `json:"body"`
		Status    string  `json:"status"`
		SentAt    *string `json:"sent_at,omitempty"`
		CreatedAt string  `json:"created_at"`
	}

	out := make([]deliveryResp, 0, len(deliveries))
	for _, d := range deliveries {
		resp := deliveryResp{
			ID:        d.ID.String(),
			Channel:   d.Channel,
			Subject:   d.Subject,
			Body:      d.Body,
			Status:    d.Status,
			CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339),
		}
		if d.SentAt != nil {
			t := d.SentAt.UTC().Format(time.RFC3339)
			resp.SentAt = &t
		}
		out = append(out, resp)
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": out, "limit": limit, "offset": offset})
}

func (h *Handlers) markRead(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxKey{}).(uuid.UUID)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.NOTIFICATION.VALIDATION", "invalid id")
		return
	}

	if err := h.deliveries.MarkRead(r.Context(), id, userID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "GOLD.NOTIFICATION.001", "notification not found")
			return
		}
		h.log.Error("mark read", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.NOTIFICATION.INTERNAL", "failed to mark read")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id.String(), "status": "read"})
}

func (h *Handlers) getPreferences(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxKey{}).(uuid.UUID)

	prefs, err := h.prefs.ByUserID(r.Context(), userID)
	if err != nil {
		h.log.Error("get preferences", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.NOTIFICATION.INTERNAL", "failed to get preferences")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"email_enabled":   prefs.EmailEnabled,
		"webhook_url":     prefs.WebhookURL,
		"webhook_enabled": prefs.WebhookEnabled,
		"inapp_enabled":   prefs.InappEnabled,
	})
}

func (h *Handlers) setPreferences(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxKey{}).(uuid.UUID)

	var req struct {
		EmailEnabled   *bool  `json:"email_enabled"`
		WebhookURL     string `json:"webhook_url"`
		WebhookEnabled *bool  `json:"webhook_enabled"`
		InappEnabled   *bool  `json:"inapp_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.NOTIFICATION.VALIDATION", "invalid request body")
		return
	}

	existing, _ := h.prefs.ByUserID(r.Context(), userID)

	p := domain.Preferences{
		ID:             existing.ID,
		UserID:         userID,
		EmailEnabled:   existing.EmailEnabled,
		WebhookURL:     existing.WebhookURL,
		WebhookEnabled: existing.WebhookEnabled,
		InappEnabled:   existing.InappEnabled,
		UpdatedAt:      time.Now().UTC(),
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.Must(uuid.NewV7())
	}
	if req.EmailEnabled != nil {
		p.EmailEnabled = *req.EmailEnabled
	}
	if req.WebhookURL != "" {
		p.WebhookURL = req.WebhookURL
	}
	if req.WebhookEnabled != nil {
		p.WebhookEnabled = *req.WebhookEnabled
	}
	if req.InappEnabled != nil {
		p.InappEnabled = *req.InappEnabled
	}

	if err := h.prefs.Upsert(r.Context(), p); err != nil {
		h.log.Error("upsert preferences", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.NOTIFICATION.INTERNAL", "failed to save preferences")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// ── middleware ───────────────────────────────────────────────────────────────

func (h *Handlers) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing_token", "Authorization header required")
			return
		}
		token := strings.TrimPrefix(hdr, "Bearer ")

		if h.verifyFunc == nil {
			writeError(w, http.StatusUnauthorized, "auth_unavailable", "JWT verification not configured")
			return
		}

		userID, err := h.verifyFunc(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_token", "access token is invalid or expired")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
