package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 스택의 Argo CD 가 올린 앱은 스택 클러스터에 선다 — 파이프라인에 다른 클러스터가
// 적혀 있어도(이 규칙이 생기기 전에 만든 파이프라인) 그렇다.
func TestPipeline_DeployClusterID_StackIntegratedUsesStackCluster(t *testing.T) {
	p := &Pipeline{StackID: "stk_1", ClusterID: "c-chosen", ExecutionMode: ExecutionModeStackIntegrated}
	assert.Equal(t, "c-stack", p.DeployClusterID("c-stack"))

	// 실행 모드를 비워 만든 스택 파이프라인도 통합 경로다.
	legacy := &Pipeline{StackID: "stk_1", ClusterID: "c-chosen"}
	assert.Equal(t, "c-stack", legacy.DeployClusterID(" c-stack "))
}

// 플랫폼이 직접 적용한 경로는 파이프라인에 적힌 클러스터에 올렸다.
// 스택 클러스터를 모르면 저장된 값이 가장 나은 추정이다.
func TestPipeline_DeployClusterID_UsesStoredCluster(t *testing.T) {
	cases := map[string]struct {
		pipeline     *Pipeline
		stackCluster string
	}{
		"스택 없이 만든 파이프라인": {&Pipeline{ClusterID: "c-chosen"}, "c-stack"},
		"긴급 직접 배포": {
			&Pipeline{StackID: "stk_1", ClusterID: "c-chosen", ExecutionMode: ExecutionModeEmergencyDirect},
			"c-stack",
		},
		"스택 클러스터를 모름": {&Pipeline{StackID: "stk_1", ClusterID: "c-chosen"}, " "},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "c-chosen", tc.pipeline.DeployClusterID(tc.stackCluster))
		})
	}
}

func TestPipeline_DeployClusterID_NilPipeline(t *testing.T) {
	var p *Pipeline
	assert.Empty(t, p.DeployClusterID("c-stack"))
}

// 스택에 묶인 앱의 클러스터는 스택 클러스터여야 한다. 비면 채우고, 다르면 거절한다 —
// 조용히 바꾸면 요청한 쪽이 고른 클러스터가 거짓이 된다.
func TestMatchStackCluster(t *testing.T) {
	got, err := MatchStackCluster("c-stack", "")
	assert.NoError(t, err)
	assert.Equal(t, "c-stack", got, "비어 있으면 스택 클러스터로 채운다")

	got, err = MatchStackCluster("c-stack", " c-stack ")
	assert.NoError(t, err)
	assert.Equal(t, "c-stack", got)

	_, err = MatchStackCluster("c-stack", "c-other")
	assert.True(t, errors.Is(err, ErrStackClusterMismatch))
	assert.Contains(t, err.Error(), "c-stack")
	assert.Contains(t, err.Error(), "c-other")

	// 스택 클러스터를 모르면 요청을 그대로 둔다.
	got, err = MatchStackCluster("", "c-other")
	assert.NoError(t, err)
	assert.Equal(t, "c-other", got)
}
