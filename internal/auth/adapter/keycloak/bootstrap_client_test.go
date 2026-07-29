package keycloak

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// 부트스트랩 클라이언트는 없으면 만들고 있으면 갱신한다 (멱등).
//
// 설치를 폐기 후 재실행하는 흐름이 흔하므로, 재발급이 마찰 없이 되어야
// "설치 후 폐기" 정책을 실제로 지킬 수 있다.
func TestEnsureBootstrapClient_CreatesWithServiceAccount(t *testing.T) {
	var created map[string]any
	var mappedRoles []map[string]any

	srv := newBootstrapStub(t, bootstrapStubState{
		existingClients: `[]`,
		onCreate: func(payload map[string]any) {
			created = payload
		},
		onRoleMap: func(roles []map[string]any) {
			mappedRoles = roles
		},
	})
	defer srv.Close()

	kc := NewKeycloakClient(srv.URL, "nullus", "admin", "admin")
	secret, err := kc.EnsureBootstrapClient(context.Background(), BootstrapClientSpec{
		ClientID: "nullus-bootstrap",
		Secret:   "bootstrap-secret",
		Roles:    []string{"admin"},
	})
	if err != nil {
		t.Fatalf("EnsureBootstrapClient 실패: %v", err)
	}
	if secret != "bootstrap-secret" {
		t.Fatalf("secret 이 그대로 반환되어야 합니다: %q", secret)
	}

	// 기계 신원이므로 service account 를 켜야 한다.
	if created["serviceAccountsEnabled"] != true {
		t.Fatalf("serviceAccountsEnabled 가 켜지지 않음: %v", created)
	}
	// 브라우저 로그인 흐름은 필요 없다.
	if created["standardFlowEnabled"] != false {
		t.Fatalf("standardFlowEnabled 는 꺼져 있어야 합니다: %v", created)
	}
	if created["publicClient"] != false {
		t.Fatalf("confidential client 여야 합니다: %v", created)
	}
	if created["secret"] != "bootstrap-secret" {
		t.Fatalf("secret 이 push 되지 않음: %v", created["secret"])
	}

	// admin 역할이 service account 사용자에 매핑되어야 Admin API 를 호출할 수 있다.
	if len(mappedRoles) != 1 || mappedRoles[0]["name"] != "admin" {
		t.Fatalf("admin 역할이 매핑되지 않음: %v", mappedRoles)
	}
}

// 이미 존재하면 PUT 으로 갱신한다 — 재실행 시 secret 이 회전된다.
func TestEnsureBootstrapClient_UpdatesExisting(t *testing.T) {
	var updated map[string]any
	srv := newBootstrapStub(t, bootstrapStubState{
		existingClients: `[{"id":"uuid-b","clientId":"nullus-bootstrap"}]`,
		onUpdate: func(payload map[string]any) {
			updated = payload
		},
	})
	defer srv.Close()

	kc := NewKeycloakClient(srv.URL, "nullus", "admin", "admin")
	if _, err := kc.EnsureBootstrapClient(context.Background(), BootstrapClientSpec{
		ClientID: "nullus-bootstrap",
		Secret:   "rotated-secret",
		Roles:    []string{"admin"},
	}); err != nil {
		t.Fatalf("EnsureBootstrapClient 실패: %v", err)
	}
	if updated["secret"] != "rotated-secret" {
		t.Fatalf("재발급 시 secret 이 갱신되지 않음: %v", updated["secret"])
	}
}

// client_credentials 로 토큰을 받아온다.
func TestIssueBootstrapToken(t *testing.T) {
	var grantType, sentClientID, sentSecret string

	srv := newBootstrapStub(t, bootstrapStubState{
		onTokenRequest: func(form url.Values) {
			grantType = form.Get("grant_type")
			sentClientID = form.Get("client_id")
			sentSecret = form.Get("client_secret")
		},
	})
	defer srv.Close()

	kc := NewKeycloakClient(srv.URL, "nullus", "admin", "admin")
	token, err := kc.IssueBootstrapToken(context.Background(), "nullus-bootstrap", "bootstrap-secret")
	if err != nil {
		t.Fatalf("IssueBootstrapToken 실패: %v", err)
	}
	if token == "" {
		t.Fatal("토큰이 비어 있습니다")
	}
	if grantType != "client_credentials" {
		t.Fatalf("client_credentials grant 여야 합니다: %q", grantType)
	}
	if sentClientID != "nullus-bootstrap" || sentSecret != "bootstrap-secret" {
		t.Fatalf("클라이언트 자격이 전달되지 않음: id=%q secret=%q", sentClientID, sentSecret)
	}
}

