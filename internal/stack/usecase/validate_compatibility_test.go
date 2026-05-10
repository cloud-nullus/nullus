package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// stubClusterReader lets us simulate the admin module without wiring pgx.
type stubClusterReader struct {
	summary *port.ClusterSummary
	err     error
}

func (s *stubClusterReader) GetClusterSummary(_ context.Context, _ string) (*port.ClusterSummary, error) {
	return s.summary, s.err
}

// hasCode returns true when any issue has the given code.
func hasCode(issues []ValidationIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestValidateCompatibility_MatchFound(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"source_repository": "GitLab CE",
			"ci_platform":       "GitLab CI",
			"cd_tool":           "Argo CD",
		},
	})

	require.NoError(t, err)
	assert.True(t, out.Compatible)
	assert.NotNil(t, out.Matrix)
	assert.NotEmpty(t, out.Message)
}

func TestValidateCompatibility_NoMatch(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"ci_platform": "Jenkins",
			"cd_tool":     "Spinnaker",
		},
	})

	require.NoError(t, err)
	assert.False(t, out.Compatible)
	assert.Nil(t, out.Matrix)
}

func TestValidateCompatibility_EmptyTools(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	_, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{},
	})

	require.Error(t, err)
	// Error message updated in F8-F3 to reflect the new persisted-mode
	// alternative: "tools or stack_id is required".
	assert.Contains(t, err.Error(), "tools map or stack_id is required")
}

func TestValidateCompatibility_GitHubArgoCD(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"source_repository": "GitHub",
			"ci_platform":       "GitHub Actions",
			"cd_tool":           "Argo CD",
		},
	})

	require.NoError(t, err)
	assert.True(t, out.Compatible)
	assert.NotNil(t, out.Matrix)
	assert.Equal(t, "untested", out.Matrix.Status)
}

// ---------------------------------------------------------------------------
// F8 Task 3: Pre-Deploy Gate ARM64 / node architecture checks.
// ---------------------------------------------------------------------------

// Verified matrix + pure amd64 cluster => pass. GitLab tools are amd64-only
// (Narwhal baseline) and the matrix status is verified, so the gate should
// stay at pass with no TOOL_ARCH_UNSUPPORTED issue.
func TestValidateCompatibility_Arch_SingleAMD64Cluster_Passes(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"source_repository":  "GitLab CE",
			"ci_platform":        "GitLab CI",
			"container_registry": "GitLab Registry",
		},
		NodeArchitectures: []string{"amd64"},
	})

	require.NoError(t, err)
	assert.Equal(t, "pass", out.Overall.State)
	assert.False(t, hasCode(out.Issues, "TOOL_ARCH_UNSUPPORTED"))
}

// Verified matrix with amd64-only Harbor / GitLab tools on a mixed
// amd64+arm64 cluster => fail (hard block). The Pre-Deploy Gate must not let
// the user proceed with a verified matrix that cannot schedule onto every
// worker arch.
func TestValidateCompatibility_Arch_MixedCluster_VerifiedMatrix_Fails(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"source_repository":  "GitLab CE",
			"ci_platform":        "GitLab CI",
			"container_registry": "GitLab Registry",
		},
		NodeArchitectures: []string{"amd64", "arm64"},
	})

	require.NoError(t, err)
	assert.False(t, out.Compatible)
	assert.Equal(t, "fail", out.Overall.State)
	assert.True(t, hasCode(out.Issues, "TOOL_ARCH_UNSUPPORTED"))
}

// Untested matrix + mixed cluster + Harbor (amd64-only) => warn, not fail.
// The base verdict is already warn from MATRIX_UNTESTED; arch miss on an
// unverified matrix stays recoverable via explicit ack in the UI.
func TestValidateCompatibility_Arch_UntestedMatrixMixedCluster_StaysWarn(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"source_repository":  "GitHub",
			"ci_platform":        "GitHub Actions",
			"container_registry": "Harbor",
		},
		NodeArchitectures: []string{"amd64", "arm64"},
	})

	require.NoError(t, err)
	assert.Equal(t, "warn", out.Overall.State)
	assert.True(t, hasCode(out.Issues, "TOOL_ARCH_UNSUPPORTED"))
	// Severity on untested-matrix arch miss is "warning" so the UI doesn't
	// render a red error.
	for _, issue := range out.Issues {
		if issue.Code == "TOOL_ARCH_UNSUPPORTED" {
			assert.Equal(t, "warning", issue.Severity)
		}
	}
}

