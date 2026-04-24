package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/admin/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/admin/internal/repo"
)

type ctxKey struct{}

type Handlers struct {
	users       repo.AdminUserRepo
	adminSecret string // master admin secret for bootstrap operations
	log         *zap.Logger
}

func NewHandlers(users repo.AdminUserRepo, adminSecret string, log *zap.Logger) *Handlers {
	return &Handlers{users: users, adminSecret: adminSecret, log: log}
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

	r.Route("/admin", func(r chi.Router) {
		r.Post("/login", h.login)

		// Bootstrap: create first admin user with master secret.
		r.With(h.requireMasterSecret).Post("/bootstrap", h.bootstrap)

		// Authenticated admin routes.
		r.Group(func(r chi.Router) {
			r.Use(h.requireAdminAuth)
			r.Get("/me", h.me)
		})
	})

	return r
}

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /admin/login
func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	user, err := h.users.ByEmail(r.Context(), req.Email)
	if err != nil {
		// Timing-safe: always run bcrypt even on miss.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$invalidhashforconstanttime..."), []byte(req.Password))
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	if !user.Active {
		writeError(w, http.StatusUnauthorized, "account_disabled", "account is disabled")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	// For now, return the user info. JWT issuance will be added when admin JWT keys are configured.
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    user.ID.String(),
		"email": user.Email,
		"role":  string(user.Role),
	})
}

// POST /admin/bootstrap — create first admin user (master secret required).
func (h *Handlers) bootstrap(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusUnprocessableEntity, "missing_fields", "email and password required")
		return
	}

	role := domain.Role(req.Role)
	if role == "" {
		role = domain.RoleSuperAdmin
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		h.log.Error("hash password", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal", "failed to hash password")
		return
	}

	now := time.Now().UTC()
	user := domain.AdminUser{
		ID:           uuid.Must(uuid.NewV7()),
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         role,
		Active:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.users.Create(r.Context(), user); err != nil {
		h.log.Error("create admin user", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal", "failed to create admin user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":    user.ID.String(),
		"email": user.Email,
		"role":  string(user.Role),
	})
}

// GET /admin/me
func (h *Handlers) me(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(ctxKey{}).(domain.AdminUser)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    user.ID.String(),
		"email": user.Email,
		"role":  string(user.Role),
	})
}

// ── middleware ───────────────────────────────────────────────────────────────

func (h *Handlers) requireMasterSecret(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-Admin-Secret")
		if subtle.ConstantTimeCompare([]byte(secret), []byte(h.adminSecret)) != 1 {
			writeError(w, http.StatusForbidden, "forbidden", "valid X-Admin-Secret required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handlers) requireAdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For now, use X-Admin-Secret as auth mechanism.
		// Full admin JWT auth will be implemented in a follow-up.
		secret := r.Header.Get("X-Admin-Secret")
		if subtle.ConstantTimeCompare([]byte(secret), []byte(h.adminSecret)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		// Look up admin user by email header if provided, otherwise use a default admin context.
		email := r.Header.Get("X-Admin-Email")
		if email != "" {
			user, err := h.users.ByEmail(r.Context(), email)
			if err != nil {
				if errors.Is(err, repo.ErrNotFound) {
					writeError(w, http.StatusUnauthorized, "unauthorized", "admin user not found")
					return
				}
				h.log.Error("fetch admin user", zap.Error(err))
				writeError(w, http.StatusInternalServerError, "internal", "internal error")
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Default admin context for X-Admin-Secret-only auth.
		ctx := context.WithValue(r.Context(), ctxKey{}, domain.AdminUser{
			ID:   uuid.Nil,
			Role: domain.RoleSuperAdmin,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── helpers ─────────────────────────────────────────────────────────────────

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
