package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
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
	// airgap 이면 보고서에 에어갭이라고 남긴다 — 취약점 DB 가 번들을 반입한 날짜 기준이다. 이미지를
	// 내부 레지스트리에서 받는 일은 스캐너가 한다(imagescan.WithMirrorRegistry).
	airgap bool

	// notifier 는 새로 발견된 "수정판 있는 CRITICAL" 을 조직의 알림 채널로 보낸다.
	// 배선되지 않으면 보내지 않는다 — 결과는 화면에서만 본다.
	notifier sharednotify.Publisher

	mu      sync.Mutex
	running map[string]struct{}
}

// WithNotifier 는 새 CVE 고지 경로를 배선한다.
func (uc *ScanStackImages) WithNotifier(p sharednotify.Publisher) *ScanStackImages {
	uc.notifier = p
	return uc
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
	stack, err := uc.stackByID(ctx, stackID)
	if err != nil {
		return nil, err
	}
	if reason := uc.skipReason(stack); reason != "" {
		report := domain.BuildImageScanReport(stack.ID, reason, nil)
		report.Airgap = uc.airgap
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

	scans, err := uc.scanner.ScanInstalledImages(ctx, kubeconfig, stack.Namespace, domain.SharedComponents())
	if err != nil {
		return nil, fmt.Errorf("스택 %s 설치 이미지 스캔 실패: %w", stack.ID, err)
	}
	for i := range scans {
		scans[i].StackID = stack.ID
	}
	if err := uc.results.ReplaceForStack(ctx, stack.ID, scans); err != nil {
		return nil, fmt.Errorf("스택 %s 설치 이미지 스캔 결과 저장 실패: %w", stack.ID, err)
	}
	uc.notifyNewCVEs(ctx, stack, scans)

	report := domain.BuildImageScanReport(stack.ID, "", scans)
	report.Airgap = uc.airgap
	return &report, nil
}

// stackByID 는 스캔 대상 스택이다. Postgres 저장소는 없거나 지운 스택에 (nil, nil) 을 돌려주므로
// 여기서 "not found" 오류로 바꾼다 — 그대로 두면 다음 줄의 역참조가 패닉이 되고, 핸들러가 404 대신 500 을 낸다.
func (uc *ScanStackImages) stackByID(ctx context.Context, stackID string) (*domain.Stack, error) {
	id := strings.TrimSpace(stackID)
	stack, err := uc.stacks.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, fmt.Errorf("stack %s not found", id)
	}
	return stack, nil
}

// Report 는 저장된 결과로 보고서를 만든다.
func (uc *ScanStackImages) Report(ctx context.Context, stackID string) (*domain.StackImageScanReport, error) {
	stack, err := uc.stackByID(ctx, stackID)
	if err != nil {
		return nil, err
	}
	return uc.reportFor(ctx, stack)
}

// StackOrgID 는 스택이 속한 조직이다. 단건 스캔 조회가 다른 조직의 스택을 보이지 않게 쓴다
// (Security Dashboard 설계 §8 G4).
func (uc *ScanStackImages) StackOrgID(ctx context.Context, stackID string) (string, error) {
	stack, err := uc.stackByID(ctx, stackID)
	if err != nil {
		return "", err
	}
	return stack.OrgID, nil
}

// ReportsForOrg 는 조직의 지우지 않은 스택마다 설치 이미지 스캔 보고서를 만든다. 보안 대시보드(#65)가
// 스택 모듈의 공개 경로로 읽는다 — 결과 테이블(stack_image_scans)은 스택 모듈이 소유한다.
//
// 보고서는 단건 조회(Report)와 같은 조립이다. 두 곳이 따로 만들면 같은 스택이 화면마다 다른 숫자로 보인다.
func (uc *ScanStackImages) ReportsForOrg(ctx context.Context, orgID string) ([]domain.OrgStackImageScan, error) {
	stacks, err := uc.stacks.List(ctx, strings.TrimSpace(orgID), false)
	if err != nil {
		return nil, fmt.Errorf("조직 %s 의 스택 조회 실패: %w", orgID, err)
	}
	out := make([]domain.OrgStackImageScan, 0, len(stacks))
	for _, stack := range stacks {
		if stack == nil {
			continue
		}
		report, err := uc.reportFor(ctx, stack)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.OrgStackImageScan{Stack: stack, Report: *report})
	}
	return out, nil
}

