// Package http provides the Admin API Gateway HTTP handlers.
package http

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
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

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/admin/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/admin/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/admin/internal/tokens"
)

type ctxKey struct{}

// Handlers holds all handler dependencies.
type Handlers struct {
	users        repo.AdminUserRepo
	apiKeys      repo.APIKeyRepo
	tokens       *tokens.Manager
	masterSecret string // used only for POST /admin/bootstrap
	log          *zap.Logger

	// Upstream proxies keyed by route prefix (e.g. "/admin/kyc").
	proxies map[string]http.Handler
}

// NewHandlers constructs the handler set.
func NewHandlers(
	users repo.AdminUserRepo,
	apiKeys repo.APIKeyRepo,
	tm *tokens.Manager,
	masterSecret string,
	proxies map[string]http.Handler,
	log *zap.Logger,
) *Handlers {
	return &Handlers{
		users:        users,
		apiKeys:      apiKeys,
		tokens:       tm,
		masterSecret: masterSecret,
		proxies:      proxies,
		log:          log,
	}
}

// Routes wires all HTTP routes and returns the root router.
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

	r.Route("/admin", func(r chi.Router) {
		// Public: login returns a signed admin JWT.
		r.Post("/login", h.login)

		// Bootstrap: create the very first admin user (master secret required).
		r.With(h.requireMasterSecret).Post("/bootstrap", h.bootstrap)

		// Authenticated routes — require a valid admin JWT.
		r.Group(func(r chi.Router) {
			r.Use(h.requireAdminJWT)

			// Identity.
			r.Get("/me", h.me)

			// API key management (super_admin only).
			r.With(requireRole(domain.RoleSuperAdmin)).Post("/api-keys", h.createAPIKey)
			r.With(requireRole(domain.RoleSuperAdmin)).Get("/api-keys", h.listAPIKeys)

			// Proxied service routes — all protected by RBAC middleware.
			r.Group(func(r chi.Router) {
				r.Use(rbacMiddleware)

				for prefix, proxy := range h.proxies {
					p := proxy // capture
					r.Handle(prefix+"/*", http.StripPrefix("", p))
					r.Handle(prefix, p)
				}
			})
		})
	})

	return r
}

// ── handlers ─────────────────────────────────────────────────────────────────

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /admin/login — exchange credentials for admin JWT.
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
		// Constant-time: always run bcrypt even on miss.
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

	tokenStr, err := h.tokens.Issue(user.ID, user.Email, string(user.Role))
	if err != nil {
		h.log.Error("issue admin jwt", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal", "failed to issue token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": tokenStr,
		"user": map[string]any{
			"id":    user.ID.String(),
			"email": user.Email,
			"role":  string(user.Role),
		},
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
	claims := claimsFromCtx(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    claims.UserID.String(),
		"email": claims.Email,
		"role":  claims.Role,
	})
}

// POST /admin/api-keys — generate a new API key (super_admin only).
func (h *Handlers) createAPIKey(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())

	var req struct {
		Name      string   `json:"name"`
		Scopes    []string `json:"scopes"`
		ExpiresAt *string  `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "missing_fields", "name required")
		return
	}

	// Generate a cryptographically random 32-byte key.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		h.log.Error("generate api key bytes", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal", "failed to generate key")
		return
	}
	plainKey := "ak_" + hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plainKey))
	keyHash := hex.EncodeToString(sum[:])

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_expires_at", "expires_at must be RFC3339")
			return
		}
		expiresAt = &t
	}

	if req.Scopes == nil {
		req.Scopes = []string{}
	}

	apiKey := domain.APIKey{
		ID:          uuid.Must(uuid.NewV7()),
		AdminUserID: claims.UserID,
		KeyHash:     keyHash,
		Name:        req.Name,
		Scopes:      req.Scopes,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now().UTC(),
	}

	if h.apiKeys != nil {
		if err := h.apiKeys.Create(r.Context(), apiKey); err != nil {
			h.log.Error("create api key", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal", "failed to create api key")
			return
		}
	}

	// Return the plaintext key once — it cannot be recovered after this response.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         apiKey.ID.String(),
		"name":       apiKey.Name,
		"key":        plainKey,
		"scopes":     apiKey.Scopes,
		"expires_at": apiKey.ExpiresAt,
		"created_at": apiKey.CreatedAt,
	})
}

// GET /admin/api-keys — list API keys for the calling admin user (super_admin only).
func (h *Handlers) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())

	if h.apiKeys == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	keys, err := h.apiKeys.ListByUser(r.Context(), claims.UserID)
	if err != nil {
		h.log.Error("list api keys", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal", "failed to list api keys")
		return
	}

	out := make([]map[string]any, len(keys))
	for i, k := range keys {
		out[i] = map[string]any{
			"id":           k.ID.String(),
			"name":         k.Name,
			"scopes":       k.Scopes,
			"last_used_at": k.LastUsedAt,
			"expires_at":   k.ExpiresAt,
			"created_at":   k.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// ── middleware ────────────────────────────────────────────────────────────────

func (h *Handlers) requireMasterSecret(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-Admin-Secret")
		if subtle.ConstantTimeCompare([]byte(secret), []byte(h.masterSecret)) != 1 {
			writeError(w, http.StatusForbidden, "forbidden", "valid X-Admin-Secret required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handlers) requireAdminJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Bearer token required")
			return
		}
		tokenStr := strings.TrimPrefix(hdr, "Bearer ")

		claims, err := h.tokens.Verify(tokenStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}

		// Verify user still exists and is active.
		if h.users != nil {
			user, err := h.users.ByID(r.Context(), claims.UserID)
			if err != nil {
				if errors.Is(err, repo.ErrNotFound) {
					writeError(w, http.StatusUnauthorized, "unauthorized", "admin user not found")
					return
				}
				h.log.Error("fetch admin user", zap.Error(err))
				writeError(w, http.StatusInternalServerError, "internal", "internal error")
				return
			}
			if !user.Active {
				writeError(w, http.StatusUnauthorized, "account_disabled", "account is disabled")
				return
			}
			// Use the live role from DB (handles role changes without token re-issue).
			claims.Role = string(user.Role)
		}

		ctx := context.WithValue(r.Context(), ctxKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── context helpers ──────────────────────────────────────────────────────────

func claimsFromCtx(ctx context.Context) tokens.AdminClaims {
	v, _ := ctx.Value(ctxKey{}).(tokens.AdminClaims)
	return v
}

// ── response helpers ─────────────────────────────────────────────────────────

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
