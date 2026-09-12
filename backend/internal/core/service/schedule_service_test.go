package service

import (
	"context"
	"errors"
	"testing"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
)

func mustDate(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return d
}

func TestScheduleService_Solve_BuildsRequestFromRepositoriesAndPersistsResult(t *testing.T) {
	// Arrange
	start := mustDate(t, "2026-09-07")
	employees := []domain.Employee{{EmployeeID: "NV01", Name: "NV01", Role: domain.RoleNV}}
	availability := domain.AvailabilityMap{"NV01": {start: {Sang: true, Dem: true}}}
	config := domain.SolverConfig{TargetHoursPerWeek: 44}
	locked := []domain.LockedAssignment{{EmployeeID: "NV01", Date: start, Off: true}}
	carryIn := domain.CarryIn{WorkedNightBeforeStart: []string{"NV01"}}

	solver := &fakeSolverGateway{solveResult: domain.SolveResult{Status: domain.StatusOptimal}}
	empRepo := &fakeEmployeeRepository{employees: employees, availability: availability}
	cfgRepo := &fakeConfigRepository{config: config}
	schedRepo := &fakeScheduleRepository{
		approvedAssignments: locked,
		carryIn:             carryIn,
		saveRunID:           42,
	}

	svc := NewScheduleService(solver, empRepo, cfgRepo, schedRepo)

	// Act
	result, err := svc.Solve(context.Background(), SolveParams{StartDate: start, NumDays: 3, TimeLimitS: 5})

	// Assert
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}
	if solver.solveReq.NumDays != 3 || solver.solveReq.TimeLimitS != 5 {
		t.Fatalf("unexpected solve request: %+v", solver.solveReq)
	}
	if len(solver.solveReq.Employees) != 1 || solver.solveReq.Employees[0].EmployeeID != "NV01" {
		t.Fatalf("expected employees from repository, got %+v", solver.solveReq.Employees)
	}
	if len(solver.solveReq.LockedAssignments) != 1 {
		t.Fatalf("expected approved assignments to be forwarded as locked_assignments, got %+v", solver.solveReq.LockedAssignments)
	}
	if len(solver.solveReq.CarryIn.WorkedNightBeforeStart) != 1 {
		t.Fatalf("expected carry_in to be forwarded, got %+v", solver.solveReq.CarryIn)
	}
	wantEnd := mustDate(t, "2026-09-09")
	if empRepo.availabilityFrom != start || empRepo.availabilityTo != wantEnd {
		t.Fatalf("expected availability window [%v,%v], got [%v,%v]", start, wantEnd, empRepo.availabilityFrom, empRepo.availabilityTo)
	}
	if schedRepo.saveRunResult.StartDate != start || schedRepo.saveRunResult.NumDays != 3 {
		t.Fatalf("expected saved run to be stamped with the request horizon, got %+v", schedRepo.saveRunResult)
	}
	if result.RunID != 42 {
		t.Fatalf("expected RunID from SaveRun, got %d", result.RunID)
	}
}

func TestScheduleService_Solve_DefaultsTimeLimit(t *testing.T) {
	solver := &fakeSolverGateway{}
	svc := NewScheduleService(solver, &fakeEmployeeRepository{}, &fakeConfigRepository{}, &fakeScheduleRepository{})

	if _, err := svc.Solve(context.Background(), SolveParams{StartDate: mustDate(t, "2026-09-07"), NumDays: 1}); err != nil {
		t.Fatalf("Solve() error = %v", err)
	}
	if solver.solveReq.TimeLimitS != defaultTimeLimitS {
		t.Fatalf("expected default time limit %d, got %d", defaultTimeLimitS, solver.solveReq.TimeLimitS)
	}
}

func TestScheduleService_Solve_RejectsInvalidNumDays(t *testing.T) {
	svc := NewScheduleService(&fakeSolverGateway{}, &fakeEmployeeRepository{}, &fakeConfigRepository{}, &fakeScheduleRepository{})
	if _, err := svc.Solve(context.Background(), SolveParams{StartDate: mustDate(t, "2026-09-07"), NumDays: 0}); err == nil {
		t.Fatal("expected error for num_days=0, got nil")
	}
}

func TestScheduleService_Solve_IgnoreApproved_SkipsLockedAssignments(t *testing.T) {
	start := mustDate(t, "2026-09-07")
	locked := []domain.LockedAssignment{{EmployeeID: "NV01", Date: start, Off: true}}

	solver := &fakeSolverGateway{}
	schedRepo := &fakeScheduleRepository{approvedAssignments: locked}
	svc := NewScheduleService(solver, &fakeEmployeeRepository{}, &fakeConfigRepository{}, schedRepo)

	if _, err := svc.Solve(context.Background(), SolveParams{StartDate: start, NumDays: 1, IgnoreApproved: true}); err != nil {
		t.Fatalf("Solve() error = %v", err)
	}
	if len(solver.solveReq.LockedAssignments) != 0 {
		t.Fatalf("expected no locked assignments when IgnoreApproved=true, got %+v", solver.solveReq.LockedAssignments)
	}
}

