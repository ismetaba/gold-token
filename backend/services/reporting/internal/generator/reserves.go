package generator

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"github.com/ismetaba/gold-token/backend/services/reporting/internal/repo"
)

// ReserveCSV generates a CSV report of daily reserve snapshots.
func ReserveCSV(ctx context.Context, queries repo.QueryRepo, from, to time.Time) ([]byte, error) {
	rows, err := queries.ReserveSummary(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("reserve query: %w", err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	_ = w.Write([]string{"date", "gold_balance_wei", "token_supply_wei", "attestation_count"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.Date,
			r.GoldBalanceWei,
			r.TokenSupplyWei,
			fmt.Sprintf("%d", r.AttestationCount),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv flush: %w", err)
	}
	return buf.Bytes(), nil
}
