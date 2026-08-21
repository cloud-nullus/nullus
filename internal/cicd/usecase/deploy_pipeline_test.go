package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type mockDeployPipelineRepo struct {
	pipelines map[string]*domain.Pipeline
	getErr    map[string]error
}

func newMockDeployPipelineRepo(seed ...*domain.Pipeline) *mockDeployPipelineRepo {
	pipelines := make(map[string]*domain.Pipeline, len(seed))
	for _, p := range seed {
		copied := *p
		pipelines[p.ID] = &copied
	}
	return &mockDeployPipelineRepo{pipelines: pipelines, getErr: map[string]error{}}
}

func (m *mockDeployPipelineRepo) Create(_ context.Context, _ *domain.Pipeline) error { return nil }
func (m *mockDeployPipelineRepo) GetByID(_ context.Context, id string) (*domain.Pipeline, error) {
	if err, ok := m.getErr[id]; ok {
		return nil, err
	}
	p, ok := m.pipelines[id]
	if !ok {
		return nil, errors.New("pipeline not found")
	}
	copied := *p
	return &copied, nil
}
func (m *mockDeployPipelineRepo) List(_ context.Context, _ string) ([]*domain.Pipeline, error) {
	return nil, nil
}
func (m *mockDeployPipelineRepo) ListByStackID(_ context.Context, _ string) ([]*domain.Pipeline, error) {
	return nil, nil
}
func (m *mockDeployPipelineRepo) Update(_ context.Context, _ *domain.Pipeline) error { return nil }
func (m *mockDeployPipelineRepo) Delete(_ context.Context, _ string) error           { return nil }

type mockDeployDeploymentRepo struct {
	created   []*domain.Deployment
	createErr error
}

func (m *mockDeployDeploymentRepo) Create(_ context.Context, d *domain.Deployment) error {
	if m.createErr != nil {
		return m.createErr
	}
	copied := *d
	m.created = append(m.created, &copied)
	return nil
}
func (m *mockDeployDeploymentRepo) GetByID(_ context.Context, id string) (*domain.Deployment, error) {
	for _, d := range m.created {
		if d.ID == id {
			copied := *d
			return &copied, nil
		}
	}
	return nil, errors.New("deployment not found")
}
func (m *mockDeployDeploymentRepo) ListByPipelineID(_ context.Context, _ string) ([]*domain.Deployment, error) {
	return nil, nil
}
func (m *mockDeployDeploymentRepo) Update(_ context.Context, d *domain.Deployment) error {
	for i, existing := range m.created {
		if existing.ID == d.ID {
			copied := *d
			m.created[i] = &copied
			return nil
		}
	}
	return nil
}

type mockKubeconfigProvider struct {
	kubeconfig []byte
	err        error
}

func (m *mockKubeconfigProvider) GetKubeconfig(_ context.Context, _ string) ([]byte, error) {
	return m.kubeconfig, m.err
}

type mockManifestApplier struct {
	appliedManifests [][]string
	// 단계 오프셋을 기록한다. 이 값이 BuildStepPlan 이 붙인 빌드 단계 수와
	// 어긋나면 매니페스트 적용 결과가 빌드 단계 이름 위에 찍힌다.
	appliedOffsets []int
	err            error
}

func (m *mockManifestApplier) Apply(_ context.Context, _ []byte, manifests []string) error {
	m.appliedManifests = append(m.appliedManifests, manifests)
	return m.err
}

func (m *mockManifestApplier) ApplyWithTracking(_ context.Context, _ []byte, manifests []string, _ string, stepOffset ...int) error {
	m.appliedManifests = append(m.appliedManifests, manifests)
	offset := 0
	if len(stepOffset) > 0 {
		offset = stepOffset[0]
	}
	m.appliedOffsets = append(m.appliedOffsets, offset)
	return m.err
}

