// Package registrycreds 는 스택이 설치한 레지스트리의 자격증명을 푼다.
//
// 스택이 Harbor·Nexus 를 직접 설치하면 그 관리자 자격증명은 설치 과정
// (provisioning_secrets)이 OpenBao 에 만들어 둔다. 플랫폼이 이미 갖고 있는 값을
// 사용자에게 다시 받아 적게 할 이유가 없다 — 화면은 묻지도 않으므로, 받지 못하면
// HARBOR_USERNAME/HARBOR_PASSWORD 가 비어 CI 의 docker login 이 죽는다.
//
// jenkins.CredentialResolver 와 같은 형태다. 발급하지 않고 읽기만 한다.
package registrycreds

import (
	"context"
	"fmt"
	"strings"
)

// secretProvider 는 자격증명을 보관하는 시크릿 백엔드다.
const secretProvider = "openbao"

const defaultEnv = "dev"

// adminUser 는 두 레지스트리 모두 차트가 만드는 관리자 계정 이름이다
// (stack/domain 의 HarborAdminUser / NexusAdminUser).
const adminUser = "admin"

// SecretStore 는 자격증명 조회에 필요한 최소 동작만 노출한다.
type SecretStore interface {
	GetTokenForStack(ctx context.Context, provider, stackID, path string) (string, error)
}

// Resolver 는 한 스택의 레지스트리 CI 변수 이름을 실제 값으로 바꾼다.
//
// 스택에 묶어 둔다. 조직·환경·스택은 번들을 조립하는 쪽이 이미 알고 있고,
// 그것을 호출부까지 다시 실어 나르면 유스케이스가 시크릿 경로 규칙을 알게 된다.
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

// knownRegistries 는 플랫폼이 자격증명을 소유한 레지스트리들이다.
//
// 경로는 스택 모듈이 쓰는 것과 정확히 같아야 한다
// (helm/secret-provisioning.go 의 managedSecrets). 갈라지면 설치는 성공하는데
// CI 로그인만 실패하는, 원인이 먼 실패가 된다.
var knownRegistries = []struct {
	usernameVar string
	passwordVar string
	pathSuffix  string
}{
	{"HARBOR_USERNAME", "HARBOR_PASSWORD", "artifacts/harbor/admin-password"},
	{"NEXUS_USERNAME", "NEXUS_PASSWORD", "artifacts/nexus/admin-password"},
}

// Resolve 는 요청된 변수 중 플랫폼이 아는 것만 채워 돌려준다.
//
// 모르는 레지스트리(사용자가 지정한 외부 주소 등)의 변수는 손대지 않는다.
// 조용히 빈 값을 채우면 CI 가 엉뚱한 자격증명으로 로그인을 시도해, 원인이 한 겹
// 더 멀어진다.
func (r *Resolver) Resolve(ctx context.Context, variables []string) (map[string]string, error) {
	if r == nil || r.secrets == nil || len(variables) == 0 {
		return nil, nil
	}
	if r.orgID == "" || r.stackID == "" {
		return nil, nil
	}

	wanted := make(map[string]struct{}, len(variables))
	for _, v := range variables {
		wanted[strings.TrimSpace(v)] = struct{}{}
	}

	resolved := make(map[string]string, 2)
	for _, registry := range knownRegistries {
		_, needUser := wanted[registry.usernameVar]
		_, needPass := wanted[registry.passwordVar]
		if !needUser && !needPass {
			continue
		}

		path := secretPath(r.env, r.orgID, registry.pathSuffix)
		password, err := r.secrets.GetTokenForStack(ctx, secretProvider, r.stackID, path)
		if err != nil {
			return nil, fmt.Errorf("read registry credential (%s): %w", path, err)
		}
		if strings.TrimSpace(password) == "" {
			// 값이 없으면 채우지 않는다. 빈 문자열을 등록하면 set -u 는 통과하지만
			// 로그인이 실패해, 원인이 한 겹 더 멀어진다.
			continue
		}

		if needUser {
			resolved[registry.usernameVar] = adminUser
		}
		if needPass {
			resolved[registry.passwordVar] = password
		}
	}

	if len(resolved) == 0 {
		return nil, nil
	}
	return resolved, nil
}

func secretPath(env, orgID, suffix string) string {
	env = strings.TrimSpace(env)
	if env == "" {
		env = defaultEnv
	}
	return fmt.Sprintf("kv/nullus/%s/%s/%s", env, strings.TrimSpace(orgID), suffix)
}
