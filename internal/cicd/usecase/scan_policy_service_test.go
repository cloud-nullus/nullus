package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type memPolicies struct {
	rows   map[string]domain.ScanPolicy
	actors map[string]string
}

func newMemPolicies() *memPolicies {
	return &memPolicies{rows: map[string]domain.ScanPolicy{}, actors: map[string]string{}}
}

func (m *memPolicies) Get(_ context.Context, stackID string) (domain.ScanPolicy, bool, error) {
	p, ok := m.rows[stackID]
	return p, ok, nil
}

func (m *memPolicies) Upsert(_ context.Context, stackID string, p domain.ScanPolicy, by string) error {
	m.rows[stackID] = p
	m.actors[stackID] = by
	return nil
}

// stackPipelines 는 스택에 묶인 파이프라인 목록을 돌려주는 저장소다.
type stackPipelines struct {
	fakePipelineRepo
	byStack []*domain.Pipeline
}

func (s *stackPipelines) ListByStackID(context.Context, string) ([]*domain.Pipeline, error) {
	return s.byStack, nil
}

type recordingPublisher struct {
	apps  []string
	vars  []port.ProjectVariable
	fail  map[string]error
	calls int
}

func (p *recordingPublisher) PublishScanPolicy(_ context.Context, apps []string, vars []port.ProjectVariable) map[string]error {
	p.calls++
	p.apps = append(p.apps, apps...)
	p.vars = vars
	out := map[string]error{}
	for _, a := range apps {
		out[a] = p.fail[a]
	}
	return out
}

func scanPipeline(id, name string) *domain.Pipeline {
	return &domain.Pipeline{ID: id, Name: name, StackID: "stk_1", Stages: []string{"Build", "ImageScan", "Deploy"}}
}

var highPolicy = domain.ScanPolicy{
	BlockSeverity: domain.SeverityHigh, IgnoreUnfixed: false, OnScannerUnreachable: domain.UnreachableAllow,
}

