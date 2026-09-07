// Package config loads runtime configuration from environment variables,
// failing fast if a required secret is missing rather than starting in a
// half-configured state.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL   string
	SolverBaseURL string
	SolverAPIKey  string
	HTTPPort      string
	CORSOrigin    string
}

func Load() (Config, error) {
	cfg := Config{
		// DatabaseURL and SolverAPIKey have no default — a demo-only
		// connection string baked in here would make this check unreachable
		// and silently point production at a throwaway local database.
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		SolverAPIKey:  os.Getenv("SOLVER_API_KEY"),
		SolverBaseURL: getenv("SOLVER_BASE_URL", "http://localhost:8080"),
		HTTPPort:      getenv("HTTP_PORT", "8081"),
		CORSOrigin:    getenv("CORS_ORIGIN", "http://localhost:5173"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.SolverAPIKey == "" {
		return Config{}, fmt.Errorf("SOLVER_API_KEY is required (must match solver-service's API_KEY)")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
