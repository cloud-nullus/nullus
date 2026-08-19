package config

import "testing"

func kcCfg(adminURL, realm, adminUser, adminPassword string) Config {
	c := Config{}
	c.Keycloak.AdminURL = adminURL
	c.Keycloak.Realm = realm
	c.Keycloak.AdminUser = adminUser
	c.Keycloak.AdminPassword = adminPassword
	return c
}

// admin_url 이 비어 있으면 SSO 프로비저닝을 하지 않는 구성이다(BYO IdP / 미사용).
// 이 경우까지 에러로 만들면 Keycloak 없이 쓰는 설치가 기동조차 못 한다.
func TestKeycloakAdmin_UnsetURLMeansNotConfigured(t *testing.T) {
	if _, ok := kcCfg("", "", "", "").KeycloakAdmin(); ok {
		t.Fatal("expected an empty admin URL to report the provisioner as unconfigured")
	}
}

// 공백만 든 값은 "설정했다" 로 보면 안 된다 — 차트 values 가 비었을 때 흔한 모양이다.
func TestKeycloakAdmin_BlankURLMeansNotConfigured(t *testing.T) {
	if _, ok := kcCfg("   ", "nullus", "admin", "admin").KeycloakAdmin(); ok {
		t.Fatal("expected a whitespace-only admin URL to report the provisioner as unconfigured")
	}
}

func TestKeycloakAdmin_ConfiguredURLIsTrimmed(t *testing.T) {
	kc, ok := kcCfg("  http://localhost:8180  ", "nullus", "admin", "admin").KeycloakAdmin()
	if !ok {
		t.Fatal("expected a configured admin URL to report the provisioner as ready")
	}
	if kc.AdminURL != "http://localhost:8180" {
		t.Fatalf("expected the admin URL to be trimmed, got %q", kc.AdminURL)
	}
}

// realm/admin_user 는 값을 안 줘도 동작하던 자리다(기존 main.go 의 envOrDefault).
// 기본값을 잃으면 차트에 값을 안 넣은 설치가 조용히 엉뚱한 realm 을 보게 된다.
func TestKeycloakAdmin_FillsRealmAndUserDefaults(t *testing.T) {
	kc, ok := kcCfg("http://localhost:8180", "", "", "admin").KeycloakAdmin()
	if !ok {
		t.Fatal("expected a configured admin URL to report the provisioner as ready")
	}
	if kc.Realm != "nullus" {
		t.Fatalf("expected realm to default to nullus, got %q", kc.Realm)
	}
	if kc.AdminUser != "admin" {
		t.Fatalf("expected admin user to default to admin, got %q", kc.AdminUser)
	}
}

func TestKeycloakAdmin_ExplicitValuesWinOverDefaults(t *testing.T) {
	kc, _ := kcCfg("http://kc", "platform", "svc-nullus", "s3cret").KeycloakAdmin()
	if kc.Realm != "platform" || kc.AdminUser != "svc-nullus" || kc.AdminPassword != "s3cret" {
		t.Fatalf("expected explicit values to be preserved, got %+v", kc)
	}
}

// 주소만 있고 비밀번호가 없으면 admin-cli 토큰 발급이 실패한다. 그런데 그 실패는
// 스택 설치가 provisioning_sso 까지 진행된 뒤에야 드러난다 — 몇 분 뒤 설치가
// 깨지는 대신 기동 시점에 끊는다.
func TestValidateKeycloakAdmin_URLWithoutPasswordFailsFast(t *testing.T) {
	c := kcCfg("http://localhost:8180", "nullus", "admin", "")
	c.Auth.OIDC.IssuerURL = "http://localhost:8180/realms/nullus"
	if err := c.ValidateKeycloakAdmin(); err == nil {
		t.Fatal("expected an admin URL without a password to be rejected at startup")
	}
}

func TestValidateKeycloakAdmin_UnconfiguredIsAccepted(t *testing.T) {
	if err := kcCfg("", "", "", "").ValidateKeycloakAdmin(); err != nil {
		t.Fatalf("expected an unconfigured provisioner to pass validation, got %v", err)
	}
}

