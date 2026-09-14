package provisioning

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// GitLab 번들의 산출물 조회기는 스택 접속 도메인으로 리포트 링크를 만든다.
func TestFor_GitLabBundleLinksReportsThroughAccessDomain(t *testing.T) {
	bundle, err := newFactory(t, gitlabStack(), &fakeTokenIssuer{token: "t"}).For(context.Background(), "stk_1")
	require.NoError(t, err)

	linker, ok := bundle.CIArtifacts.(port.CIArtifactLinker)
	require.True(t, ok, "GitLab 산출물 조회기는 리포트 링크를 만들 수 있어야 한다")
	assert.Equal(t, "https://gitlab.nullus.local/acme/app/-/jobs/9/artifacts/file/trivy-report.json",
		linker.ArtifactWebURL(port.CIArtifactRef{JobName: "app", Stage: port.CIStage{ID: "9"}, Path: "trivy-report.json"}))
}
