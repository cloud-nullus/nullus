package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	adminhandler "github.com/cloud-nullus/draft/internal/admin/adapter/handler"
	adminkube "github.com/cloud-nullus/draft/internal/admin/adapter/kube"
	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	"github.com/cloud-nullus/draft/internal/admin/rotation"
	adminscheduler "github.com/cloud-nullus/draft/internal/admin/scheduler"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	authadapter "github.com/cloud-nullus/draft/internal/auth/adapter"
	keycloakadapter "github.com/cloud-nullus/draft/internal/auth/adapter/keycloak"
	authmw "github.com/cloud-nullus/draft/internal/auth/adapter/middleware"
	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	cicdkube "github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
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
	"github.com/cloud-nullus/draft/internal/shared/secrets"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackhelm "github.com/cloud-nullus/draft/internal/stack/adapter/helm"
	logadapter "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	stackport "github.com/cloud-nullus/draft/internal/stack/port"
	stackuc "github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/cloud-nullus/draft/pkg/crypto"
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
	encryptionKey := []byte(os.Getenv("ENCRYPTION_KEY"))
	clusterUC := usecase.NewClusterUseCase(
		clusterRepo,
		usecase.WithOrgRepo(orgRepo),
		usecase.WithDiscoverer(adminkube.NewDiscoverer()),
		usecase.WithKubeconfigDecryptor(func(encrypted []byte) ([]byte, error) {
			return crypto.Decrypt(encryptionKey, string(encrypted))
		}),
	)
	userUC := usecase.NewUserUseCase(userRepo)
	auditLogger := audit.NewAuditLogger(pool)

	secretRouter := secrets.NewRouter()
	// 로컬 개발 전용 fallback: 고정 주소/토큰으로 전역 등록한다.
	// 운영 경로는 아래 resolver 가 스택별로 Kubernetes Auth Store 를 만든다.
	if openbaoAddr := strings.TrimSpace(os.Getenv("OPENBAO_ADDR")); openbaoAddr != "" {
		openbaoToken := strings.TrimSpace(os.Getenv("OPENBAO_TOKEN"))
		if openbaoToken != "" {
			secretRouter.Register("openbao", secrets.NewOpenBaoStore(openbaoAddr, openbaoToken))
		}
	}
	tokenSourceRepo := adminrepo.NewPostgresTokenSourceRepository(pool)
	tokenSourceUC := usecase.NewTokenSourceUseCase(tokenSourceRepo, usecase.WithSecretRouter(secretRouter))

	orgHandler := adminhandler.NewOrgHandler(orgUC, auditLogger)
	clusterHandler := adminhandler.NewClusterHandler(clusterUC, auditLogger)
	memberHandler := adminhandler.NewMemberHandler(userUC, auditLogger)
	pgResourceProfileRepo := adminrepo.NewPostgresResourceProfileRepository(pool)
	resourceProfileHandler := adminhandler.NewResourceProfileHandler(orgUC, pgResourceProfileRepo, auditLogger)

	// Stack: postgres repos + log streamer
	pgStackRepo := stackrepo.NewPostgresStackRepository(pool)
	pgTemplateRepo := stackrepo.NewPostgresTemplateRepository(pool)
	pgHelmStepMetadataRepo := stackrepo.NewPostgresHelmStepMetadataRepository(pool)
	pgHistoryRepo := stackrepo.NewPostgresHistoryRepository(pool)
	manageHistoryUC := stackuc.NewManageHistory(pgHistoryRepo)
	memStreamer := logadapter.NewMemoryStreamer()
	kubeconfigProvider := stackrepo.NewPostgresKubeconfigProvider(pool, []byte(os.Getenv("ENCRYPTION_KEY")))

	// 플랫폼 Keycloak 에 OSS OIDC 클라이언트를 등록하는 팩토리.
	// KEYCLOAK_URL 이 없으면 SSO 프로비저닝은 건너뛴다 (BYO / 미사용 모드).
	var ssoFactory stackport.SSOProvisionerFactory
	if kcURL := strings.TrimSpace(os.Getenv("KEYCLOAK_URL")); kcURL != "" {
		ssoFactory = keycloakadapter.NewStackSSOFactory(keycloakadapter.NewKeycloakClient(
			kcURL,
			envOrDefault("KEYCLOAK_REALM", "nullus"),
			envOrDefault("KEYCLOAK_ADMIN_USER", "admin"),
			os.Getenv("KEYCLOAK_ADMIN_PASSWORD"),
		))
	}

	// 스택별 OpenBao 해석기. OpenBao 는 스택마다 배포되므로 주소가 전역 하나일 수 없다.
	// 대상 클러스터의 kubeconfig 로 Kubernetes Auth 기반 Store 를 만든다.
	secretRouter.WithResolver(adminrepo.NewStackSecretResolver(pool, kubeconfigProvider))

	installStackUC := stackuc.NewInstallStack(
		pgStackRepo,
		memStreamer,
		stackuc.WithKubeconfigProvider(kubeconfigProvider),
		stackuc.WithTokenSourceRegistry(stackrepo.NewPostgresTokenSourceRegistry(pool, secretRouter), tokenSourceEnvironment(cfg.Server.Mode)),
		stackuc.WithSecretRouter(secretRouter),
		stackuc.WithExecutorFactory(func(kubeconfig []byte) stackport.StepExecutor {
			installer := stackhelm.NewHelmInstaller(kubeconfig)
			orch := stackhelm.NewOrchestrator(installer, kubeconfig, "", stackhelm.WithHelmStepMetadataRepository(pgHelmStepMetadataRepo))
			// SSO 프로비저너 주입 — stack 모듈은 포트만 알고 구현은 auth 모듈이 제공한다.
			if ssoFactory != nil {
				orch.SetSSOProvisionerFactory(ssoFactory)
			}
			return orch
		}),
	)
	createStackUC := stackuc.NewCreateStack(pgStackRepo, pgTemplateRepo, stackuc.WithManageHistory(manageHistoryUC))
	listStacksUC := stackuc.NewListStacks(pgStackRepo)
	deleteStackUC := stackuc.NewDeleteStack(
		pgStackRepo,
		kubeconfigProvider,
		func(kubeconfig []byte) stackport.HelmInstaller {
			return stackhelm.NewHelmInstaller(kubeconfig)
		},
	)
	addToolsUC := stackuc.NewAddToolsUseCase(pgStackRepo)
	importConfigUC := stackuc.NewImportConfig(createStackUC, addToolsUC, installStackUC)
	getTemplateUC := stackuc.NewGetTemplate(pgTemplateRepo)
	listTemplatesUC := stackuc.NewListTemplates(pgTemplateRepo)
	exportConfigUC := stackuc.NewExportConfig(pgStackRepo)
	calculateResourcesUC := stackuc.NewCalculateResources()
	pgResourceDefaultRepo := stackrepo.NewPostgresResourceDefaultRepository(pool)
	listResourceDefaultsUC := stackuc.NewListResourceDefaults(pgResourceDefaultRepo)
	upsertResourceDefaultUC := stackuc.NewUpsertResourceDefault(pgResourceDefaultRepo)

	deployHandler := stackhandler.NewDeployHandler(installStackUC, pgStackRepo, memStreamer, auditLogger).
		WithOptions(stackhandler.WithKubeconfigProvider(kubeconfigProvider), stackhandler.WithManageHistory(manageHistoryUC))
	stackHandler := stackhandler.NewStackHandler(createStackUC, listStacksUC, deleteStackUC, addToolsUC, pgStackRepo, auditLogger, stackhandler.WithStackManageHistory(manageHistoryUC), stackhandler.WithPool(pool))
	templateHandler := stackhandler.NewTemplateHandler(getTemplateUC, listTemplatesUC, pgTemplateRepo)
	exportHandler := stackhandler.NewExportHandler(exportConfigUC, importConfigUC)
	resourceHandler := stackhandler.NewResourceHandler(calculateResourcesUC, listResourceDefaultsUC, upsertResourceDefaultUC)

	pgCompatRepo := stackrepo.NewPostgresCompatibilityRepository(pool)
	validateCompatUC := stackuc.NewValidateCompatibility(pgCompatRepo)
	compatHandler := stackhandler.NewCompatibilityHandler(pgCompatRepo, validateCompatUC)

	historyHandler := stackhandler.NewHistoryHandler(pgHistoryRepo, pgStackRepo, manageHistoryUC)
	monitoringHandler := stackhandler.NewStackMonitoringHandler(pgStackRepo, kubeconfigProvider)

	// CI/CD: postgres repos
	pgCICDTemplateRepo := cicdrepo.NewPostgresCICDTemplateRepository(pool)
	pgPipelineRepo := cicdrepo.NewPostgresPipelineRepository(pool)
	pgDeploymentRepo := cicdrepo.NewPostgresDeploymentRepository(pool)
	memGoldenPathRepo := cicdrepo.NewMemoryCICDGoldenPathRepository()
	manifestApplier := cicdkube.NewManifestApplier()
	createPipelineUC := cicduc.NewCreatePipeline(pgPipelineRepo, pgCICDTemplateRepo)
	listPipelinesUC := cicduc.NewListPipelines(pgPipelineRepo)
	deployPipelineUC := cicduc.NewDeployPipeline(pgPipelineRepo, pgDeploymentRepo, kubeconfigProvider, manifestApplier)
	cicdTemplateHandler := cicdhandler.NewCICDTemplateHandler(pgCICDTemplateRepo)
	cicdGoldenPathHandler := cicdhandler.NewCICDGoldenPathHandler(memGoldenPathRepo)
	pipelineHandler := cicdhandler.NewPipelineHandler(createPipelineUC, listPipelinesUC, deployPipelineUC, pgPipelineRepo, pgDeploymentRepo, kubeconfigProvider, manifestApplier.Tracker, pool)

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
	getAlertRuleUC := obsuc.NewGetAlertRule(pgAlertRuleRepo)
	listAlertRulesUC := obsuc.NewListAlertRules(pgAlertRuleRepo)
	updateAlertRuleUC := obsuc.NewUpdateAlertRule(pgAlertRuleRepo)
	deleteAlertRuleUC := obsuc.NewDeleteAlertRule(pgAlertRuleRepo)
	listAlertsUC := obsuc.NewListAlerts(pgAlertRepo)
	dashboardHandler := obshandler.NewDashboardHandler(
		getDashboardUC,
		obshandler.WithStackRepo(pgStackRepo),
		obshandler.WithPool(pool),
		obshandler.WithKubeconfigProvider(kubeconfigProvider),
	)
	alertHandler := obshandler.NewAlertHandler(createAlertRuleUC, getAlertRuleUC, listAlertRulesUC, updateAlertRuleUC, deleteAlertRuleUC, listAlertsUC)

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
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
		MaxAge:           7200,
	}))
	e.Use(middleware.RateLimiter(middleware.RateLimitConfig{
		Authenticated:   300,
		Unauthenticated: 30,
	}))

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
	tokenSourceHandler := adminhandler.NewTokenSourceHandler(tokenSourceUC)

	orgHandler.RegisterRoutes(admin)
	clusterHandler.RegisterRoutes(admin)
	memberHandler.RegisterRoutes(admin)
	resourceProfileHandler.RegisterRoutes(admin)
	knownIssuesHandler.RegisterRoutes(admin)
	auditHandler.RegisterRoutes(admin)
	notificationHandler.RegisterRoutes(admin)
	tokenSourceHandler.RegisterRoutes(admin)
	deployHandler.RegisterRoutes(v1, e)
	stackHandler.RegisterRoutes(stacks)
	templateHandler.RegisterRoutes(stacks)
	exportHandler.RegisterRoutes(v1)
	compatHandler.RegisterRoutes(stacks)
	historyHandler.RegisterRoutes(stacks)
	monitoringHandler.RegisterRoutes(stacks)
	resourceHandler.RegisterRoutes(stacks)
	cicdTemplateHandler.RegisterRoutes(cicd)
	cicdGoldenPathHandler.RegisterRoutes(cicd)
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

	// 토큰 회전 스케줄러 기동.
	// 그동안 이 스케줄러는 정의만 되어 있고 어디서도 호출되지 않아 실제로는
	// 회전이 한 번도 일어나지 않았다.
	rotationCtx, stopRotation := context.WithCancel(context.Background())
	defer stopRotation()
	go adminscheduler.NewTokenRotationScheduler(
		pool,
		secretRouter,
		tokenRotationInterval(),
		0,
		slog.Default(),
		rotation.NewRouterReissuer(),
	).Start(rotationCtx)

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

// tokenRotationInterval 은 회전 스케줄러의 점검 주기다.
// TOKEN_ROTATION_INTERVAL 로 재정의할 수 있으며 기본값은 5분이다.
// envOrDefault 는 환경변수 값이 비어 있으면 기본값을 돌려준다.
func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func tokenRotationInterval() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("TOKEN_ROTATION_INTERVAL")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return 5 * time.Minute
}

func tokenSourceEnvironment(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "development", "dev":
		return "dev"
	case "staging":
		return "staging"
	case "production", "prod":
		return "prod"
	case "local":
		return "local"
	default:
		return "dev"
	}
}
