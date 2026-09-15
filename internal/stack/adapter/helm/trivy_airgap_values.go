package helm

import "strings"

// trivyDBRepositoryPath 는 에어갭 번들이 trivy-db 를 올리는 내부 레지스트리 경로다.
// 14-push-oci-artifacts.sh 의 compute_target 이 ghcr.io/aquasecurity/trivy-db:2 에서
// 레지스트리 호스트만 바꿔 올린다(태그는 trivy 가 스스로 붙인다).
const trivyDBRepositoryPath = "aquasecurity/trivy-db"

// trivyAirgapDBValues 는 에어갭 설치에서 Trivy 서버가 취약점 DB 를 받을 곳이다.
//
// 에어갭 스택은 API 가 설치한다(airgap/scripts/29-install-stacks-via-api.sh). 그 경로는
// airgap/helm/stack-values/trivy.yaml 을 읽지 않으므로, 여기서 넣지 않으면 서버가
// ghcr.io 에서 DB 를 받으려다 스캔이 전부 실패한다.
//
// ociRegistry 는 NULLUS_HELM_OCI_REGISTRY(예: kind-registry:5000/charts)다. 차트 경로가
// 아니라 같은 레지스트리의 루트를 쓴다 — 파드가 그 레지스트리를 부르는 이름이 이 값이다
// (호스트에서 쓰는 localhost:5001 은 파드 안에서 파드 자신이다). 내부 레지스트리는
// plain HTTP 라 insecure 를 켠다. 에어갭이 아니면 nil 이다.
func trivyAirgapDBValues(ociRegistry string) map[string]any {
	host := strings.SplitN(strings.Trim(strings.TrimSpace(ociRegistry), "/"), "/", 2)[0]
	if host == "" {
		return nil
	}
	return map[string]any{
		"trivy": map[string]any{
			"dbRepository": host + "/" + trivyDBRepositoryPath,
			"extraEnvVars": map[string]any{"TRIVY_INSECURE": "true"},
		},
	}
}
