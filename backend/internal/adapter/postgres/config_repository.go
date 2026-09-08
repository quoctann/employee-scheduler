package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

type ConfigRepository struct {
	pool *pgxpool.Pool
}

func NewConfigRepository(pool *pgxpool.Pool) *ConfigRepository {
	return &ConfigRepository{pool: pool}
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

	gateRows, err := r.pool.Query(ctx, `SELECT code, is_lead_gate FROM gates ORDER BY code`)
	if err != nil {
		return domain.SolverConfig{}, fmt.Errorf("query gates: %w", err)
	}
	for gateRows.Next() {
		var code string
		var isLead bool
		if err := gateRows.Scan(&code, &isLead); err != nil {
			gateRows.Close()
			return domain.SolverConfig{}, fmt.Errorf("scan gate: %w", err)
		}
		config.Requirements[code] = map[domain.ShiftType]domain.GateShiftRequirement{}
		config.ShiftHours[code] = map[domain.ShiftType]int{}
		if isLead {
			config.LeadGates = append(config.LeadGates, code)
		}
	}
	gateRows.Close()
	if err := gateRows.Err(); err != nil {
		return domain.SolverConfig{}, fmt.Errorf("iterate gates: %w", err)
	}

	reqRows, err := r.pool.Query(ctx, `
		SELECT gate_code, shift_type, nv, lead, lead_mandatory_role, shift_hours
		FROM gate_shift_requirements
	`)
	if err != nil {
		return domain.SolverConfig{}, fmt.Errorf("query gate_shift_requirements: %w", err)
	}
	for reqRows.Next() {
		var gateCode, shiftType string
		var nv, lead, shiftHours int
		var leadMandatory bool
		if err := reqRows.Scan(&gateCode, &shiftType, &nv, &lead, &leadMandatory, &shiftHours); err != nil {
			reqRows.Close()
			return domain.SolverConfig{}, fmt.Errorf("scan gate_shift_requirement: %w", err)
		}
		st := domain.ShiftType(shiftType)
		config.Requirements[gateCode][st] = domain.GateShiftRequirement{NV: nv, Lead: lead, LeadMandatoryRole: leadMandatory}
		config.ShiftHours[gateCode][st] = shiftHours
	}
	reqRows.Close()
	if err := reqRows.Err(); err != nil {
		return domain.SolverConfig{}, fmt.Errorf("iterate gate_shift_requirements: %w", err)
	}

	row := r.pool.QueryRow(ctx, `
		SELECT target_hours_per_week, shortfall_penalty, lead_shortfall_penalty,
		       balance_penalty_weight, streak_penalty_weight, streak_length
		FROM solver_settings
		WHERE id = 1
	`)
	if err := row.Scan(
		&config.TargetHoursPerWeek,
		&config.Weights.ShortfallPenalty,
		&config.Weights.LeadShortfallPenalty,
		&config.Weights.BalancePenaltyWeight,
		&config.Weights.StreakPenaltyWeight,
		&config.Weights.StreakLength,
	); err != nil {
		return domain.SolverConfig{}, fmt.Errorf("scan solver_settings: %w", err)
	}

	return config, nil
}

// UpdateGateShiftRequirement updates one (gate, shift) staffing row.
// ErrGateShiftNotFound if the pair doesn't already exist — this endpoint
// edits seeded config, it does not create new gates or shift types.
func (r *ConfigRepository) UpdateGateShiftRequirement(ctx context.Context, gateCode string, shiftType domain.ShiftType, requirement domain.GateShiftRequirement, shiftHours int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE gate_shift_requirements
		SET nv = $3, lead = $4, lead_mandatory_role = $5, shift_hours = $6
		WHERE gate_code = $1 AND shift_type = $2
	`, gateCode, shiftType, requirement.NV, requirement.Lead, requirement.LeadMandatoryRole, shiftHours)
	if err != nil {
		return fmt.Errorf("update gate_shift_requirement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return port.ErrGateShiftNotFound
	}
	return nil
}

// RenameGate updates gates.code; the ON UPDATE CASCADE added in
// migrations/0004_gate_rename_cascade propagates the new code to
// gate_shift_requirements, approved_assignments, schedule_assignments, and
// schedule_shortages in the same statement.
func (r *ConfigRepository) RenameGate(ctx context.Context, oldCode, newCode string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE gates SET code = $2 WHERE code = $1`, oldCode, newCode)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return port.ErrGateConflict
		}
		return fmt.Errorf("rename gate: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return port.ErrGateNotFound
	}
	return nil
}
