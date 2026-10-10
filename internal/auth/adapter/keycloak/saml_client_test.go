package keycloak

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	stackport "github.com/cloud-nullus/draft/internal/stack/port"
)

// SonarQube Community 는 SSO 로 OIDC 를 받지 않고 SAML 만 받는다. 그래서
// 다른 도구처럼 client secret 을 갖는 OIDC 클라이언트가 아니라 SAML 클라이언트를
// 등록하고, SonarQube 가 읽을 사용자 속성을 매퍼로 실어 보낸다.

func TestToolSpecs_SonarQubeLogsInWithSAML(t *testing.T) {
	spec, ok := newToolSpecs()["installing_sonarqube"]
	if !ok {
		t.Fatal("sonarqube 스펙이 없으면 스택의 SonarQube 는 로컬 계정으로만 뜬다")
	}
	if spec.Protocol != ProtocolSAML {
		t.Fatalf("SonarQube Community 는 SAML 만 받는다, got %q", spec.Protocol)
	}
	// SonarQube 가 ACS 를 sonar.core.serverBaseURL + 이 경로로 만든다. 갈라지면
	// Keycloak 이 "Invalid redirect uri" 로 응답을 보내지 않는다.
	if spec.Subdomain != "sonarqube" || spec.CallbackPath != "/oauth2/callback/saml" {
		t.Fatalf("ACS 가 SonarQube 의 콜백과 다르다: %s%s", spec.Subdomain, spec.CallbackPath)
	}
	if spec.PKCEMethod != "" {
		t.Fatal("SAML 에는 PKCE 가 없다")
	}
}

// SAML 도구는 client secret 이 없다. 이걸 모르면 시크릿 평면이 쓰지 않는 값을
// 만들고, 프로비저닝이 OpenBao 에서 그 값을 찾다가 멈춘다.
func TestSSOProvisioner_UsesClientSecretOnlyForOIDC(t *testing.T) {
	p := NewSSOProvisionerWithDomain(nil, "nullus.local")

	if p.UsesClientSecret("installing_sonarqube") {
		t.Fatal("SAML 도구에 client secret 을 요구했다")
	}
	for _, step := range []string{"installing_gitlab", "installing_grafana", "installing_argocd", "installing_harbor", "installing_gitea", "installing_jenkins", "installing_minio"} {
		if !p.UsesClientSecret(step) {
			t.Fatalf("%s 는 OIDC 클라이언트라 client secret 을 쓴다", step)
		}
	}
	if p.UsesClientSecret("installing_unknown") {
		t.Fatal("등록하지 않는 도구에 client secret 을 요구했다")
	}
}

// samlKeycloakStub 은 SAML 클라이언트 등록이 부르는 Admin API 를 흉내 낸다.
type samlKeycloakStub struct {
	existingClient  bool
	existingMappers string // GET protocol-mappers/models 응답
	defaultScopes   string // GET default-client-scopes 응답

	created        map[string]any
	updated        map[string]any
	postedMappers  []map[string]any
	putMapperPaths []string
	deletedPaths   []string
}

