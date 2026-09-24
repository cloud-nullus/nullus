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
		base = mergeMaps(base, o.jenkinsURLValues())
	}

	// Gitea 의 DB 호스트도 실제 네임스페이스를 알아야 한다.
	if step == "installing_gitea" {
		base = mergeMaps(base, o.giteaSharedServiceValues())
	}

	// GitLab Runner 의 helper 이미지와 CI 잡의 이름 해석은 차트 values 가 아니라
	// 러너 설정 TOML 안에 들어간다. 그래서 archImageValuesForStep 경로가 아니라
	// 여기서 조립한다. 둘이 같은 키(runners.config)를 쓰므로 한 번에 만든다 —
	// 따로 병합하면 뒤에 오는 쪽이 앞의 것을 지운다.
	if step == stepInstallingRunner {
		base = mergeMaps(base, o.gitLabRunnerValues())
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

	// 에어갭 설치의 Trivy 서버는 내부 레지스트리에서 DB 를 받는다. 오버라이드 유무와
	// 무관하게 필요하고, 사용자 오버라이드가 뒤에 합쳐지므로 다른 미러로 바꿀 수 있다.
	if step == "installing_trivy" {
		base = mergeMaps(base, trivyAirgapDBValues(airgapOCIRegistry()))
	}

	// 공식 이미지가 노드 아키텍처를 내지 않는 도구는 대체 출처의 이미지로 바꾼다
	// (arm64 의 Harbor). 기본값 자리에 두어 사용자 오버라이드가 뒤에서 이긴다 —
	// 자기 미러나 다른 빌드로 바꿀 수 있어야 한다.
	if images := archImageValuesForStep(step, o.nodeArchitectures()); len(images) > 0 {
		base = mergeMaps(base, images)
	}

	// OSS 가 자기 메트릭을 내주도록 켠다. 사용자가 오버라이드로 끌 수 있어야
	// 하므로 플랫폼 소유 값이 아니라 기본값 자리에 둔다.
	if monitors := serviceMonitorValuesForStep(step, cfg); len(monitors) > 0 {
		base = mergeMaps(base, monitors)
	}

	// OIDC 블록은 스택별 client ID / accessDomain 에 의존한다.
	// 에어갭 values 파일에만 있던 설정을 코드 경로로 끌어와 일반 설치에도 적용한다.
	if oidc := o.oidcValuesForStep(step); len(oidc) > 0 {
		base = mergeOIDCValues(base, oidc)
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
		if step == "installing_gitlab" {
			base = mergeMaps(base, o.gitlabSharedServiceValues())
		}
		return base
	}

	// override 가 없는 분기와 같은 공용 값을 준다. 여기에 도메인만 두었더니 다른
	// 도구 하나만 손봐도 registry.authEndpoint(https realm)와 global.hosts.https 가
	// 빠져, 이미지 스캔이 http realm 에서 다시 거부됐다.
	if step == "installing_gitlab" {
		base = mergeMaps(base, o.gitlabSharedServiceValues())
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
			// 항상 https 다. SSO·TLS 설정과 무관하다.
			//
			// Harbor 는 레지스트리 토큰 realm 을 따로 정하는 설정이 없어
			// <externalURL>/service/token 을 광고한다. 게이트웨이는 HTTPS 리스너를
			// 늘 열므로 스캐너(Trivy 등 go-containerregistry)는 https 로 붙는데,
			// http realm 을 받으면 "realm scheme "http" not allowed for a secure
			// registry" 로 거부해 스캔 잡이 매번 실패한다(GitLab 레지스트리에서 실측).
			//
			// 같은 값에서 나오는 것들도 https 여야 맞는다: OIDC redirect_uri 는
			// Keycloak 에 https 로 등록되고, 화면의 도구 링크(domain.ToolAccessURL)도
			// https 다. blob 업로드 Location 은 registry.relativeurls 로 스킴이 나가지
			// 않는다. Harbor UI 의 CSRF 쿠키가 Secure 가 되므로 UI 는 https 로 연다.
			return map[string]any{"externalURL": fmt.Sprintf("https://harbor.%s", accessDomain)}
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

// mergeOIDCValues 는 OIDC 블록을 기존 values 에 얹는다.
//
// mergeMaps 와 다른 점은 슬라이스를 이어붙인다는 것이다. OIDC values 는 기존
// 설정을 바꾸는 게 아니라 "더하는" 성격인데, 통째로 바꾸면 같은 키를 쓰던 기존
// 항목이 사라진다. 실제로 Jenkins 의 additionalExistingSecrets 에 OIDC 시크릿을
// 넣자 기존 Gitea 자격 항목 두 개가 밀려나, JCasC 가 자격을 풀지 못해 Jenkins 가
// 기동에 실패했다(SEVERE hudson.util.BootFailure).
//
// mergeMaps 자체를 바꾸지 않는다 — 사용자 오버라이드는 목록을 "교체" 하려는
// 의도일 수 있어 전역 규칙을 바꾸면 다른 곳이 조용히 달라진다.
func mergeOIDCValues(base, oidc map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for key, value := range oidc {
		switch override := value.(type) {
		case map[string]any:
			subBase, _ := base[key].(map[string]any)
			base[key] = mergeOIDCValues(subBase, override)
		case []any:
			existing, _ := base[key].([]any)
			base[key] = append(append([]any{}, existing...), override...)
		default:
			base[key] = value
		}
	}
	return base
}

// jenkinsURLValues 는 Jenkins 가 자기 주소를 알게 한다.
//
// oic-auth 는 redirect_uri 를 이 값에서 만든다. 설정하지 않으면 클러스터 내부
// 주소가 잡혀 Keycloak 에 등록된 redirect 와 어긋나고, 로그인이
// "Invalid parameter: redirect_uri" 로 막힌다 — Harbor 의 externalURL,
// Gitea 의 ROOT_URL 과 같은 실패다. 스킴도 그 둘과 같은 판단을 쓴다.
//
// 접속 도메인이 없으면 아무것도 넣지 않는다. 엉뚱한 주소를 박느니 차트 기본값에
// 맡기는 편이 낫다.
func (o *Orchestrator) jenkinsURLValues() map[string]any {
	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil {
		return nil
	}
	accessDomain := strings.TrimSpace(cfg.AccessDomain)
	if accessDomain == "" {
		return nil
	}

	return map[string]any{
		"controller": map[string]any{
			"jenkinsUrl": fmt.Sprintf("%s://jenkins.%s", o.toolURLScheme(), accessDomain),
		},
	}
}

// gitlabSharedServiceValues 는 GitLab 이 자기 외부 주소를 알게 한다.
//
// GitLab 은 redirect_uri 를 global.hosts 에서 만든다. https 를 켜지 않으면
// http 로 나가 Keycloak 에 등록된 https redirect 와 어긋나고, 로그인이
// "redirect_uri" 오류로 막힌다 — Harbor·Gitea·Jenkins 와 같은 실패라 같은
// 판단(toolURLScheme)을 쓴다.
func (o *Orchestrator) gitlabSharedServiceValues() map[string]any {
	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil {
		return nil
	}
	accessDomain := strings.TrimSpace(cfg.AccessDomain)
	if accessDomain == "" {
		return nil
	}

	return map[string]any{
		"global": map[string]any{
			"hosts": map[string]any{
				"domain": accessDomain,
				"https":  o.toolURLScheme() == "https",
			},
		},
		// 레지스트리 인증 realm 은 항상 https 다. 게이트웨이는 설정과 무관하게
		// HTTPS 리스너를 열고(defaultGatewayBundleManifest), https 로 접속한 엄격한
		// 클라이언트(Trivy·crane 등 go-containerregistry)는 http realm 을 "not allowed
		// for a secure registry" 로 거부한다 — kind 스택에서 이미지 스캔이 매번 실패했다.
		//
		// GitLab 외부 주소 전체(global.hosts.https)는 건드리지 않는다. 그것을 바꾸면
		// 러너 clone 과 deploy 잡의 push 가 내부 CA 신뢰 문제에 걸린다. 차트가
		// 뒤에 /jwt/auth 를 붙이므로 스킴과 호스트만 준다.
		"registry": map[string]any{
			"authEndpoint": fmt.Sprintf("https://gitlab.%s", accessDomain),
		},
	}
}