// ClusterID resolves through ClusterReader. When the admin DB reports
// amd64-only the gate should pass for a GitLab-only verified matrix.
func TestValidateCompatibility_Arch_ResolvesViaClusterID(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	reader := &stubClusterReader{summary: &port.ClusterSummary{
		ID:                "cluster-1",
		NodeArchitectures: []string{"amd64"},
	}}
	uc := NewValidateCompatibility(repo, WithClusterReader(reader))

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"source_repository":  "GitLab CE",
			"ci_platform":        "GitLab CI",
			"container_registry": "GitLab Registry",
		},
		ClusterID: "cluster-1",
	})

	require.NoError(t, err)
	assert.Equal(t, "pass", out.Overall.State)
	assert.Equal(t, []string{"amd64"}, out.NodeArchitectures)
}

// Unknown cluster architectures (cluster id supplied but discovery never
// ran) should be reported as a warning with code CLUSTER_ARCH_UNKNOWN and
// downgrade pass → warn. This matches the "arch unknown" policy the
// handler exposes as "Refresh discovery to continue."
func TestValidateCompatibility_Arch_ClusterIDButUnknownArchs_Warns(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	reader := &stubClusterReader{summary: &port.ClusterSummary{
		ID:                "cluster-2",
		NodeArchitectures: nil, // never discovered
	}}
	uc := NewValidateCompatibility(repo, WithClusterReader(reader))

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"source_repository":  "GitLab CE",
			"ci_platform":        "GitLab CI",
			"container_registry": "GitLab Registry",
		},
		ClusterID: "cluster-2",
	})

	require.NoError(t, err)
	assert.Equal(t, "warn", out.Overall.State)
	assert.True(t, hasCode(out.Issues, "CLUSTER_ARCH_UNKNOWN"))
}

// ---------------------------------------------------------------------------
// F8-F3: persisted mode (Tools empty, StackID set → load from StackRepository).
// ---------------------------------------------------------------------------

func seedPersistedStack(t *testing.T, clusterID string, tools []domain.ToolConfig) (*repository.MemoryStackRepository, string) {
	t.Helper()
	stackRepo := repository.NewMemoryStackRepository()
	stack := &domain.Stack{
		ID:        "stack-under-test",
		Name:      "test-stack",
		OrgID:     "org-1",
		ClusterID: clusterID,
		Namespace: "nullus",
		Tools:     tools,
		State:     domain.StatePending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, stackRepo.Create(context.Background(), stack))
	return stackRepo, stack.ID
}

func TestValidateCompatibility_PersistedMode_LoadsStackTools(t *testing.T) {
	stackRepo, stackID := seedPersistedStack(t, "cluster-1", []domain.ToolConfig{
		{Category: "source_repository", Name: "GitLab CE"},
		{Category: "ci_platform", Name: "GitLab CI"},
		{Category: "container_registry", Name: "GitLab Registry"},
	})
	reader := &stubClusterReader{summary: &port.ClusterSummary{
		ID:                "cluster-1",
		NodeArchitectures: []string{"amd64"},
	}}
	uc := NewValidateCompatibility(
		repository.NewMemoryCompatibilityRepository(),
		WithClusterReader(reader),
		WithStackRepository(stackRepo),
	)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{StackID: stackID})
	require.NoError(t, err)
	require.NotNil(t, out.Matrix)
	assert.Equal(t, "pass", out.Overall.State)
	// ClusterID fallback from stack kicks in → cluster arch is part of the result.
	assert.Equal(t, []string{"amd64"}, out.NodeArchitectures)
}

