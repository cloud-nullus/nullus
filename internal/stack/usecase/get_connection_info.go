package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// connectionStackReader 는 연결정보 조회에 필요한 최소 동작이다.
type connectionStackReader interface {
	GetByID(ctx context.Context, id string) (*domain.Stack, error)
}

// GetConnectionInfo 는 스택 접속에 필요한 정보를 조립한다.
//
// 리소스 이름은 domain 의 상수에서 온다. 화면이 같은 값을 다시 조립하면
// 규칙이 갈리는 순간 조용히 어긋나므로, 서버가 확정된 값을 내려준다.
type GetConnectionInfo struct {
	stacks connectionStackReader
}

// NewGetConnectionInfo 는 유스케이스를 만든다.
func NewGetConnectionInfo(stacks connectionStackReader) *GetConnectionInfo {
	return &GetConnectionInfo{stacks: stacks}
}

// Execute 는 스택의 연결정보를 돌려준다.
func (uc *GetConnectionInfo) Execute(ctx context.Context, stackID string) (*domain.ConnectionInfo, error) {
	id := strings.TrimSpace(stackID)
	if id == "" {
		return nil, fmt.Errorf("stack_id is required")
	}

	stack, err := uc.stacks.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get stack %s: %w", id, err)
	}
	if stack == nil {
		return nil, fmt.Errorf("stack %s not found", id)
	}

	cfg := stackConfigOf(stack)

	return &domain.ConnectionInfo{
		StackID:       stack.ID,
		Namespace:     strings.TrimSpace(stack.Namespace),
		AccessDomain:  strings.TrimSpace(cfg.AccessDomain),
		Database:      databaseConnection(cfg),
		ObjectStorage: objectStorageConnection(cfg),
		Tools:         toolCredentials(cfg),
	}, nil
}

func stackConfigOf(stack *domain.Stack) domain.StackConfig {
	if cfg, ok := stack.Config.(domain.StackConfig); ok {
		return cfg
	}
	if cfg, ok := stack.Config.(*domain.StackConfig); ok && cfg != nil {
		return *cfg
	}
	return domain.StackConfig{}
}

// databaseConnection 은 DB 접속 정보를 만든다.
//
// 사용자가 외부 DB 를 지정했으면 그 값을 그대로 쓰고, 설치가 만든 경우에만
// 프로비저닝 상수를 쓴다.
func databaseConnection(cfg domain.StackConfig) domain.StorageConnection {
	provisioned := domain.StorageConnection{
		Mode:         "create",
		Engine:       "postgres",
		Endpoint:     fmt.Sprintf("%s:%d", domain.PostgresServiceName, domain.PostgresServicePort),
		ResourceName: domain.PostgresAppDatabase,
		AuthID:       domain.PostgresAppUser,
		SecretRef:    domain.ProvisionedPostgresSecret,
		SecretKey:    domain.PostgresPasswordKey,
	}
	if cfg.Storage == nil {
		return provisioned
	}
	return mergeStorageTarget(cfg.Storage.Database, provisioned)
}

// objectStorageConnection 은 오브젝트 스토리지 접속 정보를 만든다.
func objectStorageConnection(cfg domain.StackConfig) domain.StorageConnection {
	provisioned := domain.StorageConnection{
		Mode:         "create",
		Engine:       "minio",
		Endpoint:     fmt.Sprintf("http://%s:%d", domain.MinIOServiceName, domain.MinIOServicePort),
		ResourceName: "gitlab-artifacts",
		AuthID:       domain.MinIORootUser,
		SecretRef:    domain.ProvisionedMinIOSecret,
		SecretKey:    domain.MinIOPasswordKey,
	}
	if cfg.Storage == nil {
		return provisioned
	}
	return mergeStorageTarget(cfg.Storage.ObjectStorage, provisioned)
}

