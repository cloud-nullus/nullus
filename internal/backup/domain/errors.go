package domain

import (
	"net/http"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 복구는 파괴적이라, 진행하면 안 되는 상황은 에러 코드로 구분해 화면까지
// 그대로 전달한다. "왜 막혔는지" 를 모르면 사용자는 우회를 시도한다.

func ErrPrerequisitesMissing(detail string) error {
	return &shareddomain.AppError{
		Code:       "BACKUP_PREREQUISITES_MISSING",
		HTTPStatus: http.StatusPreconditionFailed,
		Message:    "복구 전제 조건이 충족되지 않았습니다",
		Detail:     detail,
	}
}

func ErrSchemaVersionMismatch(detail string) error {
	return &shareddomain.AppError{
		Code:       "BACKUP_SCHEMA_VERSION_MISMATCH",
		HTTPStatus: http.StatusConflict,
		Message:    "백업본의 스키마 버전이 현재 코드와 맞지 않습니다",
		Detail:     detail,
	}
}

func ErrChecksumMismatch(detail string) error {
	return &shareddomain.AppError{
		Code:       "BACKUP_CHECKSUM_MISMATCH",
		HTTPStatus: http.StatusConflict,
		Message:    "백업본 무결성 검증에 실패했습니다",
		Detail:     detail,
	}
}

func ErrBackupNotFound(id string) error {
	return &shareddomain.AppError{
		Code:       "BACKUP_NOT_FOUND",
		HTTPStatus: http.StatusNotFound,
		Message:    "백업을 찾을 수 없습니다",
		Detail:     id,
	}
}

func ErrDestinationUnavailable(detail string) error {
	return &shareddomain.AppError{
		Code:       "BACKUP_DESTINATION_UNAVAILABLE",
		HTTPStatus: http.StatusFailedDependency,
		Message:    "백업 목적지에 접근할 수 없습니다",
		Detail:     detail,
	}
}

// ErrInvalidComponent 는 모르는 백업 대상을 거부한다.
//
// 이름을 Detail 에 그대로 담는다 — 어느 값이 문제인지 말해 주지 않으면
// 사용자는 다섯 개 중 무엇을 고쳐야 할지 알 수 없다.
func ErrInvalidComponent(component string) error {
	return &shareddomain.AppError{
		Code:       "BACKUP_INVALID_COMPONENT",
		HTTPStatus: http.StatusBadRequest,
		Message:    "지원하지 않는 백업 대상입니다",
		Detail:     component,
	}
}

func ErrInvalidMode(mode string) error {
	return &shareddomain.AppError{
		Code:       "BACKUP_INVALID_MODE",
		HTTPStatus: http.StatusBadRequest,
		Message:    "지원하지 않는 백업 모드입니다",
		Detail:     mode,
	}
}
