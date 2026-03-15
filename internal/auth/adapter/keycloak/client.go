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
	"sync"
	"time"
)

const tokenRefreshSkew = 30 * time.Second

type KeycloakClient struct {
	baseURL     string
	realm       string
	adminUser   string
	adminPass   string
	httpClient  *http.Client
	accessToken string
	tokenExpiry time.Time
	mu          sync.Mutex
}

type OIDCClient struct {
	ID       string `json:"id"`
	ClientID string `json:"clientId"`
	Name     string `json:"name"`
}

func NewKeycloakClient(baseURL, realm, adminUser, adminPass string) *KeycloakClient {
	return &KeycloakClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		realm:      realm,
		adminUser:  adminUser,
		adminPass:  adminPass,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (kc *KeycloakClient) getToken(ctx context.Context) (string, error) {
	kc.mu.Lock()
	if kc.accessToken != "" && time.Now().Add(tokenRefreshSkew).Before(kc.tokenExpiry) {
		token := kc.accessToken
		kc.mu.Unlock()
		return token, nil
	}
	kc.mu.Unlock()

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", "admin-cli")
	form.Set("username", kc.adminUser)
	form.Set("password", kc.adminPass)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kc.baseURL+"/realms/master/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 60
	}

	kc.mu.Lock()
	kc.accessToken = tokenResp.AccessToken
	kc.tokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	kc.mu.Unlock()

	return tokenResp.AccessToken, nil
}

func (kc *KeycloakClient) RegisterOIDCClient(ctx context.Context, clientID string, redirectURIs []string, name string) error {
	token, err := kc.getToken(ctx)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"clientId":                  clientID,
		"name":                      name,
		"enabled":                   true,
		"protocol":                  "openid-connect",
		"publicClient":              false,
		"standardFlowEnabled":       true,
		"directAccessGrantsEnabled": false,
		"redirectUris":              redirectURIs,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal register payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/admin/realms/%s/clients", kc.baseURL, kc.realm), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create register request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("register oidc client: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusConflict {
		return nil
	}
	body, _ = io.ReadAll(resp.Body)
	return fmt.Errorf("register oidc client failed: status=%d body=%s", resp.StatusCode, string(body))
}

func (kc *KeycloakClient) DeleteOIDCClient(ctx context.Context, clientID string) error {
	token, err := kc.getToken(ctx)
	if err != nil {
		return err
	}

	lookupURL := fmt.Sprintf("%s/admin/realms/%s/clients?clientId=%s", kc.baseURL, kc.realm, url.QueryEscape(clientID))
	lookupReq, err := http.NewRequestWithContext(ctx, http.MethodGet, lookupURL, nil)
	if err != nil {
		return fmt.Errorf("create lookup request: %w", err)
	}
	lookupReq.Header.Set("Authorization", "Bearer "+token)

	lookupResp, err := kc.httpClient.Do(lookupReq)
	if err != nil {
		return fmt.Errorf("lookup client id: %w", err)
	}
	defer lookupResp.Body.Close()

	if lookupResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(lookupResp.Body)
		return fmt.Errorf("lookup client id failed: status=%d body=%s", lookupResp.StatusCode, string(body))
	}

	var clients []OIDCClient
	if err := json.NewDecoder(lookupResp.Body).Decode(&clients); err != nil {
		return fmt.Errorf("decode lookup response: %w", err)
	}
	if len(clients) == 0 {
		return nil
	}

	deleteReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/admin/realms/%s/clients/%s", kc.baseURL, kc.realm, clients[0].ID), nil)
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}
	deleteReq.Header.Set("Authorization", "Bearer "+token)

	deleteResp, err := kc.httpClient.Do(deleteReq)
	if err != nil {
		return fmt.Errorf("delete oidc client: %w", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode == http.StatusNoContent || deleteResp.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(deleteResp.Body)
	return fmt.Errorf("delete oidc client failed: status=%d body=%s", deleteResp.StatusCode, string(body))
}

func (kc *KeycloakClient) ListClients(ctx context.Context) ([]OIDCClient, error) {
	token, err := kc.getToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/admin/realms/%s/clients", kc.baseURL, kc.realm), nil)
	if err != nil {
		return nil, fmt.Errorf("create list clients request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list clients failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var clients []OIDCClient
	if err := json.NewDecoder(resp.Body).Decode(&clients); err != nil {
		return nil, fmt.Errorf("decode list clients response: %w", err)
	}
	return clients, nil
}
