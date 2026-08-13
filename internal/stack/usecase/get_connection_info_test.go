package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func connStack(cfg domain.StackConfig) *domain.Stack {
	return &domain.Stack{
		ID:        "stk_1",
		Namespace: "devsecops",
		State:     "completed",
		Config:    cfg,
	}
}

func fullConfig() domain.StackConfig {
	return domain.StackConfig{
		AccessDomain: "nullus-devsecops-stack.internal",
		Artifacts: domain.ArtifactsConfig{
			SourceRepository:  domain.ToolSelection{Name: "GitLab CE", Enabled: true},
			ContainerRegistry: domain.ToolSelection{Name: "GitLab Registry", Enabled: true},
			StorageBackend:    domain.ToolSelection{Name: "MinIO", Enabled: true},
		},
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Name: "GitLab CI", Enabled: true},
			CDTool:     domain.ToolSelection{Name: "Argo CD", Enabled: true},
		},
		Monitoring: domain.MonitoringConfig{
			Collection:    domain.ToolSelection{Name: "Prometheus", Enabled: true},
			Visualization: domain.ToolSelection{Name: "Grafana", Enabled: true},
		},
	}
}

func newConnUC(stack *domain.Stack) *GetConnectionInfo {
	return NewGetConnectionInfo(&mockConnStackRepo{stack: stack})
}

type mockConnStackRepo struct {
	stack *domain.Stack
	err   error
}

func (m *mockConnStackRepo) GetByID(_ context.Context, _ string) (*domain.Stack, error) {
	return m.stack, m.err
}

func TestGetConnectionInfo_ReturnsStackLocation(t *testing.T) {
	out, err := newConnUC(connStack(fullConfig())).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	assert.Equal(t, "stk_1", out.StackID)
	assert.Equal(t, "devsecops", out.Namespace)
	assert.Equal(t, "nullus-devsecops-stack.internal", out.AccessDomain)
}

// 설치가 만든 실제 이름을 그대로 돌려줘야 한다. 프론트가 다시 조립하면
// 규칙이 갈리는 순간 어긋난다.
func TestGetConnectionInfo_UsesProvisionedNames(t *testing.T) {
	out, err := newConnUC(connStack(fullConfig())).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	assert.Equal(t, domain.ProvisionedPostgresSecret, out.Database.SecretRef)
	assert.Equal(t, domain.PostgresPasswordKey, out.Database.SecretKey)
	assert.Equal(t, domain.PostgresAppUser, out.Database.AuthID)
	assert.Equal(t, "nullus-postgresql:5432", out.Database.Endpoint)

	assert.Equal(t, domain.ProvisionedMinIOSecret, out.ObjectStorage.SecretRef)
	assert.Equal(t, domain.MinIOPasswordKey, out.ObjectStorage.SecretKey)
	assert.Equal(t, domain.MinIORootUser, out.ObjectStorage.AuthID)
	assert.Equal(t, "http://nullus-minio:9000", out.ObjectStorage.Endpoint)
}

// 서비스 이름은 Helm 릴리스명 기준이라 네임스페이스와 무관하다.
func TestGetConnectionInfo_EndpointsAreNamespaceIndependent(t *testing.T) {
	a, err := newConnUC(connStack(fullConfig())).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	other := connStack(fullConfig())
	other.Namespace = "team-a"
	b, err := newConnUC(other).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	assert.Equal(t, a.Database.Endpoint, b.Database.Endpoint)
	assert.Equal(t, a.ObjectStorage.Endpoint, b.ObjectStorage.Endpoint)
}

