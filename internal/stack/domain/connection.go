package domain

import shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"

// DefaultStackNamespace 는 스택 네임스페이스가 정해지지 않았을 때의 기본값이다.
// 설치 경로와 안내 경로가 같은 값을 봐야 하므로 domain 이 소유한다.
const DefaultStackNamespace = shareddomain.DefaultStackNamespace

// 설치가 만들어내는 리소스 이름들. 여기가 단일 출처다.
//
// 이 값들은 두 곳에서 필요하다 — 설치 경로(Helm values 가 existingSecret 으로
// 참조)와 조회 경로(사용자에게 접속 방법을 안내). 양쪽에 각각 적어 두면 한쪽만
// 바뀌었을 때 조용히 어긋나고, 화면은 존재하지 않는 이름을 계속 안내한다.
// 실제로 UI 가 `-n nullus get secret argo-cd-argocd-initial-admin-secret` 을
// 안내하다가 NotFound 를 내던 사고가 있었다.
const (
	// ProvisionedPostgresSecret 은 설치가 만드는 PostgreSQL 자격증명이다.
	ProvisionedPostgresSecret = "nullus-postgresql-credentials"
	// ProvisionedMinIOSecret 은 설치가 만드는 MinIO 자격증명이다.
	ProvisionedMinIOSecret = "nullus-minio-credentials"
	// ProvisionedObjectStorageSecret 은 GitLab 의 object_store 연결 정보다.
	ProvisionedObjectStorageSecret = "nullus-object-storage"
	// ProvisionedRegistryStorageSecret 은 Container Registry 의 S3 설정이다.
	ProvisionedRegistryStorageSecret = "nullus-registry-storage" // #nosec G101 -- Secret 리소스 이름

	// MinIORootUser 는 비밀이 아니지만 차트의 existingSecret 이 같은 Secret
	// 안에서 요구하므로 함께 프로비저닝한다.
	MinIORootUser = "nullus-admin"

	// PostgresAppUser / PostgresAppDatabase 는 GitLab 이 쓰는 계정과 DB 다.
	PostgresAppUser     = "gitlab"
	PostgresAppDatabase = "gitlabhq_production"

	// PostgresPasswordKey 는 앱 계정 비밀번호 키다.
	// postgres-password 는 superuser 용이라 앱 접속에는 쓰지 않는다.
	PostgresPasswordKey = "password"
	// MinIOUserKey / MinIOPasswordKey 는 charts.min.io 의 existingSecret 이
	// 읽는 키 이름이다. 프로비저닝과 안내가 같은 키를 봐야 한다.
	MinIOUserKey     = "rootUser"
	MinIOPasswordKey = "rootPassword"

	// PostgresServiceName / MinIOServiceName 은 Helm 릴리스명에서 온다.
	// 네임스페이스와 무관하므로 네임스페이스로 조립하면 안 된다.
	// 릴리스명 자체는 admin 의 시크릿 회전도 봐야 하므로 shared 가 소유한다.
	PostgresServiceName = shareddomain.PostgresReleaseName
	PostgresServicePort = 5432
	MinIOServiceName    = shareddomain.MinIOReleaseName
	MinIOServicePort    = 9000

	// MinIO 콘솔(웹 UI)은 API 와 다른 Service·포트를 쓴다.
	MinIOConsoleServiceName = MinIOServiceName + "-console"
	MinIOConsoleServicePort = 9001

	// GitLabArtifactsBucket 은 설치가 만드는 아티팩트 버킷이다.
	// 연결정보가 오브젝트 스토리지의 resource_name 으로 안내한다.
	GitLabArtifactsBucket = "gitlab-artifacts"
)

