package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/cloud-nullus/draft/internal/backup/port"
)

// ResourceDumper 는 네임스페이스 리소스와 Helm 릴리스를 뜬다 (D1).
//
// kubectl 을 exec 하는 이유: 스택은 Gateway/HTTPRoute 같은 CRD 를 만든다.
// 타입 클라이언트로 알려진 종류만 훑으면 카탈로그가 늘 때마다 백업이
// 뒤처진다 — §1.5 의 PVC 열거와 같은 이유다.
//
// kubectl 은 API 이미지에 이미 있다 (Dockerfile).
type ResourceDumper struct {
	kubeconfig []byte
}

func NewResourceDumper(kubeconfig []byte) *ResourceDumper {
	return &ResourceDumper{kubeconfig: kubeconfig}
}

// dumpKinds 는 뜰 리소스 종류다.
//
// Helm 릴리스는 Secret 에 들어 있으므로(sh.helm.release.v1.*) secret 을 뜨면
// 함께 담긴다 — 릴리스 메타데이터를 따로 다루지 않아도 되는 이유다.
// PVC 가 목록에 없는 것은 의도다.
//
// PVC 는 **볼륨 경로가 소유한다** — 복구는 매니페스트의 크기·StorageClass 로
// EnsurePVC 가 먼저 만들고 그 위에 아카이브를 푼다(§6.1 3~4단계). 리소스
// 덤프에도 PVC 가 들어 있으면 같은 객체를 두 경로가 소유하게 되고, 뒤따르는
// apply 가 `spec.volumeName`(이미 사라진 PV 를 가리킨다) 때문에 "spec is
// immutable" 로 죽는다 — 리허설에서 실제로 그렇게 됐다.
//
// 소유자는 하나여야 한다.
var dumpKinds = []string{
	"deployments", "statefulsets", "daemonsets", "services", "configmaps",
	"secrets", "serviceaccounts", "roles",
	"rolebindings", "ingresses", "gateways.gateway.networking.k8s.io",
	"httproutes.gateway.networking.k8s.io",
}

