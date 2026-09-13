package scaffold

import (
	"fmt"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	// defaultScannerImage 는 CI 잡이 쓸 Trivy CLI 이미지다.
	//
	// 취약점 DB 는 들어 있지 않다 — client 로 붙어 서버에 매칭을 맡기므로
	// 필요 없다. 에어갭 설치는 내부 미러 경로로 파이프라인 변수를 덮는다.
	defaultScannerImage = "aquasec/trivy:0.74.0"

	// scanReportFile 은 실행 기록 동기화가 같은 이름으로 찾는다.
	scanReportFile = port.ImageScanReportFile
)

// scanPolicyVariableNames 는 플랫폼이 푸시하는 정책 변수다.
var scanPolicyVariableNames = []string{
	port.ScanSeverityVariable,
	port.ScanIgnoreUnfixedVariable,
	port.ScanOnUnreachableVariable,
}

// scanScriptLines 는 CI 3종이 공통으로 실행하는 스캔 명령이다.
//
// 두 줄인 이유가 있다. 한 번은 사람이 볼 리포트를 남기고, 한 번은 게이트로
// 판정한다 — 한 번에 하면 차단된 실행에서 리포트가 비어 무엇에 걸렸는지 알 수 없다.
//
// 정책은 파일에 박지 않고 변수로 읽는다. 플랫폼이 스택 정책을 CI 변수로 푸시하므로
// 정책을 바꿔도 재스캐폴딩이 필요 없다. 변수가 없으면(아직 한 번도 푸시하지 않은
// 파이프라인) 설계 §6 의 기본값을 쓴다.
func scanScriptLines() []string {
	image := `"$IMAGE_REPOSITORY:$IMAGE_TAG"`
	server := fmt.Sprintf(`--server "$%s" --scanners vuln`, port.ScanServerVariable)
	unfixed := fmt.Sprintf(`$([ "${%s:-true}" = "true" ] && echo --ignore-unfixed)`,
		port.ScanIgnoreUnfixedVariable)
	defaults := domain.DefaultScanPolicy()

	return []string{
		// 리포트. 심각도를 거르지 않는다 — 거르면 HIGH 가 리포트에 없어 경고가
		// 영원히 나오지 않고 대시보드의 HIGH 건수가 늘 0 이다.
		//
		// 여기서 실패하면 스캔 자체를 못 한 것이다(스캐너 도달 불가 등). 기본은
		// 멈추고, 정책이 allow 면 통과시킨다 — 긴급 배포용 우회로다 (설계 §6.1).
		fmt.Sprintf(`if ! trivy image %s %s --format json --output %s %s; then `+
			`if [ "${%s:-%s}" = "%s" ]; then echo "이미지 스캔을 수행하지 못했습니다 — 스캐너 장애 허용 정책에 따라 통과시킵니다"; exit 0; fi; `+
			`echo "이미지 스캔을 수행하지 못했습니다 — 스캐너 장애 시 차단 정책입니다"; exit 1; fi`,
			server, unfixed, scanReportFile, image,
			port.ScanOnUnreachableVariable, defaults.UnreachableActionOrDefault(), domain.UnreachableAllow),
		// 게이트. 차단이면 non-zero 로 끝나 파이프라인이 멈춘다.
		fmt.Sprintf(`trivy image %s --severity "${%s:-%s}" %s --exit-code 1 --quiet %s`,
			server, port.ScanSeverityVariable, defaults.TrivySeverities(), unfixed, image),
	}
}
