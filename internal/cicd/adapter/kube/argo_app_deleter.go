package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// argoResourcesFinalizer 는 Argo CD 가 Application 을 지울 때 그 Application 이
// 만든 리소스까지 함께 지우게 하는 finalizer 다.
//
// 이것이 없으면 Application 만 사라지고 Deployment·Service·HTTPRoute 는 클러스터에
// 그대로 남는다. 사용자 눈에는 파이프라인을 지웠는데 앱이 계속 도는 상태가 된다.
const argoResourcesFinalizer = "resources-finalizer.argocd.argoproj.io"

// argoApplicationGVR 은 Argo CD Application 의 그룹/버전/리소스다.
var argoApplicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

// ArgoApplicationDeleter 는 Argo CD Application 과 그것이 배포한 리소스를 지운다.
type ArgoApplicationDeleter struct{}

func NewArgoApplicationDeleter() *ArgoApplicationDeleter {
	return &ArgoApplicationDeleter{}
}

// Delete 는 Application 에 정리 finalizer 를 붙인 뒤 삭제한다.
//
// 순서가 중요하다. 먼저 지우면 Argo CD 컨트롤러가 정리할 대상 정보를 잃어
// 워크로드가 고아로 남는다. 생성 시점에 finalizer 를 넣지 않는 이유는, 그러면
// Application 을 손으로 지울 때도 항상 배포까지 함께 사라져 되돌릴 수 없기
// 때문이다 — 파괴적 동작은 삭제를 요청한 이 시점에만 켠다.
//
// Application 이 이미 없으면 성공으로 본다.
func (d *ArgoApplicationDeleter) DeleteApplication(
	ctx context.Context,
	kubeconfig []byte,
	namespace, name string,
) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("argo application 의 namespace 와 name 이 모두 필요합니다")
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("parse kubeconfig: %w", err)
	}
	config.Timeout = 30 * time.Second

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}
	apps := dynClient.Resource(argoApplicationGVR).Namespace(namespace)

	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"finalizers": []string{argoResourcesFinalizer},
		},
	})
	if err != nil {
		return fmt.Errorf("build finalizer patch: %w", err)
	}

	if _, err := apps.Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("add cleanup finalizer to application %s/%s: %w", namespace, name, err)
	}

	if err := apps.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete application %s/%s: %w", namespace, name, err)
	}
	return nil
}
