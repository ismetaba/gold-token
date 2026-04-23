// Package http exposes the mint-burn service's internal admin API.
//
// Not kullanıcı-facing — bu endpoint'ler sadece ops paneli için.
// Kullanıcı akışı event-bus üzerinden (Order Service → mint-burn).
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/repo"
)

type Handlers struct {
	sagas repo.SagaRepo
	log   *zap.Logger
}

func NewHandlers(sagas repo.SagaRepo, log *zap.Logger) *Handlers {
	return &Handlers{sagas: sagas, log: log}
}

func (h *Handlers) Routes() chi.Router {
	r := chi.NewRouter()
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_uuid"})
		return
	}
	s, err := h.sagas.ByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "saga_not_found"})
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
