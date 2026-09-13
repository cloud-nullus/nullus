// Package imagescan 은 스택이 설치한 OSS 이미지를 스택의 Trivy 서버로 스캔한다.
//
// 플랫폼은 클러스터 밖에서 돈다. 이미지를 받아 분석하는 일은 스택 네임스페이스의
// Job 이 하고, 플랫폼은 그 결과를 로그로 받는다.
package imagescan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	sharedkubeconfig "github.com/cloud-nullus/draft/internal/shared/kubeconfig"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const (
	jobName = "nullus-image-scan"
	// managedByValue 로 스캔 Job 자신의 파드를 스캔 대상에서 뺀다.
	managedByValue = "nullus-image-scan"
	// dbMetadataPath 는 Trivy 차트(0.26.x) 서버가 취약점 DB 메타를 두는 곳이다.
	// 서버 모드 client 는 템플릿 출력에 DB 시각을 싣지 않으므로 서버에서 읽는다.
	dbMetadataPath = "/home/scanner/.cache/trivy/db/metadata.json"

	defaultJobTimeout   = 60 * time.Minute
	defaultPollInterval = 5 * time.Second
)

// vulnTemplate 은 취약점 하나를 "심각도[+]" 토큰 하나로 찍는다(+ 는 수정본 있음).
//
// JSON 리포트를 로그로 받으면 큰 이미지 몇 개만으로 kubelet 의 컨테이너 로그 상한
// (기본 10Mi)을 넘어 앞부분이 잘린다. 건수에 필요한 것만 남긴다.
const vulnTemplate = `{{- range . }}{{- range .Vulnerabilities }}{{ .Severity }}{{ if .FixedVersion }}+{{ end }} {{ end }}{{- end }}`

// KubectlFunc 는 kubeconfig 로 kubectl 을 실행한다.
type KubectlFunc func(ctx context.Context, kubeconfig []byte, args ...string) (string, error)

// Scanner 는 port.InstalledImageScanner 구현이다.
type Scanner struct {
	kubectl      KubectlFunc
	image        string
	jobTimeout   time.Duration
	pollInterval time.Duration
	now          func() time.Time
}

// Option 은 Scanner 설정이다.
type Option func(*Scanner)

// WithKubectl 은 kubectl 실행기를 바꾼다(테스트용).
func WithKubectl(fn KubectlFunc) Option { return func(s *Scanner) { s.kubectl = fn } }

// WithClock 은 스캔 시각의 출처를 바꾼다(테스트용).
func WithClock(fn func() time.Time) Option { return func(s *Scanner) { s.now = fn } }

// WithPollInterval 은 Job 완료를 확인하는 간격을 바꾼다(테스트용).
func WithPollInterval(d time.Duration) Option { return func(s *Scanner) { s.pollInterval = d } }

