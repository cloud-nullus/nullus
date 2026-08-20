package helm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

const (
	defaultSelfSignedBootstrapIssuer = "nullus-selfsigned-bootstrap"
	defaultInternalCAIssuer          = "nullus-internal-ca-issuer"
	defaultInternalCASecretName      = "nullus-internal-ca"
	defaultInternalCACertName        = "nullus-internal-ca-cert"
	defaultEnvoyDataPlaneTLSSecret   = "envoy"
	defaultEnvoyControlPlaneSecret   = "envoy-gateway"
	gatewayAPIStandardInstallURL     = "https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml"
	stepInstallingCertManager        = "installing_cert_manager"
	// Prometheus Operator 의 CRD 를 먼저 깐다. ServiceMonitor / Probe 를 만드는
	// 단계들이 kube-prometheus-stack 설치보다 한참 앞서기 때문이다.
	stepInstallingPrometheusCRDs = "installing_prometheus_crds"
	stepInstallingRunner         = "installing_runner"
	stepInstallingOTelCollector  = "installing_otel_collector"
	stepInstallingOTelAgent      = "installing_otel_agent"
)

var installGatewayOCIRelease = installOCIChartWithHelmCLI
var bootstrapInternalCAInstallation = func(ctx context.Context, o *Orchestrator, namespace string) error {
	return o.bootstrapInternalCA(ctx, namespace)
}
var waitForCertManagerInstallation = func(ctx context.Context, o *Orchestrator) error {
	return o.waitForCertManagerInstallation(ctx)
}
var verifyReleaseRuntimeReadiness = func(ctx context.Context, o *Orchestrator, step, releaseName, namespace string) error {
	return o.verifyReleaseRuntimeReadiness(ctx, step, releaseName, namespace)
}

// checkExistingMetricsServerInstallation 은 클러스터에 metrics-server 가 이미
// 있는지 본다.
//
// metrics-server 는 클러스터 하나에 하나만 존재하는 컴포넌트인데, 차트가
// ClusterRole 같은 클러스터 범위 리소스를 만든다. 같은 클러스터에 두 번째
// 스택을 설치하면 그 리소스를 다른 릴리스가 이미 소유하고 있어
// "invalid ownership metadata" 로 설치 전체가 멈춘다 — cert-manager 는 같은
// 이유로 이미 재사용 경로를 갖고 있다.
var checkExistingMetricsServerInstallation = func(ctx context.Context, o *Orchestrator) (bool, error) {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return false, nil
	}
	// APIService 는 metrics-server 가 동작 중일 때만 등록된다. Deployment 이름은
	// 릴리스마다 다를 수 있어 이 쪽이 더 정확하다.
	if _, err := o.runKubectl(ctx, "get", "apiservice", "v1beta1.metrics.k8s.io"); err != nil {
		return false, nil
	}
	return true, nil
}

var checkExistingCertManagerInstallation = func(ctx context.Context, o *Orchestrator) (bool, error) {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return false, nil
	}

	requiredCRDs := []string{
		"certificaterequests.cert-manager.io",
		"certificates.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"issuers.cert-manager.io",
	}
	for _, crd := range requiredCRDs {
		if _, err := o.runKubectl(ctx, "get", "crd", crd); err != nil {
			return false, nil
		}
	}

	if _, err := o.detectCertManagerNamespace(ctx); err != nil {
		return false, nil
	}

	return true, nil
}

type Orchestrator struct {
	installer            port.HelmInstaller
	helmStepMetadataRepo port.HelmStepMetadataRepository
	resourceDefaultRepo  port.ResourceDefaultRepository
	rollback             *RollbackManager
	kubeconfig           []byte
	namespace            string
	stepOrder            map[string]int
	orderedStep          []string
	stackConfig          *domain.StackConfig
	stepConfigFieldPath  map[string]string
	stepConfigEnabled    map[string]func(domain.StackConfig) bool
	mu                   sync.Mutex
	progress             map[string]int
	resourceDefaults     map[string]*domain.ResourceDefault
	defaultsLoaded       bool
	sharedClusterScoped  bool
	// 시크릿 경로 접두사(kv/nullus/{env}/{org}/)에 쓰이는 스코프
	secretEnv   string
	secretOrgID string
	// ssoFactory 는 스택별 SSO provisioner 생성기다 (auth 모듈 구현체 주입).
	ssoFactory port.SSOProvisionerFactory
	// toolOIDCIssuer 는 설치되는 OSS 가 쓸 Keycloak issuer 다 (조립 지점에서 주입).
	toolOIDCIssuer string
	// imageProjectName 은 레지스트리에 만들 프로젝트 이름이다.
	//
	// CI/CD 모듈의 그룹 경로와 같아야 이미지 주소가 맞는다. 모듈 간 직접
	// import 가 금지되므로 조립 지점에서 주입받는다.
	imageProjectName string
}

type OrchestratorOption func(*Orchestrator)

func WithResourceDefaultRepository(repo port.ResourceDefaultRepository) OrchestratorOption {
	return func(o *Orchestrator) {
		o.resourceDefaultRepo = repo
	}
}

// WithImageProjectName 은 레지스트리 프로젝트 이름을 주입한다.
//
// CI/CD 모듈의 그룹 경로(NULLUS_SCM_GROUP)와 같은 값이어야 한다 — 다르면
// 프로젝트는 만들어지는데 CI 가 push 하는 주소는 다른 프로젝트를 가리켜
// "project not found" 로 막힌다.
func WithImageProjectName(name string) OrchestratorOption {
	return func(o *Orchestrator) {
		o.imageProjectName = strings.TrimSpace(name)
	}
}

// WithToolOIDCIssuer 는 설치되는 OSS 에 넣을 Keycloak issuer 를 주입한다.
//
// 포털이 로그인한 Keycloak 과 오리진이 같아야 SSO 세션 쿠키가 실려 도구로 재인증
// 없이 넘어간다. 그래서 이 값은 스택의 access_domain 이 아니라 플랫폼 설정에서
// 온다 — 플랫폼 Keycloak 은 스택마다가 아니라 하나뿐이기 때문이다.
func WithToolOIDCIssuer(issuer string) OrchestratorOption {
	return func(o *Orchestrator) {
		o.toolOIDCIssuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	}
}

