package kube

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// VolumeArchiver 는 PVC 를 tar 로 뜨고 되돌린다.
//
// ── 데이터 경로에 대한 설계 이탈 (기록) ────────────────────────────────────
// 설계 §3.6 은 "볼륨을 마운트한 Job 이 목적지로 직접 올린다" 를 권고했다.
// 구현하면서 그것이 §5(키 취급)와 충돌한다는 것이 드러났다:
//
//	산출물은 봉인해서 올려야 하고(§5.1), 봉인 키는 컨트롤 플레인에만
//	있어야 한다(§4.2.1·§5.2). Job 이 직접 올리려면 그 키를 **백업 대상
//	클러스터 안으로** 넣어야 하는데, 그러면 클러스터가 침해될 때 백업본까지
//	함께 열린다.
//
// 그래서 컨트롤 플레인이 데이터 경로에 들어간다. 대역폭을 두 번 쓰는 대신
// 키가 대상 클러스터를 벗어나지 않는다. 이 교환은 설계 문서 §3.6 에 반영했다.
// ───────────────────────────────────────────────────────────────────────────
//
// 스트리밍은 kubectl cp 와 같은 방식이다 — 헬퍼 파드에서 `tar` 를 exec 하고
// stdout 을 그대로 흘린다. 중간 파일을 만들지 않는다.
type VolumeArchiver struct {
	// SpoolDir 가 비어 있지 않으면 Archive 는 exec 출력을 먼저 이 디렉터리의 임시 파일에 최대 속도로
	// 받아 둔 뒤 out 으로 흘린다. 느린 소비자(봉인 · 업로드)가 exec 스트림을 막아 끝부분이 떨어지는
	// 전송 계층(11-01 실측)을 피한다. 비어 있으면 중간 파일 없이 바로 흘린다(§4.3).
	SpoolDir string
	// deleteWait 는 이전 헬퍼 파드가 사라지길 기다리는 상한이다(0 이면 1분).
	deleteWait time.Duration
	client     kubernetes.Interface
	restConfig *rest.Config
	image      string
	timeout    time.Duration
}

// DefaultHelperImage 는 에어갭 번들에 있는 이미지다 (airgap/images/images.txt).
// tar 가 들어 있고 별도 반입이 필요 없다.
const DefaultHelperImage = "busybox:1.37"

func NewVolumeArchiver(client kubernetes.Interface, cfg *rest.Config, image string) *VolumeArchiver {
	if strings.TrimSpace(image) == "" {
		image = DefaultHelperImage
	}
	return &VolumeArchiver{client: client, restConfig: cfg, image: image, timeout: 10 * time.Minute}
}

// ListPVCs 는 네임스페이스의 PVC 를 **열거한다**.
//
// 알려진 도구 목록을 순회하지 않는 이유: GitLab·Gitea 는 설치기가 볼륨
// 크기를 지정하지 않아 차트 기본값을 따르고, 차트는 여러 PVC 를 자기
// 기본값으로 만든다. 정적 목록은 반드시 뒤처진다 (§1.5).
func (a *VolumeArchiver) ListPVCs(ctx context.Context, namespace string) ([]domain.VolumeSpec, error) {
	list, err := a.client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("PVC 목록 조회(%s): %w", namespace, err)
	}
	out := make([]domain.VolumeSpec, 0, len(list.Items))
	for _, p := range list.Items {
		spec := domain.VolumeSpec{Name: p.Name}
		if q, ok := p.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			spec.SizeBytes = q.Value()
		}
		if p.Spec.StorageClassName != nil {
			spec.StorageClass = *p.Spec.StorageClassName
		}
		out = append(out, spec)
	}
	return out, nil
}

func helperPodName(pvc string) string {
	name := "nullus-backup-" + pvc
	if len(name) > 63 {
		name = name[:63]
	}
	return strings.TrimRight(name, "-")
}

