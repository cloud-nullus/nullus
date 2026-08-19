package helm

import (
	"context"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// JenkinsGiteaCredentialID 는 Jenkins 에 등록되는 Gitea 자격증명 식별자다.
//
// CI/CD 모듈의 job 설정이 같은 이름을 참조한다(cicd/usecase 의 giteaCredentialID).
// 모듈 간 직접 import 는 금지되므로 값을 각자 두되, 갈라지면 job 이 브랜치를
// 하나도 찾지 못하므로 계약 테스트로 묶는다.
const JenkinsGiteaCredentialID = "nullus-gitea"

func WithHelmStepMetadataRepository(repo port.HelmStepMetadataRepository) OrchestratorOption {
	return func(o *Orchestrator) {
		o.helmStepMetadataRepo = repo
	}
}

func (o *Orchestrator) chartSpecForStep(step string) (ChartSpec, bool) {
	if o == nil {
		return ChartSpec{}, false
	}

	if o.helmStepMetadataRepo != nil {
		item, err := o.helmStepMetadataRepo.GetByStep(context.Background(), step)
		if err == nil && item != nil {
			return chartSpecFromMetadata(item), true
		}
	}

	if spec, ok := defaultChartSpecForStep(step); ok {
		return spec, true
	}

	return ChartSpec{}, false
}

func chartSpecFromMetadata(item *domain.HelmStepMetadata) ChartSpec {
	if item == nil {
		return ChartSpec{}
	}

	values := DefaultValues(item.StepName)
	return ChartSpec{
		ReleaseName: strings.TrimSpace(item.ReleaseName),
		ChartName:   strings.TrimSpace(item.ChartName),
		RepoURL:     strings.TrimSpace(item.RepoURL),
		Version:     strings.TrimSpace(item.Version),
		Namespace:   strings.TrimSpace(item.Namespace),
		Values:      values,
		Wait:        item.Wait,
	}
}

// DefaultChartSpecForStep 은 단계의 기본 차트 스펙을 돌려준다.
//
// 설치가 실제로 어떤 차트를 쓰는지 다른 패키지에서 확인할 수 있도록 공개한다 —
// 화면에 안내하는 버전과 어긋나지 않는지 계약 테스트가 이 값을 본다.
func DefaultChartSpecForStep(step string) (ChartSpec, bool) {
	return defaultChartSpecForStep(step)
}

func defaultChartSpecForStep(step string) (ChartSpec, bool) {
	switch step {
	case stepInstallingCertManager:
		return ChartSpec{
			ChartName: "cert-manager",
			RepoURL:   "https://charts.jetstack.io",
			Version:   "v1.16.3",
			Namespace: "cert-manager",
			Values:    DefaultValues(stepInstallingCertManager),
			Wait:      false,
		}, true
	case "installing_metrics_server":
		return ChartSpec{
			ChartName: "metrics-server",
			RepoURL:   "https://kubernetes-sigs.github.io/metrics-server/",
			Version:   "3.12.2",
			// 스택 네임스페이스에 깔면 안 된다. 이 차트가 만드는 APIService
			// v1beta1.metrics.k8s.io 는 cluster-scoped 라, 스택을 지우면 Service 만
			// 사라지고 APIService 는 죽은 대상을 계속 가리킨다. 그러면 API discovery 가
			// 실패해 **클러스터의 모든 네임스페이스 삭제가 교착된다** — 실제로 무관한
			// 스택 셋이 이틀 넘게 Terminating 에 갇혔다.
			// cert-manager 가 같은 이유로 자기 네임스페이스를 못박고 있다.
			Namespace: "kube-system",
			Values:    DefaultValues("installing_metrics_server"),
			Wait:      false,
		}, true
	case "installing_postgresql":
		return ChartSpec{
			// Service 이름이 릴리스명에서 나오고, 연결정보가 그 이름으로 접속을
			// 안내한다. 둘이 갈라지지 않도록 domain 상수를 쓴다.
			ReleaseName: domain.PostgresServiceName,
			ChartName:   "postgresql",
			RepoURL:     "https://charts.bitnami.com/bitnami",
			// 버전이 비어 있어 매번 최신 차트를 가져갔고, 차트 기본 이미지도
			// bitnami/postgresql:latest 라 설치 시점마다 PostgreSQL 이 달라졌다.
			//
			// bitnami 가 2025-08 에 버전 태그를 bitnamilegacy/* 로 옮기면서
			// bitnami/postgresql 에는 latest 만 남았다. 재현 가능한 설치를 위해
			// 차트와 이미지를 함께 고정한다 — bitnamilegacy 의 최신이 17.6.0 이므로
			// appVersion 이 일치하는 차트 16.7.27 과 짝을 맞춘다.
			//
			// 트레이드오프: bitnamilegacy 는 동결된 저장소라 보안 패치가 없다.
			// 그럼에도 latest 를 그대로 두는 것보다 낫다 — 무엇이 깔릴지 모르는
			// 상태에서는 취약점이 있는지조차 알 수 없다.
			Version: "16.7.27",
			Values:  DefaultValues("installing_postgresql"),
			Wait:    false,
		}, true
	case "installing_minio":
		return ChartSpec{
			ReleaseName: domain.MinIOServiceName,
			ChartName:   "minio",
			RepoURL:     "https://charts.min.io/",
			Version:     domain.MinIOChartVersion,
			Values:      DefaultValues("installing_minio"),
			Wait:        false,
		}, true
	case "installing_gitlab":
		return ChartSpec{
			ChartName: "gitlab",
			RepoURL:   "https://charts.gitlab.io/",
			Version:   domain.GitLabChartVersion,
			Values:    DefaultValues("installing_gitlab"),
			Wait:      false,
		}, true
	case "installing_gitea":
		// 릴리스명이 곧 파드 접두사이자 Service 이름의 뿌리다(gitea-http).
		// 워크로드 조회(domain.InstalledToolWorkloads)와 CI/CD 모듈의 in-cluster
		// 주소가 모두 이 이름을 보므로 domain 상수를 쓴다.
		//
		// 저장소는 dl.gitea.io 와 dl.gitea.com 이 같은 내용을 서빙하지만 공식
		// 도메인인 gitea.com 쪽을 쓴다.
		return ChartSpec{
			ReleaseName: domain.GiteaReleaseName,
			ChartName:   "gitea",
			RepoURL:     "https://dl.gitea.com/charts",
			Version:     domain.GiteaChartVersion,
			Values:      DefaultValues("installing_gitea"),
			Wait:        false,
		}, true
	case "installing_jenkins":
		// Jenkins 는 GitLab CI 와 달리 실행기를 별도 차트로 세우지 않는다 —
		// kubernetes 플러그인이 빌드마다 agent 파드를 직접 띄운다.
		return ChartSpec{
			ReleaseName: domain.JenkinsReleaseName,
			ChartName:   "jenkins",
			RepoURL:     "https://charts.jenkins.io",
			Version:     domain.JenkinsChartVersion,
			Values:      DefaultValues("installing_jenkins"),
			Wait:        false,
		}, true
	case "installing_harbor":
		// 릴리스명이 곧 진입 Service 이름이 되므로 domain 상수를 쓴다.
		return ChartSpec{
			ReleaseName: domain.HarborReleaseName,
			ChartName:   "harbor",
			RepoURL:     "https://helm.goharbor.io",
			// 호환성 매트릭스가 선언한 버전과 같아야 한다. 화면은 매트릭스 값을
			// 보여주고 설치는 이 값을 쓰므로, 갈라지면 안내와 실제가 어긋난다.
			// (고정: TestChartVersionsMatchCompatibilityMatrix)
			Version: domain.HarborChartVersion,
			Values:  DefaultValues("installing_harbor"),
			Wait:    false,
		}, true
	case "installing_nexus":
		// sonatype/nexus-repository-manager 는 상위 차트가 deprecated 로 표시돼
		// 있으나, 대체재인 nxrm-ha 는 PostgreSQL 기반 HA 구성이라 Pro 라이선스가
		// 필요하다. OSS 단일 인스턴스 경로는 아직 이 차트뿐이다.
		return ChartSpec{
			ReleaseName: domain.NexusReleaseName,
			ChartName:   "nexus-repository-manager",
			RepoURL:     "https://sonatype.github.io/helm3-charts/",
			Version:     domain.NexusChartVersion,
			Values:      DefaultValues("installing_nexus"),
			Wait:        false,
		}, true
	case "installing_openbao":
		// 공식 OpenBao 차트를 사용한다. 차트가 StatefulSet·PVC·ServiceAccount 와
		// system:auth-delegator 바인딩까지 제공하므로 자체 매니페스트가 필요 없다.
		// Values 는 선택된 StorageClass 를 반영해야 하므로 valuesForStep 에서 채운다.
		return ChartSpec{
			ReleaseName: "openbao",
			ChartName:   "openbao",
			RepoURL:     "https://openbao.github.io/openbao-helm",
			Version:     "0.28.4",
			Wait:        false,
		}, true
	case "installing_external_secrets":
		// 공식 ESO 차트. OpenBao → K8s Secret 복제를 표준 오퍼레이터에 위임한다.
		return ChartSpec{
			ReleaseName: "external-secrets",
			ChartName:   ESOChartName,
			RepoURL:     ESOChartRepo,
			Version:     ESOChartVersion,
			Values:      externalSecretsValues(),
			Wait:        false,
		}, true
	case "installing_argocd":
		return ChartSpec{
			ChartName: "argo-cd",
			RepoURL:   "https://argoproj.github.io/argo-helm",
			Version:   domain.ArgoCDChartVersion,
			Values:    DefaultValues("installing_argocd"),
			Wait:      false,
		}, true
	case stepInstallingRunner:
		return ChartSpec{
			ChartName: "gitlab-runner",
			RepoURL:   "https://charts.gitlab.io/",
			Version:   "0.72.0",
			Values:    DefaultValues(stepInstallingRunner),
			Wait:      false,
		}, true
	case stepInstallingPrometheusCRDs:
		// 버전은 kube-prometheus-stack 이 쓰는 operator 와 맞춘다. 어긋나면
		// 오퍼레이터가 모르는 스키마의 CRD 가 깔린다.
		return ChartSpec{
			ReleaseName: "prometheus-operator-crds",
			ChartName:   "prometheus-operator-crds",
			RepoURL:     "https://prometheus-community.github.io/helm-charts",
			Version:     domain.PrometheusOperatorCRDsChartVersion,
			Wait:        false,
		}, true
	case "installing_prometheus":
		return ChartSpec{
			ChartName: "kube-prometheus-stack",
			RepoURL:   "https://prometheus-community.github.io/helm-charts",
			Version:   domain.PrometheusChartVersion,
			Values:    DefaultValues("installing_prometheus"),
			Wait:      false,
		}, true
	case "installing_grafana":
		return ChartSpec{
			ChartName: "grafana",
			RepoURL:   "https://grafana.github.io/helm-charts",
			Version:   domain.GrafanaChartVersion,
			Values:    DefaultValues("installing_grafana"),
			Wait:      false,
		}, true
	case "installing_logging":
		return ChartSpec{
			ChartName: "loki",
			RepoURL:   "https://grafana.github.io/helm-charts",
			Version:   "2.10.3",
			Values:    DefaultValues("installing_logging"),
			Wait:      false,
		}, true
	case "installing_log_search":
		return ChartSpec{
			ChartName: "opensearch",
			RepoURL:   "https://opensearch-project.github.io/helm-charts",
			Version:   "2.22.0",
			Values:    DefaultValues("installing_logging_opensearch"),
			Wait:      false,
		}, true
	case "installing_opentelemetry":
		return ChartSpec{
			ChartName: "opentelemetry-collector",
			RepoURL:   "https://open-telemetry.github.io/opentelemetry-helm-charts",
			Version:   domain.OTelCollectorChartVersion,
			Values:    DefaultValues("installing_opentelemetry"),
			Wait:      false,
		}, true
	case stepInstallingOTelAgent:
		// 게이트웨이와 같은 차트지만 역할이 달라 릴리스를 따로 둔다.
		return ChartSpec{
			ReleaseName: domain.OTelAgentReleaseName,
			ChartName:   "opentelemetry-collector",
			RepoURL:     "https://open-telemetry.github.io/opentelemetry-helm-charts",
			Version:     domain.OTelCollectorChartVersion,
			Wait:        false,
		}, true
	case stepInstallingOTelCollector:
		// 릴리스명을 차트 이름과 다르게 둔다. 추적 계층 단계가 같은 차트를
		// 설치할 수 있어 이름이 겹치면 Helm 이 소유권 충돌로 거부한다.
		return ChartSpec{
			ReleaseName: domain.OTelCollectorReleaseName,
			ChartName:   "opentelemetry-collector",
			RepoURL:     "https://open-telemetry.github.io/opentelemetry-helm-charts",
			Version:     domain.OTelCollectorChartVersion,
			Values:      DefaultValues(stepInstallingOTelCollector),
			Wait:        false,
		}, true
	case "installing_gateway":
		return ChartSpec{
			ReleaseName: "eg",
			ChartName:   "oci://docker.io/envoyproxy/gateway-helm",
			Version:     "1.4.3",
			Wait:        false,
		}, true
	default:
		return ChartSpec{}, false
	}
}
