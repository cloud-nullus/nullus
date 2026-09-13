package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// ScanPolicyPushStatus 는 파이프라인 하나에 정책을 실은 결과다.
type ScanPolicyPushStatus string

const (
	ScanPolicyPushApplied ScanPolicyPushStatus = "applied"
	ScanPolicyPushFailed  ScanPolicyPushStatus = "failed"
)

// ScanPolicyPush 는 파이프라인 하나에 대한 정책 푸시 결과다.
type ScanPolicyPush struct {
	PipelineID   string               `json:"pipeline_id"`
	PipelineName string               `json:"pipeline_name"`
	Status       ScanPolicyPushStatus `json:"status"`
	Error        string               `json:"error,omitempty"`
}

// ScanPolicyView 는 스택의 현재 정책이다.
type ScanPolicyView struct {
	StackID string            `json:"stack_id"`
	Policy  domain.ScanPolicy `json:"policy"`
	// IsDefault 는 저장한 적이 없어 기본 정책을 쓰고 있다는 뜻이다.
	IsDefault bool `json:"is_default"`
}

// UpdateScanPolicyResult 는 저장한 정책과 파이프라인별 푸시 결과다.
type UpdateScanPolicyResult struct {
	ScanPolicyView
	Pushes []ScanPolicyPush `json:"pushes"`
}

// ScanPolicyService 는 스택의 이미지 스캔 정책을 저장하고 파이프라인에 싣는다.
//
// 판정은 CI 가 스스로 한다. 플랫폼은 정책이 바뀔 때와 스캔 파이프라인이 새로 생길 때
// 정책 값을 CI 가 읽는 자리에 싣는다 — CI 가 플랫폼에 되묻는 인바운드 경로(와 그
// 경로의 기계 인증)를 두지 않기 위해서다.
type ScanPolicyService struct {
	policies  port.ScanPolicyRepository
	pipelines port.PipelineRepository
	bundles   port.SCMBundleFactory
}

// NewScanPolicyService 는 ScanPolicyService 를 만든다.
func NewScanPolicyService(
	policies port.ScanPolicyRepository,
	pipelines port.PipelineRepository,
	bundles port.SCMBundleFactory,
) *ScanPolicyService {
	return &ScanPolicyService{policies: policies, pipelines: pipelines, bundles: bundles}
}

// Get 은 스택의 현재 정책이다. 저장한 적 없으면 기본 정책이고 IsDefault 가 참이다.
func (s *ScanPolicyService) Get(ctx context.Context, stackID string) (*ScanPolicyView, error) {
	id := strings.TrimSpace(stackID)
	if id == "" {
		return nil, fmt.Errorf("stack_id 가 필요합니다")
	}
	policy, found, err := s.policies.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return &ScanPolicyView{StackID: id, Policy: domain.DefaultScanPolicy(), IsDefault: true}, nil
	}
	return &ScanPolicyView{StackID: id, Policy: policy}, nil
}

