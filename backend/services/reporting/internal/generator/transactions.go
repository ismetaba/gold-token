// Package generator builds CSV report files from query results.
package generator

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"github.com/ismetaba/gold-token/backend/services/reporting/internal/repo"
)

// TransactionCSV generates a CSV report of daily transaction summaries.
func TransactionCSV(ctx context.Context, queries repo.QueryRepo, from, to time.Time) ([]byte, error) {
	rows, err := queries.TransactionSummary(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("transaction query: %w", err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	_ = w.Write([]string{"date", "mint_count", "burn_count", "mint_volume_wei", "burn_volume_wei", "fee_volume_wei"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.Date,
			fmt.Sprintf("%d", r.MintCount),
			fmt.Sprintf("%d", r.BurnCount),
			r.MintVolumeWei,
			r.BurnVolumeWei,
			r.FeeVolumeWei,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv flush: %w", err)
	}
	return buf.Bytes(), nil
}
