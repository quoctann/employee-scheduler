package service

import (
	"context"
	"fmt"

	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

type ExportService struct {
	Employees port.EmployeeRepository
	Config    port.ConfigRepository
	Schedules port.ScheduleRepository
	Renderer  port.ExportRenderer
}

func NewExportService(employees port.EmployeeRepository, config port.ConfigRepository, schedules port.ScheduleRepository, renderer port.ExportRenderer) *ExportService {
	return &ExportService{Employees: employees, Config: config, Schedules: schedules, Renderer: renderer}
}

// Export renders the latest solved schedule as an .xlsx workbook, mirroring
// what the "Lịch xếp ca" screen currently shows (same roster, same
// gate/shift-hours lookup as the grid's cell labels).
func (s *ExportService) Export(ctx context.Context) ([]byte, string, error) {
	result, found, err := s.Schedules.LatestRun(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("load latest run: %w", err)
	}
	if !found {
		return nil, "", fmt.Errorf("%w: no schedule has been solved yet", ErrNotFound)
	}

	employees, err := s.Employees.List(ctx, false)
	if err != nil {
		return nil, "", fmt.Errorf("list employees: %w", err)
	}

	config, err := s.Config.Get(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("load solver config: %w", err)
	}

	bytes, err := s.Renderer.RenderScheduleWorkbook(port.ScheduleExportData{
		Result:     result,
		Employees:  employees,
		ShiftHours: config.ShiftHours,
	})
	if err != nil {
		return nil, "", fmt.Errorf("render workbook: %w", err)
	}

	filename := fmt.Sprintf("lich-xep-ca_%s_%dngay.xlsx", result.StartDate.String(), result.NumDays)
	return bytes, filename, nil
}
