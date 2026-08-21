package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// StackRepository defines the interface for stack persistence.
type StackRepository interface {
	Create(ctx context.Context, stack *domain.Stack) error
	GetByID(ctx context.Context, id string) (*domain.Stack, error)
	FindByID(ctx context.Context, id string) (*domain.Stack, error)
	List(ctx context.Context, orgID string, includeDeleted bool) ([]*domain.Stack, error)
	// ListInFlight 는 설치가 진행 중인 상태로 남아 있는 스택을 조직과 무관하게
	// 돌려준다. 끊긴 설치를 찾아내는 데 쓴다 — 그것은 조직 경계와 무관한 일이다.
	ListInFlight(ctx context.Context) ([]*domain.Stack, error)
	Update(ctx context.Context, stack *domain.Stack) error
	// TouchUpdatedAt 은 갱신 시각만 찍는다.
	//
	// 설치가 도는 동안 살아 있음을 알리는 데 쓴다. Update 를 쓰지 않는 이유는
	// 그쪽이 메모리에 든 스택 전체를 다시 쓰기 때문이다 — 오래된 사본으로
	// 하트비트를 찍으면 그 사이 다른 경로가 바꾼 값을 되돌린다.
	TouchUpdatedAt(ctx context.Context, stackID string) error
	UpdateTools(ctx context.Context, stack *domain.Stack) error
	Delete(ctx context.Context, id string) error
}

// TemplateRepository defines the interface for template persistence.
type TemplateRepository interface {
	Create(ctx context.Context, template *domain.Template) error
	Update(ctx context.Context, template *domain.Template) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*domain.Template, error)
	List(ctx context.Context) ([]*domain.Template, error)
}
