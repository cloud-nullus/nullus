package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

type fakeResourceDefaultRepo struct {
	items map[string]*domain.ResourceDefault
}

func (f *fakeResourceDefaultRepo) List(_ context.Context) ([]*domain.ResourceDefault, error) {
	out := make([]*domain.ResourceDefault, 0, len(f.items))
	for _, v := range f.items {
		copy := *v
		out = append(out, &copy)
	}
	return out, nil
}

func (f *fakeResourceDefaultRepo) Upsert(_ context.Context, resource *domain.ResourceDefault) error {
	if f.items == nil {
		f.items = map[string]*domain.ResourceDefault{}
	}
	copy := *resource
	f.items[resource.ToolKey] = &copy
	return nil
}

func TestUpsertResourceDefault_Execute_Success(t *testing.T) {
	repo := &fakeResourceDefaultRepo{items: map[string]*domain.ResourceDefault{}}
	uc := NewUpsertResourceDefault(repo)

	out, err := uc.Execute(context.Background(), UpsertResourceDefaultInput{
		ToolKey:          "GitLab-CE",
		DisplayName:      "GitLab CE",
		CPURequest:       4,
		CPULimit:         8,
		MemoryRequestGi:  8,
		MemoryLimitGi:    16,
		StorageRequestGi: 30,
		StorageLimitGi:   60,
		IsDefault:        true,
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "gitlab-ce", out.Item.ToolKey)
	assert.Equal(t, "GitLab CE", out.Item.DisplayName)
	require.Contains(t, repo.items, "gitlab-ce")
}

func TestUpsertResourceDefault_Execute_Validation(t *testing.T) {
	repo := &fakeResourceDefaultRepo{items: map[string]*domain.ResourceDefault{}}
	uc := NewUpsertResourceDefault(repo)

	_, err := uc.Execute(context.Background(), UpsertResourceDefaultInput{
		ToolKey:          "",
		DisplayName:      "GitLab CE",
		CPURequest:       4,
		CPULimit:         8,
		MemoryRequestGi:  8,
		MemoryLimitGi:    16,
		StorageRequestGi: 30,
		StorageLimitGi:   60,
	})
	require.Error(t, err)

	_, err = uc.Execute(context.Background(), UpsertResourceDefaultInput{
		ToolKey:          "gitlab-ce",
		DisplayName:      "",
		CPURequest:       4,
		CPULimit:         8,
		MemoryRequestGi:  8,
		MemoryLimitGi:    16,
		StorageRequestGi: 30,
		StorageLimitGi:   60,
	})
	require.Error(t, err)

	_, err = uc.Execute(context.Background(), UpsertResourceDefaultInput{
		ToolKey:          "gitlab-ce",
		DisplayName:      "GitLab CE",
		CPURequest:       0,
		CPULimit:         8,
		MemoryRequestGi:  8,
		MemoryLimitGi:    16,
		StorageRequestGi: 30,
		StorageLimitGi:   60,
	})
	require.Error(t, err)

	_, err = uc.Execute(context.Background(), UpsertResourceDefaultInput{
		ToolKey:          "gitlab-ce",
		DisplayName:      "GitLab CE",
		CPURequest:       4,
		CPULimit:         2,
		MemoryRequestGi:  8,
		MemoryLimitGi:    16,
		StorageRequestGi: 30,
		StorageLimitGi:   60,
	})
	require.Error(t, err)
}
