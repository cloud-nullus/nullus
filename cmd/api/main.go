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
	"github.com/cloud-nullus/draft/internal/assistant"
	assistanthandler "github.com/cloud-nullus/draft/internal/assistant/adapter/handler"
	authadapter "github.com/cloud-nullus/draft/internal/auth/adapter"
	authhandler "github.com/cloud-nullus/draft/internal/auth/adapter/handler"
	keycloakadapter "github.com/cloud-nullus/draft/internal/auth/adapter/keycloak"
	authmw "github.com/cloud-nullus/draft/internal/auth/adapter/middleware"
	authrepo "github.com/cloud-nullus/draft/internal/auth/adapter/repository"
	authtoken "github.com/cloud-nullus/draft/internal/auth/adapter/token"
	authusecase "github.com/cloud-nullus/draft/internal/auth/usecase"
	"github.com/cloud-nullus/draft/internal/backup"
	backupbridge "github.com/cloud-nullus/draft/internal/backup/adapter/bridge"
	backuphandler "github.com/cloud-nullus/draft/internal/backup/adapter/handler"
	backupdomain "github.com/cloud-nullus/draft/internal/backup/domain"
	backupscheduler "github.com/cloud-nullus/draft/internal/backup/scheduler"
	backupusecase "github.com/cloud-nullus/draft/internal/backup/usecase"
	cicddocker "github.com/cloud-nullus/draft/internal/cicd/adapter/docker"
	cicdgitea "github.com/cloud-nullus/draft/internal/cicd/adapter/gitea"
	cicdgithub "github.com/cloud-nullus/draft/internal/cicd/adapter/github"
	cicdgitlab "github.com/cloud-nullus/draft/internal/cicd/adapter/gitlab"
	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	cicdjenkins "github.com/cloud-nullus/draft/internal/cicd/adapter/jenkins"
	cicdkube "github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	cicdprovisioning "github.com/cloud-nullus/draft/internal/cicd/adapter/provisioning"
	cicdrepo "github.com/cloud-nullus/draft/internal/cicd/adapter/repository"
	cicdrunner "github.com/cloud-nullus/draft/internal/cicd/adapter/runner"
	cicdsourcearchive "github.com/cloud-nullus/draft/internal/cicd/adapter/sourcearchive"
	cicdport "github.com/cloud-nullus/draft/internal/cicd/port"
	cicduc "github.com/cloud-nullus/draft/internal/cicd/usecase"
	notificationmod "github.com/cloud-nullus/draft/internal/notification"
	notificationhandler "github.com/cloud-nullus/draft/internal/notification/adapter/handler"
	obsalertmanager "github.com/cloud-nullus/draft/internal/observability/adapter/alertmanager"
	obsbridge "github.com/cloud-nullus/draft/internal/observability/adapter/bridge"
	obshandler "github.com/cloud-nullus/draft/internal/observability/adapter/handler"
	obsprom "github.com/cloud-nullus/draft/internal/observability/adapter/prometheus"
	obsrepo "github.com/cloud-nullus/draft/internal/observability/adapter/repository"
	obstoolhealth "github.com/cloud-nullus/draft/internal/observability/adapter/toolhealth"
	obsport "github.com/cloud-nullus/draft/internal/observability/port"
	obsscheduler "github.com/cloud-nullus/draft/internal/observability/scheduler"
	obsuc "github.com/cloud-nullus/draft/internal/observability/usecase"
	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/shared/config"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
	"github.com/cloud-nullus/draft/internal/shared/secrets"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackhelm "github.com/cloud-nullus/draft/internal/stack/adapter/helm"
	stackimagescan "github.com/cloud-nullus/draft/internal/stack/adapter/imagescan"
	logadapter "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	stackport "github.com/cloud-nullus/draft/internal/stack/port"
	stackuc "github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/cloud-nullus/draft/pkg/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// version 은 빌드 시 ldflags 로 주입된다 (-X main.version=$TAG).
// 주입되지 않았을 때의 기본값은 "dev" 이다.
var version = "dev"

type dbPinger interface {
	Ping(ctx context.Context) error
}

