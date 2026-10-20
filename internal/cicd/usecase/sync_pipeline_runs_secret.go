package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// WithSecretScans 는 시크릿 탐지 단계의 결과를 남기도록 배선한다.
func (uc *SyncPipelineRuns) WithSecretScans(repo port.SecretScanResultRepository) *SyncPipelineRuns {
	uc.secretScans = repo
	return uc
}

// recordSecretScans 는 시크릿 탐지 단계가 끝난 실행의 판정 · 탐지 위치를 남긴다(Security Dashboard 설계 §8 G2).
//
// 판정은 리포트에 종료 코드가 없어 단계 결과와 탐지 건수로 추론한다(domain.SecretScanGate). 그래서 리포트를
// 읽어야만 남긴다 — 산출물 조회가 없거나 읽지 못하면 경고와 통과를 가를 수 없다. 리포트가 확실히 없으면
// 판정 불가(error)다. 스택 정책이 off 면 단계는 돌되 바로 통과하므로 남기지 않는다 — "통과" 로 남기면
// 탐지하지 않은 앱이 초록으로 보인다.
//
// 비밀값은 남기지 않는다. 리포트의 Secret · Match 는 파싱하지도 않는다(domain.ParseGitleaksReport).
// 기록 실패로 실행 기록 동기화를 실패시키지 않는다.
func (uc *SyncPipelineRuns) recordSecretScans(
	ctx context.Context,
	input SyncPipelineRunsInput,
	pipelineID string,
	builds []port.CIBuild,
) {
	if uc.secretScans == nil || uc.artifacts == nil {
		return
	}
	policy := domain.DefaultScanPolicy()
	if input.Policy != nil {
		policy = *input.Policy
	}
	if policy.SecretDetectionOrDefault() == domain.SecretDetectionOff {
		return
	}
	recorded := uc.secretScansWithReport(ctx, pipelineID)

	for _, b := range builds {
		for _, st := range b.Stages {
			if port.StageKey(st.Name) != secretScanStageKey {
				continue
			}
			deploymentID := runDeploymentID(pipelineID, b.Number)
			id := domain.SecretScanResultID(deploymentID)

			if st.Status == port.CIStageCanceled {
				// 취소는 스캔 실패가 아니다. 남아 있던 기록을 걷어낸다 — 남기면 대시보드가 판정 불가로 센다.
				if err := uc.secretScans.Delete(ctx, id); err != nil {
					slog.Warn("취소된 실행의 시크릿 탐지 기록 정리 실패",
						"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
				}
				continue
			}
			if _, finished := domain.GateResultFromStageStatus(string(st.Status)); !finished {
				continue // 도는 중인 것을 판정하지 않는다
			}
			scannedAt := stageStartedAt(b, st)
			if at, ok := recorded[id]; ok && at.Equal(scannedAt) {
				// 끝난 단계의 리포트는 바뀌지 않는다. 잡을 다시 돌렸으면 시작 시각이 바뀌어 다시 읽는다.
				continue
			}

			ref := port.CIArtifactRef{
				JobName: input.JobName,
				Branch:  input.Branch,
				Build:   b,
				Stage:   st,
				Name:    port.SecretScanReportArtifact,
				Path:    port.SecretScanReportFile,
			}
			raw, found, err := uc.artifacts.ReadArtifact(ctx, ref)
			if err != nil {
				slog.Warn("시크릿 탐지 리포트를 읽지 못했습니다",
					"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
				continue
			}
			result := &domain.SecretScanResult{
				ID:           id,
				PipelineID:   pipelineID,
				DeploymentID: deploymentID,
				Scanner:      "gitleaks",
				Rules:        []domain.SecretRuleCount{},
				Findings:     []domain.SecretFinding{},
				ScannedAt:    scannedAt,
			}
			if found {
				report, err := domain.ParseGitleaksReport(raw)
				if err != nil {
					slog.Warn("시크릿 탐지 리포트 형식이 맞지 않습니다",
						"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
					continue
				}
				count := report.Count
				result.FindingsCount = &count
				result.Rules = report.Rules
				result.Findings = report.Findings
				result.ReportURI = uc.reportLink(ref)
				result.ReportRef = reportRefFrom(ref)
			}
			gate, ok := domain.SecretScanGate(string(st.Status), result.FindingsCount, policy)
			if !ok {
				continue
			}
			result.GateResult = gate
			if err := uc.secretScans.Upsert(ctx, result); err != nil {
				slog.Warn("시크릿 탐지 결과 기록 실패",
					"pipeline_id", pipelineID, "deployment_id", deploymentID, "error", err)
			}
		}
	}
}

// secretScansWithReport 는 이미 리포트를 읽어 탐지 건수가 채워진 기록의 스캔 시각이다. 조회에 실패하면
// 비어 있다 — 리포트를 한 번 더 내려받는 편이 기록을 빠뜨리는 것보다 낫다.
func (uc *SyncPipelineRuns) secretScansWithReport(ctx context.Context, pipelineID string) map[string]time.Time {
	existing, err := uc.secretScans.ListByPipelineID(ctx, pipelineID)
	if err != nil {
		return nil
	}
	out := make(map[string]time.Time, len(existing))
	for _, r := range existing {
		if r != nil && r.FindingsCount != nil {
			out[r.ID] = r.ScannedAt
		}
	}
	return out
}
