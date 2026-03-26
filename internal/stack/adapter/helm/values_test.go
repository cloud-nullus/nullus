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
}

func TestDefaultValues_GitLab(t *testing.T) {
	values := DefaultValues("installing_gitlab")
	require.NotNil(t, values)

	global, ok := values["global"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ce", global["edition"])

	hosts, ok := global["hosts"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "nullus.internal", hosts["domain"])

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

	minio, ok := values["minio"].(map[string]any)
	require.True(t, ok)
	minioIngress, ok := minio["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, minioIngress["enabled"])

	certmanager, ok := values["certmanager"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, certmanager["install"])

	issuer, ok := values["certmanager-issuer"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, issuer["enabled"])

	runner, ok := values["gitlab-runner"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, runner["install"])

	postgresql, ok := values["postgresql"].(map[string]any)
	require.True(t, ok)
	postgresqlImage, ok := postgresql["image"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bitnamilegacy/postgresql", postgresqlImage["repository"])
	assert.Equal(t, "16.6.0-debian-12-r2", postgresqlImage["tag"])

	postgresqlMetrics, ok := postgresql["metrics"].(map[string]any)
	require.True(t, ok)
	postgresqlMetricsImage, ok := postgresqlMetrics["image"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bitnamilegacy/postgres-exporter", postgresqlMetricsImage["repository"])
	assert.Equal(t, "0.17.1-debian-12-r16", postgresqlMetricsImage["tag"])

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
}

func TestDefaultValues_ArgoCDIngressDisabled(t *testing.T) {
	values := DefaultValues("installing_argocd")
	server, ok := values["server"].(map[string]any)
	require.True(t, ok)
	ingress, ok := server["ingress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, ingress["enabled"])
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
