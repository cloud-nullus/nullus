package kube

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 파이프라인을 만들 때 플랫폼이 넣은 Secret(이미지 pull 자격증명, CD 도구의 저장소
// 자격증명)은 Argo CD 가 배포한 것이 아니라 Application 을 지워도 남는다. 앱마다
// 이름이 달라, 남겨 두면 파이프라인을 지울 때마다 하나씩 쌓인다.

var secretGVR = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}

// ManagedSecretDeleter 는 플랫폼 표시가 있는 Secret 만 이름으로 지운다.
type ManagedSecretDeleter struct {
	newClient func(kubeconfig []byte) (dynamic.Interface, error)
}

func NewManagedSecretDeleter() *ManagedSecretDeleter {
	return &ManagedSecretDeleter{newClient: func(kubeconfig []byte) (dynamic.Interface, error) {
		config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("parse kubeconfig: %w", err)
		}
		config.Timeout = 30 * time.Second
		return dynamic.NewForConfig(config)
	}}
}

// DeleteManagedSecret 은 Secret 을 읽어 플랫폼 표시를 확인한 뒤 지운다.
//
// 읽은 그 객체만 지운다(UID 전제 조건) — 확인과 삭제 사이에 누가 같은 이름으로
// 다시 만들었다면 그것은 건드리지 않는다.
func (d *ManagedSecretDeleter) DeleteManagedSecret(
	ctx context.Context,
	kubeconfig []byte,
	namespace, name string,
) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("지울 Secret 의 namespace 와 name 이 모두 필요합니다")
	}

	client, err := d.newClient(kubeconfig)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}
	secrets := client.Resource(secretGVR).Namespace(namespace)

	secret, err := secrets.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get secret %s/%s: %w", namespace, name, err)
	}
	if secret.GetLabels()[shareddomain.AppManagedByLabel] != shareddomain.AppManagedByCICD {
		return nil
	}

	uid := secret.GetUID()
	if err := secrets.Delete(ctx, name, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &uid},
	}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete secret %s/%s: %w", namespace, name, err)
	}
	return nil
}
