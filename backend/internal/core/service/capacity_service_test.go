package service

import (
	"context"
	"testing"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

func TestCapacityService_Check_DefaultsEmployeeCountFromRepository(t *testing.T) {
	// Arrange
	employees := []domain.Employee{{EmployeeID: "NV01"}, {EmployeeID: "NV02"}}
	solver := &fakeSolverGateway{capacityResult: domain.CapacityCheckResult{IsSufficient: true}}
	empRepo := &fakeEmployeeRepository{employees: employees}
	svc := NewCapacityService(solver, empRepo, &fakeConfigRepository{})

	// Act
	_, err := svc.Check(context.Background(), CapacityParams{NumDays: 28})

	// Assert
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if solver.capacityReq.EmployeeCount != 2 {
		t.Fatalf("expected employee_count defaulted to roster size 2, got %d", solver.capacityReq.EmployeeCount)
	}
}

func TestCapacityService_Check_UsesProvidedEmployeeCount(t *testing.T) {
	solver := &fakeSolverGateway{}
	svc := NewCapacityService(solver, &fakeEmployeeRepository{employees: []domain.Employee{{EmployeeID: "NV01"}}}, &fakeConfigRepository{})

	override := 21
	if _, err := svc.Check(context.Background(), CapacityParams{NumDays: 28, EmployeeCount: &override}); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if solver.capacityReq.EmployeeCount != 21 {
		t.Fatalf("expected explicit employee_count 21 to win over roster size, got %d", solver.capacityReq.EmployeeCount)
	}
}

func TestCapacityService_Check_RejectsInvalidNumDays(t *testing.T) {
	svc := NewCapacityService(&fakeSolverGateway{}, &fakeEmployeeRepository{}, &fakeConfigRepository{})
	if _, err := svc.Check(context.Background(), CapacityParams{NumDays: 0}); err == nil {
		t.Fatal("expected error for num_days=0, got nil")
	}
}
