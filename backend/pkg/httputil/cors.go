package httputil

import (
	"net/http"
	"strings"
)

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	// AllowedOrigins is the list of allowed origins.
	// Use []string{"*"} only in local/dev mode.
	AllowedOrigins []string
	// AllowedMethods defaults to GET, POST, PUT, PATCH, DELETE, OPTIONS.
	AllowedMethods []string
	// AllowedHeaders defaults to Authorization, Content-Type, X-Idempotency-Key, X-Admin-Secret, X-Admin-Token.
	AllowedHeaders []string
	// MaxAge is the preflight cache duration in seconds. Defaults to 3600.
	MaxAge string
}

// DefaultCORSConfig returns a restrictive CORS config suitable for production.
// Override AllowedOrigins with your actual frontend domain(s).
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Idempotency-Key", "X-Admin-Secret", "X-Admin-Token", "X-Request-ID"},
		MaxAge:         "3600",
	}
}

// LocalCORSConfig returns a permissive CORS config for local development.
func LocalCORSConfig() CORSConfig {
	c := DefaultCORSConfig()
	c.AllowedOrigins = []string{"*"}
	return c
}

// CORSMiddleware returns CORS middleware using the given config.
func CORSMiddleware(cfg CORSConfig) func(http.Handler) http.Handler {
	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	maxAge := cfg.MaxAge
	if maxAge == "" {
		maxAge = "3600"
	}

	allowAll := len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*"
	allowed := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if allowed[origin] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				} else {
					// Origin not allowed — don't set CORS headers.
					if r.Method == http.MethodOptions {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					next.ServeHTTP(w, r)
					return
				}

				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				w.Header().Set("Access-Control-Max-Age", maxAge)
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
