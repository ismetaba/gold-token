// Package config loads runtime configuration for complianced.
package config

import (
	"fmt"
	"os"

	"github.com/ismetaba/gold-token/backend/pkg/secrets"
)

// Config holds runtime configuration for complianced.
type Config struct {
	Env      string // local | dev | staging | prod
	HTTPAddr string

	DatabaseURL string
	NATSURL     string
	NATSStream  string

	// AdminSecret gates the screen/status and monitoring/rules endpoints via the
	// X-Admin-Secret header. Required (non-empty) in every non-local environment.
	AdminSecret string

	// SanctionsListFile is an optional path to a custom JSON sanctions list.
	// When empty, the embedded default list is used.
	SanctionsListFile string

	// PEPListFile is an optional path to a custom JSON PEP list.
	// When empty, the embedded default list is used.
	PEPListFile string

	// MonitoringIntervalSeconds controls how often the monitoring worker polls.
	// Default: 3600 (1 hour).
	MonitoringIntervalSeconds int

	// MonitoringBatchSize is max users re-screened per monitoring tick.
	// Default: 100.
	MonitoringBatchSize int

	// MonitoringFrequencyDays is the default re-screening interval for new
	// enrollments. Default: 30.
	MonitoringFrequencyDays int
}

func FromEnv() (*Config, error) {
	c := &Config{
		Env:                       getenv("GOLD_ENV", "local"),
		HTTPAddr:                  getenv("GOLD_COMPLIANCE_HTTP_ADDR", ":8086"),
		NATSStream:                getenv("GOLD_NATS_STREAM", "GOLD"),
		SanctionsListFile:         os.Getenv("SANCTIONS_LIST_FILE"),
		PEPListFile:               os.Getenv("PEP_LIST_FILE"),
		MonitoringIntervalSeconds: getenvInt("MONITORING_INTERVAL_SECONDS", 3600),
		MonitoringBatchSize:       getenvInt("MONITORING_BATCH_SIZE", 100),
		MonitoringFrequencyDays:   getenvInt("MONITORING_FREQUENCY_DAYS", 30),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")
	c.AdminSecret = os.Getenv("COMPLIANCE_ADMIN_SECRET")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":            c.DatabaseURL,
			"NATS_URL":                c.NATSURL,
			"COMPLIANCE_ADMIN_SECRET": c.AdminSecret,
		} {
			if v == "" {
				return nil, fmt.Errorf("missing required env: %s", k)
			}
		}
	} else if c.AdminSecret == "" {
		// Generate a random admin secret for local dev so it is never a static default.
		s, err := secrets.RandomHex(32)
		if err != nil {
			return nil, err
		}
		c.AdminSecret = s
	}
	return c, nil
}

func getenvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n := 0
	_, err := fmt.Sscanf(v, "%d", &n)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
