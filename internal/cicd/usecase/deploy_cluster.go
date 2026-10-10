package usecase

import (
	"context"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// DeployClusterID 는 파이프라인의 앱이 실제로 서 있는 클러스터다.
//
// 스택에 묶인 파이프라인의 앱은 스택의 CD 도구(Argo CD)가 올린다. Argo CD 의
// Application 은 자기 클러스터(kubernetes.default.svc)를 향하므로 앱은 스택이
// 설치된 클러스터에 선다 — 파이프라인에 어느 클러스터가 적혀 있든. 파이프라인의
// 클러스터를 읽으면 모니터링은 빈 네임스페이스를 보고, 삭제는 없는 곳에서
// "이미 없음" 을 성공으로 끝내 앱이 남는다. 실제로 그랬다.
//
// 플랫폼이 직접 적용한 경로(스택 없음·긴급 직접 배포)는 파이프라인의 클러스터에
// 올렸으니 그 값을 그대로 쓴다. 스택을 읽지 못해도 저장된 값이 가장 나은 추정이다.
func DeployClusterID(ctx context.Context, reader port.StackReader, pipeline *domain.Pipeline) string {
	if pipeline == nil {
		return ""
	}
	if reader == nil || !pipeline.DelegatesBuildToRunner() {
		return pipeline.ClusterID
	}
	summary, err := reader.GetStackSummary(ctx, pipeline.StackID)
	if err != nil || summary == nil || strings.TrimSpace(summary.ClusterID) == "" {
		return pipeline.ClusterID
	}
	return summary.ClusterID
}
