package kube

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// 파이프라인이 직접 배포한 워크로드를 라벨로 찾아 지운다.
//
// Argo CD 로 배포한 앱은 Application 을 지우면 컨트롤러가 함께 걷어내지만,
// 매니페스트를 직접 적용하는 경로는 지워 줄 주체가 없다. 그래서 파이프라인을
// "클러스터 리소스까지" 지워도 Deployment·Service·Ingress 가 그대로 남아
// 앱이 계속 돌았다 — 목록에서는 사라졌는데 클러스터에는 살아 있다.

// deletableWorkloadKinds 는 매니페스트 생성기가 만드는 리소스다.
// 생성하지 않는 종류는 넣지 않는다 — 지울 것도 없는데 권한만 넓어진다.
var deletableWorkloadKinds = []schema.GroupVersionResource{
	{Group: "apps", Version: "v1", Resource: "deployments"},
	{Group: "", Version: "v1", Resource: "services"},
	{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
}

// WorkloadDeleter 는 라벨 셀렉터로 워크로드를 지운다.
type WorkloadDeleter struct{}

func NewWorkloadDeleter() *WorkloadDeleter { return &WorkloadDeleter{} }

// DeleteByLabel 은 네임스페이스에서 셀렉터에 걸리는 리소스를 지운다.
//
// 없는 것은 성공으로 본다 — Argo CD 로 배포한 앱에는 지울 것이 없고,
// 그 경우까지 실패로 만들면 정상 삭제가 막힌다.
func (d *WorkloadDeleter) DeleteByLabel(
	ctx context.Context,
	kubeconfig []byte,
	namespace, selector string,
) error {
	namespace = strings.TrimSpace(namespace)
	selector = strings.TrimSpace(selector)
	if namespace == "" || selector == "" {
		return fmt.Errorf("워크로드 삭제에는 namespace 와 라벨 셀렉터가 모두 필요합니다")
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

	// 한 종류가 실패해도 나머지는 지운다. 중간에 멈추면 무엇이 남았는지
	// 알 수 없는 상태가 된다.
	var errs []error
	for _, gvr := range deletableWorkloadKinds {
		err := dynClient.Resource(gvr).Namespace(namespace).DeleteCollection(
			ctx,
			metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: selector},
		)
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("%s 삭제 실패 (%s, %s): %w", gvr.Resource, namespace, selector, err))
		}
	}
	return errors.Join(errs...)
}
