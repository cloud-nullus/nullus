package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// ManageUpgradeBundles 는 관리자가 업그레이드 번들을 등록하고 상태를 바꾸는 유스케이스다
// (설계 §12.4). 번들은 플랫폼 전체의 것이라 조직에 묶이지 않는다. 리허설 없이 실행 가능한 경로가
// 생기지 않도록, verified 는 운영자의 리허설 확인이 있을 때만 된다 — 그 규칙은 도메인이 지킨다.
type ManageUpgradeBundles struct {
	bundles port.UpgradeBundleStore
	now     func() time.Time
}

type BundleOption func(*ManageUpgradeBundles)

// WithBundleClock 은 시각을 바꿔 넣는다(테스트).
func WithBundleClock(now func() time.Time) BundleOption {
	return func(uc *ManageUpgradeBundles) { uc.now = now }
}

func NewManageUpgradeBundles(bundles port.UpgradeBundleStore, opts ...BundleOption) *ManageUpgradeBundles {
	uc := &ManageUpgradeBundles{bundles: bundles, now: func() time.Time { return time.Now().UTC() }}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

type RegisterUpgradeBundleInput struct {
	ID                     string
	Tool                   string
	ReleaseName            string
	ChartName              string
	RepoURL                string
	SourceChartVersion     string
	SourceAppVersion       string
	TargetChartVersion     string
	TargetAppVersion       string
	RiskLevel              string
	ExpectedDisruption     string
	ReleaseNotes           string
	RequiresBackup         bool
	SupportedArchitectures []string
	FixesCVEs              []string
	Status                 string
	RehearsalConfirmed     bool
}

// List 는 상태와 무관하게 모든 번들을 돌려준다 — 관리 화면은 거둔 경로도 본다.
func (uc *ManageUpgradeBundles) List(ctx context.Context) ([]domain.UpgradeBundle, error) {
	if uc.bundles == nil {
		return nil, fmt.Errorf("업그레이드 번들 저장소가 구성되지 않았다")
	}
	items, err := uc.bundles.ListBundles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list upgrade bundles: %w", err)
	}
	if items == nil {
		items = []domain.UpgradeBundle{}
	}
	return items, nil
}

// Register 는 번들을 검사해 저장한다. 같은 ID · 같은 경로면 domain.ErrUpgradeBundleExists.
func (uc *ManageUpgradeBundles) Register(ctx context.Context, in RegisterUpgradeBundleInput) (*domain.UpgradeBundle, error) {
	if uc.bundles == nil {
		return nil, fmt.Errorf("업그레이드 번들 저장소가 구성되지 않았다")
	}
	bundle, err := domain.NewUpgradeBundle(domain.NewUpgradeBundleInput{
		ID: in.ID, Tool: in.Tool, ReleaseName: in.ReleaseName, ChartName: in.ChartName, RepoURL: in.RepoURL,
		SourceChartVersion: in.SourceChartVersion, SourceAppVersion: in.SourceAppVersion,
		TargetChartVersion: in.TargetChartVersion, TargetAppVersion: in.TargetAppVersion,
		RiskLevel: in.RiskLevel, ExpectedDisruption: in.ExpectedDisruption, ReleaseNotes: in.ReleaseNotes,
		RequiresBackup: in.RequiresBackup, SupportedArchitectures: in.SupportedArchitectures, FixesCVEs: in.FixesCVEs,
		Status: in.Status, RehearsalConfirmed: in.RehearsalConfirmed, Now: uc.now(),
	})
	if err != nil {
		return nil, err
	}
	if err := uc.bundles.CreateBundle(ctx, bundle); err != nil {
		return nil, err
	}
	return bundle, nil
}

// ChangeStatus 는 번들 상태를 바꾼다. verified 로 올릴 때는 리허설 확인이 필요하다.
func (uc *ManageUpgradeBundles) ChangeStatus(ctx context.Context, id, status string, rehearsalConfirmed bool) (*domain.UpgradeBundle, error) {
	if uc.bundles == nil {
		return nil, fmt.Errorf("업그레이드 번들 저장소가 구성되지 않았다")
	}
	bundle, err := uc.bundles.GetBundle(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if err := bundle.ChangeStatus(status, rehearsalConfirmed); err != nil {
		return nil, err
	}
	if err := uc.bundles.UpdateBundleStatus(ctx, bundle.ID, bundle.Status); err != nil {
		return nil, err
	}
	return bundle, nil
}
