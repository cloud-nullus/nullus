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
			TokenValue:    "managed-by-nullus",
		})
	}

	appendTool("artifacts", cfg.Artifacts.SourceRepository.Name)
	appendTool("artifacts", cfg.Artifacts.ContainerRegistry.Name)
	appendTool("pipeline", cfg.Pipeline.CIPlatform.Name)
	appendTool("pipeline", cfg.Pipeline.CDTool.Name)

	namespace := strings.TrimSpace(stack.Namespace)
	if namespace == "" {
		namespace = "nullus"
	}

	appendBootstrap := func(module, provider, pathSuffix, value string) {
		provider = strings.TrimSpace(strings.ToLower(provider))
		if provider == "" || strings.TrimSpace(value) == "" {
			return
		}
		provider = strings.ReplaceAll(provider, " ", "-")
		inputs = append(inputs, port.TokenSourceInput{
			OrgID:         stack.OrgID,
			Module:        module,
			Provider:      provider,
			Path:          fmt.Sprintf("kv/nullus/%s/%s/%s/%s/%s", env, stack.OrgID, module, provider, pathSuffix),
			TokenType:     "bootstrap",
			Status:        "healthy",
			SecretManager: strings.TrimSpace(strings.ToLower(cfg.Authentication.Provider)),
			TokenValue:    value,
		})
	}

	if cfg.Storage != nil && strings.TrimSpace(strings.ToLower(cfg.Storage.Database.Mode)) == "create" {
		appendBootstrap("storage", "postgresql", "access", fmt.Sprintf("host=nullus-postgresql.%s.svc.cluster.local port=5432 db=gitlabhq_production username=gitlab password=nullus-gitlab-password", namespace)) // #nosec G101 -- default bootstrap credential, matches Helm default value
	}
	if cfg.Artifacts.StorageBackend.Enabled && strings.EqualFold(strings.TrimSpace(cfg.Artifacts.StorageBackend.Name), "minio") {
		appendBootstrap("artifacts", "minio", "access", fmt.Sprintf("endpoint=http://nullus-minio.%s.svc.cluster.local:9000 access_key=nullus-admin secret_key=nullus-minio-secret", namespace)) // #nosec G101 -- default bootstrap credential, matches Helm default value
	}
	cdTool := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(cfg.Pipeline.CDTool.Name)), " ", "-")
	if cfg.Pipeline.CDTool.Enabled && (cdTool == "argocd" || cdTool == "argo-cd") {
		appendBootstrap("pipeline", "argocd", "access", fmt.Sprintf("url=http://argo-cd-argocd-server.%s.svc.cluster.local username=admin password_secret=argocd-initial-admin-secret", namespace))
	}
	if cfg.Artifacts.SourceRepository.Enabled && (strings.EqualFold(strings.TrimSpace(cfg.Artifacts.SourceRepository.Name), "gitlab") || strings.EqualFold(strings.TrimSpace(cfg.Artifacts.SourceRepository.Name), "gitlab-ce")) {
		appendBootstrap("artifacts", "gitlab", "access", fmt.Sprintf("url=http://gitlab-webservice-default.%s.svc:8181 username=root password=nullus-gitlab-password", namespace)) // #nosec G101 -- default bootstrap credential, matches Helm default value
	}

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
