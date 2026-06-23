package helm

import (
	"context"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

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
			Values:    DefaultValues("installing_metrics_server"),
			Wait:      false,
		}, true
	case "installing_postgresql":
		return ChartSpec{
			ReleaseName: "nullus-postgresql",
			ChartName:   "postgresql",
			RepoURL:     "https://charts.bitnami.com/bitnami",
			Values:      DefaultValues("installing_postgresql"),
			Wait:        false,
		}, true
	case "installing_minio":
		return ChartSpec{
			ReleaseName: "nullus-minio",
			ChartName:   "minio",
			RepoURL:     "https://charts.min.io/",
			Version:     "5.4.0",
			Values:      DefaultValues("installing_minio"),
			Wait:        false,
		}, true
	case "installing_gitlab":
		return ChartSpec{
			ChartName: "gitlab",
			RepoURL:   "https://charts.gitlab.io/",
			Version:   "8.7.2",
			Values:    DefaultValues("installing_gitlab"),
			Wait:      false,
		}, true
	case "installing_openbao":
		return ChartSpec{
			ReleaseName: "openbao",
			Values:      DefaultValues("installing_openbao"),
			Wait:        false,
		}, true
	case "installing_argocd":
		return ChartSpec{
			ChartName: "argo-cd",
			RepoURL:   "https://argoproj.github.io/argo-helm",
			Version:   "7.7.16",
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
	case "installing_prometheus":
		return ChartSpec{
			ChartName: "kube-prometheus-stack",
			RepoURL:   "https://prometheus-community.github.io/helm-charts",
			Version:   "69.3.0",
			Values:    DefaultValues("installing_prometheus"),
			Wait:      false,
		}, true
	case "installing_grafana":
		return ChartSpec{
			ChartName: "grafana",
			RepoURL:   "https://grafana.github.io/helm-charts",
			Version:   "8.9.0",
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
			Version:   "0.75.0",
			Values:    DefaultValues("installing_opentelemetry"),
			Wait:      false,
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
