package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type stubCIJobs struct {
	spec port.CIJobSpec
	err  error
}

func (s *stubCIJobs) EnsureJob(_ context.Context, spec port.CIJobSpec) (*port.CIJob, error) {
	s.spec = spec
	if s.err != nil {
		return nil, s.err
	}
	return &port.CIJob{Name: spec.Name, URL: "http://jenkins/job/" + spec.Name + "/"}, nil
}

func (s *stubCIJobs) DeleteJob(context.Context, string) error { return nil }

type stubWebhooks struct {
	target string
	err    error
}

func (s *stubWebhooks) EnsureWebhook(_ context.Context, _, targetURL, _ string) error {
	s.target = targetURL
	return s.err
}

func giteaProject() *port.SCMProject {
	return &port.SCMProject{
		ID:            "nullus/api",
		Name:          "api",
		FullPath:      "nullus/api",
		HTTPCloneURL:  "http://gitea-http.nullus.svc:3000/nullus/api.git",
		DefaultBranch: "main",
	}
}

// Jenkins 는 Jenkinsfile 만 커밋해서는 아무 일도 일어나지 않는다 —
// job 이 먼저 존재해야 한다.
func TestEnsureCIJob_CreatesMultibranchJobAndWebhook(t *testing.T) {
	jobs := &stubCIJobs{}
	hooks := &stubWebhooks{}
	uc := (&ProvisionAppProject{}).WithCIJobs(
		jobs, hooks, "http://jenkins.nullus.svc:8080", "http://gitea-http.nullus.svc:3000")

	out := &ProvisionAppProjectOutput{}
	uc.ensureCIJob(context.Background(), giteaProject(), out)

	assert.Equal(t, "api", jobs.spec.Name)
	assert.Equal(t, "nullus", jobs.spec.RepoOwner)
	assert.Equal(t, "api", jobs.spec.RepoName)
	assert.Equal(t, "http://gitea-http.nullus.svc:3000", jobs.spec.ServerURL,
		"Gitea SCM 소스는 리포 주소가 아니라 서버 루트로 브랜치를 훑는다")
	assert.Equal(t, "Jenkinsfile", jobs.spec.PipelinePath)
	assert.Equal(t, "http://jenkins.nullus.svc:8080/gitea-webhook/post", hooks.target)
	assert.Equal(t, "http://jenkins/job/api/", out.CIJobURL)
	assert.Empty(t, out.Warnings)
}

// GitLab·GitHub 은 파이프라인 정의를 푸시하면 자동 감지한다.
// 배선이 없으면 조용히 건너뛰어야 기존 경로가 그대로 동작한다.
func TestEnsureCIJob_SkippedWhenNotWired(t *testing.T) {
	uc := &ProvisionAppProject{}

	out := &ProvisionAppProjectOutput{}
	uc.ensureCIJob(context.Background(), giteaProject(), out)

	assert.Empty(t, out.Warnings)
	assert.Empty(t, out.CIJobURL)
}

// job 생성 실패로 전체를 되돌리지 않는다 — 리포와 스캐폴딩은 이미 만들어졌다.
// 다만 조용히 넘기면 사용자는 준비된 줄 알고 커밋했다가 아무 일도 일어나지
// 않는 것을 보게 되므로 경고로 남긴다.
func TestEnsureCIJob_FailureIsWarnedNotFatal(t *testing.T) {
	jobs := &stubCIJobs{err: errors.New("403 forbidden")}
	uc := (&ProvisionAppProject{}).WithCIJobs(jobs, nil, "http://jenkins", "http://gitea")

	out := &ProvisionAppProjectOutput{}
	uc.ensureCIJob(context.Background(), giteaProject(), out)

	require.Len(t, out.Warnings, 1)
	assert.Contains(t, out.Warnings[0], "CI job 생성 실패")
	assert.Empty(t, out.CIJobURL)
}

