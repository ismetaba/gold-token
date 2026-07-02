// Package http exposes the mint-burn service's internal admin API.
//
// Not kullanıcı-facing — bu endpoint'ler sadece ops paneli için.
// Kullanıcı akışı event-bus üzerinden (Order Service → mint-burn).
package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/repo"
)

type Handlers struct {
	sagas repo.SagaRepo
	log   *zap.Logger
}

func NewHandlers(sagas repo.SagaRepo, log *zap.Logger) *Handlers {
	return &Handlers{sagas: sagas, log: log}
}

func (h *Handlers) Routes(env string) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	if env == "local" {
		r.Use(httputil.CORSMiddleware(httputil.LocalCORSConfig()))
	} else {
		r.Use(httputil.CORSMiddleware(httputil.DefaultCORSConfig()))
	}

	rl := httputil.NewRateLimiter(60, time.Minute)
	r.Use(rl.Middleware)

	r.Get("/health", h.health)
	r.Get("/readyz", h.readyz)
	r.Route("/admin/sagas", func(r chi.Router) {
		r.Get("/{id}", h.getSaga)
		// TODO(Faz1): /cancel, /retry endpoints with compliance officer signature
	})
	return r
}

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) readyz(w http.ResponseWriter, _ *http.Request) {
	// TODO: DB ping + NATS ping
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handlers) getSaga(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_uuid", "saga id must be a valid UUID")
		return
	}
	s, err := h.sagas.ByID(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "saga_not_found", "no saga with that id")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	httputil.WriteJSON(w, status, body)
}
