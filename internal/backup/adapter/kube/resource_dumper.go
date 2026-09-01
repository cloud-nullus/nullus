package kube

import (
	"context"
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
var dumpKinds = []string{
	"deployments", "statefulsets", "daemonsets", "services", "configmaps",
	"secrets", "persistentvolumeclaims", "serviceaccounts", "roles",
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

func (d *ResourceDumper) Dump(ctx context.Context, namespace string, out io.Writer) (int64, error) {
	counter := &countingWriter{w: out}
	// CRD 가 설치돼 있지 않으면 해당 종류에서 실패한다. --ignore-not-found 로
	// 없는 것은 조용히 건너뛴다 — 스택 구성에 따라 Gateway API 가 없을 수 있다.
	args := []string{
		"get", strings.Join(dumpKinds, ","),
		"-n", namespace, "-o", "yaml", "--ignore-not-found",
	}
	if err := d.run(ctx, nil, counter, args...); err != nil {
		return counter.n, err
	}
	return counter.n, nil
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
