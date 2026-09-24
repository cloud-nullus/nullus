package helm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// gatewayServiceSelector 는 스택 게이트웨이의 데이터 플레인 Service 라벨이다.
// 이름에 게이트웨이 해시가 붙어 예측할 수 없으므로 라벨로 찾는다.
const gatewayServiceSelector = "gateway.envoyproxy.io/owning-gateway-name"

// loadGatewayClusterIP 는 스택 게이트웨이의 ClusterIP 를 읽어 캐시한다.
//
// ClusterIP 를 쓴다. Service 는 LoadBalancer 지만 LB 연동이 없는 클러스터에서는
// 외부 주소를 영영 받지 못하고(kind 가 그렇다), 클러스터 안에서는 kube-proxy 가
// ClusterIP 를 처리하므로 그쪽이 언제나 성립한다.
//
// 게이트웨이 컨트롤러가 Service 를 만들기까지 잠깐 걸린다. 못 읽으면 오류가
// 아니라 빈 값이다 — 이름 해석이 없다고 설치를 멈출 일은 아니다.
func (o *Orchestrator) loadGatewayClusterIP(ctx context.Context, namespace string) string {
	o.mu.Lock()
	if o.gatewayIPLoaded {
		ip := o.gatewayIP
		o.mu.Unlock()
		return ip
	}
	o.mu.Unlock()

	var ip string
	for attempt := 0; attempt < 10; attempt++ {
		out, err := o.runKubectlStdout(ctx, "get", "svc", "-n", namespace,
			"-l", gatewayServiceSelector, "-o", "jsonpath={.items[0].spec.clusterIP}")
		if err == nil {
			ip = strings.TrimSpace(string(out))
			if ip != "" && ip != "None" {
				break
			}
			ip = ""
		}
		select {
		case <-ctx.Done():
			return ""
		case <-time.After(2 * time.Second):
		}
	}

	o.mu.Lock()
	o.gatewayIP = ip
	o.gatewayIPLoaded = true
	o.mu.Unlock()
	return ip
}

func (o *Orchestrator) gatewayClusterIP() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.gatewayIP
}

// reconcileRunnerHostAliases 는 게이트웨이가 선 뒤 러너 설치를 다시 적용한다.
//
// 게이트웨이는 설치 순서의 마지막이라 러너를 깔 때는 그 주소가 아직 없다. 그래서
// 러너는 CI 잡이 스택 도구 이름을 풀 방법을 모른 채 서고, 잡은 clone 단계에서
// "Could not resolve host: gitlab.<도메인>" 으로 끝난다 — 스택은 completed 이고
// 러너 파드도 Running 이라 파이프라인을 돌려 보기 전에는 드러나지 않는다.
//
// helm upgrade --install 은 멱등하고 등록 토큰 조회도 읽기라, 같은 단계를 다시
// 도는 것으로 충분하다.
func (o *Orchestrator) reconcileRunnerHostAliases(ctx context.Context, stackID, namespace, phase string) error {
	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil || !runnerReleaseRequired(*cfg) {
		return nil
	}
	if len(runnerCIHostnames(cfg.AccessDomain)) == 0 {
		return nil
	}

	ip := o.loadGatewayClusterIP(ctx, namespace)
	if ip == "" {
		// 이름 해석이 없으면 CI 잡이 clone 에서 멈추지만, 설치를 실패로 뒤집을
		// 일은 아니다. 조용히 넘어가지 않도록 남긴다.
		slog.Warn("게이트웨이 주소를 읽지 못해 CI 잡의 스택 도구 이름 해석을 배선하지 못했습니다",
			"namespace", namespace, "stack", stackID)
		return nil
	}

	slog.Info("CI 잡이 스택 도구 이름을 풀도록 러너를 다시 적용합니다",
		"namespace", namespace, "gateway_ip", ip, "hosts", runnerCIHostnames(cfg.AccessDomain))
	if err := o.ExecuteStep(ctx, stackID, stepInstallingRunner, phase); err != nil {
		return fmt.Errorf("reconcile runner host aliases: %w", err)
	}
	return nil
}
