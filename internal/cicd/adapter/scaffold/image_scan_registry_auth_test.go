package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// section 은 content 에서 start 부터 end 앞까지를 잘라낸다.
func section(t *testing.T, content, start, end string) string {
	t.Helper()
	i := strings.Index(content, start)
	require.GreaterOrEqual(t, i, 0, "%q 가 없다", start)
	rest := content[i:]
	if j := strings.Index(rest[len(start):], end); j >= 0 {
		return rest[:len(start)+j]
	}
	return rest
}

// 스캔 잡은 빌드가 올린 이미지를 레지스트리에서 다시 받는다. 자격증명이 없으면
// 사설 레지스트리에서 받지 못하고, 빌드가 --insecure-registry 로 push 한 레지스트리를
// TLS 검증으로 막혀 받지 못한다 — 실제 kind 스택에서 스캔 잡이
// "x509: certificate signed by unknown authority" 로 매번 실패했다.
func TestRenderGitLabCI_ScanJobGetsRegistryCredentials(t *testing.T) {
	in := scanInput(port.CIPlatformGitLabCI, testScannerEndpoint)
	in.ImageTarget = gitlabTarget()
	_, content := renderPipelineFor(in)

	job := section(t, content, "image-scan:\n", "\ndeploy:")
	assert.Contains(t, job, "TRIVY_USERNAME: $CI_REGISTRY_USER")
	assert.Contains(t, job, "TRIVY_PASSWORD: $CI_REGISTRY_PASSWORD")
	assert.Contains(t, job, `TRIVY_INSECURE: "true"`,
		"빌드는 --insecure-registry 로 push 한다 — 스캔만 인증서를 검증하면 같은 레지스트리를 못 읽는다")
}

func TestRenderGitLabCI_ScanJobUsesTargetCredentialVariables(t *testing.T) {
	in := scanInput(port.CIPlatformGitLabCI, testScannerEndpoint)
	in.ImageTarget = harborTarget()
	_, content := renderPipelineFor(in)

	job := section(t, content, "image-scan:\n", "\ndeploy:")
	assert.Contains(t, job, "TRIVY_USERNAME: $HARBOR_USERNAME")
	assert.Contains(t, job, "TRIVY_PASSWORD: $HARBOR_PASSWORD")
}

// Jenkins 의 스캐너 컨테이너는 빌더와 같은 파이프라인 자격증명 Secret 을 읽는다.
// 트레이스를 끈 채로 넘긴다 — Jenkins 는 sh -xe 라 그대로면 비밀번호가 로그에 남는다.
func TestRenderJenkinsfile_ScannerGetsRegistryCredentials(t *testing.T) {
	content := renderJenkinsfile(scanInput(port.CIPlatformJenkins, testScannerEndpoint))

	scanner := section(t, content, "- name: scanner", "- name: dind")
	assert.Contains(t, scanner, "secretRef: {name: "+ciSecretName("api")+"}")

	stage := section(t, content, "stage('"+imageScanStageName+"')", "stage('Deploy')")
	exportLine := `export TRIVY_USERNAME="$HARBOR_USERNAME" TRIVY_PASSWORD="$HARBOR_PASSWORD"`
	require.Contains(t, stage, exportLine)
	assert.Less(t, strings.Index(stage, "set +x"), strings.Index(stage, exportLine),
		"자격증명을 넘기기 전에 트레이스를 꺼야 한다")
	assert.Contains(t, stage, "TRIVY_INSECURE=true",
		"빌드가 --insecure-registry 로 push 하는 레지스트리다")
}

// GitHub Actions 는 빌드와 같은 식으로 자격증명을 읽는다. 빌드가 insecure 로 push 하지
// 않으므로 스캔도 인증서를 검증한다.
func TestRenderGitHubWorkflow_ScanJobGetsRegistryCredentials(t *testing.T) {
	_, content := renderPipelineFor(scanInput(port.CIPlatformGitHubActions, testScannerEndpoint))

	job := section(t, content, "  "+imageScanStageID+":\n", "\n  deploy:")
	assert.Contains(t, job, "TRIVY_USERNAME: ${{ secrets.HARBOR_USERNAME }}")
	assert.Contains(t, job, "TRIVY_PASSWORD: ${{ secrets.HARBOR_PASSWORD }}")
	assert.NotContains(t, job, "TRIVY_INSECURE")
}
