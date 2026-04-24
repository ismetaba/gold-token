package screener

import (
	"context"
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed data/pep.json
var defaultPEPJSON []byte

// PEPResult is the outcome of a PEP screening call.
type PEPResult struct {
	// Matched is true when a PEP list hit was found.
	Matched bool
	// MatchedName is the list entry that caused the hit, or "".
	MatchedName string
	// Position is the political position of the matched entity.
	Position string
	// Provider is the provider identifier, e.g. "local_pep".
	Provider string
}

// PEPScreener is the Politically Exposed Persons check abstraction.
type PEPScreener interface {
	ScreenPEP(ctx context.Context, req Request) (PEPResult, error)
	PEPProviderName() string
}

// ─────────────────────────────────────────────────────────────────────────────
// Local JSON implementation
// ─────────────────────────────────────────────────────────────────────────────

type pepEntry struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases"`
	Position string   `json:"position"`
	Country  string   `json:"country"`
	Reason   string   `json:"reason"`
}

type pepList struct {
	Entries []pepEntry `json:"entries"`
}

// LocalPEPScreener checks against an in-memory JSON PEP list loaded at startup.
// Thread-safe (read-only after construction).
type LocalPEPScreener struct {
	entries []pepEntry
}

// NewLocalPEPScreener loads the embedded default PEP list.
func NewLocalPEPScreener() (*LocalPEPScreener, error) {
	return NewLocalPEPScreenerFromJSON(defaultPEPJSON)
}

// NewLocalPEPScreenerFromJSON parses a custom JSON payload. Useful for tests.
func NewLocalPEPScreenerFromJSON(data []byte) (*LocalPEPScreener, error) {
	var list pepList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return &LocalPEPScreener{entries: list.Entries}, nil
}

func (s *LocalPEPScreener) PEPProviderName() string { return "local_pep" }

func (s *LocalPEPScreener) ScreenPEP(_ context.Context, req Request) (PEPResult, error) {
	name := strings.ToLower(strings.TrimSpace(req.Name))
	if name == "" {
		return PEPResult{Provider: "local_pep"}, nil
	}

	for _, e := range s.entries {
		if strings.EqualFold(e.Name, req.Name) {
			return PEPResult{
				Matched:     true,
				MatchedName: e.Name,
				Position:    e.Position,
				Provider:    "local_pep",
			}, nil
		}
		for _, alias := range e.Aliases {
			if strings.EqualFold(alias, req.Name) {
				return PEPResult{
					Matched:     true,
					MatchedName: e.Name,
					Position:    e.Position,
					Provider:    "local_pep",
				}, nil
			}
		}
		if strings.Contains(strings.ToLower(e.Name), name) {
			return PEPResult{
				Matched:     true,
				MatchedName: e.Name,
				Position:    e.Position,
				Provider:    "local_pep",
			}, nil
		}
	}

	return PEPResult{Provider: "local_pep"}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// External API stub
// ─────────────────────────────────────────────────────────────────────────────

// ExternalPEPScreener is a stub for a future external PEP screening API
// (e.g. Dow Jones Risk & Compliance, World-Check).
// It always returns not-matched and logs that the call was skipped.
type ExternalPEPScreener struct{}

func (s *ExternalPEPScreener) PEPProviderName() string { return "external_pep_stub" }

func (s *ExternalPEPScreener) ScreenPEP(_ context.Context, _ Request) (PEPResult, error) {
	// TODO: integrate with external PEP API endpoint
	return PEPResult{Provider: "external_pep_stub"}, nil
}
