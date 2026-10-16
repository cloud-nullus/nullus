package cli

import (
	"strings"
	"testing"
)

// B-1 슬라이스: nullus mcp serve — 토큰 부재 안내, exit code, 옵트인 해석.

// isolateConfig 는 실제 홈 설정·env 와 격리한다.
func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("NULLUS_SERVER", "")
	t.Setenv("NULLUS_TOKEN", "")
	t.Setenv("NULLUS_CONFIG_DIR", t.TempDir())
}

func TestMCPServe_NoToken_MapsToExit3(t *testing.T) {
	isolateConfig(t)

	// 서버 주소는 있으나 토큰이 없다 — 기동 전에 안내하고 끝나야 한다.
	code, stdout, stderr := run(t, "--server", "http://127.0.0.1:1", "mcp", "serve")
	if code != 3 {
		t.Fatalf("exit = %d, want 3 (인증), stderr=%s", code, stderr)
	}
	// stdout 은 MCP 프로토콜 전용이다 — 실패 시에도 아무것도 쓰면 안 된다.
	if stdout != "" {
		t.Errorf("stdout 은 비어야 한다: %q", stdout)
	}
	if !strings.Contains(stderr, "nullus login") {
		t.Errorf("stderr 가 로그인 방법을 안내해야 한다: %q", stderr)
	}
}

func TestMCPServe_MissingServerMapsToExit2(t *testing.T) {
	isolateConfig(t)

	code, _, stderr := run(t, "mcp", "serve")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (사용법), stderr=%s", code, stderr)
	}
}

func TestResolveAllowWrite(t *testing.T) {
	cases := []struct {
		name    string
		flagSet bool
		flag    bool
		env     string
		want    bool
	}{
		{"플래그 명시 true", true, true, "", true},
		{"플래그 명시 false 는 env 를 이긴다", true, false, "true", false},
		{"env true", false, false, "true", true},
		{"env 1", false, false, "1", true},
		{"env false", false, false, "false", false},
		{"env 쓰레기값은 꺼짐", false, false, "yes-please", false},
		{"기본 꺼짐", false, false, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvAllowWrite, tc.env)
			if got := resolveAllowWrite(tc.flagSet, tc.flag); got != tc.want {
				t.Errorf("resolveAllowWrite(%v, %v) env=%q = %v, want %v",
					tc.flagSet, tc.flag, tc.env, got, tc.want)
			}
		})
	}
}
