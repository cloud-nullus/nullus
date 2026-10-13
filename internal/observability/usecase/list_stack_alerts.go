package usecase

import (
	"context"

	"github.com/cloud-nullus/draft/internal/observability/domain"
)

const (
	defaultStackAlertLimit = 100
	maxStackAlertLimit     = 500
)

// StackAlertLister 는 조직의 스택 경보 기록이다(port.StackAlertRepository 의 조회 부분).
type StackAlertLister interface {
	List(ctx context.Context, orgID string, openOnly bool, limit int) ([]*domain.StackAlert, error)
}

// ListStackAlerts 는 알림 이력 화면의 스택 경보 목록이다.
type ListStackAlerts struct {
	repo StackAlertLister
}

func NewListStackAlerts(repo StackAlertLister) *ListStackAlerts {
	return &ListStackAlerts{repo: repo}
}

func (uc *ListStackAlerts) Execute(ctx context.Context, orgID string, openOnly bool, limit int) ([]*domain.StackAlert, error) {
	if limit <= 0 {
		limit = defaultStackAlertLimit
	}
	if limit > maxStackAlertLimit {
		limit = maxStackAlertLimit
	}
	return uc.repo.List(ctx, orgID, openOnly, limit)
}
