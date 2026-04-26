package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type ListResourceDefaults struct {
	repo port.ResourceDefaultRepository
}

func NewListResourceDefaults(repo port.ResourceDefaultRepository) *ListResourceDefaults {
	return &ListResourceDefaults{repo: repo}
}

type ListResourceDefaultsOutput struct {
	Items []*domain.ResourceDefault
}

func (uc *ListResourceDefaults) Execute(ctx context.Context) (*ListResourceDefaultsOutput, error) {
	items, err := uc.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	return &ListResourceDefaultsOutput{Items: items}, nil
}

type UpsertResourceDefault struct {
	repo port.ResourceDefaultRepository
}

func NewUpsertResourceDefault(repo port.ResourceDefaultRepository) *UpsertResourceDefault {
	return &UpsertResourceDefault{repo: repo}
}

type UpsertResourceDefaultInput struct {
	ToolKey          string
	DisplayName      string
	CPURequest       float64
	CPULimit         float64
	MemoryRequestGi  float64
	MemoryLimitGi    float64
	StorageRequestGi float64
	StorageLimitGi   float64
	IsDefault        bool
}

type UpsertResourceDefaultOutput struct {
	Item *domain.ResourceDefault
}

func (uc *UpsertResourceDefault) Execute(ctx context.Context, in UpsertResourceDefaultInput) (*UpsertResourceDefaultOutput, error) {
	toolKey := strings.TrimSpace(strings.ToLower(in.ToolKey))
	if toolKey == "" {
		return nil, fmt.Errorf("tool_key is required")
	}
	if strings.TrimSpace(in.DisplayName) == "" {
		return nil, fmt.Errorf("display_name is required")
	}
	if in.CPURequest <= 0 {
		return nil, fmt.Errorf("cpu_request must be greater than 0")
	}
	if in.CPULimit < in.CPURequest {
		return nil, fmt.Errorf("cpu_limit must be greater than or equal to cpu_request")
	}
	if in.MemoryRequestGi <= 0 {
		return nil, fmt.Errorf("memory_request_gi must be greater than 0")
	}
	if in.MemoryLimitGi < in.MemoryRequestGi {
		return nil, fmt.Errorf("memory_limit_gi must be greater than or equal to memory_request_gi")
	}
	if in.StorageRequestGi < 0 {
		return nil, fmt.Errorf("storage_request_gi must be 0 or greater")
	}
	if in.StorageLimitGi < in.StorageRequestGi {
		return nil, fmt.Errorf("storage_limit_gi must be greater than or equal to storage_request_gi")
	}

	item := &domain.ResourceDefault{
		ToolKey:          toolKey,
		DisplayName:      strings.TrimSpace(in.DisplayName),
		CPURequest:       in.CPURequest,
		CPULimit:         in.CPULimit,
		MemoryRequestGi:  in.MemoryRequestGi,
		MemoryLimitGi:    in.MemoryLimitGi,
		StorageRequestGi: in.StorageRequestGi,
		StorageLimitGi:   in.StorageLimitGi,
		IsDefault:        in.IsDefault,
	}

	if err := uc.repo.Upsert(ctx, item); err != nil {
		return nil, err
	}

	return &UpsertResourceDefaultOutput{Item: item}, nil
}
