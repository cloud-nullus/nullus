package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 스택에 묶인 파이프라인의 앱은 스택의 Argo CD 가 자기 클러스터에 배포한다.
// 화면에서 고른 클러스터가 다르게 저장돼 있어도 앱이 있는 곳은 스택 클러스터다.
func TestDeployClusterID_StackIntegratedUsesStackCluster(t *testing.T) {
	reader := &stubStackReader{summary: &port.StackSummary{ID: "stk_1", ClusterID: "c-stack"}}
	pipeline := &domain.Pipeline{
		ID: "pip_1", StackID: "stk_1", ClusterID: "c-chosen",
		ExecutionMode: domain.ExecutionModeStackIntegrated,
	}

	assert.Equal(t, "c-stack", DeployClusterID(context.Background(), reader, pipeline))
}

// 플랫폼이 직접 적용한 경로는 파이프라인에 적힌 클러스터에 앱을 올렸다.
// 스택 조회가 안 되는 상황에서도 저장된 값이 가장 나은 추정이다.
func TestDeployClusterID_FallsBackToPipelineCluster(t *testing.T) {
	stackSummary := &port.StackSummary{ID: "stk_1", ClusterID: "c-stack"}
	cases := map[string]struct {
		reader   port.StackReader
		pipeline *domain.Pipeline
	}{
		"스택 없이 만든 파이프라인": {
			reader:   &stubStackReader{summary: stackSummary},
			pipeline: &domain.Pipeline{ClusterID: "c-chosen"},
		},
		"긴급 직접 배포": {
			reader: &stubStackReader{summary: stackSummary},
			pipeline: &domain.Pipeline{
				StackID: "stk_1", ClusterID: "c-chosen",
				ExecutionMode: domain.ExecutionModeEmergencyDirect,
			},
		},
		"스택 조회가 배선되지 않음": {
			reader:   nil,
			pipeline: &domain.Pipeline{StackID: "stk_1", ClusterID: "c-chosen"},
		},
		"스택 조회 실패": {
			reader:   &stubStackReader{err: errors.New("db down")},
			pipeline: &domain.Pipeline{StackID: "stk_1", ClusterID: "c-chosen"},
		},
		"스택이 사라짐": {
			reader:   &stubStackReader{summary: nil},
			pipeline: &domain.Pipeline{StackID: "stk_1", ClusterID: "c-chosen"},
		},
		"스택에 클러스터가 없음": {
			reader:   &stubStackReader{summary: &port.StackSummary{ID: "stk_1"}},
			pipeline: &domain.Pipeline{StackID: "stk_1", ClusterID: "c-chosen"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "c-chosen", DeployClusterID(context.Background(), tc.reader, tc.pipeline))
		})
	}
}

func TestDeployClusterID_NilPipeline(t *testing.T) {
	assert.Empty(t, DeployClusterID(context.Background(), &stubStackReader{}, nil))
}
