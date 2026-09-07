package service

import (
	"context"
	"testing"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

func TestCandidateService_Suggest_UsesLatestRunWindowWhenAvailable(t *testing.T) {
	// Arrange
	runStart := mustDate(t, "2026-09-01")
	slotDate := mustDate(t, "2026-09-09")
	latest := domain.SolveResult{
		StartDate: runStart,
		NumDays:   28,
		Schedule:  []domain.ScheduleEntry{{EmployeeID: "NV01", Date: slotDate, Gate: "A", Shift: domain.ShiftSang}},
	}
	solver := &fakeSolverGateway{}
	schedRepo := &fakeScheduleRepository{latestResult: latest, latestFound: true}
	svc := NewCandidateService(solver, &fakeEmployeeRepository{}, &fakeConfigRepository{}, schedRepo)

	// Act
	_, err := svc.Suggest(context.Background(), CandidateParams{
		TargetSlot: domain.TargetSlot{Date: slotDate, Gate: "A", Shift: domain.ShiftSang},
	})

	// Assert
	if err != nil {
		t.Fatalf("Suggest() error = %v", err)
	}
	if solver.candidatesReq.StartDate != runStart || solver.candidatesReq.NumDays != 28 {
		t.Fatalf("expected request window to match latest run [%v,%d], got [%v,%d]",
			runStart, 28, solver.candidatesReq.StartDate, solver.candidatesReq.NumDays)
	}
	if len(solver.candidatesReq.CurrentSchedule) != 1 {
		t.Fatalf("expected current_schedule from latest run, got %+v", solver.candidatesReq.CurrentSchedule)
	}
	if solver.candidatesReq.TopN != defaultTopN {
		t.Fatalf("expected default top_n %d, got %d", defaultTopN, solver.candidatesReq.TopN)
	}
}

func TestCandidateService_Suggest_DefaultsToSingleDayWindowWithoutAnyRun(t *testing.T) {
	slotDate := mustDate(t, "2026-09-09")
	solver := &fakeSolverGateway{}
	schedRepo := &fakeScheduleRepository{latestFound: false}
	svc := NewCandidateService(solver, &fakeEmployeeRepository{}, &fakeConfigRepository{}, schedRepo)

	if _, err := svc.Suggest(context.Background(), CandidateParams{
		TargetSlot: domain.TargetSlot{Date: slotDate, Gate: "A", Shift: domain.ShiftSang},
	}); err != nil {
		t.Fatalf("Suggest() error = %v", err)
	}

	if solver.candidatesReq.StartDate != slotDate || solver.candidatesReq.NumDays != 1 {
		t.Fatalf("expected window to fall back to [target_date, 1 day], got [%v,%d]", solver.candidatesReq.StartDate, solver.candidatesReq.NumDays)
	}
	if len(solver.candidatesReq.CurrentSchedule) != 0 {
		t.Fatalf("expected empty current_schedule with no prior run, got %+v", solver.candidatesReq.CurrentSchedule)
	}
}

func TestCandidateService_Suggest_HonorsExplicitTopN(t *testing.T) {
	solver := &fakeSolverGateway{}
	svc := NewCandidateService(solver, &fakeEmployeeRepository{}, &fakeConfigRepository{}, &fakeScheduleRepository{})

	if _, err := svc.Suggest(context.Background(), CandidateParams{
		TargetSlot: domain.TargetSlot{Date: mustDate(t, "2026-09-09"), Gate: "A", Shift: domain.ShiftSang},
		TopN:       10,
	}); err != nil {
		t.Fatalf("Suggest() error = %v", err)
	}
	if solver.candidatesReq.TopN != 10 {
		t.Fatalf("expected explicit top_n 10, got %d", solver.candidatesReq.TopN)
	}
}
