package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type fakeInstallImageScan struct {
	mu     sync.Mutex
	calls  []string
	report *domain.StackImageScanReport
	err    error
}

func (f *fakeInstallImageScan) Execute(_ context.Context, stackID string) (*domain.StackImageScanReport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, stackID)
	return f.report, f.err
}

func (f *fakeInstallImageScan) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

type recordingStreamer struct {
	*fakeStreamer
	mu       sync.Mutex
	messages []string
}

func (s *recordingStreamer) Stream(ctx context.Context, id string, entry port.LogEntry) {
	s.mu.Lock()
	s.messages = append(s.messages, entry.Level+": "+entry.Message)
	s.mu.Unlock()
	s.fakeStreamer.Stream(ctx, id, entry)
}

func (s *recordingStreamer) find(substr string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.messages {
		if strings.Contains(m, substr) {
			return m
		}
	}
	return ""
}

func installAndWaitForScan(t *testing.T, scan *fakeInstallImageScan) (*fakeStackRepo, *recordingStreamer) {
	t.Helper()
	repo := newFakeStackRepo(&domain.Stack{ID: "stk_scan01", State: domain.StatePending})
	streamer := &recordingStreamer{fakeStreamer: &fakeStreamer{}}
	uc := NewInstallStack(repo, streamer, WithInstalledImageScan(scan))

	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{StackID: "stk_scan01"}))

	deadline := time.Now().Add(dagTotalDuration() + 10*time.Second)
	for time.Now().Before(deadline) && len(scan.called()) == 0 {
		time.Sleep(100 * time.Millisecond)
	}
	return repo, streamer
}

// 설치가 끝나면 설치한 이미지를 스캔해 보고서를 남긴다. 결과는 설치 로그에도 요약한다.
func TestInstallStack_ScansInstalledImagesAfterCompletion(t *testing.T) {
	summary := shareddomain.SeverityCounts{Critical: 1, High: 2}
	scan := &fakeInstallImageScan{report: &domain.StackImageScanReport{
		StackID: "stk_scan01", Status: domain.ImageScanStateScanned, Summary: &summary,
		Items: []domain.StackImageScan{{Status: domain.ImageScanStatusScanned}, {Status: domain.ImageScanStatusFailed}},
	}}

	repo, streamer := installAndWaitForScan(t, scan)

	assert.Equal(t, []string{"stk_scan01"}, scan.called())
	assert.Equal(t, domain.StateCompleted, repo.getState("stk_scan01"),
		"스캔은 설치가 끝난 뒤에 돈다")
	assert.Contains(t, streamer.find("CRITICAL 1"), "HIGH 2")
}

// 보고용이다. 스캔이 실패해도 설치는 완료로 남고, 경고만 남긴다.
func TestInstallStack_ImageScanFailureDoesNotFailInstall(t *testing.T) {
	scan := &fakeInstallImageScan{err: errors.New("scan job timed out")}

	repo, streamer := installAndWaitForScan(t, scan)

	assert.Equal(t, domain.StateCompleted, repo.getState("stk_scan01"))
	assert.True(t, strings.HasPrefix(streamer.find("설치 이미지 취약점 스캔"), "warn:"))
}
