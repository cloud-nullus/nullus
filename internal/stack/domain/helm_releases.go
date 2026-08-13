package domain

// Helm 릴리스 이름의 단일 출처.
//
// 설치(오케스트레이터의 ChartSpec)와 삭제(DeleteStack)가 각자 목록을 들고 있으면
// 반드시 어긋난다. 실제로 external-secrets 가 설치 목록에만 있고 삭제 목록에는
// 없어서, 스택을 지워도 ESO 의 CRD 24개와 ClusterRole 5개가 남아 다음 설치를
// Helm ownership 충돌로 막았다.
//
// 두 레이어 모두 domain 을 향하므로 여기에 둔다 — helm 어댑터는 설치 시,
// usecase 는 삭제 시 이 목록을 기준으로 삼는다.

// InstalledHelmReleaseNames 는 스택 설치가 만들어 내는 모든 Helm 릴리스다.
// 로깅/트레이스처럼 설정에 따라 차트가 갈리는 단계는 가능한 변형을 모두 포함한다.
var InstalledHelmReleaseNames = []string{
	"cert-manager",
	"metrics-server",
	"openbao",
	"external-secrets",
	"nullus-postgresql",
	"nullus-minio",
	// installing_container_registry 변형. 남기면 레지스트리 파드와 PVC 가 그대로
	// 떠 있어 다음 스택이 같은 네임스페이스를 쓸 때 리소스를 물고 늘어진다.
	HarborReleaseName,
	NexusReleaseName,
	"gitlab",
	"argo-cd",
	"gitlab-runner",
	"kube-prometheus-stack",
	"grafana",
	"loki",
	// installing_log_search 변형
	"opensearch",
	"elasticsearch",
	// installing_opentelemetry 변형
	"opentelemetry-collector",
	"tempo",
	"jaeger",
	// installing_otel_collector. 추적 계층과 별개의 릴리스이므로 따로 적는다 —
	// 빼면 스택을 지워도 수집기 파드와 ServiceMonitor 가 남는다.
	OTelCollectorReleaseName,
	// installing_otel_agent. DaemonSet 과 ClusterRole 을 만들므로 빠지면
	// 스택을 지워도 노드마다 수집기가 남고 다음 설치가 소유권 충돌로 막힌다.
	OTelAgentReleaseName,
	"eg",
}

// LegacyHelmReleaseNames 는 과거 이름으로 설치된 스택을 지우기 위해 남겨 둔다.
// 설치 경로는 더 이상 이 이름을 쓰지 않는다.
var LegacyHelmReleaseNames = []string{
	"postgresql",
	"minio",
	"envoy-gateway",
}

// AllHelmReleaseNames 는 삭제가 훑어야 하는 릴리스 전체다.
func AllHelmReleaseNames() []string {
	names := make([]string, 0, len(InstalledHelmReleaseNames)+len(LegacyHelmReleaseNames))
	names = append(names, InstalledHelmReleaseNames...)
	names = append(names, LegacyHelmReleaseNames...)
	return names
}
