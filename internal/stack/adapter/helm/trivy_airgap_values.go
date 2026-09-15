package helm

import shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"

// trivyAirgapDBValues 는 에어갭 설치에서 Trivy 서버가 취약점 DB 를 받을 곳이다.
//
// 에어갭 스택은 API 가 설치한다(airgap/scripts/29-install-stacks-via-api.sh). 그 경로는
// airgap/helm/stack-values/trivy.yaml 을 읽지 않으므로, 여기서 넣지 않으면 서버가
// ghcr.io 에서 DB 를 받으려다 스캔이 전부 실패한다.
//
// 경로 규칙은 shared 가 소유한다 — CI 잡의 Java DB 미러(cicd)와 같은 레지스트리를 본다.
// 내부 레지스트리는 plain HTTP 라 insecure 를 켠다. 에어갭이 아니면 nil 이다.
func trivyAirgapDBValues(ociRegistry string) map[string]any {
	repository := shareddomain.TrivyDBRepository(ociRegistry)
	if repository == "" {
		return nil
	}
	return map[string]any{
		"trivy": map[string]any{
			"dbRepository": repository,
			"extraEnvVars": map[string]any{"TRIVY_INSECURE": "true"},
		},
	}
}
