package kube

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
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
	_ = a.client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if _, err := a.client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("헬퍼 파드 생성(%s): %w", name, err)
		}
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

func (a *VolumeArchiver) exec(ctx context.Context, namespace, pod string, cmd []string, stdin io.Reader, stdout io.Writer) error {
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

	exec, err := remotecommand.NewSPDYExecutor(a.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("exec 준비: %w", err)
	}
	var stderr strings.Builder
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin: stdin, Stdout: stdout, Stderr: &stderr,
	})
	if err != nil {
		return fmt.Errorf("%v 실패: %w (%s)", cmd, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Archive 는 PVC 내용을 tar 스트림으로 흘린다.
//
// -p (권한 보존) 를 쓰는 이유: mc mirror 같은 파일 단위 동기화 대신 tar 를
// 고른 이유가 권한·소유자·링크 보존이다. Git 저장소와 레지스트리 blob 은
// 거기에 민감하다 (§3.6).
func (a *VolumeArchiver) Archive(ctx context.Context, namespace, pvc string, out io.Writer) (int64, error) {
	counter := &countingWriter{w: out}
	err := a.withHelperPod(ctx, namespace, pvc, true, func(pod string) error {
		return a.exec(ctx, namespace, pod, []string{"tar", "-cpf", "-", "-C", "/vol", "."}, nil, counter)
	})
	return counter.n, err
}

// Restore 는 tar 스트림을 PVC 에 풀어쓴다.
func (a *VolumeArchiver) Restore(ctx context.Context, namespace, pvc string, in io.Reader) error {
	return a.withHelperPod(ctx, namespace, pvc, false, func(pod string) error {
		return a.exec(ctx, namespace, pod, []string{"tar", "-xpf", "-", "-C", "/vol"}, in, io.Discard)
	})
}

// EnsurePVC 는 매니페스트에 기록된 크기·StorageClass 대로 PVC 를 만든다.
func (a *VolumeArchiver) EnsurePVC(ctx context.Context, namespace string, spec domain.VolumeSpec) error {
	if _, err := a.client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, spec.Name, metav1.GetOptions{}); err == nil {
		return nil // 이미 있다
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("PVC 조회(%s): %w", spec.Name, err)
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: spec.Name, Namespace: namespace},
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
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

var _ port.VolumeArchiver = (*VolumeArchiver)(nil)
