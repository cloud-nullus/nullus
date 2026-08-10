package config

import "testing"

func cfgWith(serverMode, authMode, issuerURL string) Config {
	c := Config{}
	c.Server.Mode = serverMode
	c.Auth.Mode = authMode
	c.Auth.OIDC.IssuerURL = issuerURL
	return c
}

// oidc 인데 issuer 가 비면 JWKS 를 못 가져와 모든 요청이 401 이 된다. 그 상태로
// 기동하면 "왜 전부 401 이지" 를 런타임에 헤매게 되므로, 기동 시점에 끊는다.
func TestValidateAuth_OIDCWithoutIssuerFailsFast(t *testing.T) {
	err := cfgWith("production", "oidc", "").ValidateAuth()
	if err == nil {
		t.Fatal("expected oidc without an issuer URL to be rejected at startup")
	}
}

func TestValidateAuth_OIDCWithIssuerIsAccepted(t *testing.T) {
	if err := cfgWith("production", "oidc", "https://auth.example.com/realms/nullus").ValidateAuth(); err != nil {
		t.Fatalf("expected a configured oidc setup to pass, got %v", err)
	}
}

// development 는 인증 미들웨어를 아예 붙이지 않는다. 거기서 issuer 를 요구하면
// 로컬 실행이 이유 없이 막힌다.
func TestValidateAuth_DevelopmentSkipsTheIssuerRequirement(t *testing.T) {
	if err := cfgWith("development", "oidc", "").ValidateAuth(); err != nil {
		t.Fatalf("development must not require an issuer, got %v", err)
	}
}

func TestValidateAuth_ToleratesCasingAndWhitespace(t *testing.T) {
	if err := cfgWith("production", " OIDC ", "  ").ValidateAuth(); err == nil {
		t.Fatal("expected a padded/uppercased oidc mode with a blank issuer to still be rejected")
	}
}

// session 은 클라이언트가 보낸 X-User-* 헤더를 그대로 믿는다 — 인증이 아니다.
func TestTrustsClientSuppliedIdentity(t *testing.T) {
	tests := []struct {
		name       string
		serverMode string
		authMode   string
		want       bool
	}{
		{"session in production trusts the caller", "production", "session", true},
		{"oidc verifies the caller", "production", "oidc", false},
		{"development disables auth by design, not a surprise", "development", "session", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cfgWith(tc.serverMode, tc.authMode, "https://issuer").TrustsClientSuppliedIdentity()
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
