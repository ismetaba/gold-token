package generator

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"

	"github.com/ismetaba/gold-token/backend/services/reporting/internal/repo"
)

// ComplianceCSV generates a CSV compliance summary report.
func ComplianceCSV(ctx context.Context, queries repo.QueryRepo) ([]byte, error) {
	s, err := queries.ComplianceSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("compliance query: %w", err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	_ = w.Write([]string{"metric", "value"})
	_ = w.Write([]string{"total_screenings", fmt.Sprintf("%d", s.TotalScreenings)})
	_ = w.Write([]string{"approved_screenings", fmt.Sprintf("%d", s.ApprovedCount)})
	_ = w.Write([]string{"rejected_screenings", fmt.Sprintf("%d", s.RejectedCount)})
	_ = w.Write([]string{"pending_kyc", fmt.Sprintf("%d", s.PendingKYC)})
	_ = w.Write([]string{"approved_kyc", fmt.Sprintf("%d", s.ApprovedKYC)})
	_ = w.Write([]string{"rejected_kyc", fmt.Sprintf("%d", s.RejectedKYC)})

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv flush: %w", err)
	}
	return buf.Bytes(), nil
}
