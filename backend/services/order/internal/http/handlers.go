// Package http provides the order service HTTP API.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/order/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/order/internal/repo"
)

// weiPerGram = 1e18 (1 gram of GOLD = 1 GOLD token = 1e18 wei)
var weiPerGram = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// ctxKey for the authenticated user ID.
type ctxKey struct{}

// Handlers wires together the order repo and event bus.
type Handlers struct {
	orders   repo.OrderRepo
	bus      *pkgevents.Bus
	stream   string
	jwtPub   interface{} // *rsa.PublicKey or nil (local dev — skip auth)
	localDev bool
	log      *zap.Logger
}

// NewHandlers constructs the handler set.
func NewHandlers(
	orders repo.OrderRepo,
	bus *pkgevents.Bus,
	stream string,
	jwtPublicKeyFile string,
	log *zap.Logger,
) (*Handlers, error) {
	h := &Handlers{orders: orders, bus: bus, stream: stream, log: log}

	if jwtPublicKeyFile == "" {
		h.localDev = true
		log.Warn("order: JWT auth disabled — local dev mode")
		return h, nil
	}

	pem, err := os.ReadFile(jwtPublicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read JWT public key: %w", err)
	}
	pub, err := jwt.ParseRSAPublicKeyFromPEM(pem)
	if err != nil {
		return nil, fmt.Errorf("parse JWT public key: %w", err)
	}
	h.jwtPub = pub
	return h, nil
}

func (h *Handlers) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.health)

	r.Route("/orders", func(r chi.Router) {
		r.Use(h.requireAuth)
		r.Post("/", h.createOrder)
		r.Get("/", h.listOrders)
		r.Get("/{id}", h.getOrder)
	})

	return r
}

// ─────────────────────────────────────────────────────────────────────────────
// Middleware
// ─────────────────────────────────────────────────────────────────────────────

func (h *Handlers) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.localDev {
			rawID := r.Header.Get("X-Dev-User-Id")
			if rawID == "" {
				rawID = "00000000-0000-0000-0000-000000000001"
			}
			id, err := uuid.Parse(rawID)
			if err != nil {
				writeErr(w, http.StatusBadRequest, "invalid_user_id", "X-Dev-User-Id must be a valid UUID")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
			return
		}

		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "missing_token", "Authorization header required")
			return
		}
		tokenStr := strings.TrimPrefix(hdr, "Bearer ")

		t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return h.jwtPub, nil
		}, jwt.WithIssuer("gold-auth"), jwt.WithExpirationRequired())
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid_token", "access token is invalid or expired")
			return
		}

		claims, ok := t.Claims.(jwt.MapClaims)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "invalid_token", "bad claims")
			return
		}
		tt, _ := claims["token_type"].(string)
		if tt != "access" {
			writeErr(w, http.StatusUnauthorized, "invalid_token", "not an access token")
			return
		}
		subStr, _ := claims["sub"].(string)
		userID, err := uuid.Parse(subStr)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid_token", "invalid sub claim")
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, userID)))
	})
}

