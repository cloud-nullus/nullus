package config

import (
	"os"
	"path/filepath"
	"testing"
)

// keycloak 블록만 비워 둔 최소 설정. admin_url 은 환경변수로만 채워지는지 본다.
func writeMinimalConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "keycloak:\n  admin_url: \"\"\n  realm: nullus\n  admin_user: admin\n  admin_password: \"\"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return path
}

func TestLoadConfig_KeycloakAdminURLFromPrefixedEnv(t *testing.T) {
	t.Setenv("NULLUS_KEYCLOAK_ADMIN_URL", "http://kc.prefixed:8180")
	cfg, err := LoadConfig(writeMinimalConfig(t))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Keycloak.AdminURL != "http://kc.prefixed:8180" {
		t.Fatalf("expected NULLUS_KEYCLOAK_ADMIN_URL to be honoured, got %q", cfg.Keycloak.AdminURL)
	}
}

// 기존 코드는 맨 KEYCLOAK_URL 을 읽었다(cmd/nullus-bootstrap 과 에어갭 스크립트도
// 같은 이름을 쓴다). 접두사 있는 이름으로 옮기면서 옛 이름도 계속 받아 준다.
func TestLoadConfig_KeycloakAdminURLFallsBackToLegacyEnv(t *testing.T) {
	t.Setenv("KEYCLOAK_URL", "http://kc.legacy:8180")
	t.Setenv("KEYCLOAK_ADMIN_PASSWORD", "legacy-secret")
	cfg, err := LoadConfig(writeMinimalConfig(t))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Keycloak.AdminURL != "http://kc.legacy:8180" {
		t.Fatalf("expected the legacy KEYCLOAK_URL to be honoured, got %q", cfg.Keycloak.AdminURL)
	}
	if cfg.Keycloak.AdminPassword != "legacy-secret" {
		t.Fatalf("expected the legacy KEYCLOAK_ADMIN_PASSWORD to be honoured, got %q", cfg.Keycloak.AdminPassword)
	}
}

func TestLoadConfig_PrefixedEnvWinsOverLegacy(t *testing.T) {
	t.Setenv("KEYCLOAK_URL", "http://kc.legacy:8180")
	t.Setenv("NULLUS_KEYCLOAK_ADMIN_URL", "http://kc.prefixed:8180")
	cfg, err := LoadConfig(writeMinimalConfig(t))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Keycloak.AdminURL != "http://kc.prefixed:8180" {
		t.Fatalf("expected the prefixed env to win, got %q", cfg.Keycloak.AdminURL)
	}
}

// 아무 환경변수도 없으면 설정 파일의 빈 값이 그대로 남아 프로비저닝이 꺼진다.
func TestLoadConfig_KeycloakUnsetStaysUnconfigured(t *testing.T) {
	cfg, err := LoadConfig(writeMinimalConfig(t))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if _, ok := cfg.KeycloakAdmin(); ok {
		t.Fatal("expected an unset keycloak block to leave provisioning disabled")
	}
}

// 차트는 Downward API 로 NULLUS_PLATFORM_NAMESPACE 를 넣어 준다. 설정 파일에는
// 이 키가 없으므로 AutomaticEnv 만으로는 잡히지 않는다 — BindEnv 가 있어야 한다.
func TestLoadConfig_PlatformNamespaceFromEnv(t *testing.T) {
	t.Setenv("NULLUS_PLATFORM_NAMESPACE", "nullus")
	cfg, err := LoadConfig(writeMinimalConfig(t))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Platform.Namespace != "nullus" {
		t.Fatalf("expected platform namespace from env, got %q", cfg.Platform.Namespace)
	}
}

// 클러스터 밖에서 도는 개발 환경에서는 비어 있고, 그때는 네임스페이스 검사를 하지 않는다.
func TestLoadConfig_PlatformNamespaceEmptyWhenUnset(t *testing.T) {
	cfg, err := LoadConfig(writeMinimalConfig(t))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Platform.Namespace != "" {
		t.Fatalf("expected empty platform namespace, got %q", cfg.Platform.Namespace)
	}
}
