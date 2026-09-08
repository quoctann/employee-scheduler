package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

func TestConfigRepository_Get_ReturnsSeededSolverConfig(t *testing.T) {
	pool := testPool(t)
	repo := NewConfigRepository(pool)

	config, err := repo.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if config.TargetHoursPerWeek != 44 {
		t.Fatalf("expected target_hours_per_week=44, got %d", config.TargetHoursPerWeek)
	}
	if len(config.LeadGates) != 1 || config.LeadGates[0] != "B" {
		t.Fatalf("expected lead_gates=[B], got %v", config.LeadGates)
	}
	bDem, ok := config.Requirements["B"][domain.ShiftDem]
	if !ok {
		t.Fatal("expected B/dem requirement to be present")
	}
	if !bDem.LeadMandatoryRole || bDem.Lead != 1 {
		t.Fatalf("expected B/dem to require a mandatory lead, got %+v", bDem)
	}
	if config.ShiftHours["A"][domain.ShiftDem] != 13 {
		t.Fatalf("expected A/dem shift_hours=13, got %d", config.ShiftHours["A"][domain.ShiftDem])
	}
}

func TestConfigRepository_UpdateGateShiftRequirement_UpdatesAndPersists(t *testing.T) {
	pool := testPool(t)
	repo := NewConfigRepository(pool)

	err := repo.UpdateGateShiftRequirement(context.Background(), "A", domain.ShiftSang, domain.GateShiftRequirement{NV: 3, Lead: 1, LeadMandatoryRole: true}, 10)
	if err != nil {
		t.Fatalf("UpdateGateShiftRequirement() error = %v", err)
	}

	config, err := repo.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	got := config.Requirements["A"][domain.ShiftSang]
	if got.NV != 3 || got.Lead != 1 || !got.LeadMandatoryRole {
		t.Fatalf("updated requirement = %+v, want {NV:3 Lead:1 LeadMandatoryRole:true}", got)
	}
	if config.ShiftHours["A"][domain.ShiftSang] != 10 {
		t.Fatalf("updated shift_hours = %d, want 10", config.ShiftHours["A"][domain.ShiftSang])
	}
}

func TestConfigRepository_UpdateGateShiftRequirement_UnknownPair_ReturnsErrGateShiftNotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewConfigRepository(pool)

	err := repo.UpdateGateShiftRequirement(context.Background(), "does-not-exist", domain.ShiftSang, domain.GateShiftRequirement{}, 8)
	if !errors.Is(err, port.ErrGateShiftNotFound) {
		t.Fatalf("UpdateGateShiftRequirement() error = %v, want ErrGateShiftNotFound", err)
	}
}
