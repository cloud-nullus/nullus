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
	EnsurePVC(ctx context.Context, namespace string, spec domain.VolumeSpec) error
}

// ResourceDumper 는 네임스페이스 리소스와 Helm 릴리스를 뜬다 (D1).
type ResourceDumper interface {
	Dump(ctx context.Context, namespace string, out io.Writer) (int64, error)
	Apply(ctx context.Context, namespace string, in io.Reader) error
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
