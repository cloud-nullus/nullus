package domain

import "fmt"

// AppError represents a structured application error.
type AppError struct {
	Code       string `json:"code"`
	HTTPStatus int    `json:"http_status"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
	Retryable  bool   `json:"retryable"`
	TraceID    string `json:"trace_id,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Detail)
}

// NewAppError creates a new AppError.
func NewAppError(code string, httpStatus int, message, detail string, retryable bool) *AppError {
	return &AppError{
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    message,
		Detail:     detail,
		Retryable:  retryable,
	}
}

// Common errors.
var (
	ErrNotFound     = NewAppError("NOT_FOUND", 404, "Resource not found", "", false)
	ErrUnauthorized = NewAppError("UNAUTHORIZED", 401, "Authentication required", "", false)
	ErrForbidden    = NewAppError("FORBIDDEN", 403, "Access denied", "", false)
	ErrValidation   = NewAppError("VALIDATION_ERROR", 422, "Validation failed", "", false)
)
