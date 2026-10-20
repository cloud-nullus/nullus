package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/observability/domain"
)

// 보안 대시보드(nullus-plan#65)의 입력이다. 결과 테이블은 cicd(앱 축)와 stack(설치 OSS 축)이 소유하고,
// adapter/bridge 가 두 모듈의 공개 경로를 관측 도메인 타입으로 옮긴다 — 관측 모듈은 그 테이블을 직접
// 읽지 않는다(설계 결정 10).

// AppSecuritySource 는 조직 앱(파이프라인)의 단계별 최신 보안 결과다. truncated 는 상한(1000)에 닿아
// 잘랐다는 뜻이다.
type AppSecuritySource interface {
	ListAppSecurity(ctx context.Context, orgID string) (apps []domain.AppSecurityRecord, truncated bool, err error)
}

// StackSecuritySource 는 조직 스택의 설치 이미지 스캔 보고서다.
type StackSecuritySource interface {
	ListStackImageSecurity(ctx context.Context, orgID string) ([]domain.StackImageInput, error)
}

// ScopeResolver 는 범위 필터와 묶음 머리에 쓰는 조직의 스택 · 클러스터 이름이다.
type ScopeResolver interface {
	// ListStacks 는 조직의 지우지 않은 스택이다.
	ListStacks(ctx context.Context, orgID string) ([]domain.SecurityStackRef, error)
	// ClusterNames 는 조직 클러스터의 ID → 이름이다.
	ClusterNames(ctx context.Context, orgID string) (map[string]string, error)
}
