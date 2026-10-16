package bridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	stackdomain "github.com/cloud-nullus/draft/internal/stack/domain"
)

// StackRepo 는 스택 모듈이 제공하는 스택 목록 창구다(internal/stack/port.StackRepository 의 List).
type StackRepo interface {
	List(ctx context.Context, orgID string, includeDeleted bool) ([]*stackdomain.Stack, error)
}

// StackLister 는 port.StackLister 를 스택 저장소로 구현한다 — 백업 세트가 "조직의 모든 스택" 을 여기서 읽는다.
type StackLister struct {
	repo StackRepo
}

func NewStackLister(repo StackRepo) *StackLister {
	return &StackLister{repo: repo}
}

// ListCompletedStacks 는 설치가 끝나 백업할 수 있는 스택만 준다.
//
// 설치 중·실패·롤백된 스택은 뜨지 않는다 — 반쯤 깔린 네임스페이스를 뜨면 복구가 그 반쪽을 되살리고,
// 네임스페이스가 없는 스택(DB 에만 남은 기록)은 어댑터가 닿을 곳이 없다. 저장소가 배선되지 않았으면
// 오류다 — 빈 목록을 돌려주면 "플랫폼만 뜬 세트" 가 전체 백업으로 보인다.
func (l *StackLister) ListCompletedStacks(ctx context.Context, orgID string) ([]domain.StackRef, error) {
	if l == nil || l.repo == nil {
		return nil, fmt.Errorf("스택 저장소가 배선되지 않아 조직 %s 의 스택을 읽을 수 없습니다", orgID)
	}
	stacks, err := l.repo.List(ctx, orgID, false)
	if err != nil {
		return nil, err
	}
	out := make([]domain.StackRef, 0, len(stacks))
	for _, s := range stacks {
		if s == nil || s.State != stackdomain.StateCompleted || strings.TrimSpace(s.Namespace) == "" {
			continue
		}
		out = append(out, domain.StackRef{ID: s.ID, Namespace: s.Namespace})
	}
	return out, nil
}