// NewScanner 는 스캐너를 만든다.
func NewScanner(opts ...Option) *Scanner {
	s := &Scanner{
		kubectl:      runKubectl,
		image:        shareddomain.TrivyClientImage,
		jobTimeout:   defaultJobTimeout,
		pollInterval: defaultPollInterval,
		now:          time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ScanInstalledImages 는 네임스페이스에서 실행 중인 이미지를 digest 단위로 스캔한다.
func (s *Scanner) ScanInstalledImages(ctx context.Context, kubeconfig []byte, namespace string) ([]domain.StackImageScan, error) {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return nil, fmt.Errorf("스캔할 네임스페이스가 비어 있습니다")
	}

	nodesOut, err := s.kubectl(ctx, kubeconfig, "get", "nodes", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("노드 조회 실패: %w", err)
	}
	arch, err := nodeArchitectures([]byte(nodesOut))
	if err != nil {
		return nil, err
	}
	podsOut, err := s.kubectl(ctx, kubeconfig, "get", "pods", "-n", ns, "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("파드 조회 실패: %w", err)
	}
	images, err := installedImagesFromPods([]byte(podsOut), arch)
	if err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("네임스페이스 %s 에서 digest 가 확정된 실행 중 이미지를 찾지 못했습니다", ns)
	}

	manifest := scanJobManifest(ns, s.image, shareddomain.TrivyServerEndpoint(ns), images, s.jobTimeout)
	path, cleanup, err := writeTemp(manifest)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// 이전 실행이 남아 있으면 Job 은 불변 필드 때문에 갱신되지 않는다.
	_, _ = s.kubectl(ctx, kubeconfig, "delete", "job", jobName, "-n", ns, "--ignore-not-found", "--wait=true")
	if _, err := s.kubectl(ctx, kubeconfig, "apply", "-n", ns, "-f", path); err != nil {
		return nil, fmt.Errorf("스캔 Job 생성 실패: %w", err)
	}
	defer func() {
		_, _ = s.kubectl(context.WithoutCancel(ctx), kubeconfig, "delete", "job", jobName, "-n", ns, "--ignore-not-found")
	}()

	if err := s.waitForJob(ctx, kubeconfig, ns); err != nil {
		return nil, err
	}
	logs, err := s.kubectl(ctx, kubeconfig, "logs", "-n", ns, "job/"+jobName, "--tail=-1")
	if err != nil {
		return nil, fmt.Errorf("스캔 Job 로그 조회 실패: %w", err)
	}
	// DB 시각을 모르면 비워 둔다 — 화면은 모르는 DB 를 낡은 것으로 보인다.
	meta, _ := s.kubectl(ctx, kubeconfig, "exec", "-n", ns, "statefulset/"+shareddomain.TrivyReleaseName,
		"--", "cat", dbMetadataPath)

	version, outcomes := parseScanLog(logs)
	dbUpdatedAt := parseDBUpdatedAt(meta)
	scannedAt := s.now()

	scans := make([]domain.StackImageScan, 0, len(images))
	for _, img := range images {
		scan := domain.StackImageScan{
			Image:          img.Image,
			ImageDigest:    img.Digest,
			Release:        img.Release,
			Workloads:      img.Workloads,
			ScannerVersion: version,
			DBUpdatedAt:    dbUpdatedAt,
			ScannedAt:      scannedAt,
		}
		switch o := outcomes[img.Ref]; {
		case o == nil || !o.done:
			scan.Status = domain.ImageScanStatusFailed
			scan.Error = "스캔 결과가 기록되지 않았습니다"
		case o.err != "":
			scan.Status = domain.ImageScanStatusFailed
			scan.Error = o.err
		default:
			all, fixable := o.all, o.fixable
			scan.Status = domain.ImageScanStatusScanned
			scan.Counts = &all
			scan.FixableCounts = &fixable
		}
		scans = append(scans, scan)
	}
	return scans, nil
}

// waitForJob 은 Job 이 끝날 때까지 기다린다. 실패하거나 시간을 넘기면 오류다.
func (s *Scanner) waitForJob(ctx context.Context, kubeconfig []byte, ns string) error {
	deadline := s.now().Add(s.jobTimeout + time.Minute)
	for {
		out, err := s.kubectl(ctx, kubeconfig, "get", "job", jobName, "-n", ns,
			"-o", "jsonpath={.status.succeeded} {.status.failed}")
		if err == nil {
			succeeded, failed := parseJobCounts(out)
			if succeeded > 0 {
				return nil
			}
			if failed > 0 {
				return fmt.Errorf("스캔 Job 이 실패했습니다")
			}
		}
		if s.now().After(deadline) {
			return fmt.Errorf("스캔 Job 이 %s 안에 끝나지 않았습니다", s.jobTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.pollInterval):
		}
	}
}

func parseJobCounts(out string) (succeeded, failed int) {
	parts := strings.SplitN(out, " ", 2)
	succeeded, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	if len(parts) > 1 {
		failed, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	}
	return succeeded, failed
}

type installedImage struct {
	// Ref 는 스캔에 쓰는 저장소@digest 다.
	Ref       string
	Digest    string
	Image     string
	Platform  string
	Release   string
	Workloads []string
}

type containerStatus struct {
	Image   string `json:"image"`
	ImageID string `json:"imageID"`
}

