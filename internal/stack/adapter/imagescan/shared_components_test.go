package imagescan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const certManagerPodsJSON = `{"items":[
 {"metadata":{"name":"cert-manager-6db94c5b99-lbz9t","labels":{"app.kubernetes.io/instance":"cert-manager"},
   "ownerReferences":[{"kind":"ReplicaSet","name":"cert-manager-6db94c5b99"}]},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"quay.io/jetstack/cert-manager-controller:v1.16.3","imageID":"quay.io/jetstack/cert-manager-controller@sha256:c0ffee"}]}},
 {"metadata":{"name":"argo-cd-argocd-server-dup","labels":{"app.kubernetes.io/instance":"cert-manager"}},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"quay.io/argoproj/argocd:v2.13.3","imageID":"quay.io/argoproj/argocd@sha256:aaa"}]}}
]}`

// 스택이 쓰는 공용 구성요소(스택 네임스페이스 밖)도 함께 스캔하고, 공용이라고 남긴다.
// 읽지 못한 구성요소는 건너뛴다 — 공용 구성요소 때문에 스택 스캔이 멈추면 안 된다.
func TestScanner_IncludesSharedComponents(t *testing.T) {
	kubectl := &fakeKubectl{outputs: map[string]string{
		"get nodes":                nodesJSON,
		"get pods -n harbor-e2e":   podsJSON,
		"get pods -n cert-manager": certManagerPodsJSON,
		"get job":                  "1 ",
		"logs":                     "@@VERSION 0.74.0",
	}, errs: map[string]error{"get pods -n kube-system": errors.New("forbidden")}}
	s := NewScanner(WithKubectl(kubectl.run), WithPollInterval(time.Millisecond))

	scans, err := s.ScanInstalledImages(context.Background(), []byte("kc"), "harbor-e2e", []domain.SharedComponent{
		{Release: "cert-manager", Namespace: "cert-manager"},
		{Release: "metrics-server", Namespace: "kube-system"},
	})
	require.NoError(t, err)

	byDigest := map[string]domain.StackImageScan{}
	for _, sc := range scans {
		byDigest[sc.ImageDigest] = sc
	}
	cm := byDigest["sha256:c0ffee"]
	assert.True(t, cm.Shared)
	assert.Equal(t, "cert-manager", cm.Namespace)
	assert.Equal(t, "cert-manager", cm.Release)
	// 스택 네임스페이스에도 있는 이미지는 스택 것으로 남긴다.
	assert.False(t, byDigest["sha256:aaa"].Shared)
	assert.Equal(t, "harbor-e2e", byDigest["sha256:aaa"].Namespace)
	assert.Len(t, scans, 4, "스택 이미지 셋 + 공용 하나, 중복 없이")

	assert.Contains(t, kubectl.manifest, "quay.io/jetstack/cert-manager-controller@sha256:c0ffee")
	assert.Contains(t, kubectl.calls, "get pods -n cert-manager -l app.kubernetes.io/instance=cert-manager -o json")
}
