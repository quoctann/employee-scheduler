package service

import (
	"context"
	"errors"
	"testing"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
)

func TestExportService_Export_NoLatestRun_ReturnsErrNotFound(t *testing.T) {
	schedRepo := &fakeScheduleRepository{latestFound: false}
	svc := NewExportService(&fakeEmployeeRepository{}, &fakeConfigRepository{}, schedRepo, &fakeExportRenderer{})

	_, _, err := svc.Export(context.Background())

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Export() error = %v, want ErrNotFound", err)
	}
}

func TestExportService_Export_RendersLatestRunWithRosterAndShiftHours(t *testing.T) {
	start := mustDate(t, "2026-09-07")
	result := domain.SolveResult{
		StartDate: start,
		NumDays:   7,
		Schedule:  []domain.ScheduleEntry{{EmployeeID: "NV01", Date: start, Gate: "A", Shift: domain.ShiftSang}},
	}
	employees := []domain.Employee{{EmployeeID: "NV01", Name: "Nguyễn Văn A", Role: domain.RoleNV}}
	config := domain.SolverConfig{ShiftHours: map[string]map[domain.ShiftType]int{"A": {domain.ShiftSang: 11}}}

	schedRepo := &fakeScheduleRepository{latestFound: true, latestResult: result}
	empRepo := &fakeEmployeeRepository{employees: employees}
	cfgRepo := &fakeConfigRepository{config: config}
	renderer := &fakeExportRenderer{renderResult: []byte("xlsx-bytes")}

	svc := NewExportService(empRepo, cfgRepo, schedRepo, renderer)

	fileBytes, filename, err := svc.Export(context.Background())

	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if string(fileBytes) != "xlsx-bytes" {
		t.Fatalf("unexpected file bytes: %s", fileBytes)
	}
	wantFilename := "lich-xep-ca_2026-09-07_7ngay.xlsx"
	if filename != wantFilename {
		t.Fatalf("filename = %q, want %q", filename, wantFilename)
	}
	if len(renderer.lastData.Employees) != 1 || renderer.lastData.Employees[0].EmployeeID != "NV01" {
		t.Fatalf("expected roster to be forwarded to renderer, got %+v", renderer.lastData.Employees)
	}
	if renderer.lastData.Result.StartDate != start {
		t.Fatalf("expected latest run to be forwarded to renderer, got %+v", renderer.lastData.Result)
	}
	if renderer.lastData.ShiftHours["A"][domain.ShiftSang] != 11 {
		t.Fatalf("expected shift hours from config to be forwarded, got %+v", renderer.lastData.ShiftHours)
	}
}

func TestExportService_Export_RendererError_IsPropagated(t *testing.T) {
	schedRepo := &fakeScheduleRepository{latestFound: true, latestResult: domain.SolveResult{StartDate: mustDate(t, "2026-09-07"), NumDays: 1}}
	renderer := &fakeExportRenderer{renderErr: errors.New("boom")}
	svc := NewExportService(&fakeEmployeeRepository{}, &fakeConfigRepository{}, schedRepo, renderer)

	_, _, err := svc.Export(context.Background())

	if err == nil {
		t.Fatal("Export() expected an error, got nil")
	}
}
