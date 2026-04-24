package screener

import (
	"context"

	"go.uber.org/zap"
)

// AdverseMediaResult is the outcome of an adverse media screening call.
type AdverseMediaResult struct {
	// Skipped is true when the screening was not performed (stub mode).
	Skipped bool
	// Matched is true when adverse media was found.
	Matched bool
	// Summary is a human-readable description of the adverse media hit.
	Summary string
	// Provider is the provider identifier, e.g. "adverse_media_stub".
	Provider string
}

// AdverseMediaScreener is the adverse media check abstraction.
// Implementations may call external news/risk-data APIs.
type AdverseMediaScreener interface {
	ScreenAdverseMedia(ctx context.Context, req Request) (AdverseMediaResult, error)
	AdverseMediaProviderName() string
}

// ─────────────────────────────────────────────────────────────────────────────
// Stub implementation
// ─────────────────────────────────────────────────────────────────────────────

// AdverseMediaStub is a no-op implementation that logs a skip notice.
// Replace with a real implementation (e.g. Dow Jones, LexisNexis) when the
// external API contract is finalised.
type AdverseMediaStub struct {
	log *zap.Logger
}

func NewAdverseMediaStub(log *zap.Logger) *AdverseMediaStub {
	return &AdverseMediaStub{log: log}
}

func (s *AdverseMediaStub) AdverseMediaProviderName() string { return "adverse_media_stub" }

func (s *AdverseMediaStub) ScreenAdverseMedia(_ context.Context, req Request) (AdverseMediaResult, error) {
	s.log.Debug("adverse media screening skipped (stub mode)",
		zap.String("name", req.Name),
		zap.String("country", req.Country),
	)
	return AdverseMediaResult{
		Skipped:  true,
		Provider: "adverse_media_stub",
	}, nil
}
