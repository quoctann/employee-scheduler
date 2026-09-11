// Package xlsxexport is the outbound adapter for port.ExportRenderer: it
// turns a solved schedule into an .xlsx workbook laid out like the
// traditional paper/Excel "BẢNG PHÂN CA TUẦN" the demo customer already
// works with (employees as rows, one column per day, gate-hours codes
// colored by shift), plus two supporting sheets that already exist on
// screen (shortages, hours summary).
package xlsxexport

import (
	"fmt"
	"sort"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

const (
	sheetSchedule = "Lịch xếp ca"
	sheetShortage = "Thiếu ca"
	sheetSummary  = "Tổng hợp giờ làm"

	firstDateCol = 3 // A=STT, B=Họ và tên, C.. = one column per day
	dataStartRow = 4 // row 1 = title, rows 2-3 = two-tier header
)

type Renderer struct{}

func New() *Renderer { return &Renderer{} }

// RenderScheduleWorkbook builds the 3-sheet workbook and returns its raw
// .xlsx bytes.
func (r *Renderer) RenderScheduleWorkbook(data port.ScheduleExportData) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	if err := f.SetSheetName("Sheet1", sheetSchedule); err != nil {
		return nil, fmt.Errorf("rename default sheet: %w", err)
	}
	if err := buildScheduleSheet(f, data); err != nil {
		return nil, fmt.Errorf("build schedule sheet: %w", err)
	}

	if _, err := f.NewSheet(sheetShortage); err != nil {
		return nil, fmt.Errorf("create shortage sheet: %w", err)
	}
	if err := buildShortageSheet(f, data.Result.Shortages); err != nil {
		return nil, fmt.Errorf("build shortage sheet: %w", err)
	}

	if _, err := f.NewSheet(sheetSummary); err != nil {
		return nil, fmt.Errorf("create summary sheet: %w", err)
	}
	if err := buildSummarySheet(f, data.Result.EmployeeSummary); err != nil {
		return nil, fmt.Errorf("build summary sheet: %w", err)
	}

	f.SetActiveSheet(0)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write workbook: %w", err)
	}
	return buf.Bytes(), nil
}

