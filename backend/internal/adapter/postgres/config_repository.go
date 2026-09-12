package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/tantq/employee-scheduler-backend/internal/adapter/postgres/sqlc"
	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

type ConfigRepository struct {
	queries *sqlcgen.Queries
}

func NewConfigRepository(pool *pgxpool.Pool) *ConfigRepository {
	return &ConfigRepository{queries: sqlcgen.New(pool)}
}

var _ port.ConfigRepository = (*ConfigRepository)(nil)

// Get assembles domain.SolverConfig from `gates`, `gate_shift_requirements`,
// and the singleton `solver_settings` row — business-configurable scheduling
// rules as real, queryable data rather than a hardcoded default.
func (r *ConfigRepository) Get(ctx context.Context) (domain.SolverConfig, error) {
	config := domain.SolverConfig{
		Requirements: map[string]map[domain.ShiftType]domain.GateShiftRequirement{},
		ShiftHours:   map[string]map[domain.ShiftType]int{},
	}

	gates, err := r.queries.ListGates(ctx)
	if err != nil {
		return domain.SolverConfig{}, fmt.Errorf("query gates: %w", err)
	}
	for _, gate := range gates {
		config.Requirements[gate.Code] = map[domain.ShiftType]domain.GateShiftRequirement{}
		config.ShiftHours[gate.Code] = map[domain.ShiftType]int{}
		if gate.IsLeadGate {
			config.LeadGates = append(config.LeadGates, gate.Code)
		}
	}

	requirements, err := r.queries.ListGateShiftRequirements(ctx)
	if err != nil {
		return domain.SolverConfig{}, fmt.Errorf("query gate_shift_requirements: %w", err)
	}
	for _, req := range requirements {
		st := domain.ShiftType(req.ShiftType)
		config.Requirements[req.GateCode][st] = domain.GateShiftRequirement{
			NV:                int(req.Nv),
			Lead:              int(req.Lead),
			LeadMandatoryRole: req.LeadMandatoryRole,
		}
		config.ShiftHours[req.GateCode][st] = int(req.ShiftHours)
	}

	settings, err := r.queries.GetSolverSettings(ctx)
	if err != nil {
		return domain.SolverConfig{}, fmt.Errorf("query solver_settings: %w", err)
	}
	config.TargetHoursPerWeek = int(settings.TargetHoursPerWeek)
	config.Weights.ShortfallPenalty = int(settings.ShortfallPenalty)
	config.Weights.LeadShortfallPenalty = int(settings.LeadShortfallPenalty)
	config.Weights.BalancePenaltyWeight = int(settings.BalancePenaltyWeight)
	config.Weights.StreakPenaltyWeight = int(settings.StreakPenaltyWeight)
	config.Weights.StreakLength = int(settings.StreakLength)

	return config, nil
}

// UpdateGateShiftRequirement updates one (gate, shift) staffing row.
// ErrGateShiftNotFound if the pair doesn't already exist — this endpoint
// edits seeded config, it does not create new gates or shift types.
func (r *ConfigRepository) UpdateGateShiftRequirement(ctx context.Context, gateCode string, shiftType domain.ShiftType, requirement domain.GateShiftRequirement, shiftHours int) error {
	rowsAffected, err := r.queries.UpdateGateShiftRequirement(ctx, sqlcgen.UpdateGateShiftRequirementParams{
		GateCode:          gateCode,
		ShiftType:         string(shiftType),
		Nv:                int32(requirement.NV),
		Lead:              int32(requirement.Lead),
		LeadMandatoryRole: requirement.LeadMandatoryRole,
		ShiftHours:        int32(shiftHours),
	})
	if err != nil {
		return fmt.Errorf("update gate_shift_requirement: %w", err)
	}
	if rowsAffected == 0 {
		return port.ErrGateShiftNotFound
	}
	return nil
}

// RenameGate updates gates.code; the ON UPDATE CASCADE added in
// migrations/0004_gate_rename_cascade propagates the new code to
// gate_shift_requirements, approved_assignments, schedule_assignments, and
// schedule_shortages in the same statement.
func (r *ConfigRepository) RenameGate(ctx context.Context, oldCode, newCode string) error {
	rowsAffected, err := r.queries.RenameGate(ctx, sqlcgen.RenameGateParams{Code: oldCode, Code_2: newCode})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return port.ErrGateConflict
		}
		return fmt.Errorf("rename gate: %w", err)
	}
	if rowsAffected == 0 {
		return port.ErrGateNotFound
	}
	return nil
}
