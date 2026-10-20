package imagescan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 게이트웨이(HTTPRoute) → Service selector → 파드로 따라가 외부에 열린 이미지를 가린다.
const exposurePodsJSON = `{"items":[
 {"metadata":{"name":"argocd-server-1","labels":{"app.kubernetes.io/instance":"argo-cd","app.kubernetes.io/name":"argocd-server"}},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"quay.io/argoproj/argocd:v2","imageID":"quay.io/argoproj/argocd@sha256:aaa"}]}},
 {"metadata":{"name":"argocd-redis-1","labels":{"app.kubernetes.io/instance":"argo-cd","app.kubernetes.io/name":"argocd-redis"}},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"redis:7","imageID":"docker.io/library/redis@sha256:rrr"}]}}
]}`

const exposureServicesJSON = `{"items":[
 {"metadata":{"name":"argo-cd-argocd-server"},"spec":{"selector":{"app.kubernetes.io/instance":"argo-cd","app.kubernetes.io/name":"argocd-server"}}},
 {"metadata":{"name":"argo-cd-argocd-redis"},"spec":{"selector":{"app.kubernetes.io/name":"argocd-redis"}}},
 {"metadata":{"name":"headless-no-selector"},"spec":{}}
]}`

const exposureRoutesJSON = `{"items":[
 {"metadata":{"name":"argocd-route"},"spec":{"hostnames":["argocd.nullus.local"],
   "rules":[{"backendRefs":[{"name":"argo-cd-argocd-server","port":80}]}]}},
 {"metadata":{"name":"other-ns"},"spec":{"hostnames":["x.example"],
   "rules":[{"backendRefs":[{"name":"argo-cd-argocd-redis","namespace":"elsewhere","port":6379}]}]}},
 {"metadata":{"name":"not-a-service"},"spec":{"hostnames":["y.example"],
   "rules":[{"backendRefs":[{"kind":"Backend","group":"gateway.envoyproxy.io","name":"argo-cd-argocd-redis"}]}]}},
 {"metadata":{"name":"no-selector"},"spec":{"rules":[{"backendRefs":[{"name":"headless-no-selector"}]}]}}
]}`

func TestExposedPods(t *testing.T) {
	exposed, err := exposedPods("argocd-ns", []byte(exposurePodsJSON), []byte(exposureRoutesJSON), []byte(exposureServicesJSON))
	require.NoError(t, err)

	assert.Equal(t, map[string][]string{"argocd-server-1": {"argocd.nullus.local"}}, exposed,
		"다른 네임스페이스 · Service 가 아닌 백엔드 · selector 없는 Service 는 노출로 보지 않는다")
}

func TestExposedPods_NoRoutes(t *testing.T) {
	exposed, err := exposedPods("ns", []byte(exposurePodsJSON), nil, []byte(exposureServicesJSON))
	require.NoError(t, err)
	assert.Empty(t, exposed)
}

func TestScanner_MarksExposedImages(t *testing.T) {
	kubectl := &fakeKubectl{outputs: map[string]string{
		"get nodes":     nodesJSON,
		"get pods":      exposurePodsJSON,
		"get httproute": exposureRoutesJSON,
		"get services":  exposureServicesJSON,
		"get job":       "1 ",
		"logs":          "@@VERSION 0.74.0",
	}}
	s := NewScanner(WithKubectl(kubectl.run), WithPollInterval(time.Millisecond))

	scans, err := s.ScanInstalledImages(context.Background(), []byte("kc"), "argocd-ns")
	require.NoError(t, err)

	byDigest := map[string]bool{}
	hosts := map[string][]string{}
	for _, sc := range scans {
		byDigest[sc.ImageDigest] = sc.Exposed
		hosts[sc.ImageDigest] = sc.ExposedHosts
	}
	assert.True(t, byDigest["sha256:aaa"])
	assert.Equal(t, []string{"argocd.nullus.local"}, hosts["sha256:aaa"])
	assert.False(t, byDigest["sha256:rrr"])
}