// 회귀: 계획이 앞에 붙인 빌드 단계 수와 적용 오프셋이 반드시 같아야 한다.
//
// 어긋났던 적이 있다. BuildStepPlan 은 DockerfilePath 만 보고 3개를 붙이는데
// applyToCluster 는 imagePreparer·clusterTargetProvider 까지 확인한 뒤에야
// 오프셋을 3으로 올렸다. 그래서 이미지 준비기가 없는 경로에서 "Git Clone" 단계에
// "namespace/... configured" 가 기록되고, 진짜 매니페스트 단계 3개는 영원히
// pending 으로 남았다.
func TestDeployPipeline_StepOffsetMatchesPlan(t *testing.T) {
	cases := []struct {
		name     string
		pipeline *domain.Pipeline
	}{
		{"Dockerfile 없음", &domain.Pipeline{ID: "pip-1", Name: "orders", ClusterID: "c1", AppType: domain.AppTypeBackend}},
		// 이미지 준비기를 주지 않은 채 DockerfilePath 를 세팅한다 — 버그가 났던 그 조합이다.
		{"Dockerfile 있음 / 이미지 준비기 없음", &domain.Pipeline{ID: "pip-1", Name: "orders", ClusterID: "c1", AppType: domain.AppTypeBackend, DockerfilePath: "Dockerfile"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			applier := &mockManifestApplier{}
			uc := NewDeployPipeline(
				newMockDeployPipelineRepo(tc.pipeline),
				&mockDeployDeploymentRepo{},
				&mockKubeconfigProvider{},
				applier,
			)

			_, err := uc.Execute(context.Background(), DeployPipelineInput{
				PipelineID: tc.pipeline.ID,
				Version:    "v1.0.0",
				DeployedBy: "devops@acme.io",
			})
			require.NoError(t, err)

			require.Len(t, applier.appliedOffsets, 1)
			plan := BuildStepPlan(tc.pipeline)
			offset := applier.appliedOffsets[0]

			// 오프셋 위치부터가 매니페스트 단계여야 한다.
			assert.Equal(t, buildStepCount(tc.pipeline), offset)
			require.Greater(t, len(plan), offset)
			assert.Equal(t, "Create Namespace", plan[offset])
		})
	}
}

func TestDeployPipeline_Success(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{ID: "pip-1", Name: "orders", Namespace: "apps", ClusterID: "c1", AppType: domain.AppTypeBackend, OrgID: "org-1"},
	)
	deploymentRepo := &mockDeployDeploymentRepo{}
	kubeconfigProvider := &mockKubeconfigProvider{kubeconfig: []byte("fake-kubeconfig")}
	applier := &mockManifestApplier{}

	uc := NewDeployPipeline(pipelineRepo, deploymentRepo, kubeconfigProvider, applier)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "pip-1",
		Version:    "v1.2.0",
		DeployedBy: "devops@acme.io",
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Deployment)
	assert.Equal(t, "pip-1", out.Deployment.PipelineID)
	assert.Equal(t, "v1.2.0", out.Deployment.Version)
	assert.Equal(t, domain.DeploymentStatusSuccess, out.Deployment.Status)
	assert.NotEmpty(t, out.Deployment.ID)
	require.Len(t, applier.appliedManifests, 1)
	require.Len(t, applier.appliedManifests[0], 4)
	assert.Contains(t, applier.appliedManifests[0][3], "kind: Ingress")
}

func TestDeployPipeline_AppliesSelectedManifestTypes(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{ID: "pip-1", Name: "orders", Namespace: "apps", ClusterID: "c1", AppType: domain.AppTypeBackend},
	)
	deploymentRepo := &mockDeployDeploymentRepo{}
	applier := &mockManifestApplier{}
	uc := NewDeployPipeline(pipelineRepo, deploymentRepo, &mockKubeconfigProvider{kubeconfig: []byte("fake")}, applier)

	_, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID:    "pip-1",
		Version:       "v1.2.0",
		ManifestTypes: []string{"deployment"},
	})

	require.NoError(t, err)
	require.Len(t, applier.appliedManifests, 1)
	require.Len(t, applier.appliedManifests[0], 2)
	assert.Contains(t, applier.appliedManifests[0][1], "kind: Deployment")
}

