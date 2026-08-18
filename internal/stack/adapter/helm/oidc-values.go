package helm

import (
	"fmt"
	"strings"
)

// 코드 생성 values 에 OIDC 블록을 주입한다.
//
// 이전에는 OIDC 설정이 `airgap/helm/stack-values/*.yaml` 에만 있어서
// 일반(비에어갭) 설치에는 SSO 가 전혀 적용되지 않았다. 또 그 파일들은
// client secret 을 리터럴로 담고 있었다.
//
// 두 가지를 바꾼다.
//   - issuer 는 플랫폼 설정에서 주입받는다 (WithToolOIDCIssuer)
//   - client secret 은 값이 아니라 Secret 참조로 전달
//
// issuer 를 한때 accessDomain 에서 "https://keycloak.<도메인>/realms/nullus" 로
// 만들어 냈는데, 플랫폼 Keycloak 은 스택마다가 아니라 하나뿐이라 스택이 둘 이상이면
// 둘 중 하나는 반드시 없는 주소를 받았다. 접두사가 다른 배포(auth.nullus.io)도
// 표현할 수 없었다. 도구 자신의 주소(argocd.<도메인>)만 accessDomain 을 따른다.

// oidcValuesForStep 은 도구별 OIDC values 를 만든다.
//
// clientID 는 스택 단위로 네임스페이싱된 값이며, secret 은 ESO 가 만든
// Kubernetes Secret 에서 참조한다.
func (o *Orchestrator) oidcValuesForStep(step string) map[string]any {
	provisioner := o.ssoProvisioner()
	if provisioner == nil {
		return nil
	}
	clientID, ok := provisioner.ClientIDFor(step)
	if !ok || strings.TrimSpace(clientID) == "" {
		return nil
	}

	o.mu.Lock()
	cfg := o.stackConfig
	issuer := o.toolOIDCIssuer
	o.mu.Unlock()
	if cfg == nil {
		return nil
	}
	// issuer 가 없으면 값을 절반만 넣지 않는다. 반쯤 설정된 도구는 깨진 로그인
	// 화면을 보여 주지만, 아예 없으면 로컬 계정으로라도 들어갈 수 있다.
	if issuer == "" {
		return nil
	}

	secretName := SSOSecretName(clientID)

	switch step {
	case "installing_grafana":
		return map[string]any{
			// client secret 은 env 로 주입한다. values 에 값이 남지 않는다.
			"envValueFrom": map[string]any{
				"GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET": map[string]any{
					"secretKeyRef": map[string]any{
						"name": secretName,
						"key":  "client-secret",
					},
				},
			},
			"grafana.ini": map[string]any{
				"server": map[string]any{
					"root_url": fmt.Sprintf("https://grafana.%s", cfg.AccessDomain),
				},
				"auth": map[string]any{
					// 구버전 키 호환
					"oauth_auto_login": "true",
				},
				"auth.generic_oauth": map[string]any{
					"enabled": "true",
					"name":    "Keycloak",
					// 로그인 페이지를 건너뛰고 즉시 redirect — 무중단 핸드오프의 핵심
					"auto_login": "true",
					"client_id":  clientID,
					"scopes":     "openid profile email",
					"use_pkce":   "true",
					"auth_url":   issuer + "/protocol/openid-connect/auth",
					"token_url":  issuer + "/protocol/openid-connect/token",
					"api_url":    issuer + "/protocol/openid-connect/userinfo",
				},
			},
		}

	case "installing_argocd":
		return map[string]any{
			"configs": map[string]any{
				"cm": map[string]any{
					"url": fmt.Sprintf("https://argocd.%s", cfg.AccessDomain),
					"oidc.config": fmt.Sprintf(`name: Keycloak
issuer: %s
clientID: %s
clientSecret: $oidc.keycloak.clientSecret
enablePKCEAuthentication: false
requestedScopes:
  - openid
  - profile
  - email
`, issuer, clientID),
				},
				// argocd-secret 은 ESO 가 단독 소유한다.
				// 차트가 만들면 ESO 가 주기적으로 덮어써 OIDC 키가 사라진다.
				"secret": map[string]any{
					"createSecret": false,
				},
			},
		}

	case "installing_minio":
		return map[string]any{
			"oidc": map[string]any{
				"enabled":                  true,
				"configUrl":                issuer + "/.well-known/openid-configuration",
				"clientId":                 clientID,
				"existingClientSecretName": secretName,
				"existingClientSecretKey":  "client-secret",
				"redirectUri":              fmt.Sprintf("https://minio.%s/oauth_callback", cfg.AccessDomain),
			},
		}
	}
	return nil
}
