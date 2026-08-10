package usecase

import (
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// SourceControlCredentials 는 설치 요청이 들고 온 외부 SCM 자격증명이다.
//
// stacks.config 를 거치지 않는다 — 그 구조는 평문 JSONB 로 저장되므로 토큰이
// 들어가면 DB 를 읽을 수 있는 누구에게나 노출된다. 배포 요청에서 받아 이
// 구조로 들고 다니다가 OpenBao 에만 기록한다.
type SourceControlCredentials struct {
	// PersonalAccessToken 은 GitHub PAT 다 (repo·workflow·read:packages).
	PersonalAccessToken string
}

// BuildStackTokenSourceInputs 는 스택 설치가 끝난 뒤 등록할 토큰 소스를 만든다.
//
// 두 종류가 조건이 다르다. 회전 대상 항목은 사용자가 인증 공급자로 OpenBao 를
// 고른 스택에만 만들고, 사용자가 직접 준 GitHub PAT 는 그 선택과 무관하게 만든다.
// 자세한 이유는 각 분기의 주석에 있다.
func BuildStackTokenSourceInputs(
	stack *domain.Stack,
	env string,
	creds SourceControlCredentials,
) []port.TokenSourceInput {
	if stack == nil {
		return nil
	}

	cfg, ok := stackConfigFromInterface(stack.Config)
	if !ok {
		return nil
	}

	env = strings.TrimSpace(env)
	if env == "" {
		env = "dev"
	}

	inputs := []port.TokenSourceInput{}
	appendTool := func(module, provider string) {
		provider = strings.TrimSpace(strings.ToLower(provider))
		if provider == "" {
			return
		}
		provider = strings.ReplaceAll(provider, " ", "-")
		// 외부 SaaS 는 이 경로로 등록하지 않는다. 여기서 만드는 항목은 회전
		// 컨트롤러가 재발급할 수 있는 클러스터 내부 도구를 전제하는데, GitHub 은
		// 우리가 토큰을 만들 수 없다. 게다가 아래 GitHub 전용 항목과 provider 가
		// 같아 소유자 정보가 없는 행이 하나 더 생기고, 연동 설정을 읽는 쪽이
		// 둘 중 어느 것을 집을지 알 수 없게 된다.
		if isExternalSCMProvider(provider) {
			return
		}
		inputs = append(inputs, port.TokenSourceInput{
			OrgID:         stack.OrgID,
			Module:        module,
			Provider:      provider,
			Path:          fmt.Sprintf("kv/nullus/%s/%s/%s/%s/token", env, stack.OrgID, module, provider),
			TokenType:     "reissue",
			Status:        "healthy",
			SecretManager: secretManagerFor(cfg),
			// 실제 토큰 값은 provider 발급 시점에 회전 컨트롤러가 기록한다.
			// 여기서는 경로만 등록하고 값은 비워 둔다.
			TokenValue: "",
			ClusterID:  stack.ClusterID,
			Namespace:  stack.Namespace,
		})
	}

	// 회전 대상 항목은 사용자가 OpenBao 를 고른 스택에만 만든다. 이 항목들은
	// 값이 빈 경로만 등록해 두고 회전 컨트롤러가 나중에 채우는 구조라, 범위를
	// 넓히면 영영 채워지지 않는 행이 늘고 회전 실패 로그만 쌓인다.
	if usesOpenBaoAuthProvider(cfg) {
		appendTool("artifacts", cfg.Artifacts.SourceRepository.Name)
		appendTool("artifacts", cfg.Artifacts.ContainerRegistry.Name)
		appendTool("pipeline", cfg.Pipeline.CIPlatform.Name)
		appendTool("pipeline", cfg.Pipeline.CDTool.Name)
	}

	// GitHub PAT 는 위 게이트 밖이다.
	//
	// 시크릿 평면(OpenBao)은 authentication.provider 와 무관하게 항상 설치되고,
	// 마법사의 기본값은 provider='' 다 — 프론트 정규화기가 빈 값이면 authentication
	// 키를 통째로 뺀다. 그래서 게이트 안에 두면 사용자가 마법사에 직접 넣은 토큰이
	// 조용히 사라진다. 등록 실패가 아니라 "등록할 것이 없음" 이라 설치 로그에
	// 경고조차 남지 않고, 한참 뒤 파이프라인 생성에서 "등록된 PAT 가 없다" 로만
	// 드러난다. 게다가 이 값은 회전으로 복구할 수도 없다 — 사용자가 준 것이 전부다.
	if gh, ok := gitHubTokenSourceInput(stack, cfg, env, creds); ok {
		inputs = append(inputs, gh)
	}

	// bootstrap 자격증명은 더 이상 여기서 하드코딩하지 않는다.
	// provisioning_secrets 스텝이 값을 생성해 OpenBao 에 기록하고 ESO 가
	// Kubernetes Secret 으로 복제하므로, 같은 값을 두 곳에서 관리하지 않는다.

	seen := map[string]struct{}{}
	unique := make([]port.TokenSourceInput, 0, len(inputs))
	for _, input := range inputs {
		key := input.OrgID + ":" + input.Module + ":" + input.Provider + ":" + input.Path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, input)
	}

	return unique
}

