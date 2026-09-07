package postgres

import (
	"context"
	"testing"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
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
