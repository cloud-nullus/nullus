package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func sastPipeline(id, name string) *domain.Pipeline {
	return &domain.Pipeline{ID: id, Name: name, StackID: "stk_1", Stages: []string{"Build", "SAST", "Deploy"}}
}

// 분석 단계도 정책 변수를 읽는다(Quality Gate 실패 시 동작, 분석기 장애 시 동작).
// 이미지 스캔이 없는 파이프라인에도 실어야 경고 정책이 실제로 먹는다.
func TestScanPolicyService_Update_PushesToSASTPipelines(t *testing.T) {
	pub := &recordingPublisher{}
	factory := &fakeBundleFactory{bundle: &port.SCMBundle{ScanPolicy: pub}}
	repo := &stackPipelines{byStack: []*domain.Pipeline{
		sastPipeline("p1", "api"),
		{ID: "p2", Name: "plain", StackID: "stk_1", Stages: []string{"Build", "Deploy"}},
	}}
	policy := domain.DefaultScanPolicy()
	policy.SASTOnGateFailure = domain.SASTGateWarn

	res, err := NewScanPolicyService(newMemPolicies(), repo, factory).Update(context.Background(), "stk_1", policy, "alice")
	require.NoError(t, err)

	assert.Equal(t, []string{"api"}, pub.apps)
	got := map[string]string{}
	for _, v := range pub.vars {
		got[v.Key] = v.Value
	}
	assert.Equal(t, "warn", got[port.SASTOnGateFailureVariable])
	require.Len(t, res.Pushes, 1)
}

// 이 필드를 모르는 옛 클라이언트가 이미지 스캔 정책만 바꾸면, 저장된 SAST 설정을 그대로
// 둔다. 빈 값으로 덮으면 운영자가 경고로 바꿔 둔 게이트가 몰래 차단으로 돌아간다.
func TestScanPolicyService_Update_KeepsStoredSASTActionWhenOmitted(t *testing.T) {
	policies := newMemPolicies()
	stored := domain.DefaultScanPolicy()
	stored.SASTOnGateFailure = domain.SASTGateWarn
	policies.rows["stk_1"] = stored

	update := highPolicy // SASTOnGateFailure 없음
	res, err := NewScanPolicyService(policies, &stackPipelines{}, &fakeBundleFactory{}).
		Update(context.Background(), "stk_1", update, "bob")
	require.NoError(t, err)

	assert.Equal(t, domain.SASTGateWarn, res.Policy.SASTOnGateFailure)
	after, _, _ := policies.Get(context.Background(), "stk_1")
	assert.Equal(t, domain.SASTGateWarn, after.SASTOnGateFailure)
	assert.Equal(t, domain.SeverityHigh, after.BlockSeverity, "나머지 값은 요청대로 바뀐다")
}

// 새로 만든 분석 파이프라인에도 스택의 현재 정책을 싣는다.
func TestScanPolicyService_PublishToPipeline_SASTPipeline(t *testing.T) {
	pub := &recordingPublisher{}
	factory := &fakeBundleFactory{bundle: &port.SCMBundle{ScanPolicy: pub}}
	push := NewScanPolicyService(newMemPolicies(), &stackPipelines{}, factory).
		PublishToPipeline(context.Background(), sastPipeline("p1", "api"))
	require.NotNil(t, push)
	assert.Equal(t, ScanPolicyPushApplied, push.Status)
	assert.Equal(t, []string{"api"}, pub.apps)
}
