package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const testJavaDBRepository = "kind-registry:5000/aquasecurity/trivy-java-db"

// CI 잡의 trivy client 는 Maven 메타데이터가 없는 JAR 을 만나면 Java DB(약 900MiB)를
// 스스로 받는다 — 기본 주소는 mirror.gcr.io 다(kind 실측). 에어갭에서는 닿지 않으므로
// 에어갭 설치면 반입한 내부 미러를 가리킨다.
func TestRenderGitLabCI_ScanJobUsesAirgapJavaDBMirror(t *testing.T) {
	in := scanInput(port.CIPlatformGitLabCI, testScannerEndpoint)
	in.ImageTarget = gitlabTarget()
	in.ImageScannerJavaDBRepository = testJavaDBRepository
	_, content := renderPipelineFor(in)

	job := section(t, content, "image-scan:\n", "\ndeploy:")
	assert.Contains(t, job, `TRIVY_JAVA_DB_REPOSITORY: "`+testJavaDBRepository+`"`)
}

func TestRenderJenkinsfile_ScannerUsesAirgapJavaDBMirror(t *testing.T) {
	in := scanInput(port.CIPlatformJenkins, testScannerEndpoint)
	in.ImageScannerJavaDBRepository = testJavaDBRepository
	content := renderJenkinsfile(in)

	stage := section(t, content, "stage('"+imageScanStageName+"')", "stage('Deploy')")
	assert.Contains(t, stage, `export TRIVY_JAVA_DB_REPOSITORY="`+testJavaDBRepository+`"`)
}

// 온라인 설치는 업스트림 기본값을 쓴다 — 빈 값을 박으면 client 가 빈 저장소를 찾는다.
func TestRender_ScanJobsOmitJavaDBRepositoryOutsideAirgap(t *testing.T) {
	in := scanInput(port.CIPlatformGitLabCI, testScannerEndpoint)
	in.ImageTarget = gitlabTarget()
	_, gitlab := renderPipelineFor(in)
	assert.NotContains(t, gitlab, "TRIVY_JAVA_DB_REPOSITORY")

	assert.NotContains(t, renderJenkinsfile(scanInput(port.CIPlatformJenkins, testScannerEndpoint)), "TRIVY_JAVA_DB_REPOSITORY")
}

// GitHub 호스티드 러너는 클러스터 안의 내부 레지스트리에 닿지 않는다 — 에어갭 설치에서도
// 넣지 않는다. 넣으면 온라인 러너가 닿지 않는 주소에서 Java DB 를 찾는다.
func TestRenderGitHubWorkflow_ScanJobNeverUsesInternalJavaDBMirror(t *testing.T) {
	in := scanInput(port.CIPlatformGitHubActions, testScannerEndpoint)
	in.ImageScannerJavaDBRepository = testJavaDBRepository
	_, content := renderPipelineFor(in)
	assert.NotContains(t, content, "TRIVY_JAVA_DB_REPOSITORY")
}
