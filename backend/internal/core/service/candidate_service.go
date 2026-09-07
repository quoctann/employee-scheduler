package service

import (
	"context"
	"fmt"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

const defaultTopN = 5

type CandidateService struct {
	Solver    port.SolverGateway
	Employees port.EmployeeRepository
	Config    port.ConfigRepository
	Schedules port.ScheduleRepository
}

func NewCandidateService(solver port.SolverGateway, employees port.EmployeeRepository, config port.ConfigRepository, schedules port.ScheduleRepository) *CandidateService {
	return &CandidateService{Solver: solver, Employees: employees, Config: config, Schedules: schedules}
}

type CandidateParams struct {
	TargetSlot         domain.TargetSlot
	ExcludedEmployeeID *string
	TopN               int
}

// Suggest fills employees/availability/current_schedule/carry_in from
// whatever the latest persisted solve run covers, so callers only need to
// supply the slot they're trying to fill.
func (s *CandidateService) Suggest(ctx context.Context, params CandidateParams) (domain.ReplacementCandidatesResult, error) {
	startDate := params.TargetSlot.Date
	numDays := 1
	var currentSchedule []domain.ScheduleEntry

	latest, found, err := s.Schedules.LatestRun(ctx)
	if err != nil {
		return domain.ReplacementCandidatesResult{}, fmt.Errorf("load latest run: %w", err)
	}
	if found {
		startDate = latest.StartDate
		numDays = latest.NumDays
		currentSchedule = latest.Schedule
	}
	endDate := startDate.AddDays(numDays - 1)

	employees, err := s.Employees.List(ctx)
	if err != nil {
		return domain.ReplacementCandidatesResult{}, fmt.Errorf("list employees: %w", err)
	}

	availability, err := s.Employees.Availability(ctx, startDate, endDate)
	if err != nil {
		return domain.ReplacementCandidatesResult{}, fmt.Errorf("load availability: %w", err)
	}

	config, err := s.Config.Get(ctx)
	if err != nil {
		return domain.ReplacementCandidatesResult{}, fmt.Errorf("load solver config: %w", err)
	}

	carryIn, err := s.Schedules.CarryIn(ctx, startDate.AddDays(-1))
	if err != nil {
		return domain.ReplacementCandidatesResult{}, fmt.Errorf("load carry-in: %w", err)
	}

	topN := params.TopN
	if topN <= 0 {
		topN = defaultTopN
	}

	result, err := s.Solver.ReplacementCandidates(ctx, port.ReplacementCandidatesRequest{
		StartDate:          startDate,
		NumDays:            numDays,
		Employees:          employees,
		Availability:       availability,
		CurrentSchedule:    currentSchedule,
		CarryIn:            carryIn,
		TargetSlot:         params.TargetSlot,
		ExcludedEmployeeID: params.ExcludedEmployeeID,
		TopN:               topN,
		Config:             config,
	})
	if err != nil {
		return domain.ReplacementCandidatesResult{}, fmt.Errorf("call solver: %w", err)
	}
	return result, nil
}
