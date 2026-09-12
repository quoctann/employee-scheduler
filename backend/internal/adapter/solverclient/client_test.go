package solverclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/port"
)

func mustDate(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return d
}

func TestClient_Solve_SendsExpectedRequestAndDecodesResult(t *testing.T) {
	var gotPath, gotAPIKey string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("X-API-Key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"status":"OPTIMAL","objective_value":1.5,"wall_time_s":0.2,"schedule":[{"employee_id":"NV01","date":"2026-09-07","gate":"A","shift":"sang"}],"shortages":[],"employee_summary":[]},"error":null}`))
	}))
	defer server.Close()

	client := New(server.URL, "test-key", nil)

	// Deliberately pass nil slices/maps to verify they're normalized to `[]`/`{}`
	// rather than JSON `null`, which solver-service's required fields reject.
	result, err := client.Solve(context.Background(), port.SolveRequest{
		StartDate:  mustDate(t, "2026-09-07"),
		NumDays:    1,
		TimeLimitS: 5,
		Employees:  []domain.Employee{{EmployeeID: "NV01", Name: "Nhan vien 01", Role: domain.RoleNV, Active: false}},
	})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	if gotPath != "/api/v1/solve" {
		t.Fatalf("expected path /api/v1/solve, got %s", gotPath)
	}
	if gotAPIKey != "test-key" {
		t.Fatalf("expected X-API-Key header to be sent, got %q", gotAPIKey)
	}
	if gotBody["employees"] == nil {
		t.Fatalf("expected employees to be [] not omitted/null, got body %+v", gotBody)
	}
	if _, ok := gotBody["employees"].([]any); !ok {
		t.Fatalf("expected employees to serialize as a JSON array, got %T: %v", gotBody["employees"], gotBody["employees"])
	}
	employee := gotBody["employees"].([]any)[0].(map[string]any)
	if _, ok := employee["active"]; ok {
		t.Fatalf("solver request must not expose management-only active state: %+v", employee)
	}
	if _, ok := gotBody["locked_assignments"].([]any); !ok {
		t.Fatalf("expected locked_assignments to serialize as a JSON array, got %T: %v", gotBody["locked_assignments"], gotBody["locked_assignments"])
	}
	if gotBody["availability"] == nil {
		t.Fatalf("expected availability to be {} not null, got body %+v", gotBody)
	}

	if result.Status != domain.StatusOptimal {
		t.Fatalf("expected status OPTIMAL, got %s", result.Status)
	}
	if len(result.Schedule) != 1 || result.Schedule[0].EmployeeID != "NV01" {
		t.Fatalf("expected decoded schedule entry, got %+v", result.Schedule)
	}
}

func TestClient_Solve_ReturnsAPIErrorOnFailureEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"success":false,"data":null,"error":"num_days must be >= 1"}`))
	}))
	defer server.Close()

	client := New(server.URL, "test-key", nil)
	_, err := client.Solve(context.Background(), port.SolveRequest{StartDate: mustDate(t, "2026-09-07"), NumDays: 1})

	var apiErr *APIError
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !asAPIError(t, err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", apiErr.StatusCode)
	}
	if apiErr.Message != "num_days must be >= 1" {
		t.Fatalf("expected solver error message forwarded, got %q", apiErr.Message)
	}
}

func TestClient_CapacityCheck_DecodesResultDirectly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{"demand_hours_per_day":140,"demand_hours_total":3920,"target_hours_per_employee":176,"min_employees_required":22.27,"employee_count":21,"is_sufficient":false,"shortfall_ratio":0.06},"error":null}`))
	}))
	defer server.Close()

	client := New(server.URL, "test-key", nil)
	result, err := client.CapacityCheck(context.Background(), port.CapacityCheckRequest{NumDays: 28, EmployeeCount: 21})
	if err != nil {
		t.Fatalf("CapacityCheck() error = %v", err)
	}
	if result.DemandHoursPerDay != 140 || result.IsSufficient {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// asAPIError is a small helper since we don't want a dependency on errors.As
// boilerplate scattered across the test file.
func asAPIError(t *testing.T, err error, target **APIError) bool {
	t.Helper()
	apiErr, ok := err.(*APIError)
	if ok {
		*target = apiErr
	}
	return ok
}
