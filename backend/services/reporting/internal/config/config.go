package config

import (
	"fmt"
	"os"
)

type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseURL string
	NATSURL     string
	NATSStream  string
	AdminSecret string
}

func FromEnv() (*Config, error) {
	c := &Config{
		Env:         getenv("GOLD_ENV", "local"),
		HTTPAddr:    getenv("GOLD_REPORTING_HTTP_ADDR", ":8095"),
		NATSStream:  getenv("GOLD_NATS_STREAM", "GOLD"),
		AdminSecret: os.Getenv("REPORTING_ADMIN_SECRET"),
	}
	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":          c.DatabaseURL,
			"REPORTING_ADMIN_SECRET": c.AdminSecret,
		} {
			if v == "" {
				return nil, fmt.Errorf("missing required env: %s", k)
			}
		}
	} else if c.AdminSecret == "" {
		c.AdminSecret = "local-reporting-secret"
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
