package imagescan

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
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

// vulnLog 는 스캔 Job 이 이미지 하나에 대해 찍는 결과 줄이다(탭 구분 줄 → gzip → base64).
func vulnLog(t *testing.T, lines ...string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte(strings.Join(lines, "\n")))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return "@@VULNS_GZ " + base64.StdEncoding.EncodeToString(buf.Bytes())
}

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
		vulnLog(t,
			"HIGH\tCVE-1\topenssl\t3.0.1\t3.0.2\tos-pkgs\tubuntu 22.04\thttps://avd.aquasec.com/nvd/cve-1",
			"CRITICAL\tCVE-2\tlibc\t2.35\t\tos-pkgs\tubuntu 22.04\t",
			"MEDIUM\tGHSA-3\tgolang.org/x/net\t0.1\t0.2\tlang-pkgs\tusr/local/bin/argocd\t",
			"LOW\tCVE-4\tzlib\t1.2\t\tos-pkgs\tubuntu 22.04\t",
			"UNKNOWN\tCVE-5\tfoo\t1\t\tos-pkgs\tubuntu 22.04\t",
		),
		"@@IMAGE docker.io/bitnami/postgresql@sha256:ccc",
		"@@VULNS_GZ ",
		"@@IMAGE docker.io/bitnami/os-shell@sha256:bbb",
		"@@ERROR MANIFEST_UNKNOWN: manifest unknown",
		"@@IMAGE docker.io/library/redis@sha256:ggg",
		"@@IMAGE docker.io/library/broken@sha256:hhh",
		"@@VULNS_GZ not-base64!!",
	}, "\n")

	version, outcomes := parseScanLog(log)

	assert.Equal(t, "0.74.0", version)
	argocd := outcomes["quay.io/argoproj/argocd@sha256:aaa"]
	require.NotNil(t, argocd)
	assert.Equal(t, shareddomain.SeverityCounts{Critical: 1, High: 1, Medium: 1, Low: 1, Unknown: 1}, argocd.all)
	assert.Equal(t, shareddomain.SeverityCounts{High: 1, Medium: 1}, argocd.fixable)
	require.Len(t, argocd.vulns, 5)
	// Class 로 베이스 이미지 OS 패키지와 앱 의존성이 갈린다.
	assert.Equal(t, shareddomain.VulnerabilityClassOS, argocd.vulns[0].Class)
	assert.Equal(t, shareddomain.VulnerabilityClassLibrary, argocd.vulns[2].Class)
	// 취약점이 없는 이미지는 0건이다 — 결과가 없는 것과 다르다.
	postgres := outcomes["docker.io/bitnami/postgresql@sha256:ccc"]
	assert.True(t, postgres.done)
	assert.Empty(t, postgres.err)
	assert.NotNil(t, postgres.vulns)
	assert.Empty(t, postgres.vulns)
	assert.Equal(t, "MANIFEST_UNKNOWN: manifest unknown", outcomes["docker.io/bitnami/os-shell@sha256:bbb"].err)
	// Job 이 끝까지 가지 못한 이미지는 결과가 없다.
	assert.False(t, outcomes["docker.io/library/redis@sha256:ggg"].done)
	// 결과를 풀지 못하면 실패로 남긴다 — 0건으로 읽히면 안 된다.
	assert.NotEmpty(t, outcomes["docker.io/library/broken@sha256:hhh"].err)
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
	// 취약점 목록은 로그 상한(10Mi)을 넘지 않게 압축해 찍는다.
	assert.Contains(t, manifest, "gzip -c </tmp/out | base64 -w 0")
	assert.NotContains(t, manifest, "rm -rf")
	assert.NotContains(t, manifest, "$(id)")
}

