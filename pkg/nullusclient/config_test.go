package nullusclient

import (
	"os"
	"path/filepath"
	"testing"
)

// 우선순위: 명시 값(플래그) > NULLUS_* env > ~/.nullus/ 파일.
// Automation 계약 §5 — CI 에서 파일 없이 env 만으로 실행 가능해야 한다.

func TestLoad_ExplicitOverridesEnvAndFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	writeConfigFile(t, dir, "https://file.example.com")
	t.Setenv(EnvServer, "https://env.example.com")
	t.Setenv(EnvToken, "env-token")

	cfg, err := Load(Config{Server: "https://flag.example.com", Token: "flag-token"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server != "https://flag.example.com" {
		t.Errorf("Server = %q, want flag value", cfg.Server)
	}
	if cfg.Token != "flag-token" {
		t.Errorf("Token = %q, want flag value", cfg.Token)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	writeConfigFile(t, dir, "https://file.example.com")
	writeTokenFile(t, dir, "file-token", 0o600)
	t.Setenv(EnvServer, "https://env.example.com")
	t.Setenv(EnvToken, "env-token")

	cfg, err := Load(Config{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server != "https://env.example.com" {
		t.Errorf("Server = %q, want env value", cfg.Server)
	}
	if cfg.Token != "env-token" {
		t.Errorf("Token = %q, want env value", cfg.Token)
	}
}

func TestLoad_FileFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	t.Setenv(EnvServer, "")
	t.Setenv(EnvToken, "")
	writeConfigFile(t, dir, "https://file.example.com")
	writeTokenFile(t, dir, "file-token", 0o600)

	cfg, err := Load(Config{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server != "https://file.example.com" {
		t.Errorf("Server = %q, want file value", cfg.Server)
	}
	if cfg.Token != "file-token" {
		t.Errorf("Token = %q, want file value", cfg.Token)
	}
}

func TestLoad_MissingEverythingIsNotAnError(t *testing.T) {
	// 서버 미설정 검증은 클라이언트 생성(New) 몫이다 — Load 는 수집만 한다.
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	t.Setenv(EnvServer, "")
	t.Setenv(EnvToken, "")

	cfg, err := Load(Config{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server != "" || cfg.Token != "" {
		t.Errorf("cfg = %+v, want zero values", cfg)
	}
}

func TestSaveToken_Creates0600FileAnd0700Dir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nullus-home") // 없는 디렉토리 — SaveToken 이 만들어야 한다
	t.Setenv(EnvConfigDir, dir)

	if err := SaveToken("secret-token"); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	di, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 0700", perm)
	}

	fi, err := os.Stat(filepath.Join(dir, tokenFileName))
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("token file perm = %o, want 0600", perm)
	}

	got, err := ReadToken()
	if err != nil {
		t.Fatalf("ReadToken: %v", err)
	}
	if got != "secret-token" {
		t.Errorf("ReadToken = %q, want saved token", got)
	}
}

func TestReadToken_RejectsGroupOtherReadableFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	t.Setenv(EnvToken, "")
	writeTokenFile(t, dir, "leaky-token", 0o644)

	_, err := ReadToken()
	if err == nil {
		t.Fatal("ReadToken: 0644 토큰 파일을 거부해야 한다")
	}
}

func TestReadToken_MissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	t.Setenv(EnvToken, "")

	got, err := ReadToken()
	if err != nil {
		t.Fatalf("ReadToken: %v", err)
	}
	if got != "" {
		t.Errorf("ReadToken = %q, want empty (파일 없음은 오류가 아니다)", got)
	}
}

func writeConfigFile(t *testing.T, dir, server string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte("server: "+server+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeTokenFile(t *testing.T, dir, token string, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, tokenFileName), []byte(token+"\n"), perm); err != nil {
		t.Fatal(err)
	}
}
