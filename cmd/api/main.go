package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
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
	cicddocker "github.com/cloud-nullus/draft/internal/cicd/adapter/docker"
	cicdgitea "github.com/cloud-nullus/draft/internal/cicd/adapter/gitea"
	cicdgithub "github.com/cloud-nullus/draft/internal/cicd/adapter/github"
	cicdgitlab "github.com/cloud-nullus/draft/internal/cicd/adapter/gitlab"
	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	cicdjenkins "github.com/cloud-nullus/draft/internal/cicd/adapter/jenkins"
	cicdkube "github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	cicdprovisioning "github.com/cloud-nullus/draft/internal/cicd/adapter/provisioning"
	cicdrepo "github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	cicdport "github.com/cloud-nullus/draft/internal/cicd/port"
	cicduc "github.com/cloud-nullus/draft/internal/cicd/usecase"
	obshandler "github.com/cloud-nullus/draft/internal/observability/adapter/handler"
	obsprom "github.com/cloud-nullus/draft/internal/observability/adapter/prometheus"
	obsrepo "github.com/cloud-nullus/draft/internal/observability/adapter/repository"
	obstoolhealth "github.com/cloud-nullus/draft/internal/observability/adapter/toolhealth"
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

	// 인증이 실제로 동작하지 않는 조합이면 여기서 끊는다. 그대로 기동하면
	// "전부 401" 이나 "사실은 무인증" 을 운영 중에 발견하게 된다.
	if err := cfg.ValidateAuth(); err != nil {
		slog.Error("invalid auth configuration", "error", err)
		os.Exit(1)
	}
	if cfg.TrustsClientSuppliedIdentity() {
		slog.Warn("AUTH IS NOT ENFORCED: this mode trusts client-supplied X-User-* headers, "+
			"so any caller can claim any role. Use auth.mode=oidc for a real deployment.",
			"auth_mode", cfg.Auth.Mode, "server_mode", cfg.Server.Mode)
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

	// 설치 단계의 리소스 값(관리자 기본값 + 설치 마법사 계획값)을 읽는 곳이다.
	// 오케스트레이터보다 먼저 만들어야 아래 WithResourceDefaultRepository 로 넘길 수 있다.
	pgResourceDefaultRepo := stackrepo.NewPostgresResourceDefaultRepository(pool)

	installStackUC := stackuc.NewInstallStack(
		pgStackRepo,
		memStreamer,
		stackuc.WithKubeconfigProvider(kubeconfigProvider),
		stackuc.WithTokenSourceRegistry(stackrepo.NewPostgresTokenSourceRegistry(pool, secretRouter), tokenSourceEnvironment(cfg.Server.Mode)),
		stackuc.WithSecretRouter(secretRouter),
		stackuc.WithExecutorFactory(func(kubeconfig []byte) stackport.StepExecutor {
			installer := stackhelm.NewHelmInstaller(kubeconfig)
			orch := stackhelm.NewOrchestrator(
				installer,
				kubeconfig,
				"",
				stackhelm.WithHelmStepMetadataRepository(pgHelmStepMetadataRepo),
				// 이게 빠지면 loadResourceDefault 가 repo nil 을 보고 바로 빠져나가
				// 모든 차트가 resources 없이 설치된다 — 파드의 requests/limits 가
				// 통째로 비고, 스케줄러가 자원을 예약하지 못한다.
				stackhelm.WithResourceDefaultRepository(pgResourceDefaultRepo),
				// 레지스트리 프로젝트 이름은 CI/CD 모듈의 그룹 경로와 같아야 한다.
				// 두 모듈은 서로 import 할 수 없으므로 조립 지점인 여기서 같은
				// 값을 넘겨 계약을 맞춘다 — 다르면 프로젝트는 만들어지는데 CI 가
				// push 하는 주소는 다른 프로젝트를 가리켜 "project not found" 로 막힌다.
				stackhelm.WithImageProjectName(cicdGroupPath()),
			)
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
	listResourceDefaultsUC := stackuc.NewListResourceDefaults(pgResourceDefaultRepo)
	upsertResourceDefaultUC := stackuc.NewUpsertResourceDefault(pgResourceDefaultRepo)

	deployHandler := stackhandler.NewDeployHandler(installStackUC, pgStackRepo, memStreamer, auditLogger).
		WithOptions(stackhandler.WithKubeconfigProvider(kubeconfigProvider), stackhandler.WithManageHistory(manageHistoryUC))
	stackHandler := stackhandler.NewStackHandler(createStackUC, listStacksUC, deleteStackUC, addToolsUC, pgStackRepo, auditLogger, stackhandler.WithStackManageHistory(manageHistoryUC), stackhandler.WithPool(pool),
		// /workloads 가 클러스터의 실제 파드를 읽는 데 쓴다. 없으면 빈 목록을 준다.
		stackhandler.WithWorkloadKubeconfigProvider(kubeconfigProvider))
	templateHandler := stackhandler.NewTemplateHandler(getTemplateUC, listTemplatesUC, pgTemplateRepo)
	exportHandler := stackhandler.NewExportHandler(exportConfigUC, importConfigUC)
	resourceHandler := stackhandler.NewResourceHandler(calculateResourcesUC, listResourceDefaultsUC, upsertResourceDefaultUC)

	pgCompatRepo := stackrepo.NewPostgresCompatibilityRepository(pool)
	validateCompatUC := stackuc.NewValidateCompatibility(pgCompatRepo)
	compatHandler := stackhandler.NewCompatibilityHandler(pgCompatRepo, validateCompatUC)
	// 재배포 기록 조회. 프론트의 재배포 기록 화면이 이 경로를 호출한다.
	retryHistoryHandler := stackhandler.NewRetryHistoryHandler(auditLogger)

	historyHandler := stackhandler.NewHistoryHandler(pgHistoryRepo, pgStackRepo, manageHistoryUC)
	monitoringHandler := stackhandler.NewStackMonitoringHandler(pgStackRepo, kubeconfigProvider)

	// 배포된 스택의 OSS 설정을 values.yaml 수준에서 고쳐 다시 적용하는 경로.
	// 릴리스마다 대상 클러스터가 다르므로 kubeconfig 를 받아 그때그때 조립한다.
	manageReleaseValuesUC := stackuc.NewManageReleaseValues(
		pgStackRepo,
		kubeconfigProvider,
		func(kubeconfig []byte) stackport.HelmReleaseManager {
			return stackhelm.NewHelmInstaller(kubeconfig)
		},
		stackuc.WithReleaseValuesHistory(manageHistoryUC),
	)
	releaseValuesHandler := stackhandler.NewReleaseValuesHandler(manageReleaseValuesUC, auditLogger)

	// CI/CD: postgres repos
	pgCICDTemplateRepo := cicdrepo.NewPostgresCICDTemplateRepository(pool)
	pgPipelineRepo := cicdrepo.NewPostgresPipelineRepository(pool)
	pgDeploymentRepo := cicdrepo.NewPostgresDeploymentRepository(pool)
	memGoldenPathRepo := cicdrepo.NewMemoryCICDGoldenPathRepository()
	manifestApplier := cicdkube.NewManifestApplier()

	// CI/CD 저장소 프로비저닝 배선.
	// GitLab 주소·토큰·레지스트리 종류가 스택마다 달라 기동 시점에 클라이언트를
	// 하나로 만들 수 없다. 팩토리가 요청 시점에 스택을 읽어 조립한다.
	cicdStackReader := cicdrepo.NewPostgresStackReader(pool)
	cicdTokenIssuer := cicdgitlab.NewTokenIssuer(
		kubeconfigProvider,
		cicdKubectlRunner,
		secretRouter,
	)
	// GitHub 은 SaaS 라 토큰을 발급할 수 없다 — 사용자가 등록한 PAT 를 읽기만
	// 하고, organization·API 주소는 token_sources 의 metadata 에서 가져온다.
	cicdGitHubTokens := cicdgithub.NewTokenIssuer(secretRouter)
	cicdGitHubConnections := cicdrepo.NewPostgresSCMConnectionReader(pool)

	// Gitea 는 스택 안에 설치되므로 GitLab 과 같은 방식으로 토큰을 발급한다 —
	// 외부에 노출하지 않고도 동작해야 하므로 API 가 아니라 파드 안의 CLI 를 쓴다.
	cicdGiteaTokens := cicdgitea.NewTokenIssuer(
		kubeconfigProvider,
		cicdKubectlRunner,
		secretRouter,
	)
	// Jenkins 자격증명은 발급하지 않고 읽기만 한다. 관리자 비밀번호는 스택
	// 설치의 provisioning_secrets 가 이미 만들어 OpenBao 에 넣었고 같은 값을
	// ESO 가 컨트롤러에 동기화한다 — 여기서 새로 발급하면 컨트롤러가 자기
	// 비밀번호를 모르게 된다.
	cicdJenkinsCreds := cicdjenkins.NewCredentialResolver(secretRouter)

	cicdBundleFactory := cicdprovisioning.NewBundleFactory(
		cicdStackReader,
		cicdTokenIssuer,
		cicdprovisioning.Options{
			Env:       tokenSourceEnvironment(cfg.Server.Mode),
			GroupPath: cicdGroupPath(),
			// 기본은 클러스터 내부 서비스 DNS 다. API 서버를 클러스터 밖에서
			// 돌리거나 외부 GitLab 을 붙일 때만 지정한다.
			GitLabBaseURLOverride:  strings.TrimSpace(os.Getenv("NULLUS_GITLAB_URL")),
			GiteaBaseURLOverride:   strings.TrimSpace(os.Getenv("NULLUS_GITEA_URL")),
			JenkinsBaseURLOverride: strings.TrimSpace(os.Getenv("NULLUS_JENKINS_URL")),
		},
	).WithGitHub(cicdGitHubTokens, cicdGitHubConnections).
		WithGitea(cicdGiteaTokens, secretRouter).
		WithJenkins(cicdJenkinsCreds)
	runSyncUC := cicduc.NewSyncPipelineRuns(nil, pgDeploymentRepo).
		WithBundleFactory(cicdBundleFactory, pgPipelineRepo)
	provisionRepoUC := cicduc.NewProvisionPipelineRepository(
		cicdBundleFactory, manifestApplier, kubeconfigProvider)

	createPipelineUC := cicduc.NewCreatePipeline(pgPipelineRepo, pgCICDTemplateRepo, cicdStackReader).
		WithRepositoryProvisioner(provisionRepoUC)
	listPipelinesUC := cicduc.NewListPipelines(pgPipelineRepo)
	// 이미지 준비기와 클러스터 타깃 제공자를 배선한다.
	//
	// 이 둘이 없으면 DeployPipeline 이 매니페스트만 적용한다. 그런데 BuildStepPlan 은
	// DockerfilePath 가 있으면 Git Clone / Docker Build / Image Load 3단계를 계획에
	// 넣으므로, 배선이 빠진 상태에서는 계획에만 있고 절대 실행되지 않는 단계가 생겼다 —
	// 배포가 success 인데 6단계 중 3개가 pending 으로 남아 있었다.
	//
	// Builder 는 host 의 git·docker 를, kind 로드 경로는 kind CLI 를 쓴다. 세 실행 파일이
	// 없는 환경에서는 빌드 단계가 실패하는데, 그건 조용히 건너뛰는 것보다 낫다 —
	// 실패는 로그와 단계 상태에 남지만, 건너뛰면 사용자는 이미지가 빌드된 줄 안다.
	deployPipelineUC := cicduc.NewDeployPipeline(
		pgPipelineRepo, pgDeploymentRepo, kubeconfigProvider, manifestApplier,
		cicduc.WithImagePreparer(cicddocker.NewBuilder(manifestApplier.Tracker)),
		cicduc.WithClusterTargetProvider(
			cicdrepo.NewPostgresClusterTargetProvider(pool, encryptionKey)),
		// 배포되는 앱에 스택 수집기 주소(OTLP)를 넣어 주기 위해 필요하다.
		// 없으면 배포는 되고 추적 환경변수만 빠진다.
		cicduc.WithStackReader(cicdStackReader),
	)
	cicdTemplateHandler := cicdhandler.NewCICDTemplateHandler(pgCICDTemplateRepo)
	cicdGoldenPathHandler := cicdhandler.NewCICDGoldenPathHandler(memGoldenPathRepo)
	deletePipelineUC := cicduc.NewDeletePipeline(
		pgPipelineRepo, cicdBundleFactory, cicdkube.NewArgoApplicationDeleter(), kubeconfigProvider).
		// 매니페스트를 직접 적용한 파이프라인은 Argo CD Application 이 없어
		// 워크로드를 지워 줄 주체가 없다.
		WithWorkloadDeleter(cicdkube.NewWorkloadDeleter())
	pipelineHandler := cicdhandler.NewPipelineHandler(createPipelineUC, listPipelinesUC, deployPipelineUC, pgPipelineRepo, pgDeploymentRepo, kubeconfigProvider, manifestApplier.Tracker, pool).
		WithDeletePipeline(deletePipelineUC).
		// GitOps 경로의 실행 기록은 CI 서버에만 있다. 들이지 않으면 빌드가
		// 성공해도 화면의 실행 통계가 0 으로 남는다.
		WithRunSync(runSyncUC).
		// 직접 배포가 실제로 클러스터에 적용하도록 한다. 없으면 배포가
		// 실패한다 — 적용 없이 성공으로 기록하는 것보다 낫다.
		WithManifestApplier(manifestApplier).
		// 직접 배포(POST /deploy-app)도 파이프라인 배포와 같은 수집기 주소를
		// 넣어야 한다 — 한쪽만 배선하면 경로에 따라 추적이 갈린다.
		WithStackReader(cicdStackReader)

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
	// 도구 건강도는 설치된 스택의 실제 파드에서 뽑는다. Prometheus 유무와 무관하다.
	toolHealthReader := obstoolhealth.New(pgStackRepo, kubeconfigProvider)
	getDashboardUC := obsuc.NewGetDashboard(dashboardRepo, obsuc.WithToolHealth(toolHealthReader))
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
	rateLimits := middleware.RateLimitConfigForMode(cfg.Server.Mode)
	// 1단: 모든 경로를 덮는 IP 기준 폭주 상한. 전역 미들웨어는 인증보다 먼저 돌아
	// 호출자를 알 수 없으므로, 여기서는 신원을 따지지 않고 총량만 막는다.
	e.Use(middleware.IPCeilingRateLimiter(rateLimits))

	// 2단: 인증을 통과한 그룹에 붙어 사용자 단위로 센다. 인스턴스를 하나만 만들어
	// 공유해야 한 사용자가 여러 그룹을 오가도 사용량이 하나로 합산된다.
	userRateLimit := middleware.RateLimiter(rateLimits)

	// API v1 group
	v1 := e.Group("/api/v1")

	var admin, stacks, cicd, observability *echo.Group
	// wsAuth 는 /ws/* 전용 체인이다. 브라우저 WebSocket 은 Authorization 헤더를 못
	// 붙이므로 서브프로토콜로 온 토큰을 헤더로 옮긴 뒤 평소 인증을 태운다.
	var wsAuth []echo.MiddlewareFunc
	if cfg.Server.Mode == "development" {
		slog.Info("development mode: auth middleware disabled")
		admin = v1.Group("/admin", userRateLimit)
		stacks = v1.Group("/stacks", userRateLimit)
		cicd = v1.Group("/cicd", userRateLimit)
		observability = v1.Group("/observability", userRateLimit)
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
		// userRateLimit 은 authMW 바로 뒤에 둔다. 권한 검사(RequireRole)보다 앞이어야
		// 403 으로 튕기는 요청도 사용량에 잡힌다.
		admin = v1.Group("/admin", authMW, userRateLimit, authmw.RequireRole("admin"))
		stacks = v1.Group("/stacks", authMW, userRateLimit, authmw.RequireRole("admin", "devops"))
		cicd = v1.Group("/cicd", authMW, userRateLimit, authmw.RequireRole("admin", "devops", "developer"))
		observability = v1.Group("/observability", authMW, userRateLimit)

		// OIDC 모드에서만 WebSocket 을 보호한다. session 모드의 인증은 클라이언트가
		// 보낸 X-User-* 헤더를 그대로 믿는 방식인데, 브라우저 WebSocket 은 그 헤더를
		// 붙일 수 없어 무조건 401 이 된다 — 검증도 아닌 것 때문에 기능만 죽는 셈이다.
		if cfg.Auth.Mode == "oidc" {
			wsAuth = []echo.MiddlewareFunc{middleware.WebSocketBearerSubprotocol(), oidcMW}
		} else {
			slog.Warn("websocket auth disabled: session mode cannot carry credentials over a browser WebSocket",
				"auth_mode", cfg.Auth.Mode)
		}
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
	deployHandler.RegisterRoutes(stacks, e, wsAuth...)
	stackHandler.RegisterRoutes(stacks)
	templateHandler.RegisterRoutes(stacks)
	exportHandler.RegisterRoutes(v1)
	compatHandler.RegisterRoutes(stacks)
	historyHandler.RegisterRoutes(stacks)
	monitoringHandler.RegisterRoutes(stacks)
	releaseValuesHandler.RegisterRoutes(stacks)
	resourceHandler.RegisterRoutes(stacks)
	retryHistoryHandler.RegisterRoutes(stacks)
	cicdTemplateHandler.RegisterRoutes(cicd)
	cicdGoldenPathHandler.RegisterRoutes(cicd)
	pipelineHandler.RegisterRoutes(cicd)
	// 스택별 파이프라인 조회는 /stacks 그룹 아래에 붙는다. 핸들러는 처음부터 있었는데
	// 이 호출이 빠져 있어 GET /api/v1/stacks/:stackId/pipelines 가 404 였다.
	pipelineHandler.RegisterStackRoutes(stacks)
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
	// 재발급기 등록. 여기 없는 provider 는 회전 대상에서 조용히 빠진다.
	//
	// Gitea 는 클러스터 안에 있어 HTTP 로 닿지 못할 수 있으므로 발급과 같은
	// 경로(파드 안의 gitea CLI)를 재사용한다. 재발급 로직을 복제하면 스코프나
	// 토큰 이름이 갈라져 회전 후 인증이 조용히 실패한다.
	reissuerRouter := rotation.NewRouterReissuer()
	reissuerRouter.Register("gitea", rotation.NewGiteaReissuer(
		func(ctx context.Context, spec rotation.GiteaReissueSpec) (string, error) {
			return cicdGiteaTokens.EnsureToken(ctx, cicdport.SCMTokenSpec{
				StackID:   spec.StackID,
				ClusterID: spec.ClusterID,
				Namespace: spec.Namespace,
				OrgID:     spec.OrgID,
				Env:       spec.Env,
				// 회전은 기존 토큰이 살아 있어도 새로 발급해야 한다.
				Force: true,
			})
		}))

	go adminscheduler.NewTokenRotationScheduler(
		pool,
		secretRouter,
		tokenRotationInterval(),
		0,
		slog.Default(),
		reissuerRouter,
	).WithRestarter(
		// 회전 후 반영: 소비자가 기동 시점에만 설정을 읽는 경우 rolling restart 한다.
		adminrepo.NewClusterWorkloadRestarter(kubeconfigProvider),
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

// cicdGroupPath 는 CI/CD 프로젝트가 만들어질 GitLab 그룹 경로다.
// NULLUS_SCM_GROUP 으로 재정의할 수 있다.
func cicdGroupPath() string {
	return envOrDefault("NULLUS_SCM_GROUP", "nullus")
}

// cicdKubectlRunner 는 CI/CD 모듈이 쓰는 kubectl 실행기다.
//
// 다른 모듈의 동일 구현을 재사용하지 않는다 — 모듈 간 직접 import 를 피하고
// 각 컨텍스트가 자기 계약을 소유하도록 조립 지점에서 주입한다.
func cicdKubectlRunner(ctx context.Context, kubeconfig []byte, args ...string) ([]byte, error) {
	if len(kubeconfig) == 0 {
		return nil, fmt.Errorf("kubeconfig is empty")
	}
	tmp, err := os.CreateTemp("", "nullus-cicd-kubeconfig-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("create kubeconfig temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(kubeconfig); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write kubeconfig temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close kubeconfig temp file: %w", err)
	}

	full := append([]string{"--kubeconfig", tmp.Name()}, args...)
	out, err := exec.CommandContext(ctx, "kubectl", full...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("kubectl %s: %w (%s)",
			strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
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