// usesOpenBaoAuthProvider 는 사용자가 인증 공급자로 OpenBao 를 골랐는지 본다.
//
// 시크릿 평면이 깔렸는지와는 다른 질문이다 — 그쪽은 항상 깔린다.
func usesOpenBaoAuthProvider(cfg domain.StackConfig) bool {
	if cfg.Authentication == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cfg.Authentication.Provider), "openbao")
}

// secretManagerFor 는 토큰 값을 보관할 시크릿 저장소 이름이다.
//
// 설정이 비어도 OpenBao 로 본다. 시크릿 평면은 authentication.provider 와 무관하게
// 항상 설치되므로(PostgreSQL/MinIO 가 provisioning_secrets 가 만든 Secret 을
// existingSecret 으로 참조한다) 값을 넣을 곳은 언제나 있다.
func secretManagerFor(cfg domain.StackConfig) string {
	if cfg.Authentication != nil {
		if provider := strings.TrimSpace(strings.ToLower(cfg.Authentication.Provider)); provider != "" {
			return provider
		}
	}
	return "openbao"
}

// isExternalSCMProvider 는 클러스터 밖에서 도는 소스 저장소인지 본다.
func isExternalSCMProvider(provider string) bool {
	switch provider {
	case "github", "github-actions", "ghcr":
		return true
	}
	return false
}

// gitHubTokenSourceInput 은 조직 GitHub PAT 의 등록 항목을 만든다.
//
// GitLab 항목과 두 가지가 다르다. 하나, 값이 실려 있다 — GitHub 은 SaaS 라
// 회전 컨트롤러가 나중에 발급해 채울 수 없고 사용자가 준 것이 전부다.
// 둘, metadata 에 organization 을 담는다 — 토큰만으로는 어느 org 에 리포를
// 만들지 알 수 없다.
func gitHubTokenSourceInput(
	stack *domain.Stack,
	cfg domain.StackConfig,
	env string,
	creds SourceControlCredentials,
) (port.TokenSourceInput, bool) {
	if !usesGitHubSourceRepository(cfg) {
		return port.TokenSourceInput{}, false
	}
	token := strings.TrimSpace(creds.PersonalAccessToken)
	if token == "" {
		return port.TokenSourceInput{}, false
	}

	owner, apiBaseURL := "", ""
	if cfg.SourceControl != nil {
		owner = strings.TrimSpace(cfg.SourceControl.Owner)
		apiBaseURL = strings.TrimSpace(cfg.SourceControl.APIBaseURL)
	}
	if owner == "" {
		// 소유자를 모르면 토큰만 저장해도 리포를 만들 수 없다. 반쪽짜리 행을
		// 남기면 연동이 등록된 것처럼 보여 진짜 원인을 가린다.
		return port.TokenSourceInput{}, false
	}

	return port.TokenSourceInput{
		OrgID:    stack.OrgID,
		Module:   "cicd",
		Provider: "github",
		// cicd 모듈이 이 경로로 읽는다. 공유 규약을 그대로 쓴다.
		Path:          secrets.GitHubAPITokenPath(env, stack.OrgID),
		TokenType:     "pat",
		Status:        "healthy",
		SecretManager: secretManagerFor(cfg),
		TokenValue:    token,
		// OpenBao 는 스택마다 배포되므로 이 스택의 저장소에 넣어야
		// 파이프라인 프로비저닝이 같은 곳을 본다.
		StackID: stack.ID,
		Metadata: map[string]string{
			"owner":        owner,
			"api_base_url": apiBaseURL,
		},
		ClusterID: stack.ClusterID,
		Namespace: stack.Namespace,
	}, true
}

// usesGitHubSourceRepository 는 스택이 GitHub 을 소스 저장소로 골랐는지 본다.
func usesGitHubSourceRepository(cfg domain.StackConfig) bool {
	name := strings.ToLower(strings.TrimSpace(cfg.Artifacts.SourceRepository.Name))
	name = strings.NewReplacer(" ", "-", "_", "-").Replace(name)
	switch name {
	case "github", "github-enterprise", "github-enterprise-server", "github-com":
		return true
	}
	return false
}
