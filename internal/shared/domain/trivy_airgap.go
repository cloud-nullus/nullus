package domain

import "strings"

// 에어갭 번들이 Trivy DB 를 올리는 내부 레지스트리 경로다.
// airgap/scripts/14-push-oci-artifacts.sh 의 compute_target 이 업스트림
// (ghcr.io/aquasecurity/trivy-db:2 등)에서 레지스트리 호스트만 바꿔 올린다.
// 태그는 trivy 가 스키마 버전으로 스스로 붙인다.
const (
	trivyDBRepositoryPath     = "aquasecurity/trivy-db"
	trivyJavaDBRepositoryPath = "aquasecurity/trivy-java-db"
)

// AirgapRegistryHost 는 에어갭 레지스트리 설정(NULLUS_HELM_OCI_REGISTRY, 예
// kind-registry:5000/charts)에서 호스트만 뗀다. 파드가 그 레지스트리를 부르는 이름이다 —
// 호스트에서 쓰는 localhost:5001 은 파드 안에서 파드 자신이다. 에어갭이 아니면 빈 값이다.
func AirgapRegistryHost(ociRegistry string) string {
	return strings.SplitN(strings.Trim(strings.TrimSpace(ociRegistry), "/"), "/", 2)[0]
}

// TrivyDBRepository 는 에어갭 Trivy 서버가 취약점 DB 를 받을 곳이다(stack 모듈).
func TrivyDBRepository(ociRegistry string) string {
	return airgapRepository(ociRegistry, trivyDBRepositoryPath)
}

// TrivyJavaDBRepository 는 에어갭 CI 잡의 trivy client 가 Java DB 를 받을 곳이다(cicd 모듈).
// client 는 Maven 메타데이터가 없는 JAR 을 만나면 Java DB 를 스스로 받는데, 기본 주소
// (mirror.gcr.io)는 에어갭에서 닿지 않는다.
func TrivyJavaDBRepository(ociRegistry string) string {
	return airgapRepository(ociRegistry, trivyJavaDBRepositoryPath)
}

func airgapRepository(ociRegistry, path string) string {
	host := AirgapRegistryHost(ociRegistry)
	if host == "" {
		return ""
	}
	return host + "/" + path
}