func WithSharedClusterScopedComponents(enabled bool) OrchestratorOption {
	return func(o *Orchestrator) {
		o.sharedClusterScoped = enabled
	}
}

func (o *Orchestrator) VerifyDeployment(ctx context.Context, stackID string) error {
	_ = stackID

	for _, step := range o.orderedStep {
		if step == "integration_check" {
			continue
		}
		if !o.isStepEnabled(step) {
			continue
		}
		if _, ok := o.stepManifestForStep(step); ok {
			continue
		}

		spec, ok := o.chartSpecForStep(step)
		if !ok {
			continue
		}
		spec = o.resolveChartSpecForStep(step, spec)
		if strings.TrimSpace(spec.ChartName) == "" {
			continue
		}

		namespace := o.namespace
		if spec.Namespace != "" {
			namespace = spec.Namespace
		}

		releaseName := o.releaseNameForSpec(spec)
		status, err := o.installer.Status(ctx, releaseName, namespace)
		if err != nil && step == "installing_gateway" && isReleaseNotFoundError(err) {
			if fallbackErr := installGatewayOCIRelease(ctx, o.kubeconfig, releaseName, spec.ChartName, namespace, spec.Version, nil, "", false); fallbackErr == nil {
				status, err = o.installer.Status(ctx, releaseName, namespace)
			}
		}
		if err != nil {
			if step == stepInstallingCertManager && isReleaseNotFoundError(err) {
				if readinessErr := o.waitForCertManagerInstallation(ctx); readinessErr == nil {
					continue
				}
			}
			// 설치 단계에서 기존 metrics-server 를 재사용했다면 이 스택 이름의
			// 릴리스는 존재하지 않는다. 그대로 두면 설치는 끝났는데 health check
			// 에서만 실패하는, 원인이 멀리 떨어진 실패가 된다.
			if step == "installing_metrics_server" && isReleaseNotFoundError(err) {
				if reused, checkErr := checkExistingMetricsServerInstallation(ctx, o); checkErr == nil && reused {
					continue
				}
			}
			if step == stepInstallingRunner && isReleaseNotFoundError(err) {
				// 릴리스 부재를 건너뛰면 러너 없는 스택이 health_check 를 통과한다.
				// 설치 스텝이 성공했다면 릴리스는 반드시 존재해야 한다.
				//
				// 단 이 요구는 GitLab CI 스택에만 해당한다. Jenkins 는 실행기를
				// kubernetes 플러그인으로 직접 띄우므로 gitlab-runner 릴리스가
				// 없는 것이 정상이다 — 여기서 걸러 내지 않으면 Jenkins 스택은
				// 설치가 다 끝나도 completed 에 도달하지 못한다.
				o.mu.Lock()
				cfg := o.stackConfig
				o.mu.Unlock()
				if cfg == nil || runnerReleaseRequired(*cfg) {
					return fmt.Errorf("gitlab-runner 릴리스가 없습니다 (%s/%s): CI 실행기 없이 스택이 완료될 수 없습니다: %w",
						namespace, releaseName, err)
				}
				continue
			}
			return fmt.Errorf("status check failed for %s: %w", releaseName, err)
		}
		if status == nil {
			return fmt.Errorf("status check returned nil for %s", releaseName)
		}

		if !strings.EqualFold(status.Status, "deployed") {
			return fmt.Errorf("release %s is not healthy: status=%s", releaseName, status.Status)
		}
		if err := verifyReleaseRuntimeReadiness(ctx, o, step, releaseName, namespace); err != nil {
			return fmt.Errorf("runtime readiness failed for %s: %w", releaseName, err)
		}
	}

	return nil
}

