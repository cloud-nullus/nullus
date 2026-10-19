package usecase

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

type fakeImageScanner struct {
	mu         sync.Mutex
	calls      []string
	shared     []domain.SharedComponent
	kubeconfig []byte
	scans      []domain.StackImageScan
	err        error
	started    chan struct{}
	block      chan struct{}
}

func (f *fakeImageScanner) ScanInstalledImages(_ context.Context, kubeconfig []byte, namespace string, shared []domain.SharedComponent) ([]domain.StackImageScan, error) {
	f.mu.Lock()
	f.calls = append(f.calls, namespace)
	f.shared = shared
	f.kubeconfig = kubeconfig
	f.mu.Unlock()
	if f.started != nil {
		f.started <- struct{}{}
	}
	if f.block != nil {
		<-f.block
	}
	return append([]domain.StackImageScan(nil), f.scans...), f.err
}

func (f *fakeImageScanner) namespaces() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

type fakeImageScanResults struct {
	mu       sync.Mutex
	rows     map[string][]domain.StackImageScan
	notified map[string]map[string]time.Time
	knownErr error
	claimErr error
	// staleNotified 가 있으면 NotifiedCVEIDs 가 그것을 돌려준다.
	staleNotified map[string]bool
}

func newFakeImageScanResults() *fakeImageScanResults {
	return &fakeImageScanResults{rows: map[string][]domain.StackImageScan{}, notified: map[string]map[string]time.Time{}}
}

func (r *fakeImageScanResults) ClaimCVENotices(_ context.Context, stackID string, ids []string, at time.Time) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	if r.notified[stackID] == nil {
		r.notified[stackID] = map[string]time.Time{}
	}
	claimed := []string{}
	for _, id := range ids {
		if _, ok := r.notified[stackID][id]; !ok {
			r.notified[stackID][id] = at
			claimed = append(claimed, id)
		}
	}
	return claimed, nil
}

func (r *fakeImageScanResults) ReleaseCVENotices(_ context.Context, stackID string, ids []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		delete(r.notified[stackID], id)
	}
	return nil
}

func (r *fakeImageScanResults) notifiedIDs(stackID string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []string{}
	for id := range r.notified[stackID] {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (r *fakeImageScanResults) ReplaceForStack(_ context.Context, stackID string, scans []domain.StackImageScan) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[stackID] = append([]domain.StackImageScan(nil), scans...)
	return nil
}

func (r *fakeImageScanResults) ListByStack(_ context.Context, stackID string) ([]domain.StackImageScan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.StackImageScan(nil), r.rows[stackID]...), nil
}

func (r *fakeImageScanResults) ListVulnerabilities(_ context.Context, stackID, digest string) (domain.StackImageVulnerabilities, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.rows[stackID] {
		if s.ImageDigest == digest {
			return domain.StackImageVulnerabilities{
				Found: true, Recorded: s.VulnerabilitiesRecorded, Items: s.Vulnerabilities,
			}, nil
		}
	}
	return domain.StackImageVulnerabilities{}, nil
}

type fakeCompletedStacks struct{ stacks []*domain.Stack }

func (f fakeCompletedStacks) ListCompleted(context.Context) ([]*domain.Stack, error) {
	return f.stacks, nil
}

func imageScanStack(id string, withScanner bool) *domain.Stack {
	cfg := domain.StackConfig{}
	if withScanner {
		cfg.Security.ImageScanner = domain.ToolSelection{Name: "trivy", Version: "0.74.0", Enabled: true}
	}
	return &domain.Stack{ID: id, ClusterID: "cl-1", Namespace: "ns-" + id, State: domain.StateCompleted, Config: cfg}
}

func (r *fakeImageScanResults) NotifiedCVEIDs(_ context.Context, stackID string) (map[string]bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.knownErr != nil {
		return nil, r.knownErr
	}
	// staleNotified 는 다른 레플리카가 선점하기 전에 읽은 것처럼 보이게 한다.
	if r.staleNotified != nil {
		return r.staleNotified, nil
	}
	ids := map[string]bool{}
	for id := range r.notified[stackID] {
		ids[id] = true
	}
	return ids, nil
}

func newScanStackImagesUC(stacks []*domain.Stack, scanner *fakeImageScanner, results *fakeImageScanResults, airgap bool) *ScanStackImages {
	kubeconfigs := &fakeKubeconfigProvider{configs: map[string][]byte{"cl-1": []byte("kubeconfig")}}
	return NewScanStackImages(newFakeStackRepo(stacks...), fakeCompletedStacks{stacks: stacks},
		kubeconfigs, scanner, results, airgap)
}

