package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type fakePipelineConfig struct {
	vars      map[string]port.ProjectVariable
	tokenErr  error
	varErr    error
	issued    []port.AccessTokenSpec
	nextToken string
}

func newFakePipelineConfig() *fakePipelineConfig {
	return &fakePipelineConfig{vars: map[string]port.ProjectVariable{}, nextToken: "glpat-deploy"}
}

func (f *fakePipelineConfig) SetProjectVariable(_ context.Context, _ string, v port.ProjectVariable) error {
	if f.varErr != nil {
		return f.varErr
	}
	f.vars[v.Key] = v
	return nil
}

func (f *fakePipelineConfig) CreateProjectAccessToken(_ context.Context, _ string, spec port.AccessTokenSpec) (string, error) {
	if f.tokenErr != nil {
		return "", f.tokenErr
	}
	f.issued = append(f.issued, spec)
	return f.nextToken, nil
}

type fakeResolver struct {
	target *port.ImageTarget
	err    error
	seen   []port.ImageTargetSpec
}

func (f *fakeResolver) Resolve(_ context.Context, spec port.ImageTargetSpec) (*port.ImageTarget, error) {
	f.seen = append(f.seen, spec)
	if f.err != nil {
		return nil, f.err
	}
	return f.target, nil
}

func gitlabResolver() *fakeResolver {
	return &fakeResolver{target: &port.ImageTarget{
		Kind: port.RegistryKindSCMProject, Host: "registry.test",
		Repository:  "registry.test/acme/myapp",
		UsernameVar: "CI_REGISTRY_USER", PasswordVar: "CI_REGISTRY_PASSWORD",
	}}
}

func harborResolver() *fakeResolver {
	return &fakeResolver{target: &port.ImageTarget{
		Kind: port.RegistryKindHarbor, Host: "harbor.test",
		Repository:  "harbor.test/acme/myapp",
		UsernameVar: "HARBOR_USERNAME", PasswordVar: "HARBOR_PASSWORD",
		RequiredVariables: []string{"HARBOR_USERNAME", "HARBOR_PASSWORD"},
	}}
}

func appInput() ProvisionAppProjectInput {
	return ProvisionAppProjectInput{
		AppName:   "myapp",
		GroupPath: "acme",
		GroupID:   "7",
		Namespace: "acme-prod",
		Port:      8080,
		Replicas:  2,
	}
}

func TestProvisionAppProject_CreatesProjectUnderGroup(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)

	require.Len(t, scm.projects, 1)
	assert.Equal(t, "myapp", scm.projects[0].Path)
	assert.Equal(t, "7", scm.projects[0].GroupID)
	assert.True(t, scm.projects[0].InitReadme)
	assert.Equal(t, "acme/myapp", out.Project.FullPath)
}

// resolver 에 SCM 이 알려준 레지스트리 경로가 전달되어야 GitLab 구성에서
// 프로젝트 자신의 레지스트리를 쓸 수 있다.
func TestProvisionAppProject_PassesSCMRegistryToResolver(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	_, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)

	require.Len(t, res.seen, 1)
	assert.Equal(t, "myapp", res.seen[0].AppName)
	assert.Equal(t, "acme/myapp", res.seen[0].SCMProjectPath)
	assert.Equal(t, "registry.test/acme/myapp", res.seen[0].SCMRegistryURL)
	assert.Equal(t, "acme", res.seen[0].OrgPath)
}

func TestProvisionAppProject_CommitsScaffoldWithResolvedRepository(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)

	commits := scm.commits[out.Project.ID]
	require.Len(t, commits, 1)

	files := map[string]string{}
	for _, f := range commits[0].Files {
		files[f.Path] = f.Content
	}
	require.Contains(t, files, ".gitlab-ci.yml")
	require.Contains(t, files, "deploy/deployment.yaml")

	// Harbor 구성이므로 SCM 레지스트리가 아니라 Harbor 경로가 들어가야 한다.
	assert.Contains(t, files[".gitlab-ci.yml"], "harbor.test/acme/myapp")
	assert.NotContains(t, files[".gitlab-ci.yml"], "registry.test")
}

