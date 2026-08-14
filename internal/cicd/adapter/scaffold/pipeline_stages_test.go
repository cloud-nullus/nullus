package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// PipelineStageNames 는 템플릿이 선언하는 stages 의 출처다.
// 실제 렌더 결과와 어긋나면 화면이 돌지도 않은 단계를 보여준다.
func TestPipelineStageNames_MatchRenderedJenkinsfile(t *testing.T) {
	content := renderJenkinsfile("api", jenkinsTarget())

	for _, stage := range PipelineStageNames() {
		assert.Containsf(t, content, "stage('"+stage+"')",
			"선언한 단계 %q 를 Jenkinsfile 이 만들지 않는다", stage)
	}
	assert.Equal(t, len(PipelineStageNames()), strings.Count(content, "stage('"),
		"Jenkinsfile 이 선언보다 많은 단계를 만들면 화면이 일부를 놓친다")
}

// GitLab 판도 같은 단계를 만든다 — 표기만 소문자다.
func TestPipelineStageNames_MatchRenderedGitLabCI(t *testing.T) {
	_, content := renderPipelineFor(port.SCMPlatformGitLab, "", "api", jenkinsTarget())

	for _, stage := range PipelineStageNames() {
		assert.Containsf(t, content, "  - "+strings.ToLower(stage),
			"선언한 단계 %q 를 .gitlab-ci.yml 이 만들지 않는다", stage)
	}
}
