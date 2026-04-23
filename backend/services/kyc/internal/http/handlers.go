// Package http provides the KYC service's HTTP API.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/kyc/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/kyc/internal/jwtverify"
	"github.com/ismetaba/gold-token/backend/services/kyc/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/kyc/internal/storage"
)

const maxUploadBytes = 10 << 20 // 10 MiB

type ctxKey struct{}

// Handlers wires together repos, storage, event bus, and JWT verifier.
type Handlers struct {
	apps        repo.ApplicationRepo
	store       storage.Store
	verifier    *jwtverify.Verifier
	bus         *pkgevents.Bus
	adminSecret string
	log         *zap.Logger
}

// NewHandlers constructs the handler set.
func NewHandlers(
	apps repo.ApplicationRepo,
	store storage.Store,
	verifier *jwtverify.Verifier,
	bus *pkgevents.Bus,
	adminSecret string,
	log *zap.Logger,
) *Handlers {
	return &Handlers{
		apps:        apps,
		store:       store,
		verifier:    verifier,
		bus:         bus,
		adminSecret: adminSecret,
		log:         log,
	}
}

// Routes returns the configured router.
func (h *Handlers) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.health)

	r.Route("/kyc", func(r chi.Router) {
		r.With(h.requireAuth).Post("/submit", h.submit)
		r.With(h.requireAuth).Get("/status", h.status)
		r.With(h.requireAdmin).Patch("/{id}/review", h.review)
	})

	return r
}

// ── handlers ─────────────────────────────────────────────────────────────────

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// submit handles POST /kyc/submit (multipart/form-data).
//
// Form fields:
//   - document    (file)        — ID document image/PDF
//   - first_name  (string)
//   - last_name   (string)
//   - date_of_birth (string)   — YYYY-MM-DD
//   - nationality (string)
func (h *Handlers) submit(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxKey{}).(uuid.UUID)

	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_multipart", "could not parse multipart form")
		return
	}

	// Personal info fields.
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	dob := strings.TrimSpace(r.FormValue("date_of_birth"))
	nationality := strings.TrimSpace(r.FormValue("nationality"))

	if firstName == "" || lastName == "" || dob == "" || nationality == "" {
		writeErr(w, http.StatusUnprocessableEntity, "missing_fields",
			"first_name, last_name, date_of_birth, and nationality are required")
		return
	}

	// Validate date format.
	if _, err := time.Parse("2006-01-02", dob); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_date",
			"date_of_birth must be in YYYY-MM-DD format")
		return
	}

	// Document upload.
	file, header, err := r.FormFile("document")
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "missing_document", "document field is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes))
	if err != nil {
		h.log.Error("read uploaded file", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not read uploaded file")
		return
	}

	docPath, err := h.store.Save(r.Context(), userID, header.Filename, data)
	if err != nil {
		h.log.Error("store document", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not store document")
		return
	}

	now := time.Now().UTC()
	app := domain.Application{
		ID:           uuid.Must(uuid.NewV7()),
		UserID:       userID,
		Status:       domain.StatusPending,
		DocumentPath: docPath,
		FirstName:    firstName,
		LastName:     lastName,
		DateOfBirth:  dob,
		Nationality:  nationality,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.apps.Create(r.Context(), app); err != nil {
		if errors.Is(err, repo.ErrAlreadyActive) {
			writeErr(w, http.StatusConflict, "already_active",
				"you already have a pending or under-review KYC application")
			return
		}
		h.log.Error("create application", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not create application")
		return
	}

	h.publishEvent(r.Context(), domain.SubjKYCSubmitted, app.ID, userID, nil)

	writeJSON(w, http.StatusCreated, toResponse(app))
}

// status handles GET /kyc/status.
func (h *Handlers) status(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxKey{}).(uuid.UUID)

	app, err := h.apps.ByUserID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "no KYC application found for this user")
			return
		}
		h.log.Error("fetch application", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not fetch application")
		return
	}

	writeJSON(w, http.StatusOK, toResponse(app))
}

