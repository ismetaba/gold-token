// Package obs provides observability primitives — logger, tracer, metrics.
package obs

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger builds a structured JSON logger for production,
// or human-readable console logger when GOLD_ENV=local.
func NewLogger(service string) *zap.Logger {
	var cfg zap.Config
	if os.Getenv("GOLD_ENV") == "local" {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	logger, err := cfg.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(zap.String("service", service)),
	)
	if err != nil {
		// Fallback: log'ın loglayamaması — en azından hata stdout'a.
		panic(err)
	}
	return logger
}
