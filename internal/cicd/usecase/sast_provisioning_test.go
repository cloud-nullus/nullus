package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	testSASTEndpoint = "http://sonarqube.acme-prod.svc.cluster.local:9000"
	testSASTToken    = "sqa_0123456789abcdef0123456789abcdef01234567"
)

// 스택에 SonarQube 가 있으면 분석 단계를 만들고, 분석 토큰을 가려진 CI 변수로 건다.
// 토큰은 스택 설치가 발급해 OpenBao 에 둔 것이다 — 사용자에게 다시 받지 않는다.
func TestProvisionAppProject_SASTStageAndTokenOnGitLab(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	in := appInput()
	in.SASTServerEndpoint = testSASTEndpoint
	in.SASTToken = testSASTToken

	out, err := NewProvisionAppProject(scm, pipe, res).Execute(context.Background(), in)
	require.NoError(t, err)

	assert.Equal(t, []string{"Build", "SAST", "Deploy"}, out.Stages)
	v, ok := pipe.vars[port.SASTTokenVariable]
	require.True(t, ok, "토큰 변수가 없으면 분석이 인증에서 실패해 모든 배포가 막힌다")
	assert.Equal(t, testSASTToken, v.Value)
	assert.True(t, v.Masked, "토큰이 job 로그에 그대로 찍히면 안 된다")
	assert.NotContains(t, out.MissingVariables, port.SASTTokenVariable)
}

// 토큰을 못 구했으면 조용히 넘기지 않는다. 단계는 그대로 두고(스택이 분석을 요구한다)
// 사람이 채울 목록으로 알린다.
func TestProvisionAppProject_MissingSASTTokenIsReported(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	in := appInput()
	in.SASTServerEndpoint = testSASTEndpoint

	out, err := NewProvisionAppProject(scm, pipe, res).Execute(context.Background(), in)
	require.NoError(t, err)

	assert.Contains(t, out.Stages, "SAST")
	assert.Contains(t, out.MissingVariables, port.SASTTokenVariable)
	_, set := pipe.vars[port.SASTTokenVariable]
	assert.False(t, set, "빈 토큰을 등록하면 분석이 엉뚱한 인증 오류로 죽어 원인이 멀어진다")
}

// SonarQube 가 없는 스택에는 토큰도 단계도 없다.
func TestProvisionAppProject_NoSASTWithoutSonarQube(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	out, err := NewProvisionAppProject(scm, pipe, res).Execute(context.Background(), appInput())
	require.NoError(t, err)

	assert.NotContains(t, out.Stages, "SAST")
	assert.NotContains(t, out.MissingVariables, port.SASTTokenVariable)
	_, set := pipe.vars[port.SASTTokenVariable]
	assert.False(t, set)
}

// GitHub 은 리포 시크릿으로 건다(워크플로가 secrets.SONAR_TOKEN 으로 읽는다).
func TestProvisionAppProject_SASTTokenOnGitHub(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	in := appInput()
	in.Platform = port.SCMPlatformGitHub
	in.SharedAccessToken = "ghp_x"
	in.SASTServerEndpoint = testSASTEndpoint
	in.SASTToken = testSASTToken

	_, err := NewProvisionAppProject(scm, pipe, res).Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, testSASTToken, pipe.vars[port.SASTTokenVariable].Value)
}

// Gitea + Jenkins 에는 CI 변수 저장소가 없다. 토큰도 OpenBao → ESO 평면의 파이프라인
// Secret 으로 간다 — Jenkinsfile 의 분석 컨테이너가 그 Secret 을 envFrom 으로 읽는다.
func TestConfigureGiteaPipeline_IncludesSASTToken(t *testing.T) {
	plane := &stubCredentialPlane{}
	uc := (&ProvisionAppProject{}).WithCredentialPlane(plane)

	out := &ProvisionAppProjectOutput{}
	uc.configureGiteaPipeline(context.Background(), giteaProject(), giteaTarget(), ProvisionAppProjectInput{
		Platform:            port.SCMPlatformGitea,
		RepoAccessToken:     "gitea-token",
		RegistryCredentials: map[string]string{"HARBOR_USERNAME": "robot", "HARBOR_PASSWORD": "pw"},
		SASTServerEndpoint:  testSASTEndpoint,
		SASTToken:           testSASTToken,
	}, out)

	keys := map[string]string{}
	for _, v := range plane.vars {
		keys[v.Key] = v.Value
	}
	assert.Equal(t, testSASTToken, keys[port.SASTTokenVariable])
	assert.Empty(t, out.MissingVariables)
}
