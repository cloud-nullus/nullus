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
