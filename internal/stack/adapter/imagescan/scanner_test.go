package imagescan

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const nodesJSON = `{"items":[
 {"metadata":{"name":"worker"},"status":{"nodeInfo":{"architecture":"arm64"}}},
 {"metadata":{"name":"cp"},"status":{"nodeInfo":{"architecture":"amd64"}}}
]}`

// kind 스택에서 실제로 본 파드 모양을 줄였다(containerd 는 imageID 에 저장소@digest 를 싣는다).
const podsJSON = `{"items":[
 {"metadata":{"name":"argo-cd-argocd-server-7b8457988-abcde","labels":{"app.kubernetes.io/instance":"argo-cd"},
   "ownerReferences":[{"kind":"ReplicaSet","name":"argo-cd-argocd-server-7b8457988"}]},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"quay.io/argoproj/argocd:v2.13.3","imageID":"quay.io/argoproj/argocd@sha256:aaa"}]}},
 {"metadata":{"name":"argo-cd-argocd-repo-server-8489958df8-z4p69","labels":{"app.kubernetes.io/instance":"argo-cd"},
   "ownerReferences":[{"kind":"ReplicaSet","name":"argo-cd-argocd-repo-server-8489958df8"}]},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"quay.io/argoproj/argocd:v2.13.3","imageID":"quay.io/argoproj/argocd@sha256:aaa"}]}},
 {"metadata":{"name":"nullus-postgresql-0","labels":{"release":"nullus-postgresql"},
   "ownerReferences":[{"kind":"StatefulSet","name":"nullus-postgresql"}]},
  "spec":{"nodeName":"cp"},
  "status":{"phase":"Running",
   "initContainerStatuses":[{"image":"docker.io/bitnami/os-shell:12","imageID":"docker-pullable://docker.io/bitnami/os-shell@sha256:bbb"}],
   "containerStatuses":[{"image":"docker.io/bitnami/postgresql:16.4.0","imageID":"docker.io/bitnami/postgresql@sha256:ccc"}]}},
 {"metadata":{"name":"harbor-provision-8bpnj","ownerReferences":[{"kind":"Job","name":"harbor-provision"}]},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Succeeded","containerStatuses":[{"image":"curlimages/curl:8.11.1","imageID":"docker.io/curlimages/curl@sha256:ddd"}]}},
 {"metadata":{"name":"local-build","labels":{}},"spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"app:dev","imageID":"sha256:eee"}]}},
 {"metadata":{"name":"nullus-image-scan-xyz","labels":{"app.kubernetes.io/managed-by":"nullus-image-scan"}},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"aquasec/trivy:0.74.0","imageID":"docker.io/aquasec/trivy@sha256:fff"}]}}
]}`

func TestInstalledImagesFromPods(t *testing.T) {
	arch, err := nodeArchitectures([]byte(nodesJSON))
	require.NoError(t, err)

	images, err := installedImagesFromPods([]byte(podsJSON), arch)
	require.NoError(t, err)

	// 끝난 파드(Job), digest 가 없는 로컬 이미지, 스캔 Job 자신은 빠진다.
	require.Len(t, images, 3)
	assert.Equal(t, installedImage{
		Ref: "docker.io/bitnami/os-shell@sha256:bbb", Digest: "sha256:bbb", Image: "docker.io/bitnami/os-shell:12",
		Platform: "linux/amd64", Release: "nullus-postgresql", Workloads: []string{"nullus-postgresql"},
	}, images[0])
	assert.Equal(t, "docker.io/bitnami/postgresql@sha256:ccc", images[1].Ref)
	// 같은 digest 는 한 번만 스캔하고, 쓰는 워크로드를 모은다.
	assert.Equal(t, installedImage{
		Ref: "quay.io/argoproj/argocd@sha256:aaa", Digest: "sha256:aaa", Image: "quay.io/argoproj/argocd:v2.13.3",
		Platform: "linux/arm64", Release: "argo-cd",
		Workloads: []string{"argo-cd-argocd-repo-server", "argo-cd-argocd-server"},
	}, images[2])
}

func TestParseScanLog(t *testing.T) {
	log := strings.Join([]string{
		"@@VERSION 0.74.0",
		"@@IMAGE quay.io/argoproj/argocd@sha256:aaa",
		"@@VULNS HIGH+ CRITICAL MEDIUM+ LOW UNKNOWN ",
		"@@IMAGE docker.io/bitnami/postgresql@sha256:ccc",
		"@@VULNS",
		"@@IMAGE docker.io/bitnami/os-shell@sha256:bbb",
		"@@ERROR MANIFEST_UNKNOWN: manifest unknown",
		"@@IMAGE docker.io/library/redis@sha256:ggg",
	}, "\n")

	version, outcomes := parseScanLog(log)

	assert.Equal(t, "0.74.0", version)
	argocd := outcomes["quay.io/argoproj/argocd@sha256:aaa"]
	require.NotNil(t, argocd)
	assert.Equal(t, shareddomain.SeverityCounts{Critical: 1, High: 1, Medium: 1, Low: 1, Unknown: 1}, argocd.all)
	assert.Equal(t, shareddomain.SeverityCounts{High: 1, Medium: 1}, argocd.fixable)
	// 취약점이 없는 이미지는 0건이다 — 결과가 없는 것과 다르다.
	assert.True(t, outcomes["docker.io/bitnami/postgresql@sha256:ccc"].done)
	assert.Equal(t, "MANIFEST_UNKNOWN: manifest unknown", outcomes["docker.io/bitnami/os-shell@sha256:bbb"].err)
	// Job 이 끝까지 가지 못한 이미지는 결과가 없다.
	assert.False(t, outcomes["docker.io/library/redis@sha256:ggg"].done)
}

