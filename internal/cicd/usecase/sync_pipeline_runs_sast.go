package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// WithSASTResults 는 분석 단계의 결과를 남기도록 배선한다.
func (uc *SyncPipelineRuns) WithSASTResults(repo port.SASTResultRepository) *SyncPipelineRuns {
	uc.sastResults = repo
	return uc
}

// recordSASTResults 는 분석 단계가 남긴 리포트로 Quality Gate 판정·조건·지표를 남긴다.
//
// 리포트를 읽은 실행만 남긴다. 이미지 스캔과 달리 단계 결과만으로 판정하지 않는다 — 단계에는
// 정책(경고·장애 허용)이 섞여 있어, 리포트 없이는 "게이트에 걸렸다" 와 "분석을 못 했다" 를
// 가를 수 없다. 리포트를 남기기 전에 만든 파이프라인도 그래서 기록이 없다. 단계 상태는
// 실행 기록에 따로 있다.
//
// 기록 실패로 실행 기록 동기화를 실패시키지 않는다.
func (uc *SyncPipelineRuns) recordSASTResults(
	ctx context.Context,
	input SyncPipelineRunsInput,
	pipelineID string,
	builds []port.CIBuild,
) {
	if uc.sastResults == nil || uc.artifacts == nil {
		return
	}
	recorded := uc.recordedSAST(ctx, pipelineID)

	for _, b := range builds {
		for _, st := range b.Stages {
			if port.StageKey(st.Name) != sastStageKey {
				continue
			}
			if _, finished := domain.GateResultFromStageStatus(string(st.Status)); !finished {
				continue // 도는 중·취소·건너뜀은 판정하지 않는다
			}
			deploymentID := runDeploymentID(pipelineID, b.Number)
			id := domain.SASTResultID(deploymentID)
			analyzedAt := stageStartedAt(b, st)
			if at, ok := recorded[id]; ok && at.Equal(analyzedAt) {
				// 끝난 단계의 리포트는 바뀌지 않는다. 화면을 열 때마다 다시 내려받지 않는다.
				// 시작 시각이 다르면 잡을 다시 돌린 것이다(GitLab Retry · GitHub Re-run) — 다시 읽는다.
				continue
			}

			ref := port.CIArtifactRef{
				JobName: input.JobName,
				Branch:  input.Branch,
				Build:   b,
				Stage:   st,
				Name:    port.SASTReportArtifact,
				Path:    port.SASTReportFile,
			}
			raw, found, err := uc.artifacts.ReadArtifact(ctx, ref)
			if err != nil {
				slog.Warn("SAST 리포트를 읽지 못했습니다",
					"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
				continue
			}
			if !found {
				continue
			}
			report, err := domain.ParseSASTReport(raw)
			if err != nil {
				slog.Warn("SAST 리포트 형식이 맞지 않습니다",
					"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
				continue
			}
			gate, ok := domain.SASTGateFromStageAndReport(string(st.Status), report)
			if !ok {
				continue
			}

			result := &domain.SASTResult{
				ID:                id,
				PipelineID:        pipelineID,
				DeploymentID:      deploymentID,
				ProjectKey:        report.ProjectKey,
				AnalysisID:        report.AnalysisID,
				QualityGateStatus: report.QualityGateStatus,
				GateResult:        gate,
				Conditions:        report.Conditions,
				Metrics:           report.Metrics,
				DashboardURL:      domain.SASTDashboardURL(input.SASTWebURL, report.ProjectKey, report.DashboardURL),
				AnalyzedAt:        analyzedAt,
			}
			if err := uc.sastResults.Upsert(ctx, result); err != nil {
				slog.Warn("SAST 결과 기록 실패",
					"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
			}
		}
	}
}

// recordedSAST 는 이미 남긴 분석 결과의 분석 시각(단계 시작 시각)이다. 조회에 실패하면 비어
// 있다 — 리포트를 한 번 더 내려받는 편이 기록을 빠뜨리는 것보다 낫다.
func (uc *SyncPipelineRuns) recordedSAST(ctx context.Context, pipelineID string) map[string]time.Time {
	existing, err := uc.sastResults.ListByPipelineID(ctx, pipelineID)
	if err != nil {
		return nil
	}
	out := make(map[string]time.Time, len(existing))
	for _, r := range existing {
		if r != nil {
			out[r.ID] = r.AnalyzedAt
		}
	}
	return out
}

// stageStartedAt 은 단계가 시작한 시각이다. CI 가 단계 시각을 주지 않으면 빌드 시각이다.
func stageStartedAt(b port.CIBuild, st port.CIStage) time.Time {
	if !st.StartedAt.IsZero() {
		return st.StartedAt
	}
	return b.StartedAt
}
