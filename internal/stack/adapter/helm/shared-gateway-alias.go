package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// sharedGatewayProxyServiceName 은 밖에서 들어오는 배선이 가리키는 고정 이름이다.
//
// Envoy Gateway 는 데이터플레인 Service 를 envoy-<네임스페이스>-<게이트웨이>-<해시>
// 로 만든다. 해시가 붙으므로 차트나 ingress 규칙이 그 이름을 미리 적을 수 없고,
// 결국 사람이 한 번 조회해 손으로 옮겨 적어야 했다. 셀렉터와 포트를 그대로 복사한
// 별칭을 두면 그 배선이 상수를 가리킬 수 있다.
const sharedGatewayProxyServiceName = "nullus-gateway-proxy"

// sharedGatewayProxyAliasManifest 는 실제 데이터플레인 Service 에서 별칭을 만든다.
//
// 이름 규칙이나 포트 번호를 짐작하지 않고 복사한다 — Envoy Gateway 가 내부 규칙을
// 바꿔도(권한 포트를 10000 더해 매핑하는 것 등) 별칭이 따라간다.
func sharedGatewayProxyAliasManifest(serviceListJSON []byte) (string, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Selector map[string]string `json:"selector"`
				Ports    []struct {
					Name       string `json:"name"`
					Port       int    `json:"port"`
					Protocol   string `json:"protocol"`
					TargetPort any    `json:"targetPort"`
				} `json:"ports"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal(serviceListJSON, &list); err != nil {
		return "", fmt.Errorf("parse gateway data-plane service: %w", err)
	}
	if len(list.Items) == 0 {
		return "", fmt.Errorf("게이트웨이 데이터플레인 Service 를 찾지 못했습니다")
	}

	source := list.Items[0]
	if len(source.Spec.Selector) == 0 {
		return "", fmt.Errorf("데이터플레인 Service %q 에 셀렉터가 없습니다", source.Metadata.Name)
	}

	var b strings.Builder
	b.WriteString("apiVersion: v1\nkind: Service\nmetadata:\n")
	fmt.Fprintf(&b, "  name: %s\n", sharedGatewayProxyServiceName)
	fmt.Fprintf(&b, "  namespace: %s\n", domain.SharedGatewayNamespace)
	b.WriteString("  annotations:\n")
	fmt.Fprintf(&b, "    nullus.io/alias-of: %s\n", source.Metadata.Name)
	b.WriteString("spec:\n  type: ClusterIP\n  selector:\n")
	for _, key := range sortedKeys(source.Spec.Selector) {
		fmt.Fprintf(&b, "    %s: %s\n", key, source.Spec.Selector[key])
	}
	b.WriteString("  ports:\n")
	for _, port := range source.Spec.Ports {
		fmt.Fprintf(&b, "    - name: %s\n      port: %d\n", strings.TrimSpace(port.Name), port.Port)
		if protocol := strings.TrimSpace(port.Protocol); protocol != "" {
			fmt.Fprintf(&b, "      protocol: %s\n", protocol)
		}
		if target := formatTargetPort(port.TargetPort); target != "" {
			fmt.Fprintf(&b, "      targetPort: %s\n", target)
		}
	}
	return b.String(), nil
}

func formatTargetPort(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%d", int(v))
	case int:
		return fmt.Sprintf("%d", v)
	default:
		return ""
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// ensureSharedGatewayProxyAlias 는 별칭 Service 를 만들거나 최신 상태로 맞춘다.
//
// Envoy Gateway 는 Gateway 를 본 뒤에야 데이터플레인을 만들므로 잠깐 기다린다.
// 끝내 못 찾아도 설치를 멈추지 않는다 — 별칭이 없으면 밖에서 들어오는 배선만
// 손으로 해야 하고, 클러스터 안에서는 스택이 정상 동작한다.
func (o *Orchestrator) ensureSharedGatewayProxyAlias(ctx context.Context) error {
	selector := fmt.Sprintf(
		"gateway.envoyproxy.io/owning-gateway-name=%s,gateway.envoyproxy.io/owning-gateway-namespace=%s",
		domain.SharedGatewayName, domain.SharedGatewayNamespace)

	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		out, err := o.runKubectl(ctx, "get", "svc", "-n", domain.SharedGatewayNamespace,
			"-l", selector, "-o", "json")
		if err == nil {
			manifest, buildErr := sharedGatewayProxyAliasManifest(out)
			if buildErr == nil {
				return o.applyManifest(ctx, domain.SharedGatewayNamespace, manifest)
			}
			lastErr = buildErr
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	slog.Warn("gateway data-plane alias not created",
		"service", sharedGatewayProxyServiceName, "error", lastErr)
	return lastErr
}
