package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/service"
)

// maxRequestBodySize bounds request bodies as a basic resource-exhaustion
// guard, independent of the (intentionally out of scope for this demo)
// auth/rate-limiting layer. Matches solver-service's own MAX_BODY_BYTES default.
const maxRequestBodySize = "10M"

// Server holds the core services the HTTP handlers dispatch to, plus the
// logger used for error/panic logging. It has no dependency on any adapter
// (Postgres, solverclient) beyond that — only on `service`.
type Server struct {
	Logger     *zap.Logger
	Employees  *service.EmployeeService
	Config     *service.ConfigService
	Schedule   *service.ScheduleService
	Approve    *service.ApproveService
	Capacity   *service.CapacityService
	Candidates *service.CandidateService
	Export     *service.ExportService
}

func (s *Server) handleErr(c echo.Context, err error) error {
	return handleError(c, s.Logger, err)
}

func handleHealth(c echo.Context) error {
	return writeOK(c, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetEmployees(c echo.Context) error {
	includeInactive, err := parseIncludeInactive(c)
	if err != nil {
		return s.handleErr(c, err)
	}
	from, err := parseOptionalDate(c, "from")
	if err != nil {
		return s.handleErr(c, err)
	}
	to, err := parseOptionalDate(c, "to")
	if err != nil {
		return s.handleErr(c, err)
	}
	if !from.Time.IsZero() && !to.Time.IsZero() && to.Time.Before(from.Time) {
		return s.handleErr(c, fmt.Errorf("%w: to must not be before from", service.ErrInvalidInput))
	}
	employees, availability, err := s.Employees.Roster(c.Request().Context(), includeInactive, from, to)
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, employeesResponseBody{Employees: employees, Availability: availability})
}

func (s *Server) handleCreateEmployee(c echo.Context) error {
	var body createEmployeeRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	employee, err := s.Employees.Create(c.Request().Context(), domain.Employee{
		EmployeeID: body.EmployeeID,
		Name:       body.Name,
		Role:       body.Role,
	})
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusCreated, employee)
}

func (s *Server) handleUpdateEmployee(c echo.Context) error {
	var body updateEmployeeRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	employee, err := s.Employees.Update(c.Request().Context(), c.Param("employee_id"), body.Name, body.Role)
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, employee)
}

func (s *Server) handleDeactivateEmployee(c echo.Context) error {
	employee, err := s.Employees.Deactivate(c.Request().Context(), c.Param("employee_id"))
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, employee)
}

func (s *Server) handleRestoreEmployee(c echo.Context) error {
	employee, err := s.Employees.Restore(c.Request().Context(), c.Param("employee_id"))
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, employee)
}

func (s *Server) handleGetConfig(c echo.Context) error {
	config, err := s.Config.Get(c.Request().Context())
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, config)
}

func (s *Server) handleUpdateGateShiftRequirement(c echo.Context) error {
	var body updateGateShiftRequirementRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	config, err := s.Config.UpdateGateShiftRequirement(
		c.Request().Context(),
		c.Param("gate_code"),
		domain.ShiftType(c.Param("shift_type")),
		domain.GateShiftRequirement{NV: body.NV, Lead: body.Lead, LeadMandatoryRole: body.LeadMandatoryRole},
		body.ShiftHours,
	)
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, config)
}

func (s *Server) handleRenameGate(c echo.Context) error {
	var body renameGateRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	config, err := s.Config.RenameGate(c.Request().Context(), c.Param("gate_code"), body.NewCode)
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, config)
}

func parseIncludeInactive(c echo.Context) (bool, error) {
	raw := c.QueryParam("include_inactive")
	if raw == "" {
		return false, nil
	}
	includeInactive, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%w: include_inactive must be true or false", service.ErrInvalidInput)
	}
	return includeInactive, nil
}

// parseOptionalDate reads a "YYYY-MM-DD" query param, returning the zero
// Date (meaning "use the caller's default") when absent.
func parseOptionalDate(c echo.Context, name string) (domain.Date, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return domain.Date{}, nil
	}
	date, err := domain.ParseDate(raw)
	if err != nil {
		return domain.Date{}, fmt.Errorf("%w: %s must be a YYYY-MM-DD date", service.ErrInvalidInput, name)
	}
	return date, nil
}

