package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
)

// SlogLogger returns an Echo middleware that logs requests using slog.
func SlogLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			req := c.Request()
			res := c.Response()

			status := res.Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}

			slog.Info("request",
				"method", req.Method,
				"path", req.URL.Path,
				"status", status,
				"latency_ms", time.Since(start).Milliseconds(),
				"request_id", res.Header().Get(echo.HeaderXRequestID),
				"remote_addr", req.RemoteAddr,
			)

			return err
		}
	}
}
