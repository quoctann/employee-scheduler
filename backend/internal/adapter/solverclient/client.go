// Package solverclient implements port.SolverGateway against the stateless
// Python solver-service over HTTP.
package solverclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/port"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New builds a Client. transport, when non-nil, wraps the outbound HTTP
// requests (e.g. otelhttp.NewTransport, so a solve request's trace spans
// backend -> solver-service).
func New(baseURL, apiKey string, transport http.RoundTripper) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		// Solves can legitimately run close to MAX_TIME_LIMIT_S plus queueing
		// behind solver-service's concurrency semaphore.
		httpClient: &http.Client{Timeout: 5 * time.Minute, Transport: transport},
	}
}

var _ port.SolverGateway = (*Client)(nil)

// APIError is returned for any non-2xx or {success:false} response, carrying
// the HTTP status and solver-service's (plain-string, non-structured) error
// message so callers can branch on status code without string matching.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("solver-service responded %d: %s", e.StatusCode, e.Message)
}

// envelope mirrors solver-service's ApiResponse[T]: {success, data, error}.
type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *string         `json:"error"`
}

func (c *Client) Solve(ctx context.Context, req port.SolveRequest) (domain.SolveResult, error) {
	body := wireSolveRequest{
		StartDate:         req.StartDate,
		NumDays:           req.NumDays,
		Employees:         toWireEmployees(req.Employees),
		Availability:      nonNilAvailability(req.Availability),
		LockedAssignments: nonNilLocked(req.LockedAssignments),
		CarryIn:           req.CarryIn,
		Config:            req.Config,
		TimeLimitS:        req.TimeLimitS,
	}
	var out wireSolveResult
	if err := c.post(ctx, "/api/v1/solve", body, &out); err != nil {
		return domain.SolveResult{}, err
	}
	return domain.SolveResult{
		Status:          out.Status,
		ObjectiveValue:  out.ObjectiveValue,
		WallTimeS:       out.WallTimeS,
		Schedule:        out.Schedule,
		Shortages:       out.Shortages,
		EmployeeSummary: out.EmployeeSummary,
	}, nil
}

func (c *Client) CapacityCheck(ctx context.Context, req port.CapacityCheckRequest) (domain.CapacityCheckResult, error) {
	body := wireCapacityCheckRequest{
		NumDays:       req.NumDays,
		EmployeeCount: req.EmployeeCount,
		Config:        req.Config,
	}
	var out domain.CapacityCheckResult
	if err := c.post(ctx, "/api/v1/capacity-check", body, &out); err != nil {
		return domain.CapacityCheckResult{}, err
	}
	return out, nil
}

func (c *Client) ReplacementCandidates(ctx context.Context, req port.ReplacementCandidatesRequest) (domain.ReplacementCandidatesResult, error) {
	body := wireReplacementCandidatesRequest{
		StartDate:          req.StartDate,
		NumDays:            req.NumDays,
		Employees:          toWireEmployees(req.Employees),
		Availability:       nonNilAvailability(req.Availability),
		CurrentSchedule:    nonNilSchedule(req.CurrentSchedule),
		CarryIn:            req.CarryIn,
		TargetSlot:         req.TargetSlot,
		ExcludedEmployeeID: req.ExcludedEmployeeID,
		TopN:               req.TopN,
		Config:             req.Config,
	}
	var out domain.ReplacementCandidatesResult
	if err := c.post(ctx, "/api/v1/replacement-candidates", body, &out); err != nil {
		return domain.ReplacementCandidatesResult{}, err
	}
	return out, nil
}

func (c *Client) post(ctx context.Context, path string, reqBody, out any) error {
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request for %s: %w", path, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build request for %s: %w", path, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call solver-service %s: %w", path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read solver-service response from %s: %w", path, err)
	}

	var env envelope
	if jsonErr := json.Unmarshal(raw, &env); jsonErr != nil {
		return &APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("unparseable response body: %s", string(raw))}
	}

	if !env.Success {
		msg := "unknown solver-service error"
		if env.Error != nil {
			msg = *env.Error
		}
		return &APIError{StatusCode: resp.StatusCode, Message: msg}
	}

	if out == nil || len(env.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("decode solver-service data from %s: %w", path, err)
	}
	return nil
}

func nonNilAvailability(m domain.AvailabilityMap) domain.AvailabilityMap {
	if m == nil {
		return domain.AvailabilityMap{}
	}
	return m
}

func toWireEmployees(employees []domain.Employee) []wireEmployee {
	if employees == nil {
		return []wireEmployee{}
	}
	result := make([]wireEmployee, len(employees))
	for i, employee := range employees {
		result[i] = wireEmployee{
			EmployeeID: employee.EmployeeID,
			Name:       employee.Name,
			Role:       employee.Role,
			LeaveDays:  employee.LeaveDays,
		}
	}
	return result
}

func nonNilLocked(l []domain.LockedAssignment) []domain.LockedAssignment {
	if l == nil {
		return []domain.LockedAssignment{}
	}
	return l
}

func nonNilSchedule(s []domain.ScheduleEntry) []domain.ScheduleEntry {
	if s == nil {
		return []domain.ScheduleEntry{}
	}
	return s
}