func TestScanStackImages_Execute_StoresResultsForCompletedStack(t *testing.T) {
	counts := shareddomain.SeverityCounts{High: 2}
	scanner := &fakeImageScanner{scans: []domain.StackImageScan{{
		Image: "redis:7", ImageDigest: "sha256:r", Status: domain.ImageScanStatusScanned,
		Counts: &counts, ScannedAt: time.Now(),
	}}}
	results := newFakeImageScanResults()
	uc := newScanStackImagesUC([]*domain.Stack{imageScanStack("stk_1", true)}, scanner, results, false)

	report, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	assert.Equal(t, domain.ImageScanStateScanned, report.Status)
	assert.Equal(t, []string{"ns-stk_1"}, scanner.namespaces())
	assert.Equal(t, []byte("kubeconfig"), scanner.kubeconfig)
	stored, _ := results.ListByStack(context.Background(), "stk_1")
	require.Len(t, stored, 1)
	assert.Equal(t, "stk_1", stored[0].StackID)
}

// 스캔할 수 없는 스택은 스캐너를 부르지 않고 이유만 돌려준다.
func TestScanStackImages_Execute_SkipsWithReason(t *testing.T) {
	tests := []struct {
		name   string
		stack  *domain.Stack
		airgap bool
		want   domain.ImageScanSkipReason
	}{
		{"스캐너를 고르지 않은 스택", imageScanStack("stk_1", false), false, domain.ImageScanReasonScannerNotInstalled},
		{"에어갭 플랫폼", imageScanStack("stk_1", true), true, domain.ImageScanReasonAirgap},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scanner := &fakeImageScanner{}
			results := newFakeImageScanResults()
			uc := newScanStackImagesUC([]*domain.Stack{tc.stack}, scanner, results, tc.airgap)

			report, err := uc.Execute(context.Background(), "stk_1")
			require.NoError(t, err)

			assert.Equal(t, domain.ImageScanStateNotScanned, report.Status)
			assert.Equal(t, tc.want, report.Reason)
			assert.Empty(t, scanner.namespaces())
			stored, _ := results.ListByStack(context.Background(), "stk_1")
			assert.Empty(t, stored)
		})
	}
}

// 스캔이 실패하면 이전 결과를 지우지 않는다. 일시적인 레지스트리 장애로 보고서가
// 비면 "스캔 대기" 로 되돌아가 알던 취약점이 사라진 것처럼 보인다.
func TestScanStackImages_Execute_KeepsPreviousResultsOnFailure(t *testing.T) {
	results := newFakeImageScanResults()
	require.NoError(t, results.ReplaceForStack(context.Background(), "stk_1", []domain.StackImageScan{{
		ImageDigest: "sha256:old", Status: domain.ImageScanStatusScanned, ScannedAt: time.Now(),
	}}))
	scanner := &fakeImageScanner{err: errors.New("scan job timed out")}
	uc := newScanStackImagesUC([]*domain.Stack{imageScanStack("stk_1", true)}, scanner, results, false)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.Error(t, err)

	stored, _ := results.ListByStack(context.Background(), "stk_1")
	require.Len(t, stored, 1)
	assert.Equal(t, "sha256:old", stored[0].ImageDigest)
}

// 설치 중인 스택은 이미지가 다 뜨지 않았다. 반쪽 결과를 남기지 않는다.
func TestScanStackImages_Execute_RejectsUnfinishedStack(t *testing.T) {
	stack := imageScanStack("stk_1", true)
	stack.State = domain.StateInstalling
	scanner := &fakeImageScanner{}
	uc := newScanStackImagesUC([]*domain.Stack{stack}, scanner, newFakeImageScanResults(), false)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.Error(t, err)
	assert.Empty(t, scanner.namespaces())
}

// 한 스택의 스캔은 한 번에 하나다. 설치 직후 스캔과 주기 재스캔이 겹치면 같은
// 이름의 Job 을 서로 지운다.
func TestScanStackImages_Execute_OneScanPerStackAtATime(t *testing.T) {
	scanner := &fakeImageScanner{started: make(chan struct{}, 1), block: make(chan struct{})}
	uc := newScanStackImagesUC([]*domain.Stack{imageScanStack("stk_1", true)}, scanner, newFakeImageScanResults(), false)

	done := make(chan error, 1)
	go func() {
		_, err := uc.Execute(context.Background(), "stk_1")
		done <- err
	}()
	<-scanner.started

	_, err := uc.Execute(context.Background(), "stk_1")
	assert.ErrorIs(t, err, ErrImageScanInProgress)

	close(scanner.block)
	require.NoError(t, <-done)
}

