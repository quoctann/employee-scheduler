package port

import "github.com/tantq/employee-scheduler-backend/internal/core/domain"

// ScheduleExportData is everything a renderer needs to build the schedule
// workbook — plain domain data, so the rendering library (excelize) never
// leaks into core/service.
type ScheduleExportData struct {
	Result     domain.SolveResult
	Employees  []domain.Employee
	ShiftHours map[string]map[domain.ShiftType]int
}

// ExportRenderer is the outbound port for turning a solved schedule into a
// downloadable file. Implemented by adapter/xlsxexport.
type ExportRenderer interface {
	RenderScheduleWorkbook(data ScheduleExportData) ([]byte, error)
}
