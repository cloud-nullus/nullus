package rotation

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

type GitLabReissuer struct {
	baseURL string
	client  *http.Client
}

func NewGitLabReissuer(baseURL string) *GitLabReissuer {
	b := strings.TrimSpace(baseURL)
	if b == "" {
		b = "https://gitlab.com/api/v4"
	}
	return &GitLabReissuer{baseURL: strings.TrimRight(b, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

func (r *GitLabReissuer) Reissue(ctx context.Context, input ReissueInput) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider != "gitlab" && provider != "gitlab-ce" && provider != "gitlab-ci" && provider != "gitlab-registry" {
		return "", ErrReissueUnsupported
	}
	if strings.TrimSpace(input.CurrentToken) == "" {
		return "", fmt.Errorf("gitlab reissue requires current token")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/personal_access_tokens/self/rotate", bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", input.CurrentToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gitlab rotate failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Token) == "" {
		return "", fmt.Errorf("gitlab rotate returned empty token")
	}
	return out.Token, nil
}