func healthHandler(pool dbPinger) echo.HandlerFunc {
	return func(c echo.Context) error {
		dbStatus := "connected"
		if pool != nil {
			if err := pool.Ping(c.Request().Context()); err != nil {
				slog.Warn("health check db ping failed", "error", err)
				dbStatus = "unavailable"
			}
		}
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "healthy",
			"db":      dbStatus,
			"version": version,
		})
	}
}

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
	if err := cfg.ValidateKeycloakAdmin(); err != nil {
		slog.Error("invalid keycloak configuration", "error", err)
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
	// 설치 로그는 DB 에도 남긴다. 메모리에만 두면 파드가 재시작되는 순간
	// 사라지는데, 설치는 20~30분짜리라 그 사이 재시작이 겹치면 무엇이 왜
	// 멈췄는지 사후에 알 방법이 없다.
	memStreamer := logadapter.NewPersistentStreamer(
		logadapter.NewMemoryStreamer(),
		stackrepo.NewPostgresDeployLogStore(pool),
	)
	kubeconfigProvider := stackrepo.NewPostgresKubeconfigProvider(pool, []byte(os.Getenv("ENCRYPTION_KEY")))

	// 플랫폼 Keycloak 에 OSS OIDC 클라이언트를 등록하는 팩토리.
	// keycloak.admin_url 이 비면 SSO 프로비저닝은 건너뛴다 (BYO / 미사용 모드).
	//
	// 이 값이 어디서도 주입되지 않아 팩토리가 항상 nil 이던 시절이 있었다. 그때는
	// provisioning_sso 가 로그 한 줄만 남기고 성공으로 마킹돼, 설치는 초록불인데
	// OSS 는 전부 로컬 admin 계정으로 뜨는 조용한 누락이 됐다. 그래서 건너뛸 때도
	// 기동 로그에 남긴다.
	var ssoFactory stackport.SSOProvisionerFactory
	if kc, ok := cfg.KeycloakAdmin(); ok {
		ssoFactory = keycloakadapter.NewStackSSOFactory(keycloakadapter.NewKeycloakClient(
			kc.AdminURL, kc.Realm, kc.AdminUser, kc.AdminPassword,
		))
		slog.Info("OSS SSO provisioning enabled", "keycloak_url", kc.AdminURL, "realm", kc.Realm)
	} else {
		slog.Info("OSS SSO provisioning disabled: keycloak.admin_url is empty " +
			"(installed tools will use their own local accounts)")
	}

	// 스택별 OpenBao 해석기. OpenBao 는 스택마다 배포되므로 주소가 전역 하나일 수 없다.
	// 대상 클러스터의 kubeconfig 로 Kubernetes Auth 기반 Store 를 만든다.
	secretRouter.WithResolver(adminrepo.NewStackSecretResolver(pool, kubeconfigProvider))

	// 설치 단계의 리소스 값(관리자 기본값 + 설치 마법사 계획값)을 읽는 곳이다.
	// 오케스트레이터보다 먼저 만들어야 아래 WithResourceDefaultRepository 로 넘길 수 있다.
	pgResourceDefaultRepo := stackrepo.NewPostgresResourceDefaultRepository(pool)

	// 설치 이미지 취약점 스캔(보고용, 설치를 막지 않는다).
	//
	// 스택이 돌리는 OSS 이미지를 스택의 Trivy 서버로 스캔한다 — 설치 직후 한 번,
	// 이후 주기적으로. 에어갭 설치는 노드가 containerd 미러로 받은 이미지를 내부 레지스트리
	// (NULLUS_HELM_OCI_REGISTRY 의 호스트)에서 받아 스캔하고, DB 는 번들이 반입한 것을 쓴다.
	stackImageScanRepo := stackrepo.NewPostgresStackImageScanRepository(pool)
	stackImageScanUC := stackuc.NewScanStackImages(
		pgStackRepo,
		pgStackRepo,
		kubeconfigProvider,
		stackimagescan.NewScanner(stackimagescan.WithMirrorRegistry(
			shareddomain.AirgapRegistryHost(os.Getenv("NULLUS_HELM_OCI_REGISTRY")))),
		stackImageScanRepo,
		stackhelm.AirgapMode(),
	)

	stackImageScanHandler := stackhandler.NewImageScanHandler(stackImageScanUC)

	// 스택 설치기 팩토리. 설치와, 복구가 공용 컨트롤러(cert-manager·metrics-server)를 다시 세울 때
	// 같은 것을 쓴다 — 두 길이 같은 차트 메타데이터·자원 기본값·이미지 프로젝트로 같은 결과를 낸다.
	stackExecutorFactory := func(kubeconfig []byte) stackport.StepExecutor {
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
			// 설치되는 OSS 가 브라우저를 보낼 Keycloak 주소. 포털이 로그인한
			// 곳과 같아야 SSO 세션이 이어진다.
			stackhelm.WithToolOIDCIssuer(cfg.ToolOIDCIssuerURL()),
		)
		// SSO 프로비저너 주입 — stack 모듈은 포트만 알고 구현은 auth 모듈이 제공한다.
		if ssoFactory != nil {
			orch.SetSSOProvisionerFactory(ssoFactory)
		}
		return orch
	}

	installStackUC := stackuc.NewInstallStack(
		pgStackRepo,
		memStreamer,
		stackuc.WithKubeconfigProvider(kubeconfigProvider),
		stackuc.WithTokenSourceRegistry(stackrepo.NewPostgresTokenSourceRegistry(pool, secretRouter), tokenSourceEnvironment(cfg.Server.Mode)),
		stackuc.WithSecretRouter(secretRouter),
		stackuc.WithInstalledImageScan(stackImageScanUC),
		stackuc.WithExecutorFactory(stackExecutorFactory),
	)
	// 복구가 마지막 스택을 지운 클러스터로 되돌릴 때 회수된 컨트롤러를 설치기로 다시 세운다.
	ensureSharedComponentsUC := stackuc.NewEnsureSharedComponents(stackExecutorFactory)
	createStackUC := stackuc.NewCreateStack(
		pgStackRepo,
		pgTemplateRepo,
		stackuc.WithManageHistory(manageHistoryUC),
		// 설치 마법사를 거치지 않는 생성(API/CLI/에어갭)도 템플릿의 설치 규모
		// 프로파일대로 깔리게 한다. 마법사가 계획을 실어 보낸 경우에는 그 값이
		// 이기므로 UI 설치 동작은 그대로다.
		stackuc.WithResourcePlanning(pgResourceDefaultRepo),
		// 플랫폼이 사는 네임스페이스에는 스택을 세우지 못하게 한다.
		stackuc.WithPlatformNamespace(cfg.Platform.Namespace),
	)
	listStacksUC := stackuc.NewListStacks(pgStackRepo)
	deleteStackUC := stackuc.NewDeleteStack(
		pgStackRepo,
		kubeconfigProvider,
		func(kubeconfig []byte) stackport.HelmInstaller {
			return stackhelm.NewHelmInstaller(kubeconfig)
		},
	)
	// 삭제가 플랫폼 네임스페이스를 훑지 않게 자기 자리를 알려준다.
	deleteStackUC.SetPlatformNamespace(cfg.Platform.Namespace)
	// 클러스터의 마지막 스택을 지울 때 공용 릴리스(cert-manager·metrics-server)의 values 를 읽어
	// Nullus 가 설치한 것만 회수한다.
	deleteStackUC.SetReleaseManagerFactory(func(kubeconfig []byte) stackport.HelmReleaseManager {
		return stackhelm.NewHelmInstaller(kubeconfig)
	})
	// 설치가 IdP 에 등록한 OIDC 클라이언트를 삭제 때 함께 지운다. 설치와 같은
	// 팩토리를 줘야 같은 client ID 를 계산한다.
	if ssoFactory != nil {
		deleteStackUC.SetSSOProvisionerFactory(ssoFactory)
	}
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
	// 이미지 스캔 게이트 판정. 대시보드(#65)가 cicd 의 공개 경로로 읽는다.
	pgImageScanRepo := cicdrepo.NewPostgresImageScanResultRepository(pool)
	pgSASTResultRepo := cicdrepo.NewPostgresSASTResultRepository(pool)
	// 의존성 스캔(SCA) · 시크릿 탐지 결과(Security Dashboard 설계 §8 G1 · G2). 이 둘이 없으면 대시보드의
	// 네 열 중 두 열이 비고, 시크릿 경고를 통과와 가르지 못한다.
	pgDependencyScanRepo := cicdrepo.NewPostgresDependencyScanResultRepository(pool)
	pgSecretScanRepo := cicdrepo.NewPostgresSecretScanResultRepository(pool)
	pgGoldenPathRepo := cicdrepo.NewPostgresCICDGoldenPathRepository(pool)
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
			// 돌리거나 외부 GitLab 을 붙일 때만 지정한다. 주소 하나 또는
			// "네임스페이스=주소,..." 목록(스택마다 포트포워드가 다를 때 —
			// 로컬 런북이 이렇게 넣는다).
			GitLabBaseURLOverride:  strings.TrimSpace(os.Getenv("NULLUS_GITLAB_URL")),
			GiteaBaseURLOverride:   strings.TrimSpace(os.Getenv("NULLUS_GITEA_URL")),
			JenkinsBaseURLOverride: strings.TrimSpace(os.Getenv("NULLUS_JENKINS_URL")),
			// 에어갭 설치면 CI 잡의 trivy client 가 반입한 Java DB 미러를 쓴다. 스택
			// Trivy 서버의 DB 미러와 같은 레지스트리다(shareddomain 이 경로 규칙을 소유).
			TrivyJavaDBRepository: shareddomain.TrivyJavaDBRepository(os.Getenv("NULLUS_HELM_OCI_REGISTRY")),
		},
	).WithGitHub(cicdGitHubTokens, cicdGitHubConnections).
		WithGitea(cicdGiteaTokens, secretRouter).
		WithJenkins(cicdJenkinsCreds).
		// 스택이 설치한 Harbor·Nexus 의 관리자 자격증명은 설치 과정이 이미
		// OpenBao 에 만들어 두었다. 배선하지 않으면 CI 레지스트리 변수가 비어
		// 스캐폴딩된 파이프라인의 docker login 이 죽는다.
		WithRegistrySecrets(secretRouter).
		// Gitea + Jenkins 스택은 CI 변수 저장소가 없어 스캔 정책을 스택 네임스페이스의
		// ConfigMap 으로 싣는다. 배선하지 않으면 그 스택은 정책을 실을 경로가 없다.
		WithManifestApplier(manifestApplier, kubeconfigProvider).
		// 앱 HTTPRoute 는 스택 네임스페이스에 실제로 있는 게이트웨이에 붙인다. 스택은 설치 경로에 따라
		// 게이트웨이 이름 규칙이 달라, 이름을 짐작하면 라우트가 붙지 않아 앱 주소가 404 다.
		WithGatewayLocator(cicdkube.NewGatewayLocator())
	// 스택별 이미지 스캔 정책. 판정은 CI 가 푸시받은 값으로 스스로 한다(설계 §5.4) —
	// CI 가 플랫폼에 되묻는 인바운드 경로와 기계 인증을 두지 않는다.
	pgScanPolicyRepo := cicdrepo.NewPostgresScanPolicyRepository(pool)
	scanPolicyUC := cicduc.NewScanPolicyService(pgScanPolicyRepo, pgPipelineRepo, cicdBundleFactory)
	scanPolicyHandler := cicdhandler.NewScanPolicyHandler(scanPolicyUC)
	runSyncUC := cicduc.NewSyncPipelineRuns(nil, pgDeploymentRepo).
		WithBundleFactory(cicdBundleFactory, pgPipelineRepo).
		// 실행 기록을 들이면서 스캔 단계의 게이트 판정도 남긴다. 배선하지 않으면
		// 차단된 배포가 어디에도 기록되지 않는다.
		WithImageScans(pgImageScanRepo).
		// 분석 단계가 남긴 리포트로 Quality Gate 판정·지표를 남긴다.
		WithSASTResults(pgSASTResultRepo).
		// 의존성 스캔 · 시크릿 탐지 리포트로 판정 · 건수 · 탐지 위치를 남긴다(비밀값은 남기지 않는다).
		WithDependencyScans(pgDependencyScanRepo).
		WithSecretScans(pgSecretScanRepo).
		// CI 는 스택 정책으로 막는다. 동기화도 같은 정책으로 읽어야 차단과 스캔 오류를 가른다.
		WithScanPolicies(pgScanPolicyRepo).
		// 주기 동기화가 돌 파이프라인 목록. 화면을 열 때만 들이면 아무도 보지 않는
		// 파이프라인의 스캔 결과가 쌓이지 않는다.
		WithSyncablePipelines(pgPipelineRepo)
	// 템플릿 원본을 가져오는 곳. 템플릿 API 도 저장할 때 같은 것으로 주소를 확인한다.
	cicdSourceArchive := cicdsourcearchive.NewGitHub(nil)
	provisionRepoUC := cicduc.NewProvisionPipelineRepository(
		cicdBundleFactory, manifestApplier, kubeconfigProvider).
		// 원본을 가져오는 템플릿(전자정부 표준프레임워크 샘플 등)이 새 저장소에 원본을 복사한다.
		// 원본 호스트는 인터넷에 있다 — 오프라인 설치에서는 그 템플릿만 쓸 수 없다.
		WithSourceArchive(cicdSourceArchive)

	createPipelineUC := cicduc.NewCreatePipeline(pgPipelineRepo, pgCICDTemplateRepo, cicdStackReader).
		WithRepositoryProvisioner(provisionRepoUC).
		// 새 스캔 파이프라인에 스택 정책을 싣는다. 빠지면 새 파이프라인만 기본 정책으로 판정한다.
		WithScanPolicyPublisher(scanPolicyUC)
	// 기존 파이프라인의 CI 정의 파일을 플랫폼 기본으로 다시 쓰는 경로. SAST 리포트 같은 새 단계가
	// 옛 파이프라인에 붙게 한다 — 전에는 파이프라인을 다시 만드는 것 말고 방법이 없었다.
	refreshDefinitionUC := cicduc.NewRefreshPipelineDefinition(pgPipelineRepo, provisionRepoUC).
		WithScanPolicyPublisher(scanPolicyUC)
	listPipelinesUC := cicduc.NewListPipelines(pgPipelineRepo)
	// 이미지 준비기와 클러스터 타깃 제공자를 배선한다.
	//
	// 이 둘이 없으면 DeployPipeline 이 매니페스트만 적용한다. 그런데 BuildStepPlan 은
	// DockerfilePath 가 있으면 Git Clone / Docker Build / Image Load 3단계를 계획에
	// 넣으므로, 배선이 빠진 상태에서는 계획에만 있고 절대 실행되지 않는 단계가 생겼다 —
	// 배포가 success 인데 6단계 중 3개가 pending 으로 남아 있었다.
	//
	// 두 실행 경로를 함께 배선한다.
	//
	// Delegate 는 스택에 묶인 파이프라인의 일반 경로다 — 스택의 CI 러너가
	// 빌드하고 Argo CD 가 반영한다. Builder 는 스택 컴포넌트를 쓸 수 없을 때의
	// 장애 대응 경로이며 host 의 git·docker·kind CLI 를 쓴다. 어느 쪽을 탈지는
	// 파이프라인이 정한다(domain.Pipeline.DelegatesBuildToRunner).
	deployPipelineUC := cicduc.NewDeployPipeline(
		pgPipelineRepo, pgDeploymentRepo, kubeconfigProvider, manifestApplier,
		cicduc.WithBuildDelegate(cicdrunner.NewDelegate(cicdBundleFactory, manifestApplier.Tracker)),
		cicduc.WithImagePreparer(cicddocker.NewBuilder(manifestApplier.Tracker)),
		cicduc.WithClusterTargetProvider(
			cicdrepo.NewPostgresClusterTargetProvider(pool, encryptionKey)),
		// 배포되는 앱에 스택 수집기 주소(OTLP)를 넣어 주기 위해 필요하다.
		// 없으면 배포는 되고 추적 환경변수만 빠진다.
		cicduc.WithStackReader(cicdStackReader),
	)
	cicdTemplateHandler := cicdhandler.NewCICDTemplateHandler(pgCICDTemplateRepo).
		// 원본을 가져올 수 없는 주소의 템플릿은 저장하지 않는다 — 파이프라인을 만들 때마다 실패한다.
		WithSourceArchive(cicdSourceArchive)
	cicdGoldenPathHandler := cicdhandler.NewCICDGoldenPathHandler(pgGoldenPathRepo)
	deletePipelineUC := cicduc.NewDeletePipeline(
		// CD 도구의 애플리케이션 삭제기는 번들이 공급한다 — 스택마다 다른 도구를
		// 쓸 수 있고, 그 판단은 스택을 아는 쪽이 해야 한다.
		pgPipelineRepo, cicdBundleFactory, kubeconfigProvider).
		// 매니페스트를 직접 적용한 파이프라인은 Argo CD Application 이 없어
		// 워크로드를 지워 줄 주체가 없다.
		WithWorkloadDeleter(cicdkube.NewWorkloadDeleter()).
		// 파이프라인을 만들 때 넣은 pull·저장소 자격증명 Secret 은 Argo CD 가 지워 주지 않는다.
		WithSecretDeleter(cicdkube.NewManagedSecretDeleter()).
		// Gitea · Jenkins 스택의 파이프라인 자격증명(ExternalSecret · ESO Secret · OpenBao 값)을 회수한다.
		WithCredentialObjectDeleter(cicdkube.NewExternalSecretDeleter()).
		// 같은 이름의 다른 파이프라인이 쓰는 자격증명은 남긴다 — 빠짐없이 찾는다.
		WithPipelineNameLookup(pgPipelineRepo)
	pipelineHandler := cicdhandler.NewPipelineHandler(createPipelineUC, listPipelinesUC, deployPipelineUC, pgPipelineRepo, pgDeploymentRepo, kubeconfigProvider, manifestApplier.Tracker, pool).
		// 앱 배포 화면이 스택 모드에서 실제로 만들어질 저장소 주소를 보여 준다.
		WithRepositoryLocator(cicdBundleFactory).
		WithDeletePipeline(deletePipelineUC).
		// GitOps 경로의 실행 기록은 CI 서버에만 있다. 들이지 않으면 빌드가
		// 성공해도 화면의 실행 통계가 0 으로 남는다.
		WithRunSync(runSyncUC).
		// 스캔 결과의 취약점 목록. 저장하지 않고 볼 때 CI 리포트를 다시 읽는다.
		WithScanVulnerabilities(cicduc.NewScanVulnerabilities(pgPipelineRepo, pgImageScanRepo, cicdBundleFactory)).
		// 직접 배포가 실제로 클러스터에 적용하도록 한다. 없으면 배포가
		// 실패한다 — 적용 없이 성공으로 기록하는 것보다 낫다.
		WithManifestApplier(manifestApplier).
		WithDefinitionRefresh(refreshDefinitionUC).
		// 직접 배포(POST /deploy-app)도 파이프라인 배포와 같은 수집기 주소를
		// 넣어야 한다 — 한쪽만 배선하면 경로에 따라 추적이 갈린다.
		WithStackReader(cicdStackReader).
		// 스캔 결과 조회. 빠지면 503 으로 알린다 — 빈 목록은 "스캔한 적 없음" 으로 읽힌다.
		WithImageScans(pgImageScanRepo).
		WithSASTResults(pgSASTResultRepo).
		WithDependencyScans(pgDependencyScanRepo).
		WithSecretScans(pgSecretScanRepo)

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
	// 스택 심각 경보(nullus-plan#87) — 알림 이력 화면이 읽는다. 수집은 아래 스케줄러가 한다.
	pgStackAlertRepo := obsrepo.NewPostgresStackAlertRepository(pool)
	stackAlertHandler := obshandler.NewStackAlertHandler(obsuc.NewListStackAlerts(pgStackAlertRepo))
	// 보안 대시보드(nullus-plan#65) — 관측 모듈의 읽기 모델이다. 결과 테이블은 cicd · stack 이 소유하고,
	// 대시보드는 두 모듈의 공개 경로를 bridge 로 읽는다. 클러스터 이름은 admin 의 공개 저장소로 읽는다.
	securityScope := obsbridge.NewSecurityScope(pgStackRepo, clusterRepo)
	securityHandler := obshandler.NewSecurityHandler(
		obsuc.NewGetAppSecurity(
			obsbridge.NewAppSecuritySource(cicduc.NewListPipelineSecurity(cicdrepo.NewPostgresPipelineSecurityReader(pool))),
			securityScope),
		obsuc.NewGetStackSecurity(obsbridge.NewStackSecuritySource(stackImageScanUC), securityScope),
	)

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

	// ID/PW 로그인 경로. OIDC 와 나란히 서는 두 번째 인증 수단이다 —
	// IdP 가 죽어도 들어갈 수단이 있어야 한다.
	//
	// 인증 미들웨어보다 앞에 붙인다(로그인하려면 먼저 통과해야 한다).
	// 무차별 대입을 막기 위해 로그인 전용 레이트리밋을 건다.
	localIssuer := authtoken.NewLocalIssuer(cfg.Auth.Session.Secret, cfg.SessionTTL())
	if localIssuer.Enabled() {
		loginHandler := authhandler.NewLoginHandler(authusecase.NewLogin(
			authrepo.NewPostgresCredentialRepository(pool),
			authtoken.NewSessionIssuer(localIssuer),
		))
		loginHandler.RegisterRoutes(v1.Group("", middleware.LoginRateLimiter(rateLimits)))
		slog.Info("password login enabled (ID/PW alongside OIDC)")
	} else {
		slog.Warn("password login disabled: auth.session.secret is empty " +
			"(only the IdP can authenticate — an IdP outage locks everyone out)")
	}

	var admin, stacks, cicd, observability, assistantGroup *echo.Group
	// wsAuth 는 /ws/* 전용 체인이다. 브라우저 WebSocket 은 Authorization 헤더를 못
	// 붙이므로 서브프로토콜로 온 토큰을 헤더로 옮긴 뒤 평소 인증을 태운다.
	var wsAuth []echo.MiddlewareFunc
	// goldenPathWriteGuards 는 골든패스의 생성·수정·삭제에만 붙는다. development
	// 모드에서는 비어 있다 — 그 모드는 권한 검사를 전부 끄는 쪽이므로 여기만 켜 두면
	// 다른 경로와 어긋난다.
	var goldenPathWriteGuards []echo.MiddlewareFunc
	// securityStackGuards 는 보안 대시보드의 설치 OSS 축에만 붙는다 — /stacks 와 같은 admin · devops.
	// /observability 그룹에는 역할 검사가 없다. development 모드에서는 비어 있다(권한 검사를 전부 끈다).
	var securityStackGuards []echo.MiddlewareFunc
	if cfg.Server.Mode == "development" {
		slog.Info("development mode: auth middleware disabled")
		admin = v1.Group("/admin", userRateLimit)
		stacks = v1.Group("/stacks", userRateLimit)
		cicd = v1.Group("/cicd", userRateLimit)
		observability = v1.Group("/observability", userRateLimit)
		assistantGroup = v1.Group("/assistant", userRateLimit)
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
		authMW := authmw.DualAuthMiddleware(cfg.Auth.Mode, sessionMW, oidcMW,
			authmw.WithLocalTokens(localIssuer))
		// userRateLimit 은 authMW 바로 뒤에 둔다. 권한 검사(RequireRole)보다 앞이어야
		// 403 으로 튕기는 요청도 사용량에 잡힌다.
		admin = v1.Group("/admin", authMW, userRateLimit, authmw.RequireRole("admin"))
		stacks = v1.Group("/stacks", authMW, userRateLimit, authmw.RequireRole("admin", "devops"))
		cicd = v1.Group("/cicd", authMW, userRateLimit, authmw.RequireRole("admin", "devops", "developer"))
		goldenPathWriteGuards = []echo.MiddlewareFunc{authmw.RequireRole("admin", "devops")}
		securityStackGuards = []echo.MiddlewareFunc{authmw.RequireRole("admin", "devops")}
		observability = v1.Group("/observability", authMW, userRateLimit)
		// 어시스턴트는 인증만 요구한다 — 무엇을 볼 수 있는지는 도구가 부르는 API 가 요청자의 권한으로 정한다.
		assistantGroup = v1.Group("/assistant", authMW, userRateLimit)

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
	// 알림 채널(Telegram · 이메일 · Slack, nullus-plan#63). 백업 모듈이 결과를 이 모듈의 Publisher 로 낸다.
	notificationModule := notificationmod.New(context.Background(), notificationmod.Deps{
		Pool:           pool,
		EncryptionKey:  []byte(os.Getenv("ENCRYPTION_KEY")),
		TelegramAPIURL: os.Getenv("NULLUS_TELEGRAM_API_URL"),
		Logger:         slog.Default(),
	})
	// 설치 이미지 재스캔이 새로 발견한 "수정판 있는 CRITICAL" 을 조직의 알림 채널로 보낸다(nullus-plan#78).
	stackImageScanUC.WithNotifier(notificationModule.Publisher)
	notificationHandler := notificationhandler.NewNotificationHandler(
		notificationModule.Channels, notificationModule.SendTest, notificationModule.Deliveries)
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
	stackImageScanHandler.RegisterRoutes(stacks)
	cicdTemplateHandler.RegisterRoutes(cicd)
	// 골든패스를 바꾸는 것은 조직 전체의 설치 선택지를 바꾸는 일이다 — 읽기는 /cicd 의
	// 세 역할 모두, 쓰기는 admin·devops 만. DB 로 옮기기 전에는 지워도 재시작하면
	// 돌아왔지만 이제는 영구적이다.
	cicdGoldenPathHandler.RegisterRoutes(cicd, goldenPathWriteGuards...)
	pipelineHandler.RegisterRoutes(cicd)
	// 스택별 파이프라인 조회는 /stacks 그룹 아래에 붙는다. 핸들러는 처음부터 있었는데
	// 이 호출이 빠져 있어 GET /api/v1/stacks/:stackId/pipelines 가 404 였다.
	pipelineHandler.RegisterStackRoutes(stacks)
	// 스캔 정책은 스택의 모든 스캔 파이프라인의 차단 여부를 바꾼다 — 개발자 권한인
	// /cicd 가 아니라 /stacks(admin·devops) 아래에 붙인다.
	scanPolicyHandler.RegisterStackRoutes(stacks)
	dashboardHandler.RegisterRoutes(observability)
	alertHandler.RegisterRoutes(observability)
	stackAlertHandler.RegisterRoutes(observability)
	// 앱 축은 모든 역할, 설치 OSS 축은 admin · devops 만(설계 결정 11).
	securityHandler.RegisterRoutes(observability, securityStackGuards...)

	// AI 어시스턴트(nullus-plan#94). Nullus MCP 도구를 Upstage Solar 에 물린다. 도구는 API 자신을 루프백으로,
	// 질문한 사용자의 인증 헤더를 실어 부른다. NULLUS_ASSISTANT_API_KEY 가 없으면 꺼진다.
	assistantModule := assistant.NewModule(assistant.ConfigFromEnv(os.Getenv,
		fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port), version))
	assistantHandler := assistanthandler.NewAssistantHandler(assistantModule.Chat,
		assistanthandler.Status{Provider: assistantModule.Provider, Model: assistantModule.Model})
	assistantHandler.RegisterRoutes(assistantGroup)
	if assistantModule.Enabled() {
		slog.Info("AI assistant enabled", "provider", assistantModule.Provider, "model", assistantModule.Model)
	} else {
		slog.Info("AI 어시스턴트가 꺼져 있습니다 (NULLUS_ASSISTANT_API_KEY)")
	}

	if cfg.Server.Mode == "development" {
		e.GET("/debug/pprof/*", echo.WrapHandler(http.DefaultServeMux))
	}

	e.GET("/health", healthHandler(pool))

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

	// 백업이 정지 창을 여는 동안 회전을 멈춰야 하므로 참조를 보관한다.
	// 회전은 DB 와 금고를 함께 고쳐써서, 백업 중에 돌면 두 산출물 사이에
	// 시점 어긋남을 만든다 (docs/11_기능설계/Nullus_백업복구_설계.md §2.1).
	rotationScheduler := adminscheduler.NewTokenRotationScheduler(
		pool,
		secretRouter,
		tokenRotationInterval(),
		0,
		slog.Default(),
		reissuerRouter,
	).WithRestarter(
		// 회전 후 반영: 소비자가 기동 시점에만 설정을 읽는 경우 rolling restart 한다.
		adminrepo.NewClusterWorkloadRestarter(kubeconfigProvider),
	)
	go rotationScheduler.Start(rotationCtx)

	// 스택 심각 경보 전달(nullus-plan#87). 스택마다 Alertmanager 의 활성 경보를 API 서버 프록시로
	// 가져와 기록하고, 심각한 것을 조직의 알림 채널로 보낸다. NULLUS_STACK_ALERT_INTERVAL(기본 1m, "off" 로 끔).
	if interval := obsscheduler.ParseStackAlertInterval(os.Getenv("NULLUS_STACK_ALERT_INTERVAL")); interval > 0 {
		stackAlertCollector := obsuc.NewCollectStackAlerts(
			obsbridge.NewMonitoredStacks(pgStackRepo),
			obsalertmanager.NewSource(kubeconfigProvider),
			pgStackAlertRepo,
			notificationModule.Publisher,
		).WithLock(obsrepo.NewPostgresCycleLock(pool))
		go obsscheduler.NewStackAlertScheduler(stackAlertCollector, interval, slog.Default()).Start(rotationCtx)
	} else {
		slog.Info("스택 경보 수집이 꺼져 있습니다 (NULLUS_STACK_ALERT_INTERVAL)")
	}

	// ── 백업/복구 (nullus-plan#75) ──
	//
	// 조립이 실패하면 기동을 멈춘다. 설정이 잘못된 채로 뜨면 "백업이 돌고
	// 있다" 는 착각만 남고, 그 착각은 복구를 시도할 때에야 깨진다 (§9 F10).
	// 복구 뒤 러너·Argo CD hostAliases 를 스택 게이트웨이의 지금 주소로 맞춘다. 복구는 Service 를 백업
	// 때의 ClusterIP 로 되살리지만, 다른 클러스터이거나 옛 백업이면 주소가 바뀐다.
	gatewayAliasRepinUC := stackuc.NewGatewayAliasRepin(
		pgStackRepo,
		kubeconfigProvider,
		func(kubeconfig []byte) stackport.HelmReleaseManager {
			return stackhelm.NewHelmInstaller(kubeconfig)
		},
		func(kubeconfig []byte) (stackport.GatewayAddressReader, error) {
			return stackhelm.NewGatewayAddressReader(kubeconfig)
		},
	)
	backupModule, err := backup.New(backup.Deps{
		Pool:             pool,
		Config:           cfg.Backup,
		PlatformDB:       cfg.Database,
		SecretRouter:     secretRouter,
		Kubeconfig:       kubeconfigProvider,
		TokenSources:     tokenSourceRepo,
		RotationPausing:  rotationScheduler,
		GatewayAliases:   gatewayAliasRepinUC,
		SharedComponents: ensureSharedComponentsUC,
		// 백업 세트가 "조직의 모든 스택" 을 스택 모듈의 저장소로 읽는다(bridge).
		Stacks:          backupbridge.NewStackLister(pgStackRepo),
		Notifications:   notificationModule.Publisher,
		EncryptionKey:   []byte(os.Getenv("ENCRYPTION_KEY")),
		PlatformVersion: platformVersion,
		Logger:          slog.Default(),
	})
	if err != nil {
		slog.Error("백업 모듈 조립 실패", "error", err)
		os.Exit(1)
	}

	// 설치된 OSS 업그레이드. 사전 점검이 백업 모듈에 "마지막 복구 가능 백업" 을 묻기 때문에 백업
	// 모듈 뒤에서 조립한다. 백업이 꺼져 있으면(nil) 백업이 필요한 번들은 막힌다 — 안전 조건을
	// 완화하지 않는다(설계 §8).
	var upgradeBackups stackport.UpgradeBackupLookup
	if backupModule != nil {
		upgradeBackups = backupModule.Repo
	}
	// 업그레이드는 요청 밖에서 이어진다. 그 수명은 API 프로세스에 묶는다 — 종료 때 취소해 Helm 이
	// --atomic 으로 스스로 되돌리게 한다(아래 종료 처리).
	upgradeCtx, stopUpgrades := context.WithCancel(context.Background())
	defer stopUpgrades()
	upgradeUC := stackuc.NewUpgradeStack(stackuc.UpgradeStackDeps{
		Context:     upgradeCtx,
		Stacks:      pgStackRepo,
		Runs:        stackrepo.NewPostgresUpgradeRepository(pool),
		Kubeconfigs: kubeconfigProvider,
		Releases:    func(kubeconfig []byte) stackport.HelmReleaseManager { return stackhelm.NewHelmInstaller(kubeconfig) },
		Executors:   func(kubeconfig []byte) stackport.ChartUpgradeExecutor { return stackhelm.NewHelmInstaller(kubeconfig) },
		Backups:     upgradeBackups,
		Health:      stackhelm.NewArgoCDUpgradeHealthVerifier(),
		// 후보에 "이 업그레이드가 이 스택에서 발견된 CVE 를 고친다" 를 싣는다(nullus-plan#78).
		Vulnerabilities: stackImageScanRepo,
	})
	upgradeHandler := stackhandler.NewUpgradeHandler(upgradeUC, auditLogger)
	upgradeHandler.RegisterRoutes(stacks)
	if backupModule != nil {
		backupHandler := backuphandler.NewBackupHandler(
			backupModule, backupModule.Verify, backupModule.Repo, cfg.Platform.Namespace,
		).
			// X-Org-ID 의 조직이 실제로 있는지 admin 의 공개 인터페이스로 본다. 없는
			// 조직으로 만든 백업은 어떤 화면에도 보이지 않는다.
			WithOrganizationChecker(backupbridge.NewOrganizationChecker(orgRepo))
		backupHandler.RegisterRoutes(admin)

		if sc := cfg.Backup.Schedule; sc.Enabled {
			// all_stacks 면 단독 유스케이스 대신 세트를 돈다 — 플랫폼 + 조직의 모든 스택.
			var single *backupusecase.BackupUseCase
			if !sc.AllStacks {
				single = mustBackupUseCase(rotationCtx, backupModule, sc.StackID)
			}
			sched := backupscheduler.New(single, backupModule.Retention, backupscheduler.Config{
				Interval:  sc.Interval,
				OrgID:     sc.OrgID,
				StackID:   sc.StackID,
				Namespace: sc.Namespace,
				Mode:      backupdomain.Mode(sc.Mode),
				Logger:    slog.Default(),
			})
			if sc.AllStacks {
				if set, err := backupModule.BackupSet(rotationCtx); err != nil {
					slog.Error("스케줄 세트 백업 유스케이스를 만들 수 없습니다 — 주기 백업이 돌지 않습니다", "error", err)
				} else {
					sched.WithBackupSet(set)
				}
			}
			go sched.Start(rotationCtx)
		}
	}

	// 끊긴 설치를 주기적으로 실패로 옮긴다.
	//
	// 설치는 이 프로세스 안의 고루틴이 돌린다. 파드가 교체되면 그 고루틴이
	// 사라지고 아무도 실패를 기록하지 않는다 — 스택은 installing 인 채로 남고,
	// 그 상태에서는 이어서 진행조차 막혀 사용자에게 길이 없다.
	//
	// 기동 시 한 번이 아니라 주기적으로 돈다. 레플리카가 여럿이면 "내가 재시작
	// 했다" 가 "아무도 안 돌리고 있다" 를 뜻하지 않으므로, 판단 근거는 시간뿐이다.
	go func() {
		reaper := stackuc.NewReapStaleInstalls(pgStackRepo)
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-rotationCtx.Done():
				return
			case <-ticker.C:
				if reaped, err := reaper.Run(rotationCtx); err != nil {
					slog.Warn("stale install reaper failed", "error", err)
				} else if reaped > 0 {
					slog.Warn("marked stale installs as failed", "count", reaped)
				}
			}
		}
	}()

	// 파이프라인 실행 기록과 스캔 결과를 주기적으로 들인다.
	//
	// 레플리카가 여럿이면 각자 돈다. 기록은 upsert 라 겹쳐도 결과는 같고 CI 조회만 는다.
	go runEvery(rotationCtx, durationFromEnv("CICD_RUN_SYNC_INTERVAL", 10*time.Minute), func(ctx context.Context) {
		if synced, err := runSyncUC.SyncAll(ctx); err != nil {
			slog.Warn("pipeline run sync failed", "error", err)
		} else if synced > 0 {
			slog.Debug("pipeline runs synced", "pipelines", synced)
		}
	})

	// 스택이 설치한 OSS 이미지를 주기적으로 다시 스캔한다. 설치 뒤에 공개된 CVE 는
	// 다시 스캔해야 보인다. 에어갭 설치도 돈다 — DB 는 반입한 것이라 새 CVE 는 번들을 다시
	// 반입해야 보인다(화면이 DB 기준일을 보인다). 기동 직후에는 마지막 스캔이 주기보다 오래된
	// 스택만 먼저 스캔한다 — 타이머는 재시작마다 처음부터 다시 세어진다.
	rescanInterval := durationFromEnv("STACK_IMAGE_RESCAN_INTERVAL", 24*time.Hour)
	go func() {
		if scanned := stackImageScanUC.RescanStale(rotationCtx, rescanInterval); scanned > 0 {
			slog.Info("stale installed images rescanned at startup", "stacks", scanned)
		}
		runEvery(rotationCtx, rescanInterval, func(ctx context.Context) {
			if scanned := stackImageScanUC.RescanAll(ctx); scanned > 0 {
				slog.Info("installed images rescanned", "stacks", scanned)
			}
		})
	}()

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	go func() {
		slog.Info("starting server", "addr", addr, "version", version)
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
	// 진행 중인 업그레이드를 취소하고 잠시 기다린다. 기다리는 사이 끝나지 않으면 실행은 박동이 끊겨
	// 5분 뒤 수동 개입 필요로 닫힌다(domain.UpgradeHeartbeatTimeout).
	stopUpgrades()
	upgradeWait, cancelUpgradeWait := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelUpgradeWait()
	if err := upgradeUC.Wait(upgradeWait); err != nil {
		slog.Warn("upgrade jobs still running at shutdown", "error", err)
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

// runEvery 는 interval 마다 fn 을 돌린다. fn 이 끝나기 전에 온 틱은 버려지므로 실행이 겹치지 않는다.
func runEvery(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn(ctx)
		}
	}
}

