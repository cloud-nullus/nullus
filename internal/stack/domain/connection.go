package domain

import shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"

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
