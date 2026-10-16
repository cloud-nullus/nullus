// Package alertmanager 는 스택 Alertmanager 의 활성 경보를 읽는 어댑터다(nullus-plan#87).
//
// 플랫폼은 스택 클러스터의 kubeconfig 만으로 Kubernetes API 서버 프록시를 거쳐 Alertmanager 에 닿는다 —
// OpenBao 접근과 같은 길이다(internal/shared/secrets/apiserver_proxy.go). 스택 쪽에 인바운드 경로나
// 웹훅 인증을 새로 열지 않으므로 에어갭 · 다중 클러스터에서도 그대로 된다.
package alertmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

const (
	// serviceName 은 스택이 까는 kube-prometheus-stack 릴리스의 Alertmanager Service 다(릴리스 이름
	// "kube-prometheus-stack" + "-alertmanager", 스택 모듈의 KubePrometheusStackReleaseName).
	serviceName = "kube-prometheus-stack-alertmanager"
	servicePort = "9093"
	// requestTimeout 은 스택 하나를 읽는 데 쓰는 시간이다. 닿지 않는 클러스터 하나가 다른 스택을 막지 않게 짧다.
	requestTimeout = 15 * time.Second
)

// KubeconfigProvider 는 클러스터의 kubeconfig 다(스택 모듈의 것을 main 이 넘긴다).
type KubeconfigProvider interface {
	GetKubeconfig(ctx context.Context, clusterID string) ([]byte, error)
}

// Source 는 port.ActiveAlertSource 다.
type Source struct {
	kubeconfigs KubeconfigProvider
}

func NewSource(kubeconfigs KubeconfigProvider) *Source {
	return &Source{kubeconfigs: kubeconfigs}
}

var _ port.ActiveAlertSource = (*Source)(nil)

type apiAlert struct {
	Fingerprint string            `json:"fingerprint"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	Status      struct {
		State string `json:"state"`
	} `json:"status"`
}

// ActiveAlerts 는 Alertmanager API v2 의 활성 경보다. 침묵(silenced) · 억제(inhibited)된 것도 가져와
// Suppressed 로 표시한다 — 수집기는 그것을 알리지 않지만 "보인 것" 으로 쳐서 해제하지 않는다. 빼고
// 가져오면 침묵을 건 순간 해제 알림이, 침묵이 끝날 때 발생 알림이 다시 나간다.
func (s *Source) ActiveAlerts(ctx context.Context, stack domain.MonitoredStack) ([]domain.SourceAlert, error) {
	kubeconfig, err := s.kubeconfigs.GetKubeconfig(ctx, stack.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("클러스터 %s kubeconfig: %w", stack.ClusterID, err)
	}
	if len(kubeconfig) == 0 {
		return nil, errors.New("클러스터 kubeconfig 가 없습니다")
	}
	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig 읽기: %w", err)
	}
	restCfg.Timeout = requestTimeout
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("쿠버네티스 클라이언트: %w", err)
	}
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	raw, err := clientset.CoreV1().Services(stack.Namespace).
		ProxyGet("http", serviceName, servicePort, "api/v2/alerts", map[string]string{
			"active": "true",
		}).DoRaw(reqCtx)
	if err != nil {
		return nil, fmt.Errorf("Alertmanager(%s/%s) 경보 조회: %w", stack.Namespace, serviceName, err)
	}
	var alerts []apiAlert
	if err := json.Unmarshal(raw, &alerts); err != nil {
		return nil, fmt.Errorf("Alertmanager 응답 해석: %w", err)
	}
	out := make([]domain.SourceAlert, 0, len(alerts))
	for _, a := range alerts {
		out = append(out, domain.SourceAlert{
			Fingerprint: a.Fingerprint, Labels: a.Labels, Annotations: a.Annotations, StartsAt: a.StartsAt,
			Suppressed: a.Status.State == "suppressed",
		})
	}
	return out, nil
}
