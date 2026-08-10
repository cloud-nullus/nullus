package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/crypto/nacl/box"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// ErrAccessTokenUnsupported 는 GitHub 에 리포 범위 토큰 발급 API 가 없음을 알린다.
//
// GitLab 의 Project Access Token 에 대응하는 것이 GitHub 에는 없다. 워크플로는
// 내장 GITHUB_TOKEN 을 쓰고, 클러스터 쪽 인증은 스택에 등록된 PAT 를 재사용한다.
// 호출자가 이 오류를 보고 그 경로로 갈라져야 한다.
var ErrAccessTokenUnsupported = errors.New(
	"github 은 리포 범위 액세스 토큰 발급을 지원하지 않습니다")

// secretKeyLength 는 GitHub 이 돌려주는 공개키의 바이트 길이다(Curve25519).
const secretKeyLength = 32

// SetProjectVariable 은 Actions 시크릿을 등록하거나 이미 있으면 갱신한다.
//
// GitHub 은 평문 시크릿을 받지 않는다. 리포의 공개키를 받아 libsodium sealed box
// 로 암호화한 값만 올릴 수 있다 — 평문으로 PUT 하면 400 이 난다.
// PUT 자체가 upsert 라 GitLab 처럼 POST→PUT 폴백이 필요 없다.
func (c *Client) SetProjectVariable(ctx context.Context, projectID string, v port.ProjectVariable) error {
	repo := strings.Trim(strings.TrimSpace(projectID), "/")
	if repo == "" {
		return fmt.Errorf("repository (owner/name) is required")
	}
	name, err := normalizeSecretName(v.Key)
	if err != nil {
		return err
	}

	var pub struct {
		KeyID string `json:"key_id"`
		Key   string `json:"key"`
	}
	found, err := c.get(ctx, repoPath(repo, "/actions/secrets/public-key"), &pub)
	if err != nil {
		return fmt.Errorf("get actions public key for %s: %w", repo, err)
	}
	if !found {
		return fmt.Errorf("%s 의 Actions 공개키를 찾을 수 없습니다 — PAT 에 repo 스코프가 있는지 확인하세요", repo)
	}

	encrypted, err := sealSecret(pub.Key, v.Value)
	if err != nil {
		return fmt.Errorf("encrypt secret %q for %s: %w", name, repo, err)
	}

	endpoint := repoPath(repo, "/actions/secrets/"+url.PathEscape(name))
	if err := c.send(ctx, http.MethodPut, endpoint, map[string]any{
		"encrypted_value": encrypted,
		"key_id":          pub.KeyID,
	}, nil); err != nil {
		return fmt.Errorf("set secret %q on %s: %w", name, repo, err)
	}
	return nil
}

// CreateProjectAccessToken 은 GitHub 에서 지원되지 않는다.
//
// 조용히 빈 문자열을 돌려주면 호출자가 자격증명 없이 Argo CD 를 구성해
// 동기화가 "authentication required" 로 실패한다. 명시적으로 알린다.
func (c *Client) CreateProjectAccessToken(
	_ context.Context,
	_ string,
	_ port.AccessTokenSpec,
) (string, error) {
	return "", ErrAccessTokenUnsupported
}

// sealSecret 은 base64 공개키로 값을 sealed box 암호화해 base64 로 돌려준다.
func sealSecret(publicKeyB64, value string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKeyB64))
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	if len(raw) != secretKeyLength {
		return "", fmt.Errorf("공개키 길이가 %d 바이트여야 하는데 %d 입니다", secretKeyLength, len(raw))
	}

	var key [secretKeyLength]byte
	copy(key[:], raw)

	sealed, err := box.SealAnonymous(nil, []byte(value), &key, rand.Reader)
	if err != nil {
		return "", fmt.Errorf("seal secret: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// normalizeSecretName 은 GitHub 의 시크릿 이름 규칙에 맞춘다.
//
// 대문자·숫자·밑줄만 허용되고, 숫자로 시작하거나 GITHUB_ 접두사를 쓸 수 없다.
// 규칙을 어긴 이름은 422 로 거부되는데, 오류 본문이 원인을 분명히 알려주지
// 않아 파이프라인이 시크릿 없이 도는 것처럼 보인다.
func normalizeSecretName(key string) (string, error) {
	name := strings.ToUpper(strings.TrimSpace(key))
	name = strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(name)
	if name == "" {
		return "", fmt.Errorf("secret name is required")
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "", fmt.Errorf("github 시크릿 이름은 숫자로 시작할 수 없습니다 (%q)", key)
	}
	if strings.HasPrefix(name, "GITHUB_") {
		return "", fmt.Errorf("github 시크릿 이름에 GITHUB_ 접두사를 쓸 수 없습니다 (%q)", key)
	}
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_':
		default:
			return "", fmt.Errorf("github 시크릿 이름에 쓸 수 없는 문자입니다: %q", key)
		}
	}
	return name, nil
}