// deploy 단계가 매니페스트를 되쓰려면 쓰기 권한 토큰이 필요하다.
func TestProvisionAppProject_IssuesAndRegistersDeployToken(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	_, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)

	// CI 쓰기 토큰과 Argo CD 읽기 토큰을 각각 발급한다 — 권한을 분리한다.
	require.Len(t, pipe.issued, 2)
	scopes := map[string][]string{}
	for _, s := range pipe.issued {
		scopes[s.Name] = s.Scopes
	}
	assert.Contains(t, scopes[deployTokenName], "write_repository")
	assert.Contains(t, scopes[argoReadTokenName], "read_repository")

	v, ok := pipe.vars[DeployTokenVariable]
	require.True(t, ok, "발급한 토큰을 변수로 등록해야 CI 가 쓸 수 있다")
	assert.Equal(t, "glpat-deploy", v.Value)
	assert.True(t, v.Masked, "토큰이 job 로그에 남으면 안 된다")
}

// 외부 레지스트리 자격증명은 사용자가 준 것만 등록할 수 있다.
//
// 값은 GitLab 마스킹 요건(8자 이상, Base64 문자+@:.~)을 만족하는 것으로 쓴다.
// 예전에는 "s3cret"(6자) / "robot$ci"($ 포함) 처럼 실제 GitLab 이 masked 로는
// 거부하는 값으로 masked=true 를 단언했는데, 그건 API 가 받아주지 않는 조합이라
// 페이크에서만 통과하는 계약이었다.
func TestProvisionAppProject_RegistersSuppliedRegistryCredentials(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	in := appInput()
	in.RegistryCredentials = map[string]string{
		"HARBOR_USERNAME": "robot-ci-account",
		"HARBOR_PASSWORD": "s3cretPassw0rd",
	}

	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	assert.Equal(t, "robot-ci-account", pipe.vars["HARBOR_USERNAME"].Value)
	assert.True(t, pipe.vars["HARBOR_PASSWORD"].Masked)
	assert.Empty(t, out.MissingVariables)
}

// 자격증명을 못 받았으면 조용히 넘어가지 않고 무엇이 빠졌는지 알려야 한다.
// 그러지 않으면 첫 파이프라인이 로그인 실패로 죽고 원인을 찾기 어렵다.
func TestProvisionAppProject_ReportsMissingRequiredVariables(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"HARBOR_USERNAME", "HARBOR_PASSWORD"}, out.MissingVariables)
}

func TestProvisionAppProject_GitLabRegistryNeedsNoExtraVariables(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)
	assert.Empty(t, out.MissingVariables, "내장 변수만 쓰므로 등록할 것이 없다")
}

func TestProvisionAppProject_RequiresAppNameAndGroup(t *testing.T) {
	uc := NewProvisionAppProject(newFakeSCM(), newFakePipelineConfig(), gitlabResolver())

	in := appInput()
	in.AppName = ""
	_, err := uc.Execute(context.Background(), in)
	require.Error(t, err)

	in = appInput()
	in.GroupPath = ""
	_, err = uc.Execute(context.Background(), in)
	require.Error(t, err)
}

func TestProvisionAppProject_PropagatesResolverFailure(t *testing.T) {
	res := gitlabResolver()
	res.err = errors.New("registry not configured")
	uc := NewProvisionAppProject(newFakeSCM(), newFakePipelineConfig(), res)

	_, err := uc.Execute(context.Background(), appInput())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry not configured")
}

// 토큰 발급이 실패해도 프로젝트와 스캐폴딩은 이미 만들어졌다. 전체를 실패로
// 돌리면 재실행 시 무엇이 남았는지 알 수 없으므로, 경고로 알리고 계속한다.
func TestProvisionAppProject_ContinuesWhenDeployTokenIssueFails(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	pipe.tokenErr = errors.New("token api disabled")
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)
	assert.Contains(t, out.MissingVariables, DeployTokenVariable)
	assert.NotEmpty(t, out.Warnings)
}

func TestProvisionAppProject_IsRepeatable(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	first, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)
	second, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)

	assert.Equal(t, first.Project.FullPath, second.Project.FullPath)
	assert.Equal(t, first.ImageTarget.Repository, second.ImageTarget.Repository)
}

