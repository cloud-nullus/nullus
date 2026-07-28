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
	// PKCEMethod 가 비어 있으면 PKCE 속성을 설정하지 않는다.
	// 도구마다 지원 여부가 달라 일괄 적용할 수 없다.
	PKCEMethod string
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

	if resp.StatusCode < 300 {
		return nil
	}
	raw, _ := io.ReadAll(resp.Body) // #nosec G104 -- 오류 맥락용 best-effort 읽기
	return fmt.Errorf("클라이언트 upsert 실패: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
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

// pkceAttributes 는 PKCE 설정을 만든다.
// 빈 값이면 속성을 비워 두어 도구별 지원 차이를 흡수한다.
func pkceAttributes(method string) map[string]any {
	attrs := map[string]any{}
	if m := strings.TrimSpace(method); m != "" {
		attrs["pkce.code.challenge.method"] = m
	}
	return attrs
}
