package scaffold

import "fmt"

const (
	// defaultScannerImage 는 CI 잡이 쓸 Trivy CLI 이미지다.
	//
	// 취약점 DB 는 들어 있지 않다 — client 로 붙어 서버에 매칭을 맡기므로
	// 필요 없다. 에어갭 설치는 내부 미러 경로로 파이프라인 변수를 덮는다.
	defaultScannerImage = "aquasec/trivy:0.74.0"

	// defaultScanSeverity 는 차단 심각도의 기본값이다.
	//
	// HIGH 까지 막으면 흔한 베이스 이미지로 첫 배포가 안 된다. CRITICAL 만
	// 막고 HIGH 는 결과에 남긴다.
	defaultScanSeverity = "CRITICAL"

	scanReportFile = "trivy-report.json"
)

// scanScriptLines 는 CI 3종이 공통으로 실행하는 스캔 명령이다.
//
// 두 줄인 이유가 있다. 한 번은 사람이 볼 리포트를 남기고, 한 번은 게이트로
// 판정한다 — 한 번에 하면 차단된 실행에서 리포트가 비어 무엇에 걸렸는지
// 알 수 없다.
//
// --severity 와 --ignore-unfixed 를 변수로 읽는 것이 핵심이다. 값을 박으면
// 정책을 바꿀 때마다 모든 파이프라인을 다시 스캐폴딩해야 한다.
func scanScriptLines() []string {
	image := `"$IMAGE_REPOSITORY:$IMAGE_TAG"`
	common := fmt.Sprintf(`--server "$%s" --scanners vuln --severity "$%s"`,
		scanServerVar, scanSeverityVar)
	unfixed := fmt.Sprintf(`$([ "$%s" = "true" ] && echo --ignore-unfixed)`, scanIgnoreUnfixedV)

	return []string{
		// 리포트를 먼저 남긴다. exit-code 0 이라 여기서는 멈추지 않는다.
		fmt.Sprintf(`trivy image %s %s --format json --output %s %s`,
			common, unfixed, scanReportFile, image),
		// 게이트. 차단이면 non-zero 로 끝나 파이프라인이 멈춘다.
		fmt.Sprintf(`trivy image %s %s --exit-code 1 --quiet %s`,
			common, unfixed, image),
	}
}
