-- name: ListEmployees :many
SELECT employee_id, name, role, active
FROM employees
WHERE active OR sqlc.arg(include_inactive)::boolean
ORDER BY employee_id;

-- name: ListEmployeeLeaveDays :many
SELECT employee_id, leave_date
FROM employee_leave_days
ORDER BY employee_id, leave_date;

-- name: CreateEmployee :one
INSERT INTO employees (employee_id, name, role, active)
VALUES ($1, $2, $3, TRUE)
RETURNING employee_id, name, role, active;

-- name: UpdateEmployee :one
UPDATE employees
SET name = $2, role = $3, updated_at = now()
WHERE employee_id = $1
RETURNING employee_id, name, role, active;

-- name: SetEmployeeActive :one
UPDATE employees
SET active = $2, updated_at = now()
WHERE employee_id = $1
RETURNING employee_id, name, role, active;

-- name: GetAvailability :many
SELECT employee_id, avail_date, sang, dem
FROM employee_availability
WHERE avail_date BETWEEN sqlc.arg(from_date)::date AND sqlc.arg(to_date)::date
ORDER BY employee_id, avail_date;

-- name: UpsertAvailability :exec
INSERT INTO employee_availability (employee_id, avail_date, sang, dem)
VALUES ($1, $2, $3, $4)
ON CONFLICT (employee_id, avail_date) DO UPDATE SET sang = $3, dem = $4;

-- name: InsertLeaveDay :exec
INSERT INTO employee_leave_days (employee_id, leave_date)
VALUES ($1, $2)
ON CONFLICT (employee_id, leave_date) DO NOTHING;

-- name: DeleteLeaveDay :exec
DELETE FROM employee_leave_days WHERE employee_id = $1 AND leave_date = $2;
