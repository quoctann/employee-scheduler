package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

type EmployeeRepository struct {
	pool *pgxpool.Pool
}

func NewEmployeeRepository(pool *pgxpool.Pool) *EmployeeRepository {
	return &EmployeeRepository{pool: pool}
}

var _ port.EmployeeRepository = (*EmployeeRepository)(nil)

func (r *EmployeeRepository) List(ctx context.Context, includeInactive bool) ([]domain.Employee, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT employee_id, name, role, active
		FROM employees
		WHERE active OR $1
		ORDER BY employee_id
	`, includeInactive)
	if err != nil {
		return nil, fmt.Errorf("query employees: %w", err)
	}

	byID := make(map[string]*domain.Employee)
	order := make([]string, 0)
	for rows.Next() {
		var e domain.Employee
		var role string
		if err := rows.Scan(&e.EmployeeID, &e.Name, &role, &e.Active); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan employee: %w", err)
		}
		e.Role = domain.Role(role)
		byID[e.EmployeeID] = &e
		order = append(order, e.EmployeeID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate employees: %w", err)
	}

	leaveRows, err := r.pool.Query(ctx, `
		SELECT employee_id, leave_date FROM employee_leave_days ORDER BY employee_id, leave_date
	`)
	if err != nil {
		return nil, fmt.Errorf("query leave days: %w", err)
	}
	for leaveRows.Next() {
		var employeeID string
		var leaveDate domain.Date
		if err := leaveRows.Scan(&employeeID, &leaveDate.Time); err != nil {
			leaveRows.Close()
			return nil, fmt.Errorf("scan leave day: %w", err)
		}
		if e, ok := byID[employeeID]; ok {
			e.LeaveDays = append(e.LeaveDays, leaveDate)
		}
	}
	leaveRows.Close()
	if err := leaveRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate leave days: %w", err)
	}

	result := make([]domain.Employee, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, nil
}

func (r *EmployeeRepository) Create(ctx context.Context, employee domain.Employee) (domain.Employee, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO employees (employee_id, name, role, active)
		VALUES ($1, $2, $3, TRUE)
		RETURNING employee_id, name, role, active
	`, employee.EmployeeID, employee.Name, employee.Role)
	return scanEmployee(row)
}

func (r *EmployeeRepository) Update(ctx context.Context, employeeID, name string, role domain.Role) (domain.Employee, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE employees
		SET name = $2, role = $3, updated_at = now()
		WHERE employee_id = $1
		RETURNING employee_id, name, role, active
	`, employeeID, name, role)
	return scanEmployee(row)
}

func (r *EmployeeRepository) SetActive(ctx context.Context, employeeID string, active bool) (domain.Employee, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE employees
		SET active = $2, updated_at = now()
		WHERE employee_id = $1
		RETURNING employee_id, name, role, active
	`, employeeID, active)
	return scanEmployee(row)
}

type employeeRow interface {
	Scan(dest ...any) error
}

func scanEmployee(row employeeRow) (domain.Employee, error) {
	var employee domain.Employee
	var role string
	if err := row.Scan(&employee.EmployeeID, &employee.Name, &role, &employee.Active); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Employee{}, port.ErrEmployeeNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Employee{}, port.ErrEmployeeConflict
		}
		return domain.Employee{}, fmt.Errorf("scan employee: %w", err)
	}
	employee.Role = domain.Role(role)
	return employee, nil
}

// Availability is bounded to [from, to] so a solve over a 28-day horizon
// never pulls the whole table.
func (r *EmployeeRepository) Availability(ctx context.Context, from, to domain.Date) (domain.AvailabilityMap, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT employee_id, avail_date, sang, dem
		FROM employee_availability
		WHERE avail_date BETWEEN $1 AND $2
		ORDER BY employee_id, avail_date
	`, from.Time, to.Time)
	if err != nil {
		return nil, fmt.Errorf("query availability: %w", err)
	}
	defer rows.Close()

	result := domain.AvailabilityMap{}
	for rows.Next() {
		var employeeID string
		var availDate domain.Date
		var sang, dem bool
		if err := rows.Scan(&employeeID, &availDate.Time, &sang, &dem); err != nil {
			return nil, fmt.Errorf("scan availability: %w", err)
		}
		if _, ok := result[employeeID]; !ok {
			result[employeeID] = map[domain.Date]domain.ShiftAvailability{}
		}
		result[employeeID][availDate] = domain.ShiftAvailability{Sang: sang, Dem: dem}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate availability: %w", err)
	}
	return result, nil
}

// SetAvailability upserts one (employee, date) availability row. FK
// violations (unknown employee_id) surface as-is to the caller.
func (r *EmployeeRepository) SetAvailability(ctx context.Context, employeeID string, date domain.Date, avail domain.ShiftAvailability) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO employee_availability (employee_id, avail_date, sang, dem)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (employee_id, avail_date) DO UPDATE SET sang = $3, dem = $4
	`, employeeID, date.Time, avail.Sang, avail.Dem)
	if err != nil {
		return fmt.Errorf("upsert availability: %w", err)
	}
	return nil
}

// SetLeaveDay adds (onLeave=true) or removes (onLeave=false) one leave day.
// Adding is idempotent (ON CONFLICT DO NOTHING); removing a day that was
// never a leave day is a no-op, not an error.
func (r *EmployeeRepository) SetLeaveDay(ctx context.Context, employeeID string, date domain.Date, onLeave bool) error {
	if onLeave {
		_, err := r.pool.Exec(ctx, `
			INSERT INTO employee_leave_days (employee_id, leave_date)
			VALUES ($1, $2)
			ON CONFLICT (employee_id, leave_date) DO NOTHING
		`, employeeID, date.Time)
		if err != nil {
			return fmt.Errorf("insert leave day: %w", err)
		}
		return nil
	}

	_, err := r.pool.Exec(ctx, `
		DELETE FROM employee_leave_days WHERE employee_id = $1 AND leave_date = $2
	`, employeeID, date.Time)
	if err != nil {
		return fmt.Errorf("delete leave day: %w", err)
	}
	return nil
}
