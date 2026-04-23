// Package http provides the auth service's HTTP API.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/auth/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/auth/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/auth/internal/tokens"
)

const bcryptCost = 12

type ctxKey struct{}

// Handlers wires together repos, token manager, and optional event bus.
type Handlers struct {
	users  repo.UserRepo
	tokens *tokens.Manager
	bus    *pkgevents.Bus
	log    *zap.Logger
}

func NewHandlers(users repo.UserRepo, tm *tokens.Manager, bus *pkgevents.Bus, log *zap.Logger) *Handlers {
	return &Handlers{users: users, tokens: tm, bus: bus, log: log}
}

func (h *Handlers) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.health)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.register)
		r.Post("/login", h.login)
		r.Post("/refresh", h.refresh)
		r.With(h.requireAuth).Get("/me", h.me)
	})

	return r
}

// ── request/response types ───────────────────────────────────────────────────

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type meResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

// ── handlers ─────────────────────────────────────────────────────────────────

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "malformed request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeErr(w, http.StatusUnprocessableEntity, "missing_fields", "email and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeErr(w, http.StatusUnprocessableEntity, "password_too_short", "password must be at least 8 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		h.log.Error("bcrypt failed", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not process password")
		return
	}

	now := time.Now().UTC()
	user := domain.User{
		ID:           uuid.Must(uuid.NewV7()),
		Email:        req.Email,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.users.Create(r.Context(), user); err != nil {
		if errors.Is(err, repo.ErrEmailTaken) {
			writeErr(w, http.StatusConflict, "email_taken", "an account with this email already exists")
			return
		}
		h.log.Error("create user", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not create user")
		return
	}

	h.publishUserRegistered(r.Context(), user)

	pair, err := h.issuePair(user.ID)
	if err != nil {
		h.log.Error("issue token pair", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not issue tokens")
		return
	}

	writeJSON(w, http.StatusCreated, pair)
}

func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "malformed request body")
		return
	}

	user, err := h.users.ByEmail(r.Context(), req.Email)
	if err != nil {
		// Timing-safe: always run bcrypt to prevent user enumeration.
		_ = bcrypt.CompareHashAndPassword(
			[]byte("$2a$12$dummyhashdummyhashdummyhashdummyhashdummyhashd"),
			[]byte(req.Password),
		)
		writeErr(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
		return
	}

	pair, err := h.issuePair(user.ID)
	if err != nil {
		h.log.Error("issue token pair", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not issue tokens")
		return
	}

	writeJSON(w, http.StatusOK, pair)
}

func (h *Handlers) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "malformed request body")
		return
	}

	userID, err := h.tokens.VerifyRefresh(req.RefreshToken)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid_token", "refresh token is invalid or expired")
		return
	}

	// Confirm user still exists.
	if _, err := h.users.ByID(r.Context(), userID); err != nil {
		writeErr(w, http.StatusUnauthorized, "user_not_found", "user no longer exists")
		return
	}

	pair, err := h.issuePair(userID)
	if err != nil {
		h.log.Error("issue token pair", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not issue tokens")
		return
	}

	writeJSON(w, http.StatusOK, pair)
}

func (h *Handlers) me(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxKey{}).(uuid.UUID)
	user, err := h.users.ByID(r.Context(), userID)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "user_not_found", "user not found")
		return
	}
	writeJSON(w, http.StatusOK, meResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	})
}

// ── auth middleware ───────────────────────────────────────────────────────────

func (h *Handlers) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "missing_token", "Authorization header required")
			return
		}
		tokenStr := strings.TrimPrefix(hdr, "Bearer ")
		userID, err := h.tokens.VerifyAccess(tokenStr)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid_token", "access token is invalid or expired")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── events ───────────────────────────────────────────────────────────────────

type userRegisteredData struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

func (h *Handlers) publishUserRegistered(ctx context.Context, user domain.User) {
	if h.bus == nil {
		return
	}
	env := pkgevents.Envelope[userRegisteredData]{
		EventType:   "gold.user.registered.v1",
		AggregateID: user.ID.String(),
		Data:        userRegisteredData{UserID: user.ID.String(), Email: user.Email},
	}
	if err := pkgevents.Publish(ctx, h.bus, env); err != nil {
		h.log.Warn("publish user.registered failed", zap.Error(err))
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (h *Handlers) issuePair(userID uuid.UUID) (domain.TokenPair, error) {
	access, err := h.tokens.IssueAccess(userID)
	if err != nil {
		return domain.TokenPair{}, err
	}
	refresh, err := h.tokens.IssueRefresh(userID)
	if err != nil {
		return domain.TokenPair{}, err
	}
	return domain.TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    h.tokens.AccessTTLSeconds(),
		TokenType:    "Bearer",
	}, nil
}

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
