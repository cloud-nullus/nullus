package helm

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func (o *Orchestrator) resourceDefaultValuesForStep(step string, cfg *domain.StackConfig) map[string]any {
	resourceKey := o.resourceDefaultKeyForStep(step, cfg)
	if resourceKey == "" {
		return map[string]any{}
	}

	item := o.loadResourceDefault(resourceKey)
	if item == nil {
		return map[string]any{}
	}

	resources := toK8sResourceValues(item)
	if len(resources) == 0 {
		return map[string]any{}
	}

	scaled := func(ratio float64) map[string]any {
		return toK8sResourceValues(scaleResourceDefault(item, ratio))
	}
	webScaled := func() map[string]any {
		v := scaleResourceDefault(item, 0.12)
		if v == nil {
			return map[string]any{}
		}
		if v.CPURequest < 0.4 {
			v.CPURequest = 0.4
		}
		if v.CPURequest > 1.0 {
			v.CPURequest = 1.0
		}
		if v.CPULimit < 0.8 {
			v.CPULimit = 0.8
		}
		if v.CPULimit > 2.0 {
			v.CPULimit = 2.0
		}
		if v.MemoryRequestGi < 1 {
			v.MemoryRequestGi = 1
		}
		if v.MemoryRequestGi > 2 {
			v.MemoryRequestGi = 2
		}
		if v.MemoryLimitGi < 2 {
			v.MemoryLimitGi = 2
		}
		if v.MemoryLimitGi > 4 {
			v.MemoryLimitGi = 4
		}
		return toK8sResourceValues(v)
	}
	sidekiqScaled := func() map[string]any {
		v := scaleResourceDefault(item, 0.10)
		if v == nil {
			return map[string]any{}
		}
		if v.CPURequest < 0.35 {
			v.CPURequest = 0.35
		}
		if v.CPURequest > 0.8 {
			v.CPURequest = 0.8
		}
		if v.CPULimit < 0.7 {
			v.CPULimit = 0.7
		}
		if v.CPULimit > 1.6 {
			v.CPULimit = 1.6
		}
		if v.MemoryRequestGi < 1 {
			v.MemoryRequestGi = 1
		}
		if v.MemoryRequestGi > 1.5 {
			v.MemoryRequestGi = 1.5
		}
		if v.MemoryLimitGi < 2 {
			v.MemoryLimitGi = 2
		}
		if v.MemoryLimitGi > 3 {
			v.MemoryLimitGi = 3
		}
		return toK8sResourceValues(v)
	}
	redisMasterScaled := func() map[string]any {
		v := scaleResourceDefault(item, 0.06)
		if v == nil {
			return map[string]any{}
		}
		if v.CPURequest < 0.2 {
			v.CPURequest = 0.2
		}
		if v.CPURequest > 0.5 {
			v.CPURequest = 0.5
		}
		if v.CPULimit < 0.4 {
			v.CPULimit = 0.4
		}
		if v.CPULimit > 1.0 {
			v.CPULimit = 1.0
		}
		if v.MemoryRequestGi < 0.5 {
			v.MemoryRequestGi = 0.5
		}
		if v.MemoryRequestGi > 1.0 {
			v.MemoryRequestGi = 1.0
		}
		if v.MemoryLimitGi < 1.0 {
			v.MemoryLimitGi = 1.0
		}
		if v.MemoryLimitGi > 2.0 {
			v.MemoryLimitGi = 2.0
		}
		return toK8sResourceValues(v)
	}
	toolboxScaled := func() map[string]any {
		v := scaleResourceDefault(item, 0.05)
		if v == nil {
			return map[string]any{}
		}
		if v.CPURequest < 0.25 {
			v.CPURequest = 0.25
		}
		if v.CPURequest > 0.5 {
			v.CPURequest = 0.5
		}
		if v.CPULimit < 0.50 {
			v.CPULimit = 0.50
		}
		if v.CPULimit > 1.0 {
			v.CPULimit = 1.0
		}
		if v.MemoryRequestGi < 1 {
			v.MemoryRequestGi = 1
		}
		if v.MemoryRequestGi > 1.5 {
			v.MemoryRequestGi = 1.5
		}
		if v.MemoryLimitGi < 2 {
			v.MemoryLimitGi = 2
		}
		if v.MemoryLimitGi > 3 {
			v.MemoryLimitGi = 3
		}
		return toK8sResourceValues(v)
	}

	switch step {
	case stepInstallingCertManager:
		return map[string]any{
			"resources": resources,
			"webhook": map[string]any{
				"resources": resources,
			},
			"cainjector": map[string]any{
				"resources": resources,
			},
		}
	case "installing_minio":
		return map[string]any{
			"resources": resources,
		}
	case "installing_gitlab":
		return map[string]any{
			"gitlab": map[string]any{
				"webservice":      map[string]any{"resources": webScaled()},
				"sidekiq":         map[string]any{"resources": sidekiqScaled()},
				"toolbox":         map[string]any{"resources": toolboxScaled()},
				"gitaly":          map[string]any{"resources": scaled(0.20)},
				"kas":             map[string]any{"resources": scaled(0.12)},
				"gitlab-exporter": map[string]any{"resources": scaled(0.05)},
			},
			"registry": map[string]any{
				"resources": scaled(0.12),
			},
			"redis": map[string]any{
				"master": map[string]any{"resources": redisMasterScaled()},
			},
			"prometheus": map[string]any{
				"server": map[string]any{"resources": scaled(0.08)},
			},
		}
	case "installing_argocd":
		return map[string]any{
			"controller":     map[string]any{"resources": scaled(0.24)},
			"repoServer":     map[string]any{"resources": scaled(0.20)},
			"server":         map[string]any{"resources": scaled(0.20)},
			"redis":          map[string]any{"resources": scaled(0.12)},
			"dex":            map[string]any{"resources": scaled(0.10)},
			"applicationSet": map[string]any{"resources": scaled(0.07)},
			"notifications":  map[string]any{"resources": scaled(0.07)},
		}
	case stepInstallingRunner:
		return map[string]any{
			"resources": resources,
		}
	case "installing_prometheus":
		return map[string]any{
			"prometheus": map[string]any{
				"prometheusSpec": map[string]any{"resources": resources},
			},
			"alertmanager": map[string]any{
				"alertmanagerSpec": map[string]any{"resources": resources},
			},
			"kube-state-metrics":       map[string]any{"resources": resources},
			"prometheusOperator":       map[string]any{"resources": resources},
			"prometheus-node-exporter": map[string]any{"resources": resources},
		}
	case "installing_grafana":
		return map[string]any{
			"resources": resources,
		}
	case "installing_logging":
		return map[string]any{
			"resources":    resources,
			"loki":         map[string]any{"resources": resources},
			"singleBinary": map[string]any{"resources": resources},
			"read":         map[string]any{"resources": resources},
			"write":        map[string]any{"resources": resources},
			"backend":      map[string]any{"resources": resources},
			"promtail":     map[string]any{"resources": resources},
		}
	case "installing_log_search":
		return map[string]any{
			"resources": resources,
			"master":    map[string]any{"resources": resources},
		}
	case "installing_opentelemetry":
		traceName := ""
		if cfg != nil {
			traceName = strings.TrimSpace(strings.ToLower(cfg.Logging.TraceLayer.Name))
		}
		switch traceName {
		case "tempo":
			return map[string]any{
				"resources":  resources,
				"tempo":      map[string]any{"resources": resources},
				"tempoQuery": map[string]any{"resources": resources},
			}
		case "jaeger":
			return map[string]any{
				"resources": resources,
				"allInOne":  map[string]any{"resources": resources},
				"agent":     map[string]any{"resources": resources},
				"collector": map[string]any{"resources": resources},
				"query":     map[string]any{"resources": resources},
			}
		default:
			return map[string]any{
				"resources": resources,
			}
		}
	default:
		return map[string]any{}
	}
}

