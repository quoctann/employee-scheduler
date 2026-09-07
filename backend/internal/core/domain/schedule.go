package domain

import "fmt"

// SolveStatus mirrors solver-service's status enum.
type SolveStatus string

const (
	StatusOptimal      SolveStatus = "OPTIMAL"
	StatusFeasible     SolveStatus = "FEASIBLE"
	StatusInfeasible   SolveStatus = "INFEASIBLE"
	StatusUnknown      SolveStatus = "UNKNOWN"
	StatusModelInvalid SolveStatus = "MODEL_INVALID"
)

type ScheduleEntry struct {
	EmployeeID string    `json:"employee_id"`
	Date       Date      `json:"date"`
	Gate       string    `json:"gate"`
	Shift      ShiftType `json:"shift"`
}

type ShortageType string

const (
	ShortageStaff ShortageType = "staff"
	ShortageLead  ShortageType = "lead"
)

type ShortageItem struct {
	Date         Date         `json:"date"`
	Gate         string       `json:"gate"`
	Shift        ShiftType    `json:"shift"`
	ShortageType ShortageType `json:"shortage_type"`
	Missing      int          `json:"missing"`
}

type EmployeeSummary struct {
	EmployeeID     string `json:"employee_id"`
	Role           Role   `json:"role"`
	LeaveDays      int    `json:"leave_days"`
	TargetHours    int    `json:"target_hours"`
	ActualHours    int    `json:"actual_hours"`
	DeviationHours int    `json:"deviation_hours"`
	ActualShifts   int    `json:"actual_shifts"`
}

// SolveResult mirrors solver-service's SolveResult, plus a RunID once persisted
// by ScheduleRepository (zero value before the first save).
type SolveResult struct {
	RunID           int64             `json:"run_id,omitempty"`
	StartDate       Date              `json:"start_date"`
	NumDays         int               `json:"num_days"`
	Status          SolveStatus       `json:"status"`
	ObjectiveValue  *float64          `json:"objective_value"`
	WallTimeS       float64           `json:"wall_time_s"`
	Schedule        []ScheduleEntry   `json:"schedule"`
	Shortages       []ShortageItem    `json:"shortages"`
	EmployeeSummary []EmployeeSummary `json:"employee_summary"`
}

// LockedAssignment mirrors solver-service's LockedAssignment. Off implies
// Gate/Shift are both nil; otherwise both must be set (same shape validator
// as the solver's Pydantic model). Validate enforces this before it ever
// reaches the DB's own CHECK constraint, so a bad request maps to a clean
// 400 instead of a raw driver error.
type LockedAssignment struct {
	EmployeeID string     `json:"employee_id"`
	Date       Date       `json:"date"`
	Gate       *string    `json:"gate"`
	Shift      *ShiftType `json:"shift"`
	Off        bool       `json:"off"`
}

// Validate enforces the off-XOR-gate+shift shape invariant.
func (a LockedAssignment) Validate() error {
	if a.Off {
		if a.Gate != nil || a.Shift != nil {
			return fmt.Errorf("%s/%s: off=true requires gate and shift to be empty", a.EmployeeID, a.Date)
		}
		return nil
	}
	if a.Gate == nil || a.Shift == nil {
		return fmt.Errorf("%s/%s: off=false requires both gate and shift to be set", a.EmployeeID, a.Date)
	}
	return nil
}

// CarryIn mirrors solver-service's CarryIn: employee_ids who worked the night
// shift on the day before the horizon's start_date.
type CarryIn struct {
	WorkedNightBeforeStart []string `json:"worked_night_before_start,omitempty"`
}

type TargetSlot struct {
	Date         Date      `json:"date"`
	Gate         string    `json:"gate"`
	Shift        ShiftType `json:"shift"`
	RequiresLead bool      `json:"requires_lead"`
}

type CandidateItem struct {
	EmployeeID        string   `json:"employee_id"`
	Role              Role     `json:"role"`
	Score             float64  `json:"score"`
	TargetHours       int      `json:"target_hours"`
	ActualHoursSoFar  int      `json:"actual_hours_so_far"`
	DeviationHours    int      `json:"deviation_hours"`
	WouldCreateStreak bool     `json:"would_create_streak"`
	Reasons           []string `json:"reasons"`
}

type ReplacementCandidatesResult struct {
	TargetSlot    TargetSlot      `json:"target_slot"`
	Candidates    []CandidateItem `json:"candidates"`
	ExcludedCount int             `json:"excluded_count"`
}

type CapacityCheckResult struct {
	DemandHoursPerDay      int     `json:"demand_hours_per_day"`
	DemandHoursTotal       int     `json:"demand_hours_total"`
	TargetHoursPerEmployee int     `json:"target_hours_per_employee"`
	MinEmployeesRequired   float64 `json:"min_employees_required"`
	EmployeeCount          int     `json:"employee_count"`
	IsSufficient           bool    `json:"is_sufficient"`
	ShortfallRatio         float64 `json:"shortfall_ratio"`
}
