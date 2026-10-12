package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// ErrPipelineNotStackBound 는 스택에 묶이지 않은 파이프라인의 정의를 갱신하려 한 경우다.
// 저장소도 CI 도 플랫폼이 만든 것이 아니라 다시 쓸 정의가 없다.
var ErrPipelineNotStackBound = errors.New("pipeline is not bound to a stack")

// RefreshPipelineDefinition 은 기존 파이프라인의 CI 정의 파일을 플랫폼 기본으로 다시 쓴다.
//
// 스캐폴딩은 새 저장소에만 쓰인다 — 기존 저장소에 다시 쓰면 사용자가 고친 Dockerfile·매니페스트와
// CI 가 갱신한 이미지 태그가 되돌아간다. 그래서 SAST 리포트처럼 정의 파일이 새로 하는 일은 그
// 전에 만든 파이프라인에 붙지 않았고, 파이프라인을 다시 만드는 것 말고는 방법이 없었다. 이
// 유스케이스는 정의 파일 하나와 CI 변수만 다시 쓴다. 사용자가 손본 정의 파일은 덮어쓴다 —
// 호출부(화면)가 그 사실을 묻는다.
type RefreshPipelineDefinition struct {
	pipelines    port.PipelineRepository
	provisioner  RepositoryProvisioner
	scanPolicies ScanPolicyPipelinePublisher
}

func NewRefreshPipelineDefinition(pipelines port.PipelineRepository, provisioner RepositoryProvisioner) *RefreshPipelineDefinition {
	return &RefreshPipelineDefinition{pipelines: pipelines, provisioner: provisioner}
}

// WithScanPolicyPublisher 는 갱신 뒤 스택의 스캔 정책을 다시 싣도록 배선한다 — 새 정의에
// 스캔 단계가 생겼으면 정책도 있어야 한다.
func (uc *RefreshPipelineDefinition) WithScanPolicyPublisher(p ScanPolicyPipelinePublisher) *RefreshPipelineDefinition {
	uc.scanPolicies = p
	return uc
}

// RefreshPipelineDefinitionOutput 은 갱신 결과다.
type RefreshPipelineDefinitionOutput struct {
	Pipeline *domain.Pipeline
	// Stages 는 다시 쓴 정의의 단계다. 비면 번들이 알려 주지 않은 것이다.
	Stages           []string
	Warnings         []string
	MissingVariables []string
	// RepositoryRecreated 는 저장소가 없어 다시 만들고 전체 스캐폴딩을 썼음을 알린다.
	RepositoryRecreated bool
}

func (uc *RefreshPipelineDefinition) Execute(ctx context.Context, pipelineID string) (*RefreshPipelineDefinitionOutput, error) {
	id := strings.TrimSpace(pipelineID)
	if id == "" {
		return nil, fmt.Errorf("pipeline id is required")
	}
	if uc.provisioner == nil {
		return nil, fmt.Errorf("저장소 프로비저닝이 배선되지 않았습니다")
	}
	pipeline, err := uc.pipelines.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("pipeline %s: %w", id, err)
	}
	if pipeline == nil {
		return nil, fmt.Errorf("pipeline %s not found", id)
	}
	if strings.TrimSpace(pipeline.StackID) == "" {
		return nil, ErrPipelineNotStackBound
	}

	// 레코드만으로 프로비저닝 입력을 다시 만든다. 포트·replica 는 배포 매니페스트 몫이라
	// 정의 갱신에는 쓰이지 않고, 레지스트리 자격증명은 스택 것이면 번들이 다시 푼다.
	out, err := uc.provisioner.Execute(ctx, ProvisionPipelineRepositoryInput{
		AppName:               pipeline.Name,
		StackID:               pipeline.StackID,
		TemplateID:            pipeline.TemplateID,
		AppType:               pipeline.AppType,
		Namespace:             pipeline.Namespace,
		RefreshDefinitionOnly: true,
	})
	if err != nil {
		return nil, fmt.Errorf("refresh pipeline definition for %s: %w", pipeline.Name, err)
	}

	result := &RefreshPipelineDefinitionOutput{
		Pipeline:            pipeline,
		Stages:              out.Stages,
		Warnings:            out.Warnings,
		MissingVariables:    out.MissingVariables,
		RepositoryRecreated: out.RepositoryRecreated,
	}
	// 단계가 바뀌었으면 레코드를 맞춘다 — 화면은 이 단계로 실행 기록을 보여 준다.
	// 비어 오면 그대로 둔다. 비워 두면 화면이 템플릿으로 떨어진다.
	if len(out.Stages) > 0 && !slices.Equal(pipeline.Stages, out.Stages) {
		pipeline.Stages = out.Stages
		if err := uc.pipelines.Update(ctx, pipeline); err != nil {
			return nil, fmt.Errorf("update pipeline stages: %w", err)
		}
	}
	if uc.scanPolicies != nil {
		if push := uc.scanPolicies.PublishToPipeline(ctx, pipeline); push != nil && push.Status == ScanPolicyPushFailed {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"이미지 스캔 정책을 파이프라인에 싣지 못해 기본 정책으로 판정합니다 — 스택 정책을 다시 저장하면 재시도됩니다: %s",
				push.Error))
		}
	}
	return result, nil
}
