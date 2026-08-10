package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/cloud-nullus/draft/internal/shared/secrets"
)

// SecretProvider 는 토큰을 보관하는 시크릿 백엔드다.
const SecretProvider = "openbao"

// SecretStore 는 토큰 조회에 필요한 최소 동작만 노출한다.
//
// 다른 모듈의 동일 타입을 재사용하지 않는다 — 모듈 간 직접 import 를 피하기 위해
// CI/CD 컨텍스트가 자기 계약을 소유한다.
type SecretStore interface {
	GetTokenForStack(ctx context.Context, provider, stackID, path string) (string, error)
}

// TokenIssuer 는 보관된 GitHub PAT 를 돌려준다.
//
// 이름은 GitLab 쪽과 맞췄지만 실제로 발급하지는 않는다 — GitHub 은 SaaS 라
// 우리가 토큰을 만들 수 없다. 사용자가 등록한 PAT 를 읽는 것이 전부이며,
// 그래서 Force 재발급도 의미가 없다(만료·폐기되면 사람이 다시 등록해야 한다).
type TokenIssuer struct {
	secrets SecretStore
}

// NewTokenIssuer 는 TokenIssuer 를 만든다.
func NewTokenIssuer(secrets SecretStore) *TokenIssuer {
	return &TokenIssuer{secrets: secrets}
}

// TokenSecretPath 는 PAT 의 시크릿 경로다.
//
// 값을 기록하는 쪽은 stack 모듈(설치 마지막의 토큰 소스 등록)이다. 경로를
// 양쪽에 따로 적으면 한쪽만 바뀌어도 컴파일은 통과하고 런타임에 "등록된
// 토큰이 없다" 로만 드러나므로 공유 규약을 그대로 쓴다.
func TokenSecretPath(env, orgID string) string {
	return secrets.GitHubAPITokenPath(env, orgID)
}

// EnsureToken 은 보관된 PAT 를 돌려준다.
//
// 없으면 새로 만들지 않고 실패한다 — 조용히 빈 토큰으로 진행하면 리포 생성이
// 401 로 죽고, 오류가 프로비저닝 버그처럼 보여 원인을 찾기 어렵다.
func (t *TokenIssuer) EnsureToken(ctx context.Context, spec port.SCMTokenSpec) (string, error) {
	orgID := strings.TrimSpace(spec.OrgID)
	if orgID == "" {
		return "", fmt.Errorf("org_id is required to locate the github token")
	}
	stackID := strings.TrimSpace(spec.StackID)
	if stackID == "" {
		return "", fmt.Errorf("stack_id is required to locate the secret store")
	}
	if t.secrets == nil {
		return "", fmt.Errorf("시크릿 저장소가 설정되지 않아 GitHub 토큰을 읽을 수 없습니다")
	}

	path := TokenSecretPath(spec.Env, orgID)
	stored, err := t.secrets.GetTokenForStack(ctx, SecretProvider, stackID, path)
	if err != nil {
		return "", fmt.Errorf(
			"GitHub PAT 를 읽지 못했습니다 (%s) — 조직 설정에서 GitHub 토큰을 등록했는지 확인하세요: %w",
			path, err)
	}

	token := strings.TrimSpace(stored)
	if token == "" {
		return "", fmt.Errorf(
			"조직 %s 에 등록된 GitHub PAT 가 없습니다 (%s) — repo·workflow·read:packages 스코프의 "+
				"토큰을 먼저 등록하세요", orgID, path)
	}
	return token, nil
}
