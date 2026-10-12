package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// DeployClusterID 는 파이프라인의 앱이 실제로 서 있는 클러스터다.
//
// 규칙은 domain.Pipeline.DeployClusterID 에 있다. 여기서는 묶인 스택의 클러스터를
// 읽어 넘긴다. 파이프라인의 클러스터를 읽으면 모니터링은 빈 네임스페이스를 보고,
// 삭제는 없는 곳에서 "이미 없음" 을 성공으로 끝내 앱이 남는다. 실제로 그랬다.
// 스택을 읽지 못하면 저장된 값이 가장 나은 추정이다.
func DeployClusterID(ctx context.Context, reader port.StackReader, pipeline *domain.Pipeline) string {
	if pipeline == nil {
		return ""
	}
	if reader == nil || !pipeline.DelegatesBuildToRunner() {
		return pipeline.ClusterID
	}
	summary, err := reader.GetStackSummary(ctx, pipeline.StackID)
	if err != nil || summary == nil {
		return pipeline.ClusterID
	}
	return pipeline.DeployClusterID(summary.ClusterID)
}

// MatchStackTarget 은 스택에 묶일 앱의 배포 클러스터를 정한다 — 생성과 직접 배포가
// 함께 쓴다. 스택이 없으면 ErrStackNotFound, 다른 조직의 스택이면 ErrStackOrgMismatch,
// 클러스터가 다르면 ErrStackClusterMismatch 다. 비어 있으면 스택 클러스터로 채운다.
//
// 조직을 먼저 본다. 클러스터가 다르다는 오류는 스택의 클러스터를 알려 주므로, 남의
// 조직에는 그보다 먼저 막는다.
func MatchStackTarget(stack *port.StackSummary, orgID, requested string) (string, error) {
	if stack == nil {
		return "", ErrStackNotFound
	}
	if stack.OrgID != orgID {
		return "", ErrStackOrgMismatch
	}
	cluster, err := domain.MatchStackCluster(stack.ClusterID, requested)
	if err != nil {
		return "", fmt.Errorf("stack %s: %w", stack.ID, err)
	}
	return cluster, nil
}
