package domain

import "strings"

// 스택이 클러스터에 하나만 두고 함께 쓰는 공용 자원의 식별자.
//
// 설치(stack/adapter/helm)가 만들고, 클러스터의 마지막 스택을 지울 때(stack/usecase) 회수하며,
// 백업(backup/adapter/kube)이 담는다. 세 곳이 같은 이름과 같은 판별 규칙을 봐야 한다 — 백업
// 모듈은 스택 모듈을 import 하지 않으므로(모듈 경계) 여기 둔다. 지우는 쪽은 알고 백업하는
// 쪽은 모르던 것이 실제로 있었다(Argo CD CRD).
const (
	// LabelInstalledBy 는 Nullus 가 새로 설치한 공용 자원의 표시다. 설치는 클러스터에 이미
	// 있던 것을 재사용하므로, 새로 깐 것에만 달아 둘을 가른다.
	LabelInstalledBy  = "nullus.io/installed-by"
	InstalledByNullus = "nullus"

	// 내부 CA — 설치가 cert-manager 단계에서 만든다. 이름이 Nullus 전용이라 표시 없이도 Nullus
	// 것이다. CA Secret 은 cert-manager 가 설치된 네임스페이스(cluster-resource-namespace)에 있다.
	InternalCABootstrapIssuerName = "nullus-selfsigned-bootstrap"
	InternalCAIssuerName          = "nullus-internal-ca-issuer"
	InternalCACertName            = "nullus-internal-ca-cert"
	InternalCASecretName          = "nullus-internal-ca"

	// EnvoyGatewayClassName 은 설치기가 만드는 GatewayClass 다.
	EnvoyGatewayClassName      = "envoy"
	EnvoyGatewayControllerName = "gateway.envoyproxy.io/gatewayclass-controller"

	// CertManagerCRDSuffix 는 cert-manager CRD 의 API 그룹이다(acme.cert-manager.io 포함).
	CertManagerCRDSuffix = ".cert-manager.io"
)

// gatewayCRDSuffixes 는 게이트웨이 단계가 까는 CRD 의 API 그룹이다. Gateway API 표준 CRD 와 함께
// Envoy Gateway 차트가 실험용 Gateway API CRD 와 자기 CRD 를 crds/ 로 깐다 — 이름 목록으로 고르면
// backendtlspolicies·x-k8s.io·envoyproxy.io 를 놓친다(kind-nullus-platform 실측).
var gatewayCRDSuffixes = []string{
	".gateway.networking.k8s.io",
	".gateway.networking.x-k8s.io",
	".gateway.envoyproxy.io",
}

// IsGatewayCRD 는 게이트웨이 단계가 까는 CRD 인지 본다.
func IsGatewayCRD(name string) bool {
	for _, suffix := range gatewayCRDSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}
