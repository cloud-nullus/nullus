package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

// ExportConfig exports a stack's configuration as JSON or YAML.
type ExportConfig struct {
	stackRepo port.StackRepository
}

// NewExportConfig constructs an ExportConfig use case.
func NewExportConfig(stackRepo port.StackRepository) *ExportConfig {
	return &ExportConfig{stackRepo: stackRepo}
}

// ExportAsJSON returns the stack configuration serialized as indented JSON.
func (uc *ExportConfig) ExportAsJSON(ctx context.Context, stackID string) ([]byte, error) {
	stack, err := uc.stackRepo.GetByID(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("export json: %w", err)
	}

	data, err := json.MarshalIndent(stack, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	return data, nil
}

// ExportAsYAML returns the stack configuration serialized as YAML.
func (uc *ExportConfig) ExportAsYAML(ctx context.Context, stackID string) ([]byte, error) {
	stack, err := uc.stackRepo.GetByID(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("export yaml: %w", err)
	}

	data, err := yaml.Marshal(stack)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}

	return data, nil
}
