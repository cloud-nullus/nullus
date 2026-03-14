package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/labstack/echo/v4"
)

// AppErrorHandler converts AppError and echo.HTTPError into a standard JSON error response.
func AppErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	type fieldError struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}

	type errorBody struct {
		Code       string       `json:"code"`
		HTTPStatus int          `json:"http_status"`
		Message    string       `json:"message"`
		Detail     string       `json:"detail,omitempty"`
		Fields     []fieldError `json:"fields,omitempty"`
		Retryable  bool         `json:"retryable"`
		TraceID    string       `json:"trace_id,omitempty"`
	}

	type errorResponse struct {
		Error errorBody `json:"error"`
	}

	requestID := c.Response().Header().Get(echo.HeaderXRequestID)

	var (
		httpStatus int
		body       errorBody
	)

	switch e := err.(type) {
	case *shareddomain.AppError:
		httpStatus = e.HTTPStatus
		body = errorBody{
			Code:       e.Code,
			HTTPStatus: e.HTTPStatus,
			Message:    e.Message,
			Detail:     e.Detail,
			Retryable:  e.Retryable,
			TraceID:    requestID,
		}
	case *echo.HTTPError:
		httpStatus = e.Code
		msg := http.StatusText(e.Code)
		if s, ok := e.Message.(string); ok {
			msg = s
		}
		// Treat 400 HTTP errors as validation errors for better client feedback
		code := "HTTP_ERROR"
		if e.Code == http.StatusBadRequest {
			code = "VALIDATION_ERROR"
		}
		body = errorBody{
			Code:       code,
			HTTPStatus: e.Code,
			Message:    msg,
			Retryable:  false,
			TraceID:    requestID,
		}
	default:
		// Check for echo binding errors (request body / query param validation)
		var bindErr *echo.BindingError
		if errors.As(err, &bindErr) {
			httpStatus = http.StatusBadRequest
			body = errorBody{
				Code:       "VALIDATION_ERROR",
				HTTPStatus: http.StatusBadRequest,
				Message:    "Request validation failed",
				Fields: []fieldError{
					{Field: bindErr.Field, Message: bindErr.Error()},
				},
				Retryable: false,
				TraceID:   requestID,
			}
		} else {
			httpStatus = http.StatusInternalServerError
			body = errorBody{
				Code:       "INTERNAL_SERVER_ERROR",
				HTTPStatus: http.StatusInternalServerError,
				Message:    "An unexpected error occurred",
				Retryable:  false,
				TraceID:    requestID,
			}
			slog.Error("unhandled error", "error", err, "request_id", requestID)
		}
	}

	if err := c.JSON(httpStatus, errorResponse{Error: body}); err != nil {
		slog.Error("failed to write error response", "error", err)
	}
}
