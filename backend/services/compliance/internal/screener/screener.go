// Package screener provides the sanctions-screening abstraction.
//
// # Architecture
//
// Screener is the interface. LocalScreener is the POC implementation that checks
// a bundled JSON sanctions list. Future implementations (OFAC API, EU consolidated
// list) satisfy the same interface and can be swapped via config without changing
// the service logic.
package screener

import (
	"context"
	_ "embed"
	"encoding/json"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// Interface
// ─────────────────────────────────────────────────────────────────────────────

// Request carries the attributes to screen.
type Request struct {
	// Name is the full name to check (first + last, or entity name).
	Name string
	// Country is the ISO-3166-1 alpha-2 country code associated with the user.
	Country string
}

// Result is the outcome of a single screening call.
type Result struct {
	// Allowed is true when no sanctions hit was found.
	Allowed bool
	// MatchType is "none", "exact", or "fuzzy".
	MatchType string
	// MatchedName is the list entry that caused the hit, or "".
	MatchedName string
	// Provider is the provider identifier, e.g. "local".
	Provider string
}

// Screener is the sanctions-check abstraction.
type Screener interface {
	Screen(ctx context.Context, req Request) (Result, error)
	ProviderName() string
}

// ─────────────────────────────────────────────────────────────────────────────
// Local JSON implementation
// ─────────────────────────────────────────────────────────────────────────────

//go:embed data/sanctions.json
var defaultSanctionsJSON []byte

type sanctionsEntry struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	Country string   `json:"country"`
	Reason  string   `json:"reason"`
}

type sanctionsList struct {
	Entries []sanctionsEntry `json:"entries"`
}

// LocalScreener checks against an in-memory JSON list loaded at startup.
// Thread-safe (read-only after construction).
type LocalScreener struct {
	entries []sanctionsEntry
}

// NewLocalScreener loads the embedded default list.
func NewLocalScreener() (*LocalScreener, error) {
	return NewLocalScreenerFromJSON(defaultSanctionsJSON)
}

// NewLocalScreenerFromJSON parses a custom JSON payload. Useful for tests.
func NewLocalScreenerFromJSON(data []byte) (*LocalScreener, error) {
	var list sanctionsList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return &LocalScreener{entries: list.Entries}, nil
}

func (s *LocalScreener) ProviderName() string { return "local" }

func (s *LocalScreener) Screen(_ context.Context, req Request) (Result, error) {
	name := strings.ToLower(strings.TrimSpace(req.Name))

	for _, e := range s.entries {
		// Exact match on canonical name
		if strings.EqualFold(e.Name, req.Name) {
			return Result{
				Allowed:     false,
				MatchType:   "exact",
				MatchedName: e.Name,
				Provider:    "local",
			}, nil
		}
		// Exact match on any alias
		for _, alias := range e.Aliases {
			if strings.EqualFold(alias, req.Name) {
				return Result{
					Allowed:     false,
					MatchType:   "exact",
					MatchedName: e.Name,
					Provider:    "local",
				}, nil
			}
		}
		// Fuzzy: check if the name contains a significant portion of a list entry
		if name != "" && strings.Contains(strings.ToLower(e.Name), name) {
			return Result{
				Allowed:     false,
				MatchType:   "fuzzy",
				MatchedName: e.Name,
				Provider:    "local",
			}, nil
		}
		for _, alias := range e.Aliases {
			if name != "" && strings.Contains(strings.ToLower(alias), name) {
				return Result{
					Allowed:     false,
					MatchType:   "fuzzy",
					MatchedName: e.Name,
					Provider:    "local",
				}, nil
			}
		}
	}

	return Result{
		Allowed:   true,
		MatchType: "none",
		Provider:  "local",
	}, nil
}