func TestDeployPipeline_AppliesSavedReviewManifests(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{
			ID:        "pip-1",
			Name:      "orders",
			Namespace: "apps",
			ClusterID: "c1",
			AppType:   domain.AppTypeBackend,
			EnvVars: map[string]string{
				envManifestDeployment: "kind: Deployment\nmetadata:\n  name: imported\n",
				envManifestService:    "kind: Service\nmetadata:\n  name: imported-svc\n",
				envManifestIngress:    "kind: Ingress\nmetadata:\n  name: imported-ingress\n",
				"APP_ENV":             "prod",
			},
		},
	)
	deploymentRepo := &mockDeployDeploymentRepo{}
	applier := &mockManifestApplier{}
	uc := NewDeployPipeline(pipelineRepo, deploymentRepo, &mockKubeconfigProvider{kubeconfig: []byte("fake")}, applier)

	_, err := uc.Execute(context.Background(), DeployPipelineInput{PipelineID: "pip-1", Version: "v1.2.0"})

	require.NoError(t, err)
	require.Len(t, applier.appliedManifests, 1)
	require.Len(t, applier.appliedManifests[0], 4)
	assert.Contains(t, applier.appliedManifests[0][1], "name: imported")
	assert.Contains(t, applier.appliedManifests[0][2], "name: imported-svc")
	assert.Contains(t, applier.appliedManifests[0][3], "name: imported-ingress")
	assert.NotContains(t, applier.appliedManifests[0][1], envManifestDeployment)
}

func TestBuildStepPlan_IncludesIngress(t *testing.T) {
	assert.Equal(t,
		[]string{"Create Namespace", "Create Deployment", "Create Service", "Create Ingress"},
		BuildStepPlan(&domain.Pipeline{}),
	)
	assert.Equal(t,
		[]string{"Git Clone", "Docker Build", "Image Load", "Create Namespace", "Create Deployment", "Create Service", "Create Ingress"},
		BuildStepPlan(&domain.Pipeline{DockerfilePath: "Dockerfile"}),
	)
	assert.Equal(t,
		[]string{"Create Namespace", "Create Deployment"},
		BuildStepPlan(&domain.Pipeline{}, []string{"deployment"}),
	)
}

func TestDeployPipeline_PipelineNotFound(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo()
	deploymentRepo := &mockDeployDeploymentRepo{}
	kubeconfigProvider := &mockKubeconfigProvider{}
	applier := &mockManifestApplier{}

	uc := NewDeployPipeline(pipelineRepo, deploymentRepo, kubeconfigProvider, applier)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "missing",
		Version:    "v1.0.0",
		DeployedBy: "devops@acme.io",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "pipeline not found")
}

func TestDeployPipeline_MissingPipelineID(t *testing.T) {
	uc := NewDeployPipeline(
		newMockDeployPipelineRepo(), &mockDeployDeploymentRepo{},
		&mockKubeconfigProvider{}, &mockManifestApplier{},
	)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "",
		Version:    "v1.0.0",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "pipeline_id is required")
}

func TestDeployPipeline_MissingVersion(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{ID: "pip-1", Name: "orders"},
	)
	uc := NewDeployPipeline(pipelineRepo, &mockDeployDeploymentRepo{}, &mockKubeconfigProvider{}, &mockManifestApplier{})

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "pip-1",
		Version:    "",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "version is required")
}

func TestDeployPipeline_DeploymentRepoError(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{ID: "pip-1", Name: "orders", Namespace: "apps", ClusterID: "c1"},
	)
	deploymentRepo := &mockDeployDeploymentRepo{createErr: errors.New("db error")}
	uc := NewDeployPipeline(pipelineRepo, deploymentRepo, &mockKubeconfigProvider{}, &mockManifestApplier{})

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "pip-1",
		Version:    "v1.0.0",
		DeployedBy: "devops@acme.io",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "create deployment")
}

func TestDeployPipeline_ApplierError(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{ID: "pip-1", Name: "orders", Namespace: "apps", ClusterID: "c1", AppType: domain.AppTypeBackend},
	)
	deploymentRepo := &mockDeployDeploymentRepo{}
	kubeconfigProvider := &mockKubeconfigProvider{kubeconfig: []byte("fake")}
	applier := &mockManifestApplier{err: errors.New("apply failed")}

	uc := NewDeployPipeline(pipelineRepo, deploymentRepo, kubeconfigProvider, applier)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "pip-1",
		Version:    "v1.0.0",
		DeployedBy: "devops@acme.io",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "deployment failed")
	require.Len(t, deploymentRepo.created, 1)
	assert.Equal(t, domain.DeploymentStatusFailed, deploymentRepo.created[0].Status)
}