func (o *Orchestrator) RollbackDeployment(ctx context.Context, stackID string) error {
	_ = stackID
	rbErr := o.rollback.RollbackAll(ctx, o.installer, o.namespace)
	cleanupErr := o.cleanupResidualReleaseResources(ctx)
	if rbErr != nil && cleanupErr != nil {
		return fmt.Errorf("rollback: %w; residual cleanup: %v", rbErr, cleanupErr)
	}
	if rbErr != nil {
		return rbErr
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	return nil
}

func (o *Orchestrator) cleanupResidualReleaseResources(ctx context.Context) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	resourceKinds := []string{"deploy", "sts", "ds", "job", "cronjob", "svc", "cm", "secret", "pvc"}
	seen := map[string]struct{}{}
	var errs []error

	for _, step := range o.orderedStep {
		spec, ok := o.chartSpecForStep(step)
		if !ok {
			continue
		}
		spec = o.resolveChartSpecForStep(step, spec)
		releaseName := strings.TrimSpace(o.releaseNameForSpec(spec))
		if releaseName == "" {
			continue
		}
		namespace := strings.TrimSpace(o.namespace)
		if strings.TrimSpace(spec.Namespace) != "" {
			namespace = strings.TrimSpace(spec.Namespace)
		}
		if step == stepInstallingCertManager {
			if detectedNS, err := o.detectCertManagerReleaseNamespaceFromCRD(ctx); err == nil && strings.TrimSpace(detectedNS) != "" {
				namespace = strings.TrimSpace(detectedNS)
			}
		}
		if namespace == "" {
			continue
		}

		key := namespace + "::" + releaseName
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		for _, kind := range resourceKinds {
			selector := "app.kubernetes.io/instance=" + releaseName
			if _, err := o.runKubectl(ctx, "delete", kind, "-n", namespace, "-l", selector, "--ignore-not-found"); err != nil {
				errs = append(errs, fmt.Errorf("delete %s for release %s in namespace %s: %w", kind, releaseName, namespace, err))
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// isWorkloadlessRelease 는 파드를 만들지 않는 릴리스인지 본다.
//
// CRD 만 담은 차트가 그렇다. 준비 검사는 파드를 전제하므로 그대로 두면
// "no pods found for release" 로 설치가 죽는다.
//
// "파드가 없으면 통과" 로 일반화하지 않는다. 정작 파드를 만들어야 하는 릴리스가
// 아무것도 못 만들었을 때 그것까지 조용히 통과시키기 때문이다.
func isWorkloadlessRelease(releaseName string) bool {
	switch strings.TrimSpace(releaseName) {
	case "prometheus-operator-crds":
		return true
	}
	return false
}

func (o *Orchestrator) verifyReleaseRuntimeReadiness(ctx context.Context, step, releaseName, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	if isWorkloadlessRelease(releaseName) {
		return nil
	}

	if err := o.waitForReleaseRollouts(ctx, releaseName, namespace); err != nil {
		return err
	}

	snapshot, err := o.releasePodSnapshot(ctx, releaseName, namespace)
	if err != nil {
		return err
	}
	if len(snapshot.Items) == 0 {
		return fmt.Errorf("no pods found for release %s in namespace %s", releaseName, namespace)
	}

	for _, pod := range snapshot.Items {
		phase := strings.TrimSpace(strings.ToLower(pod.Status.Phase))
		if phase == "succeeded" {
			continue
		}
		if phase != "running" {
			return fmt.Errorf("pod %s phase=%s", pod.Metadata.Name, strings.TrimSpace(pod.Status.Phase))
		}
		if len(pod.Status.ContainerStatuses) == 0 {
			return fmt.Errorf("pod %s has no container status yet", pod.Metadata.Name)
		}
		for _, container := range pod.Status.ContainerStatuses {
			if !container.Ready {
				return fmt.Errorf("pod %s container %s not ready", pod.Metadata.Name, container.Name)
			}
		}
	}

	_ = step
	return nil
}

type ChartSpec struct {
	ReleaseName string
	ChartName   string
	RepoURL     string
	Version     string
	Namespace   string
	Values      map[string]any
	Wait        bool
}

func boolPtr(v bool) *bool {
	return &v
}

func NewOrchestrator(installer port.HelmInstaller, kubeconfig []byte, namespace string, opts ...OrchestratorOption) *Orchestrator {
	if namespace == "" {
		namespace = "nullus"
	}
	o := &Orchestrator{
		installer:  installer,
		rollback:   &RollbackManager{},
		kubeconfig: kubeconfig,
		namespace:  namespace,
		// 시크릿 평면(OpenBao → ESO → provisioning)이 스토리지보다 앞선다.
		// PostgreSQL/MinIO 차트가 provisioning_secrets 가 만든 Secret 을
		// existingSecret 으로 참조하기 때문이다. usecase 의 installDAG 와
		// 순서가 일치해야 한다 — ensureOrder 가 이 순서를 강제한다.
		orderedStep: []string{
			stepInstallingCertManager,
			stepInstallingPrometheusCRDs,
			"installing_metrics_server",
			"installing_openbao",
			"installing_external_secrets",
			"provisioning_secrets",
			"installing_postgresql",
			"installing_minio",
			"installing_object_storage_secret",
			"installing_object_storage_buckets",
			"installing_database_connection_check",
			"provisioning_sso",
			"installing_gitlab",
			"installing_gitea",
			"provisioning_gitea",
			"installing_harbor",
			"provisioning_harbor",
			"installing_nexus",
			"provisioning_nexus",
			"installing_argocd",
			stepInstallingRunner,
			"installing_jenkins",
			"installing_prometheus",
			"installing_grafana",
			"installing_logging",
			"installing_log_search",
			"installing_opentelemetry",
			stepInstallingOTelCollector,
			stepInstallingOTelAgent,
			"installing_gateway",
			"integration_check",
		},
		stepConfigFieldPath: map[string]string{
			"installing_postgresql":                "config.storage.database",
			"installing_minio":                     "config.artifacts.storage_backend",
			"installing_object_storage_secret":     "config.storage.object_storage",
			"installing_object_storage_buckets":    "config.storage.object_storage",
			"installing_database_connection_check": "config.storage.database",
			// 시크릿 평면 3단계는 항상 켜지므로 "비활성 스텝" 로그 경로를 타지
			// 않는다. config.authentication.provider 로 매핑해 두면 이 값이
			// 설치 여부를 좌우하는 것처럼 읽혀 오해를 부르므로 넣지 않는다.
			// provisioning_sso 는 선택형이라 매핑을 유지한다.
			"provisioning_sso":           "config.authentication.provider",
			stepInstallingPrometheusCRDs: "config.monitoring.collection",
			"installing_gitlab":          "config.artifacts.source_repository",
			"installing_gitea":           "config.artifacts.source_repository",
			"provisioning_gitea":         "config.artifacts.source_repository",
			"installing_harbor":          "config.artifacts.container_registry",
			"provisioning_harbor":        "config.artifacts.container_registry",
			"installing_nexus":           "config.artifacts.container_registry",
			"provisioning_nexus":         "config.artifacts.container_registry",
			"installing_argocd":          "config.pipeline.cd_tool",
			stepInstallingRunner:         "config.pipeline.ci_platform",
			"installing_jenkins":         "config.pipeline.ci_platform",
			"installing_prometheus":      "config.monitoring.collection",
			"installing_grafana":         "config.monitoring.visualization",
			"installing_logging":         "config.logging.collection",
			"installing_log_search":      "config.logging.search",
			"installing_opentelemetry":   "config.logging.trace_layer",
			stepInstallingOTelCollector:  "config.logging.trace_exporter",
			stepInstallingOTelAgent:      "config.logging.trace_exporter",
		},
		stepConfigEnabled: map[string]func(domain.StackConfig) bool{
			"installing_postgresql": func(cfg domain.StackConfig) bool {
				if cfg.Storage == nil {
					return true
				}
				return strings.TrimSpace(cfg.Storage.Database.Mode) == "create"
			},
			"installing_minio": func(cfg domain.StackConfig) bool {
				return cfg.Artifacts.StorageBackend.Enabled
			},
			"installing_object_storage_secret": func(cfg domain.StackConfig) bool {
				return cfg.Artifacts.StorageBackend.Enabled && isGitLabSourceRepositorySelection(cfg.Artifacts.SourceRepository)
			},
			"installing_object_storage_buckets": func(cfg domain.StackConfig) bool {
				return cfg.Artifacts.StorageBackend.Enabled && isGitLabSourceRepositorySelection(cfg.Artifacts.SourceRepository)
			},
			"installing_database_connection_check": func(cfg domain.StackConfig) bool {
				if !isGitLabSourceRepositorySelection(cfg.Artifacts.SourceRepository) || cfg.Storage == nil {
					return false
				}
				return strings.TrimSpace(cfg.Storage.Database.Mode) == "existing-connect"
			},
			"installing_gitlab": func(cfg domain.StackConfig) bool {
				return isGitLabSourceRepositorySelection(cfg.Artifacts.SourceRepository)
			},
			"installing_gitea": func(cfg domain.StackConfig) bool {
				return isGiteaSourceRepositorySelection(cfg.Artifacts.SourceRepository)
			},
			// Gitea 를 고르지 않은 스택에서는 이 단계도 서지 않는다. 술어가 없으면
			// ensureOrder 가 항상 이 단계를 기대해 다음 설치가 순서 위반으로 막힌다.
			"provisioning_gitea": func(cfg domain.StackConfig) bool {
				return isGiteaSourceRepositorySelection(cfg.Artifacts.SourceRepository)
			},
			// Harbor / Nexus 는 독립 레지스트리를 고른 경우에만 선다.
			// GitLab Registry 를 고르면 GitLab 이 겸하므로 둘 다 꺼진다.
			"installing_harbor": func(cfg domain.StackConfig) bool {
				return isHarborRegistrySelection(cfg.Artifacts.ContainerRegistry)
			},
			// 설치만 한 Harbor 에는 프로젝트가 없어 CI 가 이미지를 올릴 곳이 없다.
			"provisioning_harbor": func(cfg domain.StackConfig) bool {
				return isHarborRegistrySelection(cfg.Artifacts.ContainerRegistry)
			},
			"installing_nexus": func(cfg domain.StackConfig) bool {
				return isNexusSelection(cfg.Artifacts.ContainerRegistry) ||
					isNexusSelection(cfg.Artifacts.PackageRegistry)
			},
			// 설치 직후의 Nexus 는 Docker 커넥터도 저장소도 없다.
			// 이 단계가 없으면 CI 가 이미지를 올릴 곳이 존재하지 않는다.
			"provisioning_nexus": func(cfg domain.StackConfig) bool {
				return isNexusSelection(cfg.Artifacts.ContainerRegistry) ||
					isNexusSelection(cfg.Artifacts.PackageRegistry)
			},
			// 시크릿 평면(installing_openbao / installing_external_secrets /
			// provisioning_secrets)은 stepConfigEnabled 에 넣지 않는다 — 항상 켜진다.
			//
			// 과거에는 authentication.provider=openbao 일 때만 설치하는 선택형
			// 경로였으나, PostgreSQL/MinIO 차트가 비밀번호를 values 로 받지 않고
			// provisioning_secrets 가 만든 Secret 을 existingSecret 으로 참조하도록
			// 바뀌면서 선택형일 수 없게 되었다. 이 Secret 을 만드는 경로는
			// provisioning_secrets 하나뿐이라, 꺼지면 파드가 FailedMount 로
			// 기동하지 못한다. (기본값 provider='' 에서 설치가 멈추던 원인)
			//
			// 반면 provisioning_sso 는 OSS OIDC 연동을 켤 때만 필요하므로
			// 선택형으로 유지한다.
			// CRD 는 Prometheus 를 고른 스택에서만 필요하다.
			stepInstallingPrometheusCRDs: func(cfg domain.StackConfig) bool {
				return cfg.Monitoring.Collection.Enabled
			},
			"provisioning_sso": func(cfg domain.StackConfig) bool {
				if cfg.Authentication == nil {
					return false
				}
				return strings.EqualFold(strings.TrimSpace(cfg.Authentication.Provider), "openbao")
			},
			"installing_argocd": func(cfg domain.StackConfig) bool {
				return cfg.Pipeline.CDTool.Enabled
			},
			stepInstallingRunner: func(cfg domain.StackConfig) bool {
				return runnerReleaseRequired(cfg)
			},
			"installing_jenkins": func(cfg domain.StackConfig) bool {
				return isJenkinsCISelection(cfg.Pipeline.CIPlatform)
			},
			"installing_prometheus": func(cfg domain.StackConfig) bool {
				return cfg.Monitoring.Collection.Enabled
			},
			"installing_grafana": func(cfg domain.StackConfig) bool {
				return cfg.Monitoring.Visualization.Enabled
			},
			"installing_logging": func(cfg domain.StackConfig) bool {
				return cfg.Logging.Collection.Enabled
			},
			"installing_log_search": func(cfg domain.StackConfig) bool {
				if !cfg.Logging.Search.Enabled {
					return false
				}
				search := strings.TrimSpace(cfg.Logging.Search.Name)
				collection := strings.TrimSpace(cfg.Logging.Collection.Name)
				if search != "" && collection != "" && search == collection {
					return false
				}
				return true
			},
			"installing_opentelemetry": func(cfg domain.StackConfig) bool {
				return cfg.Logging.TraceLayer.Enabled
			},
			stepInstallingOTelCollector: func(cfg domain.StackConfig) bool {
				return cfg.Logging.TraceExporter.Enabled
			},
			// 에이전트는 파드 로그를 긁어 게이트웨이로 넘긴다. 게이트웨이가 없거나
			// 로그를 받아 줄 저장소가 없으면 긁을 이유가 없다 — 노드마다 도는
			// DaemonSet 을 갈 곳도 없는 로그를 위해 띄우지 않는다.
			stepInstallingOTelAgent: func(cfg domain.StackConfig) bool {
				if !cfg.Logging.TraceExporter.Enabled {
					return false
				}
				return lokiSelected(cfg.Logging.Collection) || lokiSelected(cfg.Logging.Search)
			},
		},
		progress: make(map[string]int),
	}
	// stepOrder 는 orderedStep 에서 파생한다. 두 목록을 손으로 맞추면 반드시
	// 어긋나고, 어긋나면 ensureOrder 가 엉뚱한 단계를 기다리다 설치가 멈춘다.
	o.stepOrder = make(map[string]int, len(o.orderedStep))
	for i, step := range o.orderedStep {
		o.stepOrder[step] = i
	}

	for _, opt := range opts {
		opt(o)
	}
	return o
}

func isGitLabSourceRepositorySelection(sel domain.ToolSelection) bool {
	if !sel.Enabled {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(sel.Version), "external") {
		return false
	}
	name := normalizeToolName(sel.Name)
	if name == "" {
		return true
	}
	return name == "gitlab" || name == "gitlab-ce"
}

// isGiteaSourceRepositorySelection 은 소스 저장소로 Gitea 를 골랐는지 본다.
//
// 이름이 비어 있으면 false 다 — 빈 이름의 기본값은 GitLab 이므로
// (isGitLabSourceRepositorySelection 이 true 를 돌려준다) 여기서도 true 를
// 돌리면 두 스텝이 동시에 서서 같은 슬롯에 제품 둘이 올라간다.
func isGiteaSourceRepositorySelection(sel domain.ToolSelection) bool {
	if !sel.Enabled {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(sel.Version), "external") {
		return false
	}
	return normalizeToolName(sel.Name) == "gitea"
}

// isHarborRegistrySelection 은 컨테이너 레지스트리로 Harbor 를 골랐는지 본다.
//
// 이름이 비어 있으면 GitLab 판단과 달리 참으로 보지 않는다 — 레지스트리는
// GitLab 내장이 기본이므로, 모르는 값에 Harbor 를 설치하면 쓰지도 않을 것을
// 올리게 된다.
func isHarborRegistrySelection(sel domain.ToolSelection) bool {
	if !sel.Enabled || isExternalSelection(sel) {
		return false
	}
	return normalizeToolName(sel.Name) == "harbor"
}

// isGitLabContainerRegistrySelection 은 GitLab 내장 레지스트리를 골랐는지 본다.
//
// 이름이 비어 있으면 참으로 본다 — 레지스트리 기본값이 GitLab 내장이기 때문이다.
// Harbor/Nexus 를 고른 경우는 명시적으로 제외한다.
func isGitLabContainerRegistrySelection(sel domain.ToolSelection) bool {
	if !sel.Enabled || isExternalSelection(sel) {
		return false
	}
	if isHarborRegistrySelection(sel) || isNexusSelection(sel) {
		return false
	}
	return true
}

// isNexusSelection 은 컨테이너 레지스트리 또는 패키지 저장소로 Nexus 를
// 골랐는지 본다. Nexus 는 둘 다 겸할 수 있어 양쪽에서 호출된다.
func isNexusSelection(sel domain.ToolSelection) bool {
	if !sel.Enabled || isExternalSelection(sel) {
		return false
	}
	switch normalizeToolName(sel.Name) {
	case "nexus", "nexus3", "sonatype-nexus", "nexus-repository", "nexus-repository-manager":
		return true
	}
	return false
}

// isExternalSelection 은 클러스터 밖 서비스를 가리키는 선택인지 본다.
func isExternalSelection(sel domain.ToolSelection) bool {
	return strings.EqualFold(strings.TrimSpace(sel.Version), "external")
}

func isGitLabCISelection(sel domain.ToolSelection) bool {
	if !sel.Enabled {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(sel.Version), "external") {
		return false
	}
	name := normalizeToolName(sel.Name)
	if name == "" {
		return true
	}
	return name == "gitlab-ci" || name == "gitlab-runner"
}

// isJenkinsCISelection 은 CI 플랫폼으로 Jenkins 를 골랐는지 본다.
//
// 이름이 비면 false 다 — 빈 이름의 기본값은 GitLab CI 이므로
// (isGitLabCISelection 이 true 를 돌려준다) 여기서도 true 를 돌리면 실행기가
// 둘 다 선다.
func isJenkinsCISelection(sel domain.ToolSelection) bool {
	if !sel.Enabled {
		return false
	}
	if isExternalSelection(sel) {
		return false
	}
	return normalizeToolName(sel.Name) == "jenkins"
}

// runnerReleaseRequired 는 이 스택이 gitlab-runner 릴리스를 반드시 가져야 하는지
// 본다. health check 의 하드 게이트가 이 값을 본다.
//
// GitLab CI 스택은 실행기 없이 완료되면 안 된다 — 파이프라인이 돌 곳이 없다.
// 반면 Jenkins 스택은 실행기를 Jenkins 가 kubernetes 플러그인으로 직접 띄우므로
// gitlab-runner 릴리스가 존재하지 않는 것이 정상이다. 이 구분이 없으면 Jenkins
// 스택은 설치가 다 끝나도 영원히 completed 에 도달하지 못한다.
func runnerReleaseRequired(cfg domain.StackConfig) bool {
	if isJenkinsCISelection(cfg.Pipeline.CIPlatform) {
		return false
	}
	return isGitLabSourceRepositorySelection(cfg.Artifacts.SourceRepository) ||
		isGitLabCISelection(cfg.Pipeline.CIPlatform)
}

func normalizeToolName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.NewReplacer(" ", "-", "_", "-").Replace(normalized)
	return normalized
}

func (o *Orchestrator) SetNamespace(namespace string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if namespace == "" {
		namespace = "nullus"
	}
	o.namespace = namespace
}

func (o *Orchestrator) IsStepEnabled(step string) bool {
	return o.isStepEnabled(step)
}

func (o *Orchestrator) SetStackConfig(config domain.StackConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	cfg := config
	o.stackConfig = &cfg
}

// ResumeFromStep initializes ordering for a new executor created during a
// continued deployment, so the failed step can be reapplied directly.
func (o *Orchestrator) ResumeFromStep(stackID, step string) {
	if strings.TrimSpace(stackID) == "" {
		return
	}
	order, ok := o.stepOrder[step]
	if !ok {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.progress[stackID] = order - 1
}

func (o *Orchestrator) ExecuteStep(ctx context.Context, stackID, step, phase string) error {
	_ = stackID
	_ = phase

	order, ok := o.stepOrder[step]
	if !ok {
		if _, specOK := o.chartSpecForStep(step); !specOK {
			return fmt.Errorf("unknown step %q", step)
		}
		return fmt.Errorf("step order not defined for %q", step)
	}
	if err := o.ensureOrder(stackID, step, order); err != nil {
		return err
	}

	if !o.isStepEnabled(step) {
		o.markCompleted(stackID, order)
		path := o.stepConfigFieldPath[step]
		slog.Info("skipping disabled stack install step", "stack_id", stackID, "step", step, "config_path", path)
		return nil
	}

	if step == "integration_check" {
		if looksLikeKubeconfig(o.kubeconfig) {
			targetNamespace := strings.TrimSpace(o.namespace)
			if targetNamespace == "" {
				targetNamespace = "nullus"
			}
			if err := o.tryReconcileGatewayDataPlaneTLSSecret(ctx, targetNamespace); err != nil {
				return fmt.Errorf("reconcile gateway data-plane tls secret: %w", err)
			}
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if step == "installing_object_storage_secret" {
		if !looksLikeKubeconfig(o.kubeconfig) {
			o.markCompleted(stackID, order)
			return nil
		}
		// ESO 주입 평면이 켜져 있으면 nullus-object-storage 는 ExternalSecret 이
		// 소유한다. 여기서 같은 이름의 Secret 을 직접 만들면 ESO 가 주기적으로
		// 덮어써 소유권이 충돌하므로 건너뛴다.
		if o.isStepEnabled("installing_external_secrets") {
			slog.Info("object storage secret 은 ESO 가 소유하므로 직접 생성하지 않습니다", "namespace", o.namespace)
			o.markCompleted(stackID, order)
			return nil
		}
		manifest := o.sharedObjectStorageSecretManifest(o.namespace)
		if strings.TrimSpace(manifest) != "" {
			if err := o.applyManifest(ctx, o.namespace, manifest); err != nil {
				return fmt.Errorf("apply object storage secret manifest: %w", err)
			}
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if step == "installing_object_storage_buckets" {
		if !looksLikeKubeconfig(o.kubeconfig) {
			o.markCompleted(stackID, order)
			return nil
		}
		if err := o.ensureGitLabObjectStorageBuckets(ctx, o.namespace); err != nil {
			return fmt.Errorf("ensure gitlab object storage buckets: %w", err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if step == "installing_database_connection_check" {
		if !looksLikeKubeconfig(o.kubeconfig) {
			o.markCompleted(stackID, order)
			return nil
		}
		if err := o.ensureGitLabDatabaseConnectivity(ctx, o.namespace); err != nil {
			return fmt.Errorf("ensure gitlab database connectivity: %w", err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	// provisioning_secrets / provisioning_sso 는 차트가 없는 단계다.
	// chartSpecForStep 아래에 두면 spec 조회에 실패해 "unknown step" 으로
	// 떨어지므로 그 앞에서 처리한다.
	if step == "provisioning_secrets" {
		// 시크릿을 생성해 OpenBao 에 기록하고 ESO 가 K8s Secret 을 만들 때까지 기다린다.
		// 후속 차트가 existingSecret 을 참조하므로 이 대기가 없으면 파드가 기동에 실패한다.
		if err := o.runSecretProvisioning(ctx, o.namespace); err != nil {
			return fmt.Errorf("시크릿 프로비저닝 실패: %w", err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if step == "provisioning_gitea" {
		// Gitea 는 OAuth 소스를 Helm values 로 받지 않는다. CLI 로만 등록할 수
		// 있어 기동된 파드에 exec 한다.
		if err := o.ensureGiteaSSOProvisioned(ctx, o.namespace); err != nil {
			return fmt.Errorf("gitea SSO 프로비저닝 실패: %w", err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if step == "provisioning_harbor" {
		if !looksLikeKubeconfig(o.kubeconfig) {
			o.markCompleted(stackID, order)
			return nil
		}
		if err := o.ensureHarborProvisioned(ctx, o.namespace); err != nil {
			return fmt.Errorf("harbor 프로비저닝 실패: %w", err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if step == "provisioning_nexus" {
		if !looksLikeKubeconfig(o.kubeconfig) {
			o.markCompleted(stackID, order)
			return nil
		}
		if err := o.ensureNexusProvisioned(ctx, o.namespace); err != nil {
			return fmt.Errorf("nexus 프로비저닝 실패: %w", err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if step == "provisioning_sso" {
		// OIDC 클라이언트를 미리 만들어 둔다. 이를 소비하는 GitLab/Argo CD/
		// Grafana 설치보다 앞서야 한다.
		if err := o.runSSOProvisioning(ctx, o.namespace); err != nil {
			return fmt.Errorf("SSO 프로비저닝 실패: %w", err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	spec, ok := o.chartSpecForStep(step)
	if !ok {
		return fmt.Errorf("unknown step %q", step)
	}
	spec = o.resolveChartSpecForStep(step, spec)

	namespace := o.namespace
	if spec.Namespace != "" {
		namespace = spec.Namespace
	}
	if step == stepInstallingCertManager && looksLikeKubeconfig(o.kubeconfig) {
		if releaseNamespace, err := o.detectCertManagerReleaseNamespaceFromCRD(ctx); err == nil && strings.TrimSpace(releaseNamespace) != "" {
			namespace = strings.TrimSpace(releaseNamespace)
		}
	}

	spec = o.resolveChartSpecForStep(step, spec)
	manifest, hasManifest := o.stepManifestForStep(step)
	if step == "installing_gateway" && !hasManifest && looksLikeKubeconfig(o.kubeconfig) {
		if generated := o.defaultGatewayBundleManifest(namespace); strings.TrimSpace(generated) != "" {
			manifest = generated
			hasManifest = true
		}
	}

	if step == "installing_external_secrets" {
		// ESO 는 CRD 소유권 보정이 필요해 전용 경로로 설치한다.
		if err := o.InstallExternalSecrets(ctx, namespace); err != nil {
			return fmt.Errorf("external-secrets 설치 실패: %w", err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if hasManifest && step != "installing_gateway" {
		if err := o.applyManifest(ctx, namespace, manifest); err != nil {
			return fmt.Errorf("apply yaml manifest for step %s: %w", step, err)
		}
		o.markCompleted(stackID, order)
		return nil
	}

	if strings.TrimSpace(spec.ChartName) == "" {
		o.markCompleted(stackID, order)
		return nil
	}
	if step == stepInstallingCertManager {
		// 첫 스텝에서 StorageClass 를 검증해 PVC 실패를 빠르게 드러낸다.
		if err := o.ensureStorageClassExists(ctx); err != nil {
			return fmt.Errorf("storageclass preflight 실패: %w", err)
		}
		installed, checkErr := checkExistingCertManagerInstallation(ctx, o)
		if checkErr != nil {
			return fmt.Errorf("detect existing cert-manager installation: %w", checkErr)
		}
		if installed {
			slog.Info("reusing existing cert-manager installation", "namespace", namespace)
			if err := waitForCertManagerInstallation(ctx, o); err != nil {
				return fmt.Errorf("wait for cert-manager readiness: %w", err)
			}
			if err := bootstrapInternalCAInstallation(ctx, o, namespace); err != nil {
				return fmt.Errorf("bootstrap internal ca: %w", err)
			}
			o.markCompleted(stackID, order)
			return nil
		}
	}
	if step == "installing_metrics_server" {
		installed, checkErr := checkExistingMetricsServerInstallation(ctx, o)
		if checkErr != nil {
			return fmt.Errorf("detect existing metrics-server installation: %w", checkErr)
		}
		if installed {
			slog.Info("reusing existing metrics-server installation", "namespace", namespace)
			o.markCompleted(stackID, order)
			return nil
		}
	}

	if step == "installing_gateway" && looksLikeKubeconfig(o.kubeconfig) {
		if err := o.ensureGatewayAPICRDs(ctx); err != nil {
			return fmt.Errorf("ensure gateway api crds: %w", err)
		}
	}
	releaseName := o.releaseNameForSpec(spec)
	values := o.valuesForStep(step, spec)
	if step == stepInstallingRunner {
		if looksLikeKubeconfig(o.kubeconfig) {
			runnerToken, tokenErr := o.discoverGitLabRunnerRegistrationToken(ctx, namespace)
			if tokenErr != nil {
				// 스킵하지 않는다 — 러너 없이 completed 로 끝나면 CI 가 한 건도
				// 돌지 않는 스택이 성공으로 보고된다.
				return wrapRunnerTokenDiscoveryError(tokenErr)
			}
			values = mergeMaps(values, map[string]any{
				"runnerToken": runnerToken,
			})
		} else {
			slog.Warn("kubeconfig unavailable; skipping gitlab runner token discovery", "namespace", namespace)
		}
	}

	// 이전 설치가 남긴 cluster-scoped 리소스(CRD/RBAC/webhook)의 소유권을 먼저
	// 인수한다. 옛 네임스페이스가 주석에 박혀 있으면 Helm 이 ownership 충돌로
	// 설치를 거부한다. 살아 있는 다른 릴리스 소유는 건드리지 않는다.
	o.adoptClusterScopedResources(ctx, releaseName, namespace)

	result, err := o.installer.Install(ctx, port.HelmInstallRequest{
		ReleaseName: releaseName,
		ChartName:   spec.ChartName,
		RepoURL:     spec.RepoURL,
		Version:     spec.Version,
		Namespace:   namespace,
		Values:      values,
		Wait:        boolPtr(spec.Wait),
	})
	if err != nil && step == "installing_gateway" && strings.Contains(strings.ToLower(err.Error()), "missing registry client") {
		if fallbackErr := installOCIChartWithHelmCLI(ctx, o.kubeconfig, releaseName, spec.ChartName, namespace, spec.Version, boolPtr(spec.Wait), "", false); fallbackErr == nil {
			result = &port.HelmInstallResult{ReleaseName: releaseName, Namespace: namespace, Status: "deployed"}
			err = nil
		} else {
			err = fmt.Errorf("%w; fallback helm cli install failed: %v", err, fallbackErr)
		}
	}
	if err != nil {
		return fmt.Errorf("install step %s: %w", step, err)
	}
	if result != nil {
		o.rollback.Push(result.ReleaseName)
	}
	if step == stepInstallingCertManager {
		if err := waitForCertManagerInstallation(ctx, o); err != nil {
			return fmt.Errorf("wait for cert-manager readiness: %w", err)
		}
		if err := bootstrapInternalCAInstallation(ctx, o, namespace); err != nil {
			return fmt.Errorf("bootstrap internal ca: %w", err)
		}
	}
	// PostgreSQL 을 세운 직후 앱 사용자의 비밀번호를 Secret 값과 맞춘다.
	//
	// 차트는 데이터 디렉터리가 비어 있을 때만 사용자를 만든다. 볼륨이 남은 채
	// 다시 설치하거나 금고가 새로 초기화되면 DB 안의 비밀번호와 Secret 이 조용히
	// 갈라지고, 설치는 멈추지 않은 채 여섯 단계쯤 뒤 Gitea 가 28P01 로 기동하지
	// 못하는 모습으로 드러난다(2026-08-20 운영에서 실측).
	if step == "installing_postgresql" && looksLikeKubeconfig(o.kubeconfig) {
		if err := o.ensureProvisionedPostgresRolePassword(ctx, namespace); err != nil {
			return err
		}
	}
	if step == "installing_openbao" {
		// 차트 설치 직후 금고를 초기화한다. Job 이 멱등하므로 재시도해도 안전하다.
		if err := o.runOpenBaoInit(ctx, namespace); err != nil {
			return fmt.Errorf("openbao 초기화 실패: %w", err)
		}
		// 이어서 시크릿 엔진·Kubernetes Auth·정책·role 을 구성한다.
		if err := o.runOpenBaoBootstrap(ctx, namespace); err != nil {
			return fmt.Errorf("openbao 부트스트랩 실패: %w", err)
		}
	}
	if step == "installing_gateway" {
		if looksLikeKubeconfig(o.kubeconfig) {
			if err := o.reconcileGatewayDataPlaneTLSSecret(ctx, namespace); err != nil {
				return fmt.Errorf("reconcile gateway data-plane tls secret: %w", err)
			}
			if err := o.applyManifest(ctx, namespace, defaultEnvoyGatewayClassManifest()); err != nil {
				return fmt.Errorf("apply default gatewayclass manifest: %w", err)
			}
		}
	}
	if step == "installing_route" && looksLikeKubeconfig(o.kubeconfig) {
		if err := o.tryReconcileGatewayDataPlaneTLSSecret(ctx, namespace); err != nil {
			return fmt.Errorf("reconcile gateway data-plane tls secret: %w", err)
		}
	}
	if step == "installing_gateway" && hasManifest {
		manifestNamespace := o.namespace
		if strings.TrimSpace(manifestNamespace) == "" {
			manifestNamespace = namespace
		}
		if looksLikeKubeconfig(o.kubeconfig) {
			filteredManifest, skippedBackendTLSPolicy, filterErr := o.filterOptionalGatewayPolicies(ctx, manifest)
			if filterErr != nil {
				return fmt.Errorf("filter optional gateway policies: %w", filterErr)
			}
			if skippedBackendTLSPolicy {
				slog.Warn("skipping BackendTLSPolicy manifest because the CRD is unavailable", "namespace", manifestNamespace)
			}
			manifest = filteredManifest
			normalizedManifest, normalizedAny, normalizeErr := normalizeGatewayBackendServiceAliases(manifest)
			if normalizeErr != nil {
				return fmt.Errorf("normalize gateway backend aliases: %w", normalizeErr)
			}
			if normalizedAny {
				slog.Warn("normalized gateway backend service aliases to installed service names", "namespace", manifestNamespace)
			}
			manifest = normalizedManifest
		}
		if err := o.applyManifest(ctx, manifestNamespace, manifest); err != nil {
			return fmt.Errorf("apply yaml manifest for step %s: %w", step, err)
		}
		// 밖에서 들어오는 길을 함께 놓는다. LoadBalancer 가 없는 클러스터에서는
		// 게이트웨이가 외부 IP 를 못 받아, 이 규칙이 없으면 요청이 ingress 까지만
		// 오고 404 로 끝난다. 실패해도 설치를 멈추지 않는다 — 클러스터 안에서는
		// 스택이 정상 동작하고, 이 배선은 나중에 손으로도 걸 수 있다.
		if looksLikeKubeconfig(o.kubeconfig) {
			accessDomain, stackLabel := o.stackAccessIdentity()
			if err := o.ensureGatewayBridgeIngress(ctx, manifestNamespace, accessDomain, stackLabel); err != nil {
				slog.Warn("gateway bridge ingress not created", "namespace", manifestNamespace, "error", err)
			}
		}
	}
	o.markCompleted(stackID, order)
	return nil
}

func looksLikeKubeconfig(kubeconfig []byte) bool {
	if len(kubeconfig) == 0 {
		return false
	}
	text := string(kubeconfig)
	return strings.Contains(text, "apiVersion:") && strings.Contains(text, "clusters:")
}

func isReleaseNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "release: not found") || strings.Contains(msg, "release not loaded")
}

func (o *Orchestrator) isStepEnabled(step string) bool {
	if step == "integration_check" {
		return true
	}
	if step == "installing_object_storage_buckets" && !looksLikeKubeconfig(o.kubeconfig) {
		return false
	}
	if o.sharedClusterScoped && isSharedClusterScopedStep(step) {
		return false
	}

	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil {
		// 시크릿 평면은 설정과 무관하게 항상 켜진다 — 차트가 참조하는
		// existingSecret 을 만드는 유일한 경로이기 때문이다.
		// 반면 선택형 단계는 설정이 없으면 끈다. 켜 두면 아무도 고르지 않은
		// 도구를 설치하게 되고, 순서 검증도 그 단계를 기다리다 멈춘다.
		return !isOptInStep(step)
	}

	enabledFn, ok := o.stepConfigEnabled[step]
	if !ok {
		return true
	}

	return enabledFn(*cfg)
}

// isOptInStep 은 스택 설정에서 명시적으로 골라야만 도는 단계인지 본다.
//
// 설정을 모를 때의 기본값을 정하는 데 쓴다. 여기 없는 단계는 설정이 없으면
// 켜진 것으로 본다 — 시크릿 평면처럼 항상 필요한 단계가 그렇다.
func isOptInStep(step string) bool {
	switch step {
	case "provisioning_sso", "installing_harbor", "provisioning_harbor", "provisioning_gitea", "installing_nexus", "provisioning_nexus":
		return true
	// Gitea 는 명시적으로 골라야 선다. 소스 저장소 슬롯의 기본값은 GitLab 이므로
	// (isGitLabSourceRepositorySelection 이 빈 이름에 true 를 돌려준다) 여기 없으면
	// 설정을 모를 때 GitLab 과 Gitea 가 함께 서 버린다.
	case "installing_gitea", "installing_jenkins":
		return true
	// CRD 는 Prometheus 를 고른 스택에서만 필요하다. 여기 없으면 설정을 모를 때
	// 켜진 것으로 보고, 순서 검증이 오지 않을 단계를 기다리다 멈춘다.
	case stepInstallingPrometheusCRDs:
		return true
	}
	return false
}

func isSharedClusterScopedStep(step string) bool {
	return step == stepInstallingCertManager || step == "installing_metrics_server"
}

func (o *Orchestrator) ensureOrder(stackID, step string, order int) error {
	if stackID == "" {
		return nil
	}

	o.mu.Lock()
	current := o.progress[stackID]
	if _, ok := o.progress[stackID]; !ok {
		current = -1
	}
	o.mu.Unlock()

	for current+1 < order {
		nextIdx := current + 1
		if nextIdx < 0 || nextIdx >= len(o.orderedStep) {
			break
		}
		nextStep := o.orderedStep[nextIdx]
		if o.isStepEnabled(nextStep) {
			break
		}
		current = nextIdx
	}

	if order != current+1 {
		expectedIdx := current + 1
		expected := ""
		if expectedIdx >= 0 && expectedIdx < len(o.orderedStep) {
			expected = o.orderedStep[expectedIdx]
		}
		return fmt.Errorf("out of order step %q for stack %s: expected %q", step, stackID, expected)
	}

	o.mu.Lock()
	o.progress[stackID] = current
	o.mu.Unlock()
	return nil
}

func (o *Orchestrator) markCompleted(stackID string, order int) {
	if stackID == "" {
		return
	}
	o.mu.Lock()
	o.progress[stackID] = order
	o.mu.Unlock()
}
