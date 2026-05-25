package helm

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func (o *Orchestrator) valuesForStep(step string, spec ChartSpec) map[string]any {
	base := deepCopyMap(spec.Values)

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()

	base = mergeMaps(base, o.resourceDefaultValuesForStep(step, cfg))

	if cfg == nil || len(cfg.YAMLOverrides) == 0 {
		if step == "installing_minio" {
			namespace := strings.TrimSpace(o.namespace)
			if namespace == "" {
				namespace = "nullus"
			}
			base = mergeMaps(base, map[string]any{"namespace": namespace})
		}
		if step == "installing_postgresql" {
			base = mergeMaps(base, o.sharedPostgresValues(nil))
		}
		if step == "installing_gitlab" {
			base = mergeMaps(base, o.gitlabExternalSharedServiceValues(nil))
		}
		if step == stepInstallingRunner {
			namespace := strings.TrimSpace(o.namespace)
			if namespace == "" {
				namespace = "nullus"
			}
			base = mergeMaps(base, map[string]any{
				"gitlabUrl": fmt.Sprintf("http://gitlab-webservice-default.%s.svc:8181", namespace),
			})
		}
		if cfg != nil && step == "installing_gitlab" && strings.TrimSpace(cfg.AccessDomain) != "" {
			base = mergeMaps(base, map[string]any{
				"global": map[string]any{
					"hosts": map[string]any{
						"domain": cfg.AccessDomain,
					},
				},
			})
		}
		return base
	}

	if step == "installing_gitlab" && strings.TrimSpace(cfg.AccessDomain) != "" {
		base = mergeMaps(base, map[string]any{
			"global": map[string]any{
				"hosts": map[string]any{
					"domain": cfg.AccessDomain,
				},
			},
		})
	}

	if step == "installing_postgresql" {
		base = mergeMaps(base, o.sharedPostgresValues(cfg))
	}

	if step == "installing_minio" {
		namespace := strings.TrimSpace(o.namespace)
		if namespace == "" {
			namespace = "nullus"
		}
		base = mergeMaps(base, map[string]any{"namespace": namespace})
	}

	if step == "installing_gitlab" {
		base = mergeMaps(base, o.gitlabExternalSharedServiceValues(cfg))
	}

	if step == stepInstallingRunner {
		namespace := strings.TrimSpace(o.namespace)
		if namespace == "" {
			namespace = "nullus"
		}
		base = mergeMaps(base, map[string]any{
			"gitlabUrl": fmt.Sprintf("http://gitlab-webservice-default.%s.svc:8181", namespace),
		})
	}

	if step == "installing_gateway" {
		return base
	}

	keys := []string{step, o.releaseNameForSpec(spec), spec.ChartName, strings.TrimPrefix(step, "installing_")}
	for _, key := range keys {
		raw, ok := cfg.YAMLOverrides[key]
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}

		override, err := decodeValuesOverride(raw)
		if err != nil {
			slog.Warn("invalid yaml override skipped", "step", step, "key", key, "error", err)
			continue
		}
		override = normalizeLegacyResourceOverrideForStep(step, override)
		base = mergeMaps(base, override)
		break
	}

	return base
}

func (o *Orchestrator) resolveChartSpecForStep(step string, spec ChartSpec) ChartSpec {
	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil {
		return spec
	}

	if step == "installing_log_search" {
		switch strings.TrimSpace(cfg.Logging.Search.Name) {
		case "opensearch":
			spec.ChartName = "opensearch"
			spec.RepoURL = "https://opensearch-project.github.io/helm-charts"
			spec.Version = "2.22.0"
			spec.Values = DefaultValues("installing_logging_opensearch")
		case "elasticsearch":
			spec.ChartName = "elasticsearch"
			spec.RepoURL = "https://helm.elastic.co"
			spec.Version = "8.5.1"
			spec.Values = DefaultValues("installing_logging_elasticsearch")
		default:
			spec.ChartName = "opensearch"
			spec.RepoURL = "https://opensearch-project.github.io/helm-charts"
			spec.Version = "2.22.0"
			spec.Values = DefaultValues("installing_logging_opensearch")
		}
	}

	if step == "installing_opentelemetry" {
		switch strings.TrimSpace(cfg.Logging.TraceLayer.Name) {
		case "tempo":
			spec.ChartName = "tempo"
			spec.RepoURL = "https://grafana.github.io/helm-charts"
			spec.Version = "1.18.1"
			spec.Values = DefaultValues("installing_tempo")
		case "jaeger":
			spec.ChartName = "jaeger"
			spec.RepoURL = "https://jaegertracing.github.io/helm-charts"
			spec.Version = "3.3.0"
			spec.Values = DefaultValues("installing_jaeger")
		default:
			spec.ChartName = "opentelemetry-collector"
			spec.RepoURL = "https://open-telemetry.github.io/opentelemetry-helm-charts"
			spec.Version = "0.75.0"
			spec.Values = DefaultValues("installing_opentelemetry")
		}
	}

	return spec
}