type mockImagePreparer struct {
	calls    []port.PrepareImageOpts
	imageRef string
	err      error
}

func (m *mockImagePreparer) PrepareImage(_ context.Context, opts port.PrepareImageOpts) (string, error) {
	m.calls = append(m.calls, opts)
	if m.imageRef == "" {
		return opts.ImageName, m.err
	}
	return m.imageRef, m.err
}

type mockClusterTargetProvider struct {
	err error
}

func (m *mockClusterTargetProvider) GetTarget(_ context.Context, _ string) (*port.ClusterTarget, error) {
	return &port.ClusterTarget{Kubeconfig: []byte("fake"), ClusterName: "kind-nullus"}, m.err
}

type mockBuildDelegate struct {
	calls  []port.DelegateBuildOpts
	runURL string
	err    error
}

func (m *mockBuildDelegate) DelegateBuild(_ context.Context, opts port.DelegateBuildOpts) (string, error) {
	m.calls = append(m.calls, opts)
	return m.runURL, m.err
}

func delegatingPipeline() *domain.Pipeline {
	return &domain.Pipeline{
		ID:             "pip-del",
		Name:           "orders-api",
		ClusterID:      "c1",
		StackID:        "stk-1",
		Namespace:      "apps",
		AppType:        domain.AppTypeBackend,
		DockerfilePath: "Dockerfile",
	}
}

// 스택에 묶인 파이프라인의 빌드는 스택의 CI 러너가 맡는다. 플랫폼이 직접
// 빌드하려 하면 API 파드 안에서 git·docker 를 찾다 죽는다 — 그 파드에는 도커
// 데몬이 없다. 통합모드 설계 3.2 가 정한 경계이기도 하다.
func TestDeployPipeline_DelegatesBuildToRunner(t *testing.T) {
	pipeline := delegatingPipeline()
	applier := &mockManifestApplier{}
	preparer := &mockImagePreparer{}
	delegate := &mockBuildDelegate{runURL: "https://jenkins.nullus.io/job/orders-api/job/main/"}

	uc := NewDeployPipeline(
		newMockDeployPipelineRepo(pipeline),
		&mockDeployDeploymentRepo{},
		&mockKubeconfigProvider{kubeconfig: []byte("fake")},
		applier,
		WithImagePreparer(preparer),
		WithClusterTargetProvider(&mockClusterTargetProvider{}),
		WithBuildDelegate(delegate),
	)

	_, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: pipeline.ID,
		Version:    "v1.0.0",
		DeployedBy: "dev@acme.io",
	})
	require.NoError(t, err)

	require.Len(t, delegate.calls, 1)
	assert.Equal(t, "stk-1", delegate.calls[0].StackID)
	assert.Equal(t, "orders-api", delegate.calls[0].JobName)
	assert.Equal(t, "main", delegate.calls[0].Branch)

	// 플랫폼은 빌드하지 않는다.
	assert.Empty(t, preparer.calls, "위임한 파이프라인을 플랫폼이 빌드하면 안 된다")
	// 클러스터 반영은 CD 도구(Argo CD)가 한다. 플랫폼이 같은 리소스를 따로
	// 적용하면 러너가 되커밋한 매니페스트와 서로를 덮어쓴다.
	assert.Empty(t, applier.appliedManifests, "위임한 파이프라인의 매니페스트를 플랫폼이 적용하면 안 된다")
}