// 이미지 참조는 셸 스크립트에 들어간다. 파드 상태에서 온 값이라도 모양을 확인한다.
func TestScanJobManifest_ExcludesUnsafeRefs(t *testing.T) {
	manifest := scanJobManifest("harbor-e2e", "aquasec/trivy:0.74.0", "http://trivy.harbor-e2e.svc.cluster.local:4954",
		[]installedImage{
			{Ref: "quay.io/argoproj/argocd@sha256:aaa", Platform: "linux/arm64"},
			{Ref: "evil@sha256:x; rm -rf /", Platform: "linux/arm64"},
			{Ref: "docker.io/library/redis@sha256:ggg", Platform: "linux/$(id)"},
		}, time.Hour)

	assert.Contains(t, manifest, "namespace: harbor-e2e")
	assert.Contains(t, manifest, "image: aquasec/trivy:0.74.0")
	assert.Contains(t, manifest, `"http://trivy.harbor-e2e.svc.cluster.local:4954"`)
	assert.Contains(t, manifest, "linux/arm64 quay.io/argoproj/argocd@sha256:aaa")
	assert.Contains(t, manifest, "activeDeadlineSeconds: 3600")
	assert.NotContains(t, manifest, "rm -rf")
	assert.NotContains(t, manifest, "$(id)")
}

type fakeKubectl struct {
	mu       sync.Mutex
	calls    []string
	manifest string
	outputs  map[string]string
}

func (f *fakeKubectl) run(_ context.Context, _ []byte, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	joined := strings.Join(args, " ")
	f.calls = append(f.calls, joined)
	if args[0] == "apply" {
		raw, err := os.ReadFile(args[len(args)-1])
		if err != nil {
			return "", err
		}
		f.manifest = string(raw)
	}
	for prefix, out := range f.outputs {
		if strings.HasPrefix(joined, prefix) {
			return out, nil
		}
	}
	return "", nil
}

func TestScanner_ScanInstalledImages(t *testing.T) {
	now := time.Date(2026, 9, 14, 1, 2, 3, 0, time.UTC)
	kubectl := &fakeKubectl{outputs: map[string]string{
		"get nodes": nodesJSON,
		"get pods":  podsJSON,
		"get job":   "1 ",
		"logs":      "@@VERSION 0.74.0\n@@IMAGE quay.io/argoproj/argocd@sha256:aaa\n@@VULNS HIGH+ HIGH\n@@IMAGE docker.io/bitnami/postgresql@sha256:ccc\n@@ERROR timeout\n",
		"exec":      `{"Version":2,"UpdatedAt":"2026-09-13T07:13:14.295197098Z"}`,
	}}
	s := NewScanner(WithKubectl(kubectl.run), WithClock(func() time.Time { return now }), WithPollInterval(time.Millisecond))

	scans, err := s.ScanInstalledImages(context.Background(), []byte("kc"), "harbor-e2e")
	require.NoError(t, err)

	require.Len(t, scans, 3)
	byDigest := map[string]domain.StackImageScan{}
	for _, sc := range scans {
		byDigest[sc.ImageDigest] = sc
	}
	argocd := byDigest["sha256:aaa"]
	assert.Equal(t, domain.ImageScanStatusScanned, argocd.Status)
	assert.Equal(t, &shareddomain.SeverityCounts{High: 2}, argocd.Counts)
	assert.Equal(t, &shareddomain.SeverityCounts{High: 1}, argocd.FixableCounts)
	assert.Equal(t, "0.74.0", argocd.ScannerVersion)
	require.NotNil(t, argocd.DBUpdatedAt)
	assert.Equal(t, 2026, argocd.DBUpdatedAt.Year())
	assert.Equal(t, now, argocd.ScannedAt)

	assert.Equal(t, domain.ImageScanStatusFailed, byDigest["sha256:ccc"].Status)
	assert.Equal(t, "timeout", byDigest["sha256:ccc"].Error)
	assert.Nil(t, byDigest["sha256:ccc"].Counts)
	// 로그에 없는 이미지도 빠뜨리지 않고 실패로 남긴다.
	assert.Equal(t, domain.ImageScanStatusFailed, byDigest["sha256:bbb"].Status)

	assert.Contains(t, kubectl.manifest, "linux/arm64 quay.io/argoproj/argocd@sha256:aaa")
	// 이전 Job 을 지우고 만들고, 끝나면 치운다.
	var deletes int
	for _, c := range kubectl.calls {
		if strings.HasPrefix(c, "delete job "+jobName) {
			deletes++
		}
	}
	assert.Equal(t, 2, deletes)
}

func TestScanner_FailsWhenNoRunningImages(t *testing.T) {
	kubectl := &fakeKubectl{outputs: map[string]string{"get nodes": nodesJSON, "get pods": `{"items":[]}`}}
	_, err := NewScanner(WithKubectl(kubectl.run)).ScanInstalledImages(context.Background(), []byte("kc"), "ns")
	assert.Error(t, err)
}

func TestScanner_FailsWhenJobFails(t *testing.T) {
	kubectl := &fakeKubectl{outputs: map[string]string{"get nodes": nodesJSON, "get pods": podsJSON, "get job": " 1"}}
	_, err := NewScanner(WithKubectl(kubectl.run), WithPollInterval(time.Millisecond)).
		ScanInstalledImages(context.Background(), []byte("kc"), "harbor-e2e")
	assert.Error(t, err)
}
