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
	authadapter "github.com/cloud-nullus/draft/internal/auth/adapter"
	authmw "github.com/cloud-nullus/draft/internal/auth/adapter/middleware"
	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	cicdrepo "github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	cicduc "github.com/cloud-nullus/draft/internal/cicd/usecase"
	obshandler "github.com/cloud-nullus/draft/internal/observability/adapter/handler"
	obsprom "github.com/cloud-nullus/draft/internal/observability/adapter/prometheus"
	obsrepo "github.com/cloud-nullus/draft/internal/observability/adapter/repository"
	obsport "github.com/cloud-nullus/draft/internal/observability/port"
	obsuc "github.com/cloud-nullus/draft/internal/observability/usecase"
	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/shared/config"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackhelm "github.com/cloud-nullus/draft/internal/stack/adapter/helm"
	logadapter "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	stackport "github.com/cloud-nullus/draft/internal/stack/port"
	stackuc "github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func main() {
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

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

	// Admin: postgres repos
	orgRepo := adminrepo.NewPostgresOrgRepository(pool)
	clusterRepo := adminrepo.NewPostgresClusterRepository(pool)
	userRepo := adminrepo.NewPostgresUserRepository(pool)

	orgUC := usecase.NewOrgUseCase(orgRepo)
	clusterUC := usecase.NewClusterUseCase(clusterRepo, usecase.WithOrgRepo(orgRepo))
	userUC := usecase.NewUserUseCase(userRepo)
	auditLogger := audit.NewAuditLogger(pool)

	orgHandler := adminhandler.NewOrgHandler(orgUC, auditLogger)
	clusterHandler := adminhandler.NewClusterHandler(clusterUC, auditLogger)
	memberHandler := adminhandler.NewMemberHandler(userUC, auditLogger)

	// Stack: postgres repos + log streamer
	pgStackRepo := stackrepo.NewPostgresStackRepository(pool)
	pgTemplateRepo := stackrepo.NewPostgresTemplateRepository(pool)
	pgResourceDefaultRepo := stackrepo.NewPostgresResourceDefaultRepository(pool)
	memStreamer := logadapter.NewMemoryStreamer()
	kubeconfigProvider := stackrepo.NewPostgresKubeconfigProvider(pool, []byte(os.Getenv("ENCRYPTION_KEY")))

	installStackUC := stackuc.NewInstallStack(
		pgStackRepo,
		memStreamer,
		stackuc.WithKubeconfigProvider(kubeconfigProvider),
		stackuc.WithExecutorFactory(func(kubeconfig []byte) stackport.StepExecutor {
			installer := stackhelm.NewHelmInstaller(kubeconfig)
			return stackhelm.NewOrchestrator(installer, kubeconfig, "")
		}),
	)
	createStackUC := stackuc.NewCreateStack(pgStackRepo, pgTemplateRepo)
	listStacksUC := stackuc.NewListStacks(pgStackRepo)
	deleteStackUC := stackuc.NewDeleteStack(
		pgStackRepo,
		kubeconfigProvider,
		func(kubeconfig []byte) stackport.HelmInstaller {
			return stackhelm.NewHelmInstaller(kubeconfig)
		},
	)
	addToolsUC := stackuc.NewAddToolsUseCase(pgStackRepo)
	getTemplateUC := stackuc.NewGetTemplate(pgTemplateRepo)
	listTemplatesUC := stackuc.NewListTemplates(pgTemplateRepo)
	exportConfigUC := stackuc.NewExportConfig(pgStackRepo)
	calculateResourcesUC := stackuc.NewCalculateResources()
	listResourceDefaultsUC := stackuc.NewListResourceDefaults(pgResourceDefaultRepo)
	upsertResourceDefaultUC := stackuc.NewUpsertResourceDefault(pgResourceDefaultRepo)

	deployHandler := stackhandler.NewDeployHandler(installStackUC, pgStackRepo, memStreamer, auditLogger)
	stackHandler := stackhandler.NewStackHandler(createStackUC, listStacksUC, deleteStackUC, addToolsUC, pgStackRepo, auditLogger)
	templateHandler := stackhandler.NewTemplateHandler(getTemplateUC, listTemplatesUC, pgTemplateRepo)
	exportHandler := stackhandler.NewExportHandler(exportConfigUC)
	resourceHandler := stackhandler.NewResourceHandler(calculateResourcesUC, listResourceDefaultsUC, upsertResourceDefaultUC)

	pgCompatRepo := stackrepo.NewPostgresCompatibilityRepository(pool)
	validateCompatUC := stackuc.NewValidateCompatibility(pgCompatRepo)
	compatHandler := stackhandler.NewCompatibilityHandler(pgCompatRepo, validateCompatUC)

	pgHistoryRepo := stackrepo.NewPostgresHistoryRepository(pool)
	manageHistoryUC := stackuc.NewManageHistory(pgHistoryRepo)
	historyHandler := stackhandler.NewHistoryHandler(pgHistoryRepo, pgStackRepo, manageHistoryUC)

	// CI/CD: postgres repos
	pgCICDTemplateRepo := cicdrepo.NewPostgresCICDTemplateRepository(pool)
	pgPipelineRepo := cicdrepo.NewPostgresPipelineRepository(pool)
	pgDeploymentRepo := cicdrepo.NewPostgresDeploymentRepository(pool)
	createPipelineUC := cicduc.NewCreatePipeline(pgPipelineRepo, pgCICDTemplateRepo)
	listPipelinesUC := cicduc.NewListPipelines(pgPipelineRepo)
	deployPipelineUC := cicduc.NewDeployPipeline(pgPipelineRepo, pgDeploymentRepo)
	cicdTemplateHandler := cicdhandler.NewCICDTemplateHandler(pgCICDTemplateRepo)
	pipelineHandler := cicdhandler.NewPipelineHandler(createPipelineUC, listPipelinesUC, deployPipelineUC, pgPipelineRepo, pgDeploymentRepo)

	// Observability: Prometheus with in-memory fallback
	var dashboardRepo obsport.DashboardRepository
	if cfg.Prometheus.URL != "" {
		promClient := obsprom.NewClient(cfg.Prometheus.URL)
		dashboardRepo = obsprom.NewDashboardRepository(promClient)
		slog.Info("using prometheus dashboard", "url", cfg.Prometheus.URL)
	} else {
		dashboardRepo = obsrepo.NewMemoryDashboardRepository()
		slog.Info("using in-memory dashboard (prometheus not configured)")
	}
	pgAlertRuleRepo := obsrepo.NewPostgresAlertRuleRepository(pool)
	pgAlertRepo := obsrepo.NewPostgresAlertRepository(pool)
	getDashboardUC := obsuc.NewGetDashboard(dashboardRepo)
	createAlertRuleUC := obsuc.NewCreateAlertRule(pgAlertRuleRepo)
	listAlertsUC := obsuc.NewListAlerts(pgAlertRepo)
	dashboardHandler := obshandler.NewDashboardHandler(getDashboardUC)
	alertHandler := obshandler.NewAlertHandler(createAlertRuleUC, listAlertsUC, pgAlertRuleRepo)

	// Echo
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	// Global middleware
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(middleware.SlogLogger())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-Org-ID"},
		AllowCredentials: true,
		MaxAge:           7200,
	}))
	if cfg.Server.Mode == "production" {
		e.Use(middleware.RateLimiter(middleware.RateLimitConfig{
			Authenticated:   300,
			Unauthenticated: 30,
		}))
	}

	// API v1 group
	v1 := e.Group("/api/v1")

	var admin, stacks, cicd, observability *echo.Group
	if cfg.Server.Mode == "development" {
		slog.Info("development mode: auth middleware disabled")
		admin = v1.Group("/admin")
		stacks = v1.Group("/stacks")
		cicd = v1.Group("/cicd")
		observability = v1.Group("/observability")
	} else {
		sessionMW := authmw.AuthMiddleware()
		oidcProvider, err := authadapter.NewOIDCProvider(cfg.Auth.OIDC.Provider)
		if err != nil {
			slog.Error("failed to initialize OIDC provider", "provider", cfg.Auth.OIDC.Provider, "error", err)
			os.Exit(1)
		}
		oidcMW := authmw.JWTAuthMiddleware(authmw.JWTConfig{
			IssuerURL: cfg.Auth.OIDC.IssuerURL,
			Audience:  cfg.Auth.OIDC.Audience,
		}, oidcProvider)
		authMW := authmw.DualAuthMiddleware(cfg.Auth.Mode, sessionMW, oidcMW)
		admin = v1.Group("/admin", authMW, authmw.RequireRole("admin"))
		stacks = v1.Group("/stacks", authMW, authmw.RequireRole("admin", "devops"))
		cicd = v1.Group("/cicd", authMW, authmw.RequireRole("admin", "devops", "developer"))
		observability = v1.Group("/observability", authMW)
	}

	knownIssuesRepo := adminrepo.NewPostgresKnownIssuesRepository(pool)
	knownIssuesHandler := adminhandler.NewKnownIssuesHandler(knownIssuesRepo)
	auditHandler := adminhandler.NewAuditHandler(auditLogger)
	notificationHandler := adminhandler.NewNotificationHandler(pool)

	orgHandler.RegisterRoutes(admin)
	clusterHandler.RegisterRoutes(admin)
	memberHandler.RegisterRoutes(admin)
	knownIssuesHandler.RegisterRoutes(admin)
	auditHandler.RegisterRoutes(admin)
	notificationHandler.RegisterRoutes(admin)
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

	if cfg.Server.Mode == "development" {
		e.GET("/debug/pprof/*", echo.WrapHandler(http.DefaultServeMux))
	}

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

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	go func() {
		slog.Info("starting server", "addr", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

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