func userIDFrom(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(ctxKey{}).(uuid.UUID)
	return v
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /orders
//
// Body:
//
//	{
//	  "type":         "buy" | "sell",
//	  "amount_grams": "1.5",          // decimal string
//	  "user_address": "0x...",        // destination (buy) or source (sell) wallet
//	  "arena":        "TR"            // optional; defaults to "TR"
//	}
//
// Header: X-Idempotency-Key: <uuid> (required)
func (h *Handlers) createOrder(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())

	// Idempotency key is required.
	iKey := r.Header.Get("X-Idempotency-Key")
	if iKey == "" {
		writeErr(w, http.StatusBadRequest, "missing_idempotency_key", "X-Idempotency-Key header required")
		return
	}

	// Check for existing order with this idempotency key.
	existing, err := h.orders.ByIdempotencyKey(r.Context(), userID, iKey)
	if err == nil {
		writeJSON(w, http.StatusOK, orderResponse(existing))
		return
	}
	if !errors.Is(err, repo.ErrNotFound) {
		h.log.Error("idempotency lookup", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not check idempotency key")
		return
	}

	var body struct {
		Type        string `json:"type"`
		AmountGrams string `json:"amount_grams"`
		UserAddress string `json:"user_address"`
		Arena       string `json:"arena"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}

	if body.Type != string(domain.OrderBuy) && body.Type != string(domain.OrderSell) {
		writeErr(w, http.StatusBadRequest, "invalid_type", "type must be 'buy' or 'sell'")
		return
	}
	if body.AmountGrams == "" {
		writeErr(w, http.StatusBadRequest, "missing_amount", "amount_grams is required")
		return
	}
	if body.UserAddress == "" {
		writeErr(w, http.StatusBadRequest, "missing_address", "user_address is required")
		return
	}
	arena := body.Arena
	if arena == "" {
		arena = "TR"
	}

	wei, err := gramsToWei(body.AmountGrams)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_amount", "amount_grams must be a positive decimal number")
		return
	}

	now := time.Now().UTC()
	order := domain.Order{
		ID:             uuid.Must(uuid.NewV7()),
		UserID:         userID,
		Type:           domain.OrderType(body.Type),
		Status:         domain.OrderCreated,
		AmountGrams:    body.AmountGrams,
		AmountWei:      wei.String(),
		UserAddress:    body.UserAddress,
		Arena:          arena,
		IdempotencyKey: iKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.orders.Create(r.Context(), order); err != nil {
		if errors.Is(err, repo.ErrAlreadyExists) {
			// Race: another request created it; return that order.
			existing, _ := h.orders.ByIdempotencyKey(r.Context(), userID, iKey)
			writeJSON(w, http.StatusOK, orderResponse(existing))
			return
		}
		h.log.Error("create order", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not create order")
		return
	}

	// Publish order.created event.
	if h.bus != nil {
		_ = pkgevents.Publish(r.Context(), h.bus, pkgevents.Envelope[orderCreatedPayload]{
			EventType:   "gold.order.created.v1",
			AggregateID: order.ID.String(),
			Data: orderCreatedPayload{
				OrderID:     order.ID.String(),
				UserID:      userID.String(),
				Type:        string(order.Type),
				AmountWei:   order.AmountWei,
				UserAddress: order.UserAddress,
				Arena:       order.Arena,
			},
		})
	}

	// Fiat payment is simulated for POC — auto-confirm immediately.
	if err := h.confirmOrder(r.Context(), &order); err != nil {
		h.log.Error("auto-confirm order", zap.String("order_id", order.ID.String()), zap.Error(err))
		// Return the order as-is; it will be confirmed on next retry.
	}

	writeJSON(w, http.StatusCreated, orderResponse(order))
}

// GET /orders?page=1&limit=20
func (h *Handlers) listOrders(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())

	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	orders, err := h.orders.ListByUserID(r.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("list orders", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not fetch orders")
		return
	}

	out := make([]map[string]interface{}, 0, len(orders))
	for _, o := range orders {
		out = append(out, orderResponse(o))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"page":   page,
		"limit":  limit,
		"orders": out,
	})
}

// GET /orders/:id
func (h *Handlers) getOrder(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())

	rawID := chi.URLParam(r, "id")
	id, err := uuid.Parse(rawID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_id", "order id must be a valid UUID")
		return
	}

	o, err := h.orders.ByID(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	if err != nil {
		h.log.Error("get order", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "internal", "could not fetch order")
		return
	}
	// Users may only read their own orders.
	if o.UserID != userID {
		writeErr(w, http.StatusNotFound, "not_found", "order not found")
		return
	}

	writeJSON(w, http.StatusOK, orderResponse(o))
}

// ─────────────────────────────────────────────────────────────────────────────
// confirmOrder — simulated fiat payment auto-confirm
// ─────────────────────────────────────────────────────────────────────────────

type orderCreatedPayload struct {
	OrderID     string `json:"order_id"`
	UserID      string `json:"user_id"`
	Type        string `json:"type"`
	AmountWei   string `json:"amount_wei"`
	UserAddress string `json:"user_address"`
	Arena       string `json:"arena"`
}

type orderConfirmedPayload struct {
	OrderID      string `json:"order_id"`
	Type         string `json:"type"`
	AllocationID string `json:"allocation_id"`
	AmountWei    string `json:"amount_wei"`
	UserAddress  string `json:"user_address"`
	Arena        string `json:"arena"`
}

// OrderReadyToMintPayload matches what mint-burn service expects.
type orderReadyToMintPayload struct {
	OrderID      string `json:"order_id"`
	AllocationID string `json:"allocation_id"`
	UserAddress  string `json:"user_address"`
	AmountWei    string `json:"amount_wei"`
	Arena        string `json:"arena"`
}

// BurnRequestedPayload for sell orders dispatched to mint-burn.
type burnRequestedPayload struct {
	OrderID        string `json:"order_id"`
	UserAddress    string `json:"user_address"`
	AmountWei      string `json:"amount_wei"`
	RedemptionType int    `json:"redemption_type"` // 0=cash
	Arena          string `json:"arena"`
}

func (h *Handlers) confirmOrder(ctx context.Context, order *domain.Order) error {
	now := time.Now().UTC()
	allocID := uuid.Must(uuid.NewV7())

	order.Status = domain.OrderConfirmed
	order.AllocationID = &allocID
	order.ConfirmedAt = &now

	if err := h.orders.Update(ctx, *order); err != nil {
		return fmt.Errorf("confirm order: %w", err)
	}

	if h.bus == nil {
		return nil
	}

	// Publish order.confirmed
	_ = pkgevents.Publish(ctx, h.bus, pkgevents.Envelope[orderConfirmedPayload]{
		EventType:   "gold.order.confirmed.v1",
		AggregateID: order.ID.String(),
		Data: orderConfirmedPayload{
			OrderID:      order.ID.String(),
			Type:         string(order.Type),
			AllocationID: allocID.String(),
			AmountWei:    order.AmountWei,
			UserAddress:  order.UserAddress,
			Arena:        order.Arena,
		},
	})

	// Dispatch to mint-burn saga based on order type.
	switch order.Type {
	case domain.OrderBuy:
		return pkgevents.Publish(ctx, h.bus, pkgevents.Envelope[orderReadyToMintPayload]{
			EventType:     pkgevents.SubjOrderReadyToMint,
			AggregateID:   order.ID.String(),
			CorrelationID: order.ID.String(),
			Data: orderReadyToMintPayload{
				OrderID:      order.ID.String(),
				AllocationID: allocID.String(),
				UserAddress:  order.UserAddress,
				AmountWei:    order.AmountWei,
				Arena:        order.Arena,
			},
		})
	case domain.OrderSell:
		return pkgevents.Publish(ctx, h.bus, pkgevents.Envelope[burnRequestedPayload]{
			EventType:     pkgevents.SubjBurnRequested,
			AggregateID:   order.ID.String(),
			CorrelationID: order.ID.String(),
			Data: burnRequestedPayload{
				OrderID:        order.ID.String(),
				UserAddress:    order.UserAddress,
				AmountWei:      order.AmountWei,
				RedemptionType: 0, // cash-back for POC
				Arena:          order.Arena,
			},
		})
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Response helpers
// ─────────────────────────────────────────────────────────────────────────────

func orderResponse(o domain.Order) map[string]interface{} {
	m := map[string]interface{}{
		"id":              o.ID.String(),
		"user_id":         o.UserID.String(),
		"type":            string(o.Type),
		"status":          string(o.Status),
		"amount_grams":    o.AmountGrams,
		"amount_wei":      o.AmountWei,
		"user_address":    o.UserAddress,
		"arena":           o.Arena,
		"idempotency_key": o.IdempotencyKey,
		"created_at":      o.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":      o.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if o.AllocationID != nil {
		m["allocation_id"] = o.AllocationID.String()
	}
	if o.ConfirmedAt != nil {
		m["confirmed_at"] = o.ConfirmedAt.UTC().Format(time.RFC3339)
	}
	if o.CompletedAt != nil {
		m["completed_at"] = o.CompletedAt.UTC().Format(time.RFC3339)
	}
	if o.FailureReason != "" {
		m["failure_reason"] = o.FailureReason
	}
	return m
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func gramsToWei(grams string) (*big.Int, error) {
	f, ok := new(big.Float).SetPrec(256).SetString(grams)
	if !ok {
		return nil, errors.New("invalid amount")
	}
	if f.Sign() <= 0 {
		return nil, errors.New("amount must be positive")
	}
	weiF := new(big.Float).SetPrec(256).Mul(f, new(big.Float).SetInt(weiPerGram))
	wei, _ := weiF.Int(nil)
	if wei.Sign() <= 0 {
		return nil, errors.New("amount rounds to zero wei")
	}
	return wei, nil
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]string{"error": errCode, "message": msg})
}

func queryInt(r *http.Request, key string, def int) int {
	if s := r.URL.Query().Get(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return def
}
