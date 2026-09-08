package port

import (
	"context"
	"errors"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

// ErrGateShiftNotFound is returned when updating a (gate, shift) pair that
// has no row in `gate_shift_requirements` (unknown gate code or shift type).
var ErrGateShiftNotFound = errors.New("gate/shift requirement not found")

// ConfigRepository is the outbound port for business-configurable scheduling
// rules (gates, per-gate/shift staffing requirements, weights). Backed by
// `gates` / `gate_shift_requirements` / `solver_settings` — data, not code.
type ConfigRepository interface {
	Get(ctx context.Context) (domain.SolverConfig, error)
	// UpdateGateShiftRequirement updates the staffing requirement (NV count,
	// lead count, whether the lead slot must be filled by a TC-role
	// employee) and shift duration for one existing (gate, shift) pair.
	UpdateGateShiftRequirement(ctx context.Context, gateCode string, shiftType domain.ShiftType, requirement domain.GateShiftRequirement, shiftHours int) error
}
