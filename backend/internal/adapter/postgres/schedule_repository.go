package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

// approvedBy is a placeholder identity for the demo scope — there is no
// login/auth yet, so every approval is attributed to a single fixed actor.
const approvedBy = "demo-user"

type ScheduleRepository struct {
	pool *pgxpool.Pool
}

func NewScheduleRepository(pool *pgxpool.Pool) *ScheduleRepository {
	return &ScheduleRepository{pool: pool}
}

var _ port.ScheduleRepository = (*ScheduleRepository)(nil)

// SaveRun persists a solve result as a new, immutable row set: one
// schedule_runs row plus its assignments/shortages/employee_summary, all in
// one transaction so a partial write can never be read back.
func (r *ScheduleRepository) SaveRun(ctx context.Context, result domain.SolveResult) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	var runID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO schedule_runs (start_date, num_days, status, objective_value, wall_time_s)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, result.StartDate.Time, result.NumDays, string(result.Status), result.ObjectiveValue, result.WallTimeS).Scan(&runID)
	if err != nil {
		return 0, fmt.Errorf("insert schedule_runs: %w", err)
	}

	// Queued as one pipelined batch (single round trip) rather than one
	// Exec per row — a 28-day/21-employee run can produce several hundred
	// assignment rows alone.
	batch := &pgx.Batch{}
	for _, entry := range result.Schedule {
		batch.Queue(`
			INSERT INTO schedule_assignments (schedule_run_id, employee_id, assignment_date, gate, shift)
			VALUES ($1, $2, $3, $4, $5)
		`, runID, entry.EmployeeID, entry.Date.Time, entry.Gate, string(entry.Shift))
	}
	for _, shortage := range result.Shortages {
		batch.Queue(`
			INSERT INTO schedule_shortages (schedule_run_id, shortage_date, gate, shift, shortage_type, missing)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, runID, shortage.Date.Time, shortage.Gate, string(shortage.Shift), string(shortage.ShortageType), shortage.Missing)
	}
	for _, summary := range result.EmployeeSummary {
		batch.Queue(`
			INSERT INTO schedule_employee_summary
				(schedule_run_id, employee_id, role, leave_days, target_hours, actual_hours, deviation_hours, actual_shifts)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, runID, summary.EmployeeID, string(summary.Role), summary.LeaveDays, summary.TargetHours, summary.ActualHours, summary.DeviationHours, summary.ActualShifts)
	}

	if err := execBatch(ctx, tx, batch); err != nil {
		return 0, fmt.Errorf("batch insert run %d children: %w", runID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}
	return runID, nil
}

// execBatch sends a pgx.Batch and consumes every queued result, so a later
// statement's error isn't left unread on the connection.
func execBatch(ctx context.Context, tx pgx.Tx, batch *pgx.Batch) error {
	if batch.Len() == 0 {
		return nil
	}
	br := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			br.Close() //nolint:errcheck // already returning the real error below
			return fmt.Errorf("statement %d/%d: %w", i+1, batch.Len(), err)
		}
	}
	return br.Close()
}

