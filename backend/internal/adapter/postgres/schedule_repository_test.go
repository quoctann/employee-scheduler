package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

func TestScheduleRepository_SaveRunThenLatestRun_RoundTrips(t *testing.T) {
	pool := testPool(t)
	repo := NewScheduleRepository(pool)
	ctx := context.Background()

	start := currentDateForTest()
	objective := 42.5
	result := domain.SolveResult{
		StartDate:      start,
		NumDays:        1,
		Status:         domain.StatusOptimal,
		ObjectiveValue: &objective,
		WallTimeS:      0.5,
		Schedule: []domain.ScheduleEntry{
			{EmployeeID: "NV01", Date: start, Gate: "A", Shift: domain.ShiftSang},
		},
		Shortages: []domain.ShortageItem{
			{Date: start, Gate: "B", Shift: domain.ShiftDem, ShortageType: domain.ShortageLead, Missing: 1},
		},
		EmployeeSummary: []domain.EmployeeSummary{
			{EmployeeID: "NV01", Role: domain.RoleNV, LeaveDays: 0, TargetHours: 44, ActualHours: 11, DeviationHours: -33, ActualShifts: 1},
		},
	}

	runID, err := repo.SaveRun(ctx, result)
	if err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM schedule_runs WHERE id = $1`, runID); err != nil {
			t.Logf("cleanup schedule_runs id=%d failed: %v", runID, err)
		}
	})

	latest, found, err := repo.LatestRun(ctx)
	if err != nil {
		t.Fatalf("LatestRun() error = %v", err)
	}
	if !found {
		t.Fatal("LatestRun() found = false, want true")
	}
	if latest.RunID != runID {
		t.Fatalf("LatestRun() RunID = %d, want %d", latest.RunID, runID)
	}
	if len(latest.Schedule) != 1 || latest.Schedule[0].EmployeeID != "NV01" {
		t.Fatalf("LatestRun() Schedule = %+v", latest.Schedule)
	}
	if len(latest.Shortages) != 1 || latest.Shortages[0].Missing != 1 {
		t.Fatalf("LatestRun() Shortages = %+v", latest.Shortages)
	}
	if len(latest.EmployeeSummary) != 1 || latest.EmployeeSummary[0].Role != domain.RoleNV {
		t.Fatalf("LatestRun() EmployeeSummary = %+v", latest.EmployeeSummary)
	}
}

func TestScheduleRepository_ApproveAssignments_FeedsApprovedAssignmentsAndCarryIn(t *testing.T) {
	pool := testPool(t)
	repo := NewScheduleRepository(pool)
	ctx := context.Background()

	nightDate := currentDateForTest()
	shift := domain.ShiftDem
	gate := "A"

	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM approved_assignments WHERE employee_id = 'NV02' AND assignment_date = $1`, nightDate.Time); err != nil {
			t.Logf("cleanup approved_assignments failed: %v", err)
		}
	})

	count, err := repo.ApproveAssignments(ctx, []domain.LockedAssignment{
		{EmployeeID: "NV02", Date: nightDate, Gate: &gate, Shift: &shift, Off: false},
	})
	if err != nil {
		t.Fatalf("ApproveAssignments() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ApproveAssignments() count = %d, want 1", count)
	}

	approved, err := repo.ApprovedAssignments(ctx, nightDate, nightDate)
	if err != nil {
		t.Fatalf("ApprovedAssignments() error = %v", err)
	}
	if len(approved) != 1 || approved[0].EmployeeID != "NV02" || approved[0].Shift == nil || *approved[0].Shift != domain.ShiftDem {
		t.Fatalf("ApprovedAssignments() = %+v", approved)
	}

	carryIn, err := repo.CarryIn(ctx, nightDate)
	if err != nil {
		t.Fatalf("CarryIn(%v) error = %v", nightDate, err)
	}
	found := false
	for _, id := range carryIn.WorkedNightBeforeStart {
		if id == "NV02" {
			found = true
		}
	}
	if !found {
		t.Fatalf("CarryIn(%v) = %+v, expected NV02 to be present (worked dem shift that day)", nightDate, carryIn)
	}
}

// Regression test: a run with zero shortages must round-trip as an empty
// JSON array, not `null` — GET /schedule/latest and POST /schedule/solve
// must agree on this shape for the same logical field.
func TestScheduleRepository_LatestRun_EmptyShortages_EncodeAsEmptyArrayNotNull(t *testing.T) {
	pool := testPool(t)
	repo := NewScheduleRepository(pool)
	ctx := context.Background()

	start := currentDateForTest().AddDays(40)
	result := domain.SolveResult{
		StartDate: start,
		NumDays:   1,
		Status:    domain.StatusOptimal,
		WallTimeS: 0.1,
		Schedule:  []domain.ScheduleEntry{{EmployeeID: "NV01", Date: start, Gate: "A", Shift: domain.ShiftSang}},
		// Shortages and EmployeeSummary intentionally left nil/empty.
	}

	runID, err := repo.SaveRun(ctx, result)
	if err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM schedule_runs WHERE id = $1`, runID); err != nil {
			t.Logf("cleanup schedule_runs id=%d failed: %v", runID, err)
		}
	})

	latest, found, err := repo.LatestRun(ctx)
	if err != nil {
		t.Fatalf("LatestRun() error = %v", err)
	}
	if !found || latest.RunID != runID {
		t.Fatalf("LatestRun() = %+v, found=%v", latest, found)
	}

	b, err := json.Marshal(latest)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	for _, field := range []string{"shortages", "employee_summary"} {
		if string(raw[field]) != "[]" {
			t.Fatalf("field %q = %s, want []", field, raw[field])
		}
	}
}

// Regression test: approving a date where the confirmed truth is "nobody
// works nights" must not be confused with "nothing approved yet" and fall
// back to stale solver output — see CarryIn's doc comment.
func TestScheduleRepository_CarryIn_ApprovedButNoNightWorkers_ReturnsEmptyNotFallback(t *testing.T) {
	pool := testPool(t)
	repo := NewScheduleRepository(pool)
	ctx := context.Background()

	date := currentDateForTest().AddDays(20)

	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM approved_assignments WHERE employee_id = 'NV03' AND assignment_date = $1`, date.Time); err != nil {
			t.Logf("cleanup approved_assignments failed: %v", err)
		}
	})

	// NV03 is approved OFF that day — nobody is approved for a dem shift.
	if _, err := repo.ApproveAssignments(ctx, []domain.LockedAssignment{
		{EmployeeID: "NV03", Date: date, Off: true},
	}); err != nil {
		t.Fatalf("ApproveAssignments() error = %v", err)
	}

	carryIn, err := repo.CarryIn(ctx, date)
	if err != nil {
		t.Fatalf("CarryIn(%v) error = %v", date, err)
	}
	if len(carryIn.WorkedNightBeforeStart) != 0 {
		t.Fatalf("CarryIn(%v) = %+v, want empty (approved-but-nobody-worked-nights must not fall back)", date, carryIn)
	}
}
