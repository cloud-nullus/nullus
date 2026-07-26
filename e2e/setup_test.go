package e2e_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	adminhandler "github.com/cloud-nullus/draft/internal/admin/adapter/handler"
	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	adminuc "github.com/cloud-nullus/draft/internal/admin/usecase"
	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	cicdkube "github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	cicdrepo "github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	cicduc "github.com/cloud-nullus/draft/internal/cicd/usecase"
	obshandler "github.com/cloud-nullus/draft/internal/observability/adapter/handler"
	obsrepo "github.com/cloud-nullus/draft/internal/observability/adapter/repository"
	obsuc "github.com/cloud-nullus/draft/internal/observability/usecase"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	logadapter "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	stackuc "github.com/cloud-nullus/draft/internal/stack/usecase"
)

var testServerURL string

type noopKubeconfigProvider struct{}

func (n *noopKubeconfigProvider) GetKubeconfig(_ context.Context, _ string) ([]byte, error) {
	return []byte("fake-kubeconfig"), nil
}

type noopManifestApplier struct{}

func (n *noopManifestApplier) Apply(_ context.Context, _ []byte, _ []string) error {
	return nil
}

func (n *noopManifestApplier) ApplyWithTracking(_ context.Context, _ []byte, _ []string, _ string, _ ...int) error {
	return nil
}

func TestMain(m *testing.M) {
	e := newEchoServer()
	ts := httptest.NewServer(e)
	testServerURL = ts.URL
	code := m.Run()
	ts.Close()
	os.Exit(code)
}

