package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultValues_CertManager(t *testing.T) {
	values := DefaultValues("installing_cert_manager")
	require.NotNil(t, values)
	assert.Equal(t, true, values["installCRDs"])

	resources, ok := values["resources"].(map[string]any)
	require.True(t, ok)
	requests, ok := resources["requests"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "500m", requests["cpu"])
	assert.Equal(t, "512Mi", requests["memory"])

	webhook, ok := values["webhook"].(map[string]any)
	require.True(t, ok)
	_, ok = webhook["resources"].(map[string]any)
	require.True(t, ok)

	cainjector, ok := values["cainjector"].(map[string]any)
	require.True(t, ok)
	_, ok = cainjector["resources"].(map[string]any)
	require.True(t, ok)
}

func TestDefaultValues_GitLab(t *testing.T) {
	values := DefaultValues("installing_gitlab")
	require.NotNil(t, values)

	postgresql, ok := values["postgresql"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, postgresql["install"])

	global, ok := values["global"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ce", global["edition"])
	globalMinio, ok := global["minio"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, globalMinio["enabled"])
	globalPSQL, ok := global["psql"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "nullus-postgresql.nullus.svc.cluster.local", globalPSQL["host"])
	assert.Equal(t, "gitlabhq_production", globalPSQL["database"])
	assert.Equal(t, "gitlab", globalPSQL["username"])

	hosts, ok := global["hosts"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "nullus.internal", hosts["domain"])
	assert.Equal(t, false, hosts["https"])

	ingress, ok := global["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, ingress["enabled"])
	assert.Equal(t, false, ingress["configureCertmanager"])

	nginxIngress, ok := values["nginx-ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, nginxIngress["enabled"])

	// GitLab 차트는 자체 Prometheus 를 함께 세운다. Nullus 는 모니터링 수집을
	// kube-prometheus-stack 으로 따로 깔므로 한 클러스터에 Prometheus 가 둘이
	// 된다 — 같은 것을 두 번 긁고 메모리만 더 쓴다.
	//
	// 게다가 이 번들 Prometheus 는 자원 규모를 낮춘 구성에서 반드시 죽는다.
	// 실제로 Local/Startup 규모(메모리 한도 328Mi)에서 OOMKilled(exit 137) 로
	// 34번 재시작하며 CrashLoopBackOff 에 갇혀 있었다. 스택은 "실행 중" 인데
	// 파드 하나가 영원히 안 뜨는 상태다.
	//
	// postgresql / minio / nginx-ingress 를 이미 같은 이유로 끄고 있다.
	prometheus, ok := values["prometheus"].(map[string]any)
	require.True(t, ok, "GitLab 번들 Prometheus 설정이 있어야 한다")
	assert.Equal(t, false, prometheus["install"],
		"Nullus 가 kube-prometheus-stack 을 따로 깐다 — 번들 Prometheus 는 끈다")

	gitlab, ok := values["gitlab"].(map[string]any)
	require.True(t, ok)
	webservice, ok := gitlab["webservice"].(map[string]any)
	require.True(t, ok)
	webserviceIngress, ok := webservice["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, webserviceIngress["enabled"])
	webserviceReadinessProbe, ok := webservice["readinessProbe"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 90, webserviceReadinessProbe["initialDelaySeconds"])
	assert.Equal(t, 18, webserviceReadinessProbe["failureThreshold"])
	webserviceLivenessProbe, ok := webservice["livenessProbe"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 180, webserviceLivenessProbe["initialDelaySeconds"])
	assert.Equal(t, 6, webserviceLivenessProbe["failureThreshold"])

	sidekiq, ok := gitlab["sidekiq"].(map[string]any)
	require.True(t, ok)
	sidekiqReadinessProbe, ok := sidekiq["readinessProbe"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 120, sidekiqReadinessProbe["initialDelaySeconds"])
	assert.Equal(t, 18, sidekiqReadinessProbe["failureThreshold"])
	sidekiqLivenessProbe, ok := sidekiq["livenessProbe"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 240, sidekiqLivenessProbe["initialDelaySeconds"])
	assert.Equal(t, 6, sidekiqLivenessProbe["failureThreshold"])

	kas, ok := gitlab["kas"].(map[string]any)
	require.True(t, ok)
	kasIngress, ok := kas["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, kasIngress["enabled"])

	registry, ok := values["registry"].(map[string]any)
	require.True(t, ok)
	registryIngress, ok := registry["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, registryIngress["enabled"])

	certmanager, ok := values["certmanager"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, certmanager["install"])

	issuer, ok := values["certmanager-issuer"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, issuer["enabled"])

	runner, ok := values["gitlab-runner"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, runner["install"])

	redis, ok := values["redis"].(map[string]any)
	require.True(t, ok)
	redisImage, ok := redis["image"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bitnamilegacy/redis", redisImage["repository"])
	assert.Equal(t, "7.4.2-debian-12-r0", redisImage["tag"])
	redisMaster, ok := redis["master"].(map[string]any)
	require.True(t, ok)
	redisMasterResources, ok := redisMaster["resources"].(map[string]any)
	require.True(t, ok)
	redisMasterRequests, ok := redisMasterResources["requests"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "250m", redisMasterRequests["cpu"])
	assert.Equal(t, "512Mi", redisMasterRequests["memory"])
	redisMasterLimits, ok := redisMasterResources["limits"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "500m", redisMasterLimits["cpu"])
	assert.Equal(t, "1Gi", redisMasterLimits["memory"])
	redisMasterReadinessProbe, ok := redisMaster["readinessProbe"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, redisMasterReadinessProbe["enabled"])
	assert.Equal(t, 30, redisMasterReadinessProbe["initialDelaySeconds"])
	assert.Equal(t, 5, redisMasterReadinessProbe["timeoutSeconds"])
	redisMasterLivenessProbe, ok := redisMaster["livenessProbe"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, redisMasterLivenessProbe["enabled"])
	assert.Equal(t, 60, redisMasterLivenessProbe["initialDelaySeconds"])
	assert.Equal(t, 10, redisMasterLivenessProbe["timeoutSeconds"])

	redisMetrics, ok := redis["metrics"].(map[string]any)
	require.True(t, ok)
	redisMetricsImage, ok := redisMetrics["image"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bitnamilegacy/redis-exporter", redisMetricsImage["repository"])
	assert.Equal(t, "1.76.0-debian-12-r0", redisMetricsImage["tag"])
}

func TestDefaultValues_GitLabRunner(t *testing.T) {
	values := DefaultValues("installing_runner")
	require.NotNil(t, values)

	rbac, ok := values["rbac"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, rbac["create"])

	runners, ok := values["runners"].(map[string]any)
	require.True(t, ok)

	// gitlab-runner 차트 0.72.0 은 runners.privileged 를 더 이상 읽지 않는다.
	// 설정은 runners.config 의 TOML 로 들어가야 실제 config.toml 에 반영된다.
	// (privileged 없이는 docker:dind 서비스가 기동하지 못해 이미지 빌드가 불가능하다)
	config, ok := runners["config"].(string)
	require.True(t, ok, "runners.config 로 넣지 않으면 차트가 무시한다")
	assert.Contains(t, config, "[runners.kubernetes]")
	assert.Contains(t, config, "privileged = true")
}

func TestDefaultValues_UnknownStepReturnsEmptyMap(t *testing.T) {
	values := DefaultValues("unknown_step")
	require.NotNil(t, values)
	assert.Empty(t, values)
}

func TestDefaultValues_MinIOIngressDisabled(t *testing.T) {
	values := DefaultValues("installing_minio")
	ingress, ok := values["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, ingress["enabled"])

	consoleIngress, ok := values["consoleIngress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, consoleIngress["enabled"])

	resources, ok := values["resources"].(map[string]any)
	require.True(t, ok)
	limits, ok := resources["limits"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1", limits["cpu"])
	assert.Equal(t, "2Gi", limits["memory"])
}

func TestDefaultValues_PostgreSQLSharedDefaults(t *testing.T) {
	values := DefaultValues("installing_postgresql")
	require.NotNil(t, values)
	assert.Equal(t, "standalone", values["architecture"])

	auth, ok := values["auth"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "gitlab", auth["username"])
	assert.Equal(t, "gitlabhq_production", auth["database"])

	primary, ok := values["primary"].(map[string]any)
	require.True(t, ok)
	resources, ok := primary["resources"].(map[string]any)
	require.True(t, ok)
	requests, ok := resources["requests"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1", requests["cpu"])
	assert.Equal(t, "2Gi", requests["memory"])

	persistence, ok := primary["persistence"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, persistence["enabled"])
	assert.Equal(t, "20Gi", persistence["size"])
}

func TestDefaultValues_MetricsServerResources(t *testing.T) {
	values := DefaultValues("installing_metrics_server")
	resources, ok := values["resources"].(map[string]any)
	require.True(t, ok)
	requests, ok := resources["requests"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "250m", requests["cpu"])
	assert.Equal(t, "256Mi", requests["memory"])
}

func TestDefaultValues_ArgoCDIngressDisabled(t *testing.T) {
	values := DefaultValues("installing_argocd")
	server, ok := values["server"].(map[string]any)
	require.True(t, ok)
	ingress, ok := server["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, ingress["enabled"])

	configs, ok := values["configs"].(map[string]any)
	require.True(t, ok)
	params, ok := configs["params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "true", params["server.insecure"])

	secret, ok := configs["secret"].(map[string]any)
	require.True(t, ok)
	extra, ok := secret["extra"].(map[string]any)
	require.True(t, ok)
	serverSecretKey, ok := extra["server.secretkey"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, serverSecretKey)
}

func TestDefaultValues_OpenSearchProtocolAndSecurity(t *testing.T) {
	values := DefaultValues("installing_logging_opensearch")
	assert.Equal(t, "http", values["protocol"])

	securityConfig, ok := values["securityConfig"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, securityConfig["enabled"])

	config, ok := values["config"].(map[string]any)
	require.True(t, ok)
	opensearchConfig, ok := config["opensearch.yml"].(string)
	require.True(t, ok)
	assert.Contains(t, opensearchConfig, "plugins.security.disabled: true")
}

func TestDefaultValues_PrometheusIngressDisabled(t *testing.T) {
	values := DefaultValues("installing_prometheus")
	prometheus, ok := values["prometheus"].(map[string]any)
	require.True(t, ok)
	promIngress, ok := prometheus["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, promIngress["enabled"])

	alertmanager, ok := values["alertmanager"].(map[string]any)
	require.True(t, ok)
	alertIngress, ok := alertmanager["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, alertIngress["enabled"])
}

func TestDefaultValues_GrafanaIngressDisabled(t *testing.T) {
	values := DefaultValues("installing_grafana")
	ingress, ok := values["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, ingress["enabled"])
}

func TestDefaultValues_LoggingDefaults(t *testing.T) {
	values := DefaultValues("installing_logging")
	require.NotNil(t, values)

	loki, ok := values["loki"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, loki["enabled"])

	promtail, ok := values["promtail"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, promtail["enabled"])
}

func TestDefaultValues_OpenTelemetryDefaults(t *testing.T) {
	values := DefaultValues("installing_opentelemetry")
	require.NotNil(t, values)
	assert.Equal(t, "deployment", values["mode"])
}
