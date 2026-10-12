// Package port 는 backup 컨텍스트의 입출력 포트다.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §8 (nullus-plan#75)
//
// 이 모듈은 다른 모듈의 테이블을 직접 읽지 않는다. 참조 정합성 검사(§6.4)가
// token_sources 를 훑지만, admin 모듈의 공개 인터페이스(TokenSourceLister)를
// 통해서만 접근한다.
package port

import (
	"context"
	"io"

	"github.com/cloud-nullus/draft/internal/backup/domain"
)

// BackupRepository 는 실행 이력을 보관한다.
type BackupRepository interface {
	CreateRun(ctx context.Context, run *domain.BackupRun) error
	UpdateRun(ctx context.Context, run *domain.BackupRun) error
	GetRun(ctx context.Context, id string) (*domain.BackupRun, error)
	ListRuns(ctx context.Context, orgID string, limit int) ([]*domain.BackupRun, error)
	ListSummaries(ctx context.Context, orgID string) ([]domain.RunSummary, error)

	AddArtifact(ctx context.Context, a *domain.Artifact) error
	ListArtifacts(ctx context.Context, backupRunID string) ([]*domain.Artifact, error)
	DeleteArtifacts(ctx context.Context, backupRunID string) error

	CreateRestore(ctx context.Context, run *domain.RestoreRun) error
	UpdateRestore(ctx context.Context, run *domain.RestoreRun) error
	GetRestore(ctx context.Context, id string) (*domain.RestoreRun, error)
}

// OrganizationChecker 는 조직이 실제로 있는지 본다.
//
// 백업은 조직 단위로 보인다 — 목록은 로그인 사용자의 조직으로만 거른다. 호출자가
// 헤더에 적은 조직을 확인 없이 받으면, 없는 조직의 백업이 만들어지되 어떤 화면에도
// 보이지 않는다. 조직 테이블은 admin 모듈의 것이므로 그 공개 인터페이스를 감싼
// 어댑터(adapter/bridge)로 구현한다.
type OrganizationChecker interface {
	OrganizationExists(ctx context.Context, orgID string) (bool, error)
}

// DBDumper 는 PostgreSQL 논리 백업/복원을 수행한다.
//
// 통합(§1.2 결정 1) 이후에도 database 는 둘이므로 대상을 명시적으로 받는다.
// 복원은 반드시 database 단위여야 한다 — 인스턴스를 통째로 되돌리면 한쪽을
// 복원하다 다른 쪽을 날린다 (§6.6).
type DBDumper interface {
	// ServerVersion 은 pg_dump 클라이언트 선택에 쓴다 (§3.1 버전 함정).
	ServerVersion(ctx context.Context, target DBTarget) (string, error)
	Dump(ctx context.Context, target DBTarget, out io.Writer) (DumpResult, error)
	Restore(ctx context.Context, target DBTarget, in io.Reader) error
	SchemaState(ctx context.Context, target DBTarget) (domain.SchemaState, error)
}

type DBTarget struct {
	Component domain.Component
	Host      string
	Port      int
	Database  string
	User      string
	Password  string
	// PreserveTables 는 복원이 건드리지 않는 테이블이다(public 스키마). 덤프에는 담긴다 —
	// 플랫폼 DB 를 잃었을 때 손으로 되살릴 사본이다. 플랫폼 DB 의 백업 이력이 그렇다(domain.HistoryTables).
	PreserveTables []string
}

type DumpResult struct {
	ClientVersion string
	BytesWritten  int64
}

// KVExporter 는 OpenBao KV 를 논리 export/import 한다.
//
// raft snapshot API 는 이 구성에 없다 — 단일 replica 라 file 스토리지를 쓴다
// (§3.2). 그래서 경로를 순회해 값을 내보낸다.
type KVExporter interface {
	Export(ctx context.Context, stackID string, out io.Writer) (KVExportResult, error)
	Import(ctx context.Context, stackID string, in io.Reader) error
	// PathExists 는 참조 정합성 검사에 쓴다 (§6.4).
	PathExists(ctx context.Context, stackID, path string) (bool, error)
}

type KVExportResult struct {
	PathCount int
	Bytes     int64
}

// WorkloadScaler 는 정지/재개를 수행한다 (§3.4).
type WorkloadScaler interface {
	List(ctx context.Context, namespace string) ([]domain.Workload, error)
	Scale(ctx context.Context, w domain.QuiesceTarget, replicas int32) error
	// WaitStopped 는 볼륨이 실제로 언마운트될 때까지 기다린다. 파드가 남아
	// 있는 채로 복사하면 정지 백업의 의미가 사라진다.
	WaitStopped(ctx context.Context, namespace string, targets []domain.QuiesceTarget) error
}