// 저장한 적 없는 스택은 기본 정책이고, 그것이 기본값이라는 사실을 함께 알린다.
func TestScanPolicyService_Get_DefaultWhenNotStored(t *testing.T) {
	svc := NewScanPolicyService(newMemPolicies(), &stackPipelines{}, &fakeBundleFactory{})

	view, err := svc.Get(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.True(t, view.IsDefault)
	assert.Equal(t, domain.DefaultScanPolicy(), view.Policy)
	assert.Equal(t, "stk_1", view.StackID)
}

func TestScanPolicyService_Get_ReturnsStoredPolicy(t *testing.T) {
	policies := newMemPolicies()
	policies.rows["stk_1"] = highPolicy
	svc := NewScanPolicyService(policies, &stackPipelines{}, &fakeBundleFactory{})

	view, err := svc.Get(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.False(t, view.IsDefault)
	assert.Equal(t, highPolicy, view.Policy)
}

func TestScanPolicyService_Get_RequiresStackID(t *testing.T) {
	_, err := NewScanPolicyService(newMemPolicies(), &stackPipelines{}, &fakeBundleFactory{}).Get(context.Background(), " ")
	assert.Error(t, err)
}

// 잘못된 정책은 저장도 푸시도 하지 않는다.
func TestScanPolicyService_Update_RejectsInvalidPolicy(t *testing.T) {
	policies := newMemPolicies()
	factory := &fakeBundleFactory{}
	svc := NewScanPolicyService(policies, &stackPipelines{byStack: []*domain.Pipeline{scanPipeline("p1", "shop")}}, factory)

	_, err := svc.Update(context.Background(), "stk_1", domain.ScanPolicy{BlockSeverity: "HIGHT"}, "alice")
	assert.True(t, errors.Is(err, domain.ErrInvalidScanPolicy))
	assert.Empty(t, policies.rows)
	assert.Empty(t, factory.asked)
}

// 스캔 단계가 있는 파이프라인에만 싣는다. 없는 파이프라인에 변수를 뿌리면 쓰이지도
// 않을 값이 프로젝트마다 쌓이고, 푸시 결과도 부풀려진다.
func TestScanPolicyService_Update_PushesToScanPipelinesOnly(t *testing.T) {
	policies := newMemPolicies()
	pub := &recordingPublisher{}
	factory := &fakeBundleFactory{bundle: &port.SCMBundle{ScanPolicy: pub}}
	repo := &stackPipelines{byStack: []*domain.Pipeline{
		scanPipeline("p1", "shop"),
		{ID: "p2", Name: "cart", StackID: "stk_1", Stages: []string{"Build", "Deploy"}},
		{ID: "p3", Name: "legacy", StackID: "stk_1"},
	}}

	res, err := NewScanPolicyService(policies, repo, factory).Update(context.Background(), "stk_1", highPolicy, "alice")
	require.NoError(t, err)

	assert.False(t, res.IsDefault)
	assert.Equal(t, highPolicy, res.Policy)
	stored, found, _ := policies.Get(context.Background(), "stk_1")
	require.True(t, found)
	assert.Equal(t, highPolicy, stored)
	assert.Equal(t, "alice", policies.actors["stk_1"])

	assert.Equal(t, []string{"shop"}, pub.apps)
	assert.Equal(t, port.ScanPolicyVariables(highPolicy), pub.vars)
	assert.Equal(t, []ScanPolicyPush{{PipelineID: "p1", PipelineName: "shop", Status: ScanPolicyPushApplied}}, res.Pushes)
}

// 일부만 실패해도 정책은 저장하고, 어느 파이프라인이 옛 정책으로 도는지 알린다.
func TestScanPolicyService_Update_ReportsPartialFailure(t *testing.T) {
	pub := &recordingPublisher{fail: map[string]error{"cart": errors.New("403 forbidden")}}
	repo := &stackPipelines{byStack: []*domain.Pipeline{scanPipeline("p1", "shop"), scanPipeline("p2", "cart")}}

	res, err := NewScanPolicyService(newMemPolicies(), repo, &fakeBundleFactory{bundle: &port.SCMBundle{ScanPolicy: pub}}).
		Update(context.Background(), "stk_1", highPolicy, "alice")
	require.NoError(t, err)

	byName := map[string]ScanPolicyPush{}
	for _, p := range res.Pushes {
		byName[p.PipelineName] = p
	}
	assert.Equal(t, ScanPolicyPushApplied, byName["shop"].Status)
	assert.Equal(t, ScanPolicyPushFailed, byName["cart"].Status)
	assert.Contains(t, byName["cart"].Error, "403")
}

// 스택에 닿지 못해도 정책은 저장한다. 다음 푸시(파이프라인 생성·정책 재저장)에서 실린다.
func TestScanPolicyService_Update_StoresEvenWhenBundleUnavailable(t *testing.T) {
	policies := newMemPolicies()
	factory := &fakeBundleFactory{err: port.ErrStackToolsUnavailable}
	repo := &stackPipelines{byStack: []*domain.Pipeline{scanPipeline("p1", "shop")}}

	res, err := NewScanPolicyService(policies, repo, factory).Update(context.Background(), "stk_1", highPolicy, "alice")
	require.NoError(t, err)

	_, found, _ := policies.Get(context.Background(), "stk_1")
	assert.True(t, found)
	require.Len(t, res.Pushes, 1)
	assert.Equal(t, ScanPolicyPushFailed, res.Pushes[0].Status)
	assert.NotEmpty(t, res.Pushes[0].Error)
}

func TestScanPolicyService_Update_FailsPushWhenNoPublisher(t *testing.T) {
	repo := &stackPipelines{byStack: []*domain.Pipeline{scanPipeline("p1", "shop")}}

	res, err := NewScanPolicyService(newMemPolicies(), repo, &fakeBundleFactory{bundle: &port.SCMBundle{}}).
		Update(context.Background(), "stk_1", highPolicy, "alice")
	require.NoError(t, err)
	require.Len(t, res.Pushes, 1)
	assert.Equal(t, ScanPolicyPushFailed, res.Pushes[0].Status, "실을 곳이 없는데 적용됐다고 말하면 안 된다")
}

// 스캔 파이프라인이 없으면 스택 도구에 묻지도 않는다 — 설치 중인 스택에서 정책을 먼저 정할 수 있어야 한다.
func TestScanPolicyService_Update_SkipsBundleWhenNoScanPipelines(t *testing.T) {
	factory := &fakeBundleFactory{err: errors.New("should not be called")}

	res, err := NewScanPolicyService(newMemPolicies(), &stackPipelines{}, factory).
		Update(context.Background(), "stk_1", highPolicy, "alice")
	require.NoError(t, err)
	assert.Empty(t, factory.asked)
	assert.Empty(t, res.Pushes)
}

// 새로 만든 스캔 파이프라인에는 스택에 저장된 정책을 싣는다. 싣지 않으면 스크립트
// 기본값으로 돌아, 운영자가 HIGH 를 막아 둔 스택에서 새 파이프라인만 HIGH 가 통과한다.
func TestScanPolicyService_PublishToPipeline_UsesStoredPolicy(t *testing.T) {
	policies := newMemPolicies()
	policies.rows["stk_1"] = highPolicy
	pub := &recordingPublisher{}

	push := NewScanPolicyService(policies, &stackPipelines{}, &fakeBundleFactory{bundle: &port.SCMBundle{ScanPolicy: pub}}).
		PublishToPipeline(context.Background(), scanPipeline("p1", "shop"))
	require.NotNil(t, push)
	assert.Equal(t, ScanPolicyPushApplied, push.Status)
	assert.Equal(t, []string{"shop"}, pub.apps)
	assert.Equal(t, port.ScanPolicyVariables(highPolicy), pub.vars)
}

func TestScanPolicyService_PublishToPipeline_SkipsPipelineWithoutScanStage(t *testing.T) {
	factory := &fakeBundleFactory{}
	push := NewScanPolicyService(newMemPolicies(), &stackPipelines{}, factory).
		PublishToPipeline(context.Background(), &domain.Pipeline{ID: "p2", Name: "cart", StackID: "stk_1", Stages: []string{"Build"}})
	assert.Nil(t, push)
	assert.Empty(t, factory.asked)
}
