package config

import (
	"fmt"
	"strings"
)

const (
	defaultKeycloakRealm     = "nullus"
	defaultKeycloakAdminUser = "admin"
)

// KeycloakAdmin 은 OSS SSO 프로비저닝(provisioning_sso)에 쓸 Keycloak 관리자
// 접속 정보를 정규화해 돌려준다.
//
// ok=false 는 "이 설치에서는 SSO 프로비저닝을 하지 않는다" 는 뜻이다 — BYO IdP 나
// SSO 미사용 구성이 여기에 해당한다. 주소가 없는 것을 에러로 만들면 Keycloak 없이
// 쓰는 설치가 기동조차 못 하므로 구분한다.
func (c Config) KeycloakAdmin() (KeycloakConfig, bool) {
	adminURL := strings.TrimSpace(c.Keycloak.AdminURL)
	if adminURL == "" {
		return KeycloakConfig{}, false
	}

	kc := KeycloakConfig{
		AdminURL:      adminURL,
		Realm:         strings.TrimSpace(c.Keycloak.Realm),
		AdminUser:     strings.TrimSpace(c.Keycloak.AdminUser),
		AdminPassword: c.Keycloak.AdminPassword,
	}
	if kc.Realm == "" {
		kc.Realm = defaultKeycloakRealm
	}
	if kc.AdminUser == "" {
		kc.AdminUser = defaultKeycloakAdminUser
	}
	return kc, true
}

// ValidateKeycloakAdmin 은 런타임에만 드러날 프로비저닝 설정 오류를 기동 시점에 끊는다.
//
// 주소만 있고 비밀번호가 없으면 admin-cli 토큰 발급이 실패하는데, 그 실패는 스택
// 설치가 provisioning_sso 단계까지 간 뒤에야 보인다. 설치를 몇 분 진행한 다음
// 깨지는 대신 여기서 멈춘다.
func (c Config) ValidateKeycloakAdmin() error {
	kc, ok := c.KeycloakAdmin()
	if !ok {
		return nil
	}
	if strings.TrimSpace(kc.AdminPassword) == "" {
		return fmt.Errorf(
			"keycloak.admin_url 이 설정됐는데 keycloak.admin_password 가 비어 있습니다. "+
				"관리자 비밀번호를 설정하거나(helm: config.keycloak.adminPassword) "+
				"admin_url 을 비워 SSO 프로비저닝을 끄세요 (admin_url=%s)",
			kc.AdminURL)
	}
	return nil
}
