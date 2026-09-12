package httpapi_test

import (
	"context"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/port"
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
	availabilityFrom   domain.Date
	availabilityTo     domain.Date
	includeInactive    bool
	listErr            error
	setAvailabilityErr error
	setLeaveDayErr     error
	createResult       domain.Employee
	createErr          error
	updateResult       domain.Employee
	updateErr          error
	setActiveResult    domain.Employee
	setActiveErr       error
}

func (f *fakeEmployeeRepository) List(_ context.Context, includeInactive bool) ([]domain.Employee, error) {
	f.includeInactive = includeInactive
	return f.employees, f.listErr
}
func (f *fakeEmployeeRepository) Create(_ context.Context, employee domain.Employee) (domain.Employee, error) {
	if f.createResult.EmployeeID == "" {
		f.createResult = employee
	}
	return f.createResult, f.createErr
}
func (f *fakeEmployeeRepository) Update(_ context.Context, employeeID, name string, role domain.Role) (domain.Employee, error) {
	if f.updateResult.EmployeeID == "" {
		f.updateResult = domain.Employee{EmployeeID: employeeID, Name: name, Role: role, Active: true}
	}
	return f.updateResult, f.updateErr
}
func (f *fakeEmployeeRepository) SetActive(_ context.Context, employeeID string, active bool) (domain.Employee, error) {
	if f.setActiveResult.EmployeeID == "" {
		f.setActiveResult = domain.Employee{EmployeeID: employeeID, Active: active}
	}
	return f.setActiveResult, f.setActiveErr
}
func (f *fakeEmployeeRepository) Availability(_ context.Context, from, to domain.Date) (domain.AvailabilityMap, error) {
	f.availabilityFrom = from
	f.availabilityTo = to
	return f.availability, nil
}

func (f *fakeEmployeeRepository) SetAvailability(context.Context, string, domain.Date, domain.ShiftAvailability) error {
	return f.setAvailabilityErr
}

func (f *fakeEmployeeRepository) SetLeaveDay(context.Context, string, domain.Date, bool) error {
	return f.setLeaveDayErr
}

type fakeConfigRepository struct {
	config    domain.SolverConfig
	err       error
	updateErr error
	renameErr error
}

func (f *fakeConfigRepository) Get(context.Context) (domain.SolverConfig, error) {
	return f.config, f.err
}

func (f *fakeConfigRepository) UpdateGateShiftRequirement(_ context.Context, gateCode string, shiftType domain.ShiftType, requirement domain.GateShiftRequirement, shiftHours int) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	if f.config.Requirements == nil {
		f.config.Requirements = map[string]map[domain.ShiftType]domain.GateShiftRequirement{}
	}
	if f.config.Requirements[gateCode] == nil {
		f.config.Requirements[gateCode] = map[domain.ShiftType]domain.GateShiftRequirement{}
	}
	f.config.Requirements[gateCode][shiftType] = requirement
	if f.config.ShiftHours == nil {
		f.config.ShiftHours = map[string]map[domain.ShiftType]int{}
	}
	if f.config.ShiftHours[gateCode] == nil {
		f.config.ShiftHours[gateCode] = map[domain.ShiftType]int{}
	}
	f.config.ShiftHours[gateCode][shiftType] = shiftHours
	return nil
}

func (f *fakeConfigRepository) RenameGate(_ context.Context, oldCode, newCode string) error {
	if f.renameErr != nil {
		return f.renameErr
	}
	if requirement, ok := f.config.Requirements[oldCode]; ok {
		delete(f.config.Requirements, oldCode)
		f.config.Requirements[newCode] = requirement
	}
	if shiftHours, ok := f.config.ShiftHours[oldCode]; ok {
		delete(f.config.ShiftHours, oldCode)
		f.config.ShiftHours[newCode] = shiftHours
	}
	for i, gate := range f.config.LeadGates {
		if gate == oldCode {
			f.config.LeadGates[i] = newCode
		}
	}
	return nil
}

type fakeScheduleRepository struct {
	saveRunID      int64
	latestResult   domain.SolveResult
	latestFound    bool
	approveCount   int
	unapproveCount int
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
func (f *fakeScheduleRepository) UnapproveAssignments(context.Context, domain.Date, domain.Date) (int, error) {
	return f.unapproveCount, nil
}

var (
	_ port.SolverGateway      = (*fakeSolverGateway)(nil)
	_ port.EmployeeRepository = (*fakeEmployeeRepository)(nil)
	_ port.ConfigRepository   = (*fakeConfigRepository)(nil)
	_ port.ScheduleRepository = (*fakeScheduleRepository)(nil)
)
