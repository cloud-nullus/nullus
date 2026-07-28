package keycloak

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 신규 클라이언트는 Nullus 가 정한 secret 과 함께 등록되어야 한다.
//
// Keycloak 이 생성한 값을 읽어오는 방식이 아니라 push 방식이다.
// OpenBao 가 SoT 여야 하고, Keycloak 이 유실돼도 복원할 수 있어야 하기 때문이다.
func TestRegisterOIDCClient_CreatesWithSecret(t *testing.T) {
	var created map[string]any

	srv := newKeycloakStub(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/clients") {
			_, _ = w.Write([]byte(`[]`)) // 없는 상태
			return true
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/clients") {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &created)
			w.WriteHeader(http.StatusCreated)
			return true
		}
		return false
	})
	defer srv.Close()

	kc := newStubClient(srv.URL)
	err := kc.UpsertOIDCClient(context.Background(), OIDCClientSpec{
		ClientID:     "prod-grafana",
		Name:         "Grafana",
		Secret:       "generated-secret",
		RedirectURIs: []string{"https://grafana.example.com/login/generic_oauth"},
		PKCEMethod:   "S256",
	})
	if err != nil {
		t.Fatalf("UpsertOIDCClient 실패: %v", err)
	}

	if created["secret"] != "generated-secret" {
		t.Fatalf("secret 이 전달되지 않음: %v", created["secret"])
	}
	if created["publicClient"] != false {
		t.Fatalf("confidential client 여야 합니다: %v", created["publicClient"])
	}
	attrs, _ := created["attributes"].(map[string]any)
	if attrs["pkce.code.challenge.method"] != "S256" {
		t.Fatalf("PKCE 설정이 누락됨: %v", attrs)
	}
}

// 이미 존재하는 클라이언트는 갱신되어야 한다.
//
// 기존 구현은 409 Conflict 를 성공으로 처리해 시크릿을 회전해도 Keycloak 에
// 반영되지 않았다. 증상이 "회전 후 SSO 로그인 실패" 로만 나타나 추적이 어렵다.
func TestRegisterOIDCClient_UpdatesExisting(t *testing.T) {
	var updated map[string]any
	var putCalled bool

	srv := newKeycloakStub(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/clients") {
			_, _ = w.Write([]byte(`[{"id":"uuid-1","clientId":"prod-grafana"}]`))
			return true
		}
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/clients/uuid-1") {
			putCalled = true
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &updated)
			w.WriteHeader(http.StatusNoContent)
			return true
		}
		return false
	})
	defer srv.Close()

	kc := newStubClient(srv.URL)
	err := kc.UpsertOIDCClient(context.Background(), OIDCClientSpec{
		ClientID:     "prod-grafana",
		Name:         "Grafana",
		Secret:       "rotated-secret",
		RedirectURIs: []string{"https://grafana.example.com/login/generic_oauth"},
	})
	if err != nil {
		t.Fatalf("UpsertOIDCClient 실패: %v", err)
	}
	if !putCalled {
		t.Fatal("기존 클라이언트가 갱신되지 않았습니다 — 회전이 Keycloak 에 반영되지 않습니다")
	}
	if updated["secret"] != "rotated-secret" {
		t.Fatalf("회전된 secret 이 반영되지 않음: %v", updated["secret"])
	}
}

// PKCE 를 쓰지 않는 도구는 해당 속성이 비어 있어야 한다.
func TestRegisterOIDCClient_OmitsPKCEWhenUnset(t *testing.T) {
	var created map[string]any
	srv := newKeycloakStub(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/clients") {
			_, _ = w.Write([]byte(`[]`))
			return true
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/clients") {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &created)
			w.WriteHeader(http.StatusCreated)
			return true
		}
		return false
	})
	defer srv.Close()

	kc := newStubClient(srv.URL)
	if err := kc.UpsertOIDCClient(context.Background(), OIDCClientSpec{
		ClientID: "prod-argocd",
		Secret:   "s",
	}); err != nil {
		t.Fatalf("UpsertOIDCClient 실패: %v", err)
	}

	attrs, _ := created["attributes"].(map[string]any)
	if v, ok := attrs["pkce.code.challenge.method"]; ok && v != "" {
		t.Fatalf("PKCE 를 쓰지 않는 도구에 설정이 붙었습니다: %v", v)
	}
}

// --- 테스트 보조 ---

func newStubClient(baseURL string) *KeycloakClient {
	return NewKeycloakClient(baseURL, "nullus", "admin", "admin")
}

func newKeycloakStub(t *testing.T, handler func(http.ResponseWriter, *http.Request) bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 토큰 발급 요청
		if strings.Contains(r.URL.Path, "/protocol/openid-connect/token") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"stub-token","expires_in":300}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if handler(w, r) {
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}