// Update 는 정책을 검증·저장한 뒤 그 스택의 스캔 파이프라인에 싣는다.
//
// 푸시 실패로 저장을 되돌리지 않는다. 저장된 정책이 기준이고, 실패한 파이프라인은
// 결과에 남겨 다시 저장하거나 원인을 고치게 한다 — 되돌리면 도달 가능한 파이프라인도
// 새 정책을 못 받는다.
func (s *ScanPolicyService) Update(
	ctx context.Context,
	stackID string,
	policy domain.ScanPolicy,
	actor string,
) (*UpdateScanPolicyResult, error) {
	id := strings.TrimSpace(stackID)
	if id == "" {
		return nil, fmt.Errorf("stack_id 가 필요합니다")
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if err := s.policies.Upsert(ctx, id, policy, actor); err != nil {
		return nil, err
	}

	result := &UpdateScanPolicyResult{
		ScanPolicyView: ScanPolicyView{StackID: id, Policy: policy},
		Pushes:         []ScanPolicyPush{},
	}
	pipelines, err := s.pipelines.ListByStackID(ctx, id)
	if err != nil {
		return result, fmt.Errorf("정책은 저장했지만 스택의 파이프라인 목록을 읽지 못해 싣지 못했습니다: %w", err)
	}
	targets := make([]*domain.Pipeline, 0, len(pipelines))
	for _, p := range pipelines {
		if hasImageScanStage(p) {
			targets = append(targets, p)
		}
	}
	result.Pushes = s.publish(ctx, id, policy, targets)
	return result, nil
}

// PublishToPipeline 은 새로 만든 스캔 파이프라인에 스택의 현재 정책을 싣는다.
//
// 싣지 않으면 스크립트 기본값으로 돌아, 운영자가 HIGH 를 막아 둔 스택에서 새
// 파이프라인만 HIGH 가 통과한다. 스캔 단계가 없는 파이프라인은 nil 이다.
func (s *ScanPolicyService) PublishToPipeline(ctx context.Context, pipeline *domain.Pipeline) *ScanPolicyPush {
	if pipeline == nil || strings.TrimSpace(pipeline.StackID) == "" || !hasImageScanStage(pipeline) {
		return nil
	}
	view, err := s.Get(ctx, pipeline.StackID)
	if err != nil {
		// 정책을 못 읽으면 기본값으로 싣지 않는다 — 저장된 더 엄격한 정책을 덮어쓸 수 있다.
		return &ScanPolicyPush{
			PipelineID: pipeline.ID, PipelineName: pipeline.Name, Status: ScanPolicyPushFailed,
			Error: fmt.Sprintf("스택 정책을 읽지 못했습니다: %v", err),
		}
	}
	pushes := s.publish(ctx, pipeline.StackID, view.Policy, []*domain.Pipeline{pipeline})
	return &pushes[0]
}

// publish 는 대상 파이프라인에 정책을 싣고 파이프라인마다 결과를 돌려준다.
func (s *ScanPolicyService) publish(
	ctx context.Context,
	stackID string,
	policy domain.ScanPolicy,
	targets []*domain.Pipeline,
) []ScanPolicyPush {
	pushes := make([]ScanPolicyPush, 0, len(targets))
	if len(targets) == 0 {
		// 스택 도구에 묻지도 않는다 — 설치 중인 스택에서도 정책을 먼저 정할 수 있어야 한다.
		return pushes
	}
	failAll := func(msg string) []ScanPolicyPush {
		for _, p := range targets {
			pushes = append(pushes, ScanPolicyPush{
				PipelineID: p.ID, PipelineName: p.Name, Status: ScanPolicyPushFailed, Error: msg,
			})
		}
		return pushes
	}

	bundle, err := s.bundles.For(ctx, stackID)
	if err != nil {
		slog.Warn("스캔 정책: 스택 도구에 닿지 못했습니다", "stack_id", stackID, "error", err)
		return failAll(fmt.Sprintf("스택 도구에 닿지 못했습니다: %v", err))
	}
	if bundle == nil || bundle.ScanPolicy == nil {
		return failAll("이 스택에는 스캔 정책을 실을 경로가 배선되지 않았습니다")
	}

	apps := make([]string, 0, len(targets))
	for _, p := range targets {
		apps = append(apps, p.Name)
	}
	results := bundle.ScanPolicy.PublishScanPolicy(ctx, apps, port.ScanPolicyVariables(policy))
	for _, p := range targets {
		push := ScanPolicyPush{PipelineID: p.ID, PipelineName: p.Name, Status: ScanPolicyPushApplied}
		if err := results[p.Name]; err != nil {
			push.Status = ScanPolicyPushFailed
			push.Error = err.Error()
		}
		pushes = append(pushes, push)
	}
	return pushes
}

// hasImageScanStage 는 파이프라인이 스캔 단계를 가졌는지 본다. 단계 기록이 없는
// 옛 파이프라인은 스캔 단계가 없는 것으로 본다 — 스캔 단계가 생긴 뒤(#254)에
// 만든 파이프라인은 모두 단계를 기록한다.
func hasImageScanStage(p *domain.Pipeline) bool {
	if p == nil {
		return false
	}
	for _, s := range p.Stages {
		if port.StageKey(s) == imageScanStageKey {
			return true
		}
	}
	return false
}