func newEchoServer() *echo.Echo {
	// Admin
	orgRepo := adminrepo.NewMemoryOrgRepository()
	clusterRepo := adminrepo.NewMemoryClusterRepository()
	orgUC := adminuc.NewOrgUseCase(orgRepo)
	clusterUC := adminuc.NewClusterUseCase(clusterRepo)
	orgHandler := adminhandler.NewOrgHandler(orgUC)
	clusterHandler := adminhandler.NewClusterHandler(clusterUC)

	// Stack
	memStackRepo := stackrepo.NewMemoryStackRepository()
	memTemplateRepo := stackrepo.NewMemoryTemplateRepository()
	memStreamer := logadapter.NewMemoryStreamer()
	memHistoryRepo := stackrepo.NewMemoryHistoryRepository()
	manageHistoryUC := stackuc.NewManageHistory(memHistoryRepo)
	installStackUC := stackuc.NewInstallStack(memStackRepo, memStreamer)
	createStackUC := stackuc.NewCreateStack(memStackRepo, memTemplateRepo, stackuc.WithManageHistory(manageHistoryUC))
	listStacksUC := stackuc.NewListStacks(memStackRepo)
	deleteStackUC := stackuc.NewDeleteStack(memStackRepo, nil, nil)
	addToolsUC := stackuc.NewAddToolsUseCase(memStackRepo)
	importConfigUC := stackuc.NewImportConfig(createStackUC, addToolsUC, installStackUC)
	getTemplateUC := stackuc.NewGetTemplate(memTemplateRepo)
	listTemplatesUC := stackuc.NewListTemplates(memTemplateRepo)
	exportConfigUC := stackuc.NewExportConfig(memStackRepo)
	deployHandler := stackhandler.NewDeployHandler(installStackUC, memStackRepo, memStreamer)
	stackHandler := stackhandler.NewStackHandler(createStackUC, listStacksUC, deleteStackUC, addToolsUC, memStackRepo, nil, stackhandler.WithStackManageHistory(manageHistoryUC))
	templateHandler := stackhandler.NewTemplateHandler(getTemplateUC, listTemplatesUC, memTemplateRepo)
	exportHandler := stackhandler.NewExportHandler(exportConfigUC, importConfigUC)

	// Compatibility + History
	memCompatRepo := stackrepo.NewMemoryCompatibilityRepository()
	validateCompatUC := stackuc.NewValidateCompatibility(memCompatRepo)
	compatHandler := stackhandler.NewCompatibilityHandler(memCompatRepo, validateCompatUC)
	historyHandler := stackhandler.NewHistoryHandler(memHistoryRepo, memStackRepo, manageHistoryUC)

	// Resources
	calcResourcesUC := stackuc.NewCalculateResources()
	memResourceDefaultRepo := stackrepo.NewMemoryResourceDefaultRepository()
	listResourceDefaultsUC := stackuc.NewListResourceDefaults(memResourceDefaultRepo)
	upsertResourceDefaultUC := stackuc.NewUpsertResourceDefault(memResourceDefaultRepo)
	resourceHandler := stackhandler.NewResourceHandler(calcResourcesUC, listResourceDefaultsUC, upsertResourceDefaultUC)

	// CI/CD
	cicdTemplateRepo := cicdrepo.NewMemoryCICDTemplateRepository()
	pipelineRepo := cicdrepo.NewMemoryPipelineRepository()
	deploymentRepo := cicdrepo.NewMemoryDeploymentRepository()
	createPipelineUC := cicduc.NewCreatePipeline(pipelineRepo, cicdTemplateRepo)
	listPipelinesUC := cicduc.NewListPipelines(pipelineRepo)
	deployPipelineUC := cicduc.NewDeployPipeline(pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, &noopManifestApplier{})
	cicdTemplateHandler := cicdhandler.NewCICDTemplateHandler(cicdTemplateRepo)
	pipelineHandler := cicdhandler.NewPipelineHandler(
		createPipelineUC,
		listPipelinesUC,
		deployPipelineUC,
		pipelineRepo,
		deploymentRepo,
		&noopKubeconfigProvider{},
		cicdkube.NewStepTracker(),
		nil,
	)

	// Observability
	dashboardRepo := obsrepo.NewMemoryDashboardRepository()
	alertRuleRepo := obsrepo.NewMemoryAlertRuleRepository()
	alertRepo := obsrepo.NewMemoryAlertRepository()
	getDashboardUC := obsuc.NewGetDashboard(dashboardRepo)
	createAlertRuleUC := obsuc.NewCreateAlertRule(alertRuleRepo)
	getAlertRuleUC := obsuc.NewGetAlertRule(alertRuleRepo)
	listAlertRulesUC := obsuc.NewListAlertRules(alertRuleRepo)
	updateAlertRuleUC := obsuc.NewUpdateAlertRule(alertRuleRepo)
	deleteAlertRuleUC := obsuc.NewDeleteAlertRule(alertRuleRepo)
	listAlertsUC := obsuc.NewListAlerts(alertRepo)
	dashboardHandler := obshandler.NewDashboardHandler(getDashboardUC)
	alertHandler := obshandler.NewAlertHandler(createAlertRuleUC, getAlertRuleUC, listAlertRulesUC, updateAlertRuleUC, deleteAlertRuleUC, listAlertsUC)

	// Echo setup
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())

	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	stacks := v1.Group("/stacks")
	cicd := v1.Group("/cicd")
	observability := v1.Group("/observability")

	orgHandler.RegisterRoutes(admin)
	clusterHandler.RegisterRoutes(admin)
	deployHandler.RegisterRoutes(v1, e)
	stackHandler.RegisterRoutes(stacks)
	templateHandler.RegisterRoutes(stacks)
	exportHandler.RegisterRoutes(v1)
	compatHandler.RegisterRoutes(stacks)
	historyHandler.RegisterRoutes(stacks)
	resourceHandler.RegisterRoutes(stacks)
	cicdTemplateHandler.RegisterRoutes(cicd)
	pipelineHandler.RegisterRoutes(cicd)
	pipelineHandler.RegisterStackRoutes(stacks)
	dashboardHandler.RegisterRoutes(observability)
	alertHandler.RegisterRoutes(observability)

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	return e
}