// durationFromEnv 는 환경 변수의 기간 값이다. 비었거나 잘못됐으면 기본값이다.
func durationFromEnv(name string, fallback time.Duration) time.Duration {
	if raw := strings.TrimSpace(os.Getenv(name)); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
		slog.Warn("invalid duration env; using default", "name", name, "value", raw, "default", fallback)
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

// platformVersion 은 매니페스트에 남기는 플랫폼 버전이다.
//
// 복구 시 "어느 버전이 뜬 백업인가" 를 열어보기 전에 알 수 있어야 한다.
// 빌드 시 -ldflags 로 주입하고, 없으면 dev 로 둔다.
var platformVersion = "dev"

// mustBackupUseCase 는 스케줄 백업용 유스케이스를 미리 만든다.
//
// 스케줄은 대상 스택이 고정이라 기동 시점에 만들 수 있다. 실패하면 nil 을
// 돌려주고, 스케줄러는 그때 "구성되지 않았다" 고 남기며 시작하지 않는다 —
// 조용히 도는 빈 스케줄러가 가장 나쁘다.
func mustBackupUseCase(ctx context.Context, m *backup.Module, stackID string) *backupusecase.BackupUseCase {
	uc, err := m.Backup(ctx, stackID)
	if err != nil {
		slog.Error("스케줄 백업 유스케이스를 만들 수 없습니다 — 주기 백업이 돌지 않습니다",
			"stack_id", stackID, "error", err)
		return nil
	}
	return uc
}
