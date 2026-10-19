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

// PrometheusOperatorCRDsReleaseName 은 Prometheus Operator CRD(ServiceMonitor·Probe·PrometheusRule 등)만
// 담은 릴리스다. 이 CRD 를 쓰는 리소스가 다른 릴리스 여럿에 들어 있다.
const PrometheusOperatorCRDsReleaseName = "prometheus-operator-crds"

// KubePrometheusStackReleaseName 은 Prometheus·Alertmanager·Operator 릴리스다.
const KubePrometheusStackReleaseName = "kube-prometheus-stack"

// InstalledHelmReleaseNames 는 스택 설치가 만들어 내는 모든 Helm 릴리스다.
// 로깅/트레이스처럼 설정에 따라 차트가 갈리는 단계는 가능한 변형을 모두 포함한다.
var InstalledHelmReleaseNames = []string{
	"cert-manager",
	// Prometheus Operator CRD. ServiceMonitor 를 만드는 단계들이
	// kube-prometheus-stack 설치보다 앞서 먼저 깔린다. 빼면 스택을 지워도
	// 릴리스가 남아 다음 설치가 소유권 충돌로 막힌다.
	PrometheusOperatorCRDsReleaseName,
	"metrics-server",
	"openbao",
	"external-secrets",
	"nullus-postgresql",
	"nullus-minio",
	// installing_seaweedfs. 오브젝트 스토리지 슬롯은 둘 중 하나만 서지만 삭제는
	// 어느 쪽인지 모른 채 훑는다. 빼면 스택을 지워도 데이터 PVC 가 남는다.
	SeaweedFSReleaseName,
	// installing_container_registry 변형. 남기면 레지스트리 파드와 PVC 가 그대로
	// 떠 있어 다음 스택이 같은 네임스페이스를 쓸 때 리소스를 물고 늘어진다.
	HarborReleaseName,
	NexusReleaseName,
	// installing_gitlab / installing_gitea 변형. 소스 저장소 슬롯은 둘 중 하나만
	// 서지만, 삭제는 어느 쪽이 설치됐는지 모른 채 훑으므로 둘 다 적는다.
	// Gitea 를 빼면 스택을 지워도 리포 데이터 PVC 가 그대로 남는다.
	"gitlab",
	GiteaReleaseName,
	"argo-cd",
	// 이미지 스캐너. 선택 항목이라 안 깔린 스택도 있지만, 삭제는 무엇이
	// 설치됐는지 모른 채 훑으므로 여기 적어 둔다. 빼면 스캐너를 고른 스택을
	// 지워도 릴리스와 취약점 DB PVC 가 그대로 남는다.
	TrivyReleaseName,
	// SAST 서버. 선택 항목이지만 삭제는 무엇이 설치됐는지 모른 채 훑는다.
	SonarQubeReleaseName,
	// installing_runner / installing_jenkins 변형. CI 슬롯도 둘 중 하나만 서지만
	// 삭제는 어느 쪽인지 모른 채 훑으므로 둘 다 적는다.
	"gitlab-runner",
	JenkinsReleaseName,
	KubePrometheusStackReleaseName,
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

// HelmReleaseNamesInUninstallOrder 는 삭제가 릴리스를 지우는 순서다.
//
// CRD 만 담은 릴리스는 그 CRD 를 쓰는 릴리스를 다 지운 뒤에 지운다. helm uninstall 은 릴리스
// 매니페스트를 객체로 만든 다음 지우는데, 그 안의 ServiceMonitor·Probe 의 CRD 가 이미 없으면 객체를
// 만들지 못해 아무것도 지우지 않고 "failed to delete release" 로 멈춘다. 2026-10-12 실측 — CRD 릴리스가
// 두 번째로 지워지자 argo-cd·nullus-minio·kube-prometheus-stack·grafana 가 모두 그렇게 멈췄고,
// 네임스페이스 회수가 안쪽은 치웠지만 kube-system 의 Service 5개와 grafana ClusterRole 이 남아 다음
// 스택 설치가 installing_prometheus 에서 소유권 충돌로 실패했다.
//
// 설치 순서(InstalledHelmReleaseNames)는 그대로 둔다 — CRD 가 먼저 깔려야 하는 것은 설치 쪽 사정이다.
func HelmReleaseNamesInUninstallOrder() []string {
	all := AllHelmReleaseNames()
	ordered := make([]string, 0, len(all))
	var crdOnly []string
	for _, name := range all {
		if name == PrometheusOperatorCRDsReleaseName {
			crdOnly = append(crdOnly, name)
			continue
		}
		ordered = append(ordered, name)
	}
	return append(ordered, crdOnly...)
}
