package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

const stackExportSchemaVersion = "v1"

// ExportSpec holds the canonical exported stack specification.
type ExportSpec struct {
	SchemaVersion string                 `json:"schema_version" yaml:"schema_version"`
	Name          string                 `json:"name" yaml:"name"`
	TemplateID    string                 `json:"template_id" yaml:"template_id"`
	OrgID         string                 `json:"org_id" yaml:"org_id"`
	ClusterID     string                 `json:"cluster_id" yaml:"cluster_id"`
	Namespace     string                 `json:"namespace" yaml:"namespace"`
	State         domain.DeploymentState `json:"state,omitempty" yaml:"state,omitempty"`
	Tools         []domain.ToolConfig    `json:"tools,omitempty" yaml:"tools,omitempty"`
	Config        domain.StackConfig     `json:"config" yaml:"config"`
	Resources     domain.ResourcesConfig `json:"resources" yaml:"resources"`
}

// ExportedStack is the portable representation used by export/import flows.
type ExportedStack struct {
	Kind       string     `json:"kind" yaml:"kind"`
	APIVersion string     `json:"apiVersion" yaml:"apiVersion"`
	Spec       ExportSpec `json:"spec" yaml:"spec"`
}

// ExportConfig exports a stack's configuration as JSON or YAML.
type ExportConfig struct {
	stackRepo port.StackRepository
}

// NewExportConfig constructs an ExportConfig use case.
func NewExportConfig(stackRepo port.StackRepository) *ExportConfig {
	return &ExportConfig{stackRepo: stackRepo}
}

// BuildExport returns the portable export payload for a stack.
func (uc *ExportConfig) BuildExport(ctx context.Context, stackID string) (*ExportedStack, error) {
	stack, err := uc.stackRepo.GetByID(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("export stack: %w", err)
	}
	if stack == nil {
		return nil, fmt.Errorf("export stack: stack %q not found", stackID)
	}

	cfg := extractStackConfigForExport(stack.Config)
	spec := ExportSpec{
		SchemaVersion: stackExportSchemaVersion,
		Name:          stack.Name,
		TemplateID:    stack.TemplateID,
		OrgID:         stack.OrgID,
		ClusterID:     stack.ClusterID,
		Namespace:     stack.Namespace,
		State:         stack.State,
		Tools:         append([]domain.ToolConfig(nil), stack.Tools...),
		Config:        cfg,
		Resources:     cfg.Resources,
	}
	payload := &ExportedStack{
		Kind:       "StackExport",
		APIVersion: "stack.nullus.dev/v1alpha1",
		Spec:       spec,
	}

	return payload, nil
}

func extractStackConfigForExport(raw any) domain.StackConfig {
	if raw == nil {
		return domain.StackConfig{}
	}

	switch cfg := raw.(type) {
	case domain.StackConfig:
		return cfg
	case *domain.StackConfig:
		if cfg != nil {
			return *cfg
		}
	}

	cfg, ok := stackConfigFromInterface(raw)
	if ok {
		return cfg
	}
	return domain.StackConfig{}
}

// ExportAsJSON returns the stack configuration serialized as indented JSON.
func (uc *ExportConfig) ExportAsJSON(ctx context.Context, stackID string) ([]byte, error) {
	payload, err := uc.BuildExport(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("export json: %w", err)
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	return data, nil
}

// ExportAsYAML returns the stack configuration serialized as YAML.
func (uc *ExportConfig) ExportAsYAML(ctx context.Context, stackID string) ([]byte, error) {
	payload, err := uc.BuildExport(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("export yaml: %w", err)
	}

	data, err := yaml.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}

	return data, nil
}
