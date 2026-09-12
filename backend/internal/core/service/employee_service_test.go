package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/port"
)

func TestEmployeeService_Roster_ReturnsEmployeesAndAvailability(t *testing.T) {
	repo := &fakeEmployeeRepository{
		employees:    []domain.Employee{{EmployeeID: "NV01", Name: "NV01", Role: domain.RoleNV}},
		availability: domain.AvailabilityMap{"NV01": {}},
	}
	svc := NewEmployeeService(repo)

	employees, availability, err := svc.Roster(context.Background(), false, domain.Date{}, domain.Date{})
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
		t.Fatalf("expected default roster window to span >= 28 days, got [%v,%v]", repo.availabilityFrom, repo.availabilityTo)
	}
}

func TestEmployeeService_Roster_ExplicitWindow_OverridesDefault(t *testing.T) {
	repo := &fakeEmployeeRepository{
		employees:    []domain.Employee{{EmployeeID: "NV01", Name: "NV01", Role: domain.RoleNV}},
		availability: domain.AvailabilityMap{"NV01": {}},
	}
	svc := NewEmployeeService(repo)
	from, _ := domain.ParseDate("2026-01-01")
	to, _ := domain.ParseDate("2026-01-07")

	_, _, err := svc.Roster(context.Background(), false, from, to)
	if err != nil {
		t.Fatalf("Roster() error = %v", err)
	}
	// A registration UI browsing a week entirely in the past (or far in the
	// future) must get back exactly that window, not the today-anchored
	// default — otherwise a write there would never be reflected on the
	// next read, even after a hard refresh.
	if repo.availabilityFrom != from || repo.availabilityTo != to {
		t.Fatalf("Roster() requested window = [%v,%v], want [%v,%v]", repo.availabilityFrom, repo.availabilityTo, from, to)
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

func TestEmployeeService_CreateUpdateAndSetActive(t *testing.T) {
	repo := &fakeEmployeeRepository{
		createResult:    domain.Employee{EmployeeID: "NV22", Name: "Nhan vien 22", Role: domain.RoleNV, Active: true},
		updateResult:    domain.Employee{EmployeeID: "NV22", Name: "Truong ca 22", Role: domain.RoleTC, Active: true},
		setActiveResult: domain.Employee{EmployeeID: "NV22", Active: false},
	}
	svc := NewEmployeeService(repo)

	if _, err := svc.Create(context.Background(), domain.Employee{EmployeeID: " NV22 ", Name: " Nhan vien 22 ", Role: domain.RoleNV}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repo.createEmployee.EmployeeID != "NV22" || !repo.createEmployee.Active {
		t.Fatalf("Create() passed %+v", repo.createEmployee)
	}
	if _, err := svc.Update(context.Background(), "NV22", "Truong ca 22", domain.RoleTC); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repo.updateEmployeeID != "NV22" || repo.updateRole != domain.RoleTC {
		t.Fatalf("Update() passed id=%q role=%q", repo.updateEmployeeID, repo.updateRole)
	}
	if _, err := svc.Deactivate(context.Background(), "NV22"); err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}
	if repo.setActiveValue {
		t.Fatal("Deactivate() set active=true")
	}
	if _, err := svc.Restore(context.Background(), "NV22"); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if !repo.setActiveValue {
		t.Fatal("Restore() set active=false")
	}
}

func TestEmployeeService_MutationsValidateAndMapExpectedErrors(t *testing.T) {
	repo := &fakeEmployeeRepository{}
	svc := NewEmployeeService(repo)

	if _, err := svc.Create(context.Background(), domain.Employee{EmployeeID: " ", Name: "Name", Role: domain.RoleNV}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Create() error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.Update(context.Background(), "NV01", "", domain.RoleNV); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update() error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.Create(context.Background(), domain.Employee{EmployeeID: strings.Repeat("A", maxEmployeeIDLength+1), Name: "Name", Role: domain.RoleNV}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Create() oversized ID error = %v, want ErrInvalidInput", err)
	}
	repo.createErr = port.ErrEmployeeConflict
	if _, err := svc.Create(context.Background(), domain.Employee{EmployeeID: "NV01", Name: "Name", Role: domain.RoleNV}); !errors.Is(err, ErrConflict) {
		t.Fatalf("Create() error = %v, want ErrConflict", err)
	}
	repo.setActiveErr = port.ErrEmployeeNotFound
	if _, err := svc.Restore(context.Background(), "NV99"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Restore() error = %v, want ErrNotFound", err)
	}
}
