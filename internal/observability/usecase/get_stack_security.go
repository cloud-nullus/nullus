package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

// StackSecurityQuery 는 설치 OSS 축 조회의 조건이다.
type StackSecurityQuery struct {
	OrgID     string
	ClusterID string
	StackID   string
}

// StackSecurityView 는 설치 OSS 축 화면 하나다.
type StackSecurityView struct {
	GeneratedAt time.Time
	Summary     domain.StackSecuritySummary
	Stacks      []domain.StackSecurityPosture
}

// GetStackSecurity 는 보안 대시보드의 설치 OSS 축(스택 설치 이미지 보고)이다. admin · devops 만 부른다 —
// 지금 /stacks 를 쓸 수 없는 역할에 이 숫자를 보이지 않는다(설계 결정 11). 역할 검사는 라우트가 한다.
type GetStackSecurity struct {
	stacks port.StackSecuritySource
	scope  port.ScopeResolver
	now    func() time.Time
}

// NewGetStackSecurity 는 유스케이스를 만든다.
func NewGetStackSecurity(stacks port.StackSecuritySource, scope port.ScopeResolver) *GetStackSecurity {
	return &GetStackSecurity{stacks: stacks, scope: scope, now: time.Now}
}

// Execute 는 조직의 스택을 범위로 거르고 등급 · 지표를 계산한다.
func (uc *GetStackSecurity) Execute(ctx context.Context, q StackSecurityQuery) (*StackSecurityView, error) {
	if uc == nil || uc.stacks == nil || uc.scope == nil {
		return nil, errors.New("보안 대시보드 설치 OSS 축이 배선되지 않았습니다")
	}
	orgID := strings.TrimSpace(q.OrgID)
	if orgID == "" {
		return nil, errors.New("org_id 가 필요합니다")
	}
	inputs, err := uc.stacks.ListStackImageSecurity(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("스택 설치 이미지 보고서 조회 실패: %w", err)
	}
	names, err := uc.scope.ClusterNames(ctx, orgID)
	if err != nil {
		slog.Warn("보안 대시보드: 클러스터 이름을 읽지 못했습니다", "org_id", orgID, "error", err)
		names = map[string]string{}
	}

	now := uc.now()
	clusterID, stackID := strings.TrimSpace(q.ClusterID), strings.TrimSpace(q.StackID)
	postures := make([]domain.StackSecurityPosture, 0, len(inputs))
	for _, in := range inputs {
		if clusterID != "" && in.ClusterID != clusterID {
			continue
		}
		if stackID != "" && in.StackID != stackID {
			continue
		}
		p := domain.EvaluateStack(in, now)
		p.ClusterName = names[in.ClusterID]
		postures = append(postures, p)
	}
	domain.SortStackPostures(postures)
	return &StackSecurityView{GeneratedAt: now, Summary: domain.SummarizeStacks(postures), Stacks: postures}, nil
}
