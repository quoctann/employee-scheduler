package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

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
	maxGateCodeLength     = 20
)

// Same charset as employee IDs (see employee_service.go) — gate codes flow
// into the same kind of places (URL path segments, badge labels).
var gateCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func validGateCode(code string) bool {
	return utf8.RuneCountInString(code) <= maxGateCodeLength && gateCodePattern.MatchString(code)
}

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

// RenameGate changes a gate's code everywhere it's referenced (staffing
// requirements, past schedule runs, approved assignments — see
// migrations/0004_gate_rename_cascade). Returns the full config as it reads
// after the rename, so the caller can refresh its view in one round trip.
func (s *ConfigService) RenameGate(ctx context.Context, oldCode, newCode string) (domain.SolverConfig, error) {
	oldCode = strings.TrimSpace(oldCode)
	newCode = strings.TrimSpace(newCode)
	if !validGateCode(oldCode) || !validGateCode(newCode) {
		return domain.SolverConfig{}, fmt.Errorf("%w: gate codes must use letters, digits, _ or - (max %d)", ErrInvalidInput, maxGateCodeLength)
	}

	if err := s.Config.RenameGate(ctx, oldCode, newCode); err != nil {
		if errors.Is(err, port.ErrGateNotFound) {
			return domain.SolverConfig{}, fmt.Errorf("%w: gate %q", ErrNotFound, oldCode)
		}
		if errors.Is(err, port.ErrGateConflict) {
			return domain.SolverConfig{}, fmt.Errorf("%w: gate %q already exists", ErrConflict, newCode)
		}
		return domain.SolverConfig{}, fmt.Errorf("rename gate: %w", err)
	}
	return s.Get(ctx)
}
