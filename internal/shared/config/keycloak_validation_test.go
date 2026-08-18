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
	if err := kcCfg("http://localhost:8180", "nullus", "admin", "").ValidateKeycloakAdmin(); err == nil {
		t.Fatal("expected an admin URL without a password to be rejected at startup")
	}
}

func TestValidateKeycloakAdmin_UnconfiguredIsAccepted(t *testing.T) {
	if err := kcCfg("", "", "", "").ValidateKeycloakAdmin(); err != nil {
		t.Fatalf("expected an unconfigured provisioner to pass validation, got %v", err)
	}
}

func TestValidateKeycloakAdmin_FullyConfiguredIsAccepted(t *testing.T) {
	if err := kcCfg("http://localhost:8180", "nullus", "admin", "admin").ValidateKeycloakAdmin(); err != nil {
		t.Fatalf("expected a fully configured provisioner to pass validation, got %v", err)
	}
}
