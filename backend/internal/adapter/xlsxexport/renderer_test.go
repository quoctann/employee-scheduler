package xlsxexport_test

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/quoctann/employee-scheduler-backend/internal/adapter/xlsxexport"
	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/port"
)

func mustDate(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return d
}

func TestRenderScheduleWorkbook_ProducesThreeSheetsWithExpectedRowsAndCells(t *testing.T) {
	start := mustDate(t, "2026-09-07") // a Monday
	data := port.ScheduleExportData{
		Result: domain.SolveResult{
			StartDate: start,
			NumDays:   2,
			Schedule: []domain.ScheduleEntry{
				{EmployeeID: "NV01", Date: start, Gate: "A", Shift: domain.ShiftSang},
				{EmployeeID: "NV02", Date: start.AddDays(1), Gate: "B", Shift: domain.ShiftDem},
			},
			Shortages: []domain.ShortageItem{
				{Date: start, Gate: "A", Shift: domain.ShiftSang, ShortageType: domain.ShortageStaff, Missing: 1},
			},
			EmployeeSummary: []domain.EmployeeSummary{
				{EmployeeID: "NV01", Role: domain.RoleNV, TargetHours: 44, ActualHours: 33, DeviationHours: -11, ActualShifts: 3},
			},
		},
		Employees: []domain.Employee{
			{EmployeeID: "NV02", Name: "Nhân viên 2", Role: domain.RoleNV},
			{EmployeeID: "NV01", Name: "Nhân viên 1", Role: domain.RoleNV, LeaveDays: []domain.Date{start.AddDays(1)}},
		},
		ShiftHours: map[string]map[domain.ShiftType]int{
			"A": {domain.ShiftSang: 11},
			"B": {domain.ShiftDem: 12},
		},
	}

	fileBytes, err := xlsxexport.New().RenderScheduleWorkbook(data)
	if err != nil {
		t.Fatalf("RenderScheduleWorkbook() error = %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(fileBytes))
	if err != nil {
		t.Fatalf("re-open rendered workbook: %v", err)
	}
	defer f.Close()

	wantSheets := []string{"Lịch xếp ca", "Thiếu ca", "Tổng hợp giờ làm"}
	gotSheets := f.GetSheetList()
	if len(gotSheets) != len(wantSheets) {
		t.Fatalf("sheets = %v, want %v", gotSheets, wantSheets)
	}
	for i, name := range wantSheets {
		if gotSheets[i] != name {
			t.Fatalf("sheet[%d] = %q, want %q", i, gotSheets[i], name)
		}
	}

	// Schedule sheet: title row, 2-tier header, then one row per employee
	// sorted by employee_id — NV01 (row 4) worked A-11 on the first day, NV02
	// (row 5) worked B-12 on the second day and is off on the first.
	title, _ := f.GetCellValue("Lịch xếp ca", "A1")
	wantTitle := "BẢNG PHÂN CA TUẦN: 07/09/2026 - 08/09/2026"
	if title != wantTitle {
		t.Fatalf("title = %q, want %q", title, wantTitle)
	}
	name4, _ := f.GetCellValue("Lịch xếp ca", "B4")
	cellC4, _ := f.GetCellValue("Lịch xếp ca", "C4")
	if name4 != "Nhân viên 1" || cellC4 != "A-11" {
		t.Fatalf("row 4 = name %q, day-1 cell %q; want %q, %q", name4, cellC4, "Nhân viên 1", "A-11")
	}
	name5, _ := f.GetCellValue("Lịch xếp ca", "B5")
	cellD5, _ := f.GetCellValue("Lịch xếp ca", "D5")
	if name5 != "Nhân viên 2" || cellD5 != "B-12" {
		t.Fatalf("row 5 = name %q, day-2 cell %q; want %q, %q", name5, cellD5, "Nhân viên 2", "B-12")
	}

	rows, err := f.GetRows("Thiếu ca")
	if err != nil {
		t.Fatalf("GetRows(Thiếu ca): %v", err)
	}
	if len(rows) != 2 { // header + 1 shortage
		t.Fatalf("Thiếu ca rows = %d, want 2 (header + 1 shortage): %v", len(rows), rows)
	}

	summaryRows, err := f.GetRows("Tổng hợp giờ làm")
	if err != nil {
		t.Fatalf("GetRows(Tổng hợp giờ làm): %v", err)
	}
	if len(summaryRows) != 2 { // header + 1 employee
		t.Fatalf("Tổng hợp giờ làm rows = %d, want 2 (header + 1 employee): %v", len(summaryRows), summaryRows)
	}
}
