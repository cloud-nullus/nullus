package helm

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// SSO 를 켜면 ArgoCD 가 기동하지 못하고 panic 한다:
//
//	level=warning msg="Unable to parse updated settings: server.secretkey is missing"
//	panic: server.secretkey is missing
//
// 값의 출처가 둘로 갈려 있었다. values.go 는 차트가 만드는 Secret 에
// configs.secret.extra["server.secretkey"] 로 넣는데, SSO 가 켜지면
// oidc-values 가 createSecret=false 로 그 생성을 끄고 ESO 가 argocd-secret 을
// 단독 소유한다. 그런데 ESO 매핑에는 server.secretkey 가 없어 그대로 사라졌다.
//
// SSO 프로비저닝이 실제로 도는 구성에서만 드러나는 조합이라, 프로비저너 배선이
// 끊겨 있던 동안에는 보이지 않았다.
func argocdManagedSecret(t *testing.T) ManagedSecret {
	t.Helper()
	cfg := domain.StackConfig{AccessDomain: "nullus.local"}
	cfg.Pipeline.CDTool = domain.ToolSelection{Name: "argocd", Enabled: true}

	o := NewOrchestrator(nil, nil, "ssoconverge")
	o.SetStackConfig(cfg)
	o.ssoFactory = func(string, string) port.SSOProvisioner { return stubSSOProvisioner{slug: "ssoconverge"} }

	for _, item := range o.ssoManagedSecrets() {
		if item.TargetSecret == ArgoCDSecretName {
			return item
		}
	}
	t.Fatalf("argocd-secret 관리 항목이 없다")
	return ManagedSecret{}
}

func TestArgoCDSecret_CarriesServerSecretKey(t *testing.T) {
	item := argocdManagedSecret(t)

	var keys []string
	for _, e := range item.Entries {
		keys = append(keys, e.TargetKey)
	}
	require.Containsf(t, keys, "server.secretkey",
		"ESO 가 argocd-secret 을 단독 소유하므로 server.secretkey 도 여기 있어야 한다. "+
			"없으면 argocd-server 와 dex-server 가 기동 즉시 panic 한다. 현재 키: %v", keys)
}

// 엔트리의 PathSuffix 는 OpenBao 생성 경로이기도 하다(provisionSecrets 가
// 항목마다 ensureSecretValue 를 돈다). 경로가 비면 값이 생성되지 않는다.
func TestArgoCDSecret_ServerSecretKeyHasGenerationPath(t *testing.T) {
	item := argocdManagedSecret(t)

	for _, e := range item.Entries {
		if e.TargetKey != "server.secretkey" {
			continue
		}
		require.NotEmpty(t, e.PathSuffix, "OpenBao 경로가 없으면 값이 생성되지 않는다")
		require.Empty(t, e.Fixed, "server.secretkey 는 고정값이 아니라 생성값이어야 한다")
		return
	}
	t.Fatal("server.secretkey 엔트리를 찾지 못했다")
}
