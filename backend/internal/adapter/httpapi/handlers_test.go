package httpapi_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/tantq/employee-scheduler-backend/internal/adapter/httpapi"
	"github.com/tantq/employee-scheduler-backend/internal/adapter/solverclient"
	"github.com/tantq/employee-scheduler-backend/internal/adapter/xlsxexport"
	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
	"github.com/tantq/employee-scheduler-backend/internal/core/service"
)

func newTestServer(t *testing.T, solver *fakeSolverGateway, empRepo *fakeEmployeeRepository, schedRepo *fakeScheduleRepository, cfgRepo *fakeConfigRepository) http.Handler {
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
	if cfgRepo == nil {
		cfgRepo = &fakeConfigRepository{}
	}

	s := &httpapi.Server{
		Employees:  service.NewEmployeeService(empRepo),
		Config:     service.NewConfigService(cfgRepo),
		Schedule:   service.NewScheduleService(solver, empRepo, cfgRepo, schedRepo),
		Approve:    service.NewApproveService(schedRepo),
		Capacity:   service.NewCapacityService(solver, empRepo, cfgRepo),
		Candidates: service.NewCandidateService(solver, empRepo, cfgRepo, schedRepo),
		Export:     service.NewExportService(empRepo, cfgRepo, schedRepo, xlsxexport.New()),
		Logger:     zap.NewNop(),
	}
	return httpapi.NewRouter(s, "http://localhost:5173", zap.NewNop())
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
	router := newTestServer(t, nil, nil, nil, nil)
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
	router := newTestServer(t, nil, empRepo, nil, nil)

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

func TestHandleGetEmployees_FromTo_OverridesDefaultWindow(t *testing.T) {
	empRepo := &fakeEmployeeRepository{
		employees: []domain.Employee{{EmployeeID: "NV01", Name: "NV01", Role: domain.RoleNV}},
	}
	router := newTestServer(t, nil, empRepo, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/employees?from=2026-01-01&to=2026-01-07", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	wantFrom, _ := domain.ParseDate("2026-01-01")
	wantTo, _ := domain.ParseDate("2026-01-07")
	if empRepo.availabilityFrom != wantFrom || empRepo.availabilityTo != wantTo {
		t.Fatalf("requested window = [%v,%v], want [%v,%v]", empRepo.availabilityFrom, empRepo.availabilityTo, wantFrom, wantTo)
	}
}

func TestHandleGetEmployees_InvalidFrom_Returns400(t *testing.T) {
	router := newTestServer(t, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/employees?from=not-a-date", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetEmployees_ToBeforeFrom_Returns400(t *testing.T) {
	router := newTestServer(t, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/employees?from=2026-01-07&to=2026-01-01", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSolve_HappyPath_PersistsAndReturnsResult(t *testing.T) {
	solver := &fakeSolverGateway{solveResult: domain.SolveResult{Status: domain.StatusOptimal}}
	schedRepo := &fakeScheduleRepository{saveRunID: 7}
	router := newTestServer(t, solver, nil, schedRepo, nil)

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
	router := newTestServer(t, solver, nil, nil, nil)

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
	router := newTestServer(t, nil, nil, nil, nil)

	body, _ := json.Marshal(map[string]any{"start_date": "2026-09-07", "num_days": 0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/solve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleLatestSchedule_NoRunYet_ReturnsFoundFalse(t *testing.T) {
	router := newTestServer(t, nil, nil, &fakeScheduleRepository{latestFound: false}, nil)

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
	router := newTestServer(t, nil, nil, &fakeScheduleRepository{approveCount: 2}, nil)

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
	router := newTestServer(t, nil, empRepo, nil, nil)

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
	router := newTestServer(t, nil, empRepo, nil, nil)

	body, _ := json.Marshal(map[string]any{"date": "2026-09-07", "sang": true, "dem": false})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/employees/NV01/availability", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSetAvailability_InvalidBody_Returns400(t *testing.T) {
	router := newTestServer(t, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/employees/NV01/availability", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSetLeaveDay_HappyPath(t *testing.T) {
	empRepo := &fakeEmployeeRepository{}
	router := newTestServer(t, nil, empRepo, nil, nil)

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
	router := newTestServer(t, nil, empRepo, nil, nil)

	body, _ := json.Marshal(map[string]any{"date": "2026-09-07", "on_leave": true})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/employees/NV01/leave", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleEmployeeManagementRoutes(t *testing.T) {
	repo := &fakeEmployeeRepository{}
	router := newTestServer(t, nil, repo, nil, nil)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/employees", bytes.NewBufferString(`{"employee_id":"NV22","name":"Nhan vien 22","role":"NV"}`))
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%s", createRec.Code, createRec.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/employees/NV22", bytes.NewBufferString(`{"name":"Truong ca 22","role":"TC"}`))
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200; body=%s", updateRec.Code, updateRec.Body.String())
	}

	deactivateRec := httptest.NewRecorder()
	router.ServeHTTP(deactivateRec, httptest.NewRequest(http.MethodDelete, "/api/v1/employees/NV22", nil))
	if deactivateRec.Code != http.StatusOK {
		t.Fatalf("deactivate status = %d, want 200; body=%s", deactivateRec.Code, deactivateRec.Body.String())
	}

	restoreRec := httptest.NewRecorder()
	router.ServeHTTP(restoreRec, httptest.NewRequest(http.MethodPost, "/api/v1/employees/NV22/restore", nil))
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("restore status = %d, want 200; body=%s", restoreRec.Code, restoreRec.Body.String())
	}
}

func TestHandleEmployeeManagementReturnsExpectedErrors(t *testing.T) {
	repo := &fakeEmployeeRepository{createErr: port.ErrEmployeeConflict, setActiveErr: port.ErrEmployeeNotFound}
	router := newTestServer(t, nil, repo, nil, nil)

	duplicateRec := httptest.NewRecorder()
	router.ServeHTTP(duplicateRec, httptest.NewRequest(http.MethodPost, "/api/v1/employees", bytes.NewBufferString(`{"employee_id":"NV01","name":"Name","role":"NV"}`)))
	if duplicateRec.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", duplicateRec.Code)
	}

	missingRec := httptest.NewRecorder()
	router.ServeHTTP(missingRec, httptest.NewRequest(http.MethodDelete, "/api/v1/employees/NV99", nil))
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, want 404", missingRec.Code)
	}
}

func TestHandleGetConfig_ReturnsOK(t *testing.T) {
	router := newTestServer(t, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("config status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateGateShiftRequirement_HappyPath(t *testing.T) {
	router := newTestServer(t, nil, nil, nil, nil)

	body, _ := json.Marshal(map[string]any{"nv": 2, "lead": 1, "lead_mandatory_role": true, "shift_hours": 12})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/gates/B/shifts/dem", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	var config domain.SolverConfig
	if err := json.Unmarshal(env.Data, &config); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	got := config.Requirements["B"][domain.ShiftDem]
	if got.NV != 2 || got.Lead != 1 || !got.LeadMandatoryRole {
		t.Fatalf("updated requirement = %+v, want {NV:2 Lead:1 LeadMandatoryRole:true}", got)
	}
	if config.ShiftHours["B"][domain.ShiftDem] != 12 {
		t.Fatalf("updated shift_hours = %d, want 12", config.ShiftHours["B"][domain.ShiftDem])
	}
}

func TestHandleUpdateGateShiftRequirement_InvalidBody_Returns400(t *testing.T) {
	router := newTestServer(t, nil, nil, nil, nil)

	body, _ := json.Marshal(map[string]any{"nv": -1, "lead": 0, "lead_mandatory_role": false, "shift_hours": 12})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/gates/B/shifts/dem", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateGateShiftRequirement_UnknownGateShift_Returns404(t *testing.T) {
	cfgRepo := &fakeConfigRepository{updateErr: port.ErrGateShiftNotFound}
	router := newTestServer(t, nil, nil, nil, cfgRepo)

	body, _ := json.Marshal(map[string]any{"nv": 1, "lead": 0, "lead_mandatory_role": false, "shift_hours": 8})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/gates/Z/shifts/sang", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleRenameGate_HappyPath(t *testing.T) {
	cfgRepo := &fakeConfigRepository{config: domain.SolverConfig{
		Requirements: map[string]map[domain.ShiftType]domain.GateShiftRequirement{"B": {domain.ShiftDem: {NV: 1}}},
		ShiftHours:   map[string]map[domain.ShiftType]int{"B": {domain.ShiftDem: 12}},
	}}
	router := newTestServer(t, nil, nil, nil, cfgRepo)

	body, _ := json.Marshal(map[string]any{"new_code": "B2"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/gates/B/rename", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	var config domain.SolverConfig
	if err := json.Unmarshal(env.Data, &config); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if _, ok := config.Requirements["B"]; ok {
		t.Fatalf("renamed config still has old code %q: %+v", "B", config.Requirements)
	}
	if config.Requirements["B2"][domain.ShiftDem].NV != 1 {
		t.Fatalf("renamed config = %+v, want requirement to follow the new code", config.Requirements)
	}
}

func TestHandleRenameGate_InvalidBody_Returns400(t *testing.T) {
	router := newTestServer(t, nil, nil, nil, nil)

	body, _ := json.Marshal(map[string]any{"new_code": "bad/code"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/gates/A/rename", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleRenameGate_UnknownGate_Returns404(t *testing.T) {
	cfgRepo := &fakeConfigRepository{renameErr: port.ErrGateNotFound}
	router := newTestServer(t, nil, nil, nil, cfgRepo)

	body, _ := json.Marshal(map[string]any{"new_code": "B2"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/gates/Z/rename", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleRenameGate_ExistingNewCode_Returns409(t *testing.T) {
	cfgRepo := &fakeConfigRepository{renameErr: port.ErrGateConflict}
	router := newTestServer(t, nil, nil, nil, cfgRepo)

	body, _ := json.Marshal(map[string]any{"new_code": "B"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/gates/A/rename", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
}

func TestCORSPreflight_ReturnsNoContentWithHeaders(t *testing.T) {
	router := newTestServer(t, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/schedule/solve", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodDelete) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want DELETE", got)
	}
}

func TestHandleExportSchedule_NoLatestRun_ReturnsJSONErrorEnvelope(t *testing.T) {
	schedRepo := &fakeScheduleRepository{latestFound: false}
	router := newTestServer(t, nil, nil, schedRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/schedule/export", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Success {
		t.Fatalf("expected success=false, got %+v", env)
	}
}

func TestHandleExportSchedule_LatestRun_ReturnsXlsxAttachment(t *testing.T) {
	start, _ := domain.ParseDate("2026-09-07")
	schedRepo := &fakeScheduleRepository{
		latestFound: true,
		latestResult: domain.SolveResult{
			StartDate: start,
			NumDays:   7,
			Schedule:  []domain.ScheduleEntry{{EmployeeID: "NV01", Date: start, Gate: "A", Shift: domain.ShiftSang}},
		},
	}
	empRepo := &fakeEmployeeRepository{employees: []domain.Employee{{EmployeeID: "NV01", Name: "NV01", Role: domain.RoleNV}}}
	router := newTestServer(t, nil, empRepo, schedRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/schedule/export", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "lich-xep-ca_2026-09-07_7ngay.xlsx") {
		t.Fatalf("Content-Disposition = %q", got)
	}
	// .xlsx is a zip archive — every zip starts with the "PK" local-file-header signature.
	if body := rec.Body.Bytes(); len(body) < 2 || body[0] != 'P' || body[1] != 'K' {
		t.Fatalf("response body doesn't look like a zip/xlsx file (first bytes: %v)", body[:min(len(body), 8)])
	}
}
