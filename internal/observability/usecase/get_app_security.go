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

// StackIDNone 은 스택 없는 앱(직접 배포 · 스택이 지워진 앱)을 고르는 stack_id 값이다.
const StackIDNone = "none"

// AppSecurityQuery 는 앱 축 조회의 조건이다. ClusterID · StackID · Query 는 범위 필터, ProblemOnly 는 표만
// 줄인다 — 지표 · 적용률 · 묶음 머리는 범위 필터만 따른다(설계 §3).
type AppSecurityQuery struct {
	OrgID       string
	ClusterID   string
	StackID     string
	Query       string
	ProblemOnly bool
}

// AppSecurityView 는 앱 축 화면 하나다.
type AppSecurityView struct {
	GeneratedAt time.Time
	Truncated   bool
	Summary     domain.AppSecuritySummary
	Coverage    domain.StageCoverage
	Groups      []domain.AppSecurityGroup
	// Apps 는 위험순 그대로다. 화면은 받은 순서를 쓰고 묶기만 한다.
	Apps []domain.AppSecurityPosture
}

// GetAppSecurity 는 보안 대시보드의 앱 축(파이프라인 보안 게이트)이다.
type GetAppSecurity struct {
	apps  port.AppSecuritySource
	scope port.ScopeResolver
	now   func() time.Time
}

// NewGetAppSecurity 는 유스케이스를 만든다.
func NewGetAppSecurity(apps port.AppSecuritySource, scope port.ScopeResolver) *GetAppSecurity {
	return &GetAppSecurity{apps: apps, scope: scope, now: time.Now}
}

// Execute 는 조직의 앱을 범위로 거르고, 셀 · 위험 순위 · 지표 · 묶음을 계산한다.
func (uc *GetAppSecurity) Execute(ctx context.Context, q AppSecurityQuery) (*AppSecurityView, error) {
	if uc == nil || uc.apps == nil || uc.scope == nil {
		return nil, errors.New("보안 대시보드 앱 축이 배선되지 않았습니다")
	}
	orgID := strings.TrimSpace(q.OrgID)
	if orgID == "" {
		return nil, errors.New("org_id 가 필요합니다")
	}
	records, truncated, err := uc.apps.ListAppSecurity(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("앱 보안 결과 조회 실패: %w", err)
	}
	stackRefs, err := uc.scope.ListStacks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("조직 스택 조회 실패: %w", err)
	}
	stacks := make(map[string]domain.SecurityStackRef, len(stackRefs))
	for _, s := range stackRefs {
		stacks[s.ID] = s
	}
	clusterNames := uc.clusterNames(ctx, orgID)

	now := uc.now()
	clusterID, stackID, query := strings.TrimSpace(q.ClusterID), strings.TrimSpace(q.StackID), strings.ToLower(strings.TrimSpace(q.Query))
	scoped := make([]domain.AppSecurityPosture, 0, len(records))
	for _, rec := range records {
		rec = withStackScope(rec, stacks)
		if clusterID != "" && rec.ClusterID != clusterID {
			continue
		}
		switch {
		case stackID == StackIDNone && rec.StackID != "":
			continue
		case stackID != "" && stackID != StackIDNone && rec.StackID != stackID:
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(rec.Name), query) {
			continue
		}
		scoped = append(scoped, domain.EvaluateApp(rec, now))
	}
	domain.SortAppPostures(scoped)

	view := &AppSecurityView{GeneratedAt: now, Truncated: truncated}
	view.Summary, view.Coverage = domain.SummarizeApps(scoped)
	view.Groups = domain.GroupAppsByStack(scoped, stacks, clusterNames)
	view.Apps = scoped
	if q.ProblemOnly {
		view.Apps = make([]domain.AppSecurityPosture, 0, len(scoped))
		for _, p := range scoped {
			if domain.IsProblemRank(p.Rank) {
				view.Apps = append(view.Apps, p)
			}
		}
	}
	return view, nil
}

// withStackScope 는 앱을 지우지 않은 스택에 묶는다. 스택이 지워진 앱(소프트 삭제로 stack_id 가 남은 것)은
// "스택 없음" 으로 둔다(설계 §2) — 이름 없는 묶음을 만들지 않는다. 클러스터는 스택의 것을 따르고, 스택이
// 없으면 파이프라인의 것이다.
func withStackScope(rec domain.AppSecurityRecord, stacks map[string]domain.SecurityStackRef) domain.AppSecurityRecord {
	if rec.StackID == "" {
		return rec
	}
	ref, ok := stacks[rec.StackID]
	if !ok {
		// 스택 없음에는 스택 정책도 없다 — 지워진 스택의 시크릿 off 가 남으면 셀이 이유 없이 "미적용" 이다.
		rec.StackID = ""
		rec.SecretPolicyOff = false
		return rec
	}
	if ref.ClusterID != "" {
		rec.ClusterID = ref.ClusterID
	}
	return rec
}

// clusterNames 는 클러스터 이름이다. 이름은 머리말일 뿐이라 읽지 못해도 화면을 실패시키지 않는다 — 비운다.
func (uc *GetAppSecurity) clusterNames(ctx context.Context, orgID string) map[string]string {
	names, err := uc.scope.ClusterNames(ctx, orgID)
	if err != nil {
		slog.Warn("보안 대시보드: 클러스터 이름을 읽지 못했습니다", "org_id", orgID, "error", err)
		return map[string]string{}
	}
	return names
}
