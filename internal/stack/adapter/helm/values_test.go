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

	gitlab, ok := values["gitlab"].(map[string]any)
	require.True(t, ok)
	webservice, ok := gitlab["webservice"].(map[string]any)
	require.True(t, ok)
	webserviceIngress, ok := webservice["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, webserviceIngress["enabled"])

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

	redisMetrics, ok := redis["metrics"].(map[string]any)
	require.True(t, ok)
	redisMetricsImage, ok := redisMetrics["image"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bitnamilegacy/redis-exporter", redisMetricsImage["repository"])
	assert.Equal(t, "1.76.0-debian-12-r0", redisMetricsImage["tag"])
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
