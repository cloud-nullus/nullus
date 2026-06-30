package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

var ErrImportConfirmationRequired = errors.New("import confirmation required")

// ImportConfigInput holds the raw export payload and target org context.
type ImportConfigInput struct {
	OrgID   string
	Payload []byte
	ReplaceExisting bool
}

// ImportConfigOutput returns the restored stack.
type ImportConfigOutput struct {
	Stack *domain.Stack
}

type ImportPreviewOutput struct {
	Mode            string      `json:"mode"`
	Name            string      `json:"name"`
	ClusterID       string      `json:"cluster_id"`
	ExistingStackID string      `json:"existing_stack_id,omitempty"`
	ExistingState   string      `json:"existing_state,omitempty"`
	Changes         *DiffResult `json:"changes,omitempty"`
}

// ImportConfig restores a stack from an export payload.
type ImportConfig struct {
	createStack *CreateStack
	addTools    *AddToolsUseCase
	installStack *InstallStack
}

type legacyExportedStack struct {
	Name       string                 `json:"name" yaml:"name"`
	TemplateID string                 `json:"template_id" yaml:"template_id"`
	OrgID      string                 `json:"org_id" yaml:"org_id"`
	ClusterID  string                 `json:"cluster_id" yaml:"cluster_id"`
	Namespace  string                 `json:"namespace" yaml:"namespace"`
	State      domain.DeploymentState `json:"state" yaml:"state"`
	Tools      []domain.ToolConfig    `json:"tools" yaml:"tools"`
	Config     domain.StackConfig     `json:"config" yaml:"config"`
	Resources  domain.ResourcesConfig `json:"resources" yaml:"resources"`
}

func NewImportConfig(createStack *CreateStack, addTools *AddToolsUseCase, installStack ...*InstallStack) *ImportConfig {
	uc := &ImportConfig{createStack: createStack, addTools: addTools}
	if len(installStack) > 0 {
		uc.installStack = installStack[0]
	}
	return uc
}

func (uc *ImportConfig) Preview(ctx context.Context, input ImportConfigInput) (*ImportPreviewOutput, error) {
	spec, err := uc.parseImportSpec(input)
	if err != nil {
		return nil, err
	}

	existing, err := uc.findExistingStack(ctx, input.OrgID, spec.Name, spec.ClusterID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return &ImportPreviewOutput{
			Mode:      "create",
			Name:      spec.Name,
			ClusterID: spec.ClusterID,
		}, nil
	}

	changes, err := uc.buildDiff(existing, spec)
	if err != nil {
		return nil, err
	}

	return &ImportPreviewOutput{
		Mode:            "update",
		Name:            spec.Name,
		ClusterID:       spec.ClusterID,
		ExistingStackID: existing.ID,
		ExistingState:   string(existing.State),
		Changes:         changes,
	}, nil
}

