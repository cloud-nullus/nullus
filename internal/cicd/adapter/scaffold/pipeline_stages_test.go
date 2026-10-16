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
	in := Input{AppName: "api", ImageTarget: jenkinsTarget()}
	content := renderJenkinsfile(in)

	for _, stage := range PipelineStageNamesFor(in) {
		assert.Containsf(t, content, "stage('"+stage+"')",
			"선언한 단계 %q 를 Jenkinsfile 이 만들지 않는다", stage)
	}
	assert.Equal(t, len(PipelineStageNamesFor(in)), strings.Count(content, "stage('"),
		"Jenkinsfile 이 선언보다 많은 단계를 만들면 화면이 일부를 놓친다")
}

// GitLab 판도 같은 단계를 만든다 — 표기만 소문자다.
func TestPipelineStageNames_MatchRenderedGitLabCI(t *testing.T) {
	in := Input{Platform: port.SCMPlatformGitLab, AppName: "api", ImageTarget: jenkinsTarget()}
	_, content := renderPipelineFor(in)

	for _, stage := range PipelineStageNamesFor(in) {
		assert.Containsf(t, content, "  - "+gitlabStageIDFor(t, stage),
			"선언한 단계 %q 를 .gitlab-ci.yml 이 만들지 않는다", stage)
	}
}

// gitlabStageIDFor 는 표시용 단계 이름을 .gitlab-ci.yml 의 stage 식별자로 옮긴다 — 둘의 대응은
// 렌더러의 상수가 정한다(port.StageKey 가 실행 기록에서 같은 두 표기를 같은 단계로 본다).
func gitlabStageIDFor(t *testing.T, name string) string {
	t.Helper()
	switch name {
	case secretScanStageName:
		return secretScanStageID
	case imageScanStageName:
		return imageScanStageID
	case sastStageName:
		return sastStageID
	default:
		return strings.ToLower(name)
	}
}