func TestScheduleService_Solve_PropagatesSolverError(t *testing.T) {
	wantErr := errors.New("solver at capacity")
	solver := &fakeSolverGateway{solveErr: wantErr}
	schedRepo := &fakeScheduleRepository{}
	svc := NewScheduleService(solver, &fakeEmployeeRepository{}, &fakeConfigRepository{}, schedRepo)

	_, err := svc.Solve(context.Background(), SolveParams{StartDate: mustDate(t, "2026-09-07"), NumDays: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
	if schedRepo.saveRunCalled {
		t.Fatalf("expected SaveRun not to be called on solver error, got %+v", schedRepo.saveRunResult)
	}
}

func TestSanitizeLockedAssignments(t *testing.T) {
	day := mustDate(t, "2026-09-10")
	otherDay := mustDate(t, "2026-09-11")
	gateA := "A"
	shiftDem := domain.ShiftDem
	shiftSang := domain.ShiftSang

	tests := []struct {
		name         string
		locked       domain.LockedAssignment
		employees    []domain.Employee
		availability domain.AvailabilityMap
		wantOff      bool
	}{
		{
			name:         "off lock is untouched regardless of availability",
			locked:       domain.LockedAssignment{EmployeeID: "NV01", Date: day, Off: true},
			availability: domain.AvailabilityMap{},
			wantOff:      true,
		},
		{
			name:         "still available for the locked shift stays unchanged",
			locked:       domain.LockedAssignment{EmployeeID: "NV01", Date: day, Gate: &gateA, Shift: &shiftDem},
			availability: domain.AvailabilityMap{"NV01": {day: {Dem: true}}},
			wantOff:      false,
		},
		{
			name:         "no longer available for the locked shift is downgraded to off",
			locked:       domain.LockedAssignment{EmployeeID: "NV01", Date: day, Gate: &gateA, Shift: &shiftDem},
			availability: domain.AvailabilityMap{"NV01": {day: {Sang: true, Dem: false}}},
			wantOff:      true,
		},
		{
			name:         "no availability row at all is downgraded to off",
			locked:       domain.LockedAssignment{EmployeeID: "NV01", Date: day, Gate: &gateA, Shift: &shiftSang},
			availability: domain.AvailabilityMap{},
			wantOff:      true,
		},
		{
			name:         "employee now on leave that day is downgraded to off even if availability still says yes",
			locked:       domain.LockedAssignment{EmployeeID: "NV01", Date: day, Gate: &gateA, Shift: &shiftDem},
			employees:    []domain.Employee{{EmployeeID: "NV01", LeaveDays: []domain.Date{day}}},
			availability: domain.AvailabilityMap{"NV01": {day: {Dem: true}}},
			wantOff:      true,
		},
		{
			name:         "leave on a different day does not affect this lock",
			locked:       domain.LockedAssignment{EmployeeID: "NV01", Date: otherDay, Gate: &gateA, Shift: &shiftDem},
			employees:    []domain.Employee{{EmployeeID: "NV01", LeaveDays: []domain.Date{day}}},
			availability: domain.AvailabilityMap{"NV01": {otherDay: {Dem: true}}},
			wantOff:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			employees := tt.employees
			if employees == nil {
				employees = []domain.Employee{{EmployeeID: "NV01"}}
			}
			got := sanitizeLockedAssignments([]domain.LockedAssignment{tt.locked}, employees, tt.availability)
			if len(got) != 1 {
				t.Fatalf("expected 1 result, got %d", len(got))
			}
			if got[0].Off != tt.wantOff {
				t.Fatalf("sanitizeLockedAssignments() off = %v, want %v (result: %+v)", got[0].Off, tt.wantOff, got[0])
			}
			if tt.wantOff && (got[0].Gate != nil || got[0].Shift != nil) {
				t.Fatalf("downgraded lock must clear gate/shift, got %+v", got[0])
			}
			if err := got[0].Validate(); err != nil {
				t.Fatalf("sanitized assignment fails its own shape invariant: %v", err)
			}
		})
	}
}

func TestScheduleService_FiltersInactiveEmployeeState(t *testing.T) {
	start := mustDate(t, "2026-09-07")
	gate := "A"
	shift := domain.ShiftSang
	active := []domain.Employee{{EmployeeID: "NV01", Role: domain.RoleNV}}
	locked := []domain.LockedAssignment{
		{EmployeeID: "NV01", Date: start, Gate: &gate, Shift: &shift},
		{EmployeeID: "NV02", Date: start, Off: true},
	}
	solver := &fakeSolverGateway{}
	svc := NewScheduleService(
		solver,
		&fakeEmployeeRepository{employees: active, availability: domain.AvailabilityMap{"NV01": {start: {Sang: true}}}},
		&fakeConfigRepository{},
		&fakeScheduleRepository{approvedAssignments: locked, carryIn: domain.CarryIn{WorkedNightBeforeStart: []string{"NV01", "NV02"}}},
	)

	if _, err := svc.Solve(context.Background(), SolveParams{StartDate: start, NumDays: 1}); err != nil {
		t.Fatalf("Solve() error = %v", err)
	}
	if len(solver.solveReq.LockedAssignments) != 1 || solver.solveReq.LockedAssignments[0].EmployeeID != "NV01" {
		t.Fatalf("inactive locks were not removed: %+v", solver.solveReq.LockedAssignments)
	}
	if got := solver.solveReq.CarryIn.WorkedNightBeforeStart; len(got) != 1 || got[0] != "NV01" {
		t.Fatalf("inactive carry-in was not removed: %+v", got)
	}
}

func TestScheduleService_Latest_ReturnsRepositoryResult(t *testing.T) {
	want := domain.SolveResult{RunID: 7, Status: domain.StatusFeasible}
	schedRepo := &fakeScheduleRepository{latestResult: want, latestFound: true}
	svc := NewScheduleService(&fakeSolverGateway{}, &fakeEmployeeRepository{}, &fakeConfigRepository{}, schedRepo)

	got, found, err := svc.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if !found || got.RunID != want.RunID || got.Status != want.Status {
		t.Fatalf("Latest() = %+v, %v, want %+v, true", got, found, want)
	}
}