// reportFor 는 스택 하나의 보고서를 저장된 결과로 만든다.
func (uc *ScanStackImages) reportFor(ctx context.Context, stack *domain.Stack) (*domain.StackImageScanReport, error) {
	reason := uc.skipReason(stack)
	var scans []domain.StackImageScan
	if reason == "" {
		var err error
		scans, err = uc.results.ListByStack(ctx, stack.ID)
		if err != nil {
			return nil, fmt.Errorf("스택 %s 설치 이미지 스캔 결과 조회 실패: %w", stack.ID, err)
		}
	}
	report := domain.BuildImageScanReport(stack.ID, reason, scans)
	report.Airgap = uc.airgap
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
	stack, err := uc.stackByID(ctx, stackID)
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
	return uc.rescan(ctx, func(*domain.Stack) bool { return true })
}

// RescanStale 은 마지막 스캔이 maxAge 보다 오래됐거나 아직 없는 스택만 다시 스캔한다. 기동 직후에
// 부른다 — 주기 재스캔의 타이머는 재시작 · 롤아웃마다 처음부터 다시 세어지므로, 그것만으로는 "24시간 안에
// 새 CVE 를 알린다" 를 지킬 수 없다. 이미 최근에 스캔한 스택은 레플리카마다 다시 스캔하지 않는다.
func (uc *ScanStackImages) RescanStale(ctx context.Context, maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	return uc.rescan(ctx, func(stack *domain.Stack) bool {
		scans, err := uc.results.ListByStack(ctx, stack.ID)
		if err != nil {
			slog.Warn("설치 이미지 재스캔: 지난 스캔 시각을 읽지 못해 다시 스캔합니다", "stack_id", stack.ID, "error", err)
			return true
		}
		for _, s := range scans {
			if s.ScannedAt.After(cutoff) {
				return false
			}
		}
		return true
	})
}