func (uc *ImportConfig) Execute(ctx context.Context, input ImportConfigInput) (*ImportConfigOutput, error) {
	if uc == nil || uc.createStack == nil {
		return nil, fmt.Errorf("import usecase is not configured")
	}
	spec, err := uc.parseImportSpec(input)
	if err != nil {
		return nil, err
	}

	existing, err := uc.findExistingStack(ctx, input.OrgID, spec.Name, spec.ClusterID)
	if err != nil {
		return nil, err
	}
	if existing != nil && !input.ReplaceExisting {
		return nil, ErrImportConfirmationRequired
	}

	if strings.TrimSpace(input.OrgID) == "" {
		input.OrgID = strings.TrimSpace(spec.OrgID)
	}
	if strings.TrimSpace(input.OrgID) == "" {
		return nil, fmt.Errorf("org_id is required")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(spec.ClusterID) == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}

	cfg := spec.Config
	cfg.Resources = spec.Resources
	cfg.OptionOverrides = spec.Config.OptionOverrides
	cfg.AppliedResourceOverrides = spec.Config.AppliedResourceOverrides
	cfg.RowUnits = spec.Config.RowUnits

	if existing != nil {
		resumeFromStep, hasChanges, err := uc.resumeStepForImport(ctx, existing, spec)
		if err != nil {
			return nil, err
		}
		if uc.createStack.manageHistory != nil {
			if currentCfg, ok := stackConfigFromInterface(existing.Config); ok {
				_, _ = uc.createStack.manageHistory.SaveVersion(ctx, SaveVersionInput{
					StackID:      existing.ID,
					Config:       currentCfg,
					ChangedBy:    "system",
					ChangeReason: "import applied",
				})
			}
		}

		existing.TemplateID = spec.TemplateID
		existing.Namespace = spec.Namespace
		existing.Config = cfg
		existing.UpdatedAt = time.Now()
		if err := uc.createStack.stackRepo.Update(ctx, existing); err != nil {
			return nil, fmt.Errorf("update imported stack: %w", err)
		}
		existing.Tools = spec.Tools
		if err := uc.createStack.stackRepo.UpdateTools(ctx, existing); err != nil {
			return nil, fmt.Errorf("replace imported stack tools: %w", err)
		}
		if uc.createStack.manageHistory != nil {
			if _, err := uc.createStack.manageHistory.SaveVersion(ctx, SaveVersionInput{
				StackID:      existing.ID,
				Config:       cfg,
				ChangedBy:    "system",
				ChangeReason: "import applied",
			}); err != nil {
				return nil, fmt.Errorf("save imported history: %w", err)
			}
		}
		if hasChanges {
			if err := uc.triggerImportDeploy(ctx, existing, true, resumeFromStep); err != nil {
				return nil, err
			}
		}
		return &ImportConfigOutput{Stack: existing}, nil
	}

	created, err := uc.createStack.Execute(ctx, CreateStackInput{
		Name:       spec.Name,
		OrgID:      input.OrgID,
		ClusterID:  spec.ClusterID,
		Namespace:  spec.Namespace,
		TemplateID: spec.TemplateID,
		Config:     cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("create restored stack: %w", err)
	}

	if len(spec.Tools) > 0 && uc.addTools != nil {
		if _, err := uc.addTools.Execute(ctx, AddToolsInput{
			StackID: created.Stack.ID,
			Tools:   spec.Tools,
		}); err != nil {
			return nil, fmt.Errorf("restore tools: %w", err)
		}
	}
	if err := uc.triggerImportDeploy(ctx, created.Stack, false, ""); err != nil {
		return nil, err
	}

	return &ImportConfigOutput{Stack: created.Stack}, nil
}

func (uc *ImportConfig) triggerImportDeploy(ctx context.Context, stack *domain.Stack, partial bool, resumeFromStep string) error {
	if uc.installStack == nil || stack == nil {
		slog.Info("import deploy skipped", "stack_id", func() string { if stack != nil { return stack.ID }; return "" }(), "reason", "install usecase not configured")
		return nil
	}
	if partial {
		slog.Info("import deploy starting partial reapply", "stack_id", stack.ID, "resume_from_step", resumeFromStep)
		stack.State = domain.StatePending
		stack.CurrentStep = resumeFromStep
		stack.LastFailedStep = resumeFromStep
		stack.LastFailureReason = ""
		stack.UpdatedAt = time.Now()
		if err := uc.createStack.stackRepo.Update(ctx, stack); err != nil {
			return fmt.Errorf("prepare partial import deploy: %w", err)
		}
		if err := uc.installStack.Execute(ctx, InstallStackInput{StackID: stack.ID, Continue: true, ResumeFromStep: resumeFromStep}); err != nil {
			return err
		}
		slog.Info("import deploy accepted partial reapply", "stack_id", stack.ID, "resume_from_step", resumeFromStep)
		return nil
	}
	slog.Info("import deploy starting full deploy", "stack_id", stack.ID)
	if err := uc.installStack.Execute(ctx, InstallStackInput{StackID: stack.ID}); err != nil {
		return err
	}
	slog.Info("import deploy accepted full deploy", "stack_id", stack.ID)
	return nil
}

func (uc *ImportConfig) resumeStepForImport(ctx context.Context, existing *domain.Stack, spec ExportSpec) (string, bool, error) {
	changes, err := uc.buildDiff(existing, spec)
	if err != nil {
		return "", false, err
	}
	if len(changes.Added) == 0 && len(changes.Removed) == 0 && len(changes.Changed) == 0 {
		return "", false, nil
	}
	return importResumeStep(changes), true, nil
}

func importResumeStep(diff *DiffResult) string {
	if diff == nil {
		return "installing_cert_manager"
	}
	keys := make([]string, 0, len(diff.Added)+len(diff.Removed)+len(diff.Changed))
	for k := range diff.Added { keys = append(keys, k) }
	for k := range diff.Removed { keys = append(keys, k) }
	for k := range diff.Changed { keys = append(keys, k) }

	stepOrder := []string{
		"installing_cert_manager",
		"installing_postgresql",
		"installing_minio",
		"installing_gitlab",
		"installing_argocd",
		"installing_runner",
		"installing_prometheus",
		"installing_grafana",
		"installing_logging",
		"installing_log_search",
		"installing_opentelemetry",
		"installing_gateway",
	}
	matched := map[string]bool{}
	for _, key := range keys {
		switch {
		case strings.HasPrefix(key, "cluster_id"), strings.HasPrefix(key, "namespace"):
			matched["installing_cert_manager"] = true
		case strings.HasPrefix(key, "config.storage."), strings.HasPrefix(key, "config.applied_resource_overrides.artifacts.storageBackend:"), strings.HasPrefix(key, "config.resources"):
			matched["installing_postgresql"] = true
		case strings.HasPrefix(key, "config.artifacts.package_registry"), strings.HasPrefix(key, "config.artifacts.source_repository"), strings.HasPrefix(key, "config.artifacts.container_registry"), strings.HasPrefix(key, "config.pipeline.ci_platform"), strings.HasPrefix(key, "config.applied_resource_overrides.artifacts.packageRegistry:"), strings.HasPrefix(key, "config.option_overrides.artifacts.packageRegistry"):
			matched["installing_gitlab"] = true
		case strings.HasPrefix(key, "config.pipeline.cd_tool"), strings.HasPrefix(key, "config.applied_resource_overrides.pipeline.cdTool:"), strings.HasPrefix(key, "config.option_overrides.pipeline.cdTool"):
			matched["installing_argocd"] = true
		case strings.HasPrefix(key, "config.monitoring.collection"), strings.HasPrefix(key, "config.applied_resource_overrides.monitoring.collection:"), strings.HasPrefix(key, "config.option_overrides.monitoring.collection"):
			matched["installing_prometheus"] = true
		case strings.HasPrefix(key, "config.monitoring.visualization"):
			matched["installing_grafana"] = true
		case strings.HasPrefix(key, "config.logging.search"), strings.HasPrefix(key, "config.logging.collection"):
			matched["installing_logging"] = true
		case strings.HasPrefix(key, "config.logging.trace_layer"), strings.HasPrefix(key, "config.logging.trace_exporter"):
			matched["installing_opentelemetry"] = true
		case strings.HasPrefix(key, "config.access_domain"), strings.HasPrefix(key, "config.access_domain_tls"):
			matched["installing_gateway"] = true
		case strings.HasPrefix(key, "template_id"):
			matched["installing_gitlab"] = true
		}
	}
	for _, step := range stepOrder {
		if matched[step] {
			return step
		}
	}
	return "installing_cert_manager"
}

func (uc *ImportConfig) parseImportSpec(input ImportConfigInput) (ExportSpec, error) {
	if len(input.Payload) == 0 {
		return ExportSpec{}, fmt.Errorf("import payload is required")
	}

	var exported ExportedStack
	if json.Valid(input.Payload) {
		if err := json.Unmarshal(input.Payload, &exported); err != nil {
			return ExportSpec{}, fmt.Errorf("parse import payload: %w", err)
		}
	} else if err := yaml.Unmarshal(input.Payload, &exported); err != nil {
		return ExportSpec{}, fmt.Errorf("parse import payload: %w", err)
	}

	spec := exported.Spec
	if spec.SchemaVersion == "" && spec.Name == "" && spec.TemplateID == "" && spec.OrgID == "" && spec.ClusterID == "" && spec.Namespace == "" {
		var legacy legacyExportedStack
		if json.Valid(input.Payload) {
			if err := json.Unmarshal(input.Payload, &legacy); err != nil {
				return ExportSpec{}, fmt.Errorf("parse legacy import payload: %w", err)
			}
		} else if err := yaml.Unmarshal(input.Payload, &legacy); err != nil {
			return ExportSpec{}, fmt.Errorf("parse legacy import payload: %w", err)
		}
		spec = ExportSpec{
			Name:       legacy.Name,
			TemplateID: legacy.TemplateID,
			OrgID:      legacy.OrgID,
			ClusterID:  legacy.ClusterID,
			Namespace:  legacy.Namespace,
			State:      legacy.State,
			Tools:      legacy.Tools,
			Config:     legacy.Config,
			Resources:  legacy.Resources,
		}
	}

	return spec, nil
}

func (uc *ImportConfig) findExistingStack(ctx context.Context, orgID, name, clusterID string) (*domain.Stack, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, nil
	}
	items, err := uc.createStack.stackRepo.List(ctx, orgID, false)
	if err != nil {
		return nil, fmt.Errorf("list stacks: %w", err)
	}
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.ClusterID != clusterID {
			continue
		}
		if strings.ToLower(strings.TrimSpace(item.Name)) == normalizedName {
			return item, nil
		}
	}
	return nil, nil
}