func TestValidateCompatibility_PersistedMode_DerivesToolsFromConfigSelections(t *testing.T) {
	stackRepo := repository.NewMemoryStackRepository()
	stack := &domain.Stack{
		ID:        "stack-config-derived",
		Name:      "config-derived-stack",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "nullus",
		Tools:     nil,
		Config: domain.StackConfig{
			Artifacts: domain.ArtifactsConfig{
				SourceRepository:  domain.ToolSelection{Name: "gitlab"},
				ContainerRegistry: domain.ToolSelection{Name: "gitlab-registry"},
				StorageBackend:    domain.ToolSelection{Name: "minio"},
			},
			Pipeline: domain.PipelineConfig{
				CIPlatform: domain.ToolSelection{Name: "gitlab-ci"},
				CDTool:     domain.ToolSelection{Name: "argocd"},
			},
			Monitoring: domain.MonitoringConfig{
				Collection:    domain.ToolSelection{Name: "prometheus"},
				Visualization: domain.ToolSelection{Name: "grafana"},
			},
		},
		State:     domain.StatePending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, stackRepo.Create(context.Background(), stack))

	reader := &stubClusterReader{summary: &port.ClusterSummary{ID: "cluster-1", NodeArchitectures: []string{"amd64"}}}
	uc := NewValidateCompatibility(
		repository.NewMemoryCompatibilityRepository(),
		WithClusterReader(reader),
		WithStackRepository(stackRepo),
	)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{StackID: stack.ID})
	require.NoError(t, err)
	require.NotNil(t, out.Matrix)
	assert.Equal(t, "pass", out.Overall.State)
}

func TestValidateCompatibility_PersistedMode_StackNotFound(t *testing.T) {
	stackRepo := repository.NewMemoryStackRepository()
	uc := NewValidateCompatibility(
		repository.NewMemoryCompatibilityRepository(),
		WithStackRepository(stackRepo),
	)

	_, err := uc.Execute(context.Background(), ValidateCompatibilityInput{StackID: "missing-id"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stack \"missing-id\" not found")
}

func TestValidateCompatibility_PersistedMode_ClusterIDFallbackTriggersArchCheck(t *testing.T) {
	// Verified matrix (GitLab* = amd64-only) + mixed arch cluster resolved
	// via stack.ClusterID → should hit the TOOL_ARCH_UNSUPPORTED fail path.
	stackRepo, stackID := seedPersistedStack(t, "cluster-mixed", []domain.ToolConfig{
		{Category: "source_repository", Name: "GitLab CE"},
		{Category: "ci_platform", Name: "GitLab CI"},
		{Category: "container_registry", Name: "GitLab Registry"},
	})
	reader := &stubClusterReader{summary: &port.ClusterSummary{
		ID:                "cluster-mixed",
		NodeArchitectures: []string{"amd64", "arm64"},
	}}
	uc := NewValidateCompatibility(
		repository.NewMemoryCompatibilityRepository(),
		WithClusterReader(reader),
		WithStackRepository(stackRepo),
	)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{StackID: stackID})
	require.NoError(t, err)
	assert.Equal(t, "fail", out.Overall.State)
	assert.True(t, hasCode(out.Issues, "TOOL_ARCH_UNSUPPORTED"))
}

func TestValidateCompatibility_PersistedMode_ExplicitToolsOverrideStack(t *testing.T) {
	// Even if the stack row has mismatched tools, explicit Tools in the
	// input should win and the use case should NOT hit persisted mode.
	stackRepo, stackID := seedPersistedStack(t, "cluster-1", []domain.ToolConfig{
		{Category: "source_repository", Name: "Jenkins-doesnt-exist"},
	})
	uc := NewValidateCompatibility(
		repository.NewMemoryCompatibilityRepository(),
		WithStackRepository(stackRepo),
	)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		StackID: stackID,
		Tools: map[string]string{
			"source_repository": "GitLab CE",
			"ci_platform":       "GitLab CI",
		},
	})
	require.NoError(t, err)
	// Matched the verified GitLab matrix from the *explicit* input.
	require.NotNil(t, out.Matrix)
	assert.Equal(t, "pass", out.Overall.State)
}

func TestValidateCompatibility_PersistedMode_NoToolsConfigured_SkipsAsPass(t *testing.T) {
	stackRepo := repository.NewMemoryStackRepository()
	stack := &domain.Stack{
		ID:        "stack-no-tools",
		Name:      "stack-no-tools",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "nullus",
		Tools:     nil,
		Config:    domain.StackConfig{},
		State:     domain.StatePending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, stackRepo.Create(context.Background(), stack))

	uc := NewValidateCompatibility(
		repository.NewMemoryCompatibilityRepository(),
		WithStackRepository(stackRepo),
	)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{StackID: stack.ID})
	require.NoError(t, err)
	assert.True(t, out.Compatible)
	assert.Equal(t, "pass", out.Overall.State)
	assert.Equal(t, 100, out.Overall.Score)
	assert.Nil(t, out.Matrix)
	assert.Contains(t, out.Message, "skipped")
}
