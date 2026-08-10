package port

import "context"

type TokenSourceInput struct {
	OrgID         string
	Module        string
	Provider      string
	Path          string
	TokenType     string
	Status        string
	SecretManager string
	TokenValue    string
	// StackID 가 있으면 그 스택의 시크릿 저장소에 기록한다.
	//
	// OpenBao 는 스택마다 배포되므로 전역 저장소에 써 두면 스택 범위로 읽는
	// 쪽(cicd 모듈)이 값을 찾지 못한다. 비면 예전처럼 전역 저장소를 쓴다.
	StackID string
	// Metadata 는 토큰과 함께 보관할 접속 정보다 (예: GitHub organization).
	// 토큰만으로는 어디에 붙어야 할지 알 수 없는 외부 SaaS 에 필요하다.
	Metadata map[string]string
	// ClusterID / Namespace 는 회전 후 반영(rolling restart) 대상을 찾는 데 쓴다.
	ClusterID string
	Namespace string
}

// TokenSourceRegistry tracks OpenBao token metadata for stack integrations.
type TokenSourceRegistry interface {
	Upsert(ctx context.Context, input TokenSourceInput) error
}