// mergeStorageTarget 은 설정에 값이 있으면 그것을, 없으면 프로비저닝 값을 쓴다.
func mergeStorageTarget(target domain.StorageTarget, provisioned domain.StorageConnection) domain.StorageConnection {
	mode := strings.TrimSpace(target.Mode)
	if mode == "" || mode == "create" {
		if mode != "" {
			provisioned.Mode = mode
		}
		return provisioned
	}

	// 외부 연결은 사용자가 준 값이 진실이다. 비어 있는 항목만 채운다.
	return domain.StorageConnection{
		Mode:         mode,
		Engine:       firstNonBlank(target.ProviderOrEngine, provisioned.Engine),
		Endpoint:     firstNonBlank(target.Endpoint, provisioned.Endpoint),
		ResourceName: firstNonBlank(target.ResourceName, provisioned.ResourceName),
		AuthID:       firstNonBlank(target.AuthID, provisioned.AuthID),
		SecretRef:    strings.TrimSpace(target.AccessSecretRef),
		SecretKey:    strings.TrimSpace(target.AuthPasswordKey),
	}
}

// toolCredentials 는 선택된 도구의 접속 안내를 만든다.
//
// 고르지 않은 도구는 넣지 않는다. 조회할 Secret 이 없으면 이름을 지어내지 않고
// Note 로 무엇을 해야 하는지 알린다.
func toolCredentials(cfg domain.StackConfig) []domain.ToolCredential {
	tools := make([]domain.ToolCredential, 0, 6)

	if sel := cfg.Artifacts.SourceRepository; sel.Enabled && isGitLabTool(sel.Name) {
		tools = append(tools, domain.ToolCredential{
			Name:      sel.Name,
			Username:  domain.GitLabRootUser,
			SecretRef: domain.GitLabRootSecret,
			SecretKey: domain.GitLabRootPassKey,
		})
	}
	if sel := cfg.Pipeline.CDTool; sel.Enabled && isArgoCDTool(sel.Name) {
		tools = append(tools, domain.ToolCredential{
			Name:      sel.Name,
			Username:  domain.ArgoCDAdminUser,
			SecretRef: domain.ArgoCDAdminSecret,
			SecretKey: domain.ArgoCDAdminPassKey,
		})
	}
	if sel := cfg.Monitoring.Visualization; sel.Enabled && normalizeTool(sel.Name) == "grafana" {
		tools = append(tools, domain.ToolCredential{
			Name:     sel.Name,
			Username: domain.GrafanaAdminUser,
			Note:     "values 에서 관리자 비밀번호를 지정하지 않았다면 차트 기본값을 확인하세요.",
		})
	}
	if sel := cfg.Artifacts.StorageBackend; sel.Enabled && normalizeTool(sel.Name) == "minio" {
		tools = append(tools, domain.ToolCredential{
			Name:      sel.Name,
			Username:  domain.MinIORootUser,
			SecretRef: domain.ProvisionedMinIOSecret,
			SecretKey: domain.MinIOPasswordKey,
		})
	}
	if sel := cfg.Monitoring.Collection; sel.Enabled && normalizeTool(sel.Name) == "prometheus" {
		tools = append(tools, domain.ToolCredential{
			Name: sel.Name,
			Note: "기본 설정에서는 로그인이 필요 없습니다.",
		})
	}
	if cfg.Authentication != nil && normalizeTool(cfg.Authentication.Provider) == "openbao" {
		tools = append(tools, domain.ToolCredential{
			Name: "OpenBao",
			Note: "관리자 인증 후 OpenBao UI 에서 토큰과 시크릿을 조회하세요.",
		})
	}

	return tools
}

func isGitLabTool(name string) bool {
	switch normalizeTool(name) {
	case "gitlab", "gitlab-ce", "gitlab-ee":
		return true
	}
	return false
}

func isArgoCDTool(name string) bool {
	switch normalizeTool(name) {
	case "argocd", "argo-cd":
		return true
	}
	return false
}

// normalizeTool 은 카탈로그 표기 흔들림("Argo CD" / "argocd")을 흡수한다.
func normalizeTool(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	return strings.NewReplacer(" ", "-", "_", "-").Replace(n)
}

func firstNonBlank(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