// 독립 설치형 아티팩트 도구(Harbor / Nexus)의 리소스 이름.
//
// GitLab 내장 레지스트리와 달리 이 둘은 자체 릴리스로 선다. 서비스 이름은
// 차트가 릴리스명에서 유도하므로 릴리스명이 곧 in-cluster 주소가 된다.
const (
	// HarborServiceName 은 Harbor 진입 Service 다. expose.type=clusterIP 로
	// 설치하면 차트가 릴리스명과 같은 이름의 Service 를 만들고, 여기로 들어온
	// 요청을 portal/core 로 갈라 보낸다. 이미지 push/pull 도 이 주소를 쓴다.
	HarborReleaseName  = "harbor"
	HarborServiceName  = HarborReleaseName
	HarborServicePort  = 80
	HarborAdminUser    = "admin"
	HarborAdminSecret  = "nullus-harbor-credentials" // #nosec G101 -- Secret 리소스 이름
	HarborAdminPassKey = "HARBOR_ADMIN_PASSWORD"     // 차트의 existingSecretAdminPasswordKey 기본값

	// NexusServiceName 은 Nexus 진입 Service 다. 차트 기본 이름은
	// {release}-nexus-repository-manager 라 길어지므로 fullnameOverride 로 맞춘다.
	NexusReleaseName = "nexus"
	NexusServiceName = NexusReleaseName
	// NexusServicePort 는 웹 UI 와 Maven/npm 저장소가 함께 쓰는 포트다.
	NexusServicePort = 8081
	// NexusDockerServicePort 는 Docker 레지스트리 커넥터 포트다. Nexus 는
	// Docker 레지스트리를 별도 HTTP 커넥터로 노출하며 기본값이 없다 —
	// provisioning_nexus 단계가 이 포트로 커넥터를 만들고 Service 를 덧붙인다.
	NexusDockerServicePort = 8082
	NexusAdminUser         = "admin"
	NexusAdminSecret       = "nullus-nexus-credentials" // #nosec G101 -- Secret 리소스 이름
	NexusAdminPassKey      = "password"

	// Nexus 가 만드는 저장소 이름. CI 가 이미지를 올리고 빌드가 패키지를
	// 받아가는 곳이라 설치와 파이프라인이 같은 이름을 봐야 한다.
	NexusDockerRepository = "docker-hosted"
	NexusMavenRepository  = "maven-hosted"
	NexusNpmRepository    = "npm-hosted"

	// GiteaReleaseName 은 Gitea 릴리스명이다. 릴리스명이 곧 파드 이름 접두사이자
	// Service 이름의 뿌리가 되므로, 워크로드 조회와 in-cluster 주소가 같은 값을 본다.
	GiteaReleaseName = "gitea"
	// GiteaHTTPServiceName 은 Gitea 차트가 만드는 HTTP 진입 Service 다.
	// 차트가 {release}-http 로 이름을 유도한다 — CI/CD 모듈이 이 주소로 API 를 부른다.
	GiteaHTTPServiceName = GiteaReleaseName + "-http"

	// 접속 도메인용 와일드카드 TLS. 설치가 게이트웨이 HTTPS 리스너를 위해 만들고
	// 삭제가 이름으로 지운다 — 그 매니페스트의 라벨은 stack.Name 이 아니라 접속
	// 도메인에서 오므로 라벨 셀렉터로는 잡히지 않는다.
	// usecase 가 adapter 를 import 하면 레이어 방향이 뒤집히므로 여기 둔다.
	AccessDomainCertName      = "nullus-access-domain-cert"
	AccessDomainTLSSecretName = "nullus-access-domain-tls"
	GiteaServicePort          = 3000
	GiteaAdminUser            = "gitea_admin"
	// GiteaAdminSecret 은 gitea.admin.existingSecret 이 참조하는 Secret 이다.
	// 차트는 이 안에서 username / password 키를 읽는다
	// (templates/gitea/deployment.yaml — GITEA_ADMIN_USERNAME / GITEA_ADMIN_PASSWORD).
	GiteaAdminSecret      = "nullus-gitea-credentials" // #nosec G101 -- Secret 리소스 이름
	GiteaAdminUserKey     = "username"
	GiteaAdminPasswordKey = "password"

	// JenkinsReleaseName 은 Jenkins 릴리스명이다. 파드 접두사이자 Service 이름의
	// 뿌리가 된다 — CI/CD 모듈이 이 주소로 job 을 만든다.
	JenkinsReleaseName = "jenkins"
	JenkinsServiceName = JenkinsReleaseName
	JenkinsServicePort = 8080
	JenkinsAdminUser   = "admin"
	// JenkinsAdminSecret 은 controller.admin.existingSecret 이 참조하는 Secret 이다.
	// 차트가 읽는 키 이름은 controller.admin.userKey / passwordKey 기본값과 같아야
	// 한다 — 틀리면 파드가 FailedMount 로 영원히 기동하지 못한다.
	JenkinsAdminSecret      = "nullus-jenkins-credentials" // #nosec G101 -- Secret 리소스 이름
	JenkinsAdminUserKey     = "jenkins-admin-user"
	JenkinsAdminPasswordKey = "jenkins-admin-password"
)

