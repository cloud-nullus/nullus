// Package sastcreds 는 스택 SonarQube 의 분석 토큰을 푼다.
//
// 스택 설치(provisioning_sonarqube)가 SonarQube 에서 분석 토큰을 발급해 스택의 OpenBao 에
// 둔다. 플랫폼이 이미 갖고 있는 값을 사용자에게 다시 받을 이유가 없다 — 받지 못하면
// SONAR_TOKEN 이 비어 분석 단계가 인증에서 죽는다.
//
// registrycreds 와 같은 형태다. 발급하지 않고 읽기만 한다.
package sastcreds

import (
	"context"
	"fmt"
	"strings"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// secretProvider 는 토큰을 보관하는 시크릿 백엔드다.
const secretProvider = "openbao"

const defaultEnv = "dev"

// SecretStore 는 토큰 조회에 필요한 최소 동작만 노출한다.
type SecretStore interface {
	GetTokenForStack(ctx context.Context, provider, stackID, path string) (string, error)
}

// Resolver 는 한 스택의 분석 토큰을 읽는다.
type Resolver struct {
	secrets SecretStore
	env     string
	orgID   string
	stackID string
}

func New(secrets SecretStore, env, orgID, stackID string) *Resolver {
	return &Resolver{
		secrets: secrets,
		env:     strings.TrimSpace(env),
		orgID:   strings.TrimSpace(orgID),
		stackID: strings.TrimSpace(stackID),
	}
}

// AnalysisToken 은 스택의 분석 토큰이다. 조직·스택을 모르면 읽지 않고 빈 값이다 —
// 엉뚱한 경로의 값을 토큰으로 쓰면 원인이 먼 인증 실패가 된다.
func (r *Resolver) AnalysisToken(ctx context.Context) (string, error) {
	if r == nil || r.secrets == nil || r.orgID == "" || r.stackID == "" {
		return "", nil
	}
	env := r.env
	if env == "" {
		env = defaultEnv
	}
	// 경로 규칙은 스택 모듈의 시크릿 평면과 같다(kv/nullus/{env}/{org}/<접미사>).
	path := fmt.Sprintf("kv/nullus/%s/%s/%s", env, r.orgID, shareddomain.SonarQubeAnalysisTokenPath)
	token, err := r.secrets.GetTokenForStack(ctx, secretProvider, r.stackID, path)
	if err != nil {
		return "", fmt.Errorf("read sonarqube analysis token (%s): %w", path, err)
	}
	return strings.TrimSpace(token), nil
}
