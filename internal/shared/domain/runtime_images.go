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
	// 오브젝트 스토리지 이미지는 MinIO 포크인 SILO(pgsty/silo)에서 받는다.
	//
	// MinIO 는 Docker Hub 배포를 멈춘 데 이어(docker.io/minio/* 거절) quay.io 저장소도
	// 익명 pull 을 막았다 — 2026-10-17 실측: quay.io/minio/minio 와 quay.io/minio/mc 가
	// `latest` 든 고정 RELEASE 태그든 401 unauthorized 다. 받을 수 있는 출처가 없어
	// 로컬 인프라 기동과 스택 설치(installing_minio)가 처음부터 멈춘다.
	//
	// SILO 는 MinIO 서버의 커뮤니티 포크다(AGPL-3.0, MinIO Inc. 와 무관). 라이선스는
	// 업스트림과 같으므로 AGPL 의무는 그대로다 — 해결되는 것은 "받을 수 없다" 뿐이다.
	// 데이터 포맷(.minio.sys)·환경변수(MINIO_*)·프로토콜(minio_* 메트릭, x-minio-* 헤더,
	// /minio/* 경로)을 그대로 유지하므로 설치 코드와 클라이언트는 손대지 않는다.
	//
	// MinIOServerImage — 스택 오브젝트 스토리지(installing_minio). 차트 기본값이
	// quay.io 를 가리키므로 values 로 덮는다.
	MinIOServerImage = "docker.io/pgsty/silo:RELEASE.2026-10-15T00-00-00Z"

	// MinIOClientImage — MinIO 차트의 버킷 훅(mcImage). 차트 기본값이 quay.io 라 덮는다.
	//
	// GitLab 버킷 부트스트랩 Job 은 더 이상 이것을 쓰지 않는다 — 구현체를 갈아
	// 끼울 수 있게 S3 CreateBucket 을 curl 로 직접 부른다(CurlImage).
	MinIOClientImage = "docker.io/pgsty/mc:RELEASE.2026-09-16T00-00-00Z"

	// SeaweedFSImage — 스택 오브젝트 스토리지의 다른 구현체(installing_seaweedfs).
	// Apache-2.0 이고 상류가 amd64·arm64 를 함께 낸다. 태그는 차트 appVersion 과 같게 둔다
	// (domain.SeaweedFSAppVersion) — 차트 기본값도 같은 태그지만, 출처를 values 로 못박아
	// 에어갭 번들과 같은 참조를 보게 한다.
	SeaweedFSImage = "docker.io/chrislusf/seaweedfs:4.48"

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

	// JenkinsAgentImage — Jenkins kubernetes 플러그인이 빌드마다 띄우는 파드.
	// GitLab CI 와 달리 실행기를 차트로 세우지 않는다.
	// (internal/cicd/adapter/scaffold/jenkins_renderer.go)
	JenkinsAgentImage = "docker:27-cli"

	// DindImage — 이미지 빌드용 docker 데몬. Jenkins 빌드 파드의 사이드카이자
	// GitLab build 잡의 서비스다.
	// (internal/cicd/adapter/scaffold/{jenkins_renderer,renderer}.go)
	DindImage = "docker:27-dind"

	// GitLabCIBuildImage / GitLabCIDeployImage — 생성된 .gitlab-ci.yml 의 build·deploy 잡.
	// 쿠버네티스 실행기가 잡마다 이 이미지로 파드를 띄운다.
	// (internal/cicd/adapter/scaffold/renderer.go)
	GitLabCIBuildImage  = "docker:27"
	GitLabCIDeployImage = "alpine:3.20"

	// TrivyCLIImage — 파이프라인 이미지 스캔 단계의 Trivy CLI. 취약점 DB 는 들어 있지 않다 —
	// client 로 붙어 스택 Trivy 서버에 매칭을 맡긴다. 에어갭 번들은 Trivy 차트가 같은 태그를
	// 이미 싣는다(images.txt). RuntimeImages 에 넣지 않는 이유다 — 넣으면 생성기가 두 번 싣는다.
	// 정책 변수와 함께 CI 에 밀린다(port.ScanPolicyVariables) — 올리면 파이프라인 정의를
	// 다시 쓰지 않아도 다음 실행부터 따라온다.
	// (internal/cicd/adapter/scaffold/image_scan.go)
	TrivyCLIImage = "aquasec/trivy:0.74.0"

	// SonarScannerImage — 파이프라인 SAST 단계(GitLab CI·GitHub Actions·Jenkins). 공식
	// sonarsource/sonar-scanner-cli 는 amd64 빌드만 게시해(Docker Hub 전 태그 실측, 2026-10)
	// arm64 노드와 에어갭 arm64 번들에서 단계가 서지 않았다 — 플랫폼 독립 스캐너 zip 을 멀티아치
	// JRE 위에 우리가 굽는다(deploy/images/sonar-scanner, CD 의 build-and-push-sonar-scanner,
	// 로컬은 scripts/build-sonar-scanner-image.sh). 태그는 Dockerfile 의 스캐너 버전이다
	// (테스트가 둘을 묶는다). ghcr 패키지는 처음 만들어질 때 private 이므로 public 으로 바꿔야
	// 익명 pull 이 된다(릴리즈 정책 §7.1).
	// (internal/cicd/adapter/scaffold/sast.go)
	SonarScannerImage = "ghcr.io/cloud-nullus/nullus/nullus-sonar-scanner:8.1.0.6389"

	// GitleaksImage 는 파이프라인 시크릿 탐지 단계가 쓰는 gitleaks 다(nullus-plan#80). 상류가
	// amd64·arm64 를 모두 내므로 그대로 쓴다. 태그는 앱 저장소에 넣는 pre-commit 훅의 rev 와
	// 같아야 한다(scaffold.gitleaksVersionTag 가 여기서 뽑는다) — 로컬과 CI 가 같은 규칙을 봐야 한다.
	GitleaksImage = "ghcr.io/gitleaks/gitleaks:v8.30.1"
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
		DindImage,
		GitLabCIBuildImage,
		GitLabCIDeployImage,
		SonarScannerImage,
		GitleaksImage,
	}
}
