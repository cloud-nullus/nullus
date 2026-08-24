package nullusclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// S-4 완료 기준: 서버 버전 조회 후 최소 호환 버전 미달 시 경고 재료 제공.
// 경고를 낼지(stderr)·중단할지(exit 5, Automation 계약 §1)는 호출측(CLI/MCP)이
// 정한다 — 라이브러리는 판정만 한다.

func newVersionServer(t *testing.T, version string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path = %q, want /health", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"healthy","db":"connected","version":"` + version + `"}`))
	}))
}

func TestClient_ServerInfo_ReadsHealthEndpoint(t *testing.T) {
	srv := newVersionServer(t, "0.1.0-alpha")
	defer srv.Close()

	c, _ := New(Config{Server: srv.URL})
	info, err := c.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if info.Version != "0.1.0-alpha" {
		t.Errorf("Version = %q", info.Version)
	}
	if info.Status != "healthy" || info.DB != "connected" {
		t.Errorf("info = %+v", info)
	}
}

func TestClient_CheckVersionSkew_CurrentServerIsCompatible(t *testing.T) {
	// 현재 서버가 보고하는 그 값(cmd/api/main.go /health)이 곧 최소 호환 버전이다.
	srv := newVersionServer(t, MinServerVersion)
	defer srv.Close()

	c, _ := New(Config{Server: srv.URL})
	skew, err := c.CheckVersionSkew(context.Background())
	if err != nil {
		t.Fatalf("CheckVersionSkew: %v", err)
	}
	if !skew.Compatible {
		t.Errorf("Compatible = false, skew = %+v", skew)
	}
	if skew.ServerVersion != MinServerVersion || skew.MinSupported != MinServerVersion {
		t.Errorf("skew = %+v", skew)
	}
}

func TestClient_CheckVersionSkew_OlderServerIsIncompatible(t *testing.T) {
	srv := newVersionServer(t, "0.0.9")
	defer srv.Close()

	c, _ := New(Config{Server: srv.URL})
	skew, err := c.CheckVersionSkew(context.Background())
	if err != nil {
		t.Fatalf("CheckVersionSkew: %v", err)
	}
	if skew.Compatible {
		t.Error("0.0.9 < 최소 호환 버전인데 Compatible = true")
	}
	if skew.ServerVersion != "0.0.9" {
		t.Errorf("ServerVersion = %q — 경고 문구에 쓸 원문이 보존되어야 한다", skew.ServerVersion)
	}
}

func TestClient_CheckVersionSkew_NewerServerIsCompatible(t *testing.T) {
	// 서버가 앞서가는 것은 스큐 경고 대상이 아니다 — 클라이언트가 낡았을 때의
	// 안내는 서버가 줄 수 없으므로(v1 범위 밖) 여기서 다루지 않는다.
	srv := newVersionServer(t, "9.9.9")
	defer srv.Close()

	c, _ := New(Config{Server: srv.URL})
	skew, err := c.CheckVersionSkew(context.Background())
	if err != nil {
		t.Fatalf("CheckVersionSkew: %v", err)
	}
	if !skew.Compatible {
		t.Error("서버가 더 새 버전인데 Compatible = false")
	}
}

func TestClient_CheckVersionSkew_PrereleaseOrdersBelowRelease(t *testing.T) {
	// semver 규칙: 0.1.0-alpha < 0.1.0. 최소 호환을 정식판으로 올린 뒤에도
	// prerelease 서버가 슬쩍 통과하면 안 된다.
	ok, err := versionCompatible("0.1.0-alpha", "0.1.0")
	if err != nil {
		t.Fatalf("versionCompatible: %v", err)
	}
	if ok {
		t.Error("0.1.0-alpha 가 최소 0.1.0 을 통과했다")
	}
}

func TestClient_CheckVersionSkew_TolerantParse(t *testing.T) {
	// "v" 접두어나 짧은 버전을 내는 빌드도 판정은 해 준다.
	srv := newVersionServer(t, "v0.2.0")
	defer srv.Close()

	c, _ := New(Config{Server: srv.URL})
	skew, err := c.CheckVersionSkew(context.Background())
	if err != nil {
		t.Fatalf("CheckVersionSkew: %v", err)
	}
	if !skew.Compatible {
		t.Errorf("v0.2.0 판정 실패: %+v", skew)
	}
}

func TestClient_CheckVersionSkew_UnparsableVersionIsError(t *testing.T) {
	// dev 빌드 등이 semver 아닌 값을 보고하면 판정 불능이다 — 조용히 통과시키지
	// 않고 오류로 알린다. 경고로 낮출지는 호출측 판단.
	srv := newVersionServer(t, "dev")
	defer srv.Close()

	c, _ := New(Config{Server: srv.URL})
	if _, err := c.CheckVersionSkew(context.Background()); err == nil {
		t.Fatal("semver 아닌 버전을 오류 없이 판정했다")
	}
}

func TestClient_CheckVersionSkew_UnreachableServerIsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // 즉시 닫아 연결 실패 유도

	c, _ := New(Config{Server: srv.URL})
	_, err := c.CheckVersionSkew(context.Background())

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Kind != KindServer {
		t.Errorf("Kind = %v, want KindServer", apiErr.Kind)
	}
}
