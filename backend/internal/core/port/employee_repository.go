package port

import (
	"context"
	"errors"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

var (
	ErrEmployeeNotFound = errors.New("employee not found")
	ErrEmployeeConflict = errors.New("employee already exists")
)

// EmployeeRepository is the outbound port for the employee roster and their
// registered availability. The Postgres implementation lives in adapter/postgres.
type EmployeeRepository interface {
	// List returns active employees by default, or all records for management.
	List(ctx context.Context, includeInactive bool) ([]domain.Employee, error)
	Create(ctx context.Context, employee domain.Employee) (domain.Employee, error)
	Update(ctx context.Context, employeeID, name string, role domain.Role) (domain.Employee, error)
	SetActive(ctx context.Context, employeeID string, active bool) (domain.Employee, error)
	// Availability returns availability bounded to [from, to] inclusive, so a
	// solve over a 28-day horizon never pulls the whole table.
	Availability(ctx context.Context, from, to domain.Date) (domain.AvailabilityMap, error)
	// SetAvailability upserts the employee's self-declared shift availability
	// for one day. Both false is a legitimate registration (the "N" code —
	// employee opted out that day) and must NOT be confused with a leave day:
	// it does not reduce their target hours.
	SetAvailability(ctx context.Context, employeeID string, date domain.Date, avail domain.ShiftAvailability) error
	// SetLeaveDay adds or removes a leave day (nghỉ/phép), which — unlike a
	// plain unavailable day — proportionally reduces the employee's target
	// hours for the horizon it falls in.
	SetLeaveDay(ctx context.Context, employeeID string, date domain.Date, onLeave bool) error
}
