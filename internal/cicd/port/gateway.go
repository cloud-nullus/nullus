package port

import "context"

// GatewayLocator 는 스택 네임스페이스에 실제로 있는 게이트웨이를 찾는다.
//
// 스택은 게이트웨이를 만드는 길이 둘이고 이름 규칙도 둘이다 — 화면 설치는 `<스택 이름>-gateway`, 화면이
// 게이트웨이 매니페스트를 보내지 않는 설치(런북 stack-up 등)는 서버 기본값이 접근 도메인에서 만든다.
// 앱 HTTPRoute 가 이름을 짐작하면 둘 중 하나는 틀리고, 틀린 이름을 가리키는 라우트는 어느 컨트롤러도
// 집지 않아 status 도 비고 주소는 404 다.
type GatewayLocator interface {
	// LocateGateway 는 namespace 의 게이트웨이 가운데 앱이 붙을 것을 고른다. preferred 가 있으면 그것을,
	// 없으면 하나뿐인 것(또는 Nullus 가 만든 것 하나)을 돌려준다. 고를 수 없으면 빈 문자열이다.
	LocateGateway(ctx context.Context, kubeconfig []byte, namespace, preferred string) (string, error)
}
