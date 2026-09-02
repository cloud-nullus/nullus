package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 정지 백업의 최악 시나리오는 "백업하려다 서비스를 못 살리는 것" 이다
// (설계 §9 F3). 그래서 정지/재개 규칙은 클러스터 없이 전 경로를 검증한다.

func TestNewQuiescePlan_기록한_replica_로_되돌린다(t *testing.T) {
	plan := NewQuiescePlan([]Workload{
		{Kind: "Deployment", Namespace: "nullus", Name: "gitlab", Replicas: 3},
		{Kind: "StatefulSet", Namespace: "nullus", Name: "openbao", Replicas: 1},
	})

	require.Len(t, plan.Targets, 2)
	for _, tgt := range plan.Targets {
		assert.Equal(t, int32(0), tgt.ScaleDownTo, "정지는 언제나 0 이다")
	}
	assert.Equal(t, int32(3), plan.Targets[0].OriginalReplicas)
	assert.Equal(t, int32(1), plan.Targets[1].OriginalReplicas)
}

func TestNewQuiescePlan_0_인_워크로드는_되살리지_않는다(t *testing.T) {
	// 이미 0 인 것을 1 로 "복원" 하면 사용자가 꺼둔 것을 백업이 켜버린다.
	plan := NewQuiescePlan([]Workload{
		{Kind: "Deployment", Namespace: "nullus", Name: "scaled-down", Replicas: 0},
		{Kind: "Deployment", Namespace: "nullus", Name: "running", Replicas: 2},
	})

	require.Len(t, plan.Targets, 1, "0 인 워크로드는 정지 대상이 아니다")
	assert.Equal(t, "running", plan.Targets[0].Name)
}

func TestQuiescePlan_ResumeOrder_는_정지의_역순이다(t *testing.T) {
	plan := NewQuiescePlan([]Workload{
		{Kind: "StatefulSet", Namespace: "nullus", Name: "postgres", Replicas: 1},
		{Kind: "Deployment", Namespace: "nullus", Name: "gitlab", Replicas: 1},
	})

	resume := plan.ResumeOrder()
	require.Len(t, resume, 2)
	assert.Equal(t, "gitlab", resume[0].Name, "나중에 멈춘 것을 먼저 되살린다")
	assert.Equal(t, "postgres", resume[1].Name)
}

func TestQuiescePlan_IsEmpty(t *testing.T) {
	assert.True(t, NewQuiescePlan(nil).IsEmpty())
	assert.True(t, NewQuiescePlan([]Workload{{Name: "a", Replicas: 0}}).IsEmpty())
	assert.False(t, NewQuiescePlan([]Workload{{Name: "a", Replicas: 1}}).IsEmpty())
}
