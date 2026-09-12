package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/tantq/employee-scheduler-backend/internal/adapter/postgres/sqlc"
	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

type EmployeeRepository struct {
	queries *sqlcgen.Queries
}

func NewEmployeeRepository(pool *pgxpool.Pool) *EmployeeRepository {
	return &EmployeeRepository{queries: sqlcgen.New(pool)}
}

var _ port.EmployeeRepository = (*EmployeeRepository)(nil)

func (r *EmployeeRepository) List(ctx context.Context, includeInactive bool) ([]domain.Employee, error) {
	rows, err := r.queries.ListEmployees(ctx, includeInactive)
	if err != nil {
		return nil, fmt.Errorf("query employees: %w", err)
	}

	byID := make(map[string]*domain.Employee, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		e := &domain.Employee{
			EmployeeID: row.EmployeeID,
			Name:       row.Name,
			Role:       domain.Role(row.Role),
			Active:     row.Active,
		}
		byID[e.EmployeeID] = e
		order = append(order, e.EmployeeID)
	}

	leaveRows, err := r.queries.ListEmployeeLeaveDays(ctx)
	if err != nil {
		return nil, fmt.Errorf("query leave days: %w", err)
	}
	for _, lr := range leaveRows {
		if e, ok := byID[lr.EmployeeID]; ok {
			e.LeaveDays = append(e.LeaveDays, domain.Date{Time: lr.LeaveDate})
		}
	}

	result := make([]domain.Employee, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, nil
}

func (r *EmployeeRepository) Create(ctx context.Context, employee domain.Employee) (domain.Employee, error) {
	row, err := r.queries.CreateEmployee(ctx, sqlcgen.CreateEmployeeParams{
		EmployeeID: employee.EmployeeID,
		Name:       employee.Name,
		Role:       string(employee.Role),
	})
	if err != nil {
		return domain.Employee{}, mapEmployeeWriteErr(err)
	}
	return domain.Employee{
		EmployeeID: row.EmployeeID,
		Name:       row.Name,
		Role:       domain.Role(row.Role),
		Active:     row.Active,
	}, nil
}

func (r *EmployeeRepository) Update(ctx context.Context, employeeID, name string, role domain.Role) (domain.Employee, error) {
	row, err := r.queries.UpdateEmployee(ctx, sqlcgen.UpdateEmployeeParams{
		EmployeeID: employeeID,
		Name:       name,
		Role:       string(role),
	})
	if err != nil {
		return domain.Employee{}, mapEmployeeWriteErr(err)
	}
	return domain.Employee{
		EmployeeID: row.EmployeeID,
		Name:       row.Name,
		Role:       domain.Role(row.Role),
		Active:     row.Active,
	}, nil
}

func (r *EmployeeRepository) SetActive(ctx context.Context, employeeID string, active bool) (domain.Employee, error) {
	row, err := r.queries.SetEmployeeActive(ctx, sqlcgen.SetEmployeeActiveParams{
		EmployeeID: employeeID,
		Active:     active,
	})
	if err != nil {
		return domain.Employee{}, mapEmployeeWriteErr(err)
	}
	return domain.Employee{
		EmployeeID: row.EmployeeID,
		Name:       row.Name,
		Role:       domain.Role(row.Role),
		Active:     row.Active,
	}, nil
}

// mapEmployeeWriteErr maps pgx.ErrNoRows -> port.ErrEmployeeNotFound and a
// unique-violation -> port.ErrEmployeeConflict, same mapping every
// single-row write below relies on.
func mapEmployeeWriteErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return port.ErrEmployeeNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return port.ErrEmployeeConflict
	}
	return fmt.Errorf("write employee: %w", err)
}

// Availability is bounded to [from, to] so a solve over a 28-day horizon
// never pulls the whole table.
func (r *EmployeeRepository) Availability(ctx context.Context, from, to domain.Date) (domain.AvailabilityMap, error) {
	rows, err := r.queries.GetAvailability(ctx, sqlcgen.GetAvailabilityParams{
		FromDate: from.Time,
		ToDate:   to.Time,
	})
	if err != nil {
		return nil, fmt.Errorf("query availability: %w", err)
	}

	result := domain.AvailabilityMap{}
	for _, row := range rows {
		if _, ok := result[row.EmployeeID]; !ok {
			result[row.EmployeeID] = map[domain.Date]domain.ShiftAvailability{}
		}
		result[row.EmployeeID][domain.Date{Time: row.AvailDate}] = domain.ShiftAvailability{Sang: row.Sang, Dem: row.Dem}
	}
	return result, nil
}

// SetAvailability upserts one (employee, date) availability row. FK
// violations (unknown employee_id) surface as-is to the caller.
func (r *EmployeeRepository) SetAvailability(ctx context.Context, employeeID string, date domain.Date, avail domain.ShiftAvailability) error {
	if err := r.queries.UpsertAvailability(ctx, sqlcgen.UpsertAvailabilityParams{
		EmployeeID: employeeID,
		AvailDate:  date.Time,
		Sang:       avail.Sang,
		Dem:        avail.Dem,
	}); err != nil {
		return fmt.Errorf("upsert availability: %w", err)
	}
	return nil
}

// SetLeaveDay adds (onLeave=true) or removes (onLeave=false) one leave day.
// Adding is idempotent (ON CONFLICT DO NOTHING); removing a day that was
// never a leave day is a no-op, not an error.
func (r *EmployeeRepository) SetLeaveDay(ctx context.Context, employeeID string, date domain.Date, onLeave bool) error {
	if onLeave {
		if err := r.queries.InsertLeaveDay(ctx, sqlcgen.InsertLeaveDayParams{EmployeeID: employeeID, LeaveDate: date.Time}); err != nil {
			return fmt.Errorf("insert leave day: %w", err)
		}
		return nil
	}

	if err := r.queries.DeleteLeaveDay(ctx, sqlcgen.DeleteLeaveDayParams{EmployeeID: employeeID, LeaveDate: date.Time}); err != nil {
		return fmt.Errorf("delete leave day: %w", err)
	}
	return nil
}
