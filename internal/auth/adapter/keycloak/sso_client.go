package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// OIDCClientSpec 은 Keycloak 에 등록할 confidential client 의 정의다.
//
// Secret 은 Nullus 가 생성해 push 한다. Keycloak 이 생성한 값을 읽어오지 않는
// 이유는 OpenBao 가 Source of Truth 여야 하고, Keycloak 이 유실돼도 OpenBao
// 에서 복원할 수 있어야 하기 때문이다.
type OIDCClientSpec struct {
	ClientID     string
	Name         string
	Secret       string
	RedirectURIs []string
	// ProtocolMappers 는 토큰에 실을 추가 클레임이다. 클라이언트 표현에 실어
	// 보내도 Keycloak 이 갱신에서 무시하므로 전용 엔드포인트로 따로 등록한다.
	ProtocolMappers []OIDCProtocolMapper
	// PKCEMethod 가 비어 있으면 PKCE 속성을 설정하지 않는다.
	// 도구마다 지원 여부가 달라 일괄 적용할 수 없다.
	PKCEMethod string
}

// OIDCProtocolMapper 는 토큰에 고정 클레임을 싣는 매퍼다.
type OIDCProtocolMapper struct {
	Name       string
	ClaimName  string
	ClaimValue string
}

// hardcodedClaimMapperPayload 는 Keycloak 프로토콜 매퍼 표현을 만든다.
//
// ID 토큰과 액세스 토큰 양쪽에 싣는다. 도구마다 어느 토큰을 읽는지 달라서
// 한쪽만 켜면 조용히 갈린다.
func hardcodedClaimMapperPayload(m OIDCProtocolMapper) map[string]any {
	return map[string]any{
		"name":           m.Name,
		"protocol":       "openid-connect",
		"protocolMapper": "oidc-hardcoded-claim-mapper",
		"config": map[string]any{
			"claim.name":           m.ClaimName,
			"claim.value":          m.ClaimValue,
			"jsonType.label":       "String",
			"id.token.claim":       "true",
			"access.token.claim":   "true",
			"userinfo.token.claim": "true",
		},
	}
}

// UpsertOIDCClient 는 클라이언트를 생성하거나 갱신한다.
//
// 갱신이 필수다. 이전 구현은 409 Conflict 를 성공으로 처리해 기존 클라이언트가
// 그대로 남았고, 그 결과 client secret 을 회전해도 Keycloak 에 반영되지 않아
// 로그인이 깨졌다. 증상이 "회전 후 SSO 실패" 로만 나타나 추적이 어렵다.
func (kc *KeycloakClient) UpsertOIDCClient(ctx context.Context, spec OIDCClientSpec) error {
	token, err := kc.getToken(ctx)
	if err != nil {
		return err
	}

	existingID, err := kc.findClientUUID(ctx, token, spec.ClientID)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"clientId":                  spec.ClientID,
		"enabled":                   true,
		"protocol":                  "openid-connect",
		"publicClient":              false,
		"standardFlowEnabled":       true,
		"directAccessGrantsEnabled": false,
		"redirectUris":              spec.RedirectURIs,
		"webOrigins":                []string{"+"},
		"attributes":                pkceAttributes(spec.PKCEMethod),
	}
	if strings.TrimSpace(spec.Name) != "" {
		payload["name"] = spec.Name
	}
	if strings.TrimSpace(spec.Secret) != "" {
		payload["secret"] = spec.Secret
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("클라이언트 페이로드 마샬 실패: %w", err)
	}

	method := http.MethodPost
	endpoint := fmt.Sprintf("%s/admin/realms/%s/clients", kc.baseURL, kc.realm)
	if existingID != "" {
		method = http.MethodPut
		endpoint = fmt.Sprintf("%s/admin/realms/%s/clients/%s", kc.baseURL, kc.realm, existingID)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("클라이언트 요청 생성 실패: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("클라이언트 upsert 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body) // #nosec G104 -- 오류 맥락용 best-effort 읽기
		return fmt.Errorf("클라이언트 upsert 실패: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if len(spec.ProtocolMappers) == 0 {
		return nil
	}
	// 생성 직후에는 UUID 를 아직 모른다(POST 응답 본문이 비어 있다).
	clientUUID := existingID
	if clientUUID == "" {
		clientUUID, err = kc.findClientUUID(ctx, token, spec.ClientID)
		if err != nil {
			return err
		}
	}
	return kc.ensureProtocolMappers(ctx, token, clientUUID, spec.ProtocolMappers)
}