// review handles PATCH /kyc/:id/review (admin only).
//
// JSON body:
//
//	{ "action": "approve" | "reject", "note": "optional reviewer note" }
func (h *Handlers) review(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(idStr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_id", "application id must be a valid UUID")
		return
	}

	var req struct {
		Action string `json:"action"` // "approve" | "reject"
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "malformed request body")
		return
	}

	var newStatus domain.Status
	var eventSubj string
	switch req.Action {
	case "approve":
		newStatus = domain.StatusApproved
		eventSubj = domain.SubjKYCApproved
	case "reject":
		newStatus = domain.StatusRejected
		eventSubj = domain.SubjKYCRejected
	default:
		writeErr(w, http.StatusUnprocessableEntity, "invalid_action",
			"action must be \"approve\" or \"reject\"")
		return
	}

	// Fetch existing application to get userID for event.
	existing, err := h.apps.ByID(r.Context(), appID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "application not found")
			return
		}
		h.log.Error("fetch application for review", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not fetch application")
		return
	}

	now := time.Now().UTC()
	if err := h.apps.UpdateStatus(r.Context(), appID, newStatus, req.Note, &now); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "application not found")
			return
		}
		h.log.Error("update application status", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not update application")
		return
	}

	h.publishEvent(r.Context(), eventSubj, appID, existing.UserID, &now)

	writeJSON(w, http.StatusOK, map[string]string{
		"id":     appID.String(),
		"status": string(newStatus),
	})
}

// ── middleware ────────────────────────────────────────────────────────────────

func (h *Handlers) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "missing_token", "Authorization header required")
			return
		}
		tokenStr := strings.TrimPrefix(hdr, "Bearer ")
		userID, err := h.verifier.VerifyAccess(tokenStr)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid_token", "access token is invalid or expired")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handlers) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-Admin-Secret")
		if secret == "" || secret != h.adminSecret {
			writeErr(w, http.StatusForbidden, "forbidden", "valid X-Admin-Secret header required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── events ────────────────────────────────────────────────────────────────────

type kycEventData struct {
	ApplicationID string  `json:"application_id"`
	UserID        string  `json:"user_id"`
	ReviewedAt    *string `json:"reviewed_at,omitempty"`
}

func (h *Handlers) publishEvent(ctx context.Context, subj string, appID, userID uuid.UUID, reviewedAt *time.Time) {
	if h.bus == nil {
		return
	}
	data := kycEventData{
		ApplicationID: appID.String(),
		UserID:        userID.String(),
	}
	if reviewedAt != nil {
		s := reviewedAt.Format(time.RFC3339)
		data.ReviewedAt = &s
	}
	env := pkgevents.Envelope[kycEventData]{
		EventType:   subj,
		AggregateID: appID.String(),
		Data:        data,
	}
	if err := pkgevents.Publish(ctx, h.bus, env); err != nil {
		h.log.Warn("publish kyc event failed", zap.String("subject", subj), zap.Error(err))
	}
}

// ── response types ────────────────────────────────────────────────────────────

type applicationResponse struct {
	ID           string  `json:"id"`
	UserID       string  `json:"user_id"`
	Status       string  `json:"status"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	DateOfBirth  string  `json:"date_of_birth"`
	Nationality  string  `json:"nationality"`
	ReviewerNote string  `json:"reviewer_note,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	ReviewedAt   *string `json:"reviewed_at,omitempty"`
}

func toResponse(a domain.Application) applicationResponse {
	r := applicationResponse{
		ID:           a.ID.String(),
		UserID:       a.UserID.String(),
		Status:       string(a.Status),
		FirstName:    a.FirstName,
		LastName:     a.LastName,
		DateOfBirth:  a.DateOfBirth,
		Nationality:  a.Nationality,
		ReviewerNote: a.ReviewerNote,
		CreatedAt:    a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    a.UpdatedAt.Format(time.RFC3339),
	}
	if a.ReviewedAt != nil {
		s := a.ReviewedAt.Format(time.RFC3339)
		r.ReviewedAt = &s
	}
	return r
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}
