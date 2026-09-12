-- name: InsertScheduleRun :one
INSERT INTO schedule_runs (start_date, num_days, status, objective_value, wall_time_s)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: GetLatestScheduleRun :one
SELECT id, start_date, num_days, status, objective_value, wall_time_s
FROM schedule_runs
ORDER BY created_at DESC
LIMIT 1;

-- name: ListScheduleAssignmentsByRun :many
SELECT employee_id, assignment_date, gate, shift
FROM schedule_assignments
WHERE schedule_run_id = $1
ORDER BY employee_id, assignment_date;

-- name: ListScheduleShortagesByRun :many
SELECT shortage_date, gate, shift, shortage_type, missing
FROM schedule_shortages
WHERE schedule_run_id = $1
ORDER BY shortage_date, gate, shift;

-- name: ListScheduleEmployeeSummaryByRun :many
SELECT employee_id, role, leave_days, target_hours, actual_hours, deviation_hours, actual_shifts
FROM schedule_employee_summary
WHERE schedule_run_id = $1
ORDER BY employee_id;

-- name: HasApprovedAssignmentsOnDate :one
SELECT EXISTS(SELECT 1 FROM approved_assignments WHERE assignment_date = $1);

-- name: ListApprovedNightWorkers :many
SELECT employee_id FROM approved_assignments
WHERE assignment_date = $1 AND shift = 'dem' AND NOT off;

-- name: ListFallbackNightWorkers :many
SELECT sa.employee_id
FROM schedule_assignments sa
WHERE sa.assignment_date = $1 AND sa.shift = 'dem'
  AND sa.schedule_run_id = (SELECT id FROM schedule_runs ORDER BY created_at DESC LIMIT 1);

-- name: ListApprovedAssignments :many
SELECT employee_id, assignment_date, gate, shift, off
FROM approved_assignments
WHERE assignment_date BETWEEN $1 AND $2
ORDER BY employee_id, assignment_date;

-- name: DeleteApprovedAssignments :execrows
DELETE FROM approved_assignments
WHERE assignment_date BETWEEN $1 AND $2;
