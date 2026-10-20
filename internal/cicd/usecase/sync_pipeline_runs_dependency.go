package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// WithDependencyScans 는 의존성 스캔(SCA) 단계의 결과를 남기도록 배선한다.
func (uc *SyncPipelineRuns) WithDependencyScans(repo port.DependencyScanResultRepository) *SyncPipelineRuns {
	uc.dependencyScans = repo
	return uc
}

// recordDependencyScans 는 의존성 스캔 단계가 끝난 실행의 판정 · 건수 · 검사 대상 수를 남긴다
// (Security Dashboard 설계 §8 G1).
//
// 이미지 스캔과 같은 Trivy 리포트 · 같은 판정 규칙(GateFromStageAndReport · GateWhenReportMissing) · 같은
// 정책이다 — 한 스택에 두 기준이 있으면 같은 취약점이 한쪽에서는 막히고 다른 쪽에서는 지나간다. 리포트를
// 읽지 못하면(CI 장애) 단계 결과로 남기고 건수는 비워 둔다 — 다음 동기화가 다시 읽는다.
//
// 기록 실패로 실행 기록 동기화를 실패시키지 않는다.
func (uc *SyncPipelineRuns) recordDependencyScans(
	ctx context.Context,
	input SyncPipelineRunsInput,
	pipelineID string,
	builds []port.CIBuild,
) {
	if uc.dependencyScans == nil {
		return
	}
	policy := domain.DefaultScanPolicy()
	if input.Policy != nil {
		policy = *input.Policy
	}
	recorded := uc.dependencyScansWithReport(ctx, pipelineID)

	for _, b := range builds {
		for _, st := range b.Stages {
			if port.StageKey(st.Name) != dependencyScanStageKey {
				continue
			}
			deploymentID := runDeploymentID(pipelineID, b.Number)
			id := domain.DependencyScanResultID(deploymentID)

			if st.Status == port.CIStageCanceled {
				// 취소는 스캔 실패가 아니다 — 새 커밋이 앞선 실행을 밀어낸 것이다. 남아 있던 기록을 걷어낸다.
				if err := uc.dependencyScans.Delete(ctx, id); err != nil {
					slog.Warn("취소된 실행의 의존성 스캔 기록 정리 실패",
						"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
				}
				continue
			}
			gate, ok := domain.GateResultFromStageStatus(string(st.Status))
			if !ok {
				continue // 도는 중인 것을 통과로 적지 않는다
			}
			scannedAt := stageStartedAt(b, st)
			if at, ok := recorded[id]; ok && at.Equal(scannedAt) {
				// 끝난 단계의 리포트는 바뀌지 않는다. 화면을 열 때마다 다시 내려받지 않는다.
				// 시작 시각이 다르면 잡을 다시 돌린 것이다(GitLab Retry · GitHub Re-run) — 다시 읽는다.
				continue
			}

			ref := port.CIArtifactRef{
				JobName: input.JobName,
				Branch:  input.Branch,
				Build:   b,
				Stage:   st,
				Name:    port.DependencyScanReportArtifact,
				Path:    port.DependencyScanReportFile,
			}
			result := &domain.DependencyScanResult{
				ID:           id,
				PipelineID:   pipelineID,
				DeploymentID: deploymentID,
				Scanner:      "trivy",
				GateResult:   gate,
				ScannedAt:    scannedAt,
			}
			uc.applyDependencyReport(ctx, result, ref, policy)
			// 리포트를 읽은 실행에만 링크를 건다 — 리포트가 없으면 열어도 없는 파일이다.
			if result.Counts != nil {
				result.ReportURI = uc.reportLink(ref)
				result.ReportRef = reportRefFrom(ref)
			}
			if err := uc.dependencyScans.Upsert(ctx, result); err != nil {
				slog.Warn("의존성 스캔 결과 기록 실패",
					"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
			}
		}
	}
}

// applyDependencyReport 는 리포트를 읽어 판정 · 건수 · 검사 대상 수를 채운다.
func (uc *SyncPipelineRuns) applyDependencyReport(
	ctx context.Context,
	result *domain.DependencyScanResult,
	ref port.CIArtifactRef,
	policy domain.ScanPolicy,
) {
	if uc.artifacts == nil {
		return
	}
	status := string(ref.Stage.Status)

	raw, found, err := uc.artifacts.ReadArtifact(ctx, ref)
	if err != nil {
		slog.Warn("의존성 스캔 리포트를 읽지 못했습니다",
			"pipeline_id", result.PipelineID, "deployment_id", result.DeploymentID, "error", err)
		return
	}
	if !found {
		// 스캔 명령은 리포트부터 쓴다. 리포트가 없으면 스캔을 수행하지 못한 것이다.
		if gate, ok := domain.GateWhenReportMissing(status); ok {
			result.GateResult = gate
		}
		return
	}
	summary, err := domain.ParseTrivyReport(raw)
	if err != nil {
		slog.Warn("의존성 스캔 리포트 형식이 맞지 않습니다",
			"pipeline_id", result.PipelineID, "deployment_id", result.DeploymentID, "error", err)
		return
	}
	if gate, ok := domain.GateFromStageAndReport(status, summary, policy); ok {
		result.GateResult = gate
	}
	// 건수는 판정이 본 것과 같아야 한다 — 수정본 없는 것을 뺐다면 건수에서도 뺀다.
	counts := summary.All
	if policy.IgnoreUnfixed {
		counts = summary.Fixable
	}
	targets := summary.Targets
	result.Counts = &counts
	result.Targets = &targets
	result.ScannerVersion = summary.ScannerVersion
	result.DBUpdatedAt = summary.DBUpdatedAt
}

// dependencyScansWithReport 는 이미 리포트를 읽어 건수가 채워진 기록의 스캔 시각(단계 시작 시각)이다.
// 조회에 실패하면 비어 있다 — 리포트를 한 번 더 내려받는 편이 기록을 빠뜨리는 것보다 낫다. 리포트가 없던
// 실행은 여기 들지 않아 다음 동기화 때 다시 확인한다.
func (uc *SyncPipelineRuns) dependencyScansWithReport(ctx context.Context, pipelineID string) map[string]time.Time {
	existing, err := uc.dependencyScans.ListByPipelineID(ctx, pipelineID)
	if err != nil {
		return nil
	}
	out := make(map[string]time.Time, len(existing))
	for _, r := range existing {
		if r != nil && r.Counts != nil {
			out[r.ID] = r.ScannedAt
		}
	}
	return out
}