// Gateway API CRD 가 없는 클러스터에서도 스캔은 한다 — 노출만 모른다.
func TestScanner_ScansWhenRoutesUnavailable(t *testing.T) {
	kubectl := &fakeKubectl{outputs: map[string]string{
		"get nodes": nodesJSON,
		"get pods":  exposurePodsJSON,
		"get job":   "1 ",
		"logs":      "@@VERSION 0.74.0",
	}, errs: map[string]error{"get httproute": errors.New(`the server doesn't have a resource type "httproutes"`)}}
	s := NewScanner(WithKubectl(kubectl.run), WithPollInterval(time.Millisecond))

	scans, err := s.ScanInstalledImages(context.Background(), []byte("kc"), "argocd-ns")
	require.NoError(t, err)
	for _, sc := range scans {
		assert.False(t, sc.Exposed)
	}
}

// 스택 게이트웨이의 Envoy 데이터플레인은 LoadBalancer Service 뒤에서 밖의 요청을 직접 받는다 — 스택의
// 모든 라우트 호스트로 노출된 것이다. 손으로 연 NodePort · LoadBalancer 도 노출이다(호스트는 모른다).
const gatewayPodsJSON = `{"items":[
 {"metadata":{"name":"envoy-1","labels":{"app.kubernetes.io/name":"envoy","gateway.envoyproxy.io/owning-gateway-namespace":"argocd-ns"}},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"envoyproxy/envoy:distroless-v1.33","imageID":"docker.io/envoyproxy/envoy@sha256:eee"}]}},
 {"metadata":{"name":"argocd-redis-1","labels":{"app.kubernetes.io/name":"argocd-redis"}},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"redis:7","imageID":"docker.io/library/redis@sha256:rrr"}]}},
 {"metadata":{"name":"internal-1","labels":{"app.kubernetes.io/name":"internal"}},
  "spec":{"nodeName":"worker"},
  "status":{"phase":"Running","containerStatuses":[{"image":"busybox:1","imageID":"docker.io/library/busybox@sha256:bbb"}]}}
]}`

const gatewayServicesJSON = `{"items":[
 {"metadata":{"name":"envoy-argocd-ns-eg-1234","labels":{"gateway.envoyproxy.io/owning-gateway-namespace":"argocd-ns"}},
  "spec":{"type":"LoadBalancer","selector":{"app.kubernetes.io/name":"envoy","gateway.envoyproxy.io/owning-gateway-namespace":"argocd-ns"}}},
 {"metadata":{"name":"redis-debug"},"spec":{"type":"NodePort","selector":{"app.kubernetes.io/name":"argocd-redis"}}},
 {"metadata":{"name":"internal"},"spec":{"type":"ClusterIP","selector":{"app.kubernetes.io/name":"internal"}}}
]}`

const gatewayRoutesJSON = `{"items":[
 {"metadata":{"name":"gitlab"},"spec":{"hostnames":["gitlab.nullus.local"],"rules":[{"backendRefs":[{"name":"gitlab-webservice"}]}]}},
 {"metadata":{"name":"argocd"},"spec":{"hostnames":["argocd.nullus.local"],"rules":[{"backendRefs":[{"name":"argocd-server"}]}]}}
]}`

func TestExposedPods_GatewayDataPlaneAndNodePorts(t *testing.T) {
	exposed, err := exposedPods("argocd-ns", []byte(gatewayPodsJSON), []byte(gatewayRoutesJSON), []byte(gatewayServicesJSON))
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"envoy-1":        {"argocd.nullus.local", "gitlab.nullus.local"},
		"argocd-redis-1": {},
	}, exposed, "ClusterIP 이고 라우트가 없는 Service 는 노출이 아니다")
}

// Gateway API CRD 가 없어도 LoadBalancer · NodePort 로 열린 것은 노출이다.
func TestExposedPods_LoadBalancerWithoutRoutes(t *testing.T) {
	exposed, err := exposedPods("argocd-ns", []byte(gatewayPodsJSON), nil, []byte(gatewayServicesJSON))
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{"envoy-1": {}, "argocd-redis-1": {}}, exposed)
}
