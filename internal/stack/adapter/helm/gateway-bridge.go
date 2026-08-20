package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// gatewayBridgeIngressName 은 브리지 Ingress 이름이다. 스택 네임스페이스 안에
// 하나만 있으므로 고정 이름을 쓴다.
const gatewayBridgeIngressName = "nullus-gateway-bridge"

// gatewayBridgeIngressClass 는 밖에서 요청을 받는 컨트롤러다.
//
// LoadBalancer 가 없는 클러스터(Zadara)에서는 ingress-nginx 를 NodePort 로 세우고
// 그것만 공인 IP 에 붙어 있다. 다른 컨트롤러를 쓰는 환경이 생기면 그때 설정으로
// 뺀다 — 지금 값을 선택지로 만들면 아무도 고르지 않는 손잡이만 늘어난다.
const gatewayBridgeIngressClass = "nginx"

// shouldCreateGatewayBridge 는 이 접속 도메인에 브리지가 의미가 있는지 본다.
//
// .internal 은 로컬 규약이다. kind 에는 ingress 컨트롤러가 없고 포트포워드로
// 게이트웨이에 직접 붙으므로, 규칙을 만들어도 아무도 읽지 않는다.
func shouldCreateGatewayBridge(accessDomain string) bool {
	domain := strings.TrimSpace(accessDomain)
	if domain == "" {
		return false
	}
	return !strings.HasSuffix(strings.ToLower(domain), ".internal")
}

// gatewayBridgeIngressManifest 는 와일드카드 호스트를 스택 게이트웨이로 넘긴다.
//
// Gateway 의 Service 는 LoadBalancer 라, LB 연동이 없는 클러스터에서는 외부 IP 를
// 영원히 받지 못한다. 그래서 밖에서 온 요청은 공인 IP 에 붙어 있는 유일한 것 —
// ingress-nginx — 에 도착하고, 거기에 이 호스트 규칙이 없으면 404 로 끝난다.
// 스택은 정상인데 아무도 들어갈 수 없는 상태다(2026-08-20 운영에서 실측).
//
// 이 Ingress 는 스택 네임스페이스에 산다. Ingress 의 백엔드는 같은 네임스페이스여야
// 하는데, 게이트웨이도 Envoy 도 스택 것이므로 자연히 한자리에 모인다 — 스택을
// 지우면 이 배선도 함께 사라진다.
//
// TLS 는 선언하지 않는다. ingress-nginx 의 기본 인증서(setup-tls.sh 가 거는
// ingress-https)가 와일드카드를 덮으므로, 여기서 secretName 을 적으면 그 시크릿을
// 스택 네임스페이스마다 복사해 둬야 한다.
func gatewayBridgeIngressManifest(namespace, stackLabel, accessDomain, envoyService string) string {
	return fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
  labels:
    nullus.io/stack-name: %s
    nullus.io/type: gateway-bridge
  annotations:
    # 원래 Host 를 그대로 넘긴다. 게이트웨이의 HTTPRoute 가 호스트로 도구를
    # 갈라내므로, 여기서 호스트가 바뀌면 어느 라우트에도 걸리지 않는다.
    nginx.ingress.kubernetes.io/upstream-vhost: $host
spec:
  ingressClassName: %s
  rules:
    - host: "*.%s"
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: %s
                port:
                  number: 80
`, gatewayBridgeIngressName, namespace, stackLabel, gatewayBridgeIngressClass, accessDomain, envoyService)
}

// ensureGatewayBridgeIngress 는 Envoy 데이터플레인을 찾아 브리지를 건다.
//
// Envoy Gateway 는 Service 이름을 envoy-<네임스페이스>-<게이트웨이>-<해시> 로
// 만든다. 해시가 붙으므로 미리 적을 수 없어, 실제로 생긴 것을 조회해 쓴다.
//
// 실패해도 설치를 멈추지 않는다. 클러스터 안에서는 스택이 정상 동작하고, 밖에서
// 들어오는 길만 없는 상태이기 때문이다 — 그 배선은 나중에 손으로도 걸 수 있다.
func (o *Orchestrator) ensureGatewayBridgeIngress(ctx context.Context, namespace, accessDomain, stackLabel string) error {
	if !shouldCreateGatewayBridge(accessDomain) {
		return nil
	}

	service, err := o.findEnvoyDataPlaneService(ctx, namespace)
	if err != nil {
		return err
	}

	return o.applyManifest(ctx, namespace,
		gatewayBridgeIngressManifest(namespace, stackLabel, accessDomain, service))
}

// findEnvoyDataPlaneService 는 이 네임스페이스의 게이트웨이가 만든 Service 를 찾는다.
//
// 게이트웨이 이름으로 거르지 않는다. UI 설치는 마법사가 매니페스트를 보내므로
// 서버가 그 이름을 모른다 — 스택 네임스페이스에는 게이트웨이가 하나뿐이라
// 네임스페이스 라벨만으로 충분하다(포트포워드 스크립트도 같은 대비책을 쓴다).
//
// Envoy Gateway 는 Gateway 를 본 뒤에야 데이터플레인을 만들므로 잠깐 기다린다.
func (o *Orchestrator) findEnvoyDataPlaneService(ctx context.Context, namespace string) (string, error) {
	selector := fmt.Sprintf("gateway.envoyproxy.io/owning-gateway-namespace=%s", namespace)

	var lastErr error
	for attempt := 0; attempt < 20; attempt++ {
		out, err := o.runKubectl(ctx, "get", "svc", "-n", namespace, "-l", selector, "-o", "json")
		if err == nil {
			if name := firstServiceName(out); name != "" {
				return name, nil
			}
			lastErr = fmt.Errorf("네임스페이스 %s 에 게이트웨이 데이터플레인 Service 가 아직 없습니다", namespace)
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	slog.Warn("gateway data-plane service not found", "namespace", namespace, "error", lastErr)
	return "", lastErr
}

func firstServiceName(raw []byte) string {
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return ""
	}
	if len(list.Items) == 0 {
		return ""
	}
	return strings.TrimSpace(list.Items[0].Metadata.Name)
}

// stackAccessIdentity 는 브리지가 필요로 하는 스택의 신원이다.
//
// 접속 도메인은 어떤 호스트를 받을지, 라벨은 스택을 지울 때 이 Ingress 도 함께
// 회수될지를 정한다.
func (o *Orchestrator) stackAccessIdentity() (accessDomain, stackLabel string) {
	cfg := o.currentStackConfig()
	if cfg == nil {
		return "", ""
	}
	accessDomain = strings.TrimSpace(cfg.AccessDomain)
	stackLabel = strings.TrimSpace(strings.TrimSuffix(accessDomain, ".internal"))
	if stackLabel == "" {
		stackLabel = "nullus-stack"
	}
	return accessDomain, stackLabel
}
