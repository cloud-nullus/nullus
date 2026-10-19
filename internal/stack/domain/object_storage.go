package domain

import (
	"fmt"
	"strings"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 스택 오브젝트 스토리지 — S3 호환 구현체를 갈아 끼울 수 있게 둔다 (nullus-plan#88).
//
// MinIO 가 오픈소스 배포를 닫고 상용(AIStor)으로 돌아서, 한 구현체에 묶여 있으면
// 그 구현체가 막히는 날 스택 설치 전체가 멈춘다. 실제로 quay.io 마저 익명 pull 을
// 막아 installing_minio 가 처음부터 서지 못했다(shareddomain.MinIOServerImage 주석).
//
// 그래서 소비자는 구현체를 모른다. GitLab object_store · Container Registry ·
// 버킷 부트스트랩 · 연결정보가 보는 것은 S3 HTTP 계약 — 엔드포인트, 액세스 키,
// 시크릿 키, 리전, path-style — 다섯 가지뿐이다. 버킷도 벤더 CLI(mc) 대신 S3 의
// CreateBucket 을 HTTP 로 직접 부른다. 구현체마다 다른 것(설치 단계·릴리스·
// Service·자격 Secret 의 키 이름·금고 경로)은 아래 표 하나에 모은다.
//
// 구현체를 더하려면 이 표에 한 줄을 넣고, 그 단계의 차트 스펙과 values 를 두면 된다.

// ObjectStorageBackend 는 스택 안에 세우는 S3 호환 오브젝트 스토리지 구현체 하나다.
type ObjectStorageBackend struct {
	// Tool 은 카탈로그 이름을 정규화한 값이다. storage_backend 선택과 맞춰 본다.
	Tool string
	// DisplayName 은 화면·로그에 쓰는 이름이다.
	DisplayName string
	// Step 은 이 구현체를 세우는 설치 단계다.
	Step string
	// ReleaseName 은 Helm 릴리스명이다. 파드 이름 접두사이기도 하다.
	ReleaseName string
	// ServiceName / ServicePort 는 S3 API 가 서는 클러스터 내부 주소다.
	ServiceName string
	ServicePort int

	// CredentialSecret 은 provisioning_secrets 가 금고에서 옮겨 오는 자격 Secret 이다.
	// 차트가 existingSecret 으로 읽으므로 키 이름은 차트가 정한다.
	CredentialSecret string
	AccessKeyKey     string
	SecretKeyKey     string

	// VaultAccessKeyPath / VaultSecretKeyPath 는 스택 금고 접두사 뒤의 경로다.
	// 자격의 원천이다 — GitLab 의 연결 Secret 도 같은 경로에서 만든다.
	VaultAccessKeyPath string
	VaultSecretKeyPath string
}

// Endpoint 는 같은 클러스터 안의 소비자가 붙을 S3 엔드포인트다.
func (b ObjectStorageBackend) Endpoint(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = DefaultStackNamespace
	}
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", b.ServiceName, namespace, b.ServicePort)
}

const (
	// ObjectStorageAccessKey 는 스택이 쓰는 S3 액세스 키(사용자명)다. 비밀이 아니다.
	// MinIO 시절의 root 사용자명과 같은 값이라, 기존 스택의 자격이 그대로 이어진다.
	ObjectStorageAccessKey = MinIORootUser

	// ObjectStorageRegion 은 서명(SigV4)에 넣는 리전이다. 자체 호스팅 구현체는
	// 리전을 따지지 않지만 서명 계산에는 들어가므로 소비자 모두가 같은 값을 써야 한다.
	ObjectStorageRegion = "us-east-1"

	ObjectStorageToolMinIO     = "minio"
	ObjectStorageToolSeaweedFS = "seaweedfs"
)

// SeaweedFS 의 리소스 이름. Apache-2.0 이고 S3 게이트웨이를 내장한다.
//
// 차트의 all-in-one 모드로 세운다 — master·volume·filer·S3 게이트웨이가 파드
// 하나에 뜬다. 스택이 MinIO 를 standalone(단일 파드)으로 세우던 것과 같은 자리다.
const (
	SeaweedFSReleaseName = shareddomain.SeaweedFSReleaseName
	// SeaweedFSServiceName 은 all-in-one Service 다. 차트가 <fullname>-all-in-one 으로
	// 짓고, 릴리스명에 차트 이름이 들어 있으면 fullname 이 릴리스명 그대로다.
	SeaweedFSServiceName = SeaweedFSReleaseName + "-all-in-one"
	// SeaweedFSS3Port 는 S3 게이트웨이 포트다. 차트 기본 s3.port 와 같다.
	SeaweedFSS3Port = 8333
	// SeaweedFSMetricsPort 는 Prometheus 메트릭 포트다. 차트 기본 allInOne.metricsPort.
	SeaweedFSMetricsPort = 9324

	// ProvisionedSeaweedFSSecret 은 S3 신원(identity) 설정을 담는다.
	ProvisionedSeaweedFSSecret = "nullus-seaweedfs-credentials" // #nosec G101 -- Secret 리소스 이름
	SeaweedFSAccessKeyKey      = "accessKey"
	SeaweedFSSecretKeyKey      = "secretKey" // #nosec G101 -- Secret 안의 키 이름
	// SeaweedFSS3ConfigKey 는 차트의 existingConfigSecret 이 찾는 키다. S3 게이트웨이가
	// -s3.config 로 읽는 신원 JSON 이 들어 있다.
	SeaweedFSS3ConfigKey = "seaweedfs_s3_config"
	// SeaweedFSSFTPSecret 은 차트가 all-in-one 이면 SFTP 를 끄더라도 만드는 Secret 이다.
	// helm.sh/resource-policy: keep 이라 helm uninstall 이 남긴다 — 스택 삭제가 이름으로 지운다.
	SeaweedFSSFTPSecret = SeaweedFSReleaseName + "-sftp-secret" // #nosec G101 -- Secret 리소스 이름

	// 버전(SeaweedFSChartVersion / SeaweedFSAppVersion)은 다른 도구처럼 connection.go 가 소유한다.
	SeaweedFSChartRepo = "https://seaweedfs.github.io/seaweedfs/helm"
)

// MinIOObjectStorage 는 MinIO(현재 이미지는 커뮤니티 포크 SILO)다.
//
// 이름·키·금고 경로는 이 표가 생기기 전 그대로다 — 이미 설치된 스택의 금고와
// Secret 을 그대로 읽어야 하므로 바꾸지 않는다.
var MinIOObjectStorage = ObjectStorageBackend{
	Tool:               ObjectStorageToolMinIO,
	DisplayName:        "MinIO",
	Step:               "installing_minio",
	ReleaseName:        MinIOServiceName,
	ServiceName:        MinIOServiceName,
	ServicePort:        MinIOServicePort,
	CredentialSecret:   ProvisionedMinIOSecret,
	AccessKeyKey:       MinIOUserKey,
	SecretKeyKey:       MinIOPasswordKey,
	VaultAccessKeyPath: "artifacts/minio/root-user",
	VaultSecretKeyPath: "artifacts/minio/root-password",
}

// SeaweedFSObjectStorage 는 SeaweedFS 다.
var SeaweedFSObjectStorage = ObjectStorageBackend{
	Tool:               ObjectStorageToolSeaweedFS,
	DisplayName:        "SeaweedFS",
	Step:               "installing_seaweedfs",
	ReleaseName:        SeaweedFSReleaseName,
	ServiceName:        SeaweedFSServiceName,
	ServicePort:        SeaweedFSS3Port,
	CredentialSecret:   ProvisionedSeaweedFSSecret,
	AccessKeyKey:       SeaweedFSAccessKeyKey,
	SecretKeyKey:       SeaweedFSSecretKeyKey,
	VaultAccessKeyPath: "artifacts/seaweedfs/access-key",
	VaultSecretKeyPath: "artifacts/seaweedfs/secret-key",
}

// ObjectStorageBackends 는 스택이 세울 수 있는 구현체 전부다.
func ObjectStorageBackends() []ObjectStorageBackend {
	return []ObjectStorageBackend{MinIOObjectStorage, SeaweedFSObjectStorage}
}

// ObjectStorageBackendFor 는 storage_backend 선택에 맞는 구현체를 돌려준다.
//
// 고른 이름이 구현체를 가리키면 그것이다. 그 밖의 이름(빈 이름 포함)은 MinIO 다 —
// 이 슬롯은 오래 이름을 보지 않고 무엇이든 MinIO 를 세웠고, 이름 없이 저장된
// 스택이 재시도에서 다른 구현체를 깔면 이미 쌓인 오브젝트를 잃는다. 이름으로
// 고르는 것은 표에 있는 구현체로만 넓힌다.
func ObjectStorageBackendFor(sel ToolSelection) (ObjectStorageBackend, bool) {
	// version=external 을 따로 보지 않는다. 다른 슬롯과 달리 이 슬롯의 외부 연결은
	// storage.object_storage(existing-connect)가 맡고, 지금까지 이 선택은 언제나
	// 클러스터 안에 MinIO 를 세웠다 — 바꾸면 GitLab 이 볼 S3 가 사라진다.
	if !sel.Enabled {
		return ObjectStorageBackend{}, false
	}
	if backend, ok := ObjectStorageBackendNamed(sel.Name); ok {
		return backend, true
	}
	return MinIOObjectStorage, true
}

// ObjectStorageBackendNamed 는 이름이 가리키는 구현체를 찾는다. 빈 이름은 MinIO 다.
//
// 호환성 매트릭스가 이것으로 슬롯을 대조에서 뺀다 — 구현체는 S3 계약으로 갈아
// 끼우므로 도구 조합을 가르지 않는다. 표에 없는 이름(s3·gcs 처럼 클러스터 밖
// 제공자)은 찾지 못한 것으로 돌려 지금처럼 매트릭스 검사를 받게 한다.
func ObjectStorageBackendNamed(name string) (ObjectStorageBackend, bool) {
	switch normalizeObjectStorageTool(name) {
	case "", "minio", "silo":
		return MinIOObjectStorage, true
	case "seaweedfs", "seaweed-fs", "seaweed":
		return SeaweedFSObjectStorage, true
	}
	return ObjectStorageBackend{}, false
}

// ObjectStorageBackendForStep 은 설치 단계로 구현체를 찾는다.
func ObjectStorageBackendForStep(step string) (ObjectStorageBackend, bool) {
	step = strings.TrimSpace(step)
	for _, backend := range ObjectStorageBackends() {
		if backend.Step == step {
			return backend, true
		}
	}
	return ObjectStorageBackend{}, false
}

func normalizeObjectStorageTool(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	// 카탈로그는 사람이 읽는 이름을 준다("MinIO (SILO)"). 괄호 뒤는 부연이다.
	if idx := strings.Index(normalized, "("); idx >= 0 {
		normalized = strings.TrimSpace(normalized[:idx])
	}
	return strings.NewReplacer(" ", "-", "_", "-").Replace(normalized)
}