func (o *Orchestrator) releaseNameForSpec(spec ChartSpec) string {
	if strings.TrimSpace(spec.ReleaseName) != "" {
		return spec.ReleaseName
	}
	return spec.ChartName
}

func (o *Orchestrator) sharedPostgresValues(cfg *domain.StackConfig) map[string]any {
	storageGi := 20.0
	if cfg != nil && cfg.Storage != nil && cfg.Storage.Database.Size > 0 {
		storageGi = cfg.Storage.Database.Size
	}

	return map[string]any{
		"auth": map[string]any{
			"username":         "gitlab",
			"password":         "nullus-gitlab-password",         // #nosec G101 -- default Helm value, expected to be overridden by operator
			"database":         "gitlabhq_production",
			"postgresPassword": "nullus-postgres-admin",           // #nosec G101 -- default Helm value, expected to be overridden by operator
		},
		"primary": map[string]any{
			"persistence": map[string]any{
				"enabled": true,
				"size":    fmt.Sprintf("%gGi", storageGi),
			},
		},
	}
}

func (o *Orchestrator) gitlabExternalSharedServiceValues(_ *domain.StackConfig) map[string]any {
	namespace := strings.TrimSpace(o.namespace)
	if namespace == "" {
		namespace = "nullus"
	}

	return map[string]any{
		"postgresql": map[string]any{
			"install": false,
		},
		"global": map[string]any{
			"minio": map[string]any{
				"enabled": false,
			},
			"psql": map[string]any{
				"host":     fmt.Sprintf("nullus-postgresql.%s.svc.cluster.local", namespace),
				"port":     5432,
				"database": "gitlabhq_production",
				"username": "gitlab",
				"password": map[string]any{
					"useSecret": true,
					"secret":    "nullus-postgresql",
					"key":       "password",
				},
			},
			"appConfig": map[string]any{
				"object_store": map[string]any{
					"enabled": true,
					"connection": map[string]any{
						"secret": "nullus-object-storage",
						"key":    "connection",
					},
				},
			},
		},
		"gitlab": map[string]any{
			"toolbox": map[string]any{
				"backups": map[string]any{
					"objectStorage": map[string]any{
						"config": map[string]any{
							"secret": "nullus-object-storage",
							"key":    "config",
						},
					},
				},
			},
		},
	}
}

func (o *Orchestrator) sharedObjectStorageSecretManifest(namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}

	endpoint := fmt.Sprintf("http://nullus-minio.%s.svc.cluster.local:9000", namespace)
	connection := fmt.Sprintf("provider: AWS\nregion: us-east-1\naws_access_key_id: nullus-admin\naws_secret_access_key: nullus-minio-secret\nendpoint: %s\npath_style: true\n", endpoint) // #nosec G101 -- default bootstrap credential, matches Helm default value

	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: nullus-object-storage
  namespace: %s
type: Opaque
stringData:
  connection: |
%s
  config: |
