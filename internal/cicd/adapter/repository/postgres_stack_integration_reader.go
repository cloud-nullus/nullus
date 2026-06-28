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
// It reads stack.config JSONB directly to build the integration profile.
// Token fields (GITLAB_TOKEN, ARGOCD_TOKEN) are expected in stack.config under
// "credentials": { "gitlab_token": "...", "argocd_token": "..." }.
type PostgresStackIntegrationReader struct {
	pool *pgxpool.Pool
}

func NewPostgresStackIntegrationReader(pool *pgxpool.Pool) *PostgresStackIntegrationReader {
	return &PostgresStackIntegrationReader{pool: pool}
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
	Artifacts    struct {
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
			// Try without helm_namespaces join
			return r.getWithoutNamespace(ctx, stackID)
		}
		return nil, fmt.Errorf("query stack integration: %w", err)
	}
	row.Config = rawCfg
	return r.buildProfile(row)
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
	return r.buildProfile(row)
}

func (r *PostgresStackIntegrationReader) buildProfile(row stackRow) (*port.StackIntegrationProfile, error) {
	var cfg stackConfigShape
	if err := json.Unmarshal(row.Config, &cfg); err != nil {
		return nil, fmt.Errorf("parse stack config: %w", err)
	}

	namespace := "nullus-stack"
	if row.Namespace != nil && *row.Namespace != "" {
		namespace = *row.Namespace
	}

	gitlabEndpoint := buildEndpoint(cfg.AccessDomain, namespace, "code_repository", cfg.Artifacts.SourceRepository.Name)
	argocdEndpoint := buildEndpoint(cfg.AccessDomain, namespace, "cd_tool", cfg.Pipeline.CDTool.Name)
	imageRegistryEndpoint := buildEndpoint(cfg.AccessDomain, namespace, "image_registry", cfg.Artifacts.ContainerRegistry.Name)

	integrations := []port.StackIntegration{
		{
			ComponentType: "code_repository",
			Provider:      cfg.Artifacts.SourceRepository.Name,
			Endpoint:      gitlabEndpoint,
			Token:         cfg.Credentials.GitLabToken,
		},
		{
			ComponentType: "image_registry",
			Provider:      cfg.Artifacts.ContainerRegistry.Name,
			Endpoint:      imageRegistryEndpoint,
			Token:         cfg.Credentials.GitLabToken,
		},
		{
			ComponentType: "ci_platform",
			Provider:      cfg.Pipeline.CIPlatform.Name,
			Endpoint:      gitlabEndpoint,
			Token:         cfg.Credentials.GitLabToken,
		},
		{
			ComponentType: "cd_tool",
			Provider:      cfg.Pipeline.CDTool.Name,
			Endpoint:      argocdEndpoint,
			Token:         cfg.Credentials.ArgoCDToken,
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

func isGitLab(normalized string) bool {
	return normalized == "gitlab" || strings.HasPrefix(normalized, "gitlab-")
}

