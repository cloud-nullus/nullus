package usecase

import (
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// BuildStackTokenSourceInputs derives the token sources that must be written for
// a stack with openbao auth enabled.
func BuildStackTokenSourceInputs(stack *domain.Stack, env string) []port.TokenSourceInput {
	if stack == nil {
		return nil
	}

	cfg, ok := stackConfigFromInterface(stack.Config)
	if !ok || cfg.Authentication == nil || strings.TrimSpace(strings.ToLower(cfg.Authentication.Provider)) != "openbao" {
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
		inputs = append(inputs, port.TokenSourceInput{
			OrgID:         stack.OrgID,
			Module:        module,
			Provider:      provider,
			Path:          fmt.Sprintf("kv/nullus/%s/%s/%s/%s/token", env, stack.OrgID, module, provider),
			TokenType:     "reissue",
			Status:        "healthy",
			SecretManager: strings.TrimSpace(strings.ToLower(cfg.Authentication.Provider)),
			// 실제 토큰 값은 provider 발급 시점에 회전 컨트롤러가 기록한다.
			// 여기서는 경로만 등록하고 값은 비워 둔다.
			TokenValue: "",
			ClusterID:  stack.ClusterID,
			Namespace:  stack.Namespace,
		})
	}

	appendTool("artifacts", cfg.Artifacts.SourceRepository.Name)
	appendTool("artifacts", cfg.Artifacts.ContainerRegistry.Name)
	appendTool("pipeline", cfg.Pipeline.CIPlatform.Name)
	appendTool("pipeline", cfg.Pipeline.CDTool.Name)

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