type podList struct {
	Items []struct {
		Metadata struct {
			Name            string            `json:"name"`
			Labels          map[string]string `json:"labels"`
			OwnerReferences []struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"ownerReferences"`
		} `json:"metadata"`
		Spec struct {
			NodeName string `json:"nodeName"`
		} `json:"spec"`
		Status struct {
			Phase                 string            `json:"phase"`
			ContainerStatuses     []containerStatus `json:"containerStatuses"`
			InitContainerStatuses []containerStatus `json:"initContainerStatuses"`
		} `json:"status"`
	} `json:"items"`
}

// nodeArchitectures 는 노드 이름 → 아키텍처다.
func nodeArchitectures(raw []byte) (map[string]string, error) {
	var nodes struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				NodeInfo struct {
					Architecture string `json:"architecture"`
				} `json:"nodeInfo"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, fmt.Errorf("노드 목록을 해석하지 못했습니다: %w", err)
	}
	out := make(map[string]string, len(nodes.Items))
	for _, n := range nodes.Items {
		out[n.Metadata.Name] = strings.TrimSpace(n.Status.NodeInfo.Architecture)
	}
	return out, nil
}

// installedImagesFromPods 는 실행 중인 파드의 이미지를 digest 단위로 모은다.
//
// 이미지는 파드가 실제로 받은 digest 로 스캔한다 — 태그는 움직인다. 멀티아치 이미지는
// 노드 아키텍처의 것을 스캔해야 실제로 돌고 있는 것과 같다(Trivy 기본값은 amd64 다).
// 끝난 파드(Job)는 설치된 것이 아니므로, digest 가 없는 로컬 이미지는 받을 수 없으므로 뺀다.
func installedImagesFromPods(raw []byte, nodeArch map[string]string) ([]installedImage, error) {
	var pods podList
	if err := json.Unmarshal(raw, &pods); err != nil {
		return nil, fmt.Errorf("파드 목록을 해석하지 못했습니다: %w", err)
	}

	byDigest := map[string]*installedImage{}
	for _, p := range pods.Items {
		if p.Status.Phase != "Running" || p.Metadata.Labels["app.kubernetes.io/managed-by"] == managedByValue {
			continue
		}
		platform := "linux/amd64"
		if a := nodeArch[p.Spec.NodeName]; a != "" {
			platform = "linux/" + a
		}
		workload := workloadName(p.Metadata.Name, p.Metadata.OwnerReferences)
		release := firstNonEmpty(p.Metadata.Labels["app.kubernetes.io/instance"], p.Metadata.Labels["release"],
			p.Metadata.Labels["app.kubernetes.io/name"])

		statuses := append(append([]containerStatus(nil), p.Status.InitContainerStatuses...), p.Status.ContainerStatuses...)
		for _, cs := range statuses {
			ref := strings.TrimPrefix(strings.TrimSpace(cs.ImageID), "docker-pullable://")
			at := strings.Index(ref, "@sha256:")
			if at <= 0 {
				continue
			}
			digest := ref[at+1:]
			img, ok := byDigest[digest]
			if !ok {
				img = &installedImage{Ref: ref, Digest: digest, Image: firstNonEmpty(cs.Image, ref),
					Platform: platform, Release: release}
				byDigest[digest] = img
			}
			if !contains(img.Workloads, workload) {
				img.Workloads = append(img.Workloads, workload)
			}
		}
	}

	out := make([]installedImage, 0, len(byDigest))
	for _, img := range byDigest {
		sort.Strings(img.Workloads)
		out = append(out, *img)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out, nil
}

// workloadName 은 파드를 만든 워크로드 이름이다. ReplicaSet 은 Deployment 이름으로 줄인다.
func workloadName(pod string, owners []struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}) string {
	if len(owners) == 0 {
		return pod
	}
	name := owners[0].Name
	if owners[0].Kind == "ReplicaSet" {
		if i := strings.LastIndex(name, "-"); i > 0 {
			return name[:i]
		}
	}
	return name
}

var (
	safeRef      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:-]*@sha256:[a-f0-9]+$`)
	safePlatform = regexp.MustCompile(`^linux/[a-z0-9]+$`)
)

// scanJobManifest 는 이미지를 하나씩 스캔해 결과를 로그로 남기는 Job 이다.
//
// 이미지 참조는 셸 스크립트에 들어간다. 파드 상태에서 온 값이라도 모양이 맞지 않으면
// 넣지 않는다 — 그 이미지는 결과가 없어 실패로 남는다.
func scanJobManifest(namespace, image, server string, images []installedImage, timeout time.Duration) string {
	var targets strings.Builder
	for _, img := range images {
		if !safeRef.MatchString(img.Ref) || !safePlatform.MatchString(img.Platform) {
			continue
		}
		fmt.Fprintf(&targets, "%s %s\n", img.Platform, img.Ref)
	}

	script := `set +e
echo "@@VERSION $(trivy --version 2>/dev/null | awk 'NR==1{print $2}')"
while read -r platform ref; do
  [ -n "$ref" ] || continue
  echo "@@IMAGE $ref"
  if trivy image --server "$NULLUS_TRIVY_SERVER" --scanners vuln --platform "$platform" --quiet --timeout 15m --cache-dir /tmp/trivy --format template --template "$NULLUS_TRIVY_TEMPLATE" "$ref" >/tmp/out 2>/tmp/err; then
    echo "@@VULNS $(tr '\n' ' ' </tmp/out)"
  else
    echo "@@ERROR $(tail -n 3 /tmp/err | tr '\n' ' ' | cut -c1-400)"
  fi
done <<'TARGETS'
` + targets.String() + `TARGETS
echo "@@DONE"`

	return fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/managed-by: %s
spec:
  backoffLimit: 0
  activeDeadlineSeconds: %d
  ttlSecondsAfterFinished: 3600
  template:
    metadata:
      labels:
        app.kubernetes.io/managed-by: %s
    spec:
      restartPolicy: Never
      containers:
      - name: trivy
        image: %s
        env:
        - name: NULLUS_TRIVY_SERVER
          value: %q
        - name: NULLUS_TRIVY_TEMPLATE
          value: %q
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: "1"
            memory: 2Gi
        command: ["sh", "-c"]
        args:
        - |
%s
`, jobName, namespace, managedByValue, int(timeout.Seconds()), managedByValue, image, server, vulnTemplate,
		indent(script, 10))
}

