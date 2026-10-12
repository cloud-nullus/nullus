package scaffold

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 스캐너 이미지는 플랫폼이 올린다(예: 멀티아치 sonar-scanner). 정의 파일에 리터럴로 박으면
// 파이프라인을 다시 쓰기 전까지 옛 이미지를 쓴다. 정책과 함께 밀리는 변수를 먼저 보고, 변수가
// 없을 때만(한 번도 밀리지 않은 리포) 파일의 기본값을 쓴다.

const testScanEndpoint = "http://trivy.sast-demo.svc.cluster.local:4954"

func TestRenderPipeline_GitHub_이미지는_리포_변수를_먼저_본다(t *testing.T) {
	_, content := renderPipelineFor(sastInput(port.CIPlatformGitHubActions, testSASTEndpoint, testScanEndpoint))

	assert.Contains(t, content,
		fmt.Sprintf("image: ${{ vars.%s || '%s' }}", port.SASTImageVariable, shareddomain.SonarScannerImage),
		"SAST 컨테이너 이미지는 vars 우선, 기본값 폴백")
	assert.Contains(t, content,
		fmt.Sprintf("image: ${{ vars.%s || '%s' }}", port.ScanImageVariable, shareddomain.TrivyCLIImage),
		"이미지 스캔 컨테이너 이미지도 같다")
	assert.NotContains(t, content, fmt.Sprintf("image: %q", shareddomain.SonarScannerImage),
		"리터럴로 박으면 플랫폼이 이미지를 올려도 따라오지 않는다")
}

// GitLab 은 프로젝트 변수가 .gitlab-ci.yml 전역 variables 보다 앞선다. 파일의 값은 한 번도
// 밀리지 않은 리포를 위한 기본값이고, 플랫폼이 미는 값과 같은 출처여야 한다.
func TestRenderPipeline_GitLab_이미지_기본값은_플랫폼_상수와_같다(t *testing.T) {
	_, content := renderPipelineFor(sastInput(port.CIPlatformGitLabCI, testSASTEndpoint, testScanEndpoint))

	assert.Contains(t, content, fmt.Sprintf("  %s: %q\n", port.SASTImageVariable, shareddomain.SonarScannerImage))
	assert.Contains(t, content, fmt.Sprintf("  %s: %q\n", port.ScanImageVariable, shareddomain.TrivyCLIImage))
	assert.Contains(t, content, "$"+port.SASTImageVariable)
	assert.Contains(t, content, "$"+port.ScanImageVariable)
}