// webhook 이 없으면 job 은 만들어졌지만 새 커밋을 모른다 — 반드시 알려야 한다.
func TestEnsureCIJob_WebhookFailureIsWarned(t *testing.T) {
	jobs := &stubCIJobs{}
	hooks := &stubWebhooks{err: errors.New("connection refused")}
	uc := (&ProvisionAppProject{}).WithCIJobs(jobs, hooks, "http://jenkins", "http://gitea")

	out := &ProvisionAppProjectOutput{}
	uc.ensureCIJob(context.Background(), giteaProject(), out)

	require.Len(t, out.Warnings, 1)
	assert.Contains(t, out.Warnings[0], "빌드가 자동으로 시작되지 않습니다")
}

func TestSplitFullPath(t *testing.T) {
	owner, name, ok := splitFullPath("nullus/api")
	require.True(t, ok)
	assert.Equal(t, "nullus", owner)
	assert.Equal(t, "api", name)

	_, _, ok = splitFullPath("api")
	assert.False(t, ok, "소유자를 모르면 job 이 브랜치를 찾지 못한 채 조용히 서 있는다")
}

// Jenkins 에 등록되는 credential 이름은 스택 모듈의 JCasC 설정이 만드는 이름과
// 같아야 한다. 모듈 간 직접 import 가 금지돼 양쪽이 값을 각자 들고 있으므로,
// 한쪽만 바뀌면 job 은 만들어지되 브랜치를 하나도 찾지 못한다 — 실패가 설치
// 시점이 아니라 첫 스캔 시점에 나타나 원인을 찾기 어렵다.
//
// 스택 쪽 값은 internal/stack/adapter/helm 의 JenkinsGiteaCredentialID 다.
func TestGiteaCredentialID_MatchesStackJCasCContract(t *testing.T) {
	const stackSideValue = "nullus-gitea"

	assert.Equal(t, stackSideValue, giteaCredentialID,
		"internal/stack/adapter/helm 의 JenkinsGiteaCredentialID 와 같아야 한다")
}

type stubCredentialPlane struct {
	app  string
	vars []port.PipelineVariable
	err  error
}

func (s *stubCredentialPlane) Provision(
	_ context.Context, app string, vars []port.PipelineVariable,
) (string, error) {
	s.app = app
	s.vars = vars
	if s.err != nil {
		return "", s.err
	}
	return "apiVersion: external-secrets.io/v1\nkind: ExternalSecret\n", nil
}

func giteaTarget() *port.ImageTarget {
	return &port.ImageTarget{
		Host:              "registry.otel.internal",
		Repository:        "registry.otel.internal/nullus/api",
		UsernameVar:       "HARBOR_USERNAME",
		PasswordVar:       "HARBOR_PASSWORD",
		RequiredVariables: []string{"HARBOR_USERNAME", "HARBOR_PASSWORD"},
	}
}

// Gitea 에는 CI 변수 저장소가 없다. 자격증명은 OpenBao → ESO 평면이 나른다.
func TestConfigureGiteaPipeline_ProvisionsCredentialSecret(t *testing.T) {
	plane := &stubCredentialPlane{}
	uc := (&ProvisionAppProject{}).WithCredentialPlane(plane)

	out := &ProvisionAppProjectOutput{}
	uc.configureGiteaPipeline(context.Background(), giteaProject(), giteaTarget(), ProvisionAppProjectInput{
		Platform:        port.SCMPlatformGitea,
		RepoAccessToken: "gitea-token",
		RegistryCredentials: map[string]string{
			"HARBOR_USERNAME": "robot", "HARBOR_PASSWORD": "pw",
		},
	}, out)

	keys := map[string]string{}
	for _, v := range plane.vars {
		keys[v.Key] = v.Value
	}
	assert.Equal(t, "gitea_admin", keys["GIT_USERNAME"])
	assert.Equal(t, "gitea-token", keys["GIT_PASSWORD"],
		"deploy 단계가 매니페스트 태그를 되쓰려면 저장소 쓰기 자격증명이 필요하다")
	assert.Equal(t, "robot", keys["HARBOR_USERNAME"])
	assert.Len(t, out.CredentialManifests, 1)
	assert.Empty(t, out.MissingVariables)
}

