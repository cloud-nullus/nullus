package e2e_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	adminhandler "github.com/cloud-nullus/draft/internal/admin/adapter/handler"
	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	adminuc "github.com/cloud-nullus/draft/internal/admin/usecase"
	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
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
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

var testServerURL string

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
	installStackUC := stackuc.NewInstallStack(memStackRepo, memStreamer)
	createStackUC := stackuc.NewCreateStack(memStackRepo, memTemplateRepo)
	listStacksUC := stackuc.NewListStacks(memStackRepo)
	getTemplateUC := stackuc.NewGetTemplate(memTemplateRepo)
	listTemplatesUC := stackuc.NewListTemplates(memTemplateRepo)
	exportConfigUC := stackuc.NewExportConfig(memStackRepo)
	deployHandler := stackhandler.NewDeployHandler(installStackUC, memStackRepo, memStreamer)
	stackHandler := stackhandler.NewStackHandler(createStackUC, listStacksUC, memStackRepo)
	templateHandler := stackhandler.NewTemplateHandler(getTemplateUC, listTemplatesUC, memTemplateRepo)
	exportHandler := stackhandler.NewExportHandler(exportConfigUC)

	// Compatibility + History
	memCompatRepo := stackrepo.NewMemoryCompatibilityRepository()
	validateCompatUC := stackuc.NewValidateCompatibility(memCompatRepo)
	compatHandler := stackhandler.NewCompatibilityHandler(memCompatRepo, validateCompatUC)
	memHistoryRepo := stackrepo.NewMemoryHistoryRepository()
	manageHistoryUC := stackuc.NewManageHistory(memHistoryRepo)
	historyHandler := stackhandler.NewHistoryHandler(memHistoryRepo, memStackRepo, manageHistoryUC)

	// Resources
	calcResourcesUC := stackuc.NewCalculateResources()
	resourceHandler := stackhandler.NewResourceHandler(calcResourcesUC)

	// CI/CD
	cicdTemplateRepo := cicdrepo.NewMemoryCICDTemplateRepository()
	pipelineRepo := cicdrepo.NewMemoryPipelineRepository()
	deploymentRepo := cicdrepo.NewMemoryDeploymentRepository()
	createPipelineUC := cicduc.NewCreatePipeline(pipelineRepo, cicdTemplateRepo)
	listPipelinesUC := cicduc.NewListPipelines(pipelineRepo)
	deployPipelineUC := cicduc.NewDeployPipeline(pipelineRepo, deploymentRepo)
	cicdTemplateHandler := cicdhandler.NewCICDTemplateHandler(cicdTemplateRepo)
	pipelineHandler := cicdhandler.NewPipelineHandler(createPipelineUC, listPipelinesUC, deployPipelineUC, pipelineRepo, deploymentRepo)

	// Observability
	dashboardRepo := obsrepo.NewMemoryDashboardRepository()
	alertRuleRepo := obsrepo.NewMemoryAlertRuleRepository()
	alertRepo := obsrepo.NewMemoryAlertRepository()
	getDashboardUC := obsuc.NewGetDashboard(dashboardRepo)
	createAlertRuleUC := obsuc.NewCreateAlertRule(alertRuleRepo)
	listAlertsUC := obsuc.NewListAlerts(alertRepo)
	dashboardHandler := obshandler.NewDashboardHandler(getDashboardUC)
	alertHandler := obshandler.NewAlertHandler(createAlertRuleUC, listAlertsUC, alertRuleRepo)

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
	dashboardHandler.RegisterRoutes(observability)
	alertHandler.RegisterRoutes(observability)

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	return e
}
