package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.uber.org/zap"

	"github.com/quoctann/employee-scheduler-backend/internal/platform/logging"
)

// NewRouter wires the minimal HTTP contract onto s and layers CORS, panic
// recovery, tracing, and request logging around it. No auth — demo scope.
func NewRouter(s *Server, corsOrigin string, logger *zap.Logger) http.Handler {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = newHTTPErrorHandler(logger)

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{corsOrigin},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderContentType},
	}))
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			logger.Error("panic",
				zap.String("method", c.Request().Method),
				zap.String("path", c.Path()),
				zap.Error(err),
				zap.ByteString("stack", stack),
			)
			return writeError(c, http.StatusInternalServerError, "internal server error")
		},
	}))
	e.Use(otelecho.Middleware("employee-scheduler-backend"))
	e.Use(logging.Middleware(logger))
	e.Use(middleware.BodyLimit(maxRequestBodySize))

	g := e.Group("/api/v1")
	g.GET("/health", handleHealth)

	g.GET("/employees", s.handleGetEmployees)
	g.POST("/employees", s.handleCreateEmployee)
	g.PUT("/employees/:employee_id", s.handleUpdateEmployee)
	g.DELETE("/employees/:employee_id", s.handleDeactivateEmployee)
	g.POST("/employees/:employee_id/restore", s.handleRestoreEmployee)
	g.PUT("/employees/:employee_id/availability", s.handleSetAvailability)
	g.PUT("/employees/:employee_id/leave", s.handleSetLeaveDay)

	g.GET("/config", s.handleGetConfig)
	g.PUT("/config/gates/:gate_code/shifts/:shift_type", s.handleUpdateGateShiftRequirement)
	g.PUT("/config/gates/:gate_code/rename", s.handleRenameGate)

	g.POST("/schedule/solve", s.handleSolve)
	g.GET("/schedule/latest", s.handleLatestSchedule)
	g.POST("/schedule/approve", s.handleApprove)
	g.POST("/schedule/unapprove", s.handleUnapprove)
	g.GET("/schedule/approved", s.handleListApproved)
	g.GET("/schedule/export", s.handleExportSchedule)

	g.POST("/capacity-check", s.handleCapacityCheck)

	g.POST("/candidates", s.handleCandidates)

	return e
}
