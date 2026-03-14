package handler

import "github.com/labstack/echo/v4"

// errorResponse writes a standard API error response.
func errorResponse(c echo.Context, httpStatus int, code, message string) error {
	return c.JSON(httpStatus, map[string]any{
		"error": map[string]any{
			"code":        code,
			"http_status": httpStatus,
			"message":     message,
		},
	})
}