// LatestRun returns the most recently saved run (by created_at), if any.
func (r *ScheduleRepository) LatestRun(ctx context.Context) (domain.SolveResult, bool, error) {
	var result domain.SolveResult
	row := r.pool.QueryRow(ctx, `
		SELECT id, start_date, num_days, status, objective_value, wall_time_s
		FROM schedule_runs
		ORDER BY created_at DESC
		LIMIT 1
	`)
	var status string
	if err := row.Scan(&result.RunID, &result.StartDate.Time, &result.NumDays, &status, &result.ObjectiveValue, &result.WallTimeS); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SolveResult{}, false, nil
		}
		return domain.SolveResult{}, false, fmt.Errorf("query latest schedule_run: %w", err)
	}
	result.Status = domain.SolveStatus(status)
	// Pre-allocate as empty (not nil) so JSON encodes `[]`, matching what a
	// fresh solverclient response always contains — GET /schedule/latest and
	// POST /schedule/solve must not disagree on null-vs-empty for the same field.
	result.Schedule = []domain.ScheduleEntry{}
	result.Shortages = []domain.ShortageItem{}
	result.EmployeeSummary = []domain.EmployeeSummary{}

	assignmentRows, err := r.pool.Query(ctx, `
		SELECT employee_id, assignment_date, gate, shift
		FROM schedule_assignments
		WHERE schedule_run_id = $1
		ORDER BY employee_id, assignment_date
	`, result.RunID)
	if err != nil {
		return domain.SolveResult{}, false, fmt.Errorf("query schedule_assignments: %w", err)
	}
	for assignmentRows.Next() {
		var e domain.ScheduleEntry
		var shift string
		if err := assignmentRows.Scan(&e.EmployeeID, &e.Date.Time, &e.Gate, &shift); err != nil {
			assignmentRows.Close()
			return domain.SolveResult{}, false, fmt.Errorf("scan schedule_assignment: %w", err)
		}
		e.Shift = domain.ShiftType(shift)
		result.Schedule = append(result.Schedule, e)
	}
	assignmentRows.Close()
	if err := assignmentRows.Err(); err != nil {
		return domain.SolveResult{}, false, fmt.Errorf("iterate schedule_assignments: %w", err)
	}

	shortageRows, err := r.pool.Query(ctx, `
		SELECT shortage_date, gate, shift, shortage_type, missing
		FROM schedule_shortages
		WHERE schedule_run_id = $1
		ORDER BY shortage_date, gate, shift
	`, result.RunID)
	if err != nil {
		return domain.SolveResult{}, false, fmt.Errorf("query schedule_shortages: %w", err)
	}
	for shortageRows.Next() {
		var s domain.ShortageItem
		var shift, shortageType string
		if err := shortageRows.Scan(&s.Date.Time, &s.Gate, &shift, &shortageType, &s.Missing); err != nil {
			shortageRows.Close()
			return domain.SolveResult{}, false, fmt.Errorf("scan schedule_shortage: %w", err)
		}
		s.Shift = domain.ShiftType(shift)
		s.ShortageType = domain.ShortageType(shortageType)
		result.Shortages = append(result.Shortages, s)
	}
	shortageRows.Close()
	if err := shortageRows.Err(); err != nil {
		return domain.SolveResult{}, false, fmt.Errorf("iterate schedule_shortages: %w", err)
	}

	summaryRows, err := r.pool.Query(ctx, `
		SELECT employee_id, role, leave_days, target_hours, actual_hours, deviation_hours, actual_shifts
		FROM schedule_employee_summary
		WHERE schedule_run_id = $1
		ORDER BY employee_id
	`, result.RunID)
	if err != nil {
		return domain.SolveResult{}, false, fmt.Errorf("query schedule_employee_summary: %w", err)
	}
	for summaryRows.Next() {
		var s domain.EmployeeSummary
		var role string
		if err := summaryRows.Scan(&s.EmployeeID, &role, &s.LeaveDays, &s.TargetHours, &s.ActualHours, &s.DeviationHours, &s.ActualShifts); err != nil {
			summaryRows.Close()
			return domain.SolveResult{}, false, fmt.Errorf("scan schedule_employee_summary: %w", err)
		}
		s.Role = domain.Role(role)
		result.EmployeeSummary = append(result.EmployeeSummary, s)
	}
	summaryRows.Close()
	if err := summaryRows.Err(); err != nil {
		return domain.SolveResult{}, false, fmt.Errorf("iterate schedule_employee_summary: %w", err)
	}

	return result, true, nil
}

