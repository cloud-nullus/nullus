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

// 무인 설치용 부트스트랩 자격.
//
// 에어갭 무인 설치는 Admin API(자기 클러스터 등록, 스택 생성, 배포)를 호출해야
// 하는데 그 시점에 로그인할 사람이 없다. 그래서 기계 신원(service account)을
// 잠깐 만들어 쓰고 설치가 끝나면 없앤다.
//
// 정책은 "폐기 + 멱등 재발급"이다.
//   - 폐기: 쓰지 않는 admin 권한 자격을 번들·셸 히스토리·CI 로그에 남기지 않는다
//   - 멱등 재발급: 재시도·스택 추가로 다시 필요해지면 마찰 없이 다시 만든다
//
// 재발급이 쉬워야 폐기 정책이 실제로 지켜진다. 재발급이 번거로우면
// 운영자가 자격을 그냥 남겨두게 된다.

// BootstrapClientSpec 은 부트스트랩 service account 클라이언트 정의다.
type BootstrapClientSpec struct {
	ClientID string
	// Secret 은 호출자가 생성해 넘긴다. Keycloak 이 만든 값을 읽어오지 않는 것은
	// OSS 클라이언트와 같은 이유다 — 생성 주체를 Nullus 로 통일한다.
	Secret string
	// Roles 는 service account 사용자에게 매핑할 realm role 이다.
	// Admin API 호출에는 admin 이 필요하다.
	Roles []string
}

// EnsureBootstrapClient 는 부트스트랩 클라이언트를 만들거나 갱신하고
// 사용할 secret 을 돌려준다. 멱등하다.
func (kc *KeycloakClient) EnsureBootstrapClient(ctx context.Context, spec BootstrapClientSpec) (string, error) {
	token, err := kc.getToken(ctx)
	if err != nil {
		return "", err
	}

	existingID, err := kc.findClientUUID(ctx, token, spec.ClientID)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"clientId": spec.ClientID,
		"name":     "Nullus bootstrap (unattended install)",
		"enabled":  true,
		"protocol": "openid-connect",
		// 기계 신원이므로 confidential + service account 조합을 쓴다.
		"publicClient":              false,
		"serviceAccountsEnabled":    true,
		"standardFlowEnabled":       false, // 브라우저 로그인 흐름 불필요
		"directAccessGrantsEnabled": false, // 사용자 비밀번호 grant 불필요
		"secret":                    spec.Secret,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("부트스트랩 클라이언트 페이로드 마샬 실패: %w", err)
	}

	method, endpoint := http.MethodPost, fmt.Sprintf("%s/admin/realms/%s/clients", kc.baseURL, kc.realm)
	if existingID != "" {
		method = http.MethodPut
		endpoint = fmt.Sprintf("%s/admin/realms/%s/clients/%s", kc.baseURL, kc.realm, existingID)
	}
	if err := kc.doAdminJSON(ctx, token, method, endpoint, body); err != nil {
		return "", fmt.Errorf("부트스트랩 클라이언트 생성/갱신 실패: %w", err)
	}

	// role 매핑은 클라이언트가 존재해야 가능하므로 UUID 를 다시 찾는다.
	clientUUID := existingID
	if clientUUID == "" {
		clientUUID, err = kc.findClientUUID(ctx, token, spec.ClientID)
		if err != nil {
			return "", err
		}
	}
	if len(spec.Roles) > 0 {
		// 여기서 조용히 건너뛰면 토큰은 발급되는데 Admin API 에서 403 이 난다.
		// 원인이 발급 시점과 멀어져 추적이 어려우므로 명시적으로 실패시킨다.
		if clientUUID == "" {
			return "", fmt.Errorf("부트스트랩 클라이언트 %q 를 생성 후 찾지 못해 role 을 매핑할 수 없습니다", spec.ClientID)
		}
		if err := kc.mapRealmRolesToServiceAccount(ctx, token, clientUUID, spec.Roles); err != nil {
			return "", err
		}
	}

	return spec.Secret, nil
}

