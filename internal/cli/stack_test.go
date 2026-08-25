package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A-1 슬라이스: nullus stack ls — 표 출력, -o json, exit code 규약.

func mockStacksServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/stacks" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

const twoStacks = `{"items":[
  {"id":"stk_a1","name":"team-alpha","state":"completed","cluster_name":"prod-01","namespace":"nullus-alpha","created_at":"2026-08-20T09:00:00Z"},
  {"id":"stk_b2","name":"team-beta","state":"deploying","cluster_name":"","namespace":"nullus-beta","created_at":"2026-08-23T02:30:00Z"}
],"total":2}`

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Main(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestStackLs_RendersTable(t *testing.T) {
	srv := mockStacksServer(t, http.StatusOK, twoStacks)
	defer srv.Close()

	code, stdout, _ := run(t, "--server", srv.URL, "stack", "ls")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"stk_a1", "team-alpha", "completed", "prod-01", "team-beta", "deploying"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("표에 %q 가 없다\n%s", want, stdout)
		}
	}
	// 이름 없는 클러스터는 "-" 로 — 모른다는 것을 모른다고 표시한다.
	if !strings.Contains(stdout, "-") {
		t.Errorf("빈 cluster_name 이 - 로 표기되어야 한다\n%s", stdout)
	}
}

func TestStackLs_JSONOutputIsMachineReadable(t *testing.T) {
	srv := mockStacksServer(t, http.StatusOK, twoStacks)
	defer srv.Close()

	code, stdout, _ := run(t, "--server", srv.URL, "-o", "json", "stack", "ls")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("stdout 이 단일 JSON 객체가 아니다: %v\n%s", err, stdout)
	}
	if len(payload.Items) != 2 {
		t.Errorf("items = %d, want 2", len(payload.Items))
	}
}

func TestStackLs_AuthErrorMapsToExit3(t *testing.T) {
	srv := mockStacksServer(t, http.StatusUnauthorized,
		`{"error":{"code":"UNAUTHENTICATED","http_status":401,"message":"token expired","trace_id":"req-1"}}`)
	defer srv.Close()

	code, stdout, stderr := run(t, "--server", srv.URL, "stack", "ls")
	if code != 3 {
		t.Fatalf("exit = %d, want 3 (인증)", code)
	}
	if stdout != "" {
		t.Errorf("실패 시 stdout 은 비어야 한다: %q", stdout)
	}
	if !strings.Contains(stderr, "token expired") {
		t.Errorf("stderr 에 서버 메시지가 없다: %q", stderr)
	}
}

func TestStackLs_ConnectionFailureMapsToExit5(t *testing.T) {
	srv := mockStacksServer(t, http.StatusOK, twoStacks)
	srv.Close() // 죽은 서버

	code, _, stderr := run(t, "--server", srv.URL, "stack", "ls")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (연결 실패), stderr=%s", code, stderr)
	}
}

func TestStackLs_MissingServerMapsToExit2(t *testing.T) {
	t.Setenv("NULLUS_SERVER", "")
	t.Setenv("NULLUS_CONFIG_DIR", t.TempDir()) // 홈의 실제 설정과 격리

	code, _, stderr := run(t, "stack", "ls")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (사용법), stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "NULLUS_SERVER") {
		t.Errorf("stderr 가 해결 방법을 안내해야 한다: %q", stderr)
	}
}

func TestVersion_PrintsWithoutServer(t *testing.T) {
	code, stdout, _ := run(t, "version")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(stdout, "nullus") {
		t.Errorf("version 출력이 비었다: %q", stdout)
	}
}
