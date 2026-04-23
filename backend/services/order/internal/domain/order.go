// Package domain holds the core order types and state machine.
package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OrderType distinguishes buy (mint) from sell (burn/redeem).
type OrderType string

const (
	OrderBuy  OrderType = "buy"
	OrderSell OrderType = "sell"
)

// OrderStatus is the state-machine status of an order.
//
//	created → confirmed → processing → completed
//	                    ↘             ↘
//	                     failed        failed
type OrderStatus string

const (
	OrderCreated    OrderStatus = "created"
	OrderConfirmed  OrderStatus = "confirmed"
	OrderProcessing OrderStatus = "processing"
	OrderCompleted  OrderStatus = "completed"
	OrderFailed     OrderStatus = "failed"
)

// IsTerminal returns true when no further transitions are possible.
func (s OrderStatus) IsTerminal() bool {
	return s == OrderCompleted || s == OrderFailed
}

// AllowedTransitions defines the valid state-machine edges.
var AllowedTransitions = map[OrderStatus][]OrderStatus{
	OrderCreated:    {OrderConfirmed, OrderFailed},
	OrderConfirmed:  {OrderProcessing, OrderFailed},
	OrderProcessing: {OrderCompleted, OrderFailed},
}

// Transition validates and returns nil if the transition from → to is allowed.
func Transition(from, to OrderStatus) error {
	for _, allowed := range AllowedTransitions[from] {
		if allowed == to {
			return nil
		}
	}
	return fmt.Errorf("invalid order status transition: %s → %s", from, to)
}

// Order is the aggregate root for a buy or sell request.
type Order struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Type           OrderType
	Status         OrderStatus
	AmountGrams    string     // decimal string, e.g. "1.500000000000000000"
	AmountWei      string     // grams * 1e18 as decimal string
	UserAddress    string     // 0x-prefixed Ethereum address (from wallet service)
	Arena          string     // ISO-3166 jurisdiction code, e.g. "TR"
	AllocationID   *uuid.UUID // generated on confirm; passed to mint saga
	IdempotencyKey string
	FailureReason  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ConfirmedAt    *time.Time
	CompletedAt    *time.Time
}

// ErrInvalidTransition is returned when a state-machine edge is violated.
var ErrInvalidTransition = errors.New("invalid order status transition")