func buildScheduleSheet(f *excelize.File, data port.ScheduleExportData) error {
	sheet := sheetSchedule
	result := data.Result
	numDays := result.NumDays
	if numDays < 1 {
		return fmt.Errorf("num_days must be >= 1, got %d", numDays)
	}

	dates := make([]domain.Date, numDays)
	for i := range dates {
		dates[i] = result.StartDate.AddDays(i)
	}

	lastDateCol := firstDateCol + numDays - 1
	phoneCol := lastDateCol + 1
	stampCol := lastDateCol + 2

	phoneColName, err := excelize.ColumnNumberToName(phoneCol)
	if err != nil {
		return err
	}
	stampColName, err := excelize.ColumnNumberToName(stampCol)
	if err != nil {
		return err
	}
	lastColName, err := excelize.ColumnNumberToName(stampCol)
	if err != nil {
		return err
	}

	styles, err := newScheduleStyles(f)
	if err != nil {
		return err
	}

	// Row 1: merged title.
	if err := f.MergeCell(sheet, "A1", lastColName+"1"); err != nil {
		return err
	}
	title := fmt.Sprintf("BẢNG PHÂN CA TUẦN: %s - %s", formatDMY(result.StartDate), formatDMY(dates[numDays-1]))
	if err := f.SetCellValue(sheet, "A1", title); err != nil {
		return err
	}
	if err := f.SetCellStyle(sheet, "A1", lastColName+"1", styles.title); err != nil {
		return err
	}

	// Rows 2-3: two-tier header.
	if err := setMergedHeader(f, sheet, "A", "STT", styles.header); err != nil {
		return err
	}
	if err := setMergedHeader(f, sheet, "B", "Họ và tên", styles.header); err != nil {
		return err
	}
	if err := setMergedHeader(f, sheet, phoneColName, "Điện thoại", styles.header); err != nil {
		return err
	}
	if err := setMergedHeader(f, sheet, stampColName, "Mã dấu", styles.header); err != nil {
		return err
	}
	for i, d := range dates {
		colName, err := excelize.ColumnNumberToName(firstDateCol + i)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, colName+"2", formatDM(d)); err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, colName+"3", weekdayLabel(d)); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, colName+"2", colName+"3", styles.header); err != nil {
			return err
		}
	}

	// Data rows: one per active employee, sorted by employee_id (same order
	// as ScheduleTable.tsx).
	employees := append([]domain.Employee(nil), data.Employees...)
	sort.Slice(employees, func(i, j int) bool { return employees[i].EmployeeID < employees[j].EmployeeID })

	byEmployeeDate := make(map[string]map[domain.Date]domain.ScheduleEntry, len(employees))
	for _, entry := range result.Schedule {
		m, ok := byEmployeeDate[entry.EmployeeID]
		if !ok {
			m = make(map[domain.Date]domain.ScheduleEntry)
			byEmployeeDate[entry.EmployeeID] = m
		}
		m[entry.Date] = entry
	}

	for i, emp := range employees {
		row := dataStartRow + i
		sttCell := fmt.Sprintf("A%d", row)
		nameCell := fmt.Sprintf("B%d", row)
		if err := f.SetCellValue(sheet, sttCell, i+1); err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, nameCell, emp.Name); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, sttCell, sttCell, styles.cell); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, nameCell, nameCell, styles.nameCell); err != nil {
			return err
		}

		leaveDays := make(map[domain.Date]bool, len(emp.LeaveDays))
		for _, d := range emp.LeaveDays {
			leaveDays[d] = true
		}

		for di, d := range dates {
			colName, err := excelize.ColumnNumberToName(firstDateCol + di)
			if err != nil {
				return err
			}
			cellRef := fmt.Sprintf("%s%d", colName, row)
			entry, worked := byEmployeeDate[emp.EmployeeID][d]
			switch {
			case worked:
				hours := "?"
				if h, ok := data.ShiftHours[entry.Gate][entry.Shift]; ok {
					hours = fmt.Sprintf("%d", h)
				}
				if err := f.SetCellValue(sheet, cellRef, fmt.Sprintf("%s-%s", entry.Gate, hours)); err != nil {
					return err
				}
				style := styles.sang
				if entry.Shift == domain.ShiftDem {
					style = styles.dem
				}
				if err := f.SetCellStyle(sheet, cellRef, cellRef, style); err != nil {
					return err
				}
			case leaveDays[d]:
				if err := f.SetCellStyle(sheet, cellRef, cellRef, styles.leave); err != nil {
					return err
				}
			default:
				if err := f.SetCellStyle(sheet, cellRef, cellRef, styles.cell); err != nil {
					return err
				}
			}
		}

		phoneRef := fmt.Sprintf("%s%d", phoneColName, row)
		stampRef := fmt.Sprintf("%s%d", stampColName, row)
		if err := f.SetCellStyle(sheet, phoneRef, phoneRef, styles.cell); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, stampRef, stampRef, styles.cell); err != nil {
			return err
		}
	}

	if err := f.SetColWidth(sheet, "A", "A", 5); err != nil {
		return err
	}
	if err := f.SetColWidth(sheet, "B", "B", 22); err != nil {
		return err
	}
	firstDateColName, err := excelize.ColumnNumberToName(firstDateCol)
	if err != nil {
		return err
	}
	lastDateColName, err := excelize.ColumnNumberToName(lastDateCol)
	if err != nil {
		return err
	}
	if err := f.SetColWidth(sheet, firstDateColName, lastDateColName, 10); err != nil {
		return err
	}
	if err := f.SetColWidth(sheet, phoneColName, stampColName, 14); err != nil {
		return err
	}

	// Freeze STT+Họ và tên and the header rows so they stay visible while
	// scrolling a wide/tall horizon.
	return f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		XSplit:      2,
		YSplit:      dataStartRow - 1,
		TopLeftCell: fmt.Sprintf("C%d", dataStartRow),
		ActivePane:  "bottomRight",
	})
}

func buildShortageSheet(f *excelize.File, shortages []domain.ShortageItem) error {
	sheet := sheetShortage
	headerStyle, err := newHeaderStyle(f)
	if err != nil {
		return err
	}
	if err := writeHeaderRow(f, sheet, headerStyle, "Ngày", "Cổng", "Ca", "Loại", "Thiếu"); err != nil {
		return err
	}

	for i, item := range shortages {
		row := i + 2
		shiftLabel := "Sáng"
		if item.Shift == domain.ShiftDem {
			shiftLabel = "Đêm"
		}
		typeLabel := "Thiếu NV"
		if item.ShortageType == domain.ShortageLead {
			typeLabel = "Thiếu lead"
		}
		if err := writeRow(f, sheet, row, formatDM(item.Date), item.Gate, shiftLabel, typeLabel, item.Missing); err != nil {
			return err
		}
	}
	return f.SetColWidth(sheet, "A", "E", 14)
}

