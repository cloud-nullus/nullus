package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

type fakeCVEPublisher struct {
	events []sharednotify.Event
	err    error
}

func (f *fakeCVEPublisher) Publish(_ context.Context, e sharednotify.Event) error {
	f.events = append(f.events, e)
	return f.err
}

func cveScan(digest string, exposed bool, hosts []string, vulns ...shareddomain.ImageVulnerability) domain.StackImageScan {
	return domain.StackImageScan{
		Image: "img-" + digest, ImageDigest: digest, Status: domain.ImageScanStatusScanned,
		VulnerabilitiesRecorded: true, Vulnerabilities: vulns, Exposed: exposed, ExposedHosts: hosts,
		Counts: &shareddomain.SeverityCounts{}, ScannedAt: time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC),
	}
}

func critical(id string) shareddomain.ImageVulnerability {
	return shareddomain.ImageVulnerability{ID: id, Severity: "CRITICAL", PkgName: "openssl", InstalledVersion: "3.0.1", FixedVersion: "3.0.9"}
}

// cveUC 는 notified 를 이미 알린 CVE 로 둔 스택의 유스케이스다.
func cveUC(t *testing.T, scans []domain.StackImageScan, notified ...string) (*ScanStackImages, *fakeCVEPublisher, *fakeImageScanResults) {
	t.Helper()
	stack := imageScanStack("stk_1", true)
	stack.Name, stack.OrgID = "devsecops", "org-1"
	results := newFakeImageScanResults()
	if len(notified) > 0 {
		_, err := results.ClaimCVENotices(context.Background(), "stk_1", notified, time.Now())
		require.NoError(t, err)
	}
	pub := &fakeCVEPublisher{}
	uc := newScanStackImagesUC([]*domain.Stack{stack}, &fakeImageScanner{scans: scans}, results, false).WithNotifier(pub)
	return uc, pub, results
}

// 아직 알리지 않은 "수정판 있는 CRITICAL" 만 알린다 — 매일 같은 내용이 오면 아무도 읽지 않는다.
func TestScanStackImages_Execute_NotifiesNewFixableCriticals(t *testing.T) {
	scans := []domain.StackImageScan{cveScan("sha256:w", true, []string{"gitlab.nullus.local"},
		critical("CVE-OLD"), critical("CVE-NEW"),
		shareddomain.ImageVulnerability{ID: "CVE-HIGH", Severity: "HIGH", FixedVersion: "2"},
		shareddomain.ImageVulnerability{ID: "CVE-NOFIX", Severity: "CRITICAL"})}
	uc, pub, _ := cveUC(t, scans, "CVE-OLD")

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	require.Len(t, pub.events, 1)
	e := pub.events[0]
	assert.Equal(t, sharednotify.EventStackCVEFound, e.Type)
	assert.Equal(t, "org-1", e.OrgID)
	assert.Equal(t, sharednotify.SeverityCritical, e.Severity, "외부에 노출된 이미지면 심각하다")
	assert.Contains(t, e.Title, "devsecops")
	assert.Contains(t, e.Summary, "CVE-NEW")
	assert.Contains(t, e.Summary, "gitlab.nullus.local")
	assert.NotContains(t, e.Summary, "CVE-OLD")
	assert.NotContains(t, e.Summary, "CVE-HIGH")
	assert.NotContains(t, e.Summary, "CVE-NOFIX")
	assert.Contains(t, fieldValue(e, "새 CVE"), "외부 노출 이미지에 있는 것 1건")
}

func fieldValue(e sharednotify.Event, name string) string {
	for _, f := range e.Fields {
		if f.Name == name {
			return f.Value
		}
	}
	return ""
}

// 한 CVE 가 이미지 열 개에 걸쳐 있으면(GitLab 공통 베이스) 줄이 메신저 한도를 넘는다 — 이미지는
// 셋까지 적고 나머지는 수만 적는다(실측: GitLab 스택 첫 스캔).
func TestScanStackImages_Execute_CapsImagesPerCVE(t *testing.T) {
	var scans []domain.StackImageScan
	for _, d := range []string{"sha256:1", "sha256:2", "sha256:3", "sha256:4", "sha256:5"} {
		scans = append(scans, cveScan(d, false, nil, critical("CVE-SHARED")))
	}
	uc, pub, _ := cveUC(t, scans)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	require.Len(t, pub.events, 1)
	summary := pub.events[0].Summary
	assert.Contains(t, summary, "img-sha256:1, img-sha256:2, img-sha256:3 외 2개")
	assert.NotContains(t, summary, "img-sha256:4")
}

