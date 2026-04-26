package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func TestManageHistory_SaveVersion(t *testing.T) {
	repo := repository.NewMemoryHistoryRepository()
	uc := NewManageHistory(repo)

	out, err := uc.SaveVersion(context.Background(), SaveVersionInput{
		StackID:      "stk_abc",
		Config:       domain.StackConfig{},
		ChangedBy:    "user1",
		ChangeReason: "initial setup",
	})

	require.NoError(t, err)
	assert.Equal(t, "stk_abc", out.Version.StackID)
	assert.Equal(t, 1, out.Version.Version)
	assert.Equal(t, "user1", out.Version.ChangedBy)
	assert.NotEmpty(t, out.Version.ID)
}

func TestManageHistory_VersionNumberIncrement(t *testing.T) {
	repo := repository.NewMemoryHistoryRepository()
	uc := NewManageHistory(repo)

	for i := 0; i < 3; i++ {
		_, err := uc.SaveVersion(context.Background(), SaveVersionInput{
			StackID:   "stk_abc",
			Config:    domain.StackConfig{},
			ChangedBy: "user1",
		})
		require.NoError(t, err)
	}

	listOut, err := uc.ListVersions(context.Background(), ListVersionsInput{StackID: "stk_abc"})
	require.NoError(t, err)
	require.Len(t, listOut.Versions, 3)
	assert.Equal(t, 1, listOut.Versions[0].Version)
	assert.Equal(t, 2, listOut.Versions[1].Version)
	assert.Equal(t, 3, listOut.Versions[2].Version)
}

func TestManageHistory_GetDiff(t *testing.T) {
	repo := repository.NewMemoryHistoryRepository()
	uc := NewManageHistory(repo)

	config1 := domain.StackConfig{
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Name: "GitLab CI", Version: "17.7.2", Enabled: true},
		},
	}
	config2 := domain.StackConfig{
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Name: "GitHub Actions", Version: "external", Enabled: true},
		},
	}

	v1, err := uc.SaveVersion(context.Background(), SaveVersionInput{
		StackID:   "stk_abc",
		Config:    config1,
		ChangedBy: "user1",
	})
	require.NoError(t, err)

	v2, err := uc.SaveVersion(context.Background(), SaveVersionInput{
		StackID:   "stk_abc",
		Config:    config2,
		ChangedBy: "user1",
	})
	require.NoError(t, err)

	// v1 vs nothing: all fields are new
	diffOut1, err := uc.GetDiff(context.Background(), GetDiffInput{
		StackID:   "stk_abc",
		VersionID: v1.Version.ID,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, diffOut1.Diffs)

	// v2 vs v1: only changed fields
	diffOut2, err := uc.GetDiff(context.Background(), GetDiffInput{
		StackID:   "stk_abc",
		VersionID: v2.Version.ID,
	})
	require.NoError(t, err)

	// ci_platform name and version should differ
	changedFields := make(map[string]domain.ConfigDiff)
	for _, d := range diffOut2.Diffs {
		changedFields[d.Field] = d
	}
	ciNameDiff, ok := changedFields["pipeline.ci_platform.name"]
	require.True(t, ok, "expected ci_platform name diff")
	assert.Equal(t, "GitLab CI", ciNameDiff.OldValue)
	assert.Equal(t, "GitHub Actions", ciNameDiff.NewValue)
}

func TestManageHistory_SaveVersion_MissingStackID(t *testing.T) {
	repo := repository.NewMemoryHistoryRepository()
	uc := NewManageHistory(repo)

	_, err := uc.SaveVersion(context.Background(), SaveVersionInput{
		StackID:   "",
		ChangedBy: "user1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stack_id")
}

func TestManageHistory_SaveVersion_MissingChangedBy(t *testing.T) {
	repo := repository.NewMemoryHistoryRepository()
	uc := NewManageHistory(repo)

	_, err := uc.SaveVersion(context.Background(), SaveVersionInput{
		StackID:   "stk_abc",
		ChangedBy: "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed_by")
}
