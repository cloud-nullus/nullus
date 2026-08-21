package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 리소스를 하나씩 지우는 방식은 놓치는 것이 생긴다 — 실제로 볼륨이 남아 다음
// 설치가 옛 데이터베이스를 물려받았고, Gitea 의 28P01 과 Harbor 의 401 로 두 번
// 드러났다. 스택 몫의 네임스페이스는 통째로 회수한다.
func TestNamespaceReclaimable_ReclaimsTheStacksOwnNamespace(t *testing.T) {
	uc := NewDeleteStack(newFakeStackRepo(), nil, nil)
	uc.SetPlatformNamespace("nullus")
	stack := &domain.Stack{Name: "demo-stack", Namespace: domain.DefaultStackNamespaceFor("demo-stack")}

	assert.True(t, uc.namespaceReclaimable(context.Background(), stack))
}

// 플랫폼이 사는 자리를 지우면 플랫폼이 사라진다. 2026-08-20 에 실제로 그렇게 됐다.
func TestNamespaceReclaimable_NeverTouchesPlatformNamespace(t *testing.T) {
	uc := NewDeleteStack(newFakeStackRepo(), nil, nil)
	uc.SetPlatformNamespace("nullus")
	stack := &domain.Stack{Name: "demo-stack", Namespace: "nullus"}

	assert.False(t, uc.namespaceReclaimable(context.Background(), stack))
}

// 옛 스택들은 플랫폼과 같은 nullus 에 함께 살았다. 플랫폼 네임스페이스를 모르는
// 환경에서도 그 이름만은 지우지 않는다.
func TestNamespaceReclaimable_NeverTouchesLegacyDefault(t *testing.T) {
	uc := NewDeleteStack(newFakeStackRepo(), nil, nil)
	stack := &domain.Stack{Name: "demo-stack", Namespace: domain.DefaultStackNamespace}

	assert.False(t, uc.namespaceReclaimable(context.Background(), stack))
}

// 사용자가 직접 고른 자리는 다른 것과 함께 쓸 수 있다.
//
// 판정이 namespaceReclaimable 로 옮겨졌다. 접두사 없는 자리를 손대지 않는다는
// 규칙은 그대로다 — 그 부분이 이 테스트가 지키려던 것이다.
func TestNamespaceReclaimable_LeavesUserChosenNamespaces(t *testing.T) {
	uc := NewDeleteStack(newFakeStackRepo(), nil, nil)
	uc.SetPlatformNamespace("nullus")

	assert.False(t, uc.namespaceReclaimable(context.Background(),
		&domain.Stack{Name: "demo-stack", Namespace: "team-a"}))
}

func TestNamespaceReclaimable_IgnoresEmptyValues(t *testing.T) {
	uc := NewDeleteStack(newFakeStackRepo(), nil, nil)
	uc.SetPlatformNamespace("nullus")
	ctx := context.Background()

	assert.False(t, uc.namespaceReclaimable(ctx, nil))
	assert.False(t, uc.namespaceReclaimable(ctx, &domain.Stack{Name: "demo", Namespace: ""}))
	assert.False(t, uc.namespaceReclaimable(ctx, &domain.Stack{Name: "demo", Namespace: "default"}))
}
