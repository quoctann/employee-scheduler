package solverclient

import "github.com/tantq/employee-scheduler-backend/internal/core/domain"

// These wire-format structs carry the exact JSON field names solver-service
// expects on its request bodies. They stay local to this adapter so the core
// `port` package doesn't need to know about JSON at all — the domain types
// they embed (Employee, LockedAssignment, CarryIn, SolverConfig, ...) already
// carry the matching tags since they mirror solver-service's schemas 1:1.

type wireSolveRequest struct {
	StartDate         domain.Date               `json:"start_date"`
	NumDays           int                       `json:"num_days"`
	Employees         []wireEmployee            `json:"employees"`
	Availability      domain.AvailabilityMap    `json:"availability"`
	LockedAssignments []domain.LockedAssignment `json:"locked_assignments"`
	CarryIn           domain.CarryIn            `json:"carry_in"`
	Config            domain.SolverConfig       `json:"config"`
	TimeLimitS        int                       `json:"time_limit_s"`
}

type wireSolveResult struct {
	Status          domain.SolveStatus       `json:"status"`
	ObjectiveValue  *float64                 `json:"objective_value"`
	WallTimeS       float64                  `json:"wall_time_s"`
	Schedule        []domain.ScheduleEntry   `json:"schedule"`
	Shortages       []domain.ShortageItem    `json:"shortages"`
	EmployeeSummary []domain.EmployeeSummary `json:"employee_summary"`
}

type wireCapacityCheckRequest struct {
	NumDays       int                 `json:"num_days"`
	EmployeeCount int                 `json:"employee_count"`
	Config        domain.SolverConfig `json:"config"`
}

type wireReplacementCandidatesRequest struct {
	StartDate          domain.Date            `json:"start_date"`
	NumDays            int                    `json:"num_days"`
	Employees          []wireEmployee         `json:"employees"`
	Availability       domain.AvailabilityMap `json:"availability"`
	CurrentSchedule    []domain.ScheduleEntry `json:"current_schedule"`
	CarryIn            domain.CarryIn         `json:"carry_in"`
	TargetSlot         domain.TargetSlot      `json:"target_slot"`
	ExcludedEmployeeID *string                `json:"excluded_employee_id"`
	TopN               int                    `json:"top_n"`
	Config             domain.SolverConfig    `json:"config"`
}

// wireEmployee intentionally excludes management-only state such as Active.
type wireEmployee struct {
	EmployeeID string        `json:"employee_id"`
	Name       string        `json:"name"`
	Role       domain.Role   `json:"role"`
	LeaveDays  []domain.Date `json:"leave_days,omitempty"`
}
