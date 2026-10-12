package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	stackport "github.com/cloud-nullus/draft/internal/stack/port"
)

// SAMLClientSpec 은 Keycloak 에 등록할 SAML 클라이언트의 정의다.
//
// SonarQube Community 처럼 SSO 로 SAML 만 받는 도구에 쓴다. OIDC 클라이언트와 달리
// client secret 이 없다 — 신뢰는 렐름의 서명 인증서로 맺는다(RealmSigningCertificate).
type SAMLClientSpec struct {
	// ClientID 는 SP 의 entity ID 다. 도구 쪽 설정(SonarQube 의 applicationId)과 같아야
	// 한다 — 다르면 Keycloak 이 "Invalid requester" 로 요청을 거부한다.
	ClientID string
	Name     string
	// ACSURL 은 IdP 가 응답을 POST 할 도구의 주소다.
	ACSURL string
}

// samlUserAttributes 는 SAML 응답에 실을 사용자 속성이다(속성 이름 ← 사용자 속성).
//
// 속성 이름은 도구가 읽는 이름이라 포트의 상수를 쓴다. name 은 username 에서
// 가져온다 — 이름(firstName)이 빈 계정이 있는데, SonarQube 는 name 속성이 없으면
// 로그인을 거부한다.
var samlUserAttributes = []struct{ attribute, property string }{
	{stackport.SAMLLoginAttribute, "username"},
	{stackport.SAMLNameAttribute, "username"},
	{stackport.SAMLEmailAttribute, "email"},
}

// samlRoleListScope 는 Keycloak 이 SAML 클라이언트에 기본으로 붙이는 범위다.
//
// 그 매퍼는 역할마다 Role 속성을 따로 싣는데, SonarQube 는 이름이 겹치는 속성을
// 보면 응답 전체를 거부한다("Found an Attribute element with duplicated Name").
// SonarQube 는 역할을 읽지 않으므로 이 클라이언트에서만 뺀다 — 렐름의 범위를
// 고치면 다른 SAML 클라이언트까지 바뀐다.
const samlRoleListScope = "role_list"

// samlUserPropertyMapperPayload 는 사용자 속성 하나를 SAML 속성으로 싣는 매퍼다.
func samlUserPropertyMapperPayload(attribute, property string) map[string]any {
	return map[string]any{
		"name":           attribute,
		"protocol":       "saml",
		"protocolMapper": "saml-user-property-mapper",
		"config": map[string]any{
			"user.attribute":       property,
			"attribute.name":       attribute,
			"attribute.nameformat": "Basic",
			"friendly.name":        attribute,
		},
	}
}

// UpsertSAMLClient 는 SAML 클라이언트를 만들거나 갱신하고, 도구가 읽을 사용자 속성을
// 매퍼로 붙인다. 다시 돌려도 같은 결과다.
func (kc *KeycloakClient) UpsertSAMLClient(ctx context.Context, spec SAMLClientSpec) error {
	token, err := kc.getToken(ctx)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"clientId":     spec.ClientID,
		"enabled":      true,
		"protocol":     "saml",
		"redirectUris": []string{spec.ACSURL},
		"attributes": map[string]any{
			// SonarQube 는 기본으로 인증 요청에 서명하지 않는다. Keycloak 은 SAML
			// 클라이언트에 요청 서명을 요구하는 것이 기본이라, 끄지 않으면 로그인이
			// "Invalid requester" 로 막힌다.
			"saml.client.signature": "false",
			// 응답에는 렐름 키로 서명한다. SonarQube 가 받은 인증서로 이 서명을 검증한다.
			"saml.server.signature":            "true",
			"saml.signature.algorithm":         "RSA_SHA256",
			"saml.force.post.binding":          "true",
			"saml.authnstatement":              "true",
			"saml_assertion_consumer_url_post": spec.ACSURL,
		},
	}
	if strings.TrimSpace(spec.Name) != "" {
		payload["name"] = spec.Name
	}

	clientUUID, err := kc.upsertClient(ctx, token, spec.ClientID, payload)
	if err != nil {
		return err
	}
	if clientUUID == "" {
		// 생성 응답은 본문이 비어 UUID 를 모른다.
		if clientUUID, err = kc.findClientUUID(ctx, token, spec.ClientID); err != nil {
			return err
		}
		if clientUUID == "" {
			return fmt.Errorf("만든 SAML 클라이언트를 찾지 못했습니다 (%s)", spec.ClientID)
		}
	}

	mappers := make([]map[string]any, 0, len(samlUserAttributes))
	for _, a := range samlUserAttributes {
		mappers = append(mappers, samlUserPropertyMapperPayload(a.attribute, a.property))
	}
	if err := kc.ensureProtocolMapperPayloads(ctx, token, clientUUID, mappers); err != nil {
		return err
	}
	return kc.dropDefaultClientScope(ctx, token, clientUUID, samlRoleListScope)
}

// dropDefaultClientScope 는 이 클라이언트의 기본 범위에서 하나를 뺀다. 없으면 그대로 둔다.
func (kc *KeycloakClient) dropDefaultClientScope(ctx context.Context, token, clientUUID, scopeName string) error {
	endpoint := fmt.Sprintf("%s/admin/realms/%s/clients/%s/default-client-scopes", kc.baseURL, kc.realm, clientUUID)
	raw, err := kc.getAdminJSON(ctx, token, endpoint)
	if err != nil {
		return fmt.Errorf("클라이언트 기본 범위 조회 실패: %w", err)
	}

	var scopes []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &scopes); err != nil {
		return fmt.Errorf("클라이언트 기본 범위 파싱 실패: %w", err)
	}
	for _, scope := range scopes {
		if scope.Name != scopeName {
			continue
		}
		if err := kc.doAdminJSON(ctx, token, http.MethodDelete, endpoint+"/"+scope.ID, nil); err != nil {
			return fmt.Errorf("클라이언트 기본 범위 %s 제거 실패: %w", scopeName, err)
		}
	}
	return nil
}

// RealmSigningCertificate 는 렐름이 서명에 쓰는 활성 RS256 키의 인증서다(머리말 없는
// base64 DER).
//
// Keycloak 은 SAML 응답에 이 키로 서명한다. 키를 회전하면 도구에 넣은 인증서도
// 바꿔야 한다 — 회전 전 인증서로는 서명 검증이 실패한다.
func (kc *KeycloakClient) RealmSigningCertificate(ctx context.Context) (string, error) {
	token, err := kc.getToken(ctx)
	if err != nil {
		return "", err
	}
	raw, err := kc.getAdminJSON(ctx, token, fmt.Sprintf("%s/admin/realms/%s/keys", kc.baseURL, kc.realm))
	if err != nil {
		return "", fmt.Errorf("렐름 키 조회 실패: %w", err)
	}

	var keys struct {
		Active map[string]string `json:"active"`
		Keys   []struct {
			Kid         string `json:"kid"`
			Certificate string `json:"certificate"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(raw, &keys); err != nil {
		return "", fmt.Errorf("렐름 키 파싱 실패: %w", err)
	}

	kid := keys.Active["RS256"]
	if kid == "" {
		return "", fmt.Errorf("렐름 %s 에 활성 RS256 키가 없어 SAML 응답을 검증할 인증서가 없습니다", kc.realm)
	}
	for _, key := range keys.Keys {
		if key.Kid == kid && strings.TrimSpace(key.Certificate) != "" {
			return strings.TrimSpace(key.Certificate), nil
		}
	}
	return "", fmt.Errorf("렐름 %s 의 활성 RS256 키(%s)에 인증서가 없습니다", kc.realm, kid)
}
