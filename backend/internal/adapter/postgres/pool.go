// Package postgres implements EmployeeRepository, ConfigRepository, and
// ScheduleRepository against a real Postgres database via pgx and sqlc —
// no ORM, matching the repository pattern (interface in `port`, storage
// details confined here).
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgxpool.Pool. tracer, when non-nil, is attached as the
// pool's pgx.QueryTracer so every query becomes a child span of the request
// that issued it (see internal/platform/tracing).
func NewPool(ctx context.Context, databaseURL string, tracer pgx.QueryTracer) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	if tracer != nil {
		poolCfg.ConnConfig.Tracer = tracer
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}
