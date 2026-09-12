package service

import (
	"context"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/port"
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
	includeInactive  bool
	availabilityErr  error
	availabilityFrom domain.Date
	availabilityTo   domain.Date

	setAvailabilityErr   error
	setAvailabilityCalls []domain.ShiftAvailability
	setLeaveDayErr       error
	setLeaveDayCalls     []bool

	createEmployee   domain.Employee
	createResult     domain.Employee
	createErr        error
	updateEmployeeID string
	updateName       string
	updateRole       domain.Role
	updateResult     domain.Employee
	updateErr        error
	setActiveID      string
	setActiveValue   bool
	setActiveResult  domain.Employee
	setActiveErr     error
}

func (f *fakeEmployeeRepository) List(_ context.Context, includeInactive bool) ([]domain.Employee, error) {
	f.includeInactive = includeInactive
	return f.employees, f.listErr
}

func (f *fakeEmployeeRepository) Create(_ context.Context, employee domain.Employee) (domain.Employee, error) {
	f.createEmployee = employee
	return f.createResult, f.createErr
}

func (f *fakeEmployeeRepository) Update(_ context.Context, employeeID, name string, role domain.Role) (domain.Employee, error) {
	f.updateEmployeeID = employeeID
	f.updateName = name
	f.updateRole = role
	return f.updateResult, f.updateErr
}

func (f *fakeEmployeeRepository) SetActive(_ context.Context, employeeID string, active bool) (domain.Employee, error) {
	f.setActiveID = employeeID
	f.setActiveValue = active
	return f.setActiveResult, f.setActiveErr
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

	updateErr      error
	lastUpdateGate string
	lastUpdateType domain.ShiftType
	lastUpdateReq  domain.GateShiftRequirement
	lastUpdateHrs  int

	renameErr     error
	lastRenameOld string
	lastRenameNew string
}

func (f *fakeConfigRepository) Get(_ context.Context) (domain.SolverConfig, error) {
	return f.config, f.err
}

func (f *fakeConfigRepository) UpdateGateShiftRequirement(_ context.Context, gateCode string, shiftType domain.ShiftType, requirement domain.GateShiftRequirement, shiftHours int) error {
	f.lastUpdateGate = gateCode
	f.lastUpdateType = shiftType
	f.lastUpdateReq = requirement
	f.lastUpdateHrs = shiftHours
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
	f.lastRenameOld = oldCode
	f.lastRenameNew = newCode
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

	unapproveCount int
	unapproveErr   error
	unapproveFrom  domain.Date
	unapproveTo    domain.Date
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

func (f *fakeScheduleRepository) UnapproveAssignments(_ context.Context, from, to domain.Date) (int, error) {
	f.unapproveFrom = from
	f.unapproveTo = to
	return f.unapproveCount, f.unapproveErr
}

type fakeExportRenderer struct {
	lastData     port.ScheduleExportData
	renderResult []byte
	renderErr    error
}

func (f *fakeExportRenderer) RenderScheduleWorkbook(data port.ScheduleExportData) ([]byte, error) {
	f.lastData = data
	return f.renderResult, f.renderErr
}

var (
	_ port.SolverGateway      = (*fakeSolverGateway)(nil)
	_ port.EmployeeRepository = (*fakeEmployeeRepository)(nil)
	_ port.ConfigRepository   = (*fakeConfigRepository)(nil)
	_ port.ScheduleRepository = (*fakeScheduleRepository)(nil)
	_ port.ExportRenderer     = (*fakeExportRenderer)(nil)
)
