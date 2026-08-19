package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// valuesForStep 은 단계에 실을 Helm values 를 만든다.
//
// 사용자 오버라이드를 마지막에 병합하되, 그 뒤에 "플랫폼이 소유하는 값"을 다시
// 못박는다. release values 의 live 편집이 플랫폼 계산값까지 그대로 오버라이드로
// 얼려 담기 때문에, 그대로 두면 옛 스냅샷이 현재 배선을 이긴다.
func (o *Orchestrator) valuesForStep(step string, spec ChartSpec) map[string]any {
	return o.enforcePlatformOwnedValues(step, o.mergedValuesForStep(step, spec))
}

func (o *Orchestrator) mergedValuesForStep(step string, spec ChartSpec) map[string]any {
	base := deepCopyMap(spec.Values)

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()

	// Jenkins 의 Gitea 서버 등록은 주소가 네임스페이스에 달려 있어 여기서
	// 조립한다. 등록하지 않으면 job 의 SCM 소스가 가리키는 서버를 플러그인이
	// 모르는 서버로 보고 스캔을 거부한다.
	if step == "installing_jenkins" {
		base = mergeMaps(base, jenkinsGiteaServerValues(o.namespace))
	}

	// Gitea 의 DB 호스트도 실제 네임스페이스를 알아야 한다.
	if step == "installing_gitea" {
		base = mergeMaps(base, o.giteaSharedServiceValues())
	}

	// OpenBao values 는 선택된 StorageClass 에 의존하므로 여기서 조립한다.
	if step == "installing_openbao" {
		base = mergeMaps(base, openBaoValues(o.stackStorageClass()))
	}

	// 수집기가 어느 백엔드로 내보낼지는 스택이 무엇을 함께 설치했는지에 달렸다.
	// YAML 오버라이드 유무와 무관하게 필요하므로 이른 반환 앞에서 붙인다.
	if step == stepInstallingOTelCollector {
		base = mergeMaps(base, otelCollectorValues(cfg))
	}
	if step == stepInstallingOTelAgent {
		base = mergeMaps(base, otelAgentValues(o.namespace))
	}

	// OSS 가 자기 메트릭을 내주도록 켠다. 사용자가 오버라이드로 끌 수 있어야
	// 하므로 플랫폼 소유 값이 아니라 기본값 자리에 둔다.
	if monitors := serviceMonitorValuesForStep(step, cfg); len(monitors) > 0 {
		base = mergeMaps(base, monitors)
	}

	// OIDC 블록은 스택별 client ID / accessDomain 에 의존한다.
	// 에어갭 values 파일에만 있던 설정을 코드 경로로 끌어와 일반 설치에도 적용한다.
	if oidc := o.oidcValuesForStep(step); len(oidc) > 0 {
		base = mergeMaps(base, oidc)
	}

	base = mergeMaps(base, o.resourceDefaultValuesForStep(step, cfg))

	if cfg == nil || len(cfg.YAMLOverrides) == 0 {
		if step == "installing_minio" {
			namespace := strings.TrimSpace(o.namespace)
			if namespace == "" {
				namespace = "nullus"
			}
			base = mergeMaps(base, map[string]any{"namespace": namespace})
		}
		// 여기도 cfg 를 넘긴다. nil 이면 사용자가 고른 디스크 크기가 무시되고
		// 기본값으로 깔린다 — 설치 후에 늘리기 어려운 값이라 조용히 틀리면 안 된다.
		if step == "installing_postgresql" {
			base = mergeMaps(base, o.sharedPostgresValues(cfg))
		}
		if step == "installing_gitlab" {
			base = mergeMaps(base, o.gitlabExternalSharedServiceValues(cfg))
		}
		// cfg 를 그대로 넘긴다. nil 을 넘기면 externalURL 이 클러스터 내부
		// 서비스 DNS 로 남고, 레지스트리가 그 주소를 토큰 realm 으로 광고한다.
		// 노드의 containerd 는 클러스터 DNS 를 쓰지 않으므로 그 이름을 풀지 못해
		// 배포된 앱이 ImagePullBackOff 에서 벗어나지 못한다.
		if step == "installing_harbor" {
			base = mergeMaps(base, o.harborExternalURLValues(cfg))
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

	if step == "installing_harbor" {
		base = mergeMaps(base, o.harborExternalURLValues(cfg))
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
		case "loki":
			// 화면은 Loki 를 고를 수 있게 열어 두는데 여기에 분기가 없어
			// 아래 default 로 떨어졌다 — Loki 를 골라도 OpenSearch 가 깔렸다.
			spec.ChartName = "loki"
			spec.RepoURL = "https://grafana.github.io/helm-charts"
			spec.Version = "2.10.3"
			spec.Values = DefaultValues("installing_logging")
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
		// 비밀번호는 values 가 아니라 프로비저닝된 Secret 에서 온다.
		"auth": map[string]any{
			"username":       domain.PostgresAppUser,
			"database":       domain.PostgresAppDatabase,
			"existingSecret": ProvisionedPostgresSecret,
			"secretKeys": map[string]any{
				"userPasswordKey":        domain.PostgresPasswordKey,
				"adminPasswordKey":       "postgres-password",
				"replicationPasswordKey": "replication-password",
			},
		},
		"primary": map[string]any{
			"persistence": map[string]any{
				"enabled": true,
				"size":    fmt.Sprintf("%gGi", storageGi),
			},
		},
	}
}

// harborExternalURLValues 는 Harbor 의 externalURL 을 실제 주소로 맞춘다.
//
// Harbor 는 이 값을 토큰 발급 엔드포인트로 클라이언트에게 되돌려준다. 기본값을
// 그대로 두면 docker login/push 가 존재하지 않는 호스트로 토큰을 요청해
// "no such host" 로 실패한다 — 레지스트리는 떠 있는데 push 만 안 되는,
// 원인이 멀리 떨어진 실패다.
func (o *Orchestrator) harborExternalURLValues(cfg *domain.StackConfig) map[string]any {
	if cfg != nil {
		if accessDomain := strings.TrimSpace(cfg.AccessDomain); accessDomain != "" {
			// Harbor 는 redirect_uri 도 이 값에서 만든다. SSO 를 켠 설치에서는
			// Keycloak 에 등록된 redirect(https://harbor.<도메인>/c/oidc/callback)와
			// 스킴이 같아야 한다 — 다르면 로그인이 "Invalid parameter: redirect_uri"
			// 로 막힌다.
			//
			// SSO 를 쓰지 않으면 http 그대로 둔다. 이 값은 docker login/push 의
			// 토큰 realm 이기도 해서, 스킴을 바꾸면 클라이언트(containerd 포함)가
			// 게이트웨이 인증서의 CA 를 신뢰해야 한다.
			return map[string]any{"externalURL": fmt.Sprintf("%s://harbor.%s", o.toolURLScheme(), accessDomain)}
		}
	}

	namespace := strings.TrimSpace(o.namespace)
	if namespace == "" {
		namespace = defaultStackNamespace
	}
	return map[string]any{
		"externalURL": fmt.Sprintf("http://%s.%s.svc.cluster.local", domain.HarborServiceName, namespace),
	}
}

// giteaSharedServiceValues 는 Gitea 가 스택의 공용 PostgreSQL 을 가리키게 한다.
//
// DefaultValues 는 네임스페이스를 모르므로 기본값을 쓸 수밖에 없다. 그대로
// 설치하면 init 컨테이너가 nullus-postgresql.nullus.svc 를 찾다가 "no such host"
// 로 CrashLoopBackOff 에 빠진다 — 파드는 뜨는데 DB 만 못 찾는, 원인이 멀리
// 떨어진 실패다. GitLab 이 gitlabExternalSharedServiceValues 로 같은 문제를
// 푸는 것과 같은 방식이다.
func (o *Orchestrator) giteaSharedServiceValues() map[string]any {
	namespace := strings.TrimSpace(o.namespace)
	if namespace == "" {
		namespace = defaultStackNamespace
	}

	// ROOT_URL 은 Gitea 가 돌려주는 클론 주소의 출처다. 차트 기본값
	// (http://git.example.com)을 그대로 두면 Argo CD 와 Jenkins 가 존재하지 않는
	// 호스트를 클론하려 한다 — 리포는 만들어지는데 동기화와 빌드만 조용히 실패한다.
	//
	// 접근 도메인이 있으면 그것을 쓴다(GitLab 의 global.hosts.domain 과 같은 규약).
	// 없으면 클러스터 내부 주소로 떨어뜨린다 — 최소한 in-cluster 소비자는 클론할
	// 수 있다.
	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()

	host := ""
	if cfg != nil {
		if accessDomain := strings.TrimSpace(cfg.AccessDomain); accessDomain != "" {
			host = "gitea." + accessDomain
		}
	}
	rootURL := fmt.Sprintf("http://%s.%s.svc:%d/",
		domain.GiteaHTTPServiceName, namespace, domain.GiteaServicePort)
	if host != "" {
		// ROOT_URL 은 Gitea 가 만드는 OAuth redirect_uri 의 출처이기도 하다.
		// Keycloak 에 등록된 redirect 와 스킴이 다르면 로그인이 막힌다.
		rootURL = fmt.Sprintf("%s://%s/", o.toolURLScheme(), host)
	}

	server := map[string]any{"ROOT_URL": rootURL}
	if host != "" {
		server["DOMAIN"] = host
	}

	return map[string]any{
		"gitea": map[string]any{
			"config": map[string]any{
				"database": map[string]any{
					"HOST": fmt.Sprintf("%s.%s.svc.cluster.local:%d",
						domain.PostgresServiceName, namespace, domain.PostgresServicePort),
				},
				"server": server,
			},
		},
	}
}

func (o *Orchestrator) gitlabExternalSharedServiceValues(_ *domain.StackConfig) map[string]any {
	namespace := strings.TrimSpace(o.namespace)
	if namespace == "" {
		namespace = defaultStackNamespace
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
				"host":     fmt.Sprintf("%s.%s.svc.cluster.local", domain.PostgresServiceName, namespace),
				"port":     domain.PostgresServicePort,
				"database": domain.PostgresAppDatabase,
				"username": domain.PostgresAppUser,
				// PostgreSQL 차트를 existingSecret 으로 설치하므로 bitnami 차트는
				// 자기 이름의 Secret 을 만들지 않는다. 프로비저닝된 Secret 을 가리켜야 한다.
				"password": map[string]any{
					"useSecret": true,
					"secret":    ProvisionedPostgresSecret,
					"key":       domain.PostgresPasswordKey,
				},
			},
			"appConfig": map[string]any{
				"object_store": map[string]any{
					"enabled": true,
					"connection": map[string]any{
						"secret": ProvisionedObjectStorageSecret,
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
							"secret": ProvisionedObjectStorageSecret,
							"key":    "config",
						},
					},
				},
			},
		},
		// Container Registry 를 S3(MinIO) 백엔드로 고정한다. 차트 기본값인
		// filesystem 은 PVC 없이 /tmp 를 쓰므로 파드 재시작 시 이미지가 사라지고,
		// replica 2개 사이에 스토리지가 공유되지 않아 pull 이 비결정적으로 실패한다.
		"registry": map[string]any{
			"storage": map[string]any{
				"secret": ProvisionedRegistryStorageSecret,
				"key":    RegistryStorageSecretKey,
			},
		},
	}
}

// sharedObjectStorageSecretManifest 는 ESO 주입 평면을 쓰지 않는 구성을 위한
// 폴백이다. authentication.provider=openbao 인 경우에는 호출되지 않으며,
// nullus-object-storage 는 ExternalSecret 이 소유한다.
func (o *Orchestrator) sharedObjectStorageSecretManifest(namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}

	endpoint := fmt.Sprintf("http://nullus-minio.%s.svc.cluster.local:9000", namespace)
	accessKey := MinIORootUser
	secretKey, err := o.readSecretValue(context.Background(), namespace, ProvisionedMinIOSecret, "rootPassword")
	if err != nil {
		slog.Warn("MinIO 자격증명을 읽지 못해 object storage secret 생성을 건너뜁니다",
			"namespace", namespace, "error", err)
		return ""
	}

	connection := fmt.Sprintf("provider: AWS\nregion: us-east-1\naws_access_key_id: %s\naws_secret_access_key: %s\nendpoint: %s\npath_style: true\n",
		accessKey, secretKey, endpoint)

	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
stringData:
  connection: |
%s
  config: |
%s
`, ProvisionedObjectStorageSecret, namespace, indentYAML(connection, 4), indentYAML(connection, 4))
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
