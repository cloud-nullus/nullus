package handler

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/manifests"
)

// 네임스페이스가 먼저 적용되어야 한다. 없는 네임스페이스에 Deployment 를
// 넣으면 그 자리에서 실패한다.
func TestDeployAppManifestDocs_NamespaceComesFirst(t *testing.T) {
	generated, err := manifests.Generate(manifests.DeployAppRequest{
		AppName:   "api",
		Namespace: "team-a",
		Template:  "express-api",
	})
	require.NoError(t, err)

	docs := deployAppManifestDocs(generated)
	require.Len(t, docs, 4)
	assert.Contains(t, docs[0], "kind: Namespace")
	assert.Contains(t, docs[1], "kind: Deployment")
	assert.Contains(t, docs[2], "kind: Service")
	assert.Contains(t, docs[3], "kind: Ingress")
}

// 빈 문서는 kubectl apply 가 오류를 낸다.
func TestDeployAppManifestDocs_SkipsEmptyDocuments(t *testing.T) {
	docs := deployAppManifestDocs(&manifests.GeneratedManifests{
		Namespace:  "apiVersion: v1\nkind: Namespace\n",
		Deployment: "",
		Service:    "   ",
		Ingress:    "apiVersion: networking.k8s.io/v1\nkind: Ingress\n",
	})

	require.Len(t, docs, 2)
	for _, doc := range docs {
		assert.NotEmpty(t, strings.TrimSpace(doc))
	}
}

func TestDeployAppManifestDocs_NilIsEmpty(t *testing.T) {
	assert.Empty(t, deployAppManifestDocs(nil))
}

// 배선이 없으면 조용히 건너뛰지 않는다. 건너뛰면 "적용하지 않고 성공으로
// 기록" 하는 예전 상태로 되돌아간다.
func TestApplyDeployAppManifests_FailsWithoutWiring(t *testing.T) {
	h := &PipelineHandler{}
	err := h.applyDeployAppManifests(t.Context(), "c1", &manifests.GeneratedManifests{
		Namespace: "apiVersion: v1\nkind: Namespace\n",
	}, "dep_1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "배선")
}
