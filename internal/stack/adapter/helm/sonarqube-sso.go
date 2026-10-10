package helm

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

// SonarQube 의 Keycloak 로그인.
//
// SonarQube Community 는 SSO 로 OIDC 를 받지 않고 SAML 만 받는다. OIDC 도구와 달리
// 디스커버리 문서가 없어서, IdP 의 서명 인증서를 values 로 직접 받아 응답 서명을
// 검증한다. 클라이언트 등록은 provisioning_sso 가 다른 도구와 함께 한다(SAML
// 클라이언트에는 client secret 이 없다).
//
// SSO 로 들어온 사용자는 일반 사용자(sonar-users)다. 관리는 설치가 만든 admin 계정으로
// 한다 — Keycloak 이 멈춰도 들어갈 수단이기도 하다.

// loadSAMLSigningCertificate 는 SAML 도구에 넣을 IdP 서명 인증서를 읽어 둔다.
//
// SSO 를 쓰지 않는 스택은 읽지 않는다. SSO 를 쓰는데 못 읽으면 멈춘다 — SAML 없이
// 깔고 넘어가면 그 SonarQube 는 다시 깔 때까지 로컬 계정으로만 뜬다.
func (o *Orchestrator) loadSAMLSigningCertificate(ctx context.Context) error {
	// 클라이언트를 등록하는 판단(provisioning_sso)과 같은 근거를 본다. 갈리면 등록하지
	// 않은 클라이언트로 로그인하라고 values 를 넣게 된다.
	if !o.isStepEnabled("provisioning_sso") {
		return nil
	}
	cert, err := o.ssoProvisioner().SAMLSigningCertificate(ctx)
	if err != nil {
		return fmt.Errorf("SonarQube 에 넣을 Keycloak 서명 인증서를 읽지 못했습니다: %w", err)
	}
	cert = strings.TrimSpace(cert)
	if cert == "" {
		return fmt.Errorf("SonarQube 에 넣을 Keycloak 서명 인증서가 비어 있습니다")
	}

	o.mu.Lock()
	o.samlSigningCert = cert
	o.mu.Unlock()
	return nil
}

// sonarQubeSAMLValues 는 SonarQube 가 Keycloak 으로 로그인하게 하는 설정이다.
//
// 인증서가 없으면 넣지 않는다. 인증서 없이 SAML 을 켜면 로그인 버튼은 있는데 누를
// 때마다 서명 검증이 실패한다 — 그럴 바에는 관리자 계정으로라도 들어가게 둔다.
func (o *Orchestrator) sonarQubeSAMLValues(clientID, issuer string) map[string]any {
	o.mu.Lock()
	cert := o.samlSigningCert
	o.mu.Unlock()
	if cert == "" {
		return nil
	}

	// Keycloak 의 IdP entity ID 는 realm 주소, 곧 OIDC issuer 와 같은 값이다. 응답의
	// Issuer 가 이 값과 다르면 SonarQube 가 응답을 거부한다.
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	return map[string]any{
		"sonarProperties": map[string]any{
			"sonar.auth.saml.enabled":      "true",
			"sonar.auth.saml.providerName": "Keycloak",
			// Keycloak 에 등록한 clientId(SP entity ID)와 같아야 한다.
			"sonar.auth.saml.applicationId":       clientID,
			"sonar.auth.saml.providerId":          issuer,
			"sonar.auth.saml.loginUrl":            issuer + "/protocol/saml",
			"sonar.auth.saml.certificate.secured": cert,
			// IdP 가 싣는 속성 이름과 같아야 한다(포트의 상수).
			"sonar.auth.saml.user.login": port.SAMLLoginAttribute,
			"sonar.auth.saml.user.name":  port.SAMLNameAttribute,
			"sonar.auth.saml.user.email": port.SAMLEmailAttribute,
		},
	}
}

// sonarQubeOwnedProperties 는 sonarProperties 중 플랫폼이 계산하는 값이다 — 서버
// 주소와 Keycloak 로그인 설정.
//
// 배포된 values 를 편집하면 그 스냅샷이 오버라이드로 저장돼 병합의 맨 마지막에
// 적용된다. 그 안에 옛 인증서나 옛 주소가 얼어 있으면, 렐름 키를 회전하거나 접속
// 도메인을 바꾼 뒤 단계를 다시 돌려도 옛 값이 이긴다. 그래서 이 값들만 오버라이드
// 위에 다시 못박는다. 사용자가 더한 다른 속성은 건드리지 않는다.
func (o *Orchestrator) sonarQubeOwnedProperties() []platformOwnedValue {
	props := map[string]any{}
	for _, source := range []map[string]any{
		o.sonarqubeSharedServiceValues(),
		o.oidcValuesForStep("installing_sonarqube"),
	} {
		if p, ok := source["sonarProperties"].(map[string]any); ok {
			for key, value := range p {
				props[key] = value
			}
		}
	}

	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	owned := make([]platformOwnedValue, 0, len(keys))
	for _, key := range keys {
		owned = append(owned, platformOwnedValue{
			path:   []string{"sonarProperties", key},
			value:  props[key],
			reason: "접속 도메인과 Keycloak 에서 파생되는 값이라 옛 값으로 두면 로그인이 깨집니다",
		})
	}
	return owned
}