// withHelperPod 은 PVC 를 마운트한 파드를 띄우고 fn 을 실행한 뒤 정리한다.
func (a *VolumeArchiver) withHelperPod(ctx context.Context, namespace, pvc string, readOnly bool, fn func(pod string) error) error {
	name := helperPodName(pvc)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "nullus-backup"},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:    "helper",
				Image:   a.image,
				Command: []string{"sh", "-c", "sleep 3600"},
				VolumeMounts: []corev1.VolumeMount{{
					Name: "vol", MountPath: "/vol", ReadOnly: readOnly,
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "vol",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc, ReadOnly: readOnly,
					},
				},
			}},
		},
	}

	// 앞선 실행이 남긴 파드가 있으면 지우고 시작한다 (멱등).
	// 이전 시도의 헬퍼가 아직 지워지는 중이면 그대로 두고 만들 수 없다 — AlreadyExists 를 넘기고
	// 종료 중인 컨테이너에 exec 하면 137(SIGKILL) 이나 "task not found" 로 끝난다(11-01 실측).
	// 헬퍼는 sleep 뿐이라 유예 없이 지우고, 사라진 것을 확인한 뒤 만든다.
	if err := a.replaceHelperPod(ctx, namespace, pod); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = a.client.CoreV1().Pods(namespace).Delete(cleanup, name, metav1.DeleteOptions{})
	}()

	if err := a.waitReady(ctx, namespace, name); err != nil {
		return err
	}
	return fn(name)
}