type scanOutcome struct {
	all     shareddomain.SeverityCounts
	fixable shareddomain.SeverityCounts
	err     string
	done    bool
}

// parseScanLog 는 스캔 Job 의 로그를 이미지별 결과로 읽는다.
func parseScanLog(log string) (string, map[string]*scanOutcome) {
	version := ""
	outcomes := map[string]*scanOutcome{}
	var current *scanOutcome
	for _, line := range strings.Split(log, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "@@VERSION"):
			version = strings.TrimSpace(strings.TrimPrefix(line, "@@VERSION"))
		case strings.HasPrefix(line, "@@IMAGE "):
			current = &scanOutcome{}
			outcomes[strings.TrimSpace(strings.TrimPrefix(line, "@@IMAGE "))] = current
		case strings.HasPrefix(line, "@@VULNS") && current != nil:
			for _, tok := range strings.Fields(strings.TrimPrefix(line, "@@VULNS")) {
				severity := strings.TrimSuffix(tok, "+")
				current.all.Add(severity)
				if strings.HasSuffix(tok, "+") {
					current.fixable.Add(severity)
				}
			}
			current.done = true
		case strings.HasPrefix(line, "@@ERROR") && current != nil:
			current.err = strings.TrimSpace(strings.TrimPrefix(line, "@@ERROR"))
			if current.err == "" {
				current.err = "trivy 실행 실패"
			}
			current.done = true
		}
	}
	return version, outcomes
}

func parseDBUpdatedAt(raw string) *time.Time {
	var meta struct {
		UpdatedAt *time.Time `json:"UpdatedAt"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &meta); err != nil || meta.UpdatedAt == nil || meta.UpdatedAt.IsZero() {
		return nil
	}
	t := meta.UpdatedAt.UTC()
	return &t
}

func writeTemp(content string) (string, func(), error) {
	f, err := os.CreateTemp("", "nullus-image-scan-*.yaml")
	if err != nil {
		return "", func() {}, fmt.Errorf("스캔 Job 매니페스트 임시 파일 생성 실패: %w", err)
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("스캔 Job 매니페스트 기록 실패: %w", err)
	}
	_ = f.Close()
	return f.Name(), cleanup, nil
}

// runKubectl 은 kubeconfig 를 임시 파일로 두고 kubectl 을 실행한다.
//
// 서버 주소가 없는 kubeconfig 는 거부한다 — kubectl 은 오류 없이 localhost:8080 으로
// 폴백해 엉뚱한 곳에 Job 을 만든다.
func runKubectl(ctx context.Context, kubeconfig []byte, args ...string) (string, error) {
	if err := sharedkubeconfig.RequireServer(kubeconfig); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp("", "nullus-image-scan-kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create kubeconfig temp file: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	if _, err := tmp.Write(kubeconfig); err != nil {
		return "", fmt.Errorf("write kubeconfig temp file: %w", err)
	}
	cmd := exec.CommandContext(ctx, "kubectl", append([]string{"--kubeconfig", tmp.Name()}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		return "", fmt.Errorf("kubectl %s failed: %w (%s)", strings.Join(args, " "), err, stderr)
	}
	return string(out), nil
}

func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