func (uc *ScanStackImages) rescan(ctx context.Context, due func(*domain.Stack) bool) int {
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
		if stack == nil || uc.skipReason(stack) != "" || !due(stack) {
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
	return domain.ImageScanSkipReasonFor(cfg)
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

// maxCVEsInNotice · maxImagesPerCVE 는 알림 본문에 싣는 CVE 수와 CVE 하나에 적는 이미지 수다. 나머지는
// 수만 적는다 — 메신저 본문 한도(Telegram 4096자)가 있고, 목록 전체는 화면에 있다. GitLab 은 공통
// 베이스 때문에 CVE 하나가 이미지 열 개에 걸친다(실측).
const (
	maxCVEsInNotice = 5
	maxImagesPerCVE = 3
)

// notifyNewCVEs 는 아직 알리지 않은 "수정판 있는 CRITICAL" 을 스택당 한 건으로 묶어 보낸다.
//
// 새것은 알린 기록(stack_cve_notices)으로 가린다. 보내기 전에 기록을 선점해, 레플리카 둘이 같은
// 결과를 보고도 한 번만 보낸다. 외부에 노출된 이미지의 것이 있으면 심각(critical), 아니면
// 경고(warning)다 — 정책의 대응 기한이 노출 여부로 갈린다. 보내지 못해도 스캔은 성공이다.
func (uc *ScanStackImages) notifyNewCVEs(ctx context.Context, stack *domain.Stack, scans []domain.StackImageScan) {
	if uc.notifier == nil {
		return
	}
	notified, err := uc.results.NotifiedCVEIDs(ctx, stack.ID)
	if err != nil {
		// 무엇을 알렸는지 모르면 전부 새것으로 보게 된다 — 이번엔 건너뛰고 다음 스캔이 다시 본다.
		slog.Warn("설치 이미지 스캔: 알린 CVE 를 읽지 못해 새 CVE 고지를 건너뜁니다", "stack_id", stack.ID, "error", err)
		return
	}
	findings := domain.NewFixableCriticals(notified, scans)
	if len(findings) == 0 {
		return
	}
	ids := make([]string, 0, len(findings))
	for _, f := range findings {
		ids = append(ids, f.ID)
	}
	now := time.Now()
	claimed, err := uc.results.ClaimCVENotices(ctx, stack.ID, ids, now)
	if err != nil {
		slog.Warn("설치 이미지 스캔: 알릴 CVE 를 남기지 못해 고지를 건너뜁니다", "stack_id", stack.ID, "error", err)
		return
	}
	findings = onlyCVEs(findings, claimed)
	if len(findings) == 0 {
		return // 다른 레플리카가 먼저 보냈다
	}
	firstNotice := len(notified) == 0
	severity := sharednotify.SeverityWarning
	exposed := 0
	for _, f := range findings {
		if f.Exposed {
			severity = sharednotify.SeverityCritical
			exposed++
		}
	}
	name := firstNonEmptyString(stack.Name, stack.ID)
	title := fmt.Sprintf("스택 %s: 수정판 있는 CRITICAL 취약점 %d건 새로 발견", name, len(findings))
	if firstNotice {
		title = fmt.Sprintf("스택 %s 설치 이미지 첫 고지: 수정판 있는 CRITICAL 취약점 %d건", name, len(findings))
	}

	var b strings.Builder
	for i, f := range findings {
		if i == maxCVEsInNotice {
			fmt.Fprintf(&b, "외 %d건 — 스택 상세의 이미지 취약점에서 모두 볼 수 있습니다\n", len(findings)-maxCVEsInNotice)
			break
		}
		fmt.Fprintf(&b, "• %s — %s · %s", f.ID, strings.Join(f.Packages, ", "), limitedList(f.Images, maxImagesPerCVE))
		if f.Exposed {
			if len(f.Hosts) > 0 {
				fmt.Fprintf(&b, " (외부 노출: %s)", strings.Join(f.Hosts, ", "))
			} else {
				b.WriteString(" (외부 노출)")
			}
		}
		b.WriteString("\n")
	}

	event := sharednotify.Event{
		Type:     sharednotify.EventStackCVEFound,
		OrgID:    stack.OrgID,
		Severity: severity,
		Title:    title,
		Summary:  strings.TrimRight(b.String(), "\n"),
		Fields: []sharednotify.Field{
			{Name: "스택", Value: fmt.Sprintf("%s (%s)", name, stack.ID)},
			{Name: "새 CVE", Value: fmt.Sprintf("%d건 (외부 노출 이미지에 있는 것 %d건)", len(findings), exposed)},
		},
		OccurredAt: now,
	}
	if err := uc.notifier.Publish(ctx, event); err != nil {
		slog.Warn("설치 이미지 스캔: 새 CVE 고지를 보내지 못했습니다", "stack_id", stack.ID, "error", err)
		// 채널이 실패한 것은 발송 이력에 남고 다시 보내지 않는다(받은 채널에 반복해 가지 않게).
		// 종료로 취소돼 보내지 못한 것만 되돌려, 다음 기동 뒤 스캔이 보낸다.
		if ctx.Err() != nil {
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if err := uc.results.ReleaseCVENotices(releaseCtx, stack.ID, claimed); err != nil {
				slog.Warn("설치 이미지 스캔: 보내지 못한 고지의 선점을 되돌리지 못했습니다", "stack_id", stack.ID, "error", err)
			}
		}
	}
}

// onlyCVEs 는 findings 중 ids 에 있는 것만 남긴다(순서 유지).
func onlyCVEs(findings []domain.CVEFinding, ids []string) []domain.CVEFinding {
	keep := make(map[string]bool, len(ids))
	for _, id := range ids {
		keep[id] = true
	}
	out := findings[:0:0]
	for _, f := range findings {
		if keep[f.ID] {
			out = append(out, f)
		}
	}
	return out
}

// limitedList 는 앞의 max 개만 적고 나머지는 수로 적는다.
func limitedList(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s 외 %d개", strings.Join(items[:max], ", "), len(items)-max)
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
