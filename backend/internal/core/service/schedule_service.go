package service

import (
	"context"
	"fmt"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/port"
)

const defaultTimeLimitS = 30

type ScheduleService struct {
	Solver    port.SolverGateway
	Employees port.EmployeeRepository
	Config    port.ConfigRepository
	Schedules port.ScheduleRepository
}

func NewScheduleService(solver port.SolverGateway, employees port.EmployeeRepository, config port.ConfigRepository, schedules port.ScheduleRepository) *ScheduleService {
	return &ScheduleService{Solver: solver, Employees: employees, Config: config, Schedules: schedules}
}

type SolveParams struct {
	StartDate      domain.Date
	NumDays        int
	TimeLimitS     int
	IgnoreApproved bool
}

// Solve builds a SolveRequest from the current roster, availability,
// business config, and already-approved cells for this horizon, calls the
// solver, and persists the resulting run.
func (s *ScheduleService) Solve(ctx context.Context, params SolveParams) (domain.SolveResult, error) {
	if params.NumDays < 1 {
		return domain.SolveResult{}, fmt.Errorf("%w: num_days must be >= 1, got %d", ErrInvalidInput, params.NumDays)
	}

	endDate := params.StartDate.AddDays(params.NumDays - 1)

	employees, err := s.Employees.List(ctx, false)
	if err != nil {
		return domain.SolveResult{}, fmt.Errorf("list employees: %w", err)
	}

	availability, err := s.Employees.Availability(ctx, params.StartDate, endDate)
	if err != nil {
		return domain.SolveResult{}, fmt.Errorf("load availability: %w", err)
	}

	config, err := s.Config.Get(ctx)
	if err != nil {
		return domain.SolveResult{}, fmt.Errorf("load solver config: %w", err)
	}

	// IgnoreApproved lets a manager preview a full re-optimization for this
	// one run without touching approved_assignments: nothing is pinned, but
	// the DB's approved state is left as-is (re-Approve still overwrites it
	// with whatever this run produced).
	var locked []domain.LockedAssignment
	if !params.IgnoreApproved {
		locked, err = s.Schedules.ApprovedAssignments(ctx, params.StartDate, endDate)
		if err != nil {
			return domain.SolveResult{}, fmt.Errorf("load approved assignments: %w", err)
		}
		locked = sanitizeLockedAssignments(locked, employees, availability)
	}

	carryIn, err := s.Schedules.CarryIn(ctx, params.StartDate.AddDays(-1))
	if err != nil {
		return domain.SolveResult{}, fmt.Errorf("load carry-in: %w", err)
	}
	carryIn = filterCarryIn(carryIn, employees)

	timeLimitS := params.TimeLimitS
	if timeLimitS <= 0 {
		timeLimitS = defaultTimeLimitS
	}

	result, err := s.Solver.Solve(ctx, port.SolveRequest{
		StartDate:         params.StartDate,
		NumDays:           params.NumDays,
		Employees:         employees,
		Availability:      availability,
		LockedAssignments: locked,
		CarryIn:           carryIn,
		Config:            config,
		TimeLimitS:        timeLimitS,
	})
	if err != nil {
		return domain.SolveResult{}, fmt.Errorf("call solver: %w", err)
	}

	// The solver response has no notion of the requested horizon; stamp it
	// before persisting so LatestRun/CarryIn/ApprovedAssignments can use it.
	result.StartDate = params.StartDate
	result.NumDays = params.NumDays

	runID, err := s.Schedules.SaveRun(ctx, result)
	if err != nil {
		return domain.SolveResult{}, fmt.Errorf("save run: %w", err)
	}
	result.RunID = runID

	return result, nil
}

// sanitizeLockedAssignments drops locks for employees no longer on the active
// roster and downgrades a locked (approved) assignment to an
// explicit "off" lock wherever it no longer matches the employee's *current*
// availability/leave registration, instead of forwarding it as-is and having
// solver-service reject the whole solve.
//
// An approved cell is a snapshot taken at approval time; availability and
// leave_days are live, editable data (see EmployeeService.SetAvailability /
// SetLeaveDay). An employee registering themselves unavailable — or going on
// leave — *after* being locked into a shift is an expected, reachable
// business event (the real-world equivalent of an already-scheduled
// employee calling in sick), not a data-integrity bug, so it must not make
// every future solve over that horizon fail outright.
func sanitizeLockedAssignments(locked []domain.LockedAssignment, employees []domain.Employee, availability domain.AvailabilityMap) []domain.LockedAssignment {
	leaveDays := make(map[string]map[domain.Date]bool, len(employees))
	for _, e := range employees {
		days := make(map[domain.Date]bool, len(e.LeaveDays))
		for _, d := range e.LeaveDays {
			days[d] = true
		}
		leaveDays[e.EmployeeID] = days
	}

	sanitized := make([]domain.LockedAssignment, 0, len(locked))
	for _, la := range locked {
		if _, active := leaveDays[la.EmployeeID]; !active {
			continue
		}
		if la.Off || la.Shift == nil {
			sanitized = append(sanitized, la)
			continue
		}
		stillAvailable := !leaveDays[la.EmployeeID][la.Date]
		if stillAvailable {
			avail := availability[la.EmployeeID][la.Date]
			stillAvailable = (*la.Shift == domain.ShiftSang && avail.Sang) || (*la.Shift == domain.ShiftDem && avail.Dem)
		}
		if !stillAvailable {
			la = domain.LockedAssignment{EmployeeID: la.EmployeeID, Date: la.Date, Off: true}
		}
		sanitized = append(sanitized, la)
	}
	return sanitized
}

func activeEmployeeIDs(employees []domain.Employee) map[string]struct{} {
	ids := make(map[string]struct{}, len(employees))
	for _, employee := range employees {
		ids[employee.EmployeeID] = struct{}{}
	}
	return ids
}

func filterCarryIn(carryIn domain.CarryIn, employees []domain.Employee) domain.CarryIn {
	activeIDs := activeEmployeeIDs(employees)
	filtered := make([]string, 0, len(carryIn.WorkedNightBeforeStart))
	for _, employeeID := range carryIn.WorkedNightBeforeStart {
		if _, active := activeIDs[employeeID]; active {
			filtered = append(filtered, employeeID)
		}
	}
	carryIn.WorkedNightBeforeStart = filtered
	return carryIn
}

// Latest returns the most recently persisted solve result, if any.
func (s *ScheduleService) Latest(ctx context.Context) (domain.SolveResult, bool, error) {
	result, found, err := s.Schedules.LatestRun(ctx)
	if err != nil {
		return domain.SolveResult{}, false, fmt.Errorf("load latest run: %w", err)
	}
	return result, found, nil
}