// VolumeArchiver 는 PVC 를 아카이브하고 되돌린다.
//
// PVC 를 **열거해서** 처리한다 — 알려진 도구 목록을 순회하지 않는다.
// GitLab·Gitea 는 설치기가 볼륨 크기를 지정하지 않아 차트 기본값을 따르므로,
// 정적 목록은 반드시 뒤처진다 (§1.5).
type VolumeArchiver interface {
	ListPVCs(ctx context.Context, namespace string) ([]domain.VolumeSpec, error)
	Archive(ctx context.Context, namespace, pvc string, out io.Writer) (int64, error)
	Restore(ctx context.Context, namespace, pvc string, in io.Reader) error
	EnsurePVC(ctx context.Context, namespace string, spec domain.VolumeSpec, restoredFromBackupID string) error
}

// NamespaceEnsurer 는 복구할 네임스페이스를 보장한다.
//
// 새 클러스터나 지운 네임스페이스로 되돌리면 네임스페이스부터 없다. 리소스 덤프는
// 네임스페이스 안의 것만 담으므로 Namespace 자체는 복구가 만들어야 한다 — 그러지 않으면 첫
// PVC 생성이 "namespaces not found" 로 실패한다. 이미 있으면 그대로 둔다.
type NamespaceEnsurer interface {
	EnsureNamespace(ctx context.Context, namespace, restoredFromBackupID string) error
}

// GatewayAliasRepinner 는 복구 뒤 스택 게이트웨이의 지금 주소로 러너·Argo CD 의 hostAliases 를 맞춘다.
//
// 설치가 게이트웨이 ClusterIP 를 두 릴리스의 values 에 박는다. 복구는 Service 를 백업 때의 IP 로
// 되살리지만, 그 IP 를 쓸 수 없거나(다른 클러스터) 백업에 IP 가 없으면(옛 백업) 주소가 바뀐다.
// 스택 모듈이 구현한다 — 백업 모듈은 스택 내부를 모르므로 기본 타입만 주고받는다.
// kubeconfig 는 복구가 실제로 되돌린 클러스터다(full 복구는 DB 의 클러스터 정보까지 백업 시점으로
// 돌린다). 게이트웨이 주소와 다시 적용한 릴리스를 돌려준다.
type GatewayAliasRepinner interface {
	RepinGatewayAliases(ctx context.Context, stackID, namespace string, kubeconfig []byte) (gatewayIP string, updated []string, err error)
}

// HelmReleaseLister 는 네임스페이스에 설치된 차트의 버전을 읽는다.
//
// 복구가 다른 버전으로 재설치하면 도구들이 기동하며 스키마 마이그레이션을
// 돌린다 — 그러면 데이터는 백업 시점의 모습이 아니게 되고 되돌릴 수 없다.
// 그래서 "무엇이 이 데이터를 만들었는가" 를 매니페스트에 남긴다.
type HelmReleaseLister interface {
	ListHelmReleases(ctx context.Context, namespace string) ([]domain.HelmReleaseSpec, error)
}

// ResourceDumper 는 네임스페이스 리소스와 Helm 릴리스를 뜬다 (D1).
type ResourceDumper interface {
	Dump(ctx context.Context, namespace string, out io.Writer) (int64, error)
	Apply(ctx context.Context, namespace string, in io.Reader) error
}

// SharedResourceDumper 는 클러스터의 마지막 스택을 지우면 회수되는, Nullus 가 설치한 공용 자원을
// 뜨고 되살린다 — 설치 표시가 붙은 Gateway CRD·GatewayClass·내부 CA.
//
// 네임스페이스 덤프(ResourceDumper)는 네임스페이스 안의 것과 스택 릴리스가 소유한 클러스터 범위
// 리소스만 담는다. Gateway API CRD 는 kubectl 로, GatewayClass 는 create 로 만들어져 Helm 소유
// 어노테이션이 없고, 내부 CA 는 cert-manager 네임스페이스에 있다 — 둘 다 그 덤프 밖이다.
type SharedResourceDumper interface {
	// DumpShared 는 담을 것이 없으면 0 을 돌려주고 아무것도 쓰지 않는다.
	DumpShared(ctx context.Context, out io.Writer) (int64, error)
	// ApplyShared 는 공용 자원을 **없을 때만** 만든다. 다른 스택이 쓰는 지금의 CA 나 CRD 를
	// 백업 때의 것으로 덮으면 그 스택이 깨진다. 종류의 CRD 가 없으면(cert-manager 없음)
	// 무엇을 먼저 설치해야 하는지 말하고 오류를 돌려준다.
	ApplyShared(ctx context.Context, in io.Reader) error
	// ApplySharedSecrets 는 덤프 가운데 CRD 가 필요 없는 것 — 내부 CA Secret 과 그 네임스페이스 — 만
	// 없을 때 먼저 만든다. cert-manager 를 다시 세우기 **전에** 불러야 한다: 부트스트랩이 Secret 이
	// 있으면 옛 키를 채택하고, 없으면 새 키를 발급해 백업의 CA 는 영영 돌아오지 못한다.
	ApplySharedSecrets(ctx context.Context, in io.Reader) error
}

