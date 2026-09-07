package service

import (
	"context"
	"fmt"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

const defaultRosterWindowDays = 28

type EmployeeService struct {
	Employees port.EmployeeRepository
}

func NewEmployeeService(employees port.EmployeeRepository) *EmployeeService {
	return &EmployeeService{Employees: employees}
}

// Roster returns the full employee list plus their availability over the
// next `defaultRosterWindowDays` days (matching the default demo horizon),
// for the UI to show before anyone triggers a solve.
func (s *EmployeeService) Roster(ctx context.Context) ([]domain.Employee, domain.AvailabilityMap, error) {
	employees, err := s.Employees.List(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list employees: %w", err)
	}

	today := domain.Today()
	availability, err := s.Employees.Availability(ctx, today, today.AddDays(defaultRosterWindowDays-1))
	if err != nil {
		return nil, nil, fmt.Errorf("load availability: %w", err)
	}

	return employees, availability, nil
}

// SetAvailability records an employee's self-declared shift availability for
// one day (both false is a valid "opted out that day" registration, not an
// error — see domain.ShiftAvailability).
func (s *EmployeeService) SetAvailability(ctx context.Context, employeeID string, date domain.Date, avail domain.ShiftAvailability) error {
	if employeeID == "" {
		return fmt.Errorf("%w: employee_id is required", ErrInvalidInput)
	}
	if date.Time.IsZero() {
		return fmt.Errorf("%w: date is required", ErrInvalidInput)
	}
	if err := s.Employees.SetAvailability(ctx, employeeID, date, avail); err != nil {
		return fmt.Errorf("set availability: %w", err)
	}
	return nil
}

// SetLeaveDay registers or clears a leave day (nghỉ/phép) for an employee.
func (s *EmployeeService) SetLeaveDay(ctx context.Context, employeeID string, date domain.Date, onLeave bool) error {
	if employeeID == "" {
		return fmt.Errorf("%w: employee_id is required", ErrInvalidInput)
	}
	if date.Time.IsZero() {
		return fmt.Errorf("%w: date is required", ErrInvalidInput)
	}
	if err := s.Employees.SetLeaveDay(ctx, employeeID, date, onLeave); err != nil {
		return fmt.Errorf("set leave day: %w", err)
	}
	return nil
}