%s
`, namespace, indentYAML(connection, 4), indentYAML(connection, 4))
}

func indentYAML(value string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	trimmed := strings.TrimRight(value, "\n")
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}

func deepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(src)
	if err != nil {
		return map[string]any{}
	}
	var copied map[string]any
	if err := json.Unmarshal(b, &copied); err != nil {
		return map[string]any{}
	}
	return copied
}

func decodeValuesOverride(raw string) (map[string]any, error) {
	var parsed any
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	b, err := json.Marshal(parsed)
	if err != nil {
		return nil, fmt.Errorf("normalize yaml: %w", err)
	}

	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("expected mapping yaml for helm values: %w", err)
	}

	if _, hasAPIVersion := out["apiVersion"]; hasAPIVersion {
		if _, hasKind := out["kind"]; hasKind {
			if converted, ok := resourceOverrideFromManifest(out); ok {
				return converted, nil
			}
			return nil, fmt.Errorf("manifest yaml is not supported for helm values override")
		}
	}

	return out, nil
}

func mergeMaps(base, override map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for key, value := range override {
		subOverride, ok := value.(map[string]any)
		if !ok {
			base[key] = value
			continue
		}

		subBase, _ := base[key].(map[string]any)
		base[key] = mergeMaps(subBase, subOverride)
	}
	return base
}

func normalizeLegacyResourceOverrideForStep(step string, override map[string]any) map[string]any {
	if len(override) == 0 {
		return override
	}
	resources, ok := override["resources"].(map[string]any)
	if (!ok || len(resources) == 0) && step == "installing_logging" {
		resources = firstResourcesFromNestedLoggingOverride(override)
		if len(resources) > 0 {
			override = mergeMaps(map[string]any{"resources": resources}, override)
			ok = true
		}
	}
	if !ok || len(resources) == 0 {
		return override
	}

	switch step {
	case "installing_gitlab":
		return mergeMaps(map[string]any{
			"gitlab": map[string]any{
				"webservice":      map[string]any{"resources": resources},
				"sidekiq":         map[string]any{"resources": resources},
				"toolbox":         map[string]any{"resources": resources},
				"gitaly":          map[string]any{"resources": resources},
				"kas":             map[string]any{"resources": resources},
				"gitlab-exporter": map[string]any{"resources": resources},
			},
			"registry": map[string]any{"resources": resources},
			"redis":    map[string]any{"master": map[string]any{"resources": resources}},
			"prometheus": map[string]any{
				"server": map[string]any{"resources": resources},
			},
		}, override)
	case "installing_argocd":
		return mergeMaps(map[string]any{
			"controller":     map[string]any{"resources": resources},
			"repoServer":     map[string]any{"resources": resources},
			"server":         map[string]any{"resources": resources},
			"redis":          map[string]any{"resources": resources},
			"dex":            map[string]any{"resources": resources},
			"applicationSet": map[string]any{"resources": resources},
			"notifications":  map[string]any{"resources": resources},
		}, override)
	case "installing_prometheus":
		return mergeMaps(map[string]any{
			"prometheus":               map[string]any{"prometheusSpec": map[string]any{"resources": resources}},
			"alertmanager":             map[string]any{"alertmanagerSpec": map[string]any{"resources": resources}},
			"kube-state-metrics":       map[string]any{"resources": resources},
			"prometheusOperator":       map[string]any{"resources": resources},
			"prometheus-node-exporter": map[string]any{"resources": resources},
		}, override)
	case "installing_logging":
		return mergeMaps(map[string]any{
			"resources":    resources,
			"loki":         map[string]any{"resources": resources},
			"singleBinary": map[string]any{"resources": resources},
			"read":         map[string]any{"resources": resources},
			"write":        map[string]any{"resources": resources},
			"backend":      map[string]any{"resources": resources},
			"promtail":     map[string]any{"resources": resources},
		}, override)
	case "installing_log_search":
		return mergeMaps(map[string]any{
			"master": map[string]any{"resources": resources},
		}, override)
	default:
		return override
	}
}

func firstResourcesFromNestedLoggingOverride(override map[string]any) map[string]any {
	candidates := []string{"loki", "singleBinary", "read", "write", "backend", "promtail"}
	for _, key := range candidates {
		node, ok := override[key].(map[string]any)
		if !ok {
			continue
		}
		resources, ok := node["resources"].(map[string]any)
		if !ok || len(resources) == 0 {
			continue
		}
		return resources
	}
	return map[string]any{}
}

func resourceOverrideFromManifest(doc map[string]any) (map[string]any, bool) {
	if len(doc) == 0 {
		return nil, false
	}
	spec, ok := doc["spec"].(map[string]any)
	if !ok {
		return nil, false
	}

	if template, ok := spec["template"].(map[string]any); ok {
		if templateSpec, ok := template["spec"].(map[string]any); ok {
			spec = templateSpec
		}
	}

	containers, ok := spec["containers"].([]any)
	if !ok || len(containers) == 0 {
		return nil, false
	}

	for _, c := range containers {
		containerMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		resources, ok := containerMap["resources"].(map[string]any)
		if !ok || len(resources) == 0 {
			continue
		}
		return map[string]any{"resources": resources}, true
	}

	return nil, false
}
