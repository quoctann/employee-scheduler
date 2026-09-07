package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

func TestEmployeeService_Roster_ReturnsEmployeesAndAvailability(t *testing.T) {
	repo := &fakeEmployeeRepository{
		employees:    []domain.Employee{{EmployeeID: "NV01", Name: "NV01", Role: domain.RoleNV}},
		availability: domain.AvailabilityMap{"NV01": {}},
	}
	svc := NewEmployeeService(repo)

	employees, availability, err := svc.Roster(context.Background())
	if err != nil {
		t.Fatalf("Roster() error = %v", err)
	}
	if len(employees) != 1 || employees[0].EmployeeID != "NV01" {
		t.Fatalf("Roster() employees = %+v", employees)
	}
	if availability == nil {
		t.Fatal("Roster() availability = nil")
	}
	if repo.availabilityTo.Time.Sub(repo.availabilityFrom.Time).Hours() < 27*24 {
		t.Fatalf("expected roster window to span >= 28 days, got [%v,%v]", repo.availabilityFrom, repo.availabilityTo)
	}
}

func TestEmployeeService_SetAvailability_HappyPath(t *testing.T) {
	repo := &fakeEmployeeRepository{}
	svc := NewEmployeeService(repo)
	date, _ := domain.ParseDate("2026-09-07")

	err := svc.SetAvailability(context.Background(), "NV01", date, domain.ShiftAvailability{Sang: true, Dem: false})
	if err != nil {
		t.Fatalf("SetAvailability() error = %v", err)
	}
	if len(repo.setAvailabilityCalls) != 1 || repo.setAvailabilityCalls[0] != (domain.ShiftAvailability{Sang: true}) {
		t.Fatalf("unexpected repo calls: %+v", repo.setAvailabilityCalls)
	}
}

func TestEmployeeService_SetAvailability_EmptyEmployeeID_ReturnsInvalidInput(t *testing.T) {
	repo := &fakeEmployeeRepository{}
	svc := NewEmployeeService(repo)
	date, _ := domain.ParseDate("2026-09-07")

	err := svc.SetAvailability(context.Background(), "", date, domain.ShiftAvailability{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SetAvailability() error = %v, want ErrInvalidInput", err)
	}
	if len(repo.setAvailabilityCalls) != 0 {
		t.Fatalf("expected repo not to be called, got %+v", repo.setAvailabilityCalls)
	}
}

func TestEmployeeService_SetAvailability_RepoError_IsWrapped(t *testing.T) {
	repoErr := errors.New("db down")
	repo := &fakeEmployeeRepository{setAvailabilityErr: repoErr}
	svc := NewEmployeeService(repo)
	date, _ := domain.ParseDate("2026-09-07")

	err := svc.SetAvailability(context.Background(), "NV01", date, domain.ShiftAvailability{})
	if !errors.Is(err, repoErr) {
		t.Fatalf("SetAvailability() error = %v, want wrapped %v", err, repoErr)
	}
}

func TestEmployeeService_SetLeaveDay_HappyPath(t *testing.T) {
	repo := &fakeEmployeeRepository{}
	svc := NewEmployeeService(repo)
	date, _ := domain.ParseDate("2026-09-07")

	err := svc.SetLeaveDay(context.Background(), "NV01", date, true)
	if err != nil {
		t.Fatalf("SetLeaveDay() error = %v", err)
	}
	if len(repo.setLeaveDayCalls) != 1 || repo.setLeaveDayCalls[0] != true {
		t.Fatalf("unexpected repo calls: %+v", repo.setLeaveDayCalls)
	}
}

func TestEmployeeService_SetAvailability_ZeroDate_ReturnsInvalidInput(t *testing.T) {
	repo := &fakeEmployeeRepository{}
	svc := NewEmployeeService(repo)

	err := svc.SetAvailability(context.Background(), "NV01", domain.Date{}, domain.ShiftAvailability{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SetAvailability() error = %v, want ErrInvalidInput", err)
	}
	if len(repo.setAvailabilityCalls) != 0 {
		t.Fatalf("expected repo not to be called, got %+v", repo.setAvailabilityCalls)
	}
}

func TestEmployeeService_SetLeaveDay_ZeroDate_ReturnsInvalidInput(t *testing.T) {
	repo := &fakeEmployeeRepository{}
	svc := NewEmployeeService(repo)

	err := svc.SetLeaveDay(context.Background(), "NV01", domain.Date{}, true)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SetLeaveDay() error = %v, want ErrInvalidInput", err)
	}
}

func TestEmployeeService_SetLeaveDay_EmptyEmployeeID_ReturnsInvalidInput(t *testing.T) {
	repo := &fakeEmployeeRepository{}
	svc := NewEmployeeService(repo)
	date, _ := domain.ParseDate("2026-09-07")

	err := svc.SetLeaveDay(context.Background(), "", date, false)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SetLeaveDay() error = %v, want ErrInvalidInput", err)
	}
}
