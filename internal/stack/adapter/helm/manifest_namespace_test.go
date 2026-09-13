package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeManifestNamespace_RewritesLegacyNullusPlaceholder(t *testing.T) {
	manifest := `apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: stack
  namespace: nullus
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: stack
  namespace: nullus # generated preview
spec:
  parentRefs:
    - name: stack
      namespace: nullus
`

	got, changed := normalizeManifestNamespace(manifest, "nullus-demo")

	assert.True(t, changed)
	assert.NotContains(t, got, "namespace: nullus\n")
	assert.NotContains(t, got, "namespace: nullus #")
	assert.Equal(t, 3, strings.Count(got, "namespace: nullus-demo"))
}

func TestNormalizeManifestNamespace_LeavesExplicitNamespaceUntouched(t *testing.T) {
	manifest := "metadata:\n  namespace: shared-gateway\n"

	got, changed := normalizeManifestNamespace(manifest, "nullus-demo")

	assert.False(t, changed)
	assert.Equal(t, manifest, got)
}
