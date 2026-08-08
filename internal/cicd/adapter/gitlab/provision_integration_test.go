//go:build integration

package gitlab_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/gitlab"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
)

// 실제 GitLab 인스턴스를 상대로 프로비저닝 경로를 검증한다.
//
// 실행 방법 (스택이 설치된 클러스터에서):
//
//	kubectl -n <ns> port-forward svc/gitlab-webservice-default 18181:8181 &
//	export GITLAB_URL=http://localhost:18181
//	export GITLAB_TOKEN=<toolbox rails runner 로 발급한 PAT>
//	go test -tags integration ./internal/cicd/adapter/gitlab/ -run Integration -v
//
// 환경변수가 없으면 graceful skip 한다 — 기본 `go test ./...` 를 오염시키지 않는다.
func requireGitLabEnv(t *testing.T) (string, string) {
	t.Helper()
	baseURL := strings.TrimSpace(os.Getenv("GITLAB_URL"))
	token := strings.TrimSpace(os.Getenv("GITLAB_TOKEN"))
	if baseURL == "" || token == "" {
		t.Skip("GITLAB_URL / GITLAB_TOKEN 이 없어 건너뜁니다")
	}
	return baseURL, token
}

func TestIntegration_ProvisionCommonProject(t *testing.T) {
	baseURL, token := requireGitLabEnv(t)

	client := gitlab.NewClient(baseURL, token).WithRegistryHost("registry.nullus.local")
	uc := usecase.NewProvisionCommonProject(client)

	input := usecase.ProvisionCommonProjectInput{
		GroupPath: "nullus-verify",
		GroupName: "Nullus Verify",
	}

	out, err := uc.Execute(context.Background(), input)
	require.NoError(t, err)

	assert.NotEmpty(t, out.Group.ID)
	assert.Equal(t, "nullus-verify", out.Group.FullPath)
	assert.Equal(t, "nullus-verify/common", out.Project.FullPath)
	assert.NotEmpty(t, out.Project.RegistryURL, "레지스트리 경로를 모르면 CI 가 이미지를 올릴 곳을 정할 수 없다")
	assert.NotEmpty(t, out.Project.DefaultBranch, "기본 브랜치가 없으면 이후 파일 커밋이 실패한다")

	t.Logf("group=%s project=%s registry=%s branch=%s",
		out.Group.FullPath, out.Project.FullPath, out.Project.RegistryURL, out.Project.DefaultBranch)

	// 스택 설치는 재시도될 수 있으므로 두 번째 실행도 같은 결과여야 한다.
	again, err := uc.Execute(context.Background(), input)
	require.NoError(t, err, "재실행이 실패하면 설치 재시도에서 스택이 깨진다")
	assert.Equal(t, out.Project.ID, again.Project.ID, "재실행이 새 프로젝트를 만들면 안 된다")
}
