package helm

import (
	"fmt"
	"strings"
)

func (o *Orchestrator) stepManifestForStep(step string) (string, bool) {
	if step == "installing_openbao" {
		return o.openBaoManifest(o.namespace), true
	}
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

func (o *Orchestrator) openBaoManifest(namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: openbao
  namespace: %s
  labels:
    app.kubernetes.io/name: openbao
    app.kubernetes.io/instance: openbao
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: openbao
      app.kubernetes.io/instance: openbao
  template:
    metadata:
      labels:
        app.kubernetes.io/name: openbao
        app.kubernetes.io/instance: openbao
    spec:
      containers:
        - name: openbao
          image: openbao/openbao:latest
          imagePullPolicy: IfNotPresent
          args: ["server", "-dev", "-dev-root-token-id=root"]
          env:
            - name: VAULT_DEV_LISTEN_ADDRESS
              value: 0.0.0.0:8200
            - name: VAULT_UI
              value: "true"
          ports:
            - containerPort: 8200
              name: http
          readinessProbe:
            httpGet:
              path: /v1/sys/health?standbyok=true
              port: 8200
            initialDelaySeconds: 10
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: openbao
  namespace: %s
  labels:
    app.kubernetes.io/name: openbao
    app.kubernetes.io/instance: openbao
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: openbao
    app.kubernetes.io/instance: openbao
  ports:
    - name: http
      port: 8200
      targetPort: 8200
`, namespace, namespace)
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
        namespaces:
          from: Same
`, gatewayName, namespace, stackLabel, accessDomain)}

	type routeSpec struct {
		name    string
		host    string
		service string
		port    int
	}
	routes := make([]routeSpec, 0, 6)

	if cfg.Pipeline.CDTool.Enabled && (strings.EqualFold(cfg.Pipeline.CDTool.Name, "argocd") || strings.EqualFold(cfg.Pipeline.CDTool.Name, "argo-cd")) {
		routes = append(routes, routeSpec{name: "argocd-route", host: fmt.Sprintf("argocd.%s", accessDomain), service: "argo-cd-argocd-server", port: 80})
	}
	if cfg.Logging.Search.Enabled && strings.EqualFold(cfg.Logging.Search.Name, "opensearch") {
		routes = append(routes, routeSpec{name: "opensearch-route", host: fmt.Sprintf("opensearch.%s", accessDomain), service: "opensearch-cluster-master", port: 9200})
	}
	if cfg.Artifacts.SourceRepository.Enabled || cfg.Pipeline.CIPlatform.Enabled || cfg.Artifacts.PackageRegistry.Enabled || cfg.Artifacts.ContainerRegistry.Enabled {
		routes = append(routes, routeSpec{name: "gitlab-route", host: fmt.Sprintf("gitlab.%s", accessDomain), service: "gitlab-webservice-default", port: 8080})
	}
	if cfg.Monitoring.Visualization.Enabled && strings.EqualFold(cfg.Monitoring.Visualization.Name, "grafana") {
		routes = append(routes, routeSpec{name: "grafana-route", host: fmt.Sprintf("grafana.%s", accessDomain), service: "grafana", port: 80})
	}
	if cfg.Monitoring.Collection.Enabled && strings.EqualFold(cfg.Monitoring.Collection.Name, "prometheus") {
		routes = append(routes, routeSpec{name: "prometheus-route", host: fmt.Sprintf("prometheus.%s", accessDomain), service: "kube-prometheus-stack-prometheus", port: 9090})
	}
	if cfg.Artifacts.StorageBackend.Enabled && strings.EqualFold(cfg.Artifacts.StorageBackend.Name, "minio") {
		routes = append(routes, routeSpec{name: "minio-route", host: fmt.Sprintf("minio.%s", accessDomain), service: "nullus-minio-console", port: 9001})
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
