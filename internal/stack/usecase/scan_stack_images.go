package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// ErrStackImageScanNotFound 는 그 이미지의 스캔 결과가 없다는 뜻이다.
var ErrStackImageScanNotFound = errors.New("이 스택에서 그 이미지의 스캔 결과를 찾지 못했습니다")

// ErrImageScanInProgress 는 같은 스택의 스캔이 이미 돌고 있다는 뜻이다.
var ErrImageScanInProgress = errors.New("이 스택의 설치 이미지 스캔이 이미 진행 중입니다")

// ScanStackImages 는 스택이 설치한 OSS 이미지를 스캔해 보고서로 남긴다.
//
// 보고용이다 — 설치를 막지 않는다. 설치 직후 한 번, 이후 주기적으로 다시 돈다.
// 설치 뒤에 공개된 CVE 는 다시 스캔해야 보이기 때문이다.
type ScanStackImages struct {
	stacks      port.StackRepository
	completed   port.CompletedStackLister
	kubeconfigs port.KubeconfigProvider
	scanner     port.InstalledImageScanner
	results     port.StackImageScanRepository
	// airgap 이면 스캔하지 않는다. 이미지를 받을 외부 레지스트리에 닿지 못하고,
	// 취약점 DB 는 사람이 넣은 만큼만 새것이다.
	airgap bool

	mu      sync.Mutex
	running map[string]struct{}
}

// NewScanStackImages 는 설치 이미지 스캔 유스케이스를 만든다.
func NewScanStackImages(
	stacks port.StackRepository,
	completed port.CompletedStackLister,
	kubeconfigs port.KubeconfigProvider,
	scanner port.InstalledImageScanner,
	results port.StackImageScanRepository,
	airgap bool,
) *ScanStackImages {
	return &ScanStackImages{
		stacks:      stacks,
		completed:   completed,
		kubeconfigs: kubeconfigs,
		scanner:     scanner,
		results:     results,
		airgap:      airgap,
		running:     map[string]struct{}{},
	}
}

// Execute 는 스택 하나의 설치 이미지를 스캔하고 결과를 저장한다.
//
// 스캔할 수 없는 스택이면 스캐너를 부르지 않고 이유를 담은 보고서를 돌려준다.
// 스캔이 실패하면 이전 결과를 그대로 둔다 — 일시적인 장애로 보고서가 비면 알던
// 취약점이 사라진 것처럼 보인다.
func (uc *ScanStackImages) Execute(ctx context.Context, stackID string) (*domain.StackImageScanReport, error) {
	stack, err := uc.stacks.GetByID(ctx, strings.TrimSpace(stackID))
	if err != nil {
		return nil, err
	}
	if reason := uc.skipReason(stack); reason != "" {
		report := domain.BuildImageScanReport(stack.ID, reason, nil)
		return &report, nil
	}
	if stack.State != domain.StateCompleted {
		// 설치 중인 스택은 이미지가 다 뜨지 않았다. 반쪽 결과를 남기지 않는다.
		return nil, fmt.Errorf("스택 %s 의 설치가 끝나지 않아 이미지를 스캔하지 않습니다 (state=%s)", stack.ID, stack.State)
	}
	if !uc.acquire(stack.ID) {
		return nil, ErrImageScanInProgress
	}
	defer uc.release(stack.ID)

	kubeconfig, err := uc.kubeconfigs.GetKubeconfig(ctx, stack.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("클러스터 %s kubeconfig 조회 실패: %w", stack.ClusterID, err)
	}
	if len(kubeconfig) == 0 {
		return nil, fmt.Errorf("클러스터 %s 의 kubeconfig 가 없습니다", stack.ClusterID)
	}

	scans, err := uc.scanner.ScanInstalledImages(ctx, kubeconfig, stack.Namespace)
	if err != nil {
		return nil, fmt.Errorf("스택 %s 설치 이미지 스캔 실패: %w", stack.ID, err)
	}
	for i := range scans {
		scans[i].StackID = stack.ID
	}
	if err := uc.results.ReplaceForStack(ctx, stack.ID, scans); err != nil {
		return nil, fmt.Errorf("스택 %s 설치 이미지 스캔 결과 저장 실패: %w", stack.ID, err)
	}

	report := domain.BuildImageScanReport(stack.ID, "", scans)
	return &report, nil
}