// ConnectionResetter 는 플랫폼 DB 연결 풀을 비운다(pgxpool.Pool.Reset).
//
// pg_restore --clean 은 테이블을 지우고 다시 만든다. 풀의 연결이 들고 있던 준비된 문장은 옛 테이블을
// 가리켜, 복원 직후 첫 질의가 "cached plan must not change result type (SQLSTATE 0A000)" 으로 죽는다 —
// 어느 연결이 잡히느냐에 따라 간헐적이다(2026-10-12 리허설 4차의 게이트웨이 주소 재설정). 플랫폼 DB 를
// 되돌린 직후 부른다.
type ConnectionResetter interface {
	Reset()
}

// SharedComponentsEnsurer 는 클러스터 공용 컨트롤러(cert-manager·metrics-server)를 없으면 세운다 —
// 스택 모듈의 설치기가 구현한다. 클러스터의 마지막 스택을 지우면 컨트롤러까지 회수되는데, 컨트롤러는
// 설치물이라 백업이 담지 않는다. 복구가 공용 자원(ClusterIssuer·Certificate)을 되살리기 전에 부른다.
type SharedComponentsEnsurer interface {
	// EnsureSharedComponents 는 돌린 단계 이름과, 멈추지 않고 넘긴 실패(metrics-server)를 돌려준다.
	// cert-manager 를 세우지 못하면 오류다 — 그것 없이는 내부 CA 를 되살릴 수 없다.
	EnsureSharedComponents(ctx context.Context, kubeconfig []byte, namespace string) (steps []string, warnings []string, err error)
}

// Sealer 는 산출물을 잠그고 연다.
//
// #68(BYOK)의 교체점이다. v1 은 플랫폼 생성 키를 쓰되, 매니페스트에 KeyID 를
// 남겨 나중에 봉투 암호화 구현체로 바꿔도 기존 백업본을 열 수 있게 한다 (§5.4).
//
// 스트리밍이어야 한다 — E1 규모(수십 GB)에서 전체를 메모리에 올릴 수 없다.
type Sealer interface {
	KeyID() string
	Seal(ctx context.Context, plaintext io.Reader, out io.Writer) error
	Unseal(ctx context.Context, ciphertext io.Reader, out io.Writer) error
}

// ArtifactStore 는 클러스터 외부 오브젝트 스토리지다 (§4.2).
type ArtifactStore interface {
	// Preflight 는 정지 창에 들어가기 전에 연결·인증·쓰기·용량을 검사한다.
	//
	// 정지 창을 소비하고도 산출물을 못 만드는 것이 최악의 조합이라
	// (§9 F7b·F8), 멈추기 전에 실패할 수 있는 것은 전부 먼저 확인한다.
	Preflight(ctx context.Context, requiredBytes int64) error
	Put(ctx context.Context, key string, r io.Reader) (PutResult, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, prefix string) error
	Stat(ctx context.Context, key string) (int64, string, error)
}

type PutResult struct {
	Bytes          int64
	ChecksumSHA256 string
	Location       string
}

// TokenSourceLister 는 admin 모듈의 공개 창구다 (§6.4 참조 정합성 검사).
type TokenSourceLister interface {
	ListPaths(ctx context.Context, orgID string) ([]TokenSourceRef, error)
}

type TokenSourceRef struct {
	ID   string
	Path string
}

// Notifier 는 백업 실패를 알린다 (B3-3, #63 의존).
//
// 백업 실패보다 백업 실패를 모르는 것이 더 나쁘다 (§9 F10).
type Notifier interface {
	NotifyBackupResult(ctx context.Context, run *domain.BackupRun) error
}
