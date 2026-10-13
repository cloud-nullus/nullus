package domain

import (
	"errors"
	"fmt"
)

// ErrChannelNotFound 는 채널이 없거나 다른 조직의 것일 때다.
var ErrChannelNotFound = errors.New("notification channel not found")

// ValidationError 는 채널 설정 한 칸이 잘못됐다는 뜻이다. Field 는 API 의 필드 이름이다.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

func invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Reason: fmt.Sprintf(format, args...)}
}