func TestScanStackImages_Execute_InternalOnlyIsWarning(t *testing.T) {
	uc, pub, _ := cveUC(t, []domain.StackImageScan{cveScan("sha256:w", false, nil, critical("CVE-NEW"))}, "CVE-OLD")

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	require.Len(t, pub.events, 1)
	assert.Equal(t, sharednotify.SeverityWarning, pub.events[0].Severity)
}

// 스택의 첫 고지는 지금 있는 것을 한 번에 알린다 — 한 건으로 묶고, 첫 고지라고 밝힌다. 새로 설치한
// 스택이면 설치 시점에 있던 것이고, 이 기능 전에 설치한 스택이면 지금의 현황이다.
func TestScanStackImages_Execute_FirstNoticeSummarizesOnce(t *testing.T) {
	scans := []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"), critical("CVE-2"))}
	uc, pub, _ := cveUC(t, scans)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	require.Len(t, pub.events, 1)
	assert.Contains(t, pub.events[0].Title, "첫 고지")
	assert.Contains(t, pub.events[0].Title, "2건")
}

func TestScanStackImages_Execute_NoEventWhenNothingNew(t *testing.T) {
	uc, pub, _ := cveUC(t, []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"))}, "CVE-1")

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Empty(t, pub.events)
}

// 알린 기록을 읽지 못하면 무엇이 새것인지 모른다 — 전부 새것으로 알리지 않는다. 남긴 것이 없으니
// 다음 스캔이 다시 판단한다(놓치지 않는다).
func TestScanStackImages_Execute_NoEventWhenNotifiedUnknown(t *testing.T) {
	uc, pub, results := cveUC(t, []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"))})
	results.knownErr = errors.New("db down")

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err, "알림을 못 판단해도 스캔 결과는 저장한다")
	assert.Empty(t, pub.events)
	stored, _ := results.ListByStack(context.Background(), "stk_1")
	assert.Len(t, stored, 1)

	results.knownErr = nil
	_, err = uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	require.Len(t, pub.events, 1, "다음 스캔에서 알린다")
	assert.Contains(t, pub.events[0].Summary, "CVE-1")
}

// 알릴 CVE 를 남기지 못하면 보내지 않는다 — 다른 레플리카가 보냈을 수 있고, 다음 스캔이 다시 본다.
func TestScanStackImages_Execute_NoEventWhenClaimFails(t *testing.T) {
	uc, pub, results := cveUC(t, []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"))})
	results.claimErr = errors.New("db down")

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Empty(t, pub.events)
}

// API 레플리카 둘이 같은 시각에 재스캔하면 둘 다 "아직 안 알렸다" 를 읽는다. 먼저 남긴 쪽만 보낸다.
func TestScanStackImages_Execute_TwoReplicasNotifyOnce(t *testing.T) {
	uc, pub, results := cveUC(t, []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"), critical("CVE-2"))}, "CVE-1")
	results.staleNotified = map[string]bool{} // 다른 레플리카가 CVE-1 을 남기기 전에 읽었다

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	require.Len(t, pub.events, 1)
	assert.Contains(t, pub.events[0].Summary, "CVE-2")
	assert.NotContains(t, pub.events[0].Summary, "CVE-1", "다른 레플리카가 이미 남긴 것은 보내지 않는다")
	assert.Contains(t, pub.events[0].Title, "1건")
}

// 종료로 컨텍스트가 취소돼 보내지 못했으면 남긴 것을 되돌린다 — 다음 기동 뒤 스캔이 보낸다.
func TestScanStackImages_Execute_ReleasesClaimWhenCanceled(t *testing.T) {
	uc, _, results := cveUC(t, []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"))})
	ctx, cancel := context.WithCancel(context.Background())
	uc.notifier = publisherFunc(func(context.Context, sharednotify.Event) error {
		cancel()
		return context.Canceled
	})

	_, err := uc.Execute(ctx, "stk_1")
	require.NoError(t, err)
	assert.Empty(t, results.notifiedIDs("stk_1"))
}

type publisherFunc func(context.Context, sharednotify.Event) error

func (f publisherFunc) Publish(ctx context.Context, e sharednotify.Event) error { return f(ctx, e) }

// 알림을 보내지 못해도 스캔은 성공이다. 실패는 발송 이력에 남고 다시 보내지 않는다 — 채널 하나가
// 계속 실패하면 받은 채널에 매일 같은 알림이 간다.
func TestScanStackImages_Execute_PublishFailureDoesNotFailScan(t *testing.T) {
	uc, pub, results := cveUC(t, []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"))})
	pub.err = errors.New("smtp down")

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Len(t, pub.events, 1)
	assert.Equal(t, []string{"CVE-1"}, results.notifiedIDs("stk_1"))
}

// 이미지 하나의 스캔이 한 번 실패하면 그 이미지의 취약점 행이 사라진다. 다음 스캔이 그 CVE 를
// "새로 발견" 으로 다시 알리면 안 된다 — 한 번 알린 것은 기억한다(실클러스터에서 드러남).
func TestScanStackImages_Execute_DoesNotRenotifyAfterImageScanFailure(t *testing.T) {
	stack := imageScanStack("stk_1", true)
	stack.OrgID = "org-1"
	results := newFakeImageScanResults()
	require.NoError(t, results.ReplaceForStack(context.Background(), "stk_1",
		[]domain.StackImageScan{cveScan("sha256:w", false, nil)}))
	scanner := &fakeImageScanner{scans: []domain.StackImageScan{cveScan("sha256:w", false, nil, critical("CVE-A"))}}
	pub := &fakeCVEPublisher{}
	uc := newScanStackImagesUC([]*domain.Stack{stack}, scanner, results, false).WithNotifier(pub)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	require.Len(t, pub.events, 1, "처음 나온 CVE-A 는 알린다")
	assert.Equal(t, []string{"CVE-A"}, results.notifiedIDs("stk_1"))

	// 다음 스캔: 이미지 스캔이 일시적으로 실패한다(레이어를 받다 끊김).
	scanner.scans = []domain.StackImageScan{{Image: "img-sha256:w", ImageDigest: "sha256:w",
		Status: domain.ImageScanStatusFailed, Error: "unexpected EOF", ScannedAt: time.Now()}}
	_, err = uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	// 그다음 스캔: 다시 성공해 같은 CVE 가 보인다.
	scanner.scans = []domain.StackImageScan{cveScan("sha256:w", false, nil, critical("CVE-A"))}
	_, err = uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Len(t, pub.events, 1, "이미 알린 CVE-A 를 다시 알리지 않는다")
}

// 모든 이미지가 실패해 목록이 하나도 없어도, 이미 알린 적이 있는 스택은 "첫 고지" 가 아니다.
func TestScanStackImages_Execute_NotFirstScanOnceNotified(t *testing.T) {
	stack := imageScanStack("stk_1", true)
	results := newFakeImageScanResults()
	scanner := &fakeImageScanner{scans: []domain.StackImageScan{cveScan("sha256:w", false, nil, critical("CVE-A"))}}
	pub := &fakeCVEPublisher{}
	uc := newScanStackImagesUC([]*domain.Stack{stack}, scanner, results, false).WithNotifier(pub)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	require.Len(t, pub.events, 1)
	assert.Contains(t, pub.events[0].Title, "첫 고지")

	scanner.scans = []domain.StackImageScan{{Image: "img-sha256:w", ImageDigest: "sha256:w",
		Status: domain.ImageScanStatusFailed, Error: "unexpected EOF", ScannedAt: time.Now()}}
	_, err = uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	scanner.scans = []domain.StackImageScan{cveScan("sha256:w", false, nil, critical("CVE-A"), critical("CVE-B"))}
	_, err = uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	require.Len(t, pub.events, 2)
	assert.NotContains(t, pub.events[1].Title, "첫 고지")
	assert.Contains(t, pub.events[1].Summary, "CVE-B")
	assert.NotContains(t, pub.events[1].Summary, "CVE-A")
}
