// Package config loads runtime configuration from environment variables,
// failing fast if a required secret is missing rather than starting in a
// half-configured state.
package config

import (
	"fmt"
	"net"
	"net/url"
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
	databaseURL, err := loadDatabaseURL()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		DatabaseURL:   databaseURL,
		SolverAPIKey:  os.Getenv("SOLVER_API_KEY"),
		SolverBaseURL: getenv("SOLVER_BASE_URL", "http://localhost:8080"),
		HTTPPort:      getenv("HTTP_PORT", "8081"),
		CORSOrigin:    getenv("CORS_ORIGIN", "http://localhost:5173"),
	}
	if cfg.SolverAPIKey == "" {
		return Config{}, fmt.Errorf("SOLVER_API_KEY is required (must match solver-service's API_KEY)")
	}
	return cfg, nil
}

func loadDatabaseURL() (string, error) {
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		return databaseURL, nil
	}

	host := os.Getenv("DATABASE_HOST")
	if host == "" {
		return "", fmt.Errorf("DATABASE_URL or DATABASE_HOST is required")
	}

	user := os.Getenv("DATABASE_USER")
	if user == "" {
		return "", fmt.Errorf("DATABASE_USER is required when DATABASE_HOST is set")
	}

	userInfo := url.User(user)
	if password := os.Getenv("DATABASE_PASSWORD"); password != "" {
		userInfo = url.UserPassword(user, password)
	}

	connectionURL := &url.URL{
		Scheme: "postgres",
		User:   userInfo,
		Host:   net.JoinHostPort(host, getenv("DATABASE_PORT", "5432")),
		Path:   getenv("DATABASE_NAME", "employee_scheduler"),
	}
	query := connectionURL.Query()
	query.Set("sslmode", getenv("DATABASE_SSLMODE", "disable"))
	connectionURL.RawQuery = query.Encode()
	return connectionURL.String(), nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
