package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// PipelineVariableWriter 는 비밀이 아닌 파이프라인 변수를 프로젝트에 싣는다.
//
// PipelineConfigurator.SetProjectVariable 과 나눈다 — GitHub 에서 그쪽은 Actions
// 시크릿이라, 워크플로가 ${{ vars.X }} 로 읽는 값을 넣을 수 없고 로그에서도 *** 로
// 가려진다. projectID 는 SCM 의 전체 경로다(GitLab 그룹/앱, GitHub owner/repo).
type PipelineVariableWriter interface {
	SetPipelineVariable(ctx context.Context, projectID, key, value string) error
}

// ScanPolicyPublisher 는 스캔 정책을 스택의 파이프라인들이 읽는 자리에 싣는다.
//
// 자리는 CI 마다 다르다 — GitLab·GitHub 은 앱 프로젝트의 변수, Jenkins 는 스택
// 네임스페이스의 ConfigMap 하나. 유스케이스가 그 차이를 알지 않도록 번들이 CI 에
// 맞는 구현을 쥔다. 결과는 앱마다 돌려준다 — 일부만 실패해도 어느 파이프라인이
// 옛 정책으로 도는지 알려야 한다.
type ScanPolicyPublisher interface {
	PublishScanPolicy(ctx context.Context, apps []string, vars []ProjectVariable) map[string]error
}

// ScanPolicyRepository 는 스택별 이미지 스캔 정책을 보관한다. cicd 모듈이 소유한다.
type ScanPolicyRepository interface {
	// Get 은 저장된 정책이다. 저장한 적 없으면 found=false 다 — 기본값을 지어
	// 돌려주면 호출부가 운영자가 고른 값인지 구분할 수 없다.
	Get(ctx context.Context, stackID string) (policy domain.ScanPolicy, found bool, err error)
	// Upsert 는 스택의 정책을 저장하거나 덮어쓴다.
	Upsert(ctx context.Context, stackID string, policy domain.ScanPolicy, updatedBy string) error
}