func buildSummarySheet(f *excelize.File, summary []domain.EmployeeSummary) error {
	sheet := sheetSummary
	headerStyle, err := newHeaderStyle(f)
	if err != nil {
		return err
	}
	if err := writeHeaderRow(f, sheet, headerStyle, "Nhân viên", "Vai trò", "Mục tiêu (h)", "Thực tế (h)", "Chênh lệch (h)", "Số ca"); err != nil {
		return err
	}

	sorted := append([]domain.EmployeeSummary(nil), summary...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].EmployeeID < sorted[j].EmployeeID })

	for i, s := range sorted {
		row := i + 2
		if err := writeRow(f, sheet, row, s.EmployeeID, string(s.Role), s.TargetHours, s.ActualHours, s.DeviationHours, s.ActualShifts); err != nil {
			return err
		}
	}
	return f.SetColWidth(sheet, "A", "F", 14)
}

func writeHeaderRow(f *excelize.File, sheet string, style int, headers ...string) error {
	for i, h := range headers {
		colName, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		cell := colName + "1"
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cell, cell, style); err != nil {
			return err
		}
	}
	return nil
}

func writeRow(f *excelize.File, sheet string, row int, values ...any) error {
	for i, v := range values {
		colName, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("%s%d", colName, row), v); err != nil {
			return err
		}
	}
	return nil
}

func setMergedHeader(f *excelize.File, sheet, col, value string, style int) error {
	top := col + "2"
	bottom := col + "3"
	if err := f.MergeCell(sheet, top, bottom); err != nil {
		return err
	}
	if err := f.SetCellValue(sheet, top, value); err != nil {
		return err
	}
	return f.SetCellStyle(sheet, top, bottom, style)
}

func weekdayLabel(d domain.Date) string {
	switch d.Weekday() {
	case time.Sunday:
		return "C.NHẬT"
	case time.Monday:
		return "THỨ 2"
	case time.Tuesday:
		return "THỨ 3"
	case time.Wednesday:
		return "THỨ 4"
	case time.Thursday:
		return "THỨ 5"
	case time.Friday:
		return "THỨ 6"
	default:
		return "THỨ 7"
	}
}

func formatDM(d domain.Date) string  { return d.Format("02/01") }
func formatDMY(d domain.Date) string { return d.Format("02/01/2006") }

type scheduleStyles struct {
	title    int
	header   int
	cell     int
	nameCell int
	sang     int
	dem      int
	leave    int
}

func newScheduleStyles(f *excelize.File) (scheduleStyles, error) {
	border := []excelize.Border{
		{Type: "left", Color: "999999", Style: 1},
		{Type: "right", Color: "999999", Style: 1},
		{Type: "top", Color: "999999", Style: 1},
		{Type: "bottom", Color: "999999", Style: 1},
	}
	center := &excelize.Alignment{Horizontal: "center", Vertical: "center"}

	title, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: center,
	})
	if err != nil {
		return scheduleStyles{}, err
	}

	header, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#E5E7EB"}, Pattern: 1},
		Border:    border,
	})
	if err != nil {
		return scheduleStyles{}, err
	}

	cell, err := f.NewStyle(&excelize.Style{Alignment: center, Border: border})
	if err != nil {
		return scheduleStyles{}, err
	}

	nameCell, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    border,
	})
	if err != nil {
		return scheduleStyles{}, err
	}

	// Approximate hex equivalents of the app's UI badge colors
	// (--status-sang / --status-dem in frontend/src/index.css, defined in
	// OKLCH) — excelize fills only accept hex.
	sang, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#5C3A00", Bold: true},
		Alignment: center,
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#F2CB6B"}, Pattern: 1},
		Border:    border,
	})
	if err != nil {
		return scheduleStyles{}, err
	}

	dem, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#FFFFFF", Bold: true},
		Alignment: center,
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#33415E"}, Pattern: 1},
		Border:    border,
	})
	if err != nil {
		return scheduleStyles{}, err
	}

	leave, err := f.NewStyle(&excelize.Style{
		Alignment: center,
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#D1D5DB"}, Pattern: 1},
		Border:    border,
	})
	if err != nil {
		return scheduleStyles{}, err
	}

	return scheduleStyles{
		title: title, header: header, cell: cell, nameCell: nameCell,
		sang: sang, dem: dem, leave: leave,
	}, nil
}

func newHeaderStyle(f *excelize.File) (int, error) {
	return f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E5E7EB"}, Pattern: 1},
	})
}

var _ port.ExportRenderer = (*Renderer)(nil)
