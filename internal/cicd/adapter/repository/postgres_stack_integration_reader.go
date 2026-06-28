package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// PostgresStackIntegrationReader implements port.StackIntegrationReader.
// Token resolution order:
//  1. OpenBao (if secretReader is set and stack uses openbao auth)
//  2. stack.config.credentials JSON (fallback / OpenBao 미선택 시)
type PostgresStackIntegrationReader struct {
	pool         *pgxpool.Pool
	secretReader port.SecretReader
	env          string // "dev" | "prod" — OpenBao KV path prefix
}

type IntegrationReaderOption func(*PostgresStackIntegrationReader)

func WithSecretReader(r port.SecretReader) IntegrationReaderOption {
	return func(s *PostgresStackIntegrationReader) { s.secretReader = r }
}

func WithEnv(env string) IntegrationReaderOption {
	return func(s *PostgresStackIntegrationReader) { s.env = env }
}

func NewPostgresStackIntegrationReader(pool *pgxpool.Pool, opts ...IntegrationReaderOption) *PostgresStackIntegrationReader {
	r := &PostgresStackIntegrationReader{pool: pool, env: "dev"}
	for _, o := range opts {
		o(r)
	}
	return r
}

type stackRow struct {
	ID        string
	OrgID     string
	ClusterID string
	State     string
	Config    []byte
	Namespace *string
}

// stackConfigShape is the minimal subset of stack config we need here.
// Mirrors domain.StackConfig but lives in the CI/CD adapter to avoid cross-module imports.
type stackConfigShape struct {
	AccessDomain string `json:"access_domain"`
	Authentication *struct {
		Provider string `json:"provider"` // "openbao" when OpenBao is selected
	} `json:"authentication"`
	Artifacts struct {
		SourceRepository  struct{ Name string `json:"name"` } `json:"source_repository"`
		ContainerRegistry struct{ Name string `json:"name"` } `json:"container_registry"`
	} `json:"artifacts"`
	Pipeline struct {
		CIPlatform struct{ Name string `json:"name"` } `json:"ci_platform"`
		CDTool     struct{ Name string `json:"name"` } `json:"cd_tool"`
	} `json:"pipeline"`
	Credentials struct {
		GitLabToken string `json:"gitlab_token"`
		ArgoCDToken string `json:"argocd_token"`
	} `json:"credentials"`
}

func (r *PostgresStackIntegrationReader) GetStackIntegrationProfile(ctx context.Context, stackID string) (*port.StackIntegrationProfile, error) {
	const q = `
		SELECT s.id, s.org_id, s.cluster_id, s.state, s.config, hn.namespace
		FROM stacks s
		LEFT JOIN helm_namespaces hn ON hn.stack_id = s.id
		WHERE s.id = $1
		LIMIT 1`

	var row stackRow
	var rawCfg []byte
	err := r.pool.QueryRow(ctx, q, stackID).Scan(
		&row.ID, &row.OrgID, &row.ClusterID, &row.State, &rawCfg, &row.Namespace,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return r.getWithoutNamespace(ctx, stackID)
		}
		return nil, fmt.Errorf("query stack integration: %w", err)
	}
	row.Config = rawCfg
	return r.buildProfile(ctx, row)
}

func (r *PostgresStackIntegrationReader) getWithoutNamespace(ctx context.Context, stackID string) (*port.StackIntegrationProfile, error) {
	const q = `SELECT id, org_id, cluster_id, state, config FROM stacks WHERE id = $1`
	var row stackRow
	err := r.pool.QueryRow(ctx, q, stackID).Scan(
		&row.ID, &row.OrgID, &row.ClusterID, &row.State, &row.Config,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query stack: %w", err)
	}
	return r.buildProfile(ctx, row)
}

