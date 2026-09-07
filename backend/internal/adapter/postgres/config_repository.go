package postgres

import (
	"context"
	"fmt"

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
