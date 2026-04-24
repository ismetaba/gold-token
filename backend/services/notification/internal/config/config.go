package config

import (
	"fmt"
	"os"
)

type Config struct {
	Env               string
	HTTPAddr          string
	DatabaseURL       string
	NATSURL           string
	NATSStream        string
	JWTPublicKeyFile  string
	SMTPHost          string
	SMTPPort          string
	SMTPUser          string
	SMTPPass          string
	SMTPFrom          string
	SendGridAPIKey    string
}

func FromEnv() (*Config, error) {
	c := &Config{
		Env:              getenv("GOLD_ENV", "local"),
		HTTPAddr:         getenv("GOLD_NOTIFICATION_HTTP_ADDR", ":8093"),
		NATSStream:       getenv("GOLD_NATS_STREAM", "GOLD"),
		JWTPublicKeyFile: os.Getenv("JWT_PUBLIC_KEY_FILE"),
		SMTPHost:         os.Getenv("SMTP_HOST"),
		SMTPPort:         getenv("SMTP_PORT", "587"),
		SMTPUser:         os.Getenv("SMTP_USER"),
		SMTPPass:         os.Getenv("SMTP_PASS"),
		SMTPFrom:         getenv("SMTP_FROM", "noreply@gold-token.io"),
		SendGridAPIKey:   os.Getenv("SENDGRID_API_KEY"),
	}
	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL": c.DatabaseURL,
			"NATS_URL":     c.NATSURL,
		} {
			if v == "" {
				return nil, fmt.Errorf("missing required env: %s", k)
			}
		}
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