func TestScanStackImages_Report(t *testing.T) {
	results := newFakeImageScanResults()
	counts := shareddomain.SeverityCounts{Critical: 1}
	require.NoError(t, results.ReplaceForStack(context.Background(), "stk_1", []domain.StackImageScan{{
		ImageDigest: "sha256:p", Status: domain.ImageScanStatusScanned, Counts: &counts, ScannedAt: time.Now(),
	}}))
	stacks := []*domain.Stack{imageScanStack("stk_1", true)}

	report, err := newScanStackImagesUC(stacks, &fakeImageScanner{}, results, false).Report(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Equal(t, domain.ImageScanStateScanned, report.Status)
	assert.Len(t, report.Items, 1)

	// 에어갭 플랫폼에서는 남은 결과가 있어도 스캔하지 않는 스택으로 보인다.
	report, err = newScanStackImagesUC(stacks, &fakeImageScanner{}, results, true).Report(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Equal(t, domain.ImageScanStateNotScanned, report.Status)
	assert.Empty(t, report.Items)
}

func TestScanStackImages_Report_UnknownStack(t *testing.T) {
	_, err := newScanStackImagesUC(nil, &fakeImageScanner{}, newFakeImageScanResults(), false).
		Report(context.Background(), "missing")
	assert.Error(t, err)
}

func TestScanStackImages_RescanAll_OnlyEligibleStacks(t *testing.T) {
	stacks := []*domain.Stack{imageScanStack("stk_1", true), imageScanStack("stk_2", false), imageScanStack("stk_3", true)}
	scanner := &fakeImageScanner{}
	uc := newScanStackImagesUC(stacks, scanner, newFakeImageScanResults(), false)

	assert.Equal(t, 2, uc.RescanAll(context.Background()))
	assert.ElementsMatch(t, []string{"ns-stk_1", "ns-stk_3"}, scanner.namespaces())
}

// 재시작 · 롤아웃마다 재스캔 타이머가 처음부터 다시 세어지면 24시간 안의 고지를 지킬 수 없다. 기동 직후에는
// 마지막 스캔이 주기보다 오래됐거나 아직 없는 스택만 스캔한다 — 레플리카마다 전부 다시 스캔하지 않는다.
func TestScanStackImages_RescanStale_OnlyStaleOrNeverScanned(t *testing.T) {
	stacks := []*domain.Stack{imageScanStack("stk_fresh", true), imageScanStack("stk_old", true),
		imageScanStack("stk_never", true), imageScanStack("stk_noscanner", false)}
	results := newFakeImageScanResults()
	ctx := context.Background()
	require.NoError(t, results.ReplaceForStack(ctx, "stk_fresh", []domain.StackImageScan{
		{Image: "a", ImageDigest: "sha256:a", Status: domain.ImageScanStatusScanned, ScannedAt: time.Now().Add(-time.Hour)}}))
	require.NoError(t, results.ReplaceForStack(ctx, "stk_old", []domain.StackImageScan{
		{Image: "a", ImageDigest: "sha256:a", Status: domain.ImageScanStatusScanned, ScannedAt: time.Now().Add(-30 * time.Hour)},
		{Image: "b", ImageDigest: "sha256:b", Status: domain.ImageScanStatusFailed, ScannedAt: time.Now().Add(-26 * time.Hour)}}))
	scanner := &fakeImageScanner{}
	uc := newScanStackImagesUC(stacks, scanner, results, false)

	assert.Equal(t, 2, uc.RescanStale(ctx, 24*time.Hour))
	assert.ElementsMatch(t, []string{"ns-stk_old", "ns-stk_never"}, scanner.namespaces())
}

// 에어갭 플랫폼은 주기 재스캔도 하지 않는다.
func TestScanStackImages_RescanAll_NothingOnAirgap(t *testing.T) {
	scanner := &fakeImageScanner{}
	uc := newScanStackImagesUC([]*domain.Stack{imageScanStack("stk_1", true)}, scanner, newFakeImageScanResults(), true)

	assert.Zero(t, uc.RescanAll(context.Background()))
	assert.Empty(t, scanner.namespaces())
}

// 스택 스캔은 스택이 쓰는 클러스터 공용 구성요소를 함께 넘긴다.
func TestScanStackImages_Execute_PassesSharedComponents(t *testing.T) {
	scanner := &fakeImageScanner{scans: []domain.StackImageScan{{
		Image: "redis:7", ImageDigest: "sha256:r", Status: domain.ImageScanStatusScanned, ScannedAt: time.Now(),
	}}}
	uc := newScanStackImagesUC([]*domain.Stack{imageScanStack("stk_1", true)}, scanner, newFakeImageScanResults(), false)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Equal(t, domain.SharedComponents(), scanner.shared)
}