// replaceHelperPod 는 같은 이름의 헬퍼가 남아 있으면 즉시 지우고 사라질 때까지 기다린 뒤 새로 만든다.
func (a *VolumeArchiver) replaceHelperPod(ctx context.Context, namespace string, pod *corev1.Pod) error {
	name := pod.Name
	pods := a.client.CoreV1().Pods(namespace)
	grace := int64(0)
	if err := pods.Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("이전 헬퍼 파드 삭제(%s): %w", name, err)
	}
	wait := a.deleteWait
	if wait <= 0 {
		wait = time.Minute
	}
	deadline := time.Now().Add(wait)
	for {
		if _, err := pods.Get(ctx, name, metav1.GetOptions{}); apierrors.IsNotFound(err) {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("이전 헬퍼 파드 %s 가 %s 안에 사라지지 않았습니다", name, wait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if _, err := pods.Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("헬퍼 파드 생성(%s): %w", name, err)
	}
	return nil
}

func (a *VolumeArchiver) waitReady(ctx context.Context, namespace, name string) error {
	deadline := time.Now().Add(2 * time.Minute)
	for {
		p, err := a.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil && p.Status.Phase == corev1.PodRunning {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("헬퍼 파드 %s 가 기동하지 않았습니다", name)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// exec 는 헬퍼 파드에서 명령을 돌리고 stderr 내용을 돌려준다(Archive 가 바이트 수 보고를 읽는다).
//
// 전송은 WebSocket(v5)을 먼저 쓰고 업그레이드가 안 되면 SPDY 로 간다 — kubectl 과 같다.
// SPDY 는 사용 중단 경로이고, 11-01 에 1.6 GB 스트림이 64 KiB 경계에서 조용히 끊긴 채
// 성공으로 끝나는 것을 봤다.
func (a *VolumeArchiver) exec(ctx context.Context, namespace, pod string, cmd []string, stdin io.Reader, stdout io.Writer) (string, error) {
	stderr := &syncBuffer{}
	err := a.execWithStderr(ctx, namespace, pod, cmd, stdin, stdout, stderr)
	return stderr.String(), err
}

// syncBuffer 는 exec 가 쓰는 동안 다른 고루틴이 읽어도 안전한 stderr 버퍼다.
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}
func (b *syncBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }

func (a *VolumeArchiver) execWithStderr(ctx context.Context, namespace, pod string, cmd []string, stdin io.Reader, stdout io.Writer, stderr *syncBuffer) error {
	if a.restConfig == nil {
		return fmt.Errorf("rest.Config 가 없어 파드에 exec 할 수 없습니다")
	}
	req := a.client.CoreV1().RESTClient().Post().
		Resource("pods").Name(pod).Namespace(namespace).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "helper",
			Command:   cmd,
			Stdin:     stdin != nil,
			Stdout:    stdout != nil,
			Stderr:    true,
		}, scheme.ParameterCodec)

	spdy, err := remotecommand.NewSPDYExecutor(a.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("exec 준비: %w", err)
	}
	ws, err := remotecommand.NewWebSocketExecutor(a.restConfig, "GET", req.URL().String())
	if err != nil {
		return fmt.Errorf("exec 준비(websocket): %w", err)
	}
	exec, err := remotecommand.NewFallbackExecutor(ws, spdy, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	if err != nil {
		return fmt.Errorf("exec 준비(fallback): %w", err)
	}
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin: stdin, Stdout: stdout, Stderr: stderr,
	})
	if err != nil {
		return fmt.Errorf("%v 실패: %w (%s)", cmd, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// archiveCommand 는 tar 스트림을 stdout 으로 내보내며 파드 안에서 센 바이트 수를 stderr 마지막 줄에
// 남기고, stdout 을 명시적으로 닫은 뒤 잠깐 기다렸다가 tar 의 종료 코드로 끝난다.
//
// 11-01 실측: 프로세스가 stdout 을 열어 둔 채 끝나면 아직 전달되지 않은 끝부분(수 MB~)이 떨어지고도
// exec 는 성공으로 끝났다 — kubectl 로 느리게 읽어도 같다(kind + Docker 프록시). stdout 을 먼저
// 닫고 잠깐 기다린 뒤 끝내면 같은 조건에서 손실 없이 도착했다. 그래도 받은 쪽은 보고된 바이트 수와
// 대조한다(verifyArchiveStream). stdin ack 는 쓸 수 없다 — stdout 을 닫으면 kubelet 이 stdin 도 닫는다.
// pipefail 로 tar 의 실패도 그대로 종료 코드에 남긴다.
var archiveCommand = []string{"sh", "-c",
	"exec 3>&1; set -o pipefail; tar -cpf - -C /vol . | tee /dev/fd/3 | wc -c >&2; rc=$?; exec 1>&- 3>&-; sleep 2; exit $rc"}

// parseReportedBytes 는 stderr 의 마지막 숫자 줄(wc -c)을 돌려준다.
func parseReportedBytes(stderr string) (int64, bool) {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l == "" {
			continue
		}
		if n, err := strconv.ParseInt(l, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

// verifyArchiveStream 은 파드가 보고한 바이트 수(stderr 의 마지막 숫자 줄)와 받은 바이트 수를 대조한다.
// 보고가 없으면 온전함을 알 수 없으므로 실패로 본다.
func verifyArchiveStream(pvc string, received int64, stderr string) error {
	sent, ok := parseReportedBytes(stderr)
	if !ok {
		return fmt.Errorf("볼륨 %s 아카이브: 파드가 보낸 바이트 수 보고가 없어 온전함을 확인할 수 없습니다 (stderr: %q)", pvc, strings.TrimSpace(stderr))
	}
	if sent != received {
		return fmt.Errorf("볼륨 %s 아카이브 스트림이 잘렸습니다: 받은 %d 바이트 / 파드가 보낸 %d 바이트", pvc, received, sent)
	}
	return nil
}

// Archive 는 PVC 내용을 tar 스트림으로 흘린다.
//
// -p (권한 보존) 를 쓰는 이유: mc mirror 같은 파일 단위 동기화 대신 tar 를
// 고른 이유가 권한·소유자·링크 보존이다. Git 저장소와 레지스트리 blob 은
// 거기에 민감하다 (§3.6).
func (a *VolumeArchiver) Archive(ctx context.Context, namespace, pvc string, out io.Writer) (int64, error) {
	if a.SpoolDir != "" {
		return a.archiveViaSpool(ctx, namespace, pvc, out)
	}
	counter := &countingWriter{w: out}
	err := a.withHelperPod(ctx, namespace, pvc, true, func(pod string) error {
		stderr := &syncBuffer{}
		if err := a.execWithStderr(ctx, namespace, pod, archiveCommand, nil, counter, stderr); err != nil {
			return err
		}
		return verifyArchiveStream(pvc, counter.count(), stderr.String())
	})
	return counter.count(), err
}

// archiveViaSpool 은 exec 출력을 스풀 파일에 최대 속도로 받아 검증한 뒤 out 으로 흘린다.
// 스풀 파일은 끝나면 지운다(실패해도).
func (a *VolumeArchiver) archiveViaSpool(ctx context.Context, namespace, pvc string, out io.Writer) (int64, error) {
	f, err := newSpoolFile(a.SpoolDir, pvc)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()

	counter := &countingWriter{w: f}
	err = a.withHelperPod(ctx, namespace, pvc, true, func(pod string) error {
		stderr := &syncBuffer{}
		if err := a.execWithStderr(ctx, namespace, pod, archiveCommand, nil, counter, stderr); err != nil {
			return err
		}
		return verifyArchiveStream(pvc, counter.count(), stderr.String())
	})
	if err != nil {
		return counter.count(), err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("스풀 파일 되감기: %w", err)
	}
	n, err := io.Copy(out, f)
	if err != nil {
		return n, fmt.Errorf("스풀에서 흘리기(%s): %w", pvc, err)
	}
	return n, nil
}

// newSpoolFile 은 스풀 디렉터리(없으면 만든다) 안에 볼륨 이름이 붙은 임시 파일을 만든다.
func newSpoolFile(dir, pvc string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("스풀 디렉터리 %s: %w", dir, err)
	}
	f, err := os.CreateTemp(dir, "nullus-backup-"+pvc+"-*.tar")
	if err != nil {
		return nil, fmt.Errorf("스풀 파일 생성(%s): %w", dir, err)
	}
	return f, nil
}

// Restore 는 tar 스트림을 PVC 에 풀어쓴다.
func (a *VolumeArchiver) Restore(ctx context.Context, namespace, pvc string, in io.Reader) error {
	return a.withHelperPod(ctx, namespace, pvc, false, func(pod string) error {
		_, err := a.exec(ctx, namespace, pod, []string{"tar", "-xpf", "-", "-C", "/vol"}, in, io.Discard)
		return err
	})
}

// EnsurePVC 는 매니페스트에 기록된 크기·StorageClass 대로 PVC 를 만든다.
func (a *VolumeArchiver) EnsurePVC(ctx context.Context, namespace string, spec domain.VolumeSpec, restoredFromBackupID string) error {
	if _, err := a.client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, spec.Name, metav1.GetOptions{}); err == nil {
		return nil // 이미 있다
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("PVC 조회(%s): %w", spec.Name, err)
	}

	meta := metav1.ObjectMeta{Name: spec.Name, Namespace: namespace}
	// 출처를 볼륨 자체에 남긴다. 설치 preflight 는 남은 볼륨을 보고 설치를
	// 막는데, 복구가 놓아둔 것과 실패한 설치의 잔여를 구분할 근거가 여기밖에
	// 없다 — 네임스페이스 밖의 기록은 사라질 수 있다.
	if restoredFromBackupID != "" {
		meta.Annotations = map[string]string{
			shareddomain.RestoredFromBackupAnnotation: restoredFromBackupID,
		}
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: meta,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *resource.NewQuantity(spec.SizeBytes, resource.BinarySI),
				},
			},
		},
	}
	if spec.StorageClass != "" {
		sc := spec.StorageClass
		pvc.Spec.StorageClassName = &sc
	}
	if _, err := a.client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("PVC 생성(%s): %w", spec.Name, err)
	}
	return nil
}

type countingWriter struct {
	mu sync.Mutex
	w  io.Writer
	n  int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.mu.Lock()
	c.n += int64(n)
	c.mu.Unlock()
	return n, err
}

func (c *countingWriter) count() int64 { c.mu.Lock(); defer c.mu.Unlock(); return c.n }

var _ port.VolumeArchiver = (*VolumeArchiver)(nil)
