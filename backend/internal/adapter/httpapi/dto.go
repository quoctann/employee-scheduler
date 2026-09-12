// Package httpapi is the inbound HTTP adapter: it translates the Go
// backend's minimal REST contract into calls against the core services and
// wraps every response in the same {success,data,error} envelope
// solver-service uses.
package httpapi

import (
	"encoding/json"

	"github.com/labstack/echo/v4"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
)

// bindJSON decodes the request body as JSON regardless of its Content-Type
// header (unlike echo.Context.Bind, which 415s without an
// "application/json" Content-Type) — callers here are trusted internal
// clients (the bundled frontend), not arbitrary browsers submitting forms.
func bindJSON(c echo.Context, out any) error {
	return json.NewDecoder(c.Request().Body).Decode(out)
}

type response[T any] struct {
	Success bool    `json:"success"`
	Data    *T      `json:"data"`
	Error   *string `json:"error"`
}

func writeOK[T any](c echo.Context, status int, data T) error {
	return c.JSON(status, response[T]{Success: true, Data: &data})
}

func writeError(c echo.Context, status int, msg string) error {
	return c.JSON(status, response[any]{Success: false, Error: &msg})
}

// Request bodies reuse domain types directly (Date, TargetSlot,
// LockedAssignment already carry the exact wire-format JSON tags), so no
// parallel DTO structs are needed for the fields they cover.

type solveRequestBody struct {
	StartDate      domain.Date `json:"start_date"`
	NumDays        int         `json:"num_days"`
	TimeLimitS     int         `json:"time_limit_s"`
	IgnoreApproved bool        `json:"ignore_approved"`
}

type unapproveRequestBody struct {
	StartDate domain.Date `json:"start_date"`
	NumDays   int         `json:"num_days"`
}

type capacityCheckRequestBody struct {
	NumDays       int  `json:"num_days"`
	EmployeeCount *int `json:"employee_count"`
}

type candidatesRequestBody struct {
	TargetSlot         domain.TargetSlot `json:"target_slot"`
	ExcludedEmployeeID *string           `json:"excluded_employee_id"`
	TopN               int               `json:"top_n"`
}

type approveRequestBody struct {
	Assignments []domain.LockedAssignment `json:"assignments"`
}

type setAvailabilityRequestBody struct {
	Date domain.Date `json:"date"`
	domain.ShiftAvailability
}

type setLeaveDayRequestBody struct {
	Date    domain.Date `json:"date"`
	OnLeave bool        `json:"on_leave"`
}

type createEmployeeRequestBody struct {
	EmployeeID string      `json:"employee_id"`
	Name       string      `json:"name"`
	Role       domain.Role `json:"role"`
}

type updateEmployeeRequestBody struct {
	Name string      `json:"name"`
	Role domain.Role `json:"role"`
}

type updateGateShiftRequirementRequestBody struct {
	NV                int  `json:"nv"`
	Lead              int  `json:"lead"`
	LeadMandatoryRole bool `json:"lead_mandatory_role"`
	ShiftHours        int  `json:"shift_hours"`
}

type renameGateRequestBody struct {
	NewCode string `json:"new_code"`
}

type ackResponseBody struct {
	OK bool `json:"ok"`
}

type employeesResponseBody struct {
	Employees    []domain.Employee      `json:"employees"`
	Availability domain.AvailabilityMap `json:"availability"`
}

type latestScheduleResponseBody struct {
	Found  bool                `json:"found"`
	Result *domain.SolveResult `json:"result,omitempty"`
}

type approveResponseBody struct {
	ApprovedCount int `json:"approved_count"`
}

type unapproveResponseBody struct {
	UnapprovedCount int `json:"unapproved_count"`
}

type listApprovedResponseBody struct {
	Assignments []domain.LockedAssignment `json:"assignments"`
}
