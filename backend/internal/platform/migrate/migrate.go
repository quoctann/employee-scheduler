// Package migrate embeds the SQL migrations into the binary (via
// migrations.FS) and runs them with golang-migrate, replacing the external
// migrate CLI that used to be invoked separately at deploy time.
package migrate

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/quoctann/employee-scheduler-backend/migrations"
)

// open builds a *migrate.Migrate wired to the embedded migration files and
// a short-lived sql.DB connection independent of the app's long-lived
// pgxpool.Pool.
func open(databaseURL string) (*migrate.Migrate, func() error, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	dbDriver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create migrate database driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create migrate source driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "pgx5", dbDriver)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create migrate instance: %w", err)
	}

	return m, db.Close, nil
}

// Up applies all pending migrations. A no-op (nil error) if the schema is
// already up to date.
func Up(databaseURL string) error {
	m, closeDB, err := open(databaseURL)
	if err != nil {
		return err
	}
	defer closeDB()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Down rolls back all migrations.
func Down(databaseURL string) error {
	m, closeDB, err := open(databaseURL)
	if err != nil {
		return err
	}
	defer closeDB()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// Version reports the current schema version and whether it's in a dirty
// (partially applied) state.
func Version(databaseURL string) (version uint, dirty bool, err error) {
	m, closeDB, err := open(databaseURL)
	if err != nil {
		return 0, false, err
	}
	defer closeDB()

	version, dirty, err = m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("migrate version: %w", err)
	}
	return version, dirty, nil
}
