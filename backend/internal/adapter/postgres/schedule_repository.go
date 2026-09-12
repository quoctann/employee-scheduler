package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/tantq/employee-scheduler-backend/internal/adapter/postgres/sqlc"
	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

// approvedBy is a placeholder identity for the demo scope — there is no
// login/auth yet, so every approval is attributed to a single fixed actor.
const approvedBy = "demo-user"

type ScheduleRepository struct {
	pool    *pgxpool.Pool
	queries *sqlcgen.Queries
}

func NewScheduleRepository(pool *pgxpool.Pool) *ScheduleRepository {
	return &ScheduleRepository{pool: pool, queries: sqlcgen.New(pool)}
}

var _ port.ScheduleRepository = (*ScheduleRepository)(nil)

// SaveRun persists a solve result as a new, immutable row set: one
// schedule_runs row plus its assignments/shortages/employee_summary, all in
// one transaction so a partial write can never be read back. The header row
// goes through sqlc; the three child-table inserts stay hand-written
// pgx.Batch (a dynamic-length, no-conflict bulk insert sqlc doesn't
// generate cleanly).
func (r *ScheduleRepository) SaveRun(ctx context.Context, result domain.SolveResult) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	runID, err := r.queries.WithTx(tx).InsertScheduleRun(ctx, sqlcgen.InsertScheduleRunParams{
		StartDate:      result.StartDate.Time,
		NumDays:        int32(result.NumDays),
		Status:         string(result.Status),
		ObjectiveValue: result.ObjectiveValue,
		WallTimeS:      result.WallTimeS,
	})
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
	header, err := r.queries.GetLatestScheduleRun(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SolveResult{}, false, nil
		}
		return domain.SolveResult{}, false, fmt.Errorf("query latest schedule_run: %w", err)
	}

	result := domain.SolveResult{
		RunID:          header.ID,
		StartDate:      domain.Date{Time: header.StartDate},
		NumDays:        int(header.NumDays),
		Status:         domain.SolveStatus(header.Status),
		ObjectiveValue: header.ObjectiveValue,
		WallTimeS:      header.WallTimeS,
		// Pre-allocated as empty (not nil) so JSON encodes `[]`, matching what
		// a fresh solverclient response always contains — GET /schedule/latest
		// and POST /schedule/solve must not disagree on null-vs-empty.
		Schedule:        []domain.ScheduleEntry{},
		Shortages:       []domain.ShortageItem{},
		EmployeeSummary: []domain.EmployeeSummary{},
	}

	assignments, err := r.queries.ListScheduleAssignmentsByRun(ctx, header.ID)
	if err != nil {
		return domain.SolveResult{}, false, fmt.Errorf("query schedule_assignments: %w", err)
	}
	for _, a := range assignments {
		result.Schedule = append(result.Schedule, domain.ScheduleEntry{
			EmployeeID: a.EmployeeID,
			Date:       domain.Date{Time: a.AssignmentDate},
			Gate:       a.Gate,
			Shift:      domain.ShiftType(a.Shift),
		})
	}

	shortages, err := r.queries.ListScheduleShortagesByRun(ctx, header.ID)
	if err != nil {
		return domain.SolveResult{}, false, fmt.Errorf("query schedule_shortages: %w", err)
	}
	for _, s := range shortages {
		result.Shortages = append(result.Shortages, domain.ShortageItem{
			Date:         domain.Date{Time: s.ShortageDate},
			Gate:         s.Gate,
			Shift:        domain.ShiftType(s.Shift),
			ShortageType: domain.ShortageType(s.ShortageType),
			Missing:      int(s.Missing),
		})
	}

	summaries, err := r.queries.ListScheduleEmployeeSummaryByRun(ctx, header.ID)
	if err != nil {
		return domain.SolveResult{}, false, fmt.Errorf("query schedule_employee_summary: %w", err)
	}
	for _, s := range summaries {
		result.EmployeeSummary = append(result.EmployeeSummary, domain.EmployeeSummary{
			EmployeeID:     s.EmployeeID,
			Role:           domain.Role(s.Role),
			LeaveDays:      int(s.LeaveDays),
			TargetHours:    int(s.TargetHours),
			ActualHours:    int(s.ActualHours),
			DeviationHours: int(s.DeviationHours),
			ActualShifts:   int(s.ActualShifts),
		})
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
	hasApproved, err := r.queries.HasApprovedAssignmentsOnDate(ctx, beforeDate.Time)
	if err != nil {
		return domain.CarryIn{}, fmt.Errorf("check approved_assignments existence: %w", err)
	}

	if hasApproved {
		approvedIDs, err := r.queries.ListApprovedNightWorkers(ctx, beforeDate.Time)
		if err != nil {
			return domain.CarryIn{}, fmt.Errorf("query approved carry-in: %w", err)
		}
		return domain.CarryIn{WorkedNightBeforeStart: approvedIDs}, nil
	}

	fallbackIDs, err := r.queries.ListFallbackNightWorkers(ctx, beforeDate.Time)
	if err != nil {
		return domain.CarryIn{}, fmt.Errorf("query fallback carry-in: %w", err)
	}
	return domain.CarryIn{WorkedNightBeforeStart: fallbackIDs}, nil
}

// ApproveAssignments upserts the manager-confirmed state for each cell. Kept
// as hand-written pgx.Batch (a dynamic-length, per-row ON CONFLICT DO UPDATE
// upsert sqlc doesn't generate cleanly), same reasoning as SaveRun.
func (r *ScheduleRepository) ApproveAssignments(ctx context.Context, assignments []domain.LockedAssignment) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

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
	rows, err := r.queries.ListApprovedAssignments(ctx, sqlcgen.ListApprovedAssignmentsParams{
		AssignmentDate:   from.Time,
		AssignmentDate_2: to.Time,
	})
	if err != nil {
		return nil, fmt.Errorf("query approved_assignments: %w", err)
	}

	result := make([]domain.LockedAssignment, 0, len(rows))
	for _, row := range rows {
		a := domain.LockedAssignment{
			EmployeeID: row.EmployeeID,
			Date:       domain.Date{Time: row.AssignmentDate},
			Gate:       row.Gate,
			Off:        row.Off,
		}
		if row.Shift != nil {
			st := domain.ShiftType(*row.Shift)
			a.Shift = &st
		}
		result = append(result, a)
	}
	return result, nil
}

// UnapproveAssignments deletes approved cells in [from, to] so a future
// solve is free to reassign them, and returns how many rows were removed.
func (r *ScheduleRepository) UnapproveAssignments(ctx context.Context, from, to domain.Date) (int, error) {
	rowsAffected, err := r.queries.DeleteApprovedAssignments(ctx, sqlcgen.DeleteApprovedAssignmentsParams{
		AssignmentDate:   from.Time,
		AssignmentDate_2: to.Time,
	})
	if err != nil {
		return 0, fmt.Errorf("delete approved_assignments: %w", err)
	}
	return int(rowsAffected), nil
}
