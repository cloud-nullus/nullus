package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// SaveVersionInput holds parameters for saving a new stack version snapshot.
type SaveVersionInput struct {
	StackID      string
	Config       domain.StackConfig
	ChangedBy    string
	ChangeReason string
}

// SaveVersionOutput holds the result of saving a stack version.
type SaveVersionOutput struct {
	Version *domain.StackVersion
}

// ListVersionsInput holds parameters for listing stack versions.
type ListVersionsInput struct {
	StackID string
}

// ListVersionsOutput holds the result of listing stack versions.
type ListVersionsOutput struct {
	Versions []*domain.StackVersion
}

// GetDiffInput holds parameters for computing a version diff.
type GetDiffInput struct {
	StackID   string
	VersionID string
}

// GetDiffOutput holds the computed diff result.
type GetDiffOutput struct {
	Diffs []domain.ConfigDiff
}

// ManageHistory provides use-case operations for stack version history.
type ManageHistory struct {
	repo port.HistoryRepository
}

// NewManageHistory constructs a ManageHistory use case.
func NewManageHistory(repo port.HistoryRepository) *ManageHistory {
	return &ManageHistory{repo: repo}
}

// SaveVersion creates and persists a new version snapshot for a stack.
// The version number is derived from the current count of existing versions + 1.
func (uc *ManageHistory) SaveVersion(ctx context.Context, input SaveVersionInput) (*SaveVersionOutput, error) {
	if input.StackID == "" {
		return nil, fmt.Errorf("stack_id is required")
	}
	if input.ChangedBy == "" {
		return nil, fmt.Errorf("changed_by is required")
	}

	existing, err := uc.repo.ListVersions(ctx, input.StackID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}

	version := &domain.StackVersion{
		ID:           generateID("ver"),
		StackID:      input.StackID,
		Version:      len(existing) + 1,
		Config:       input.Config,
		ChangedBy:    input.ChangedBy,
		ChangeReason: input.ChangeReason,
		CreatedAt:    time.Now(),
	}

	if err := uc.repo.SaveVersion(ctx, version); err != nil {
		return nil, fmt.Errorf("save version: %w", err)
	}

	return &SaveVersionOutput{Version: version}, nil
}

// ListVersions returns all versions for a stack ordered by version number.
func (uc *ManageHistory) ListVersions(ctx context.Context, input ListVersionsInput) (*ListVersionsOutput, error) {
	if input.StackID == "" {
		return nil, fmt.Errorf("stack_id is required")
	}

	versions, err := uc.repo.ListVersions(ctx, input.StackID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}

	return &ListVersionsOutput{Versions: versions}, nil
}

// GetDiff returns the field-level diff between a version and its predecessor.
func (uc *ManageHistory) GetDiff(ctx context.Context, input GetDiffInput) (*GetDiffOutput, error) {
	if input.StackID == "" {
		return nil, fmt.Errorf("stack_id is required")
	}
	if input.VersionID == "" {
		return nil, fmt.Errorf("version_id is required")
	}

	diffs, err := uc.repo.GetDiff(ctx, input.StackID, input.VersionID)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}

	return &GetDiffOutput{Diffs: diffs}, nil
}