// 설치가 끝나면 클라이언트를 삭제한다.
// 쓰지 않는 admin 자격을 남기지 않는 것이 목적이다.
func TestRevokeBootstrapClient_DeletesClient(t *testing.T) {
	var deletedPath string
	srv := newBootstrapStub(t, bootstrapStubState{
		existingClients: `[{"id":"uuid-b","clientId":"nullus-bootstrap"}]`,
		onDelete: func(path string) {
			deletedPath = path
		},
	})
	defer srv.Close()

	kc := NewKeycloakClient(srv.URL, "nullus", "admin", "admin")
	if err := kc.RevokeBootstrapClient(context.Background(), "nullus-bootstrap"); err != nil {
		t.Fatalf("RevokeBootstrapClient 실패: %v", err)
	}
	if !strings.Contains(deletedPath, "uuid-b") {
		t.Fatalf("클라이언트가 삭제되지 않음: %q", deletedPath)
	}
}

// 이미 없으면 성공으로 처리한다 (멱등).
func TestRevokeBootstrapClient_IdempotentWhenAbsent(t *testing.T) {
	srv := newBootstrapStub(t, bootstrapStubState{existingClients: `[]`})
	defer srv.Close()

	kc := NewKeycloakClient(srv.URL, "nullus", "admin", "admin")
	if err := kc.RevokeBootstrapClient(context.Background(), "nullus-bootstrap"); err != nil {
		t.Fatalf("없는 클라이언트 폐기는 성공이어야 합니다: %v", err)
	}
}

// --- 테스트 보조 ---

type bootstrapStubState struct {
	existingClients string
	onCreate        func(map[string]any)
	onUpdate        func(map[string]any)
	onRoleMap       func([]map[string]any)
	onDelete        func(string)
	onTokenRequest  func(url.Values)
}

func newBootstrapStub(t *testing.T, state bootstrapStubState) *httptest.Server {
	t.Helper()
	if state.existingClients == "" {
		state.existingClients = `[]`
	}

	created := false
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)

		switch {
		case strings.Contains(r.URL.Path, "/protocol/openid-connect/token"):
			if state.onTokenRequest != nil {
				form, _ := url.ParseQuery(string(body))
				state.onTokenRequest(form)
			}
			_, _ = w.Write([]byte(`{"access_token":"issued-token","expires_in":300}`))

		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/clients"):
			// 생성 이후에는 조회에서 나와야 role 매핑이 가능하다 (실제 Keycloak 동작).
			if created {
				_, _ = w.Write([]byte(`[{"id":"uuid-new","clientId":"nullus-bootstrap"}]`))
				return
			}
			_, _ = w.Write([]byte(state.existingClients))

		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/clients"):
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			if state.onCreate != nil {
				state.onCreate(payload)
			}
			created = true
			w.WriteHeader(http.StatusCreated)

		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/clients/"):
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			if state.onUpdate != nil {
				state.onUpdate(payload)
			}
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/clients/"):
			if state.onDelete != nil {
				state.onDelete(r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)

		case strings.Contains(r.URL.Path, "/service-account-user"):
			_, _ = w.Write([]byte(`{"id":"sa-user-1","username":"service-account-nullus-bootstrap"}`))

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/roles/"):
			_, _ = w.Write([]byte(`{"id":"role-admin","name":"admin"}`))

		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/role-mappings/realm"):
			var roles []map[string]any
			_ = json.Unmarshal(body, &roles)
			if state.onRoleMap != nil {
				state.onRoleMap(roles)
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}
