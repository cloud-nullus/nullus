package helm

import (
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func (o *Orchestrator) stepManifestForStep(step string) (string, bool) {
	// installing_openbao 는 더 이상 자체 매니페스트를 쓰지 않는다.
	// 공식 Helm 차트로 설치되며 초기화는 openbao-init.go 의 Job 이 담당한다.
	if step != "installing_prometheus" && step != "installing_grafana" && step != "installing_logging" && step != "installing_log_search" && step != "installing_opentelemetry" && step != "installing_gateway" {
		return "", false
	}

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil || len(cfg.YAMLOverrides) == 0 {
		return "", false
	}

	keys := []string{step}
	if step == "installing_prometheus" {
		keys = append(keys, "prometheus", cfg.Monitoring.Collection.Name)
	}
	if step == "installing_grafana" {
		keys = append(keys, "grafana", cfg.Monitoring.Visualization.Name)
	}
	if step == "installing_logging" {
		keys = append(keys, "logging", cfg.Logging.Collection.Name)
	}
	if step == "installing_log_search" {
		keys = append(keys, "log_search", cfg.Logging.Search.Name)
	}
	if step == "installing_opentelemetry" {
		keys = append(keys, "opentelemetry", "opentelemetry-collector", cfg.Logging.TraceLayer.Name)
	}
	if step == "installing_gateway" {
		keys = append(keys, "gateway")
	}

	for _, key := range keys {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		raw, ok := cfg.YAMLOverrides[k]
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "apiVersion:") && strings.Contains(trimmed, "kind:") {
			return raw, true
		}
	}

	return "", false
}

