package domain

import "strings"

// 스택이 클러스터에 하나만 두고 함께 쓰는 공용 자원.
//
// 설치(adapter/helm)가 만들고 클러스터의 마지막 스택을 지울 때(usecase) 회수한다. 양쪽이 같은
// 이름과 같은 판별 규칙을 봐야 하므로 여기 둔다 — usecase 가 adapter 를 import 하면 레이어
// 방향이 뒤집힌다.
//
// 회수 원칙은 하나다. Nullus 가 설치한 것은 남기지 않고, Nullus 가 설치하지 않은 것은 건드리지
// 않는다. 설치는 클러스터에 이미 있는 cert-manager·metrics-server 를 재사용하므로 둘이 섞여
// 있을 수 있다 — 그것을 가르는 것이 설치 표시(LabelInstalledBy)다. 남기면 다음 설치를 막는다:
// 소유 표시가 남은 CRD 는 사용자가 다른 이름으로 cert-manager 를 설치할 때 Helm ownership
// 충돌을 낸다(ESO·Argo CD CRD 에서 겪은 것과 같다).
const (
	// LabelInstalledBy 는 Nullus 가 새로 설치한 공용 자원의 표시다.
	LabelInstalledBy  = "nullus.io/installed-by"
	InstalledByNullus = "nullus"

	CertManagerReleaseName   = "cert-manager"
	MetricsServerReleaseName = "metrics-server"

	// 내부 CA — 설치가 cert-manager 단계에서 만든다. 이름이 Nullus 전용이라 표시 없이도
	// Nullus 것이다.
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

// installMarkerValuePaths 는 공용 릴리스 values 에서 설치 표시가 들어가는 자리다. 두 차트 모두
// 공식 필드로 차트가 만드는 리소스 전부에 라벨을 단다. 표시를 values 에 두는 이유는 삭제가
// 릴리스 values 로 판별하기 때문이다 — 옛 설치의 지문도 같은 values 에서 읽는다.
var installMarkerValuePaths = map[string][]string{
	CertManagerReleaseName:   {"global", "commonLabels", LabelInstalledBy},
	MetricsServerReleaseName: {"commonLabels", LabelInstalledBy},
}

// InstallMarkerValuePath 는 릴리스의 설치 표시 자리다. ok=false 면 공용 릴리스가 아니다.
func InstallMarkerValuePath(releaseName string) ([]string, bool) {
	path, ok := installMarkerValuePaths[releaseName]
	if !ok {
		return nil, false
	}
	return append([]string(nil), path...), true
}

// IsNullusInstalledRelease 는 릴리스 values 를 보고 Nullus 가 설치한 공용 릴리스인지 판별한다.
//
// 설치 표시가 있으면 Nullus 것이다. 표시가 없으면 표시를 달기 전 Nullus 가 설치한 모양(지문)인지
// 본다 — 그 전에는 표시가 없었으므로 지문 없이는 옛 설치를 회수할 수 없다.
//
// 지문은 자원 수치가 아니라 구조로 본다. 수치는 관리자 기본값으로 바뀐다(kind-nullus-platform 의
// webhook 은 DefaultValues 의 250m 가 아니라 500m 로 설치돼 있었다). 지문이 고객 설치와 우연히
// 겹쳐도, 삭제는 그 자원을 쓰는 곳이 있으면 남기므로 쓰던 것이 사라지지는 않는다.
func IsNullusInstalledRelease(releaseName string, values map[string]any) bool {
	path, ok := installMarkerValuePaths[releaseName]
	if !ok {
		return false
	}
	if marker, found := lookupNestedValue(values, path); found && marker == InstalledByNullus {
		return true
	}
	switch releaseName {
	case CertManagerReleaseName:
		return isLegacyNullusCertManagerValues(values)
	case MetricsServerReleaseName:
		return isLegacyNullusMetricsServerValues(values)
	}
	return false
}

// isLegacyNullusCertManagerValues 는 옛 설치기의 cert-manager values 모양이다.
//
// 지금 차트 문서는 crds.enabled 를 쓰라고 하는데 설치기는 옛 키 installCRDs 를 쓰고, 컨트롤러·
// webhook·cainjector 셋 모두에 자원을 지정한다. installCRDs 하나만으로는 고객 설치와 구분되지
// 않는다(흔한 --set installCRDs=true).
func isLegacyNullusCertManagerValues(values map[string]any) bool {
	if installCRDs, _ := values["installCRDs"].(bool); !installCRDs {
		return false
	}
	for _, path := range [][]string{
		{"resources"},
		{"webhook", "resources"},
		{"cainjector", "resources"},
	} {
		value, found := lookupNestedValue(values, path)
		if !found {
			return false
		}
		if _, isMap := value.(map[string]any); !isMap {
			return false
		}
	}
	return true
}

// legacyMetricsServerArgs 는 옛 설치기가 넘긴 인자다. kind 사용자는 --kubelet-insecure-tls 만 다는
// 경우가 많아, 주소 종류 순서까지 맞아야 Nullus 것으로 본다.
var legacyMetricsServerArgs = []string{
	"--kubelet-insecure-tls",
	"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
}

func isLegacyNullusMetricsServerValues(values map[string]any) bool {
	if _, isMap := values["resources"].(map[string]any); !isMap {
		return false
	}
	args := map[string]bool{}
	switch raw := values["args"].(type) {
	case []any:
		for _, arg := range raw {
			if s, ok := arg.(string); ok {
				args[s] = true
			}
		}
	case []string:
		for _, arg := range raw {
			args[arg] = true
		}
	}
	for _, want := range legacyMetricsServerArgs {
		if !args[want] {
			return false
		}
	}
	return true
}

// lookupNestedValue 는 중첩 경로의 값을 찾는다. 중간이 매핑이 아니면 없는 것으로 본다.
func lookupNestedValue(values map[string]any, path []string) (any, bool) {
	if len(path) == 0 {
		return nil, false
	}
	current := values
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	value, ok := current[path[len(path)-1]]
	return value, ok
}
