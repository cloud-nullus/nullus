package port

import "context"

// StackSummary contains the minimal information the CI/CD module needs
// from the Stack context. This avoids importing stack/domain directly,
// keeping the Bounded Context boundary intact.
type StackSummary struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	ClusterID string `json:"cluster_id"`
	State     string `json:"state"` // "completed", "failed", etc.

	// Namespace 는 스택이 설치된 네임스페이스다. GitLab/Argo CD 의 클러스터 내
	// 주소를 만들 때 쓴다.
	Namespace string `json:"namespace"`
	// SourceRepository / ContainerRegistry 는 스택이 고른 도구 이름이다
	// (예: "GitLab CE", "GitLab Registry", "Harbor").
	// 이미지 저장 위치를 결정하는 근거가 된다.
	SourceRepository  string `json:"source_repository"`
	ContainerRegistry string `json:"container_registry"`
	// AccessDomain 은 스택의 외부 접근 도메인이다. 비어 있을 수 있다.
	AccessDomain string `json:"access_domain"`
}

// StackReader provides read-only access to Stack context data.
// This is a Port (in Hexagonal Architecture terms) that the CI/CD
// context uses to validate cross-context references without
// importing the Stack domain directly.
type StackReader interface {
	// GetStackSummary retrieves minimal stack information by ID.
	// Returns nil and no error if the stack does not exist.
	GetStackSummary(ctx context.Context, stackID string) (*StackSummary, error)
}
