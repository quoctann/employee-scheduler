// Package config loads runtime configuration from environment variables
// (and, for local dev, a ".env" file), failing fast if a required secret is
// missing rather than starting in a half-configured state.
package config

import (
	"fmt"
	"net"
	"net/url"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL     string `env:"DATABASE_URL"`
	DatabaseHost    string `env:"DATABASE_HOST"`
	DatabasePort    string `env:"DATABASE_PORT" envDefault:"5432"`
	DatabaseUser    string `env:"DATABASE_USER"`
	DatabasePass    string `env:"DATABASE_PASSWORD"`
	DatabaseName    string `env:"DATABASE_NAME" envDefault:"employee_scheduler"`
	DatabaseSSLMode string `env:"DATABASE_SSLMODE" envDefault:"disable"`

	SolverAPIKey  string `env:"SOLVER_API_KEY,required"`
	SolverBaseURL string `env:"SOLVER_BASE_URL" envDefault:"http://localhost:8080"`

	HTTPPort   string `env:"HTTP_PORT" envDefault:"8081"`
	CORSOrigin string `env:"CORS_ORIGIN" envDefault:"http://localhost:5173"`

	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	DBAutoMigrate bool `env:"DB_AUTO_MIGRATE" envDefault:"false"`

	OTelSDKDisabled          bool   `env:"OTEL_SDK_DISABLED" envDefault:"false"`
	OTelExporterOTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:"lgtm.observability.svc:4317"`
	OTelServiceName          string `env:"OTEL_SERVICE_NAME" envDefault:"employee-scheduler-backend"`
}

// Load reads configuration from the environment, first loading a ".env"
// file if one is present (e.g. local dev). godotenv.Load never overwrites
// variables already set in the environment, so this is safe to call
// unconditionally even where no ".env" exists (the container image never
// ships one).
func Load() (Config, error) {
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	databaseURL, err := resolveDatabaseURL(cfg)
	if err != nil {
		return Config{}, err
	}
	cfg.DatabaseURL = databaseURL

	return cfg, nil
}

func resolveDatabaseURL(cfg Config) (string, error) {
	if cfg.DatabaseURL != "" {
		return cfg.DatabaseURL, nil
	}

	if cfg.DatabaseHost == "" {
		return "", fmt.Errorf("DATABASE_URL or DATABASE_HOST is required")
	}
	if cfg.DatabaseUser == "" {
		return "", fmt.Errorf("DATABASE_USER is required when DATABASE_HOST is set")
	}

	userInfo := url.User(cfg.DatabaseUser)
	if cfg.DatabasePass != "" {
		userInfo = url.UserPassword(cfg.DatabaseUser, cfg.DatabasePass)
	}

	connectionURL := &url.URL{
		Scheme: "postgres",
		User:   userInfo,
		Host:   net.JoinHostPort(cfg.DatabaseHost, cfg.DatabasePort),
		Path:   cfg.DatabaseName,
	}
	query := connectionURL.Query()
	query.Set("sslmode", cfg.DatabaseSSLMode)
	connectionURL.RawQuery = query.Encode()
	return connectionURL.String(), nil
}
