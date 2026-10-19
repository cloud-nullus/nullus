package provisioning

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type fakeGatewayLocator struct {
	name  string
	err   error
	calls []string
}

func (f *fakeGatewayLocator) LocateGateway(_ context.Context, kubeconfig []byte, namespace, preferred string) (string, error) {
	f.calls = append(f.calls, string(kubeconfig)+"|"+namespace+"|"+preferred)
	return f.name, f.err
}

type fakeKubeconfigs struct{ err error }

func (f fakeKubeconfigs) GetKubeconfig(_ context.Context, clusterID string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte("kc-" + clusterID), nil
}

func namedGitLabStack() *port.StackSummary {
	s := gitlabStack()
	s.Name = "nullus-gitlab-argocd-sonarqube"
	return s
}

// 스택은 게이트웨이를 만드는 길이 둘이다 — 화면 설치는 `<스택 이름>-gateway`, 화면이 게이트웨이 매니페스트를
// 보내지 않는 설치(런북 stack-up 등)는 서버 기본값이 **접근 도메인**에서 이름을 만든다(nullus.local →
// nullus-local-gateway). 이름을 짐작하면 둘 중 하나는 틀리고, 틀린 이름을 가리키는 HTTPRoute 는 어느
// 컨트롤러도 집지 않아 앱 주소가 404 다(2026-10-19 kind 실측 — demo-api 도 그랬다). 그래서 스택 네임스페이스에
// 실제로 있는 게이트웨이를 쓴다.
func TestFor_UsesGatewayThatExistsInStackNamespace(t *testing.T) {
	locator := &fakeGatewayLocator{name: "nullus-local-gateway"}
	f := newFactory(t, namedGitLabStack(), &fakeTokenIssuer{token: "glpat-x"}).
		WithManifestApplier(nil, fakeKubeconfigs{}).
		WithGatewayLocator(locator)

	bundle, err := f.For(context.Background(), "stk_1")
	require.NoError(t, err)

	assert.Equal(t, "nullus-local-gateway", bundle.GatewayName)
	assert.Equal(t, []string{"kc-c1|devsecops|nullus-gitlab-argocd-sonarqube-gateway"}, locator.calls,
		"스택 네임스페이스에서, 화면 설치 규칙의 이름을 우선해 찾는다")
}

// 클러스터에 닿지 못하면 지금까지처럼 이름을 계산한다 — 파이프라인 생성을 막지 않는다.
func TestFor_FallsBackToComputedGatewayWhenLookupFails(t *testing.T) {
	for name, f := range map[string]*BundleFactory{
		"locator error": newFactory(t, namedGitLabStack(), &fakeTokenIssuer{token: "glpat-x"}).
			WithManifestApplier(nil, fakeKubeconfigs{}).
			WithGatewayLocator(&fakeGatewayLocator{err: errors.New("connection refused")}),
		"kubeconfig error": newFactory(t, namedGitLabStack(), &fakeTokenIssuer{token: "glpat-x"}).
			WithManifestApplier(nil, fakeKubeconfigs{err: errors.New("no cluster")}).
			WithGatewayLocator(&fakeGatewayLocator{name: "other"}),
		"ambiguous": newFactory(t, namedGitLabStack(), &fakeTokenIssuer{token: "glpat-x"}).
			WithManifestApplier(nil, fakeKubeconfigs{}).
			WithGatewayLocator(&fakeGatewayLocator{name: ""}),
		"not wired": newFactory(t, namedGitLabStack(), &fakeTokenIssuer{token: "glpat-x"}),
	} {
		t.Run(name, func(t *testing.T) {
			bundle, err := f.For(context.Background(), "stk_1")
			require.NoError(t, err)
			assert.Equal(t, "nullus-gitlab-argocd-sonarqube-gateway", bundle.GatewayName)
		})
	}
}

// 접근 도메인이 없으면 HTTPRoute 를 만들지 않으므로(renderHTTPRoute) 클러스터에 묻지 않는다.
func TestFor_DoesNotLookUpGatewayWithoutAccessDomain(t *testing.T) {
	stack := namedGitLabStack()
	stack.AccessDomain = ""
	locator := &fakeGatewayLocator{name: "nullus-local-gateway"}
	_, err := newFactory(t, stack, &fakeTokenIssuer{token: "glpat-x"}).
		WithManifestApplier(nil, fakeKubeconfigs{}).
		WithGatewayLocator(locator).
		For(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Empty(t, locator.calls)
}
