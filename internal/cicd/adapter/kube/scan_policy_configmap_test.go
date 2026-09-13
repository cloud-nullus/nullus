package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// Jenkins 에는 CI 변수 저장소가 없다. 스캐너 컨테이너가 envFrom 으로 읽는
// ConfigMap 을 스택 네임스페이스에 둔다 — 스택 하나에 하나라 정책을 바꾸면
// 그 스택의 Jenkins 파이프라인 전체가 다음 실행부터 따른다.
func TestRenderScanPolicyConfigMap(t *testing.T) {
	out, err := RenderScanPolicyConfigMap("devsecops", port.ScanPolicyVariables(domain.ScanPolicy{
		BlockSeverity: domain.SeverityHigh, IgnoreUnfixed: true, OnScannerUnreachable: domain.UnreachableBlock,
	}))
	require.NoError(t, err)

	var doc struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name      string            `json:"name"`
			Namespace string            `json:"namespace"`
			Labels    map[string]string `json:"labels"`
		} `json:"metadata"`
		Data map[string]string `json:"data"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(out), &doc))

	assert.Equal(t, "v1", doc.APIVersion)
	assert.Equal(t, "ConfigMap", doc.Kind)
	assert.Equal(t, port.ScanPolicyConfigMapName, doc.Metadata.Name, "렌더러의 envFrom 이 이 이름을 찾는다")
	assert.Equal(t, "devsecops", doc.Metadata.Namespace)
	assert.Equal(t, "nullus-cicd", doc.Metadata.Labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, map[string]string{
		port.ScanSeverityVariable:      "HIGH,CRITICAL",
		port.ScanIgnoreUnfixedVariable: "true",
		port.ScanOnUnreachableVariable: "block",
	}, doc.Data)
}

// 네임스페이스를 모르면 kubectl 기본 네임스페이스에 떨어져 에이전트 파드가 못 읽는다.
func TestRenderScanPolicyConfigMap_RequiresNamespace(t *testing.T) {
	_, err := RenderScanPolicyConfigMap(" ", port.ScanPolicyVariables(domain.DefaultScanPolicy()))
	assert.Error(t, err)
}
