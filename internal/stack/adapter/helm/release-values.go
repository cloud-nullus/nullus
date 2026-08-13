package helm

import (
	"context"
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 배포가 끝난 릴리스의 values 를 읽고 다시 적용하는 경로다.
//
// 차트는 배포된 버전에 고정한다 — 설정 한 줄 바꾸려다 차트 버전이 함께
// 올라가는 사고를 막는다. 가능하면 릴리스에 저장된 차트를 그대로 쓰고,
// 그것만으로 렌더되지 않을 때만 같은 버전을 다시 받아 온다(chartForRelease).

// releaseStepNames 는 릴리스 이름 → 설치 단계 역매핑이다.
//
// 오케스트레이터가 단계별로 어떤 릴리스를 만드는지는 chartSpecForStep 이
// 알고 있지만, 로깅/트레이스처럼 설정에 따라 차트가 갈리는 단계는 기본값
// 하나만으로는 되짚을 수 없다. 변형까지 모두 적어 둔다.
var releaseStepNames = map[string]string{
	"cert-manager":             stepInstallingCertManager,
	"metrics-server":           "installing_metrics_server",
	"openbao":                  "installing_openbao",
	"external-secrets":         "installing_external_secrets",
	domain.PostgresServiceName: "installing_postgresql",
	domain.MinIOServiceName:    "installing_minio",
	"gitlab":                   "installing_gitlab",
	"gitlab-runner":            stepInstallingRunner,
	domain.HarborReleaseName:   "installing_harbor",
	domain.NexusReleaseName:    "installing_nexus",
	"argo-cd":                  "installing_argocd",
	"kube-prometheus-stack":    "installing_prometheus",
	"grafana":                  "installing_grafana",
	"loki":                     "installing_logging",
	"eg":                       "installing_gateway",
	// installing_log_search 변형
	"opensearch":    "installing_log_search",
	"elasticsearch": "installing_log_search",
	// installing_opentelemetry 변형
	"opentelemetry-collector": "installing_opentelemetry",
	"tempo":                   "installing_opentelemetry",
	"jaeger":                  "installing_opentelemetry",
	// 수집기는 추적 계층과 별개 단계다. 여기가 비면 values 편집을 어느 오버라이드
	// 키로 저장할지 알 수 없어 다음 재배포에서 편집이 조용히 사라진다.
	domain.OTelCollectorReleaseName: stepInstallingOTelCollector,
	domain.OTelAgentReleaseName:     stepInstallingOTelAgent,
}

// StepForRelease 는 릴리스를 만든 설치 단계를 돌려준다.
//
// 이 값이 비면 편집한 values 를 어느 오버라이드 키로 저장할지 알 수 없고,
// 그러면 다음 재배포에서 사용자의 편집이 조용히 사라진다.
func StepForRelease(releaseName string) string {
	return releaseStepNames[strings.TrimSpace(releaseName)]
}

func (h *HelmInstaller) ListReleases(ctx context.Context, namespace string) ([]port.ReleaseInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg, err := h.newActionConfig(nil, namespace)
	if err != nil {
		return nil, fmt.Errorf("init action config: %w", err)
	}

	client := action.NewList(cfg)
	client.All = true
	releases, err := client.Run()
	if err != nil {
		return nil, fmt.Errorf("list releases in %s: %w", namespace, err)
	}

	out := make([]port.ReleaseInfo, 0, len(releases))
	for _, rel := range releases {
		if rel == nil {
			continue
		}
		info := port.ReleaseInfo{
			ReleaseName: rel.Name,
			StepName:    StepForRelease(rel.Name),
			Namespace:   rel.Namespace,
			Revision:    rel.Version,
		}
		if rel.Info != nil {
			info.Status = rel.Info.Status.String()
		}
		if rel.Chart != nil && rel.Chart.Metadata != nil {
			info.ChartName = rel.Chart.Metadata.Name
			info.ChartVersion = rel.Chart.Metadata.Version
			info.AppVersion = rel.Chart.Metadata.AppVersion
		}
		out = append(out, info)
	}

	return out, nil
}

func (h *HelmInstaller) GetValues(ctx context.Context, releaseName, namespace string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg, err := h.newActionConfig(nil, namespace)
	if err != nil {
		return nil, fmt.Errorf("init action config: %w", err)
	}

	client := action.NewGetValues(cfg)
	// AllValues=false. 차트 기본값까지 합치면 수천 줄이 되고, 그중 사용자가
	// 실제로 지정한 값이 무엇인지 구분할 수 없게 된다.
	client.AllValues = false
	// 실패한 리비전은 건너뛰고 실제로 배포된 것을 읽는다. 업그레이드가 실패하면
	// Helm 은 그 리비전을 최신으로 남기는데, 거기 담긴 values 는 클러스터에
	// 적용된 적이 없다. 그대로 보여 주면 사용자가 돌지도 않는 설정을 현재 값으로
	// 착각하고, 오버라이드 모드에서는 그 위에 병합까지 된다.
	if history, err := action.NewHistory(cfg).Run(releaseName); err == nil {
		client.Version = readableRevision(history)
	}
	values, err := client.Run(releaseName)
	if err != nil {
		return nil, fmt.Errorf("get values for release %s: %w", releaseName, err)
	}
	if values == nil {
		return map[string]any{}, nil
	}
	return values, nil
}

// chartForRelease 는 values 만 바꾸는 업그레이드에 쓸 차트를 고른다.
//
// 우선 릴리스에 저장된 차트를 쓴다 — 네트워크가 필요 없고 배포된 그대로다.
// 다만 Helm 은 저장 시 의존 서브차트를 함께 담지 않으므로(chart.Chart 의 Raw 는
// json:"-", dependencies 는 비공개 필드), bitnami common 같은 라이브러리에
// 기대는 차트는 저장본만으로 렌더되지 않는다. 그때만 설치 때와 같은 경로로
// 차트를 다시 찾는다. 버전은 배포된 그대로 고정하므로 설정 변경이 차트 업그레이드를
// 끌고 오지 않는다.
func (h *HelmInstaller) chartForRelease(ctx context.Context, existing *release.Release) (*chart.Chart, error) {
	if !storedChartLostDependencies(existing.Chart) {
		return existing.Chart, nil
	}

	spec, ok := defaultChartSpecForStep(StepForRelease(existing.Name))
	if !ok {
		return nil, fmt.Errorf(
			"릴리스 %s 의 차트에 의존 서브차트가 없고 다시 받아 올 저장소도 모른다 — values 만 바꾸는 업그레이드를 할 수 없다",
			existing.Name)
	}

	chartName := spec.ChartName
	version := existing.Chart.Metadata.Version

	opts := action.ChartPathOptions{RepoURL: spec.RepoURL, Version: version}
	settings := cli.New()
	chartPath, err := locateChartWithRetry(ctx, func() (string, error) {
		return opts.LocateChart(chartName, settings)
	})
	if err != nil {
		return nil, fmt.Errorf("locate chart %s %s: %w", chartName, version, err)
	}

	located, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart %s: %w", chartPath, err)
	}
	return located, nil
}

