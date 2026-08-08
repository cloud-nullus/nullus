package port

import "context"

// ProjectVariable 은 파이프라인에 등록할 CI/CD 변수다.
type ProjectVariable struct {
	Key   string
	Value string
	// Masked 는 job 로그에서 값을 가린다. 자격증명에는 반드시 켠다.
	Masked bool
	// Protected 를 켜면 보호 브랜치에서만 노출된다.
	Protected bool
}

// AccessTokenSpec 은 프로젝트 범위 토큰 발급 요청이다.
type AccessTokenSpec struct {
	Name   string
	Scopes []string
	// ExpiresInDays 가 0 이면 구현체 기본값을 쓴다.
	ExpiresInDays int
}

// PipelineConfigurator 는 파이프라인 실행에 필요한 설정을 프로젝트에 건다.
//
// SCMProvisioner 와 분리한다 — 저장소 프로비저닝과 CI 설정은 플랫폼에 따라
// 제공 주체가 다를 수 있다(예: GitHub + 외부 CI).
type PipelineConfigurator interface {
	// SetProjectVariable 은 변수를 등록하거나 이미 있으면 갱신한다.
	SetProjectVariable(ctx context.Context, projectID string, v ProjectVariable) error
	// CreateProjectAccessToken 은 프로젝트 범위 토큰을 발급한다.
	// 값은 발급 시점에만 읽을 수 있다.
	CreateProjectAccessToken(ctx context.Context, projectID string, spec AccessTokenSpec) (string, error)
}
