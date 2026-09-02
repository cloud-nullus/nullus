package domain

// 런타임에 만들어지는 매니페스트가 참조하는 컨테이너 이미지.
//
// ── 왜 여기 모아 두는가 ──────────────────────────────────────────────────
// 에어갭 이미지 목록(`airgap/images/images.txt`)은 `helm template` 렌더 결과
// 에서 자동 생성된다. 그래서 **차트에 없는 이미지는 목록에 오르지 않는다.**
//
// Nullus 는 설치·프로비저닝·파이프라인 과정에서 매니페스트를 Go 코드로 직접
// 만들어 적용한다. 그 안의 이미지는 helm 이 볼 수 없어 번들에서 조용히 빠지고,
// 폐쇄망에서 ImagePullBackOff 로만 드러난다 — 설치가 한참 진행된 뒤에.
//
// 실제로 그렇게 빠져 있었다(2026-09-02 실측): mc · curl · node · docker 5종.
// 그중 Jenkins 이미지 하나만 과거에 손으로 INFRA_IMAGES 에 덧붙여져 있었고,
// 그 주석이 같은 문제를 이미 기록하고 있었다 — 한 건씩 손으로 막는 방식으로는
// 다음 것을 놓친다.
//
// 그래서 상수를 여기 모으고, RuntimeImages() 가 돌려주는 것이 번들에 전부
// 들어 있는지 테스트가 지킨다(runtime_images_test.go).
//
// **이미지를 새로 참조하게 되면 여기 상수를 만들고 아래 목록에 넣는다.**
// 그러지 않으면 폐쇄망 설치에서 그 단계가 실패한다.
const (
	// MinIOClientImage — GitLab 오브젝트 스토리지 버킷 부트스트랩 Job.
	// (internal/stack/adapter/helm/object-storage-buckets.go)
	MinIOClientImage = "minio/mc:RELEASE.2025-05-21T01-59-54Z"

	// CurlImage — Harbor·Nexus 프로비저닝 Job 이 REST API 를 호출한다.
	// (internal/stack/adapter/helm/{harbor,nexus}-provisioning.go)
	CurlImage = "curlimages/curl:8.11.1"

	// KubectlImage — OpenBao init/bootstrap 사이드카.
	// (internal/stack/adapter/helm/openbao-values.go)
	KubectlImage = "docker.io/bitnamilegacy/kubectl:1.33.4"

	// NodeBuilderImage — 생성된 React 앱 Dockerfile 의 빌드 단계.
	// 파이프라인이 클러스터 안에서 이 이미지로 빌드한다.
	// (internal/cicd/adapter/scaffold/react_app.go)
	NodeBuilderImage = "node:22-alpine"

	// JenkinsAgentImage / JenkinsDindImage — Jenkins kubernetes 플러그인이
	// 빌드마다 띄우는 파드. GitLab CI 와 달리 실행기를 차트로 세우지 않는다.
	// (internal/cicd/adapter/scaffold/jenkins_renderer.go)
	JenkinsAgentImage = "docker:27-cli"
	JenkinsDindImage  = "docker:27-dind"
)

// RuntimeImages 는 위 이미지 전부를 돌려준다.
//
// 에어갭 번들 생성기와 드리프트 검사가 이 목록을 단일 출처로 쓴다.
func RuntimeImages() []string {
	return []string{
		MinIOClientImage,
		CurlImage,
		KubectlImage,
		NodeBuilderImage,
		JenkinsAgentImage,
		JenkinsDindImage,
	}
}