func TestValidateKeycloakAdmin_FullyConfiguredIsAccepted(t *testing.T) {
	c := kcCfg("http://localhost:8180", "nullus", "admin", "admin")
	// 도구에 넣을 issuer 까지 있어야 "완전 설정" 이다 — 이게 없으면 클라이언트만
	// 만들어지고 도구는 SSO 없이 뜬다.
	c.Auth.OIDC.IssuerURL = "http://localhost:8180/realms/nullus"
	if err := c.ValidateKeycloakAdmin(); err != nil {
		t.Fatalf("expected a fully configured provisioner to pass validation, got %v", err)
	}
}

// ── 설치되는 OSS 가 쓸 issuer ────────────────────────────────────────────────
//
// 도구는 브라우저를 이 주소로 보낸다. 포털이 로그인한 Keycloak 과 오리진이 같아야
// 세션 쿠키가 실려 재인증 없이 넘어간다.

func issuerCfg(publicURL, realm, authIssuer string) Config {
	c := Config{}
	c.Keycloak.PublicURL = publicURL
	c.Keycloak.Realm = realm
	c.Auth.OIDC.IssuerURL = authIssuer
	return c
}

// 기본은 포털(API)이 쓰는 issuer 를 그대로 물려받는다. 두 값이 따로 놀 수 있게
// 두면 조용히 어긋나고, 어긋난 순간 도구 로그인만 깨진다 — 포털은 멀쩡해서
// 원인을 찾기 어렵다.
func TestToolOIDCIssuerURL_DefaultsToPortalIssuer(t *testing.T) {
	got := issuerCfg("", "nullus", "https://auth.nullus.io/realms/nullus").ToolOIDCIssuerURL()
	if got != "https://auth.nullus.io/realms/nullus" {
		t.Fatalf("expected the portal issuer to be reused, got %q", got)
	}
}

// API 가 검증에 쓰는 주소와 브라우저가 접근하는 주소가 다른 구성(내부 주소 대 공개
// 주소)을 위해 public_url 로 덮어쓸 수 있다.
func TestToolOIDCIssuerURL_PublicURLOverridesPortalIssuer(t *testing.T) {
	got := issuerCfg("https://keycloak.nullus.local", "nullus", "http://kc.internal/realms/nullus").ToolOIDCIssuerURL()
	if got != "https://keycloak.nullus.local/realms/nullus" {
		t.Fatalf("expected public_url to win and get the realm path appended, got %q", got)
	}
}

func TestToolOIDCIssuerURL_TrimsTrailingSlash(t *testing.T) {
	got := issuerCfg("https://keycloak.nullus.local/  ", "nullus", "").ToolOIDCIssuerURL()
	if got != "https://keycloak.nullus.local/realms/nullus" {
		t.Fatalf("expected a trailing slash to be dropped before appending the realm, got %q", got)
	}
}

func TestToolOIDCIssuerURL_UsesRealmDefault(t *testing.T) {
	got := issuerCfg("https://keycloak.nullus.local", "", "").ToolOIDCIssuerURL()
	if got != "https://keycloak.nullus.local/realms/nullus" {
		t.Fatalf("expected the realm to default to nullus, got %q", got)
	}
}

func TestToolOIDCIssuerURL_UnconfiguredIsEmpty(t *testing.T) {
	if got := issuerCfg("", "", "").ToolOIDCIssuerURL(); got != "" {
		t.Fatalf("expected an unconfigured issuer to be empty, got %q", got)
	}
}

// 클라이언트는 Keycloak 에 만들어지는데 도구에 넣을 issuer 가 없으면, 설치는
// 초록불로 끝나고 도구만 SSO 없이 뜬다. 예전 배선 누락과 똑같은 조용한 실패라
// 기동 시점에 끊는다.
func TestValidateKeycloakAdmin_ProvisioningWithoutToolIssuerFailsFast(t *testing.T) {
	c := kcCfg("http://localhost:8180", "nullus", "admin", "admin")
	if err := c.ValidateKeycloakAdmin(); err == nil {
		t.Fatal("expected provisioning without a tool issuer to be rejected at startup")
	}
}