func (d *ResourceDumper) writeKubeconfig() (string, error) {
	f, err := os.CreateTemp("", "nullus-backup-kubeconfig-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(d.kubeconfig); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), f.Close()
}

func (d *ResourceDumper) run(ctx context.Context, stdin io.Reader, stdout io.Writer, args ...string) error {
	path, err := d.writeKubeconfig()
	if err != nil {
		return fmt.Errorf("kubeconfig 임시 파일: %w", err)
	}
	defer func() { _ = os.Remove(path) }()

	cmd := exec.CommandContext(ctx, "kubectl", append([]string{"--kubeconfig", path}, args...)...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// availableKinds 는 대상 클러스터에 실제로 존재하는 종류만 남긴다.
//
// `--ignore-not-found` 는 없는 **객체**를 다루지 없는 **리소스 타입**은
// 다루지 못한다. Gateway API CRD 가 없는 클러스터에서 `kubectl get
// gateways...` 는 exit 1 로 죽고, 그러면 네임스페이스 덤프 전체가 실패한다 —
// 리허설에서 실제로 그렇게 됐다. 스택 구성에 따라 Gateway API 가 없을 수
// 있으므로, 무엇이 있는지 먼저 물어본다.
func (d *ResourceDumper) availableKinds(ctx context.Context) ([]string, error) {
	var buf strings.Builder
	if err := d.run(ctx, nil, &buf, "api-resources", "--verbs=list", "--namespaced=true", "-o", "name"); err != nil {
		return nil, fmt.Errorf("리소스 종류 조회: %w", err)
	}
	return filterAvailable(dumpKinds, buf.String()), nil
}

// filterAvailable 은 `kubectl api-resources -o name` 출력과 대조해 실제로
// 존재하는 종류만 남긴다.
//
// kubectl 호출과 분리해 둔 이유: 이 판정이 틀리면 네임스페이스 덤프가 통째로
// 실패하거나(없는 종류를 조회) 조용히 빠진다(있는 종류를 누락). 둘 다 비싸고,
// 클러스터 없이 검증할 수 있어야 한다.
func filterAvailable(want []string, apiResourcesOutput string) []string {
	present := map[string]bool{}
	for _, line := range strings.Split(apiResourcesOutput, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		present[name] = true
		// `deployments.apps` 처럼 그룹이 붙어 오므로 짧은 이름도 넣는다.
		if i := strings.Index(name, "."); i > 0 {
			present[name[:i]] = true
		}
	}

	out := make([]string, 0, len(want))
	for _, k := range want {
		if present[k] {
			out = append(out, k)
			continue
		}
		if i := strings.Index(k, "."); i > 0 && present[k[:i]] {
			out = append(out, k)
		}
	}
	return out
}

func (d *ResourceDumper) Dump(ctx context.Context, namespace string, out io.Writer) (int64, error) {
	kinds, err := d.availableKinds(ctx)
	if err != nil {
		return 0, err
	}
	if len(kinds) == 0 {
		return 0, fmt.Errorf("덤프할 리소스 종류를 하나도 찾지 못했습니다 — 클러스터 접근을 확인하세요")
	}

	var raw bytes.Buffer
	args := []string{
		"get", strings.Join(kinds, ","),
		"-n", namespace, "-o", "json", "--ignore-not-found",
	}
	if err := d.run(ctx, nil, &raw, args...); err != nil {
		return 0, err
	}
	if raw.Len() == 0 {
		return 0, nil
	}

	cleaned, err := sanitize(raw.Bytes())
	if err != nil {
		return 0, err
	}
	counter := &countingWriter{w: out}
	n, err := counter.Write(cleaned)
	_ = n
	return counter.n, err
}

// sanitize 는 서버가 관리하는 필드를 걷어낸다.
//
// `kubectl get -o json` 은 resourceVersion·uid·status·managedFields 를 그대로
// 담는다. 그것을 되돌리면 "the object has been modified" 로 apply 가 깨진다 —
// 리허설에서 실제로 그렇게 됐다. 되살릴 것은 **의도(spec)** 이지 그때의
// 서버 상태가 아니다.
func sanitize(raw []byte) ([]byte, error) {
	var list struct {
		APIVersion string           `json:"apiVersion"`
		Kind       string           `json:"kind"`
		Items      []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("리소스 목록 해석: %w", err)
	}

	kept := make([]map[string]any, 0, len(list.Items))
	for _, item := range list.Items {
		if skipResource(item) {
			continue
		}
		delete(item, "status")

		meta, _ := item["metadata"].(map[string]any)
		for _, f := range []string{
			"resourceVersion", "uid", "creationTimestamp", "generation",
			"managedFields", "selfLink", "ownerReferences", "finalizers",
		} {
			delete(meta, f)
		}

		// Service 의 clusterIP 는 클러스터가 배정한다. 그대로 되돌리면
		// 이미 쓰이는 주소와 충돌하거나, 다른 클러스터에서는 범위 밖이다.
		if kind, _ := item["kind"].(string); kind == "Service" {
			if spec, ok := item["spec"].(map[string]any); ok {
				delete(spec, "clusterIP")
				delete(spec, "clusterIPs")
			}
		}
		kept = append(kept, item)
	}

	out := map[string]any{"apiVersion": "v1", "kind": "List", "items": kept}
	return json.MarshalIndent(out, "", "  ")
}

// skipResource 는 되돌리면 안 되는 것을 걸러낸다.
func skipResource(item map[string]any) bool {
	meta, _ := item["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	kind, _ := item["kind"].(string)

	// 컨트롤러가 소유한 것은 소유자가 다시 만든다. 먼저 되돌리면 소유자와
	// 다투게 된다.
	if refs, ok := meta["ownerReferences"].([]any); ok && len(refs) > 0 {
		return true
	}

	// 쿠버네티스가 네임스페이스마다 자동으로 만드는 것들.
	switch {
	case kind == "PersistentVolumeClaim":
		// 볼륨 경로가 소유한다 (dumpKinds 주석 참고). 다른 경로로 흘러들어와도
		// 여기서 막는다.
		return true
	case kind == "ServiceAccount" && name == "default":
		return true
	case kind == "ConfigMap" && name == "kube-root-ca.crt":
		return true
	case kind == "Secret":
		// 서비스 계정 토큰은 자동 발급된다. 옛 토큰을 되돌리면 무효한 값이
		// 유효한 것처럼 남는다.
		if t, _ := item["type"].(string); t == "kubernetes.io/service-account-token" {
			return true
		}
	}
	return false
}

// Apply 는 뜬 리소스를 되돌린다.
//
// --server-side 를 쓰는 이유: 덤프에는 resourceVersion·uid 같은 서버 관리
// 필드가 섞여 있다. 클라이언트 사이드 apply 는 그것들 때문에 충돌한다.
func (d *ResourceDumper) Apply(ctx context.Context, namespace string, in io.Reader) error {
	return d.run(ctx, in, io.Discard,
		"apply", "-n", namespace, "-f", "-", "--server-side", "--force-conflicts")
}

var _ port.ResourceDumper = (*ResourceDumper)(nil)
