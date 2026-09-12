package httpapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

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

func handleError(c echo.Context, logger *zap.Logger, err error) error {
	status := statusForError(err)
	var solverErr *solverclient.APIError
	if status >= http.StatusInternalServerError && !errors.As(err, &solverErr) {
		logger.Error("request failed", zap.Error(err))
		return writeError(c, status, "internal server error")
	}
	return writeError(c, status, err.Error())
}

// newHTTPErrorHandler keeps the {success,data,error} envelope for errors
// Echo itself produces (e.g. 404 on an unmatched route, 405 on a wrong
// method) instead of Echo's default plain-text/JSON error shape.
func newHTTPErrorHandler(logger *zap.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		status := http.StatusInternalServerError
		msg := "internal server error"
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			status = httpErr.Code
			if s, ok := httpErr.Message.(string); ok {
				msg = s
			}
		} else {
			logger.Error("unhandled error", zap.Error(err))
		}
		if writeErr := writeError(c, status, msg); writeErr != nil {
			logger.Error("write error response", zap.Error(writeErr))
		}
	}
}