// IssueBootstrapToken 은 client_credentials grant 로 access token 을 받는다.
//
// 이 토큰 자체는 수명이 짧아 별도 폐기가 필요 없다.
// 실제 폐기 대상은 클라이언트(=지속되는 자격)다.
func (kc *KeycloakClient) IssueBootstrapToken(ctx context.Context, clientID, clientSecret string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", kc.baseURL, kc.realm)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("토큰 요청 생성 실패: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("부트스트랩 토큰 발급 실패: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body) // #nosec G104 -- 오류 맥락용 best-effort 읽기
		return "", fmt.Errorf("부트스트랩 토큰 발급 실패: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("토큰 응답 파싱 실패: %w", err)
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return "", fmt.Errorf("발급된 토큰이 비어 있습니다")
	}
	return out.AccessToken, nil
}

// RevokeBootstrapClient 는 부트스트랩 클라이언트를 삭제한다.
//
// 이미 없으면 성공으로 처리한다 — 설치 스크립트가 여러 번 돌아도 안전해야 한다.
func (kc *KeycloakClient) RevokeBootstrapClient(ctx context.Context, clientID string) error {
	token, err := kc.getToken(ctx)
	if err != nil {
		return err
	}

	clientUUID, err := kc.findClientUUID(ctx, token, clientID)
	if err != nil {
		return err
	}
	if clientUUID == "" {
		return nil // 이미 폐기됨
	}

	endpoint := fmt.Sprintf("%s/admin/realms/%s/clients/%s", kc.baseURL, kc.realm, clientUUID)
	return kc.doAdminJSON(ctx, token, http.MethodDelete, endpoint, nil)
}

// mapRealmRolesToServiceAccount 는 service account 사용자에 realm role 을 매핑한다.
//
// 이 매핑이 없으면 토큰은 발급되지만 Admin API 에서 403 이 난다.
func (kc *KeycloakClient) mapRealmRolesToServiceAccount(ctx context.Context, token, clientUUID string, roles []string) error {
	saUser, err := kc.serviceAccountUserID(ctx, token, clientUUID)
	if err != nil {
		return err
	}
	if saUser == "" {
		return fmt.Errorf("service account 사용자를 찾지 못했습니다")
	}

	payload := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		roleRepr, err := kc.realmRole(ctx, token, role)
		if err != nil {
			return err
		}
		payload = append(payload, roleRepr)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("role 매핑 페이로드 마샬 실패: %w", err)
	}
	endpoint := fmt.Sprintf("%s/admin/realms/%s/users/%s/role-mappings/realm", kc.baseURL, kc.realm, saUser)
	if err := kc.doAdminJSON(ctx, token, http.MethodPost, endpoint, body); err != nil {
		return fmt.Errorf("realm role 매핑 실패: %w", err)
	}
	return nil
}

func (kc *KeycloakClient) serviceAccountUserID(ctx context.Context, token, clientUUID string) (string, error) {
	endpoint := fmt.Sprintf("%s/admin/realms/%s/clients/%s/service-account-user", kc.baseURL, kc.realm, clientUUID)
	raw, err := kc.getAdminJSON(ctx, token, endpoint)
	if err != nil {
		return "", err
	}
	var user struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &user); err != nil {
		return "", fmt.Errorf("service account 사용자 파싱 실패: %w", err)
	}
	return user.ID, nil
}

func (kc *KeycloakClient) realmRole(ctx context.Context, token, role string) (map[string]any, error) {
	endpoint := fmt.Sprintf("%s/admin/realms/%s/roles/%s", kc.baseURL, kc.realm, url.PathEscape(role))
	raw, err := kc.getAdminJSON(ctx, token, endpoint)
	if err != nil {
		return nil, fmt.Errorf("realm role %q 조회 실패: %w", role, err)
	}
	var repr map[string]any
	if err := json.Unmarshal(raw, &repr); err != nil {
		return nil, fmt.Errorf("realm role 파싱 실패: %w", err)
	}
	return repr, nil
}

// doAdminJSON 은 Admin API 에 본문 있는 요청을 보낸다.
func (kc *KeycloakClient) doAdminJSON(ctx context.Context, token, method, endpoint string, body []byte) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("요청 생성 실패: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	raw, _ := io.ReadAll(resp.Body) // #nosec G104 -- 오류 맥락용 best-effort 읽기
	return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
}

// getAdminJSON 은 Admin API 에서 JSON 을 읽는다.
func (kc *KeycloakClient) getAdminJSON(ctx context.Context, token, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("요청 생성 실패: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body) // #nosec G104 -- 오류 맥락용 best-effort 읽기
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}