// Report 는 저장된 결과로 보고서를 만든다.
func (uc *ScanStackImages) Report(ctx context.Context, stackID string) (*domain.StackImageScanReport, error) {
	stack, err := uc.stacks.GetByID(ctx, strings.TrimSpace(stackID))
	if err != nil {
		return nil, err
	}
	reason := uc.skipReason(stack)
	var scans []domain.StackImageScan
	if reason == "" {
		scans, err = uc.results.ListByStack(ctx, stack.ID)
		if err != nil {
			return nil, fmt.Errorf("스택 %s 설치 이미지 스캔 결과 조회 실패: %w", stack.ID, err)
		}
	}
	report := domain.BuildImageScanReport(stack.ID, reason, scans)
	return &report, nil
}

// Vulnerabilities 는 설치 이미지 하나의 취약점 목록을 조건에 맞춰 한 쪽씩 돌려준다.
//
// 스캔하지 않는 스택이면 그 이유로, 목록을 저장하지 않은 스캔이면 not_recorded 로
// 목록을 보일 수 없다고 답한다 — 빈 목록을 돌려주면 취약점 0건으로 읽힌다.
func (uc *ScanStackImages) Vulnerabilities(
	ctx context.Context,
	stackID, digest string,
	filter shareddomain.VulnerabilityFilter,
) (*shareddomain.VulnerabilityPage, error) {
	stack, err := uc.stacks.GetByID(ctx, strings.TrimSpace(stackID))
	if err != nil {
		return nil, err
	}
	if reason := uc.skipReason(stack); reason != "" {
		page := shareddomain.UnavailableVulnerabilityPage(string(reason), filter)
		return &page, nil
	}
	record, err := uc.results.ListVulnerabilities(ctx, stack.ID, strings.TrimSpace(digest))
	if err != nil {
		return nil, fmt.Errorf("스택 %s 설치 이미지 취약점 목록 조회 실패: %w", stack.ID, err)
	}
	if !record.Found {
		return nil, ErrStackImageScanNotFound
	}
	if !record.Recorded {
		page := shareddomain.UnavailableVulnerabilityPage(shareddomain.VulnerabilityReasonNotRecorded, filter)
		return &page, nil
	}
	page := shareddomain.NewVulnerabilityPage(record.Items, filter)
	return &page, nil
}

// RescanAll 은 스캔할 수 있는 모든 완료 스택을 다시 스캔하고, 성공한 수를 돌려준다.
//
// 한 스택의 실패로 나머지를 멈추지 않는다.
func (uc *ScanStackImages) RescanAll(ctx context.Context) int {
	if uc.completed == nil {
		return 0
	}
	stacks, err := uc.completed.ListCompleted(ctx)
	if err != nil {
		slog.Warn("설치 이미지 재스캔: 완료된 스택 목록을 읽지 못했습니다", "error", err)
		return 0
	}

	scanned := 0
	for _, stack := range stacks {
		if ctx.Err() != nil {
			break
		}
		if stack == nil || uc.skipReason(stack) != "" {
			continue
		}
		if _, err := uc.Execute(ctx, stack.ID); err != nil {
			slog.Warn("설치 이미지 재스캔 실패", "stack_id", stack.ID, "error", err)
			continue
		}
		scanned++
	}
	return scanned
}

func (uc *ScanStackImages) skipReason(stack *domain.Stack) domain.ImageScanSkipReason {
	cfg, ok := stackConfigFromInterface(stack.Config)
	if !ok {
		cfg = domain.StackConfig{}
	}
	return domain.ImageScanSkipReasonFor(cfg, uc.airgap)
}

func (uc *ScanStackImages) acquire(stackID string) bool {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if _, busy := uc.running[stackID]; busy {
		return false
	}
	uc.running[stackID] = struct{}{}
	return true
}

func (uc *ScanStackImages) release(stackID string) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	delete(uc.running, stackID)
}
