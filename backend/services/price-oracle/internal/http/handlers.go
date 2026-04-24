// Package http provides the market data service's HTTP API.
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/oracle"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/repo"
)

var wsUpgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second,
	ReadBufferSize:   256,
	WriteBufferSize:  1024,
	// Allow all origins; callers should restrict at the gateway/proxy layer.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handlers wires together the oracle, repo, and HTTP endpoints.
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
		r.Get("/candles", h.candles)
		r.Get("/ws", h.ws)
	})

	return r
}

// ── handlers ─────────────────────────────────────────────────────────────────

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type currentResponse struct {
	Pair         string    `json:"pair"`
	PricePerGram float64   `json:"price_per_gram"`
	Provider     string    `json:"provider"`
	FetchedAt    time.Time `json:"fetched_at"`
}

// GET /price/current?pair=XAU/USD
// Returns the latest cached price for the requested pair (default: XAU/USD).
func (h *Handlers) current(w http.ResponseWriter, r *http.Request) {
	pair := r.URL.Query().Get("pair")
	if pair == "" {
		pair = "XAU/USD"
	}
	if !isSupportedPair(pair) {
		writeErr(w, http.StatusBadRequest, "unsupported_pair",
			"pair must be one of: XAU/USD, XAU/TRY, XAU/EUR, XAU/CHF")
		return
	}

	p := h.oracle.Current(pair)
	if p == nil {
		writeErr(w, http.StatusServiceUnavailable, "no_price_yet",
			"price not yet available; first fetch in progress")
		return
	}
	writeJSON(w, http.StatusOK, currentResponse{
		Pair:         p.Pair,
		PricePerGram: p.PricePerGram,
		Provider:     p.Provider,
		FetchedAt:    p.FetchedAt,
	})
}

type historyResponse struct {
	Pair   string         `json:"pair"`
	Window string         `json:"window"`
	Count  int            `json:"count"`
	Prices []domain.Price `json:"prices"`
}

// GET /price/history?pair=XAU/USD&window=24h
func (h *Handlers) history(w http.ResponseWriter, r *http.Request) {
	pair := r.URL.Query().Get("pair")
	if pair == "" {
		pair = "XAU/USD"
	}
	if !isSupportedPair(pair) {
		writeErr(w, http.StatusBadRequest, "unsupported_pair",
			"pair must be one of: XAU/USD, XAU/TRY, XAU/EUR, XAU/CHF")
		return
	}

	windowStr := r.URL.Query().Get("window")
	if windowStr == "" {
		windowStr = "24h"
	}
	window, err := time.ParseDuration(windowStr)
	if err != nil || window <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid_window",
			"window must be a positive duration (e.g. 1h, 24h, 168h)")
		return
	}
	const maxWindow = 720 * time.Hour
	if window > maxWindow {
		window = maxWindow
	}
	since := time.Now().UTC().Add(-window)

	if h.repo == nil {
		var prices []domain.Price
		if p := h.oracle.Current(pair); p != nil && p.FetchedAt.After(since) {
			prices = []domain.Price{*p}
		}
		writeJSON(w, http.StatusOK, historyResponse{
			Pair: pair, Window: window.String(), Count: len(prices), Prices: prices,
		})
		return
	}

	prices, err := h.repo.History(r.Context(), pair, since)
	if err != nil {
		h.log.Error("history query failed", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not retrieve price history")
		return
	}
	if prices == nil {
		prices = []domain.Price{}
	}
	writeJSON(w, http.StatusOK, historyResponse{
		Pair: pair, Window: window.String(), Count: len(prices), Prices: prices,
	})
}

type candlesResponse struct {
	Pair     string          `json:"pair"`
	Interval string          `json:"interval"`
	Count    int             `json:"count"`
	Candles  []domain.Candle `json:"candles"`
}

// GET /price/candles?pair=XAU/USD&interval=1h&from=2024-01-01T00:00:00Z&to=2024-01-02T00:00:00Z
func (h *Handlers) candles(w http.ResponseWriter, r *http.Request) {
	pair := r.URL.Query().Get("pair")
	if pair == "" {
		pair = "XAU/USD"
	}
	if !isSupportedPair(pair) {
		writeErr(w, http.StatusBadRequest, "unsupported_pair",
			"pair must be one of: XAU/USD, XAU/TRY, XAU/EUR, XAU/CHF")
		return
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1h"
	}
	if interval != "1h" && interval != "4h" && interval != "1d" {
		writeErr(w, http.StatusBadRequest, "invalid_interval", "interval must be one of: 1h, 4h, 1d")
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour) // default: last 24h
	to := now

	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t.UTC()
		} else {
			writeErr(w, http.StatusBadRequest, "invalid_from", "from must be RFC3339 (e.g. 2024-01-01T00:00:00Z)")
			return
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t.UTC()
		} else {
			writeErr(w, http.StatusBadRequest, "invalid_to", "to must be RFC3339 (e.g. 2024-01-02T00:00:00Z)")
			return
		}
	}
	if !to.After(from) {
		writeErr(w, http.StatusBadRequest, "invalid_range", "to must be after from")
		return
	}

	if h.repo == nil {
		writeJSON(w, http.StatusOK, candlesResponse{
			Pair: pair, Interval: interval, Count: 0, Candles: []domain.Candle{},
		})
		return
	}

	candles, err := h.repo.GetCandles(r.Context(), pair, interval, from, to)
	if err != nil {
		h.log.Error("candles query failed", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal_error", "could not retrieve candles")
		return
	}
	if candles == nil {
		candles = []domain.Candle{}
	}
	writeJSON(w, http.StatusOK, candlesResponse{
		Pair: pair, Interval: interval, Count: len(candles), Candles: candles,
	})
}

// GET /price/ws — upgrades to WebSocket; streams PriceUpdate JSON messages
// for all pairs as they arrive from the oracle's fetch loop.
func (h *Handlers) ws(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("websocket upgrade failed", zap.Error(err))
		return
	}

	ch, unsub := h.oracle.Hub().Subscribe()
	defer func() {
		unsub()
		conn.Close()
	}()

	// Write pump: forward messages from hub to the WebSocket connection.
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				// Hub closed the channel (shutdown).
				conn.WriteMessage(websocket.CloseMessage, //nolint:errcheck
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutdown"))
				return
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

var supportedPairSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(domain.SupportedPairs))
	for _, p := range domain.SupportedPairs {
		m[p] = struct{}{}
	}
	return m
}()

func isSupportedPair(pair string) bool {
	_, ok := supportedPairSet[pair]
	return ok
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
