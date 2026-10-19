package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// gatewayGVR 은 Gateway API 의 Gateway 다.
var gatewayGVR = schema.GroupVersionResource{
	Group:    "gateway.networking.k8s.io",
	Version:  "v1",
	Resource: "gateways",
}

// stackGatewayLabel 은 스택 설치가 자기 게이트웨이에 다는 라벨이다(화면 빌더·서버 기본값 둘 다).
const stackGatewayLabel = "nullus.io/stack-name"

// GatewayLocator 는 스택 네임스페이스의 게이트웨이를 조회한다.
type GatewayLocator struct {
	newClient func(kubeconfig []byte) (dynamic.Interface, error)
}

var _ port.GatewayLocator = (*GatewayLocator)(nil)

// NewGatewayLocator 는 kubeconfig 로 클러스터에 붙는 조회기를 만든다.
func NewGatewayLocator() *GatewayLocator {
	return &GatewayLocator{newClient: func(kubeconfig []byte) (dynamic.Interface, error) {
		config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("parse kubeconfig: %w", err)
		}
		config.Timeout = 15 * time.Second
		return dynamic.NewForConfig(config)
	}}
}

// LocateGateway 는 namespace 의 게이트웨이 가운데 앱이 붙을 것을 고른다(chooseGateway).
func (l *GatewayLocator) LocateGateway(ctx context.Context, kubeconfig []byte, namespace, preferred string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "", fmt.Errorf("namespace is required")
	}
	client, err := l.newClient(kubeconfig)
	if err != nil {
		return "", err
	}
	list, err := client.Resource(gatewayGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list gateways in %s: %w", namespace, err)
	}
	gateways := make([]gatewayInfo, 0, len(list.Items))
	for _, item := range list.Items {
		gateways = append(gateways, gatewayInfo{name: item.GetName(), labels: item.GetLabels()})
	}
	return chooseGateway(gateways, strings.TrimSpace(preferred)), nil
}

type gatewayInfo struct {
	name   string
	labels map[string]string
}

// chooseGateway 는 앱 HTTPRoute 가 붙을 게이트웨이를 고른다.
//
// 화면 설치 규칙의 이름이 있으면 그것, 아니면 하나뿐인 것, 여럿이면 Nullus 가 만든 것(스택 라벨) 하나다.
// 그래도 하나로 좁혀지지 않으면 고르지 않는다 — 남의 게이트웨이에 앱을 붙이지 않는다.
func chooseGateway(gateways []gatewayInfo, preferred string) string {
	if len(gateways) == 0 {
		return ""
	}
	if preferred != "" {
		for _, g := range gateways {
			if g.name == preferred {
				return g.name
			}
		}
	}
	if len(gateways) == 1 {
		return gateways[0].name
	}
	var ours []string
	for _, g := range gateways {
		if strings.TrimSpace(g.labels[stackGatewayLabel]) != "" {
			ours = append(ours, g.name)
		}
	}
	sort.Strings(ours)
	if len(ours) == 1 {
		return ours[0]
	}
	return ""
}
