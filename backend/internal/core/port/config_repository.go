package port

import (
	"context"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

// ConfigRepository is the outbound port for business-configurable scheduling
// rules (gates, per-gate/shift staffing requirements, weights). Backed by
// `gates` / `gate_shift_requirements` / `solver_settings` — data, not code.
type ConfigRepository interface {
	Get(ctx context.Context) (domain.SolverConfig, error)
}
