package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

// Mirrors the CHECK constraints on gate_shift_requirements (see
// backend/migrations/0001_init_schema.up.sql) — validated here too so a bad
// request comes back as a clean 400 instead of a raw DB constraint error.
const (
	maxGateShiftHeadcount = 1000
	minShiftHours         = 1
	maxShiftHours         = 24
)

// ConfigService exposes scheduling configuration — gates, per-gate/shift
// staffing requirements, weights — to inbound adapters, and lets managers
// edit the staffing requirements.
type ConfigService struct {
	Config port.ConfigRepository
}

func NewConfigService(config port.ConfigRepository) *ConfigService {
	return &ConfigService{Config: config}
}

func (s *ConfigService) Get(ctx context.Context) (domain.SolverConfig, error) {
	config, err := s.Config.Get(ctx)
	if err != nil {
		return domain.SolverConfig{}, fmt.Errorf("load config: %w", err)
	}
	return config, nil
}

// UpdateGateShiftRequirement edits how many staff (NV) and leads a given
// gate/shift needs, whether that lead slot must be filled by a TC-role
// employee, and how long the shift runs. Returns the full config as it
// reads after the update, so the caller can refresh its view in one round
// trip.
func (s *ConfigService) UpdateGateShiftRequirement(ctx context.Context, gateCode string, shiftType domain.ShiftType, requirement domain.GateShiftRequirement, shiftHours int) (domain.SolverConfig, error) {
	gateCode = strings.TrimSpace(gateCode)
	if gateCode == "" {
		return domain.SolverConfig{}, fmt.Errorf("%w: gate_code is required", ErrInvalidInput)
	}
	if shiftType != domain.ShiftSang && shiftType != domain.ShiftDem {
		return domain.SolverConfig{}, fmt.Errorf("%w: shift_type must be sang or dem", ErrInvalidInput)
	}
	if requirement.NV < 0 || requirement.NV > maxGateShiftHeadcount || requirement.Lead < 0 || requirement.Lead > maxGateShiftHeadcount {
		return domain.SolverConfig{}, fmt.Errorf("%w: nv and lead must be between 0 and %d", ErrInvalidInput, maxGateShiftHeadcount)
	}
	if shiftHours < minShiftHours || shiftHours > maxShiftHours {
		return domain.SolverConfig{}, fmt.Errorf("%w: shift_hours must be between %d and %d", ErrInvalidInput, minShiftHours, maxShiftHours)
	}

	if err := s.Config.UpdateGateShiftRequirement(ctx, gateCode, shiftType, requirement, shiftHours); err != nil {
		if errors.Is(err, port.ErrGateShiftNotFound) {
			return domain.SolverConfig{}, fmt.Errorf("%w: gate %q has no %q shift", ErrNotFound, gateCode, shiftType)
		}
		return domain.SolverConfig{}, fmt.Errorf("update gate shift requirement: %w", err)
	}
	return s.Get(ctx)
}