// 사용자가 지정한 외부 스토리지는 그 값을 그대로 쓴다.
func TestGetConnectionInfo_HonorsExistingConnectTargets(t *testing.T) {
	cfg := fullConfig()
	cfg.Storage = &domain.StorageConfig{
		Database: domain.StorageTarget{
			Mode: "existing-connect", Endpoint: "pg.example.com:5432",
			ResourceName: "appdb", AuthID: "appuser",
			AccessSecretRef: "my-db-secret", AuthPasswordKey: "pw",
			ProviderOrEngine: "postgres",
		},
	}

	out, err := newConnUC(connStack(cfg)).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	assert.Equal(t, "existing-connect", out.Database.Mode)
	assert.Equal(t, "pg.example.com:5432", out.Database.Endpoint)
	assert.Equal(t, "my-db-secret", out.Database.SecretRef)
	assert.Equal(t, "pw", out.Database.SecretKey)
}

func TestGetConnectionInfo_ListsSelectedToolCredentials(t *testing.T) {
	out, err := newConnUC(connStack(fullConfig())).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	byName := map[string]domain.ToolCredential{}
	for _, t := range out.Tools {
		byName[t.Name] = t
	}

	argo, ok := byName["Argo CD"]
	require.True(t, ok)
	assert.Equal(t, domain.ArgoCDAdminSecret, argo.SecretRef)
	assert.Equal(t, "admin", argo.Username)

	gitlab, ok := byName["GitLab CE"]
	require.True(t, ok)
	assert.Equal(t, domain.GitLabRootSecret, gitlab.SecretRef)
	assert.Equal(t, "root", gitlab.Username)
}

// 고르지 않은 도구는 안내에 나오면 안 된다.
func TestGetConnectionInfo_SkipsDisabledTools(t *testing.T) {
	cfg := fullConfig()
	cfg.Pipeline.CDTool.Enabled = false

	out, err := newConnUC(connStack(cfg)).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	for _, tool := range out.Tools {
		assert.NotEqual(t, "Argo CD", tool.Name)
	}
}

// 조회할 Secret 이 없는 도구는 이름을 지어내지 않고 안내문만 남긴다.
func TestGetConnectionInfo_ToolWithoutSecretCarriesNote(t *testing.T) {
	out, err := newConnUC(connStack(fullConfig())).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	for _, tool := range out.Tools {
		if tool.Name == "Prometheus" {
			assert.Empty(t, tool.SecretRef)
			assert.NotEmpty(t, tool.Note)
			return
		}
	}
	t.Fatal("Prometheus 항목이 없다")
}

// 수집기는 자격증명이 없다. 안내의 본체는 "어디로 보내야 하는가" 이므로
// 그 주소가 비면 수집기가 떠 있어도 아무도 텔레메트리를 보내지 않는다.
func TestGetConnectionInfo_AnnouncesOTelCollectorEndpoint(t *testing.T) {
	cfg := fullConfig()
	cfg.Logging.TraceExporter = domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true}

	out, err := newConnUC(connStack(cfg)).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	for _, tool := range out.Tools {
		if tool.Name != "opentelemetry-collector" {
			continue
		}
		assert.Empty(t, tool.SecretRef)
		// 스택 네임스페이스를 반영한 실제 주소여야 한다.
		assert.Contains(t, tool.Note, "otel-collector-opentelemetry-collector.devsecops.svc.cluster.local:4317")
		assert.Contains(t, tool.Note, ":4318")
		return
	}
	t.Fatal("OpenTelemetry Collector 항목이 없다")
}

func TestGetConnectionInfo_SkipsOTelCollectorWhenNotSelected(t *testing.T) {
	out, err := newConnUC(connStack(fullConfig())).Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	for _, tool := range out.Tools {
		assert.NotEqual(t, "opentelemetry-collector", tool.Name)
	}
}

func TestGetConnectionInfo_FailsWhenStackMissing(t *testing.T) {
	uc := NewGetConnectionInfo(&mockConnStackRepo{stack: nil})
	_, err := uc.Execute(context.Background(), "stk_missing")
	require.Error(t, err)
}

func TestGetConnectionInfo_RequiresStackID(t *testing.T) {
	_, err := newConnUC(connStack(fullConfig())).Execute(context.Background(), "  ")
	require.Error(t, err)
}
