package helm

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GatewayAddressReader 는 스택 게이트웨이 데이터 플레인 Service 의 ClusterIP 를 읽는다
// (port.GatewayAddressReader). 설치가 hostAliases 에 박는 바로 그 주소다(loadGatewayClusterIP).
type GatewayAddressReader struct {
	client   kubernetes.Interface
	interval time.Duration
	timeout  time.Duration
}

// NewGatewayAddressReader 는 대상 클러스터의 kubeconfig 로 만든다.
//
// 복구 직후에는 게이트웨이 Service 가 아직 없을 수 있다 — 덤프에 없으면 Envoy Gateway 컨트롤러가
// 다시 만든다. 그래서 잠시 기다린다.
func NewGatewayAddressReader(kubeconfig []byte) (*GatewayAddressReader, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig 해석: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("클러스터 클라이언트 생성: %w", err)
	}
	return newGatewayAddressReader(client, 3*time.Second, 2*time.Minute), nil
}

func newGatewayAddressReader(client kubernetes.Interface, interval, timeout time.Duration) *GatewayAddressReader {
	return &GatewayAddressReader{client: client, interval: interval, timeout: timeout}
}

// GatewayClusterIP 는 네임스페이스의 게이트웨이 Service 주소다. 기다려도 없으면 오류다.
func (r *GatewayAddressReader) GatewayClusterIP(ctx context.Context, namespace string) (string, error) {
	deadline := time.Now().Add(r.timeout)
	var lastErr error
	for {
		list, err := r.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: gatewayServiceSelector})
		if err == nil {
			for _, svc := range list.Items {
				ip := strings.TrimSpace(svc.Spec.ClusterIP)
				if ip != "" && ip != "None" {
					return ip, nil
				}
			}
			lastErr = fmt.Errorf("네임스페이스 %s 에 게이트웨이 Service(%s)가 없다", namespace, gatewayServiceSelector)
		} else {
			lastErr = fmt.Errorf("게이트웨이 Service 조회(%s): %w", namespace, err)
		}
		if time.Now().After(deadline) {
			return "", lastErr
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(r.interval):
		}
	}
}
