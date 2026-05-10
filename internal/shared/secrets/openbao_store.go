package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenBaoStore struct {
	addr   string
	token  string
	client *http.Client
}

func (s *OpenBaoStore) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.addr+"/v1/sys/health", nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openbao health check failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	lookupReq, err := http.NewRequestWithContext(ctx, http.MethodGet, s.addr+"/v1/auth/token/lookup-self", nil)
	if err != nil {
		return err
	}
	lookupReq.Header.Set("X-Vault-Token", s.token)
	lookupResp, err := s.client.Do(lookupReq)
	if err != nil {
		return err
	}
	defer lookupResp.Body.Close()
	if lookupResp.StatusCode >= 300 {
		b, _ := io.ReadAll(lookupResp.Body)
		return fmt.Errorf("openbao token lookup failed: status=%d body=%s", lookupResp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func NewOpenBaoStore(addr, token string) *OpenBaoStore {
	return &OpenBaoStore{
		addr:  strings.TrimRight(strings.TrimSpace(addr), "/"),
		token: strings.TrimSpace(token),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *OpenBaoStore) PutToken(ctx context.Context, path, value string) error {
	mount, subpath, err := splitKVPath(path)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"data": map[string]any{"token": value}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.addr+"/v1/"+mount+"/data/"+subpath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vault-Token", s.token)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openbao write failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (s *OpenBaoStore) GetToken(ctx context.Context, path string) (string, error) {
	mount, subpath, err := splitKVPath(path)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.addr+"/v1/"+mount+"/data/"+subpath, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Vault-Token", s.token)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openbao read failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	v, _ := out.Data.Data["token"].(string)
	return v, nil
}

func splitKVPath(path string) (string, string, error) {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid openbao path: %s", path)
	}
	mount := parts[0]
	if mount == "kv" {
		mount = "secret"
	}
	return mount, strings.Join(parts[1:], "/"), nil
}
