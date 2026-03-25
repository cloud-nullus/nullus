package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// CreateStackInput holds the parameters for creating a new stack.
type CreateStackInput struct {
	Name       string
	OrgID      string
	ClusterID  string
	Namespace  string
	TemplateID string
	Config     domain.StackConfig
}

// CreateStackOutput holds the result of creating a stack.
type CreateStackOutput struct {
	Stack *domain.Stack
}

// CreateStack creates a new stack configuration, optionally loading defaults from a template.
type CreateStack struct {
	stackRepo    port.StackRepository
	templateRepo port.TemplateRepository
}

// NewCreateStack constructs a CreateStack use case.
func NewCreateStack(stackRepo port.StackRepository, templateRepo port.TemplateRepository) *CreateStack {
	return &CreateStack{
		stackRepo:    stackRepo,
		templateRepo: templateRepo,
	}
}

// Execute creates a new stack, merging template defaults when a TemplateID is provided.
func (uc *CreateStack) Execute(ctx context.Context, input CreateStackInput) (*CreateStackOutput, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("stack name is required")
	}
	if input.ClusterID == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}
	if input.OrgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}
	if err := validateAccessDomainTLS(input.Config.AccessDomainTLS); err != nil {
		return nil, err
	}
	if err := validateStorageConfig(input.Config.Storage); err != nil {
		return nil, err
	}

	now := time.Now()
	namespace := input.Namespace
	if namespace == "" {
		namespace = "nullus"
	}
	stack := &domain.Stack{
		ID:         generateID("stk"),
		Name:       input.Name,
		TemplateID: input.TemplateID,
		OrgID:      input.OrgID,
		ClusterID:  input.ClusterID,
		Namespace:  namespace,
		State:      domain.StatePending,
		Config:     input.Config,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := uc.stackRepo.Create(ctx, stack); err != nil {
		return nil, fmt.Errorf("create stack: %w", err)
	}

	return &CreateStackOutput{Stack: stack}, nil
}

func validateStorageConfig(storage *domain.StorageConfig) error {
	if storage == nil {
		return nil
	}

	planMode := strings.TrimSpace(storage.PlanMode)
	if planMode != "integrated-create" && planMode != "existing-connect" {
		return fmt.Errorf("storage.plan_mode must be integrated-create or existing-connect")
	}

	if err := validateStorageTarget("storage.database", storage.Database); err != nil {
		return err
	}
	if err := validateStorageTarget("storage.object_storage", storage.ObjectStorage); err != nil {
		return err
	}

	if planMode == "integrated-create" {
		if storage.Database.Mode != "create" || storage.ObjectStorage.Mode != "create" {
			return fmt.Errorf("integrated-create 모드에서는 database/object_storage 모두 create 이어야 합니다")
		}
	}

	return nil
}

func validateStorageTarget(path string, target domain.StorageTarget) error {
	mode := strings.TrimSpace(target.Mode)
	switch mode {
	case "create":
		if strings.TrimSpace(target.ProviderOrEngine) == "" {
			return fmt.Errorf("%s.provider_or_engine is required in create mode", path)
		}
		if target.Size <= 0 {
			return fmt.Errorf("%s.size must be greater than 0 in create mode", path)
		}
	case "existing-connect":
		if strings.TrimSpace(target.Endpoint) == "" {
			return fmt.Errorf("%s.endpoint is required in existing-connect mode", path)
		}
		hasSecretRef := strings.TrimSpace(target.AccessSecretRef) != ""
		hasPair := strings.TrimSpace(target.AuthID) != "" && strings.TrimSpace(target.AuthPasswordKey) != ""
		if !hasSecretRef && !hasPair {
			return fmt.Errorf("%s requires access_secret_ref or auth_id/auth_password_key in existing-connect mode", path)
		}
	default:
		return fmt.Errorf("%s.mode must be create or existing-connect", path)
	}

	return nil
}

func validateAccessDomainTLS(tls *domain.AccessDomainTLSConfig) error {
	if tls == nil || !tls.Enabled {
		return nil
	}

	if strings.TrimSpace(tls.SecretName) == "" {
		return fmt.Errorf("access_domain_tls.secret_name is required when enabled")
	}
	if strings.TrimSpace(tls.SecretNamespace) == "" {
		return fmt.Errorf("access_domain_tls.secret_namespace is required when enabled")
	}
	if strings.TrimSpace(tls.IssuerName) == "" {
		return fmt.Errorf("access_domain_tls.issuer_name is required when enabled")
	}

	return nil
}
