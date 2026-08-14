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
