// Package httpapi is the inbound HTTP adapter: it translates the Go
// backend's minimal REST contract into calls against the core services and
// wraps every response in the same {success,data,error} envelope
// solver-service uses. Named httpapi (not "http") to avoid shadowing the
// stdlib net/http package this file imports.
package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

type response[T any] struct {
	Success bool    `json:"success"`
	Data    *T      `json:"data"`
	Error   *string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// Headers are already flushed at this point, so nothing more useful
		// than logging can be done — at least this response now shows up in
		// logs as truncated/broken instead of silently failing.
		log.Printf("write response body: %v", err)
	}
}

func writeOK[T any](w http.ResponseWriter, status int, data T) {
	writeJSON(w, status, response[T]{Success: true, Data: &data})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, response[any]{Success: false, Error: &msg})
}

// Request bodies reuse domain types directly (Date, TargetSlot,
// LockedAssignment already carry the exact wire-format JSON tags), so no
// parallel DTO structs are needed for the fields they cover.

type solveRequestBody struct {
	StartDate  domain.Date `json:"start_date"`
	NumDays    int         `json:"num_days"`
	TimeLimitS int         `json:"time_limit_s"`
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
