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

const defaultRosterWindowDays = 28

const (
	maxEmployeeIDLength   = 100
	maxEmployeeNameLength = 200
)

var employeeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type EmployeeService struct {
	Employees port.EmployeeRepository
}

func NewEmployeeService(employees port.EmployeeRepository) *EmployeeService {
	return &EmployeeService{Employees: employees}
}

// Roster returns the full employee list plus their availability for [from, to].
// A zero-value from/to defaults to the next `defaultRosterWindowDays` days
// starting today (matching the default demo horizon), for the UI to show
// before anyone triggers a solve. Callers that need a different window (e.g.
// a registration UI browsing past or future weeks) pass explicit dates —
// otherwise a write to any day outside the default window would succeed but
// never come back on the next GET.
func (s *EmployeeService) Roster(ctx context.Context, includeInactive bool, from, to domain.Date) ([]domain.Employee, domain.AvailabilityMap, error) {
	employees, err := s.Employees.List(ctx, includeInactive)
	if err != nil {
		return nil, nil, fmt.Errorf("list employees: %w", err)
	}

	if from.Time.IsZero() {
		from = domain.Today()
	}
	if to.Time.IsZero() {
		to = domain.Today().AddDays(defaultRosterWindowDays - 1)
	}
	availability, err := s.Employees.Availability(ctx, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("load availability: %w", err)
	}

	// Availability is stored independently and can contain inactive employees.
	// Keep the response consistent with the returned roster.
	allowed := make(map[string]struct{}, len(employees))
	for _, employee := range employees {
		allowed[employee.EmployeeID] = struct{}{}
	}
	filteredAvailability := make(domain.AvailabilityMap, len(availability))
	for employeeID, byDate := range availability {
		if _, ok := allowed[employeeID]; ok {
			filteredAvailability[employeeID] = byDate
		}
	}

	return employees, filteredAvailability, nil
}

func (s *EmployeeService) Create(ctx context.Context, employee domain.Employee) (domain.Employee, error) {
	employee.EmployeeID = strings.TrimSpace(employee.EmployeeID)
	employee.Name = strings.TrimSpace(employee.Name)
	if !validEmployeeID(employee.EmployeeID) || !validEmployeeName(employee.Name) || !validRole(employee.Role) {
		return domain.Employee{}, fmt.Errorf("%w: employee_id must use letters, digits, _ or - (max %d); name is required (max %d); role must be NV, TC, or PC", ErrInvalidInput, maxEmployeeIDLength, maxEmployeeNameLength)
	}
	employee.Active = true
	created, err := s.Employees.Create(ctx, employee)
	if errors.Is(err, port.ErrEmployeeConflict) {
		return domain.Employee{}, fmt.Errorf("%w: employee_id %q already exists", ErrConflict, employee.EmployeeID)
	}
	if err != nil {
		return domain.Employee{}, fmt.Errorf("create employee: %w", err)
	}
	return created, nil
}

func (s *EmployeeService) Update(ctx context.Context, employeeID, name string, role domain.Role) (domain.Employee, error) {
	employeeID = strings.TrimSpace(employeeID)
	name = strings.TrimSpace(name)
	if employeeID == "" || !validEmployeeName(name) || !validRole(role) {
		return domain.Employee{}, fmt.Errorf("%w: name is required (max %d) and role must be NV, TC, or PC", ErrInvalidInput, maxEmployeeNameLength)
	}
	updated, err := s.Employees.Update(ctx, employeeID, name, role)
	if errors.Is(err, port.ErrEmployeeNotFound) {
		return domain.Employee{}, fmt.Errorf("%w: employee_id %q", ErrNotFound, employeeID)
	}
	if err != nil {
		return domain.Employee{}, fmt.Errorf("update employee: %w", err)
	}
	return updated, nil
}

func (s *EmployeeService) Deactivate(ctx context.Context, employeeID string) (domain.Employee, error) {
	return s.setActive(ctx, employeeID, false)
}

func (s *EmployeeService) Restore(ctx context.Context, employeeID string) (domain.Employee, error) {
	return s.setActive(ctx, employeeID, true)
}

func (s *EmployeeService) setActive(ctx context.Context, employeeID string, active bool) (domain.Employee, error) {
	employeeID = strings.TrimSpace(employeeID)
	if employeeID == "" {
		return domain.Employee{}, fmt.Errorf("%w: employee_id is required", ErrInvalidInput)
	}
	employee, err := s.Employees.SetActive(ctx, employeeID, active)
	if errors.Is(err, port.ErrEmployeeNotFound) {
		return domain.Employee{}, fmt.Errorf("%w: employee_id %q", ErrNotFound, employeeID)
	}
	if err != nil {
		return domain.Employee{}, fmt.Errorf("set employee active state: %w", err)
	}
	return employee, nil
}

func validRole(role domain.Role) bool {
	return role == domain.RoleNV || role == domain.RoleTC || role == domain.RolePC
}

func validEmployeeID(employeeID string) bool {
	return utf8.RuneCountInString(employeeID) <= maxEmployeeIDLength && employeeIDPattern.MatchString(employeeID)
}

func validEmployeeName(name string) bool {
	return name != "" && utf8.RuneCountInString(name) <= maxEmployeeNameLength
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
