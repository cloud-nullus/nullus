package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// ScanPolicyPipelinePublisher 는 새로 만든 스캔 파이프라인에 스택 정책을 싣는다.
// 스캔 단계가 없는 파이프라인에는 nil 을 돌려준다.
type ScanPolicyPipelinePublisher interface {
	PublishToPipeline(ctx context.Context, pipeline *domain.Pipeline) *ScanPolicyPush
}

// WithScanPolicyPublisher 는 파이프라인 생성 직후 스택의 스캔 정책을 싣도록 배선한다.
//
// 싣지 않으면 새 파이프라인은 스크립트 기본값으로 판정한다 — 운영자가 HIGH 를 막아 둔
// 스택에서 새 파이프라인만 HIGH 가 통과한다.
func (uc *CreatePipeline) WithScanPolicyPublisher(p ScanPolicyPipelinePublisher) *CreatePipeline {
	uc.scanPolicies = p
	return uc
}

// publishScanPolicy 는 새 파이프라인에 정책을 싣고, 실패하면 경고로 남긴다.
//
// 생성 자체를 실패시키지 않는다 — 저장소·CI job 은 이미 만들어졌고, 정책은 스택
// 정책을 다시 저장하면 재시도된다.
func (uc *CreatePipeline) publishScanPolicy(ctx context.Context, pipeline *domain.Pipeline, out *CreatePipelineOutput) {
	if uc.scanPolicies == nil {
		return
	}
	push := uc.scanPolicies.PublishToPipeline(ctx, pipeline)
	if push == nil || push.Status != ScanPolicyPushFailed {
		return
	}
	out.Warnings = append(out.Warnings, fmt.Sprintf(
		"이미지 스캔 정책을 파이프라인에 싣지 못해 기본 정책으로 판정합니다 — 스택 정책을 다시 저장하면 재시도됩니다: %s",
		push.Error))
}