// CarryIn prefers approved_assignments (manager-confirmed truth); only if
// beforeDate has no approved cells *at all* yet does it fall back to the
// latest run's raw solved assignments. Deciding the fallback by "zero
// night-shift workers" instead (the previous implementation) would wrongly
// discard a manager's confirmed "nobody works nights that day" in favor of
// stale solver output — a legitimately-empty approved set must still win.
func (r *ScheduleRepository) CarryIn(ctx context.Context, beforeDate domain.Date) (domain.CarryIn, error) {
	var hasApproved bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM approved_assignments WHERE assignment_date = $1)
	`, beforeDate.Time).Scan(&hasApproved)
	if err != nil {
		return domain.CarryIn{}, fmt.Errorf("check approved_assignments existence: %w", err)
	}

	if hasApproved {
		approvedIDs, err := r.queryEmployeeIDs(ctx, `
			SELECT employee_id FROM approved_assignments
			WHERE assignment_date = $1 AND shift = 'dem' AND NOT off
		`, beforeDate.Time)
		if err != nil {
			return domain.CarryIn{}, fmt.Errorf("query approved carry-in: %w", err)
		}
		return domain.CarryIn{WorkedNightBeforeStart: approvedIDs}, nil
	}

	fallbackIDs, err := r.queryEmployeeIDs(ctx, `
		SELECT sa.employee_id
		FROM schedule_assignments sa
		WHERE sa.assignment_date = $1 AND sa.shift = 'dem'
		  AND sa.schedule_run_id = (SELECT id FROM schedule_runs ORDER BY created_at DESC LIMIT 1)
	`, beforeDate.Time)
	if err != nil {
		return domain.CarryIn{}, fmt.Errorf("query fallback carry-in: %w", err)
	}
	return domain.CarryIn{WorkedNightBeforeStart: fallbackIDs}, nil
}

func (r *ScheduleRepository) queryEmployeeIDs(ctx context.Context, sql string, args ...any) ([]string, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ApproveAssignments upserts the manager-confirmed state for each cell.
func (r *ScheduleRepository) ApproveAssignments(ctx context.Context, assignments []domain.LockedAssignment) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	// Pipelined as one batch (single round trip) instead of one Exec per
	// assignment, same reasoning as SaveRun's execBatch.
	batch := &pgx.Batch{}
	for _, a := range assignments {
		var shift *string
		if a.Shift != nil {
			s := string(*a.Shift)
			shift = &s
		}
		batch.Queue(`
			INSERT INTO approved_assignments (employee_id, assignment_date, gate, shift, off, approved_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (employee_id, assignment_date) DO UPDATE SET
				gate = EXCLUDED.gate,
				shift = EXCLUDED.shift,
				off = EXCLUDED.off,
				approved_at = now(),
				approved_by = EXCLUDED.approved_by
		`, a.EmployeeID, a.Date.Time, a.Gate, shift, a.Off, approvedBy)
	}

	count := 0
	br := tx.SendBatch(ctx, batch)
	for _, a := range assignments {
		tag, err := br.Exec()
		if err != nil {
			br.Close() //nolint:errcheck // already returning the real error below
			return 0, fmt.Errorf("upsert approved_assignments for %s/%s: %w", a.EmployeeID, a.Date, err)
		}
		count += int(tag.RowsAffected())
	}
	if err := br.Close(); err != nil {
		return 0, fmt.Errorf("close batch results: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}
	return count, nil
}

// ApprovedAssignments returns the currently-approved cells in [from, to].
func (r *ScheduleRepository) ApprovedAssignments(ctx context.Context, from, to domain.Date) ([]domain.LockedAssignment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT employee_id, assignment_date, gate, shift, off
		FROM approved_assignments
		WHERE assignment_date BETWEEN $1 AND $2
		ORDER BY employee_id, assignment_date
	`, from.Time, to.Time)
	if err != nil {
		return nil, fmt.Errorf("query approved_assignments: %w", err)
	}
	defer rows.Close()

	var result []domain.LockedAssignment
	for rows.Next() {
		var a domain.LockedAssignment
		var gate, shift *string
		if err := rows.Scan(&a.EmployeeID, &a.Date.Time, &gate, &shift, &a.Off); err != nil {
			return nil, fmt.Errorf("scan approved_assignment: %w", err)
		}
		a.Gate = gate
		if shift != nil {
			st := domain.ShiftType(*shift)
			a.Shift = &st
		}
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approved_assignments: %w", err)
	}
	return result, nil
}
