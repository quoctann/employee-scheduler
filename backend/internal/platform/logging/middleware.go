package logging

import (
	"errors"
	"time"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Middleware logs one line per request (method, path, status, latency) plus
// trace_id/span_id when a span is active on the request context, so log
// lines in Loki correlate with traces in Tempo. It must run after any
// tracing middleware so the span is already active on the request context.
func Middleware(logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)

			req := c.Request()
			res := c.Response()
			// For a matched route, the handler has already written the
			// response (res.Status is accurate) by the time next(c) returns.
			// For an unmatched route/method, Echo's own HTTPErrorHandler
			// only runs *after* the whole middleware chain returns, so
			// res.Status is still its zero value here — resolve the status
			// from the returned *echo.HTTPError instead in that case.
			status := res.Status
			var httpErr *echo.HTTPError
			if errors.As(err, &httpErr) {
				status = httpErr.Code
			}
			path := c.Path()
			if path == "" {
				path = req.URL.Path // unmatched route: no registered pattern to report
			}
			fields := []zap.Field{
				zap.String("method", req.Method),
				zap.String("path", path),
				zap.Int("status", status),
				zap.Duration("latency", time.Since(start)),
			}
			if sc := trace.SpanContextFromContext(req.Context()); sc.IsValid() {
				fields = append(fields,
					zap.String("trace_id", sc.TraceID().String()),
					zap.String("span_id", sc.SpanID().String()),
				)
			}
			if err != nil {
				fields = append(fields, zap.Error(err))
			}
			logger.Info("request", fields...)

			return err
		}
	}
}
