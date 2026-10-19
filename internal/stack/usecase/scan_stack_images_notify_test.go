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

func cveUC(t *testing.T, scans []domain.StackImageScan, previous []domain.StackImageScan) (*ScanStackImages, *fakeCVEPublisher, *fakeImageScanResults) {
	t.Helper()
	stack := imageScanStack("stk_1", true)
	stack.Name, stack.OrgID = "devsecops", "org-1"
	results := newFakeImageScanResults()
	if previous != nil {
		require.NoError(t, results.ReplaceForStack(context.Background(), "stk_1", previous))
	}
	pub := &fakeCVEPublisher{}
	uc := newScanStackImagesUC([]*domain.Stack{stack}, &fakeImageScanner{scans: scans}, results, false).WithNotifier(pub)
	return uc, pub, results
}

// 재스캔에서 새로 생긴 "수정판 있는 CRITICAL" 만 알린다 — 매일 같은 내용이 오면 아무도 읽지 않는다.
func TestScanStackImages_Execute_NotifiesNewFixableCriticals(t *testing.T) {
	previous := []domain.StackImageScan{cveScan("sha256:w", true, nil, critical("CVE-OLD"))}
	scans := []domain.StackImageScan{cveScan("sha256:w", true, []string{"gitlab.nullus.local"},
		critical("CVE-OLD"), critical("CVE-NEW"),
		shareddomain.ImageVulnerability{ID: "CVE-HIGH", Severity: "HIGH", FixedVersion: "2"},
		shareddomain.ImageVulnerability{ID: "CVE-NOFIX", Severity: "CRITICAL"})}
	uc, pub, _ := cveUC(t, scans, previous)

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
	uc, pub, _ := cveUC(t, scans, nil)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	require.Len(t, pub.events, 1)
	summary := pub.events[0].Summary
	assert.Contains(t, summary, "img-sha256:1, img-sha256:2, img-sha256:3 외 2개")
	assert.NotContains(t, summary, "img-sha256:4")
}

func TestScanStackImages_Execute_InternalOnlyIsWarning(t *testing.T) {
	previous := []domain.StackImageScan{cveScan("sha256:w", false, nil)}
	uc, pub, _ := cveUC(t, []domain.StackImageScan{cveScan("sha256:w", false, nil, critical("CVE-NEW"))}, previous)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	require.Len(t, pub.events, 1)
	assert.Equal(t, sharednotify.SeverityWarning, pub.events[0].Severity)
}

// 첫 스캔은 설치 시점에 이미 있던 것을 한 번에 알린다 — 한 건으로 묶고, 첫 스캔이라고 밝힌다.
func TestScanStackImages_Execute_FirstScanSummarizesOnce(t *testing.T) {
	scans := []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"), critical("CVE-2"))}
	uc, pub, _ := cveUC(t, scans, nil)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)

	require.Len(t, pub.events, 1)
	assert.Contains(t, pub.events[0].Title, "첫 스캔")
	assert.Contains(t, pub.events[0].Title, "2건")
}

func TestScanStackImages_Execute_NoEventWhenNothingNew(t *testing.T) {
	previous := []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"))}
	uc, pub, _ := cveUC(t, []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"))}, previous)

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Empty(t, pub.events)
}

// 지난 결과를 읽지 못하면 무엇이 새것인지 모른다 — 전부 새것으로 알리지 않는다.
func TestScanStackImages_Execute_NoEventWhenPreviousUnknown(t *testing.T) {
	uc, pub, results := cveUC(t, []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"))}, nil)
	results.knownErr = errors.New("db down")

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err, "알림을 못 판단해도 스캔 결과는 저장한다")
	assert.Empty(t, pub.events)
	stored, _ := results.ListByStack(context.Background(), "stk_1")
	assert.Len(t, stored, 1)
}

// 알림을 보내지 못해도 스캔은 성공이다.
func TestScanStackImages_Execute_PublishFailureDoesNotFailScan(t *testing.T) {
	uc, pub, _ := cveUC(t, []domain.StackImageScan{cveScan("sha256:a", false, nil, critical("CVE-1"))}, nil)
	pub.err = errors.New("smtp down")

	_, err := uc.Execute(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Len(t, pub.events, 1)
}
