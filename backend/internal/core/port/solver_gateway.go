package port

import (
	"context"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
)

// SolveRequest is what SolverGateway.Solve sends to solver-service's POST /solve.
type SolveRequest struct {
	StartDate         domain.Date
	NumDays           int
	Employees         []domain.Employee
	Availability      domain.AvailabilityMap
	LockedAssignments []domain.LockedAssignment
	CarryIn           domain.CarryIn
	Config            domain.SolverConfig
	TimeLimitS        int
}

type CapacityCheckRequest struct {
	NumDays       int
	EmployeeCount int
	Config        domain.SolverConfig
}

type ReplacementCandidatesRequest struct {
	StartDate          domain.Date
	NumDays            int
	Employees          []domain.Employee
	Availability       domain.AvailabilityMap
	CurrentSchedule    []domain.ScheduleEntry
	CarryIn            domain.CarryIn
	TargetSlot         domain.TargetSlot
	ExcludedEmployeeID *string
	TopN               int
	Config             domain.SolverConfig
}

// SolverGateway is the outbound port to the stateless Python solver-service.
// The HTTP implementation lives in adapter/solverclient.
type SolverGateway interface {
	Solve(ctx context.Context, req SolveRequest) (domain.SolveResult, error)
	CapacityCheck(ctx context.Context, req CapacityCheckRequest) (domain.CapacityCheckResult, error)
	ReplacementCandidates(ctx context.Context, req ReplacementCandidatesRequest) (domain.ReplacementCandidatesResult, error)
}