func (o *Orchestrator) resourceDefaultKeyForStep(step string, cfg *domain.StackConfig) string {
	switch step {
	case stepInstallingCertManager:
		return "cert-manager"
	case "installing_minio":
		return "minio"
	case "installing_gitlab":
		return "gitlab-ce"
	case "installing_argocd":
		return "argocd"
	case stepInstallingRunner:
		return "gitlab-runner"
	case "installing_prometheus":
		return "prometheus"
	case "installing_grafana":
		return "grafana"
	case "installing_logging":
		return "loki"
	case "installing_log_search":
		if cfg != nil {
			switch strings.TrimSpace(strings.ToLower(cfg.Logging.Search.Name)) {
			case "elasticsearch":
				return "elasticsearch"
			case "opensearch", "":
				return "opensearch"
			}
		}
		return "opensearch"
	case "installing_opentelemetry":
		if cfg != nil {
			switch strings.TrimSpace(strings.ToLower(cfg.Logging.TraceLayer.Name)) {
			case "tempo":
				return "tempo"
			case "jaeger":
				return "jaeger"
			}
		}
		return "opentelemetry"
	default:
		return ""
	}
}

func (o *Orchestrator) loadResourceDefault(key string) *domain.ResourceDefault {
	if strings.TrimSpace(key) == "" || o.resourceDefaultRepo == nil {
		return nil
	}

	o.mu.Lock()
	loaded := o.defaultsLoaded
	o.mu.Unlock()

	if !loaded {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		items, err := o.resourceDefaultRepo.List(ctx)
		if err != nil {
			slog.Warn("resource default load failed", "error", err)
		} else {
			loadedMap := make(map[string]*domain.ResourceDefault, len(items))
			for _, item := range items {
				if item == nil {
					continue
				}
				loadedMap[strings.ToLower(strings.TrimSpace(item.ToolKey))] = item
			}
			o.mu.Lock()
			o.resourceDefaults = loadedMap
			o.defaultsLoaded = true
			o.mu.Unlock()
		}
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if o.resourceDefaults == nil {
		return nil
	}
	return o.resourceDefaults[strings.ToLower(strings.TrimSpace(key))]
}

func toK8sResourceValues(item *domain.ResourceDefault) map[string]any {
	if item == nil {
		return map[string]any{}
	}

	requests := map[string]any{}
	limits := map[string]any{}

	if v := cpuQuantity(item.CPURequest); v != "" {
		requests["cpu"] = v
	}
	if v := cpuQuantity(item.CPULimit); v != "" {
		limits["cpu"] = v
	}
	if v := memoryGiQuantity(item.MemoryRequestGi); v != "" {
		requests["memory"] = v
	}
	if v := memoryGiQuantity(item.MemoryLimitGi); v != "" {
		limits["memory"] = v
	}

	out := map[string]any{}
	if len(requests) > 0 {
		out["requests"] = requests
	}
	if len(limits) > 0 {
		out["limits"] = limits
	}
	return out
}

func scaleResourceDefault(item *domain.ResourceDefault, ratio float64) *domain.ResourceDefault {
	if item == nil {
		return nil
	}
	if ratio <= 0 {
		ratio = 1
	}
	round2 := func(v float64) float64 {
		return math.Round(v*100) / 100
	}
	scaled := *item
	scaled.CPURequest = round2(math.Max(0.05, item.CPURequest*ratio))
	scaled.CPULimit = round2(math.Max(0.10, item.CPULimit*ratio))
	scaled.MemoryRequestGi = round2(math.Max(0.08, item.MemoryRequestGi*ratio))
	scaled.MemoryLimitGi = round2(math.Max(0.16, item.MemoryLimitGi*ratio))
	scaled.StorageRequestGi = round2(math.Max(0, item.StorageRequestGi*ratio))
	scaled.StorageLimitGi = round2(math.Max(0, item.StorageLimitGi*ratio))
	return &scaled
}

func cpuQuantity(cores float64) string {
	if cores <= 0 {
		return ""
	}
	milli := int64(math.Round(cores * 1000))
	if milli <= 0 {
		return ""
	}
	if milli%1000 == 0 {
		return fmt.Sprintf("%d", milli/1000)
	}
	return fmt.Sprintf("%dm", milli)
}

func memoryGiQuantity(gi float64) string {
	if gi <= 0 {
		return ""
	}
	if math.Mod(gi, 1.0) == 0 {
		return fmt.Sprintf("%dGi", int64(gi))
	}
	return fmt.Sprintf("%gGi", gi)
}
