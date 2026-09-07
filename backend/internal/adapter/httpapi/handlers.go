package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tantq/employee-scheduler-backend/internal/core/service"
)

// Server holds the core services the HTTP handlers dispatch to. It has no
// dependency on any adapter (Postgres, solverclient) — only on `service`.
type Server struct {
	Employees  *service.EmployeeService
	Schedule   *service.ScheduleService
	Approve    *service.ApproveService
	Capacity   *service.CapacityService
	Candidates *service.CandidateService
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetEmployees(w http.ResponseWriter, r *http.Request) {
	employees, availability, err := s.Employees.Roster(r.Context())
	if err != nil {
		handleError(w, err)
		return
	}
	writeOK(w, http.StatusOK, employeesResponseBody{Employees: employees, Availability: availability})
}

func (s *Server) handleSolve(w http.ResponseWriter, r *http.Request) {
	var body solveRequestBody
	if !decodeJSON(w, r, &body) {
		return
	}
	result, err := s.Schedule.Solve(r.Context(), service.SolveParams{
		StartDate:  body.StartDate,
		NumDays:    body.NumDays,
		TimeLimitS: body.TimeLimitS,
	})
	if err != nil {
		handleError(w, err)
		return
	}
	writeOK(w, http.StatusOK, result)
}

func (s *Server) handleLatestSchedule(w http.ResponseWriter, r *http.Request) {
	result, found, err := s.Schedule.Latest(r.Context())
	if err != nil {
		handleError(w, err)
		return
	}
	body := latestScheduleResponseBody{Found: found}
	if found {
		body.Result = &result
	}
	writeOK(w, http.StatusOK, body)
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	var body approveRequestBody
	if !decodeJSON(w, r, &body) {
		return
	}
	count, err := s.Approve.Approve(r.Context(), body.Assignments)
	if err != nil {
		handleError(w, err)
		return
	}
	writeOK(w, http.StatusOK, approveResponseBody{ApprovedCount: count})
}

func (s *Server) handleCapacityCheck(w http.ResponseWriter, r *http.Request) {
	var body capacityCheckRequestBody
	if !decodeJSON(w, r, &body) {
		return
	}
	result, err := s.Capacity.Check(r.Context(), service.CapacityParams{
		NumDays:       body.NumDays,
		EmployeeCount: body.EmployeeCount,
	})
	if err != nil {
		handleError(w, err)
		return
	}
	writeOK(w, http.StatusOK, result)
}

func (s *Server) handleSetAvailability(w http.ResponseWriter, r *http.Request) {
	var body setAvailabilityRequestBody
	if !decodeJSON(w, r, &body) {
		return
	}
	employeeID := r.PathValue("employee_id")
	if err := s.Employees.SetAvailability(r.Context(), employeeID, body.Date, body.ShiftAvailability); err != nil {
		handleError(w, err)
		return
	}
	writeOK(w, http.StatusOK, ackResponseBody{OK: true})
}

func (s *Server) handleSetLeaveDay(w http.ResponseWriter, r *http.Request) {
	var body setLeaveDayRequestBody
	if !decodeJSON(w, r, &body) {
		return
	}
	employeeID := r.PathValue("employee_id")
	if err := s.Employees.SetLeaveDay(r.Context(), employeeID, body.Date, body.OnLeave); err != nil {
		handleError(w, err)
		return
	}
	writeOK(w, http.StatusOK, ackResponseBody{OK: true})
}

func (s *Server) handleCandidates(w http.ResponseWriter, r *http.Request) {
	var body candidatesRequestBody
	if !decodeJSON(w, r, &body) {
		return
	}
	result, err := s.Candidates.Suggest(r.Context(), service.CandidateParams{
		TargetSlot:         body.TargetSlot,
		ExcludedEmployeeID: body.ExcludedEmployeeID,
		TopN:               body.TopN,
	})
	if err != nil {
		handleError(w, err)
		return
	}
	writeOK(w, http.StatusOK, result)
}

// maxRequestBodyBytes bounds request bodies as a basic resource-exhaustion
// guard, independent of the (intentionally out of scope for this demo)
// auth/rate-limiting layer. Matches solver-service's own MAX_BODY_BYTES default.
const maxRequestBodyBytes = 10 << 20 // 10 MiB

// decodeJSON writes a 400 envelope and returns false on failure, so handlers
// can bail out in one line.
func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return false
	}
	return true
}
