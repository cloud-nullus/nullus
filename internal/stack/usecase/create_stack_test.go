package usecase

import (
	"context"
	"testing"

	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateStack_DefaultNamespaceWhenEmpty(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	templateRepo := stackrepo.NewMemoryTemplateRepository()
	uc := NewCreateStack(stackRepo, templateRepo)

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "stack-default-ns",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Stack)
	assert.Equal(t, "nullus", out.Stack.Namespace)
}

func TestCreateStack_UsesProvidedNamespace(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	templateRepo := stackrepo.NewMemoryTemplateRepository()
	uc := NewCreateStack(stackRepo, templateRepo)

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "stack-custom-ns",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "production",
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Stack)
	assert.Equal(t, "production", out.Stack.Namespace)
}

func TestCreateStack_DefaultAccessDomainWhenEmpty(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	templateRepo := stackrepo.NewMemoryTemplateRepository()
	uc := NewCreateStack(stackRepo, templateRepo)

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "stack-domain-default",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Config:    domain.StackConfig{},
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Stack)
	config, ok := out.Stack.Config.(domain.StackConfig)
	require.True(t, ok)
	assert.Equal(t, "stack-domain-default.internal", config.AccessDomain)
}

func TestCreateStack_PreservesProvidedAccessDomain(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	templateRepo := stackrepo.NewMemoryTemplateRepository()
	uc := NewCreateStack(stackRepo, templateRepo)

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "stack-domain-custom",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Config: domain.StackConfig{
			AccessDomain: "custom.example.internal",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Stack)
	config, ok := out.Stack.Config.(domain.StackConfig)
	require.True(t, ok)
	assert.Equal(t, "custom.example.internal", config.AccessDomain)
}

func TestCreateStack_RejectsInvalidStoragePlanMode(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	templateRepo := stackrepo.NewMemoryTemplateRepository()
	uc := NewCreateStack(stackRepo, templateRepo)

	_, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "stack-invalid-storage",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Config: domain.StackConfig{
			Storage: &domain.StorageConfig{PlanMode: "invalid"},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "storage.plan_mode")
}

func TestCreateStack_RejectsExistingConnectWithoutCredential(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	templateRepo := stackrepo.NewMemoryTemplateRepository()
	uc := NewCreateStack(stackRepo, templateRepo)

	_, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "stack-existing-connect-invalid",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Config: domain.StackConfig{
			Storage: &domain.StorageConfig{
				PlanMode: "existing-connect",
				Database: domain.StorageTarget{
					Mode:     "existing-connect",
					Endpoint: "postgres.internal:5432",
				},
				ObjectStorage: domain.StorageTarget{
					Mode:     "existing-connect",
					Endpoint: "minio.internal:9000",
				},
			},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires access_secret_ref or auth_id/auth_password_key")
}

func TestCreateStack_AllowsValidStorageConfig(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	templateRepo := stackrepo.NewMemoryTemplateRepository()
	uc := NewCreateStack(stackRepo, templateRepo)

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "stack-storage-valid",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Config: domain.StackConfig{
			Storage: &domain.StorageConfig{
				PlanMode: "integrated-create",
				Database: domain.StorageTarget{
					Mode:             "create",
					ProviderOrEngine: "postgresql",
					Size:             100,
				},
				ObjectStorage: domain.StorageTarget{
					Mode:             "create",
					ProviderOrEngine: "minio",
					Size:             200,
				},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Stack)
}

func TestCreateStack_RejectsTlsEnabledWithoutIssuer(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	templateRepo := stackrepo.NewMemoryTemplateRepository()
	uc := NewCreateStack(stackRepo, templateRepo)

	_, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "stack-tls-invalid",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Config: domain.StackConfig{
			AccessDomainTLS: &domain.AccessDomainTLSConfig{
				Enabled:         true,
				SecretName:      "nullus-wildcard-tls",
				SecretNamespace: "nullus",
			},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_domain_tls.issuer_name")
}
