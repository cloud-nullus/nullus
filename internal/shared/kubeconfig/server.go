// Package kubeconfig 는 kubectl·helm 을 실행하기 전에 kubeconfig 를 검사한다.
package kubeconfig

import (
	"errors"
	"fmt"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

// ErrNoServer 는 kubeconfig 가 접속할 API 서버를 가리키지 않을 때의 오류다.
var ErrNoServer = errors.New("kubeconfig 에 API 서버 주소가 없습니다")

// RequireServer 는 kubeconfig 의 현재 컨텍스트가 서버 주소가 있는 클러스터를
// 가리키는지 확인한다. 네트워크는 쓰지 않는다.
//
// kubectl 은 서버를 못 찾으면 오류를 내지 않고 http://localhost:8080 으로
// 폴백한다. 그 자리에 무엇이 떠 있든 apply·delete 가 그쪽으로 간다 — 응답이
// 없으면 타임아웃 × 재시도로 매달리고, 응답하면 엉뚱한 클러스터를 건드린다.
// 실행 전에 막는다.
func RequireServer(raw []byte) error {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fmt.Errorf("%w: kubeconfig 가 비어 있습니다", ErrNoServer)
	}
	cfg, err := clientcmd.Load(raw)
	if err != nil {
		return fmt.Errorf("%w: kubeconfig 를 읽을 수 없습니다: %v", ErrNoServer, err)
	}

	contextName := cfg.CurrentContext
	if contextName == "" {
		return fmt.Errorf("%w: current-context 가 없습니다", ErrNoServer)
	}
	kubeContext, ok := cfg.Contexts[contextName]
	if !ok || kubeContext == nil {
		return fmt.Errorf("%w: 컨텍스트 %q 가 없습니다", ErrNoServer, contextName)
	}
	cluster, ok := cfg.Clusters[kubeContext.Cluster]
	if !ok || cluster == nil {
		return fmt.Errorf("%w: 클러스터 %q 가 없습니다", ErrNoServer, kubeContext.Cluster)
	}
	if strings.TrimSpace(cluster.Server) == "" {
		return fmt.Errorf("%w: 클러스터 %q", ErrNoServer, kubeContext.Cluster)
	}
	return nil
}
