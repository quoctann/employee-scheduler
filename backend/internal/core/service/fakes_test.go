package service

import (
	"context"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

// fakeSolverGateway is a hand-written test double for port.SolverGateway —
// no mocking framework needed for a handful of methods.
type fakeSolverGateway struct {
	solveReq    port.SolveRequest
	solveResult domain.SolveResult
	solveErr    error

	capacityReq    port.CapacityCheckRequest
	capacityResult domain.CapacityCheckResult
	capacityErr    error

	candidatesReq    port.ReplacementCandidatesRequest
	candidatesResult domain.ReplacementCandidatesResult
	candidatesErr    error
}

func (f *fakeSolverGateway) Solve(_ context.Context, req port.SolveRequest) (domain.SolveResult, error) {
	f.solveReq = req
	return f.solveResult, f.solveErr
}

func (f *fakeSolverGateway) CapacityCheck(_ context.Context, req port.CapacityCheckRequest) (domain.CapacityCheckResult, error) {
	f.capacityReq = req
	return f.capacityResult, f.capacityErr
}

func (f *fakeSolverGateway) ReplacementCandidates(_ context.Context, req port.ReplacementCandidatesRequest) (domain.ReplacementCandidatesResult, error) {
	f.candidatesReq = req
	return f.candidatesResult, f.candidatesErr
}

type fakeEmployeeRepository struct {
	employees        []domain.Employee
	availability     domain.AvailabilityMap
	listErr          error
	availabilityErr  error
	availabilityFrom domain.Date
	availabilityTo   domain.Date

	setAvailabilityErr   error
	setAvailabilityCalls []domain.ShiftAvailability
	setLeaveDayErr       error
	setLeaveDayCalls     []bool
}

func (f *fakeEmployeeRepository) List(_ context.Context) ([]domain.Employee, error) {
	return f.employees, f.listErr
}

func (f *fakeEmployeeRepository) Availability(_ context.Context, from, to domain.Date) (domain.AvailabilityMap, error) {
	f.availabilityFrom = from
	f.availabilityTo = to
	return f.availability, f.availabilityErr
}

func (f *fakeEmployeeRepository) SetAvailability(_ context.Context, _ string, _ domain.Date, avail domain.ShiftAvailability) error {
	f.setAvailabilityCalls = append(f.setAvailabilityCalls, avail)
	return f.setAvailabilityErr
}

func (f *fakeEmployeeRepository) SetLeaveDay(_ context.Context, _ string, _ domain.Date, onLeave bool) error {
	f.setLeaveDayCalls = append(f.setLeaveDayCalls, onLeave)
	return f.setLeaveDayErr
}

type fakeConfigRepository struct {
	config domain.SolverConfig
	err    error
}

func (f *fakeConfigRepository) Get(_ context.Context) (domain.SolverConfig, error) {
	return f.config, f.err
}

type fakeScheduleRepository struct {
	saveRunCalled bool
	saveRunResult domain.SolveResult
	saveRunID     int64
	saveRunErr    error

	latestResult domain.SolveResult
	latestFound  bool
	latestErr    error

	carryIn       domain.CarryIn
	carryInErr    error
	carryInBefore domain.Date

	approveCount int
	approveErr   error
	approved     []domain.LockedAssignment

	approvedAssignments []domain.LockedAssignment
	approvedErr         error
}

func (f *fakeScheduleRepository) SaveRun(_ context.Context, result domain.SolveResult) (int64, error) {
	f.saveRunCalled = true
	f.saveRunResult = result
	return f.saveRunID, f.saveRunErr
}

func (f *fakeScheduleRepository) LatestRun(_ context.Context) (domain.SolveResult, bool, error) {
	return f.latestResult, f.latestFound, f.latestErr
}

func (f *fakeScheduleRepository) CarryIn(_ context.Context, beforeDate domain.Date) (domain.CarryIn, error) {
	f.carryInBefore = beforeDate
	return f.carryIn, f.carryInErr
}

func (f *fakeScheduleRepository) ApproveAssignments(_ context.Context, assignments []domain.LockedAssignment) (int, error) {
	f.approved = assignments
	return f.approveCount, f.approveErr
}

func (f *fakeScheduleRepository) ApprovedAssignments(_ context.Context, _, _ domain.Date) ([]domain.LockedAssignment, error) {
	return f.approvedAssignments, f.approvedErr
}

var (
	_ port.SolverGateway      = (*fakeSolverGateway)(nil)
	_ port.EmployeeRepository = (*fakeEmployeeRepository)(nil)
	_ port.ConfigRepository   = (*fakeConfigRepository)(nil)
	_ port.ScheduleRepository = (*fakeScheduleRepository)(nil)
)