func (s *Server) handleSolve(c echo.Context) error {
	var body solveRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	result, err := s.Schedule.Solve(c.Request().Context(), service.SolveParams{
		StartDate:      body.StartDate,
		NumDays:        body.NumDays,
		TimeLimitS:     body.TimeLimitS,
		IgnoreApproved: body.IgnoreApproved,
	})
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, result)
}

func (s *Server) handleLatestSchedule(c echo.Context) error {
	result, found, err := s.Schedule.Latest(c.Request().Context())
	if err != nil {
		return s.handleErr(c, err)
	}
	body := latestScheduleResponseBody{Found: found}
	if found {
		body.Result = &result
	}
	return writeOK(c, http.StatusOK, body)
}

func (s *Server) handleApprove(c echo.Context) error {
	var body approveRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	count, err := s.Approve.Approve(c.Request().Context(), body.Assignments)
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, approveResponseBody{ApprovedCount: count})
}

func (s *Server) handleUnapprove(c echo.Context) error {
	var body unapproveRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	count, err := s.Approve.Unapprove(c.Request().Context(), service.HorizonParams{
		StartDate: body.StartDate,
		NumDays:   body.NumDays,
	})
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, unapproveResponseBody{UnapprovedCount: count})
}

func (s *Server) handleListApproved(c echo.Context) error {
	raw := c.QueryParam("start_date")
	if raw == "" {
		return s.handleErr(c, fmt.Errorf("%w: start_date is required", service.ErrInvalidInput))
	}
	startDate, err := domain.ParseDate(raw)
	if err != nil {
		return s.handleErr(c, fmt.Errorf("%w: start_date must be a YYYY-MM-DD date", service.ErrInvalidInput))
	}
	numDays, err := strconv.Atoi(c.QueryParam("num_days"))
	if err != nil {
		return s.handleErr(c, fmt.Errorf("%w: num_days must be an integer", service.ErrInvalidInput))
	}
	assignments, err := s.Approve.ListApproved(c.Request().Context(), service.HorizonParams{
		StartDate: startDate,
		NumDays:   numDays,
	})
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, listApprovedResponseBody{Assignments: assignments})
}

// handleExportSchedule is the only endpoint that doesn't use the
// {success,data,error} JSON envelope — it streams a binary .xlsx workbook
// instead. Errors still go through handleError, so a missing schedule (or
// any other failure) still comes back as the usual JSON error envelope.
func (s *Server) handleExportSchedule(c echo.Context) error {
	fileBytes, filename, err := s.Export.Export(c.Request().Context())
	if err != nil {
		return s.handleErr(c, err)
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", fileBytes)
}

func (s *Server) handleCapacityCheck(c echo.Context) error {
	var body capacityCheckRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	result, err := s.Capacity.Check(c.Request().Context(), service.CapacityParams{
		NumDays:       body.NumDays,
		EmployeeCount: body.EmployeeCount,
	})
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, result)
}

func (s *Server) handleSetAvailability(c echo.Context) error {
	var body setAvailabilityRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	employeeID := c.Param("employee_id")
	if err := s.Employees.SetAvailability(c.Request().Context(), employeeID, body.Date, body.ShiftAvailability); err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, ackResponseBody{OK: true})
}

func (s *Server) handleSetLeaveDay(c echo.Context) error {
	var body setLeaveDayRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	employeeID := c.Param("employee_id")
	if err := s.Employees.SetLeaveDay(c.Request().Context(), employeeID, body.Date, body.OnLeave); err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, ackResponseBody{OK: true})
}

func (s *Server) handleCandidates(c echo.Context) error {
	var body candidatesRequestBody
	if err := bindJSON(c, &body); err != nil {
		return writeError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	result, err := s.Candidates.Suggest(c.Request().Context(), service.CandidateParams{
		TargetSlot:         body.TargetSlot,
		ExcludedEmployeeID: body.ExcludedEmployeeID,
		TopN:               body.TopN,
	})
	if err != nil {
		return s.handleErr(c, err)
	}
	return writeOK(c, http.StatusOK, result)
}
