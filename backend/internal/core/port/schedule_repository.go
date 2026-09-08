package port

import (
	"context"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

// ScheduleRepository is the outbound port owning everything solver-service
// itself refuses to store: solve history, the manager-approved/locked cells,
// and the rolling-horizon carry-in derived from them.
type ScheduleRepository interface {
	// SaveRun persists a solve result (schedule_runs + assignments + shortages
	// + employee_summary) as a new, immutable row set and returns its run id.
	SaveRun(ctx context.Context, result domain.SolveResult) (int64, error)

	// LatestRun returns the most recently saved run, if any.
	LatestRun(ctx context.Context) (domain.SolveResult, bool, error)

	// CarryIn returns who worked the night shift on beforeDate, derived from
	// ApprovedAssignments (falling back to the latest run if nothing has been
	// approved yet) — never stored separately, so it can't drift out of sync.
	CarryIn(ctx context.Context, beforeDate domain.Date) (domain.CarryIn, error)

	// ApproveAssignments upserts the manager-confirmed state for each cell and
	// returns how many rows were written.
	ApproveAssignments(ctx context.Context, assignments []domain.LockedAssignment) (int, error)

	// ApprovedAssignments returns the currently-approved cells in [from, to],
	// used to populate LockedAssignments on the next solve for that horizon.
	ApprovedAssignments(ctx context.Context, from, to domain.Date) ([]domain.LockedAssignment, error)

	// UnapproveAssignments deletes approved cells in [from, to] so a future
	// solve is free to reassign them, and returns how many rows were removed.
	UnapproveAssignments(ctx context.Context, from, to domain.Date) (int, error)
}
