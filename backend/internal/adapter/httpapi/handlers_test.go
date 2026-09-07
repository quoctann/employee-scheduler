package httpapi_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tantq/employee-scheduler-backend/internal/adapter/httpapi"
	"github.com/tantq/employee-scheduler-backend/internal/adapter/solverclient"
	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/service"
)

func newTestServer(t *testing.T, solver *fakeSolverGateway, empRepo *fakeEmployeeRepository, schedRepo *fakeScheduleRepository) http.Handler {
	t.Helper()
	if solver == nil {
		solver = &fakeSolverGateway{}
	}
	if empRepo == nil {
		empRepo = &fakeEmployeeRepository{}
	}
	if schedRepo == nil {
		schedRepo = &fakeScheduleRepository{}
	}
	cfgRepo := &fakeConfigRepository{}

	s := &httpapi.Server{
		Employees:  service.NewEmployeeService(empRepo),
		Schedule:   service.NewScheduleService(solver, empRepo, cfgRepo, schedRepo),
		Approve:    service.NewApproveService(schedRepo),
		Capacity:   service.NewCapacityService(solver, empRepo, cfgRepo),
		Candidates: service.NewCandidateService(solver, empRepo, cfgRepo, schedRepo),
	}
	return httpapi.NewRouter(s, "http://localhost:5173")
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *string         `json:"error"`
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode response envelope: %v (body: %s)", err, rec.Body.String())
	}
	return env
}

func TestHandleHealth_ReturnsOK(t *testing.T) {
	router := newTestServer(t, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	env := decodeEnvelope(t, rec)
	if !env.Success {
		t.Fatalf("expected success=true, got %+v", env)
	}
}

func TestHandleGetEmployees_ReturnsRosterFromRepository(t *testing.T) {
	empRepo := &fakeEmployeeRepository{
		employees: []domain.Employee{{EmployeeID: "NV01", Name: "NV01", Role: domain.RoleNV}},
	}
	router := newTestServer(t, nil, empRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/employees", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	var data struct {
		Employees []domain.Employee `json:"employees"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if len(data.Employees) != 1 || data.Employees[0].EmployeeID != "NV01" {
		t.Fatalf("unexpected employees: %+v", data.Employees)
	}
}

func TestHandleSolve_HappyPath_PersistsAndReturnsResult(t *testing.T) {
	solver := &fakeSolverGateway{solveResult: domain.SolveResult{Status: domain.StatusOptimal}}
	schedRepo := &fakeScheduleRepository{saveRunID: 7}
	router := newTestServer(t, solver, nil, schedRepo)

	body, _ := json.Marshal(map[string]any{
		"start_date":   "2026-09-07",
		"num_days":     3,
		"time_limit_s": 5,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/solve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	var result domain.SolveResult
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if result.RunID != 7 || result.Status != domain.StatusOptimal {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestHandleSolve_SolverFailure_ForwardsUpstreamStatusAndMessage(t *testing.T) {
	solver := &fakeSolverGateway{solveErr: &solverclient.APIError{StatusCode: http.StatusServiceUnavailable, Message: "solver is at capacity, please retry"}}
	router := newTestServer(t, solver, nil, nil)

	body, _ := json.Marshal(map[string]any{"start_date": "2026-09-07", "num_days": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/solve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body=%s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Success {
		t.Fatal("expected success=false")
	}
	wantSubstring := "solver-service responded 503: solver is at capacity, please retry"
	if env.Error == nil {
		t.Fatalf("expected error message containing %q, got nil", wantSubstring)
	}
	if !strings.Contains(*env.Error, wantSubstring) {
		t.Fatalf("error message = %q, want it to contain %q", *env.Error, wantSubstring)
	}
}

func TestHandleSolve_InvalidNumDays_Returns400(t *testing.T) {
	router := newTestServer(t, nil, nil, nil)

	body, _ := json.Marshal(map[string]any{"start_date": "2026-09-07", "num_days": 0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/solve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleLatestSchedule_NoRunYet_ReturnsFoundFalse(t *testing.T) {
	router := newTestServer(t, nil, nil, &fakeScheduleRepository{latestFound: false})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/schedule/latest", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	env := decodeEnvelope(t, rec)
	var data struct {
		Found bool `json:"found"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if data.Found {
		t.Fatal("expected found=false when no run has been saved")
	}
}

func TestHandleApprove_HappyPath(t *testing.T) {
	router := newTestServer(t, nil, nil, &fakeScheduleRepository{approveCount: 2})

	body, _ := json.Marshal(map[string]any{
		"assignments": []map[string]any{
			{"employee_id": "NV01", "date": "2026-09-07", "off": true},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/approve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	var data struct {
		ApprovedCount int `json:"approved_count"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if data.ApprovedCount != 2 {
		t.Fatalf("approved_count = %d, want 2", data.ApprovedCount)
	}
}

func TestHandleSetAvailability_HappyPath(t *testing.T) {
	empRepo := &fakeEmployeeRepository{}
	router := newTestServer(t, nil, empRepo, nil)

	body, _ := json.Marshal(map[string]any{"date": "2026-09-07", "sang": true, "dem": false})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/employees/NV01/availability", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if !env.Success {
		t.Fatalf("expected success=true, got %+v", env)
	}
}

func TestHandleSetAvailability_RepoError_Returns500(t *testing.T) {
	empRepo := &fakeEmployeeRepository{setAvailabilityErr: errors.New("db down")}
	router := newTestServer(t, nil, empRepo, nil)

	body, _ := json.Marshal(map[string]any{"date": "2026-09-07", "sang": true, "dem": false})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/employees/NV01/availability", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSetAvailability_InvalidBody_Returns400(t *testing.T) {
	router := newTestServer(t, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/employees/NV01/availability", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSetLeaveDay_HappyPath(t *testing.T) {
	empRepo := &fakeEmployeeRepository{}
	router := newTestServer(t, nil, empRepo, nil)

	body, _ := json.Marshal(map[string]any{"date": "2026-09-07", "on_leave": true})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/employees/NV01/leave", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if !env.Success {
		t.Fatalf("expected success=true, got %+v", env)
	}
}

func TestHandleSetLeaveDay_RepoError_Returns500(t *testing.T) {
	empRepo := &fakeEmployeeRepository{setLeaveDayErr: errors.New("db down")}
	router := newTestServer(t, nil, empRepo, nil)

	body, _ := json.Marshal(map[string]any{"date": "2026-09-07", "on_leave": true})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/employees/NV01/leave", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body=%s", rec.Code, rec.Body.String())
	}
}

func TestCORSPreflight_ReturnsNoContentWithHeaders(t *testing.T) {
	router := newTestServer(t, nil, nil, nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/schedule/solve", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}