func (uc *ImportConfig) buildDiff(existing *domain.Stack, spec ExportSpec) (*DiffResult, error) {
	existingCfg, _ := stackConfigFromInterface(existing.Config)
	existingMap := map[string]any{
		"template_id": existing.TemplateID,
		"namespace":   existing.Namespace,
		"config":      existingCfg,
		"tools":       toolsDiffMap(existing.Tools),
	}
	importMap := map[string]any{
		"template_id": spec.TemplateID,
		"namespace":   spec.Namespace,
		"config":      spec.Config,
		"tools":       toolsDiffMap(spec.Tools),
	}
	left, err := configToMap(existingMap)
	if err != nil {
		return nil, err
	}
	right, err := configToMap(importMap)
	if err != nil {
		return nil, err
	}
	flatA := map[string]any{}
	flatB := map[string]any{}
	flattenMeaningful("", left, flatA)
	flattenMeaningful("", right, flatB)
	result := &DiffResult{Added: map[string]any{}, Removed: map[string]any{}, Changed: map[string][2]any{}}
	keys := map[string]struct{}{}
	for k := range flatA { keys[k] = struct{}{} }
	for k := range flatB { keys[k] = struct{}{} }
	for k := range keys {
		av, aok := flatA[k]
		bv, bok := flatB[k]
		switch {
		case !aok && bok:
			result.Added[k] = bv
		case aok && !bok:
			result.Removed[k] = av
		case !reflect.DeepEqual(av, bv):
			result.Changed[k] = [2]any{av, bv}
		}
	}
	return result, nil
}

func toolsDiffMap(tools []domain.ToolConfig) map[string]map[string]string {
	out := make(map[string]map[string]string, len(tools))
	for _, tool := range tools {
		key := tool.Category
		if key == "" {
			key = tool.Tool
		}
		out[key] = map[string]string{
			"tool":    tool.Tool,
			"version": tool.Version,
		}
	}
	return out
}
