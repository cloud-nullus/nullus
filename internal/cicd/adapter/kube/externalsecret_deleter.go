package kube

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nullus/draft/internal/cicd/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// Gitea · Jenkins 스택의 파이프라인 자격증명은 OpenBao → ExternalSecret → Secret 으로
// 실린다. ExternalSecret 을 지우면 ESO 가 만든 Secret 은 소유자 참조(creationPolicy: Owner)
// 로 함께 회수된다.

var externalSecretGVR = schema.GroupVersionResource{
	Group:    "external-secrets.io",
	Version:  "v1",
	Resource: "externalsecrets",
}

// ExternalSecretDeleter 는 파이프라인 자격증명 ExternalSecret 을 지운다.
type ExternalSecretDeleter struct {
	newClient func(kubeconfig []byte) (dynamic.Interface, error)
}

func NewExternalSecretDeleter() *ExternalSecretDeleter {
	return &ExternalSecretDeleter{newClient: func(kubeconfig []byte) (dynamic.Interface, error) {
		config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("parse kubeconfig: %w", err)
		}
		config.Timeout = 30 * time.Second
		return dynamic.NewForConfig(config)
	}}
}

// DeleteCredentialObject 는 이 파이프라인이 만든 ExternalSecret 만 지운다.
//
// 플랫폼 표시가 있으면 지운다. 표시가 붙기 전에 만든 것은 원격 키가 모두 이 파이프라인의
// 금고 경로 아래일 때만 지운다 — 같은 네임스페이스에서 스택 모듈도 같은 SecretStore 로
// ExternalSecret 을 만든다. 읽은 그 객체만 지운다(UID 전제 조건).
func (d *ExternalSecretDeleter) DeleteCredentialObject(
	ctx context.Context,
	kubeconfig []byte,
	obj port.PipelineCredentialObject,
) error {
	namespace := strings.TrimSpace(obj.Namespace)
	name := strings.TrimSpace(obj.Name)
	if namespace == "" || name == "" {
		return fmt.Errorf("지울 ExternalSecret 의 namespace 와 name 이 모두 필요합니다")
	}
	client, err := d.newClient(kubeconfig)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}
	externalSecrets := client.Resource(externalSecretGVR).Namespace(namespace)

	current, err := externalSecrets.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get externalsecret %s/%s: %w", namespace, name, err)
	}
	if !ownedByPipeline(current, obj.RemoteKeyPrefix) {
		return nil
	}

	uid := current.GetUID()
	if err := externalSecrets.Delete(ctx, name, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &uid},
	}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete externalsecret %s/%s: %w", namespace, name, err)
	}
	return nil
}

func ownedByPipeline(es *unstructured.Unstructured, remoteKeyPrefix string) bool {
	if es.GetLabels()[shareddomain.AppManagedByLabel] == shareddomain.AppManagedByCICD {
		return true
	}
	prefix := strings.TrimSpace(remoteKeyPrefix)
	if prefix == "" {
		return false
	}
	data, _, _ := unstructured.NestedSlice(es.Object, "spec", "data")
	if len(data) == 0 {
		return false
	}
	for _, item := range data {
		entry, ok := item.(map[string]any)
		if !ok {
			return false
		}
		key, _, _ := unstructured.NestedString(entry, "remoteRef", "key")
		if !strings.HasPrefix(key, prefix) {
			return false
		}
	}
	return true
}
