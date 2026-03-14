package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	adminhandler "github.com/cloud-nullus/draft/internal/admin/adapter/handler"
	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	cicdrepo "github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	cicduc "github.com/cloud-nullus/draft/internal/cicd/usecase"
	obshandler "github.com/cloud-nullus/draft/internal/observability/adapter/handler"
	obsrepo "github.com/cloud-nullus/draft/internal/observability/adapter/repository"
	obsuc "github.com/cloud-nullus/draft/internal/observability/usecase"
	"github.com/cloud-nullus/draft/internal/shared/config"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
	logadapter "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	stackuc "github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize database pool
	dsn := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Name,
		cfg.Database.User, cfg.Database.Password, cfg.Database.SSLMode,
	)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		slog.Error("failed to parse database config", "error", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.Database.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.Database.ConnMaxLifetime
	poolCfg.MaxConnIdleTime = cfg.Database.ConnMaxIdleTime
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Initialize repositories
	orgRepo := adminrepo.NewPostgresOrgRepository(pool)
	clusterRepo := adminrepo.NewPostgresClusterRepository(pool)

	// Initialize use cases
	orgUC := usecase.NewOrgUseCase(orgRepo)
	clusterUC := usecase.NewClusterUseCase(clusterRepo)

	// Initialize handlers
	orgHandler := adminhandler.NewOrgHandler(orgUC)
	clusterHandler := adminhandler.NewClusterHandler(clusterUC)

	// Stack: in-memory repos + log streamer
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
	templateHandler := stackhandler.NewTemplateHandler(getTemplateUC, listTemplatesUC)
	exportHandler := stackhandler.NewExportHandler(exportConfigUC)

	// Compatibility
	memCompatRepo := stackrepo.NewMemoryCompatibilityRepository()
	validateCompatUC := stackuc.NewValidateCompatibility(memCompatRepo)
	compatHandler := stackhandler.NewCompatibilityHandler(memCompatRepo, validateCompatUC)

	// History
	memHistoryRepo := stackrepo.NewMemoryHistoryRepository()
	manageHistoryUC := stackuc.NewManageHistory(memHistoryRepo)
	historyHandler := stackhandler.NewHistoryHandler(memHistoryRepo, memStackRepo, manageHistoryUC)

	// CI/CD: in-memory repos
	memCICDTemplateRepo := cicdrepo.NewMemoryCICDTemplateRepository()
	memPipelineRepo := cicdrepo.NewMemoryPipelineRepository()
	memDeploymentRepo := cicdrepo.NewMemoryDeploymentRepository()
	createPipelineUC := cicduc.NewCreatePipeline(memPipelineRepo, memCICDTemplateRepo)
	listPipelinesUC := cicduc.NewListPipelines(memPipelineRepo)
	deployPipelineUC := cicduc.NewDeployPipeline(memPipelineRepo, memDeploymentRepo)
	cicdTemplateHandler := cicdhandler.NewCICDTemplateHandler(memCICDTemplateRepo)
	pipelineHandler := cicdhandler.NewPipelineHandler(createPipelineUC, listPipelinesUC, deployPipelineUC, memPipelineRepo, memDeploymentRepo)

	// Observability: in-memory repos
	memDashboardRepo := obsrepo.NewMemoryDashboardRepository()
	memAlertRuleRepo := obsrepo.NewMemoryAlertRuleRepository()
	memAlertRepo := obsrepo.NewMemoryAlertRepository()
	getDashboardUC := obsuc.NewGetDashboard(memDashboardRepo)
	createAlertRuleUC := obsuc.NewCreateAlertRule(memAlertRuleRepo)
	listAlertsUC := obsuc.NewListAlerts(memAlertRepo)
	dashboardHandler := obshandler.NewDashboardHandler(getDashboardUC)
	alertHandler := obshandler.NewAlertHandler(createAlertRuleUC, listAlertsUC, memAlertRuleRepo)

	// Initialize Echo
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	// Global middleware
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(middleware.SlogLogger())

	// API v1 group
	v1 := e.Group("/api/v1")
	orgHandler.RegisterRoutes(v1)
	clusterHandler.RegisterRoutes(v1)
	deployHandler.RegisterRoutes(v1, e)
	stackHandler.RegisterRoutes(v1)
	templateHandler.RegisterRoutes(v1)
	exportHandler.RegisterRoutes(v1)
	compatHandler.RegisterRoutes(v1)
	historyHandler.RegisterRoutes(v1)
	cicdTemplateHandler.RegisterRoutes(v1)
	pipelineHandler.RegisterRoutes(v1)
	dashboardHandler.RegisterRoutes(v1)
	alertHandler.RegisterRoutes(v1)

	// Development-only profiling endpoints
	if cfg.Server.Mode == "development" {
		e.GET("/debug/pprof/*", echo.WrapHandler(http.DefaultServeMux))
	}

	// Health check with DB ping
	e.GET("/health", func(c echo.Context) error {
		dbStatus := "connected"
		if err := pool.Ping(c.Request().Context()); err != nil {
			slog.Warn("health check db ping failed", "error", err)
			dbStatus = "unavailable"
		}
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "healthy",
			"db":      dbStatus,
			"version": "0.1.0-alpha",
		})
	})

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	go func() {
		slog.Info("starting server", "addr", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	slog.Info("server stopped")
}
