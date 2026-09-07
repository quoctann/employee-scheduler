package config

import "testing"

func TestLoadBuildsDatabaseURLFromDiscreteEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DATABASE_HOST", "localhost")
	t.Setenv("DATABASE_PORT", "5432")
	t.Setenv("DATABASE_USER", "postgres")
	t.Setenv("DATABASE_PASSWORD", "password@with:special/chars")
	t.Setenv("DATABASE_NAME", "employee_scheduler")
	t.Setenv("DATABASE_SSLMODE", "disable")
	t.Setenv("SOLVER_API_KEY", "test-api-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := "postgres://postgres:password%40with%3Aspecial%2Fchars@localhost:5432/employee_scheduler?sslmode=disable"
	if cfg.DatabaseURL != want {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
}

func TestLoadPrefersDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://configured-url/database")
	t.Setenv("DATABASE_HOST", "ignored-host")
	t.Setenv("SOLVER_API_KEY", "test-api-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseURL != "postgres://configured-url/database" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
}

func TestLoadRequiresDatabaseConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DATABASE_HOST", "")
	t.Setenv("SOLVER_API_KEY", "test-api-key")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing database configuration error")
	}
}