// 레지스트리 자격증명이 없으면 조용히 넘기지 않고 사람이 채울 목록으로 알린다.
func TestConfigureGiteaPipeline_ReportsMissingRegistryCredentials(t *testing.T) {
	plane := &stubCredentialPlane{}
	uc := (&ProvisionAppProject{}).WithCredentialPlane(plane)

	out := &ProvisionAppProjectOutput{}
	uc.configureGiteaPipeline(context.Background(), giteaProject(), giteaTarget(), ProvisionAppProjectInput{
		Platform:        port.SCMPlatformGitea,
		RepoAccessToken: "gitea-token",
	}, out)

	assert.Contains(t, out.MissingVariables, "HARBOR_USERNAME")
	assert.Contains(t, out.MissingVariables, "HARBOR_PASSWORD")
}

// 평면이 배선되지 않았는데 조용히 성공하면 사용자는 준비된 줄 안다.
func TestConfigureGiteaPipeline_WarnsWhenPlaneMissing(t *testing.T) {
	uc := &ProvisionAppProject{}

	out := &ProvisionAppProjectOutput{}
	uc.configureGiteaPipeline(context.Background(), giteaProject(), giteaTarget(), ProvisionAppProjectInput{
		Platform: port.SCMPlatformGitea,
	}, out)

	require.NotEmpty(t, out.Warnings)
	assert.Empty(t, out.CredentialManifests)
}

// CI 서버가 SCM 을 스캔할 주소는 API 서버가 쓰는 주소와 다른 관심사다.
//
// Provisioner 의 base URL 은 로컬 실행에서 우회 주소(localhost 포트포워드)일 수
// 있는데, 그것을 job 에 넣으면 Jenkins 가
// "FATAL: Unknown server: http://localhost:3000" 으로 스캔을 거부한다 —
// job 은 만들어지는데 브랜치를 하나도 못 찾는, 원인이 먼 실패다.
func TestSCMServerURLFor_PrefersInClusterAddress(t *testing.T) {
	bundle := &port.SCMBundle{
		Provisioner:     stubBaseURLProvisioner{base: "http://localhost:3000"},
		SCMInClusterURL: "http://gitea-http.nullus-gj3.svc:3000",
	}

	assert.Equal(t, "http://gitea-http.nullus-gj3.svc:3000", scmServerURLFor(bundle),
		"Jenkins 는 클러스터 안에서 돌므로 서비스 DNS 를 받아야 한다")
}

// in-cluster 주소가 없으면 기존 동작을 유지한다(GitLab·GitHub 경로 무회귀).
func TestSCMServerURLFor_FallsBackToProvisionerBaseURL(t *testing.T) {
	bundle := &port.SCMBundle{Provisioner: stubBaseURLProvisioner{base: "https://gitlab.example.com"}}

	assert.Equal(t, "https://gitlab.example.com", scmServerURLFor(bundle))
}

type stubBaseURLProvisioner struct {
	base string
	port.SCMProvisioner
}

func (s stubBaseURLProvisioner) BaseURL() string { return s.base }

// Argo CD 는 private 저장소를 읽을 자격증명이 필요하다.
//
// 비워 두면 Application 은 만들어지되 동기화가 "authentication required" 로
// 실패한다 — 배포가 조용히 멈추는 형태다. Gitea 는 프로젝트 범위 토큰이 없으므로
// 스택의 자동화 토큰을 그대로 쓴다(write 스코프가 read 를 포함한다).
func TestConfigureGiteaPipeline_SetsArgoReadToken(t *testing.T) {
	uc := (&ProvisionAppProject{}).WithCredentialPlane(&stubCredentialPlane{})

	out := &ProvisionAppProjectOutput{}
	uc.configureGiteaPipeline(context.Background(), giteaProject(), giteaTarget(), ProvisionAppProjectInput{
		Platform:        port.SCMPlatformGitea,
		RepoAccessToken: "gitea-token",
	}, out)

	assert.Equal(t, "gitea-token", out.ArgoReadToken,
		"Argo CD 자격증명이 없으면 동기화가 authentication required 로 실패한다")
}
