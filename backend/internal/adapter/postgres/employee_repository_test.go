package postgres

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

// currentDateForTest mirrors CURRENT_DATE as seen by the migration that
// seeded availability rows — safe to assume "today" in the same run.
func currentDateForTest() domain.Date {
	return domain.NewDate(time.Now())
}

func TestEmployeeRepository_List_ReturnsSeededRoster(t *testing.T) {
	pool := testPool(t)
	repo := NewEmployeeRepository(pool)

	employees, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(employees) != 21 {
		t.Fatalf("List() returned %d employees, want 21 (seed dataset)", len(employees))
	}

	var nv08 *domain.Employee
	for i := range employees {
		if employees[i].EmployeeID == "NV08" {
			nv08 = &employees[i]
		}
	}
	if nv08 == nil {
		t.Fatal("expected seeded employee NV08 to be present")
	}
	if len(nv08.LeaveDays) != 5 {
		t.Fatalf("expected NV08 to have 5 seeded leave days, got %d", len(nv08.LeaveDays))
	}
}

func TestEmployeeRepository_Availability_IsBoundedToRequestedWindow(t *testing.T) {
	pool := testPool(t)
	repo := NewEmployeeRepository(pool)

	from := currentDateForTest()
	to := from.AddDays(2) // 3-day window: day 0,1,2

	avail, err := repo.Availability(context.Background(), from, to)
	if err != nil {
		t.Fatalf("Availability() error = %v", err)
	}
	if len(avail) == 0 {
		t.Fatal("expected non-empty availability for the seeded window")
	}
	for employeeID, byDate := range avail {
		for d := range byDate {
			if d.Time.Before(from.Time) || d.Time.After(to.Time) {
				t.Fatalf("Availability() returned out-of-window date %v for %s (window [%v,%v])", d, employeeID, from, to)
			}
		}
	}
}

func TestEmployeeRepository_SetAvailability_UpsertsAndOverwrites(t *testing.T) {
	pool := testPool(t)
	repo := NewEmployeeRepository(pool)
	ctx := context.Background()
	date := currentDateForTest().AddDays(90) // far enough out to avoid seeded rows

	if err := repo.SetAvailability(ctx, "NV01", date, domain.ShiftAvailability{Sang: true, Dem: false}); err != nil {
		t.Fatalf("SetAvailability() error = %v", err)
	}
	avail, err := repo.Availability(ctx, date, date)
	if err != nil {
		t.Fatalf("Availability() error = %v", err)
	}
	if got := avail["NV01"][date]; got != (domain.ShiftAvailability{Sang: true, Dem: false}) {
		t.Fatalf("Availability() = %+v, want {Sang:true, Dem:false}", got)
	}

	// Overwrite the same day — must update, not duplicate.
	if err := repo.SetAvailability(ctx, "NV01", date, domain.ShiftAvailability{Sang: false, Dem: true}); err != nil {
		t.Fatalf("SetAvailability() overwrite error = %v", err)
	}
	avail, err = repo.Availability(ctx, date, date)
	if err != nil {
		t.Fatalf("Availability() error = %v", err)
	}
	if got := avail["NV01"][date]; got != (domain.ShiftAvailability{Sang: false, Dem: true}) {
		t.Fatalf("Availability() after overwrite = %+v, want {Sang:false, Dem:true}", got)
	}
}

func TestEmployeeRepository_SetLeaveDay_AddsAndRemoves(t *testing.T) {
	pool := testPool(t)
	repo := NewEmployeeRepository(pool)
	ctx := context.Background()
	date := currentDateForTest().AddDays(91)

	if err := repo.SetLeaveDay(ctx, "NV01", date, true); err != nil {
		t.Fatalf("SetLeaveDay(true) error = %v", err)
	}
	// Adding twice must not error (idempotent) or duplicate the row.
	if err := repo.SetLeaveDay(ctx, "NV01", date, true); err != nil {
		t.Fatalf("SetLeaveDay(true) second call error = %v", err)
	}
	employees, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !hasLeaveDay(employees, "NV01", date) {
		t.Fatalf("expected NV01 to have leave day %v after SetLeaveDay(true)", date)
	}

	if err := repo.SetLeaveDay(ctx, "NV01", date, false); err != nil {
		t.Fatalf("SetLeaveDay(false) error = %v", err)
	}
	employees, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if hasLeaveDay(employees, "NV01", date) {
		t.Fatalf("expected NV01 to no longer have leave day %v after SetLeaveDay(false)", date)
	}
}

func hasLeaveDay(employees []domain.Employee, employeeID string, date domain.Date) bool {
	for _, e := range employees {
		if e.EmployeeID == employeeID && slices.Contains(e.LeaveDays, date) {
			return true
		}
	}
	return false
}
