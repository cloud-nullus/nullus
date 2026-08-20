package usecase

import (
	"context"
	"fmt"
	"log/slog"
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
	stackRepo        port.StackRepository
	templateRepo     port.TemplateRepository
	manageHistory    *ManageHistory
	resourceDefaults port.ResourceDefaultRepository
	// platformNamespace 는 플랫폼 자신이 사는 네임스페이스다. 알고 있으면 그곳에는
	// 스택을 세우지 못하게 막는다. 비어 있으면 모른다는 뜻이라 검사하지 않는다.
	platformNamespace string
}

// CreateStackOption configures optional CreateStack dependencies.
type CreateStackOption func(*CreateStack)

// WithManageHistory enables automatic initial version snapshots.
func WithManageHistory(manageHistory *ManageHistory) CreateStackOption {
	return func(uc *CreateStack) {
		uc.manageHistory = manageHistory
	}
}

// WithResourcePlanning 은 템플릿의 planning_profile 로 설치 규모를 계획하게 한다.
//
// 계획 계산은 원래 설치 마법사에만 있었다. 그래서 API 로 만든 스택은 프로파일을
// 저장만 하고 크기에는 반영하지 않아, Lite 템플릿도 standard 크기로 깔렸다.
func WithResourcePlanning(resourceDefaults port.ResourceDefaultRepository) CreateStackOption {
	return func(uc *CreateStack) {
		uc.resourceDefaults = resourceDefaults
	}
}

// WithPlatformNamespace 는 플랫폼이 사는 네임스페이스를 알려준다.
//
// 그곳에 스택을 세우면 설치는 Helm 소유권 충돌로 실패하고(플랫폼의
// nullus-postgresql 과 이름이 겹친다), 삭제는 플랫폼 리소스를 지운다 —
// 2026-08-20 에 실제로 nullus.io 가 통째로 내려갔다.
func WithPlatformNamespace(namespace string) CreateStackOption {
	return func(uc *CreateStack) {
		uc.platformNamespace = strings.TrimSpace(namespace)
	}
}

// NewCreateStack constructs a CreateStack use case.
func NewCreateStack(stackRepo port.StackRepository, templateRepo port.TemplateRepository, opts ...CreateStackOption) *CreateStack {
	uc := &CreateStack{
		stackRepo:    stackRepo,
		templateRepo: templateRepo,
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
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

	if accessDomain := strings.TrimSpace(input.Config.AccessDomain); accessDomain != "" {
		if err := domain.ValidateAccessDomain(accessDomain); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		// 스택마다 자기 네임스페이스를 준다. 예전에는 전부 "nullus" 로 모였고,
		// 그것이 플랫폼 자신이 사는 곳이었다.
		namespace = domain.DefaultStackNamespaceFor(input.Name)
	}
	if uc.platformNamespace != "" && strings.EqualFold(namespace, uc.platformNamespace) {
		return nil, fmt.Errorf(
			"네임스페이스 %q 에는 스택을 설치할 수 없습니다 — 플랫폼이 사는 곳입니다. 다른 이름을 지정하세요",
			namespace)
	}

	existingStacks, err := uc.stackRepo.List(ctx, input.OrgID, false)
	if err != nil {
		return nil, fmt.Errorf("check stack name uniqueness: %w", err)
	}
	normalizedName := strings.ToLower(strings.TrimSpace(input.Name))
	for _, existing := range existingStacks {
		if existing == nil {
			continue
		}
		if existing.ClusterID != input.ClusterID {
			continue
		}
		if strings.ToLower(strings.TrimSpace(existing.Name)) == normalizedName {
			return nil, fmt.Errorf("stack name %q already exists", input.Name)
		}
	}

	if strings.TrimSpace(input.Config.AccessDomain) == "" {
		input.Config.AccessDomain = fmt.Sprintf("%s.internal", input.Name)
	}

	uc.planResources(ctx, &input)

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

	if uc.manageHistory != nil {
		cfg, ok := stackConfigFromInterface(stack.Config)
		if !ok {
			return nil, fmt.Errorf("save initial history: invalid stack config")
		}
		if _, err := uc.manageHistory.SaveVersion(ctx, SaveVersionInput{
			StackID:      stack.ID,
			Config:       cfg,
			ChangedBy:    "system",
			ChangeReason: "stack created",
		}); err != nil {
			return nil, fmt.Errorf("save initial history: %w", err)
		}
	}

	return &CreateStackOutput{Stack: stack}, nil
}

// planResources 는 템플릿의 설치 규모 프로파일로 자원 계획을 채운다.
//
// 계획은 부가 정보다 — 못 세워도 스택 생성을 막지 않는다. 계획이 없으면 설치가
// 관리자 기본값(stack_resource_defaults)이나 차트 기본값으로 진행될 뿐이다.
// 그래서 이 함수는 error 를 돌려주지 않고 조용히 비운다.
func (uc *CreateStack) planResources(ctx context.Context, input *CreateStackInput) {
	if uc.resourceDefaults == nil || uc.templateRepo == nil {
		return
	}
	// 호출자가 계획을 실어 보냈으면 그것이 이긴다. 설치 마법사에서 사용자가
	// 손으로 조정한 값을 서버가 덮어쓰면 계획 화면이 무의미해진다.
	if len(input.Config.AppliedResourceOverrides) > 0 {
		return
	}
	if strings.TrimSpace(input.TemplateID) == "" {
		return
	}

	template, err := uc.templateRepo.GetByID(ctx, input.TemplateID)
	if err != nil || template == nil {
		slog.Debug("resource planning skipped: template not found",
			"template_id", input.TemplateID, "error", err)
		return
	}

	defaults, err := uc.resourceDefaults.List(ctx)
	if err != nil {
		slog.Warn("resource planning skipped: resource defaults unavailable", "error", err)
		return
	}

	baseByToolKey := make(map[string]domain.ResourceVector, len(defaults))
	for _, item := range defaults {
		if item == nil {
			continue
		}
		baseByToolKey[item.ToolKey] = domain.ResourceVector{
			CPURequest:       item.CPURequest,
			CPULimit:         item.CPULimit,
			MemoryRequestGi:  item.MemoryRequestGi,
			MemoryLimitGi:    item.MemoryLimitGi,
			StorageRequestGi: item.StorageRequestGi,
			StorageLimitGi:   item.StorageLimitGi,
		}
	}

	planned := domain.PlanAppliedResources(template.PlanningProfile, input.Config, baseByToolKey)
	if len(planned) == 0 {
		return
	}
	input.Config.AppliedResourceOverrides = planned
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
