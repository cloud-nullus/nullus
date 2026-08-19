package keycloak

import "testing"

// MinIO 로그인이 이렇게 거부됐다:
//
//	Policy claim missing from the JWT token, credentials will not be generated
//
// MinIO 는 토큰의 policy 클레임으로 부여할 정책을 정한다. Keycloak 은 기본적으로
// 그런 클레임을 보내지 않는다. 클라이언트를 만들 때 매퍼도 함께 붙여야 한다.
//
// 다른 도구는 클레임을 요구하지 않으므로 매퍼도 도구별이다.

func TestToolSpecs_MinIOCarriesPolicyClaimMapper(t *testing.T) {
	spec, ok := newToolSpecs()["installing_minio"]
	if !ok {
		t.Fatal("minio 스펙이 없다")
	}
	if len(spec.ProtocolMappers) == 0 {
		t.Fatal("policy 클레임 매퍼가 없으면 MinIO 가 자격을 발급하지 않는다")
	}

	m := spec.ProtocolMappers[0]
	if m.ClaimName != "policy" {
		t.Fatalf("MinIO 는 policy 클레임을 읽는다, got %q", m.ClaimName)
	}
	if m.ClaimValue == "" {
		t.Fatal("값이 없으면 클레임이 비어 나가 같은 실패가 난다")
	}
}

// 클레임도 역할도 요구하지 않는 도구에는 매퍼를 붙이지 않는다 — 불필요한
// 클레임은 토큰만 키운다.
//
// 넷은 자동 온보딩이 일반 사용자를 만들고 그 기본값이 합리적이라, 지금은 역할을
// 넘길 이유가 없다(GitLab·Harbor·Gitea 는 일반 사용자, Grafana 는 기본 역할).
func TestToolSpecs_ToolsWithoutClaimNeedsHaveNoMappers(t *testing.T) {
	specs := newToolSpecs()
	for _, step := range []string{"installing_gitlab", "installing_harbor", "installing_gitea"} {
		if len(specs[step].ProtocolMappers) != 0 {
			t.Fatalf("%s 에 불필요한 매퍼가 붙었다", step)
		}
	}
}

// ID 토큰과 액세스 토큰 양쪽에 실려야 한다. MinIO 는 ID 토큰을 읽는데, 한쪽만
// 켜면 어느 쪽을 읽는지에 따라 조용히 갈린다.
func TestProtocolMapperPayload_TargetsBothTokens(t *testing.T) {
	cfg := hardcodedClaimMapperPayload(OIDCProtocolMapper{
		Name: "minio-policy", ClaimName: "policy", ClaimValue: "consoleAdmin",
	})["config"].(map[string]any)

	for _, key := range []string{"id.token.claim", "access.token.claim"} {
		if cfg[key] != "true" {
			t.Fatalf("%s 가 켜져 있지 않다", key)
		}
	}
	if cfg["claim.name"] != "policy" || cfg["claim.value"] != "consoleAdmin" {
		t.Fatalf("클레임이 잘못 구성됐다: %v", cfg)
	}
}

// 매퍼는 클라이언트 표현에 실어 보내도 갱신되지 않는다(Keycloak 이 client
// update 에서 무시한다). 전용 엔드포인트로 따로 등록해야 한다.
//
// 이미 있으면 이름으로 찾아 갱신한다 — 다시 만들면 409 가 나고, 그것을 성공으로
// 다루면 값이 바뀌어도 반영되지 않는다.
func TestProvisionSSO_RegistersMappersViaDedicatedEndpoint(t *testing.T) {
	src := readKeycloakSource(t, "sso_client.go")

	if !contains(src, "protocol-mappers/models") {
		t.Fatal("매퍼 전용 엔드포인트를 쓰지 않는다 — 클라이언트 갱신만으로는 반영되지 않는다")
	}
	if !contains(src, "http.MethodPut") {
		t.Fatal("이미 있는 매퍼를 갱신하지 않으면 값이 바뀌어도 반영되지 않는다")
	}
}

// ArgoCD 는 policy.default 와 policy.csv 가 모두 비어 있어 OIDC 사용자에게 권한이
// 0 이다. 로그인은 되는데 아무것도 못 본다. 권한을 주려면 토큰에 역할이 실려야
// 하는데, 클라이언트에 역할 매퍼가 없어 realm 역할이 나가지 않는다.
//
// Keycloak 은 realm_access.roles 를 기본적으로 액세스 토큰에만 싣는다. 도구는
// 대개 ID 토큰을 읽으므로 별도 매퍼로 명시해야 한다.
func TestToolSpecs_ToolsThatUseRolesCarryRoleMapper(t *testing.T) {
	specs := newToolSpecs()

	for _, step := range []string{"installing_argocd"} {
		var found bool
		for _, m := range specs[step].ProtocolMappers {
			if m.Kind == MapperKindRealmRoles {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s 에 역할 매퍼가 없으면 권한을 줄 근거가 토큰에 없다", step)
		}
	}
}

// ArgoCD 는 groups 클레임을 읽는다(argocd-rbac-cm 의 scopes: [groups]).
func TestRealmRoleMapperPayload(t *testing.T) {
	cfg := realmRoleMapperPayload(OIDCProtocolMapper{
		Kind: MapperKindRealmRoles, Name: "realm-roles", ClaimName: "groups",
	})["config"].(map[string]any)

	if cfg["claim.name"] != "groups" {
		t.Fatalf("claim.name=%v", cfg["claim.name"])
	}
	// 역할은 여럿일 수 있다. 단일값으로 두면 하나만 실려 나머지 역할이 사라진다.
	if cfg["multivalued"] != "true" {
		t.Fatal("multivalued 가 아니면 역할이 하나만 실린다")
	}
	for _, k := range []string{"id.token.claim", "access.token.claim"} {
		if cfg[k] != "true" {
			t.Fatalf("%s 가 꺼져 있다", k)
		}
	}
}

// 매퍼 종류가 갈리므로 페이로드 생성도 갈려야 한다.
func TestMapperPayload_DispatchesByKind(t *testing.T) {
	hard := mapperPayload(OIDCProtocolMapper{Kind: MapperKindHardcoded, Name: "p", ClaimName: "policy", ClaimValue: "consoleAdmin"})
	if hard["protocolMapper"] != "oidc-hardcoded-claim-mapper" {
		t.Fatalf("고정 클레임 매퍼가 아니다: %v", hard["protocolMapper"])
	}
	roles := mapperPayload(OIDCProtocolMapper{Kind: MapperKindRealmRoles, Name: "r", ClaimName: "groups"})
	if roles["protocolMapper"] != "oidc-usermodel-realm-role-mapper" {
		t.Fatalf("realm 역할 매퍼가 아니다: %v", roles["protocolMapper"])
	}
}
