package usecase

import (
	"context"
	"testing"

	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
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
