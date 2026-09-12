// Package logging builds the application's structured (zap) logger. Logs are
// always JSON on stdout; the cluster's log-collection agent scrapes stdout
// into Loki, so the app never ships logs anywhere itself.
package logging

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a JSON zap.Logger at the given level (debug/info/warn/error;
// defaults to info for an empty or unrecognized value).
func New(level string) (*zap.Logger, error) {
	var zapLevel zapcore.Level
	if level == "" {
		zapLevel = zapcore.InfoLevel
	} else if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("parse log level %q: %w", level, err)
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return logger, nil
}
