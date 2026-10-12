package port

import "context"

// GatewayAddressReader 는 스택 게이트웨이(Envoy 데이터 플레인 Service)의 클러스터 내 주소를 읽는다.
// 설치가 러너·Argo CD 의 hostAliases 에 박는 바로 그 주소다.
type GatewayAddressReader interface {
	GatewayClusterIP(ctx context.Context, namespace string) (string, error)
}