// 위임 경로에는 러너가 반드시 배선돼 있어야 한다. 없다고 조용히 직접 빌드로
// 되돌아가면 실패할 수 없는 경로로 되돌아가는 것이고, 사용자는 왜 실패했는지
// 알 수 없다.
func TestDeployPipeline_DelegationRequiresRunner(t *testing.T) {
	pipeline := delegatingPipeline()

	uc := NewDeployPipeline(
		newMockDeployPipelineRepo(pipeline),
		&mockDeployDeploymentRepo{},
		&mockKubeconfigProvider{kubeconfig: []byte("fake")},
		&mockManifestApplier{},
	)

	_, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: pipeline.ID,
		Version:    "v1.0.0",
		DeployedBy: "dev@acme.io",
	})
	require.Error(t, err)
}

// 긴급모드는 예전 그대로다 — 스택이 있어도 사용자가 직접 배포를 골랐으면
// 플랫폼이 빌드한다.
func TestDeployPipeline_EmergencyDirectStillBuildsInPlatform(t *testing.T) {
	pipeline := delegatingPipeline()
	pipeline.ExecutionMode = domain.ExecutionModeEmergencyDirect
	preparer := &mockImagePreparer{}
	delegate := &mockBuildDelegate{}

	uc := NewDeployPipeline(
		newMockDeployPipelineRepo(pipeline),
		&mockDeployDeploymentRepo{},
		&mockKubeconfigProvider{kubeconfig: []byte("fake")},
		&mockManifestApplier{},
		WithImagePreparer(preparer),
		WithClusterTargetProvider(&mockClusterTargetProvider{}),
		WithBuildDelegate(delegate),
	)

	_, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: pipeline.ID,
		Version:    "v1.0.0",
		DeployedBy: "dev@acme.io",
	})
	require.NoError(t, err)

	assert.Len(t, preparer.calls, 1)
	assert.Empty(t, delegate.calls)
}

// 화면의 단계 계획과 실제 실행이 어긋나면 결과가 엉뚱한 단계 이름 위에 찍힌다.
// 위임 경로에서 플랫폼이 하는 일은 실행을 넘기는 것 하나뿐이다.
func TestBuildStepPlan_DelegatedPipelineHasSingleStep(t *testing.T) {
	assert.Equal(t, []string{"Trigger CI"}, BuildStepPlan(delegatingPipeline()))
}

type blockingBuildDelegate struct {
	started chan struct{}
	once    sync.Once
}

func (b *blockingBuildDelegate) DelegateBuild(ctx context.Context, _ port.DelegateBuildOpts) (string, error) {
	b.once.Do(func() { close(b.started) })
	// 러너 위임은 스택 번들을 조립하며 OpenBao 호출과 Gitea 파드 exec 까지 한다.
	// 스택이 온전치 않으면 그 호출이 돌아오지 않는다.
	<-ctx.Done()
	return "", ctx.Err()
}

// 배포는 데드라인 없이 돌면 안 된다.
//
// ApplyAsync 는 context.Background() 를 그대로 썼다. 위임 경로가 매달리면 배포
// 기록이 영원히 running 으로 남아, 화면은 "실행 중" 만 보여 주고 사용자는 무엇이
// 잘못됐는지 영영 알 수 없다 — 2026-08-21 운영에서 그랬다.
func TestDeployPipeline_FailsDeploymentWhenWorkExceedsTimeout(t *testing.T) {
	pipeline := delegatingPipeline()
	deploymentRepo := &mockDeployDeploymentRepo{}
	delegate := &blockingBuildDelegate{started: make(chan struct{})}

	uc := NewDeployPipeline(
		newMockDeployPipelineRepo(pipeline),
		deploymentRepo,
		&mockKubeconfigProvider{kubeconfig: []byte("fake")},
		&mockManifestApplier{},
		WithBuildDelegate(delegate),
		WithDeployTimeout(50*time.Millisecond),
	)

	out, err := uc.Start(context.Background(), DeployPipelineInput{
		PipelineID: pipeline.ID,
		Version:    "v1.0.0",
		DeployedBy: "dev@acme.io",
	})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		uc.ApplyAsync(out.Deployment.ID)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("데드라인이 없어 배포가 끝나지 않았습니다")
	}

	stored, err := deploymentRepo.GetByID(context.Background(), out.Deployment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.DeploymentStatusFailed, stored.Status,
		"시간을 넘긴 배포는 실패로 남아야 한다 — running 으로 두면 아무도 원인을 모른다")
}
