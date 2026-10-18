package port

import (
	"context"
	"errors"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// ErrUpgradeIdempotencyConflict 는 같은 스택에 같은 Idempotency-Key 의 실행이 이미 있을 때
// CreateRun 이 돌려준다. 유스케이스는 그 실행을 찾아 그대로 돌려준다.
var ErrUpgradeIdempotencyConflict = errors.New("upgrade run with the same idempotency key exists")

type UpgradeRepository interface {
	ListVerifiedBundles(ctx context.Context, tool, sourceChartVersion string) ([]domain.UpgradeBundle, error)
	// GetVerifiedBundle 은 verified 번들만 돌려준다. 없으면 domain.ErrUpgradeBundleNotFound.
	GetVerifiedBundle(ctx context.Context, id string) (*domain.UpgradeBundle, error)
	// CreateRun 은 진행 중 실행이 이미 있으면 domain.ErrUpgradeInProgress, 같은 키의 실행이
	// 있으면 ErrUpgradeIdempotencyConflict 를 돌려준다. 동시에 들어온 요청은 DB 가 가린다.
	CreateRun(ctx context.Context, run *domain.UpgradeRun) error
	UpdateRun(ctx context.Context, run *domain.UpgradeRun) error
	// GetRun 은 없으면 domain.ErrUpgradeRunNotFound.
	GetRun(ctx context.Context, stackID, runID string) (*domain.UpgradeRun, error)
	// FindRunByIdempotencyKey 와 아래 조회는 없으면 (nil, nil) 이다.
	FindRunByIdempotencyKey(ctx context.Context, stackID, key string) (*domain.UpgradeRun, error)
	ActiveRun(ctx context.Context, stackID string) (*domain.UpgradeRun, error)
	// LatestRun 은 실제로 실행된(사전 점검에서 막히지 않은) 실행 중 가장 최근 것이다. 막힌 시도는
	// 요청한 자리에서 이유를 돌려주므로, 화면의 "마지막 실행" 을 가리지 않게 뺀다.
	LatestRun(ctx context.Context, stackID, tool string) (*domain.UpgradeRun, error)
}

type ChartUpgradeRequest struct {
	ReleaseName string
	ChartName   string
	RepoURL     string
	Version     string
	Namespace   string
	Values      map[string]any
	DryRun      bool
}

type ChartUpgradeResult struct {
	Revision int
	Status   string
	Manifest string
}

// ChartUpgradeExecutor 는 차트 버전을 바꾸는 경로다. values 만 바꾸는 HelmReleaseManager.Upgrade 와
// 나눠 둔다 — 그쪽은 설치된 차트를 그대로 다시 쓰는 것이 계약이다.
type ChartUpgradeExecutor interface {
	UpgradeChart(ctx context.Context, req ChartUpgradeRequest) (*ChartUpgradeResult, error)
	RollbackRelease(ctx context.Context, releaseName, namespace string, revision int) error
}

// UpgradeBackupLookup 은 백업 모듈이 답한다 — 스택의 마지막 복구 가능 백업(성공했고 네임스페이스
// 리소스 산출물이 있는 것)이 끝난 시각. 스택 모듈은 백업 테이블을 직접 읽지 않는다.
type UpgradeBackupLookup interface {
	LatestRestorableBackup(ctx context.Context, stackID string) (finishedAt time.Time, found bool, err error)
}

// UpgradeHealthVerifier 는 Helm 이 성공이라 해도 도구가 실제로 그 버전으로 살아 있는지 본다.
// 되돌린 뒤에도 같은 검사를 출발 버전으로 한 번 더 한다.
type UpgradeHealthVerifier interface {
	Verify(ctx context.Context, kubeconfig []byte, namespace string, bundle domain.UpgradeBundle) error
}
