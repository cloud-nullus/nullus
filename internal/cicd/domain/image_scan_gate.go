package domain

// GateFromStageAndReport 는 스캔 단계 결과와 리포트로 판정한다.
//
// 단계 결과만으로는 "막혔다" 와 "스캔이 못 돌았다" 를 가를 수 없다 — Trivy 는
// 서버에 닿지 못해도 게이트와 같은 exit 1 로 끝난다. 리포트가 차단 사유를 담고
// 있을 때만 차단이다.
//
// report 가 nil 이면 리포트를 모른다는 뜻이다(읽지 못함, 산출물 조회 미배선).
// 그때는 단계 결과를 따른다. 끝나지 않은 단계는 판정하지 않는다.
func GateFromStageAndReport(stageStatus string, report *TrivyReportSummary, policy ScanPolicy) (GateResult, bool) {
	fromStage, ok := GateResultFromStageStatus(stageStatus)
	if !ok {
		return "", false
	}
	if report == nil {
		return fromStage, true
	}

	verdict := EvaluateGate(report, policy)
	if fromStage == GateResultPass {
		// 게이트가 막지 않았다. 리포트에 CRITICAL 이 있어도(파이프라인의 정책 변수가
		// 달랐다) 배포는 나갔으므로 차단으로 세지 않는다 — 경고로 남긴다.
		if verdict == GateResultBlock {
			return GateResultWarn, true
		}
		return verdict, true
	}
	if verdict == GateResultBlock {
		return GateResultBlock, true
	}
	// 단계는 실패했는데 리포트에 차단 사유가 없다 — 취약점이 아니라 스캔이 실패했다.
	return GateResultError, true
}

// GateWhenReportMissing 은 리포트가 확실히 없을 때의 판정이다.
//
// 스캔 명령은 리포트부터 쓴다. 실패했는데 리포트가 없으면 스캐너에 닿지 못한
// 것이다 — 차단으로 세면 스캐너 장애가 "취약한 배포" 로 보인다. 성공했는데
// 없으면 게이트는 통과했고 업로드만 빠진 것이다.
func GateWhenReportMissing(stageStatus string) (GateResult, bool) {
	fromStage, ok := GateResultFromStageStatus(stageStatus)
	if !ok {
		return "", false
	}
	if fromStage == GateResultBlock {
		return GateResultError, true
	}
	return fromStage, true
}
