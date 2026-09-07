package httpapi

import "net/http"

// NewRouter wires the minimal HTTP contract onto s and layers CORS, panic
// recovery, and request logging around it. No auth — demo scope.
func NewRouter(s *Server, corsOrigin string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", handleHealth)
	mux.HandleFunc("GET /api/v1/employees", s.handleGetEmployees)
	mux.HandleFunc("PUT /api/v1/employees/{employee_id}/availability", s.handleSetAvailability)
	mux.HandleFunc("PUT /api/v1/employees/{employee_id}/leave", s.handleSetLeaveDay)
	mux.HandleFunc("POST /api/v1/schedule/solve", s.handleSolve)
	mux.HandleFunc("GET /api/v1/schedule/latest", s.handleLatestSchedule)
	mux.HandleFunc("POST /api/v1/schedule/approve", s.handleApprove)
	mux.HandleFunc("POST /api/v1/capacity-check", s.handleCapacityCheck)
	mux.HandleFunc("POST /api/v1/candidates", s.handleCandidates)

	return withCORS(withRecover(withLogging(mux)), corsOrigin)
}
