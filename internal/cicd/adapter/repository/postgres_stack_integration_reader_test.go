package repository

import (
	"context"
	"testing"
)

// mockSecretReader stubs port.SecretReader for testing OpenBao token resolution.
type mockSecretReader struct {
	tokens map[string]string
}

func (m *mockSecretReader) GetToken(_ context.Context, path string) (string, error) {
	return m.tokens[path], nil
}

func TestBuildProfile_OpenBaoTokenResolution(t *testing.T) {
	env := "dev"
	orgID := "org-1"
	gitlabPath := "kv/nullus/" + env + "/" + orgID + "/artifacts/gitlab/token"
	argocdPath := "kv/nullus/" + env + "/" + orgID + "/pipeline/argocd/token"

	reader := &PostgresStackIntegrationReader{
		env: env,
		secretReader: &mockSecretReader{tokens: map[string]string{
			gitlabPath: "glpat-real-token",
			argocdPath: "argocd-real-token",
		}},
	}

	row := stackRow{
		ID:        "stack-1",
		OrgID:     orgID,
		ClusterID: "cluster-1",
		State:     "completed",
		Config: []byte(`{
			"access_domain": "example.internal",
			"authentication": {"provider": "openbao"},
			"artifacts": {
				"source_repository": {"name": "gitlab"},
				"container_registry": {"name": "gitlab"}
			},
			"pipeline": {
				"ci_platform": {"name": "gitlab"},
				"cd_tool": {"name": "argocd"}
			},
			"credentials": {
				"gitlab_token": "stale-config-token",
				"argocd_token": "stale-argocd-token"
			}
		}`),
	}

	profile, err := reader.buildProfile(context.Background(), row)
	if err != nil {
		t.Fatalf("buildProfile error: %v", err)
	}

	gitlab := profile.ByType("code_repository")
	if gitlab == nil {
		t.Fatal("code_repository integration missing")
	}
	if gitlab.Token != "glpat-real-token" {
		t.Errorf("gitlab token = %q, want %q (should come from OpenBao)", gitlab.Token, "glpat-real-token")
	}

	argocd := profile.ByType("cd_tool")
	if argocd == nil {
		t.Fatal("cd_tool integration missing")
	}
	if argocd.Token != "argocd-real-token" {
		t.Errorf("argocd token = %q, want %q (should come from OpenBao)", argocd.Token, "argocd-real-token")
	}
}

func TestBuildProfile_FallbackToConfigCredentials(t *testing.T) {
	// OpenBao not selected (no authentication.provider) — should use config.credentials
	reader := &PostgresStackIntegrationReader{env: "dev"}

	row := stackRow{
		ID:    "stack-2",
		OrgID: "org-2",
		Config: []byte(`{
			"access_domain": "example.internal",
			"artifacts": {
				"source_repository": {"name": "gitlab"},
				"container_registry": {"name": "gitlab"}
			},
			"pipeline": {
				"ci_platform": {"name": "gitlab"},
				"cd_tool": {"name": "argocd"}
			},
			"credentials": {
				"gitlab_token": "config-gitlab-token",
				"argocd_token": "config-argocd-token"
			}
		}`),
	}

	profile, err := reader.buildProfile(context.Background(), row)
	if err != nil {
		t.Fatalf("buildProfile error: %v", err)
	}

	gitlab := profile.ByType("code_repository")
	if gitlab.Token != "config-gitlab-token" {
		t.Errorf("gitlab token = %q, want config-gitlab-token", gitlab.Token)
	}
	argocd := profile.ByType("cd_tool")
	if argocd.Token != "config-argocd-token" {
		t.Errorf("argocd token = %q, want config-argocd-token", argocd.Token)
	}
}

func TestBuildProfile_OpenBaoFallbackWhenTokenEmpty(t *testing.T) {
	// OpenBao selected but returns empty (token not yet rotated) — should fall back to config
	reader := &PostgresStackIntegrationReader{
		env: "dev",
		secretReader: &mockSecretReader{tokens: map[string]string{}}, // empty
	}

	row := stackRow{
		ID:    "stack-3",
		OrgID: "org-3",
		Config: []byte(`{
			"authentication": {"provider": "openbao"},
			"artifacts": {
				"source_repository": {"name": "gitlab"},
				"container_registry": {"name": "gitlab"}
			},
			"pipeline": {
				"ci_platform": {"name": "gitlab"},
				"cd_tool": {"name": "argocd"}
			},
			"credentials": {
				"gitlab_token": "fallback-gitlab",
				"argocd_token": "fallback-argocd"
			}
		}`),
	}

	profile, err := reader.buildProfile(context.Background(), row)
	if err != nil {
		t.Fatalf("buildProfile error: %v", err)
	}

	gitlab := profile.ByType("code_repository")
	if gitlab.Token != "fallback-gitlab" {
		t.Errorf("gitlab token = %q, want fallback-gitlab", gitlab.Token)
	}
}