// storedChartLostDependencies 는 저장된 차트가 선언한 의존성을 잃었는지 본다.
func storedChartLostDependencies(c *chart.Chart) bool {
	if c == nil || c.Metadata == nil {
		return false
	}
	return len(c.Metadata.Dependencies) > 0 && len(c.Dependencies()) == 0
}

// readableRevision 은 "지금 실제로 돌고 있는" 리비전을 고른다.
// 0 은 helm 에서 최신을 뜻한다 — deployed 가 하나도 없는 릴리스의 폴백이다.
func readableRevision(history []*release.Release) int {
	best := 0
	for _, rel := range history {
		if rel == nil || rel.Info == nil {
			continue
		}
		if rel.Info.Status == release.StatusDeployed && rel.Version > best {
			best = rel.Version
		}
	}
	return best
}

func (h *HelmInstaller) Upgrade(ctx context.Context, req port.HelmUpgradeRequest) (*port.HelmUpgradeResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg, err := h.newActionConfig(nil, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("init action config: %w", err)
	}

	existing, err := action.NewGet(cfg).Run(req.ReleaseName)
	if err != nil {
		return nil, fmt.Errorf("get release %s: %w", req.ReleaseName, err)
	}
	if existing == nil || existing.Chart == nil {
		return nil, fmt.Errorf("release %s has no chart to upgrade", req.ReleaseName)
	}

	targetChart, err := h.chartForRelease(ctx, existing)
	if err != nil {
		return nil, err
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = req.Namespace
	upgrade.Timeout = helmOperationTimeout
	upgrade.DryRun = req.DryRun
	if req.DryRun {
		// 미리보기는 클러스터를 조회할 수 있어야 한다. 기본 드라이런은 템플릿의
		// lookup 을 빈 값으로 만드는데, bitnami 차트들은 그 lookup 으로 기존
		// 비밀번호 Secret 을 확인한 뒤 "현재 비밀번호를 넘겨라" 며 렌더를 거부한다.
		// 실제 업그레이드는 통과하는데 미리보기만 실패하는, 사용자를 오도하는
		// 결과가 된다. server 모드는 읽기만 할 뿐 클러스터를 바꾸지 않는다.
		upgrade.DryRunOption = "server"
	}
	// values 만 바꾸는 작업이므로 기다리지 않는다. 파드가 새로 뜨는 것은
	// 워크로드 탭에서 지켜보게 하고, 요청은 즉시 돌려준다 — GitLab 처럼
	// 재기동이 긴 차트에서 HTTP 요청이 30분 붙잡히면 안 된다.
	upgrade.Wait = false
	// ResetValues 로 이전 사용자 값을 버리고 넘긴 values 를 그대로 쓴다.
	// 이래야 사용자가 에디터에서 지운 키가 실제로 사라진다 — 병합으로
	// 처리하면 "지웠는데 그대로인" 상태가 되어 편집을 신뢰할 수 없다.
	upgrade.ResetValues = true

	values := req.Values
	if values == nil {
		values = map[string]any{}
	}

	rel, err := upgrade.RunWithContext(ctx, req.ReleaseName, targetChart, values)
	if err != nil {
		return nil, fmt.Errorf("upgrade release %s: %w", req.ReleaseName, err)
	}

	result := &port.HelmUpgradeResult{
		ReleaseName: rel.Name,
		Namespace:   rel.Namespace,
		Revision:    rel.Version,
		Manifest:    rel.Manifest,
	}
	if rel.Info != nil {
		result.Status = rel.Info.Status.String()
	}
	return result, nil
}
