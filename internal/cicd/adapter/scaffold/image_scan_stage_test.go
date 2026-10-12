package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const testScannerEndpoint = "http://trivy.scan-demo.svc.cluster.local:4954"

func scanInput(ci port.CIPlatform, endpoint string) Input {
	return Input{
		AppName:              "api",
		Platform:             port.SCMPlatformGitLab,
		CIPlatform:           ci,
		ImageTarget:          jenkinsTarget(),
		ImageScannerEndpoint: endpoint,
	}
}

// 스캐너가 있으면 세 CI 모두 스캔 단계를 만든다.
//
// 게이트는 파이프라인 단계다 — 지원 레지스트리 5종 중 자체 스캔 기능이 있는
// 것은 Harbor 하나뿐이라, 레지스트리에 게이트를 두면 나머지 4종이 게이트 없이
// 초록불이 된다.
func TestRenderPipeline_IncludesImageScanStage(t *testing.T) {
	tests := []struct {
		name string
		ci   port.CIPlatform
		want string
	}{
		{"GitLab CI", port.CIPlatformGitLabCI, "  - " + imageScanStageID},
		{"Jenkins", port.CIPlatformJenkins, "stage('" + imageScanStageName + "')"},
		{"GitHub Actions", port.CIPlatformGitHubActions, "  " + imageScanStageID + ":"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, content := renderPipelineFor(scanInput(tc.ci, testScannerEndpoint))

			assert.Contains(t, content, tc.want, "스캔 단계가 없으면 차단 게이트가 존재하지 않는다")
			assert.Contains(t, content, testScannerEndpoint,
				"서버 주소가 없으면 client 가 붙을 곳을 모른다")
		})
	}
}

// 스캐너가 없으면 단계를 만들지 않는다.
//
// 돌지도 않을 단계를 선언하면 화면이 그것을 성공으로 보여준다 —
// 마이그레이션 000070 이 정확히 그 실패를 되돌린 이력이다.
func TestRenderPipeline_OmitsScanStageWithoutScanner(t *testing.T) {
	for _, ci := range []port.CIPlatform{
		port.CIPlatformGitLabCI, port.CIPlatformJenkins, port.CIPlatformGitHubActions,
	} {
		t.Run(string(ci), func(t *testing.T) {
			_, content := renderPipelineFor(scanInput(ci, ""))
			assert.NotContains(t, strings.ToLower(content), imageScanStageID)
		})
	}
}

// DB 를 내려받지 않는다. client 가 DB 를 받으면 파이프라인마다 수백 MB 를
// 감당하게 되고, 에어갭에서는 반입 지점이 모든 러너로 늘어난다.
func TestRenderPipeline_ScanUsesServerMode(t *testing.T) {
	_, content := renderPipelineFor(scanInput(port.CIPlatformGitLabCI, testScannerEndpoint))

	assert.Contains(t, content, "--server", "server 모드가 아니면 client 가 DB 를 받는다")
	assert.NotContains(t, content, "--download-db-only")
}

// 차단 기준을 스크립트에 박지 않는다.
//
// 박으면 정책을 바꿀 때마다 모든 파이프라인을 다시 스캐폴딩해야 한다.
// 변수로 두면 파이프라인 변수만 갱신해 바꿀 수 있다.
func TestRenderPipeline_ScanPolicyComesFromVariables(t *testing.T) {
	_, content := renderPipelineFor(scanInput(port.CIPlatformGitLabCI, testScannerEndpoint))

	assert.Contains(t, content, scanSeverityVar)
	assert.NotContains(t, content, "--severity CRITICAL",
		"심각도를 스크립트에 박으면 정책 변경이 재스캐폴딩을 요구한다")
}

// 선언한 단계와 실제로 만든 단계가 같아야 한다 (000070 고정).
func TestPipelineStageNames_MatchRenderedWithScanner(t *testing.T) {
	in := scanInput(port.CIPlatformJenkins, testScannerEndpoint)
	content := renderJenkinsfile(in)

	names := PipelineStageNames(StageOptions{ImageScan: true})
	require.Contains(t, names, imageScanStageName)
	for _, stage := range names {
		assert.Containsf(t, content, "stage('"+stage+"')",
			"선언한 단계 %q 를 Jenkinsfile 이 만들지 않는다", stage)
	}
	assert.Equal(t, len(names), strings.Count(content, "stage('"),
		"Jenkinsfile 이 선언보다 많은 단계를 만들면 화면이 일부를 놓친다")
}

// 스캐너가 없으면 선언도 늘지 않는다.
func TestPipelineStageNames_WithoutScanner(t *testing.T) {
	assert.Equal(t, []string{"Build", "Deploy"}, PipelineStageNames(StageOptions{}))
}
