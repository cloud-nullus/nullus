package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

type stubPipelineLookup struct {
	pipelines map[string]*domain.Pipeline
}

func (s stubPipelineLookup) Create(context.Context, *domain.Pipeline) error { return nil }
func (s stubPipelineLookup) GetByID(_ context.Context, id string) (*domain.Pipeline, error) {
	if p, ok := s.pipelines[id]; ok {
		return p, nil
	}
	return nil, errors.New("pipeline not found")
}
func (s stubPipelineLookup) List(context.Context, string) ([]*domain.Pipeline, error) {
	return nil, nil
}
func (s stubPipelineLookup) ListByStackID(context.Context, string) ([]*domain.Pipeline, error) {
	return nil, nil
}
func (s stubPipelineLookup) Update(context.Context, *domain.Pipeline) error { return nil }
func (s stubPipelineLookup) Delete(context.Context, string) error           { return nil }

type fixedBundleFactory struct {
	bundle *port.SCMBundle
	err    error
}

func (f fixedBundleFactory) For(context.Context, string) (*port.SCMBundle, error) {
	return f.bundle, f.err
}

func scanVulnsFixture(t *testing.T, arts *stubArtifacts, factoryErr error, ref *domain.ScanReportRef) *ScanVulnerabilities {
	t.Helper()
	scans := newMemScanResults()
	require.NoError(t, scans.Upsert(context.Background(), &domain.ImageScanResult{
		ID: "scan_1", PipelineID: "pip_1", GateResult: domain.GateResultWarn, ScannedAt: time.Now(), ReportRef: ref,
	}))
	require.NoError(t, scans.Upsert(context.Background(), &domain.ImageScanResult{
		ID: "scan_other", PipelineID: "pip_2", GateResult: domain.GateResultPass, ScannedAt: time.Now(),
	}))
	pipelines := stubPipelineLookup{pipelines: map[string]*domain.Pipeline{
		"pip_1": {ID: "pip_1", Name: "app", StackID: "stk_1"},
	}}
	return NewScanVulnerabilities(pipelines, scans,
		fixedBundleFactory{bundle: &port.SCMBundle{CIArtifacts: arts}, err: factoryErr})
}

var gitlabRef = &domain.ScanReportRef{JobName: "app", Branch: "main", BuildNumber: 7, StageID: "11",
	StageName: "image-scan", Artifact: port.ImageScanReportArtifact, Path: port.ImageScanReportFile}

// 목록은 볼 때 CI 리포트를 다시 읽어 만든다. 기록한 위치 그대로 읽는다.
func TestScanVulnerabilities_ReadsReportFromRecordedRef(t *testing.T) {
	raw, _ := nodeReport(t)
	arts := &stubArtifacts{data: raw, found: true}
	uc := scanVulnsFixture(t, arts, nil, gitlabRef)

	page, err := uc.Execute(context.Background(), "pip_1", "scan_1",
		shareddomain.VulnerabilityFilter{Class: shareddomain.VulnerabilityClassOS})
	require.NoError(t, err)

	assert.Equal(t, shareddomain.VulnerabilityListAvailable, page.Status)
	assert.Equal(t, 18, page.Total, "베이스 이미지(debian) OS 패키지만")
	assert.Len(t, page.Targets, 2)
	require.Len(t, arts.refs, 1)
	assert.Equal(t, "11", arts.refs[0].Stage.ID)
	assert.Equal(t, 7, arts.refs[0].Build.Number)
}

func TestScanVulnerabilities_Unavailable(t *testing.T) {
	tests := []struct {
		name       string
		arts       *stubArtifacts
		factoryErr error
		ref        *domain.ScanReportRef
		reason     string
	}{
		// 보관 기간이 지나 CI 가 리포트를 지웠다. 건수는 기록에 남아 있다.
		{"리포트 만료", &stubArtifacts{found: false}, nil, gitlabRef, shareddomain.VulnerabilityReasonReportExpired},
		{"리포트 위치 없음", &stubArtifacts{found: true}, nil, nil, shareddomain.VulnerabilityReasonReportMissing},
		{"CI 번들 실패", &stubArtifacts{found: true}, errors.New("stack installing"), gitlabRef, shareddomain.VulnerabilityReasonCIUnreachable},
		{"산출물 조회 실패", &stubArtifacts{err: errors.New("timeout")}, nil, gitlabRef, shareddomain.VulnerabilityReasonCIUnreachable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := scanVulnsFixture(t, tc.arts, tc.factoryErr, tc.ref).
				Execute(context.Background(), "pip_1", "scan_1", shareddomain.VulnerabilityFilter{})
			require.NoError(t, err)
			assert.Equal(t, shareddomain.VulnerabilityListUnavailable, page.Status)
			assert.Equal(t, tc.reason, page.Reason)
			assert.Empty(t, page.Items)
		})
	}
}

func TestScanVulnerabilities_NotFound(t *testing.T) {
	uc := scanVulnsFixture(t, &stubArtifacts{}, nil, gitlabRef)

	_, err := uc.Execute(context.Background(), "pip_missing", "scan_1", shareddomain.VulnerabilityFilter{})
	assert.ErrorIs(t, err, ErrScanPipelineNotFound)

	_, err = uc.Execute(context.Background(), "pip_1", "scan_missing", shareddomain.VulnerabilityFilter{})
	assert.ErrorIs(t, err, ErrImageScanNotFound)

	// 다른 파이프라인의 스캔은 이 파이프라인에서 볼 수 없다.
	_, err = uc.Execute(context.Background(), "pip_1", "scan_other", shareddomain.VulnerabilityFilter{})
	assert.ErrorIs(t, err, ErrImageScanNotFound)
}
