package httpapi_test

import (
	"context"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

// Minimal hand-written port fakes so these tests can build real
// service.* instances end-to-end (JSON decode -> service -> JSON envelope)
// without needing a database or a live solver-service.

type fakeSolverGateway struct {
	solveResult      domain.SolveResult
	solveErr         error
	capacityResult   domain.CapacityCheckResult
	candidatesResult domain.ReplacementCandidatesResult
}

func (f *fakeSolverGateway) Solve(context.Context, port.SolveRequest) (domain.SolveResult, error) {
	return f.solveResult, f.solveErr
}
func (f *fakeSolverGateway) CapacityCheck(context.Context, port.CapacityCheckRequest) (domain.CapacityCheckResult, error) {
	return f.capacityResult, nil
}
func (f *fakeSolverGateway) ReplacementCandidates(context.Context, port.ReplacementCandidatesRequest) (domain.ReplacementCandidatesResult, error) {
	return f.candidatesResult, nil
}

type fakeEmployeeRepository struct {
	employees          []domain.Employee
	availability       domain.AvailabilityMap
	setAvailabilityErr error
	setLeaveDayErr     error
}

func (f *fakeEmployeeRepository) List(context.Context) ([]domain.Employee, error) {
	return f.employees, nil
}
func (f *fakeEmployeeRepository) Availability(context.Context, domain.Date, domain.Date) (domain.AvailabilityMap, error) {
	return f.availability, nil
}

func (f *fakeEmployeeRepository) SetAvailability(context.Context, string, domain.Date, domain.ShiftAvailability) error {
	return f.setAvailabilityErr
}

func (f *fakeEmployeeRepository) SetLeaveDay(context.Context, string, domain.Date, bool) error {
	return f.setLeaveDayErr
}

type fakeConfigRepository struct {
	config domain.SolverConfig
}

func (f *fakeConfigRepository) Get(context.Context) (domain.SolverConfig, error) {
	return f.config, nil
}

type fakeScheduleRepository struct {
	saveRunID    int64
	latestResult domain.SolveResult
	latestFound  bool
	approveCount int
}

func (f *fakeScheduleRepository) SaveRun(context.Context, domain.SolveResult) (int64, error) {
	return f.saveRunID, nil
}
func (f *fakeScheduleRepository) LatestRun(context.Context) (domain.SolveResult, bool, error) {
	return f.latestResult, f.latestFound, nil
}
func (f *fakeScheduleRepository) CarryIn(context.Context, domain.Date) (domain.CarryIn, error) {
	return domain.CarryIn{}, nil
}
func (f *fakeScheduleRepository) ApproveAssignments(context.Context, []domain.LockedAssignment) (int, error) {
	return f.approveCount, nil
}
func (f *fakeScheduleRepository) ApprovedAssignments(context.Context, domain.Date, domain.Date) ([]domain.LockedAssignment, error) {
	return nil, nil
}

var (
	_ port.SolverGateway      = (*fakeSolverGateway)(nil)
	_ port.EmployeeRepository = (*fakeEmployeeRepository)(nil)
	_ port.ConfigRepository   = (*fakeConfigRepository)(nil)
	_ port.ScheduleRepository = (*fakeScheduleRepository)(nil)
)