// 설치가 사용하는 차트 버전.
//
// 화면은 호환성 매트릭스가 선언한 버전을 보여주고 설치는 이 값을 쓴다. 둘이
// 갈라지면 "안내된 버전"과 "설치된 버전"이 달라지므로 같은 값을 봐야 한다.
// (고정: TestChartVersionsMatchCompatibilityMatrix)
// AppVersion 은 그 차트가 실제로 세우는 제품 버전이다. 차트의 appVersion 필드를
// 그대로 옮기지 않은 곳이 하나 있다 — 아래 Prometheus 주석 참고.
const (
	HarborChartVersion = "1.15.0"
	HarborAppVersion   = "2.11.0"
	NexusChartVersion  = "64.2.0"
	NexusAppVersion    = "3.64.0"

	// GitLab 차트 하나가 소스 저장소·CI·컨테이너 레지스트리를 함께 세운다.
	// 세 항목이 같은 버전을 봐야 한다.
	GitLabChartVersion = "8.7.2"
	GitLabAppVersion   = "v17.7.0"

	// Gitea 는 소스 저장소만 담당한다 — GitLab 과 달리 CI 도 레지스트리도 겸하지
	// 않으므로, 이 스택은 CI(Jenkins)와 레지스트리(Harbor)를 따로 세운다.
	GiteaChartVersion = "12.7.0"
	GiteaAppVersion   = "1.27.0"

	// Jenkins 차트 버전은 자유롭게 내릴 수 없다. Gitea multibranch 스캔에 쓰는
	// jenkinsci/gitea 플러그인이 Jenkins 2.528.3 이상을 요구하므로, appVersion 이
	// 그 하한을 넘는 차트여야 한다. 5.9.54 → 2.568.2 로 요건을 만족한다.
	// (고정: TestJenkinsAppVersion_SatisfiesGiteaPluginMinimum)
	JenkinsChartVersion = "5.9.54"
	JenkinsAppVersion   = "2.568.2"
	// JenkinsGiteaPluginMinAppVersion 은 gitea 플러그인이 요구하는 Jenkins 하한이다.
	JenkinsGiteaPluginMinAppVersion = "2.528.3"

	MinIOChartVersion = "5.4.0"
	MinIOAppVersion   = "RELEASE.2024-12-18T13-15-44Z"

	ArgoCDChartVersion = "7.7.16"
	ArgoCDAppVersion   = "v2.13.3"

	// kube-prometheus-stack 의 appVersion 은 prometheus-operator 버전(v0.80.0)이라
	// 그대로 쓰면 화면에 오퍼레이터 버전이 Prometheus 버전인 양 뜬다. 사용자가
	// 알아야 하는 것은 실제로 서는 Prometheus 서버 버전이므로 그것을 적는다.
	PrometheusChartVersion = "69.3.0"
	// PrometheusOperatorCRDsChartVersion 은 위 차트가 쓰는 operator(v0.80.0)와
	// 같은 버전의 CRD 를 담는다. ServiceMonitor / Probe 를 만드는 단계가
	// kube-prometheus-stack 설치보다 앞서므로 CRD 를 먼저 깐다.
	PrometheusOperatorCRDsChartVersion = "18.0.0"
	PrometheusAppVersion               = "v3.1.0"

	GrafanaChartVersion = "8.9.0"
	GrafanaAppVersion   = "11.5.1"

	// 추적 저장소. 차트 기본값으로 OTLP 수신기(4317/4318)가 켜져 있어
	// 수집기가 별도 설정 없이 곧바로 보낼 수 있다.
	TempoChartVersion = "1.18.1"
	TempoAppVersion   = "2.7.0"

	// OpenTelemetry Collector. 차트 0.75.0 의 appVersion 은 0.90.0 이다 —
	// 프론트 기본값(0.104.0)이 차트와 갈라져 있었으므로 차트를 기준으로 삼는다.
	OTelCollectorChartVersion = "0.75.0"
	OTelCollectorAppVersion   = "0.90.0"
)