func (s *samlKeycloakStub) handle(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/clients"):
		if s.existingClient || s.created != nil {
			_, _ = w.Write([]byte(`[{"id":"uuid-sq","clientId":"acme-sonarqube"}]`))
		} else {
			_, _ = w.Write([]byte(`[]`))
		}
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/clients"):
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &s.created)
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodPut && strings.HasSuffix(path, "/clients/uuid-sq"):
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &s.updated)
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/protocol-mappers/models"):
		if s.existingMappers == "" {
			s.existingMappers = `[]`
		}
		_, _ = w.Write([]byte(s.existingMappers))
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/protocol-mappers/models"):
		var m map[string]any
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &m)
		s.postedMappers = append(s.postedMappers, m)
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodPut && strings.Contains(path, "/protocol-mappers/models/"):
		// 실제 Keycloak 처럼 굴린다: 갱신할 매퍼를 경로가 아니라 본문의 id 로 찾고,
		// 없으면 NullPointerException 으로 500 을 낸다(26.0 실측).
		var m map[string]any
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &m)
		if id, _ := m["id"].(string); id == "" || !strings.HasSuffix(path, "/"+id) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"unknown_error"}`))
			return true
		}
		s.putMapperPaths = append(s.putMapperPaths, path)
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/default-client-scopes"):
		if s.defaultScopes == "" {
			s.defaultScopes = `[]`
		}
		_, _ = w.Write([]byte(s.defaultScopes))
	case r.Method == http.MethodDelete:
		s.deletedPaths = append(s.deletedPaths, path)
		w.WriteHeader(http.StatusNoContent)
	default:
		return false
	}
	return true
}

func TestProvisionSSO_SonarQubeRegistersSAMLClient(t *testing.T) {
	stub := &samlKeycloakStub{}
	srv := newKeycloakStub(t, stub.handle)
	defer srv.Close()

	p := NewSSOProvisionerWithDomain(newStubClient(srv.URL), "nullus.local").WithStackSlug("acme")
	if err := p.ProvisionSSO(context.Background(), "installing_sonarqube", ""); err != nil {
		t.Fatalf("SAML 클라이언트 등록 실패: %v", err)
	}

	c := stub.created
	if c == nil {
		t.Fatal("클라이언트를 만들지 않았다")
	}
	if c["protocol"] != "saml" {
		t.Fatalf("SAML 클라이언트여야 한다, got %v", c["protocol"])
	}
	// clientId 는 SAML 에서 SP 의 entity ID 다. SonarQube 의 applicationId 와 같아야 한다.
	if c["clientId"] != "acme-sonarqube" {
		t.Fatalf("스택 단위로 나눈 clientId 여야 한다, got %v", c["clientId"])
	}
	if _, has := c["secret"]; has {
		t.Fatal("SAML 클라이언트에 client secret 을 실었다")
	}
	uris, _ := c["redirectUris"].([]any)
	if len(uris) != 1 || uris[0] != "https://sonarqube.nullus.local/oauth2/callback/saml" {
		t.Fatalf("ACS 가 등록되지 않았다: %v", c["redirectUris"])
	}

	attrs, _ := c["attributes"].(map[string]any)
	// SonarQube 는 기본으로 인증 요청에 서명하지 않는다. Keycloak 은 SAML 클라이언트를
	// 만들면 요청 서명을 요구하므로, 끄지 않으면 로그인이 "Invalid requester" 로 막힌다.
	if attrs["saml.client.signature"] != "false" {
		t.Fatalf("요청 서명 요구를 끄지 않았다: %v", attrs)
	}
	// 응답에는 서명한다 — SonarQube 는 받은 인증서로 그 서명을 검증한다.
	if attrs["saml.server.signature"] != "true" {
		t.Fatalf("응답 서명을 켜지 않았다: %v", attrs)
	}
	if attrs["saml_assertion_consumer_url_post"] != "https://sonarqube.nullus.local/oauth2/callback/saml" {
		t.Fatalf("POST 바인딩 ACS 가 없다: %v", attrs)
	}

	// SonarQube 는 login·name 속성이 없으면 로그인을 거부한다. 이름은 포트의 상수로
	// 맞춘다 — SonarQube values 가 같은 이름을 읽는다.
	want := map[string]string{
		stackport.SAMLLoginAttribute: "username",
		stackport.SAMLNameAttribute:  "username",
		stackport.SAMLEmailAttribute: "email",
	}
	got := map[string]string{}
	for _, m := range stub.postedMappers {
		if m["protocol"] != "saml" || m["protocolMapper"] != "saml-user-property-mapper" {
			t.Fatalf("SAML 사용자 속성 매퍼가 아니다: %v", m)
		}
		cfg, _ := m["config"].(map[string]any)
		name, _ := cfg["attribute.name"].(string)
		property, _ := cfg["user.attribute"].(string)
		got[name] = property
	}
	for attr, property := range want {
		if got[attr] != property {
			t.Fatalf("속성 %q 는 사용자 %q 에서 와야 한다, got %v", attr, property, got)
		}
	}
}

// 다시 돌려도 같은 결과여야 한다. 이미 있는 클라이언트와 매퍼는 갱신한다 — 다시
// 만들면 409 가 나고, 그것을 성공으로 다루면 값이 바뀌어도 반영되지 않는다.
func TestUpsertSAMLClient_UpdatesExistingClientAndMappers(t *testing.T) {
	stub := &samlKeycloakStub{
		existingClient:  true,
		existingMappers: `[{"id":"m-login","name":"login"}]`,
	}
	srv := newKeycloakStub(t, stub.handle)
	defer srv.Close()

	p := NewSSOProvisionerWithDomain(newStubClient(srv.URL), "nullus.local").WithStackSlug("acme")
	if err := p.ProvisionSSO(context.Background(), "installing_sonarqube", ""); err != nil {
		t.Fatalf("SAML 클라이언트 갱신 실패: %v", err)
	}
	if stub.created != nil {
		t.Fatal("이미 있는 클라이언트를 다시 만들었다")
	}
	if stub.updated == nil || stub.updated["protocol"] != "saml" {
		t.Fatalf("기존 클라이언트를 갱신하지 않았다: %v", stub.updated)
	}
	if len(stub.putMapperPaths) != 1 || !strings.HasSuffix(stub.putMapperPaths[0], "/m-login") {
		t.Fatalf("이미 있는 매퍼는 이름으로 찾아 갱신해야 한다: %v", stub.putMapperPaths)
	}
	if len(stub.postedMappers) != 2 {
		t.Fatalf("없는 매퍼(name·email)만 새로 만들어야 한다: %d", len(stub.postedMappers))
	}
}

// Keycloak 은 SAML 클라이언트에 기본 범위 role_list 를 붙인다. 그 매퍼는 역할마다
// Role 속성을 따로 실어, SonarQube 가 "duplicated Name" 으로 응답 전체를 거부한다.
// SonarQube 는 역할을 읽지 않으므로 이 클라이언트에서만 뺀다 — 렐름의 범위를
// 고치면 다른 SAML 클라이언트까지 바뀐다.
func TestUpsertSAMLClient_DropsRoleListScopeOnly(t *testing.T) {
	stub := &samlKeycloakStub{
		existingClient: true,
		defaultScopes:  `[{"id":"scope-role-list","name":"role_list"},{"id":"scope-org","name":"saml_organization"}]`,
	}
	srv := newKeycloakStub(t, stub.handle)
	defer srv.Close()

	p := NewSSOProvisionerWithDomain(newStubClient(srv.URL), "nullus.local").WithStackSlug("acme")
	if err := p.ProvisionSSO(context.Background(), "installing_sonarqube", ""); err != nil {
		t.Fatalf("SAML 클라이언트 등록 실패: %v", err)
	}
	if len(stub.deletedPaths) != 1 || !strings.HasSuffix(stub.deletedPaths[0], "/clients/uuid-sq/default-client-scopes/scope-role-list") {
		t.Fatalf("이 클라이언트의 role_list 만 빼야 한다: %v", stub.deletedPaths)
	}
}

// SonarQube 는 IdP 메타데이터 주소를 받지 않고 서명 인증서를 직접 받는다.
// Keycloak 은 렐름의 활성 RS256 키로 SAML 응답에 서명한다.
func TestRealmSigningCertificate_PicksActiveRS256Key(t *testing.T) {
	srv := newKeycloakStub(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/admin/realms/nullus/keys") {
			_, _ = w.Write([]byte(`{"active":{"RS256":"kid-2","HS512":"kid-h"},"keys":[
				{"kid":"kid-1","algorithm":"RS256","certificate":"OLD-CERT","status":"PASSIVE"},
				{"kid":"kid-2","algorithm":"RS256","certificate":"CURRENT-CERT","status":"ACTIVE"},
				{"kid":"kid-h","algorithm":"HS512","status":"ACTIVE"}]}`))
			return true
		}
		return false
	})
	defer srv.Close()

	cert, err := newStubClient(srv.URL).RealmSigningCertificate(context.Background())
	if err != nil {
		t.Fatalf("인증서 조회 실패: %v", err)
	}
	if cert != "CURRENT-CERT" {
		t.Fatalf("활성 RS256 키의 인증서여야 한다 — 회전 전 키로는 서명 검증이 실패한다, got %q", cert)
	}
}

func TestRealmSigningCertificate_NoActiveRS256KeyFails(t *testing.T) {
	srv := newKeycloakStub(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/keys") {
			_, _ = w.Write([]byte(`{"active":{"HS512":"kid-h"},"keys":[{"kid":"kid-h","algorithm":"HS512"}]}`))
			return true
		}
		return false
	})
	defer srv.Close()

	if _, err := newStubClient(srv.URL).RealmSigningCertificate(context.Background()); err == nil {
		t.Fatal("인증서 없이 SAML 을 켜면 SonarQube 가 모든 로그인을 거부한다 — 오류로 멈춰야 한다")
	}
}

// stack 모듈은 포트로만 본다. 어댑터가 두 기능을 그대로 넘겨야 한다.
func TestStackSSOAdapter_ExposesSAMLCapabilities(t *testing.T) {
	srv := newKeycloakStub(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/keys") {
			_, _ = w.Write([]byte(`{"active":{"RS256":"k"},"keys":[{"kid":"k","algorithm":"RS256","certificate":"CERT"}]}`))
			return true
		}
		return false
	})
	defer srv.Close()

	var adapter stackport.SSOProvisioner = NewStackSSOFactory(newStubClient(srv.URL))("nullus.local", "acme")
	if adapter.UsesClientSecret("installing_sonarqube") || !adapter.UsesClientSecret("installing_grafana") {
		t.Fatal("어댑터가 도구별 client secret 사용 여부를 넘기지 않는다")
	}
	cert, err := adapter.SAMLSigningCertificate(context.Background())
	if err != nil || cert != "CERT" {
		t.Fatalf("어댑터가 서명 인증서를 넘기지 않는다: %q %v", cert, err)
	}
}

// MinIO 의 policy 클레임 매퍼도 같은 경로로 갱신한다. 본문에 id 가 없어 다시
// 프로비저닝할 때마다 500 으로 SSO 단계가 멈췄다.
func TestUpsertOIDCClient_UpdatesExistingMapperWithItsID(t *testing.T) {
	stub := &samlKeycloakStub{
		existingClient:  true,
		existingMappers: `[{"id":"m-policy","name":"minio-policy"}]`,
	}
	srv := newKeycloakStub(t, stub.handle)
	defer srv.Close()

	err := newStubClient(srv.URL).UpsertOIDCClient(context.Background(), OIDCClientSpec{
		ClientID:        "acme-sonarqube", // 스텁이 아는 clientId 다
		Secret:          "s",
		RedirectURIs:    []string{"https://minio.nullus.local/oauth_callback"},
		ProtocolMappers: []OIDCProtocolMapper{{Name: "minio-policy", ClaimName: "policy", ClaimValue: "consoleAdmin"}},
	})
	if err != nil {
		t.Fatalf("이미 있는 매퍼 갱신이 실패했다: %v", err)
	}
	if len(stub.putMapperPaths) != 1 || !strings.HasSuffix(stub.putMapperPaths[0], "/m-policy") {
		t.Fatalf("이미 있는 매퍼를 그 id 로 갱신해야 한다: %v", stub.putMapperPaths)
	}
}
