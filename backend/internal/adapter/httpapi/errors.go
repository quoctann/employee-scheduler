package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/tantq/employee-scheduler-backend/internal/adapter/solverclient"
	"github.com/tantq/employee-scheduler-backend/internal/core/service"
)

// statusForError maps a service-layer error to an HTTP status: our own input
// validation is a 400; anything solver-service itself rejected is forwarded
// with its original status code; everything else (DB/network failures) is a
// 500. Error messages are returned as-is — fine for a local demo, but real
// deployments should stop doing that (see docs/mvp_production_gaps.md).
func statusForError(err error) int {
	if errors.Is(err, service.ErrInvalidInput) {
		return http.StatusBadRequest
	}
	if errors.Is(err, service.ErrNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, service.ErrConflict) {
		return http.StatusConflict
	}
	var apiErr *solverclient.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return http.StatusInternalServerError
}

func handleError(w http.ResponseWriter, err error) {
	status := statusForError(err)
	var solverErr *solverclient.APIError
	if status >= http.StatusInternalServerError && !errors.As(err, &solverErr) {
		log.Printf("request failed: %v", err)
		writeError(w, status, "internal server error")
		return
	}
	writeError(w, status, err.Error())
}