// OpenTelemetry 수집기의 이름·주소 규칙은 shared 가 소유한다.
//
// cicd 는 배포하는 앱에 이 주소를 넣어 줘야 하는데, 모듈끼리 서로의 internal 을
// 참조할 수 없어 규칙을 shared 에 두고 양쪽이 같은 것을 본다. 여기서는 stack
// 코드가 계속 domain 이름으로 쓰도록 별칭만 남긴다.
const (
	OTelCollectorReleaseName  = shareddomain.OTelCollectorReleaseName
	OTelAgentReleaseName      = shareddomain.OTelAgentReleaseName
	OTelCollectorOTLPGRPCPort = shareddomain.OTelCollectorOTLPGRPCPort
	OTelCollectorOTLPHTTPPort = shareddomain.OTelCollectorOTLPHTTPPort
)

// OTelCollectorServiceName 은 수집기 Service 이름이다.
func OTelCollectorServiceName() string { return shareddomain.OTelCollectorServiceName() }

// OTelCollectorOTLPGRPCEndpoint 는 애플리케이션이 OTLP/gRPC 로 보낼 주소다.
func OTelCollectorOTLPGRPCEndpoint(namespace string) string {
	return shareddomain.OTelCollectorOTLPGRPCEndpoint(namespace)
}

// OTelCollectorOTLPHTTPEndpoint 는 애플리케이션이 OTLP/HTTP 로 보낼 주소다.
func OTelCollectorOTLPHTTPEndpoint(namespace string) string {
	return shareddomain.OTelCollectorOTLPHTTPEndpoint(namespace)
}

// OSS 도구의 초기 자격증명이 담기는 Secret.
//
// 차트마다 이름 규칙이 다르다 — Argo CD 는 Deployment/Service 에는 릴리스
// 접두사를 붙이지만 Secret 에는 붙이지 않는다.
const (
	ArgoCDAdminSecret   = "argocd-initial-admin-secret" // #nosec G101 -- Secret 리소스 이름
	ArgoCDAdminUser     = "admin"
	ArgoCDAdminPassKey  = "password"
	GitLabRootSecret    = "gitlab-gitlab-initial-root-password" // #nosec G101 -- Secret 리소스 이름
	GitLabRootUser      = "root"
	GitLabRootPassKey   = "password"
	GrafanaAdminUser    = "admin"
	OpenSearchAdminUser = "admin"
)

// StorageConnection 은 스토리지 접속 정보다.
type StorageConnection struct {
	Mode     string `json:"mode"`
	Engine   string `json:"engine"`
	Endpoint string `json:"endpoint"`
	// ResourceName 은 데이터베이스명 또는 버킷명이다.
	ResourceName string `json:"resource_name"`
	AuthID       string `json:"auth_id"`
	// SecretRef / SecretKey 는 비밀번호가 담긴 Secret 과 그 키다.
	SecretRef string `json:"secret_ref,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
}

// ToolCredential 은 OSS 도구 하나의 접속 안내다.
//
// SecretRef 가 비어 있으면 조회할 Secret 이 없다는 뜻이고, 그때는 Note 가
// 무엇을 해야 하는지 설명한다. 값을 모를 때 그럴듯한 명령을 지어내지 않는다.
type ToolCredential struct {
	Name      string `json:"name"`
	Username  string `json:"username,omitempty"`
	SecretRef string `json:"secret_ref,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
	Note      string `json:"note,omitempty"`
}

// ConnectionInfo 는 스택 접속에 필요한 정보 일체다.
type ConnectionInfo struct {
	StackID       string            `json:"stack_id"`
	Namespace     string            `json:"namespace"`
	AccessDomain  string            `json:"access_domain,omitempty"`
	Database      StorageConnection `json:"database"`
	ObjectStorage StorageConnection `json:"object_storage"`
	Tools         []ToolCredential  `json:"tools"`
}
