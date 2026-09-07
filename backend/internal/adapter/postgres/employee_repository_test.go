package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

// currentDateForTest mirrors CURRENT_DATE as seen by the migration that
// seeded availability rows — safe to assume "today" in the same run.
func currentDateForTest() domain.Date {
	return domain.NewDate(time.Now())
}

func TestEmployeeRepository_List_ReturnsSeededRoster(t *testing.T) {
	pool := testPool(t)
	repo := NewEmployeeRepository(pool)

	employees, err := repo.List(context.Background(), false)
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
	employees, err := repo.List(ctx, false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !hasLeaveDay(employees, "NV01", date) {
		t.Fatalf("expected NV01 to have leave day %v after SetLeaveDay(true)", date)
	}

	if err := repo.SetLeaveDay(ctx, "NV01", date, false); err != nil {
		t.Fatalf("SetLeaveDay(false) error = %v", err)
	}
	employees, err = repo.List(ctx, false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if hasLeaveDay(employees, "NV01", date) {
		t.Fatalf("expected NV01 to no longer have leave day %v after SetLeaveDay(false)", date)
	}
}

func TestEmployeeRepository_ManagesEmployeeLifecycle(t *testing.T) {
	pool := testPool(t)
	repo := NewEmployeeRepository(pool)
	ctx := context.Background()
	id := fmt.Sprintf("TEST%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM employees WHERE employee_id = $1`, id)
	})

	created, err := repo.Create(ctx, domain.Employee{EmployeeID: id, Name: "Test Employee", Role: domain.RoleNV})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !created.Active || created.Name != "Test Employee" {
		t.Fatalf("Create() = %+v", created)
	}
	if _, err := repo.Create(ctx, created); !errors.Is(err, port.ErrEmployeeConflict) {
		t.Fatalf("duplicate Create() error = %v, want ErrEmployeeConflict", err)
	}

	updated, err := repo.Update(ctx, id, "Updated Employee", domain.RoleTC)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "Updated Employee" || updated.Role != domain.RoleTC {
		t.Fatalf("Update() = %+v", updated)
	}
	if _, err := repo.Update(ctx, "MISSING", "Nobody", domain.RoleNV); !errors.Is(err, port.ErrEmployeeNotFound) {
		t.Fatalf("missing Update() error = %v, want ErrEmployeeNotFound", err)
	}

	if _, err := repo.SetActive(ctx, id, false); err != nil {
		t.Fatalf("SetActive(false) error = %v", err)
	}
	active, err := repo.List(ctx, false)
	if err != nil {
		t.Fatalf("List(false) error = %v", err)
	}
	if hasEmployee(active, id) {
		t.Fatalf("List(false) unexpectedly contains inactive employee %s", id)
	}
	all, err := repo.List(ctx, true)
	if err != nil {
		t.Fatalf("List(true) error = %v", err)
	}
	if employee, ok := findEmployee(all, id); !ok || employee.Active {
		t.Fatalf("List(true) did not return inactive employee: %+v", employee)
	}
	if restored, err := repo.SetActive(ctx, id, true); err != nil || !restored.Active {
		t.Fatalf("SetActive(true) = %+v, %v", restored, err)
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

func hasEmployee(employees []domain.Employee, employeeID string) bool {
	_, ok := findEmployee(employees, employeeID)
	return ok
}

func findEmployee(employees []domain.Employee, employeeID string) (domain.Employee, bool) {
	for _, employee := range employees {
		if employee.EmployeeID == employeeID {
			return employee, true
		}
	}
	return domain.Employee{}, false
}
