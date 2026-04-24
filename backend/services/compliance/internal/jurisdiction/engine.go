// Package jurisdiction provides the jurisdiction-specific compliance rule engine.
//
// Rules are stored in compliance.jurisdiction_rules and evaluated during
// order screening. Each rule is matched by arena (ISO 3166-1 alpha-2 country
// code) and may impose additional due-diligence actions on the order.
package jurisdiction

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ismetaba/gold-token/backend/services/compliance/internal/domain"
)

// RuleRepo provides read/write access to jurisdiction rules.
type RuleRepo interface {
	ListActiveRules(ctx context.Context, arena string) ([]domain.JurisdictionRule, error)
	ListAllRules(ctx context.Context) ([]domain.JurisdictionRule, error)
	GetRule(ctx context.Context, id string) (domain.JurisdictionRule, error)
	UpdateRule(ctx context.Context, id string, active bool, action string) (domain.JurisdictionRule, error)
}

// OrderContext carries the attributes of the order being screened.
type OrderContext struct {
	Arena         string // ISO 3166-1 alpha-2
	AmountWei     string // order amount in grams-wei (NUMERIC(78,0) as string)
}

// Engine evaluates jurisdiction rules against an order.
type Engine struct {
	repo RuleRepo
}

func NewEngine(repo RuleRepo) *Engine { return &Engine{repo: repo} }

// Evaluate returns all rule violations triggered by the given order context.
// An empty slice means no violations.
func (e *Engine) Evaluate(ctx context.Context, oc OrderContext) ([]domain.RuleViolation, error) {
	rules, err := e.repo.ListActiveRules(ctx, oc.Arena)
	if err != nil {
		return nil, fmt.Errorf("jurisdiction engine: list rules: %w", err)
	}

	var violations []domain.RuleViolation
	for _, rule := range rules {
		if v, triggered := evaluate(rule, oc); triggered {
			violations = append(violations, v)
		}
	}
	return violations, nil
}

// evaluate tests a single rule against the order context.
func evaluate(rule domain.JurisdictionRule, oc OrderContext) (domain.RuleViolation, bool) {
	switch rule.RuleType {
	case "enhanced_due_diligence":
		// Triggered when order amount exceeds threshold.
		if rule.ThresholdGramsWei == nil {
			// No threshold — always triggered for this arena.
			return violation(rule, "EDD required for all orders in "+rule.Arena), true
		}
		threshold, ok := new(big.Int).SetString(*rule.ThresholdGramsWei, 10)
		if !ok {
			return domain.RuleViolation{}, false
		}
		amount, ok2 := new(big.Int).SetString(oc.AmountWei, 10)
		if !ok2 {
			return domain.RuleViolation{}, false
		}
		if amount.Cmp(threshold) >= 0 {
			return violation(rule, fmt.Sprintf("amount %s exceeds EDD threshold %s for arena %s",
				oc.AmountWei, *rule.ThresholdGramsWei, rule.Arena)), true
		}

	case "source_of_funds":
		// Always triggered — source-of-funds declaration required regardless of amount.
		return violation(rule, "source-of-funds declaration required for arena "+rule.Arena), true
	}

	return domain.RuleViolation{}, false
}

func violation(rule domain.JurisdictionRule, reason string) domain.RuleViolation {
	return domain.RuleViolation{Rule: rule, Reason: reason}
}