// 설치 직후 스캔에서 레이어를 받다 끊기는 일시적 실패가 났다(kind 실측: argocd 의
// "failed to extract the archive: unexpected EOF", gitlab-runner 의 파일 열기 실패).
// 같은 이미지를 다시 스캔하면 통과했지만, Job 은 한 번만 시도해 다음 주기(24h)까지
// 실패로 남았다. 스크립트를 가짜 trivy 로 실제로 돌려 재시도를 확인한다.
func TestScanScript_RetriesTransientScanFailures(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh 가 없다")
	}
	bin, state := t.TempDir(), t.TempDir()
	writeExec := func(name, body string) {
		require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755))
	}
	// 마지막 인자가 이미지 참조다. 호출 횟수를 이미지별로 센다.
	writeExec("trivy", `#!/bin/sh
case "$1" in --version) echo "Version: 0.74.0"; exit 0;; esac
for a in "$@"; do ref="$a"; done
key=$(echo "$ref" | tr '/:@' '___')
n=$(cat "$STATE/$key" 2>/dev/null || echo 0); n=$((n + 1)); echo "$n" > "$STATE/$key"
case "$ref" in
  *flaky*) if [ "$n" -lt 2 ]; then echo "failed to extract the archive: unexpected EOF" >&2; exit 1; fi ;;
  *broken*) echo "MANIFEST_UNKNOWN: manifest unknown" >&2; exit 1 ;;
esac
printf 'HIGH\tCVE-1\topenssl\t3.0.1\t3.0.2\tos-pkgs\talpine 3.19\t\n'
`)
	// 컨테이너의 base64 -w 0 과 같은 한 줄 출력을 흉내 낸다(macOS base64 는 -w 가 없다).
	writeExec("base64", `#!/bin/sh
real=$(PATH=/usr/bin:/bin command -v base64)
"$real" | tr -d '\n'
`)

	targets := "linux/arm64 quay.io/x/flaky@sha256:aaa\n" +
		"linux/arm64 quay.io/x/broken@sha256:bbb\n" +
		"linux/arm64 quay.io/x/ok@sha256:ccc\n"
	cmd := exec.Command("sh", "-c", scanScript(targets))
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"STATE="+state,
		"NULLUS_SCAN_RETRY_DELAY=0",
		"NULLUS_TRIVY_SERVER=http://trivy:4954",
		"NULLUS_TRIVY_TEMPLATE=unused",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	calls := func(ref string) string {
		raw, _ := os.ReadFile(filepath.Join(state, strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(ref)))
		return strings.TrimSpace(string(raw))
	}
	version, outcomes := parseScanLog(string(out))
	assert.Equal(t, "0.74.0", version)
	assert.Contains(t, string(out), "@@DONE")

	flaky := outcomes["quay.io/x/flaky@sha256:aaa"]
	require.NotNil(t, flaky)
	assert.True(t, flaky.done)
	assert.Empty(t, flaky.err, "한 번 끊긴 이미지는 다시 스캔해 결과를 남긴다")
	assert.Len(t, flaky.vulns, 1)
	assert.Equal(t, "2", calls("quay.io/x/flaky@sha256:aaa"))

	broken := outcomes["quay.io/x/broken@sha256:bbb"]
	require.NotNil(t, broken)
	assert.Contains(t, broken.err, "MANIFEST_UNKNOWN", "마지막 시도의 오류를 남긴다")
	assert.Contains(t, broken.err, "3회", "몇 번 시도했는지 남긴다")
	assert.Equal(t, "3", calls("quay.io/x/broken@sha256:bbb"))

	ok := outcomes["quay.io/x/ok@sha256:ccc"]
	require.NotNil(t, ok)
	assert.Len(t, ok.vulns, 1)
	assert.Equal(t, "1", calls("quay.io/x/ok@sha256:ccc"), "성공한 이미지는 다시 스캔하지 않는다")
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
		"logs": strings.Join([]string{
			"@@VERSION 0.74.0",
			"@@IMAGE quay.io/argoproj/argocd@sha256:aaa",
			vulnLog(t,
				"HIGH\tCVE-1\topenssl\t3.0.1\t3.0.2\tos-pkgs\tubuntu 22.04\t",
				"HIGH\tCVE-2\tlibc\t2.35\t\tos-pkgs\tubuntu 22.04\t"),
			"@@IMAGE docker.io/bitnami/postgresql@sha256:ccc",
			"@@ERROR timeout",
		}, "\n"),
		"exec": `{"Version":2,"UpdatedAt":"2026-09-13T07:13:14.295197098Z"}`,
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
	// 목록도 함께 남긴다. 스캔에 성공한 이미지만 목록이 기록된 것으로 본다.
	assert.True(t, argocd.VulnerabilitiesRecorded)
	require.Len(t, argocd.Vulnerabilities, 2)
	assert.Equal(t, "CVE-1", argocd.Vulnerabilities[0].ID)
	assert.Equal(t, "0.74.0", argocd.ScannerVersion)
	require.NotNil(t, argocd.DBUpdatedAt)
	assert.Equal(t, 2026, argocd.DBUpdatedAt.Year())
	assert.Equal(t, now, argocd.ScannedAt)

	assert.Equal(t, domain.ImageScanStatusFailed, byDigest["sha256:ccc"].Status)
	assert.Equal(t, "timeout", byDigest["sha256:ccc"].Error)
	assert.Nil(t, byDigest["sha256:ccc"].Counts)
	assert.False(t, byDigest["sha256:ccc"].VulnerabilitiesRecorded)
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
