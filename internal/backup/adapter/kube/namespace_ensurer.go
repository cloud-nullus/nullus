package kube

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// NamespaceEnsurer 는 port.NamespaceEnsurer 의 구현이다.
type NamespaceEnsurer struct {
	client kubernetes.Interface
}

// NewNamespaceEnsurer 는 클러스터 클라이언트로 만든다.
func NewNamespaceEnsurer(client kubernetes.Interface) *NamespaceEnsurer {
	return &NamespaceEnsurer{client: client}
}

// EnsureNamespace 는 네임스페이스가 없으면 만든다. 있으면 손대지 않는다.
//
// 설치는 helm --create-namespace 로 라벨 없이 만든다 — 복구도 같은 모양이면 된다. 만들 때만
// 출처를 주석으로 남긴다(PVC 와 같다): 설치 preflight 가 남은 자원을 볼 때 복구가 놓아둔
// 것과 실패한 설치의 잔여를 가를 근거가 클러스터 안에 있어야 한다.
func (e *NamespaceEnsurer) EnsureNamespace(ctx context.Context, namespace, restoredFromBackupID string) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return fmt.Errorf("복구할 네임스페이스가 비어 있습니다")
	}
	if _, err := e.client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); err == nil {
		return nil
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("네임스페이스 조회(%s): %w", namespace, err)
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	if restoredFromBackupID != "" {
		ns.Annotations = map[string]string{shareddomain.RestoredFromBackupAnnotation: restoredFromBackupID}
	}
	if _, err := e.client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil &&
		!apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("네임스페이스 생성(%s): %w", namespace, err)
	}
	return nil
}
