package imagescan

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 에어갭 노드는 containerd 미러로 이미지를 받는다 — 파드의 imageID 는 원래 레지스트리 이름이다.
// 스캔 Job 의 trivy client 는 그 이름으로 받을 수 없으니, 번들이 올린 경로(호스트만 바꾼 것)로 바꾼다
// (airgap/scripts/12-push-to-registry.sh 의 compute_target).
func TestMirrorRef(t *testing.T) {
	const host = "kind-registry:5000"
	cases := map[string]string{
		"docker.io/library/redis@sha256:abc":                           "kind-registry:5000/library/redis@sha256:abc",
		"quay.io/jetstack/cert-manager-controller@sha256:abc":          "kind-registry:5000/jetstack/cert-manager-controller@sha256:abc",
		"registry.gitlab.com/gitlab-org/build/cng/webservice@sha256:a": "kind-registry:5000/gitlab-org/build/cng/webservice@sha256:a",
		"ghcr.io/external-secrets/external-secrets@sha256:abc":         "kind-registry:5000/external-secrets/external-secrets@sha256:abc",
		"kind-registry:5000/library/redis@sha256:abc":                  "kind-registry:5000/library/redis@sha256:abc",
		"localhost:5001/library/redis@sha256:abc":                      "kind-registry:5000/library/redis@sha256:abc",
		"redis@sha256:abc":         "kind-registry:5000/library/redis@sha256:abc",
		"bitnami/redis@sha256:abc": "kind-registry:5000/bitnami/redis@sha256:abc",
	}
	for in, want := range cases {
		assert.Equal(t, want, mirrorRef(in, host), in)
	}
}

func TestScanner_AirgapScansFromMirror(t *testing.T) {
	kubectl := &fakeKubectl{outputs: map[string]string{
		"get nodes": nodesJSON,
		"get pods":  podsJSON,
		"get job":   "1 ",
		"logs": "@@VERSION 0.74.0\n" +
			"@@IMAGE kind-registry:5000/argoproj/argocd@sha256:aaa\n" +
			vulnLog(t, "HIGH\tCVE-1\topenssl\t3.0.1\t3.0.2\tos-pkgs\tubuntu 22.04\t"),
	}}
	s := NewScanner(WithKubectl(kubectl.run), WithPollInterval(time.Millisecond), WithMirrorRegistry("kind-registry:5000"))

	scans, err := s.ScanInstalledImages(context.Background(), []byte("kc"), "harbor-e2e", nil)
	require.NoError(t, err)

	assert.Contains(t, kubectl.manifest, "linux/arm64 kind-registry:5000/argoproj/argocd@sha256:aaa")
	assert.NotContains(t, kubectl.manifest, " quay.io/argoproj/argocd@sha256:aaa")
	// 내부 레지스트리는 plain HTTP 다. Java DB 도 번들이 올린 미러에서 받는다.
	assert.Contains(t, kubectl.manifest, "TRIVY_INSECURE")
	assert.Contains(t, kubectl.manifest, "kind-registry:5000/aquasecurity/trivy-java-db")

	byDigest := map[string]domain.StackImageScan{}
	for _, sc := range scans {
		byDigest[sc.ImageDigest] = sc
	}
	argocd := byDigest["sha256:aaa"]
	assert.Equal(t, domain.ImageScanStatusScanned, argocd.Status, "결과는 원래 이미지에 붙는다")
	assert.Equal(t, "quay.io/argoproj/argocd:v2.13.3", argocd.Image, "보여 주는 이름은 원래 것이다")
}

// 에어갭이 아니면 참조와 환경을 건드리지 않는다.
func TestScanner_NoMirrorByDefault(t *testing.T) {
	kubectl := &fakeKubectl{outputs: map[string]string{
		"get nodes": nodesJSON, "get pods": podsJSON, "get job": "1 ", "logs": "@@VERSION 0.74.0",
	}}
	_, err := NewScanner(WithKubectl(kubectl.run), WithPollInterval(time.Millisecond)).
		ScanInstalledImages(context.Background(), []byte("kc"), "harbor-e2e", nil)
	require.NoError(t, err)
	assert.Contains(t, kubectl.manifest, "linux/arm64 quay.io/argoproj/argocd@sha256:aaa")
	assert.NotContains(t, kubectl.manifest, "TRIVY_INSECURE")
}