func (o *Orchestrator) defaultGatewayBundleManifest(namespace string) string {
	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil {
		return ""
	}
	accessDomain := strings.TrimSpace(cfg.AccessDomain)
	if accessDomain == "" {
		return ""
	}
	stackLabel := strings.TrimSpace(strings.TrimSuffix(accessDomain, ".internal"))
	if stackLabel == "" {
		stackLabel = "nullus-stack"
	}

	gatewayName := fmt.Sprintf("%s-gateway", sanitizeK8sName(stackLabel))
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}

	manifests := []string{fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: %s
  namespace: %s
  labels:
    nullus.io/stack-name: %s
spec:
  gatewayClassName: envoy
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      hostname: "*.%s"
      allowedRoutes:
        # 배포된 애플리케이션은 자기 네임스페이스에 산다. Same 으로 두면
        # 앱이 게이트웨이에 라우트를 붙일 수 없어 외부에서 접근할 방법이 없다.
        namespaces:
          from: All
`, gatewayName, namespace, stackLabel, accessDomain)}

	type routeSpec struct {
		name    string
		host    string
		service string
		port    int
	}
	routes := make([]routeSpec, 0, 6)

	// 도구 이름은 카탈로그에서 "Argo CD" 처럼 공백을 포함해 온다.
	// EqualFold 만으로 비교하면 공백 때문에 매칭에 실패해 라우트가 만들어지지
	// 않고, 설치는 성공했는데 UI 에 접근할 수 없는 상태가 된다.
	if cfg.Pipeline.CDTool.Enabled && isArgoCDSelection(cfg.Pipeline.CDTool.Name) {
		routes = append(routes, routeSpec{name: "argocd-route", host: fmt.Sprintf("argocd.%s", accessDomain), service: "argo-cd-argocd-server", port: 80})
	}
	if cfg.Logging.Search.Enabled && strings.EqualFold(cfg.Logging.Search.Name, "opensearch") {
		routes = append(routes, routeSpec{name: "opensearch-route", host: fmt.Sprintf("opensearch.%s", accessDomain), service: "opensearch-cluster-master", port: 9200})
	}
	if cfg.Artifacts.SourceRepository.Enabled || cfg.Pipeline.CIPlatform.Enabled || cfg.Artifacts.PackageRegistry.Enabled || cfg.Artifacts.ContainerRegistry.Enabled {
		// 8181(workhorse)로 보낸다. 8080 은 puma 직결이라 웹 UI 는 뜨지만
		// git clone/push 가 workhorse 를 거치지 못해 "Nil JSON web token" 403 으로 실패한다.
		routes = append(routes, routeSpec{name: "gitlab-route", host: fmt.Sprintf("gitlab.%s", accessDomain), service: "gitlab-webservice-default", port: 8181})
	}
	// 컨테이너 레지스트리는 GitLab webservice 와 다른 서비스다. 노출하지 않으면
	// CI 가 빌드한 이미지를 올릴 곳이 없고 kubelet 도 그것을 받아올 수 없다.
	//
	// registry.<도메인> 은 "이 스택이 고른 이미지 레지스트리" 하나를 가리킨다.
	// 선택과 무관하게 gitlab-registry 로 보내면 Harbor/Nexus 를 고른 스택이
	// 존재하지도 않는(혹은 비어 있는) GitLab 레지스트리로 push 하게 된다.
	if isGitLabContainerRegistrySelection(cfg.Artifacts.ContainerRegistry) {
		routes = append(routes, routeSpec{name: "registry-route", host: fmt.Sprintf("registry.%s", accessDomain), service: "gitlab-registry", port: 5000})
	}
	// Gitea 는 GitLab 과 같은 슬롯의 다른 선택지다. 노출하지 않으면 사용자가
	// 웹 UI 로 들어갈 수도, git clone 을 할 수도 없다.
	if isGiteaSourceRepositorySelection(cfg.Artifacts.SourceRepository) {
		routes = append(routes, routeSpec{name: "gitea-route", host: fmt.Sprintf("gitea.%s", accessDomain), service: domain.GiteaHTTPServiceName, port: domain.GiteaServicePort})
	}
	// Jenkins 웹 UI. 노출하지 않으면 사용자가 빌드 로그를 볼 수 없고, Gitea
	// webhook 도 컨트롤러에 닿지 못한다.
	if isJenkinsCISelection(cfg.Pipeline.CIPlatform) {
		routes = append(routes, routeSpec{name: "jenkins-route", host: fmt.Sprintf("jenkins.%s", accessDomain), service: domain.JenkinsServiceName, port: domain.JenkinsServicePort})
	}
	if cfg.Monitoring.Visualization.Enabled && strings.EqualFold(cfg.Monitoring.Visualization.Name, "grafana") {
		routes = append(routes, routeSpec{name: "grafana-route", host: fmt.Sprintf("grafana.%s", accessDomain), service: "grafana", port: 80})
	}
	if cfg.Monitoring.Collection.Enabled && strings.EqualFold(cfg.Monitoring.Collection.Name, "prometheus") {
		routes = append(routes, routeSpec{name: "prometheus-route", host: fmt.Sprintf("prometheus.%s", accessDomain), service: "kube-prometheus-stack-prometheus", port: 9090})
	}
	if isHarborRegistrySelection(cfg.Artifacts.ContainerRegistry) {
		// Harbor 는 UI 와 레지스트리를 같은 포트로 서비스하므로 두 호스트가
		// 같은 곳을 가리킨다. registry.<도메인> 을 함께 두는 이유는 레지스트리
		// 주소를 도구와 무관하게 쓰려는 파이프라인이 있기 때문이다.
		routes = append(routes, routeSpec{name: "harbor-route", host: fmt.Sprintf("harbor.%s", accessDomain), service: domain.HarborServiceName, port: domain.HarborServicePort})
		routes = append(routes, routeSpec{name: "harbor-registry-route", host: fmt.Sprintf("registry.%s", accessDomain), service: domain.HarborServiceName, port: domain.HarborServicePort})
	}
	if isNexusSelection(cfg.Artifacts.ContainerRegistry) || isNexusSelection(cfg.Artifacts.PackageRegistry) {
		routes = append(routes, routeSpec{name: "nexus-route", host: fmt.Sprintf("nexus.%s", accessDomain), service: domain.NexusServiceName, port: domain.NexusServicePort})
		// Docker 레지스트리는 별도 포트라 호스트도 나눈다. 하나의 호스트로
		// 합치면 docker push 가 UI 로 흘러들어 401 을 받는다.
		routes = append(routes, routeSpec{name: "nexus-docker-route", host: fmt.Sprintf("registry.%s", accessDomain), service: domain.NexusServiceName + "-docker", port: domain.NexusDockerServicePort})
	}
	if cfg.Artifacts.StorageBackend.Enabled && strings.EqualFold(cfg.Artifacts.StorageBackend.Name, "minio") {
		routes = append(routes, routeSpec{name: "minio-route", host: fmt.Sprintf("minio.%s", accessDomain), service: domain.MinIOConsoleServiceName, port: domain.MinIOConsoleServicePort})
	}
	if cfg.Authentication != nil && strings.EqualFold(strings.TrimSpace(cfg.Authentication.Provider), "openbao") {
		routes = append(routes, routeSpec{name: "openbao-route", host: fmt.Sprintf("openbao.%s", accessDomain), service: "openbao", port: 8200})
	}

	for _, route := range routes {
		manifests = append(manifests, fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: %s
  namespace: %s
  labels:
    nullus.io/stack-name: %s
spec:
  parentRefs:
    - name: %s
  hostnames:
    - %s
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: %s
          port: %d
`, route.name, namespace, stackLabel, gatewayName, route.host, route.service, route.port))
	}

	return strings.Join(manifests, "\n---\n")
}

func sanitizeK8sName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, ".", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")
	parts := make([]rune, 0, len(normalized))
	lastDash := false
	for _, r := range normalized {
		isAlpha := r >= 'a' && r <= 'z'
		isNum := r >= '0' && r <= '9'
		if isAlpha || isNum {
			parts = append(parts, r)
			lastDash = false
			continue
		}
		if !lastDash {
			parts = append(parts, '-')
			lastDash = true
		}
	}
	out := strings.Trim(string(parts), "-")
	if out == "" {
		return "nullus-stack"
	}
	return out
}

func defaultEnvoyGatewayClassManifest() string {
	return `apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: envoy
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
`
}

// isArgoCDSelection 은 Argo CD 표기 흔들림을 흡수한다.
// 카탈로그는 "Argo CD", 설정 파일은 "argocd" 를 쓰는 등 표기가 갈린다.
func isArgoCDSelection(name string) bool {
	switch normalizeToolName(name) {
	case "argocd", "argo-cd":
		return true
	}
	return false
}
