package rotation

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type GitHubReissuer struct {
	apiBase string
	client  *http.Client
}

func NewGitHubReissuer(apiBase string) *GitHubReissuer {
	b := strings.TrimSpace(apiBase)
	if b == "" {
		b = "https://api.github.com"
	}
	return &GitHubReissuer{apiBase: strings.TrimRight(b, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

func (r *GitHubReissuer) Reissue(ctx context.Context, input ReissueInput) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider != "github" && provider != "github-actions" {
		return "", ErrReissueUnsupported
	}
	appID, installationID, privateKeyPEM, err := githubAppMetadata(input.Metadata)
	if err != nil {
		return "", err
	}

	appJWT, err := buildGitHubAppJWT(appID, privateKeyPEM)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", r.apiBase, installationID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body) // #nosec G104 -- best-effort body read for error context
		return "", fmt.Errorf("github installation token issue failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Token) == "" {
		return "", fmt.Errorf("github installation token is empty")
	}
	return out.Token, nil
}

func githubAppMetadata(metadata map[string]any) (appID int64, installationID int64, privateKeyPEM string, err error) {
	if metadata == nil {
		return 0, 0, "", fmt.Errorf("github reissue requires metadata")
	}
	readInt := func(keys ...string) int64 {
		for _, key := range keys {
			v, ok := metadata[key]
			if !ok {
				continue
			}
			switch t := v.(type) {
			case float64:
				return int64(t)
			case int64:
				return t
			case string:
				n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
				return n
			}
		}
		return 0
	}
	readString := func(keys ...string) string {
		for _, key := range keys {
			v, ok := metadata[key]
			if !ok {
				continue
			}
			if s, ok := v.(string); ok {
				if strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
		return ""
	}

	appID = readInt("github_app_id", "app_id")
	installationID = readInt("github_installation_id", "installation_id")
	privateKeyPEM = readString("github_app_private_key_pem", "private_key_pem")
	if appID <= 0 || installationID <= 0 || strings.TrimSpace(privateKeyPEM) == "" {
		return 0, 0, "", fmt.Errorf("github reissue metadata missing (app_id, installation_id, private_key_pem)")
	}
	return appID, installationID, privateKeyPEM, nil
}

func buildGitHubAppJWT(appID int64, privateKeyPEM string) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return "", err
	}
	return signGitHubJWT(appID, key)
}

func signGitHubJWT(appID int64, key *rsa.PrivateKey) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": appID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(key)
}