func (r *PostgresStackIntegrationReader) buildProfile(ctx context.Context, row stackRow) (*port.StackIntegrationProfile, error) {
	var cfg stackConfigShape
	if err := json.Unmarshal(row.Config, &cfg); err != nil {
		return nil, fmt.Errorf("parse stack config: %w", err)
	}

	namespace := "nullus-stack"
	if row.Namespace != nil && *row.Namespace != "" {
		namespace = *row.Namespace
	}

	useOpenBao := r.secretReader != nil &&
		cfg.Authentication != nil &&
		strings.EqualFold(strings.TrimSpace(cfg.Authentication.Provider), "openbao")

	gitlabToken := cfg.Credentials.GitLabToken
	argocdToken := cfg.Credentials.ArgoCDToken

	if useOpenBao {
		env := r.env
		if env == "" {
			env = "dev"
		}
		orgID := row.OrgID

		gitlabProvider := normalizeProvider(cfg.Artifacts.SourceRepository.Name)
		if gitlabProvider != "" {
			path := fmt.Sprintf("kv/nullus/%s/%s/artifacts/%s/token", env, orgID, gitlabProvider)
			if tok, err := r.secretReader.GetToken(ctx, path); err == nil && tok != "" && tok != "managed-by-nullus" {
				gitlabToken = tok
			}
		}

		argocdProvider := normalizeProvider(cfg.Pipeline.CDTool.Name)
		if argocdProvider != "" {
			path := fmt.Sprintf("kv/nullus/%s/%s/pipeline/%s/token", env, orgID, argocdProvider)
			if tok, err := r.secretReader.GetToken(ctx, path); err == nil && tok != "" && tok != "managed-by-nullus" {
				argocdToken = tok
			}
		}
	}

	gitlabEndpoint := buildEndpoint(cfg.AccessDomain, namespace, "code_repository", cfg.Artifacts.SourceRepository.Name)
	argocdEndpoint := buildEndpoint(cfg.AccessDomain, namespace, "cd_tool", cfg.Pipeline.CDTool.Name)
	imageRegistryEndpoint := buildEndpoint(cfg.AccessDomain, namespace, "image_registry", cfg.Artifacts.ContainerRegistry.Name)

	integrations := []port.StackIntegration{
		{
			ComponentType: "code_repository",
			Provider:      cfg.Artifacts.SourceRepository.Name,
			Endpoint:      gitlabEndpoint,
			Token:         gitlabToken,
		},
		{
			ComponentType: "image_registry",
			Provider:      cfg.Artifacts.ContainerRegistry.Name,
			Endpoint:      imageRegistryEndpoint,
			Token:         gitlabToken,
		},
		{
			ComponentType: "ci_platform",
			Provider:      cfg.Pipeline.CIPlatform.Name,
			Endpoint:      gitlabEndpoint,
			Token:         gitlabToken,
		},
		{
			ComponentType: "cd_tool",
			Provider:      cfg.Pipeline.CDTool.Name,
			Endpoint:      argocdEndpoint,
			Token:         argocdToken,
		},
	}

	return &port.StackIntegrationProfile{
		StackID:      row.ID,
		OrgID:        row.OrgID,
		ClusterID:    row.ClusterID,
		State:        row.State,
		Integrations: integrations,
	}, nil
}

func buildEndpoint(accessDomain, namespace, componentType, provider string) string {
	if provider == "" {
		return ""
	}
	normalized := strings.ToLower(strings.ReplaceAll(provider, " ", "-"))

	if accessDomain != "" {
		switch componentType {
		case "code_repository", "ci_platform":
			if isGitLab(normalized) {
				return "https://gitlab." + accessDomain
			}
		case "image_registry":
			if isGitLab(normalized) {
				return "https://registry." + accessDomain
			}
			if normalized == "harbor" {
				return "https://harbor." + accessDomain
			}
		case "cd_tool":
			if normalized == "argocd" || normalized == "argo-cd" {
				return "https://argocd." + accessDomain
			}
		}
		return "https://" + normalized + "." + accessDomain
	}

	// In-cluster fallback
	switch componentType {
	case "code_repository", "ci_platform":
		if isGitLab(normalized) {
			return fmt.Sprintf("http://gitlab-webservice-default.%s.svc:8181", namespace)
		}
	case "cd_tool":
		if normalized == "argocd" || normalized == "argo-cd" {
			return fmt.Sprintf("http://argocd-server.%s.svc", namespace)
		}
	}
	return ""
}

func normalizeProvider(name string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "-")
}

func isGitLab(normalized string) bool {
	return normalized == "gitlab" || strings.HasPrefix(normalized, "gitlab-")
}
