package helm

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

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

// ArgoCD 가 실제로 읽는 admin 키는 admin.password(bcrypt 해시)와
// admin.passwordMtime 이다. ESO 가 argocd-secret 을 단독 소유하므로
// (creationPolicy=Owner, refresh 5m) ArgoCD 가 스스로 써넣어도 다음 동기화에
// 되돌려진다. 실측한 키는 [clearPassword, oidc.keycloak.clientSecret,
// server.secretkey] 뿐이라 비밀번호 로그인이 성립하지 않았다.
//
// SSO(IdP)가 죽었을 때 들어갈 수단이 없으면 안 되므로 두 경로를 모두 둔다.
func TestArgoCDSecret_CarriesAdminPasswordHash(t *testing.T) {
	item := argocdManagedSecret(t)

	var keys []string
	for _, e := range item.Entries {
		keys = append(keys, e.TargetKey)
	}
	require.Containsf(t, keys, "admin.password",
		"ArgoCD 는 bcrypt 해시를 admin.password 에서 읽는다. 현재 키: %v", keys)
	require.Containsf(t, keys, "admin.passwordMtime",
		"mtime 이 없으면 ArgoCD 가 비밀번호 설정을 무시한다. 현재 키: %v", keys)
}

// 해시는 OpenBao 에 있는 평문과 짝이어야 한다. 따로 생성하면 사용자가 안내받는
// 비밀번호(clearPassword)로는 로그인할 수 없다.
func TestArgoCDSecret_AdminHashDerivesFromStoredPlaintext(t *testing.T) {
	item := argocdManagedSecret(t)

	for _, e := range item.Entries {
		if e.TargetKey != "admin.password" {
			continue
		}
		require.Equal(t, "pipeline/argocd/admin-password", e.DeriveFrom,
			"평문 비밀번호 경로에서 파생해야 한다")
		require.NotNil(t, e.Derive, "파생 함수가 있어야 한다")

		hash, err := e.Derive("s3cret-plaintext")
		require.NoError(t, err)
		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("s3cret-plaintext")),
			"생성된 해시가 평문과 맞지 않으면 안내된 비밀번호로 로그인할 수 없다")
		return
	}
	t.Fatal("admin.password 엔트리를 찾지 못했다")
}

func TestArgoCDSecret_AdminPasswordMtimeIsRFC3339(t *testing.T) {
	item := argocdManagedSecret(t)

	for _, e := range item.Entries {
		if e.TargetKey != "admin.passwordMtime" {
			continue
		}
		require.NotEmpty(t, e.Fixed, "mtime 은 생성 랜덤이 아니라 시각이어야 한다")
		_, err := time.Parse(time.RFC3339, e.Fixed)
		require.NoErrorf(t, err, "ArgoCD 는 RFC3339 를 기대한다: %q", e.Fixed)
		return
	}
	t.Fatal("admin.passwordMtime 엔트리를 찾지 못했다")
}
