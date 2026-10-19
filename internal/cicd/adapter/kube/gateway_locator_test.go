package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func gateway(name string, labels map[string]string) gatewayInfo {
	return gatewayInfo{name: name, labels: labels}
}

func TestChooseGateway(t *testing.T) {
	stackLabel := map[string]string{"nullus.io/stack-name": "nullus.local"}
	cases := map[string]struct {
		gateways  []gatewayInfo
		preferred string
		want      string
	}{
		"preferred exists":   {[]gatewayInfo{gateway("a", nil), gateway("stack-gateway", nil)}, "stack-gateway", "stack-gateway"},
		"only one":           {[]gatewayInfo{gateway("nullus-local-gateway", stackLabel)}, "stack-gateway", "nullus-local-gateway"},
		"one made by nullus": {[]gatewayInfo{gateway("customer", nil), gateway("nullus-local-gateway", stackLabel)}, "stack-gateway", "nullus-local-gateway"},
		"ambiguous":          {[]gatewayInfo{gateway("a", nil), gateway("b", nil)}, "stack-gateway", ""},
		"two made by nullus": {[]gatewayInfo{gateway("a", stackLabel), gateway("b", stackLabel)}, "stack-gateway", ""},
		"none":               {nil, "stack-gateway", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, chooseGateway(tc.gateways, tc.preferred))
		})
	}
}

func TestGatewayLocator_ListsGatewaysInNamespace(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	obj := func(ns, name string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("gateway.networking.k8s.io/v1")
		u.SetKind("Gateway")
		u.SetNamespace(ns)
		u.SetName(name)
		u.SetLabels(map[string]string{"nullus.io/stack-name": "nullus.local"})
		return u
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "GatewayList"})
	// 생성자에 넘긴 unstructured 객체는 이 GVR 로 잡히지 않아 Create 로 넣는다.
	for _, o := range []*unstructured.Unstructured{obj("stack-ns", "nullus-local-gateway"), obj("other-ns", "other-gateway")} {
		_, err := client.Resource(gvr).Namespace(o.GetNamespace()).Create(context.Background(), o, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	l := &GatewayLocator{newClient: func([]byte) (dynamic.Interface, error) { return client, nil }}
	name, err := l.LocateGateway(context.Background(), []byte("kc"), "stack-ns", "stack-gateway")
	require.NoError(t, err)
	assert.Equal(t, "nullus-local-gateway", name, "다른 네임스페이스의 게이트웨이는 보지 않는다")
}
