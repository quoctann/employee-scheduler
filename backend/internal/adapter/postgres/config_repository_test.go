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

func TestConfigRepository_RenameGate_CascadesToReferencingTables(t *testing.T) {
	pool := testPool(t)
	repo := NewConfigRepository(pool)

	if err := repo.RenameGate(context.Background(), "A", "A2"); err != nil {
		t.Fatalf("RenameGate() error = %v", err)
	}

	config, err := repo.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, stillThere := config.ShiftHours["A"]; stillThere {
		t.Fatalf("Get() still has old code %q: %+v", "A", config.ShiftHours)
	}
	if config.ShiftHours["A2"][domain.ShiftDem] != 13 {
		t.Fatalf("Get() = %+v, want the seeded A/dem requirement to follow the new code A2", config.ShiftHours)
	}

	var orphaned int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM gate_shift_requirements WHERE gate_code = 'A'`).Scan(&orphaned); err != nil {
		t.Fatalf("count orphaned gate_shift_requirements: %v", err)
	}
	if orphaned != 0 {
		t.Fatalf("gate_shift_requirements still has %d row(s) referencing the old code, want the FK's ON UPDATE CASCADE to have renamed them", orphaned)
	}
}

func TestConfigRepository_RenameGate_UnknownGate_ReturnsErrGateNotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewConfigRepository(pool)

	err := repo.RenameGate(context.Background(), "does-not-exist", "X")
	if !errors.Is(err, port.ErrGateNotFound) {
		t.Fatalf("RenameGate() error = %v, want ErrGateNotFound", err)
	}
}

func TestConfigRepository_RenameGate_ExistingNewCode_ReturnsErrGateConflict(t *testing.T) {
	pool := testPool(t)
	repo := NewConfigRepository(pool)

	err := repo.RenameGate(context.Background(), "A", "B")
	if !errors.Is(err, port.ErrGateConflict) {
		t.Fatalf("RenameGate() error = %v, want ErrGateConflict", err)
	}
}
