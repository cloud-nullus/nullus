package usecase

import (
	"context"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// OTLPEndpointFor 는 배포되는 앱에 넣어 줄 수집기 주소를 찾는다.
//
// 배포 경로가 둘이라(파이프라인 배포, 직접 배포) 각자 구현하면 한쪽만 고쳐져
// "어떤 경로로 배포했느냐에 따라 추적이 되기도 하고 안 되기도 하는" 상태가 된다.
// 그래서 판단을 여기 하나로 모은다.
//
// 조회에 실패하거나 스택에 수집기가 없으면 빈 값을 돌려준다. 관측은 부가
// 기능이고, 그것 때문에 배포가 실패하면 더 나쁘다. 빈 값이면 호출부가 관련
// 환경변수를 아예 넣지 않는다 — 닿지 않는 주소를 박으면 앱이 영원히 재시도한다.
func OTLPEndpointFor(ctx context.Context, reader port.StackReader, stackID string) string {
	if reader == nil || strings.TrimSpace(stackID) == "" {
		return ""
	}
	summary, err := reader.GetStackSummary(ctx, stackID)
	if err != nil || summary == nil {
		return ""
	}
	return summary.OTLPEndpoint
}
