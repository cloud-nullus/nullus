package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// PipelineRepository defines the interface for pipeline persistence.
type PipelineRepository interface {
	Create(ctx context.Context, pipeline *domain.Pipeline) error
	GetByID(ctx context.Context, id string) (*domain.Pipeline, error)
	List(ctx context.Context, orgID string) ([]*domain.Pipeline, error)
	ListByStackID(ctx context.Context, stackID string) ([]*domain.Pipeline, error)
	Update(ctx context.Context, pipeline *domain.Pipeline) error
	Delete(ctx context.Context, id string) error
}

// SyncablePipelineLister 는 스택에 묶인 파이프라인을 조직과 무관하게 돌려준다.
// 실행 기록 주기 동기화가 쓴다 — 그것은 조직 경계와 무관한 일이다.
type SyncablePipelineLister interface {
	ListWithStack(ctx context.Context) ([]*domain.Pipeline, error)
}

// PipelineTemplateRepository defines the interface for pipeline template persistence.
type PipelineTemplateRepository interface {
	GetByID(ctx context.Context, id string) (*domain.PipelineTemplate, error)
	List(ctx context.Context) ([]*domain.PipelineTemplate, error)
	Create(ctx context.Context, tmpl *domain.PipelineTemplate) error
	Update(ctx context.Context, tmpl *domain.PipelineTemplate) error
	Delete(ctx context.Context, id string) error
}

// DeploymentRepository defines the interface for deployment persistence.
type DeploymentRepository interface {
	Create(ctx context.Context, deployment *domain.Deployment) error
	GetByID(ctx context.Context, id string) (*domain.Deployment, error)
	ListByPipelineID(ctx context.Context, pipelineID string) ([]*domain.Deployment, error)
	Update(ctx context.Context, deployment *domain.Deployment) error
}

// ImageScanResultRepository 는 이미지 스캔 결과를 보관한다.
//
// cicd 모듈이 소유한다. 대시보드(#65)는 이 테이블을 직접 조회하지 않고 cicd 의
// 공개 경로로 읽는다.
type ImageScanResultRepository interface {
	// Upsert 는 같은 ID 면 덮어쓴다. 같은 실행을 여러 번 동기화해도 기록이
	// 늘지 않아야 한다.
	Upsert(ctx context.Context, result *domain.ImageScanResult) error
	// ListByPipelineID 는 최신 결과부터 돌려준다.
	ListByPipelineID(ctx context.Context, pipelineID string) ([]*domain.ImageScanResult, error)
	// Delete 는 기록을 지운다. 없으면 오류가 아니다 — 동기화가 반복해서 부른다.
	Delete(ctx context.Context, id string) error
}

// CICDGoldenPathRepository defines the interface for CI/CD Golden Path persistence.
type CICDGoldenPathRepository interface {
	GetByID(ctx context.Context, id string) (*domain.CICDGoldenPath, error)
	List(ctx context.Context) ([]*domain.CICDGoldenPath, error)
	Create(ctx context.Context, goldenPath *domain.CICDGoldenPath) error
	Update(ctx context.Context, goldenPath *domain.CICDGoldenPath) error
	Delete(ctx context.Context, id string) error
}
