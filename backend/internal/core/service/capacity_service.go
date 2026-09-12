package service

import (
	"context"
	"fmt"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/port"
)

type CapacityService struct {
	Solver    port.SolverGateway
	Employees port.EmployeeRepository
	Config    port.ConfigRepository
}

func NewCapacityService(solver port.SolverGateway, employees port.EmployeeRepository, config port.ConfigRepository) *CapacityService {
	return &CapacityService{Solver: solver, Employees: employees, Config: config}
}

type CapacityParams struct {
	NumDays       int
	EmployeeCount *int // nil => default to the current roster size
}

func (s *CapacityService) Check(ctx context.Context, params CapacityParams) (domain.CapacityCheckResult, error) {
	if params.NumDays < 1 {
		return domain.CapacityCheckResult{}, fmt.Errorf("%w: num_days must be >= 1, got %d", ErrInvalidInput, params.NumDays)
	}

	config, err := s.Config.Get(ctx)
	if err != nil {
		return domain.CapacityCheckResult{}, fmt.Errorf("load solver config: %w", err)
	}

	employeeCount := 0
	if params.EmployeeCount != nil {
		employeeCount = *params.EmployeeCount
	} else {
		employees, err := s.Employees.List(ctx, false)
		if err != nil {
			return domain.CapacityCheckResult{}, fmt.Errorf("list employees: %w", err)
		}
		employeeCount = len(employees)
	}

	result, err := s.Solver.CapacityCheck(ctx, port.CapacityCheckRequest{
		NumDays:       params.NumDays,
		EmployeeCount: employeeCount,
		Config:        config,
	})
	if err != nil {
		return domain.CapacityCheckResult{}, fmt.Errorf("call solver: %w", err)
	}
	return result, nil
}
