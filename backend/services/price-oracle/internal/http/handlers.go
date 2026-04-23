// Package http provides the price oracle service's HTTP API.
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/oracle"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/repo"
)

// Handlers wires together the oracle and optional repo.
type Handlers struct {
	oracle *oracle.Oracle
	repo   repo.PriceRepo // may be nil in local mode
	log    *zap.Logger
}

// NewHandlers returns a Handlers instance.
func NewHandlers(o *oracle.Oracle, r repo.PriceRepo, log *zap.Logger) *Handlers {
	return &Handlers{oracle: o, repo: r, log: log}
}

// Routes returns the chi router with all endpoints registered.
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

	r.Route("/price", func(r chi.Router) {
		r.Get("/current", h.current)
		r.Get("/history", h.history)
	})

	return r
}

// ── handlers ─────────────────────────────────────────────────────────────────

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type currentResponse struct {
	PriceUSDPerGram float64   `json:"price_usd_per_gram"`
	Provider        string    `json:"provider"`
	FetchedAt       time.Time `json:"fetched_at"`
}

func (h *Handlers) current(w http.ResponseWriter, _ *http.Request) {
	p := h.oracle.Current()
	if p == nil {
		writeErr(w, http.StatusServiceUnavailable, "no_price_yet", "price not yet available; first fetch in progress")
		return
	}
	writeJSON(w, http.StatusOK, currentResponse{
		PriceUSDPerGram: p.PriceUSDg,
		Provider:        p.Provider,
		FetchedAt:       p.FetchedAt,
	})
}

type historyResponse struct {
	Window  string         `json:"window"`
	Count   int            `json:"count"`
	Prices  []domain.Price `json:"prices"`
}

func (h *Handlers) history(w http.ResponseWriter, r *http.Request) {
	// ?window=24h  (default 24h; max 720h / 30 days)
	windowStr := r.URL.Query().Get("window")
	if windowStr == "" {
		windowStr = "24h"
	}
	window, err := time.ParseDuration(windowStr)
	if err != nil || window <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid_window", "window must be a positive duration (e.g. 1h, 24h, 168h)")
		return
	}
	const maxWindow = 720 * time.Hour
	if window > maxWindow {
		window = maxWindow
	}

	since := time.Now().UTC().Add(-window)

	if h.repo == nil {
		// Local mode: return only in-memory snapshot.
		var prices []domain.Price
		if p := h.oracle.Current(); p != nil && p.FetchedAt.After(since) {
			prices = []domain.Price{*p}
		}
		writeJSON(w, http.StatusOK, historyResponse{
			Window: window.String(),
			Count:  len(prices),
			Prices: prices,
		})
		return
	}

	prices, err := h.repo.History(r.Context(), since)
	if err != nil {
		h.log.Error("history query failed", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not retrieve price history")
		return
	}
	if prices == nil {
		prices = []domain.Price{}
	}
	writeJSON(w, http.StatusOK, historyResponse{
		Window: window.String(),
		Count:  len(prices),
		Prices: prices,
	})
}

// ── helpers ──────────────────────────────────────────────────────────────────

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