// ensureProtocolMappers 는 매퍼를 등록하거나 갱신한다.
//
// 이미 있는 이름을 다시 만들면 409 가 난다. 그것을 성공으로 다루면 값이 바뀌어도
// 반영되지 않으므로, 이름으로 찾아 PUT 으로 갱신한다.
func (kc *KeycloakClient) ensureProtocolMappers(ctx context.Context, token, clientUUID string, mappers []OIDCProtocolMapper) error {
	base := fmt.Sprintf("%s/admin/realms/%s/clients/%s/protocol-mappers/models", kc.baseURL, kc.realm, clientUUID)

	existing, err := kc.listProtocolMapperIDs(ctx, token, base)
	if err != nil {
		return err
	}

	for _, m := range mappers {
		body, marshalErr := json.Marshal(hardcodedClaimMapperPayload(m))
		if marshalErr != nil {
			return fmt.Errorf("매퍼 페이로드 마샬 실패 (%s): %w", m.Name, marshalErr)
		}

		method, endpoint := http.MethodPost, base
		if id, ok := existing[m.Name]; ok {
			method, endpoint = http.MethodPut, base+"/"+id
		}

		req, reqErr := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
		if reqErr != nil {
			return fmt.Errorf("매퍼 요청 생성 실패 (%s): %w", m.Name, reqErr)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, doErr := kc.httpClient.Do(req)
		if doErr != nil {
			return fmt.Errorf("매퍼 등록 실패 (%s): %w", m.Name, doErr)
		}
		raw, _ := io.ReadAll(resp.Body) // #nosec G104 -- 오류 맥락용 best-effort 읽기
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("매퍼 등록 실패 (%s): status=%d body=%s", m.Name, resp.StatusCode, strings.TrimSpace(string(raw)))
		}
	}
	return nil
}

// listProtocolMapperIDs 는 이름 → 매퍼 ID 를 돌려준다.
func (kc *KeycloakClient) listProtocolMapperIDs(ctx context.Context, token, endpoint string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("매퍼 조회 요청 생성 실패: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("매퍼 조회 실패: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("매퍼 조회 실패: status=%d", resp.StatusCode)
	}

	var items []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("매퍼 목록 파싱 실패: %w", err)
	}

	out := make(map[string]string, len(items))
	for _, it := range items {
		out[it.Name] = it.ID
	}
	return out, nil
}

// findClientUUID 는 clientId 로 내부 UUID 를 찾는다. 없으면 빈 문자열이다.
func (kc *KeycloakClient) findClientUUID(ctx context.Context, token, clientID string) (string, error) {
	endpoint := fmt.Sprintf("%s/admin/realms/%s/clients?clientId=%s",
		kc.baseURL, kc.realm, url.QueryEscape(clientID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("클라이언트 조회 요청 생성 실패: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("클라이언트 조회 실패: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body) // #nosec G104 -- 오류 맥락용 best-effort 읽기
		return "", fmt.Errorf("클라이언트 조회 실패: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var clients []OIDCClient
	if err := json.NewDecoder(resp.Body).Decode(&clients); err != nil {
		return "", fmt.Errorf("클라이언트 목록 파싱 실패: %w", err)
	}
	if len(clients) == 0 {
		return "", nil
	}
	return clients[0].ID, nil
}

// pkceAttributes 는 클라이언트의 PKCE 요구 설정을 만든다.
//
// 쓰지 않을 때 키를 빼지 않고 빈 값으로 넣는다. 빼 버리면 이미 PKCE 로 등록된
// 클라이언트에서 기존 속성이 그대로 남아, 스펙을 고쳐도 로그인이 계속 깨진다.
func pkceAttributes(method string) map[string]any {
	return map[string]any{
		"pkce.code.challenge.method": strings.TrimSpace(method),
	}
}
