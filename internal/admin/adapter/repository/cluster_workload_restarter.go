package repository

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cloud-nullus/draft/internal/admin/rotation"
)

// ClusterWorkloadRestarter 는 클러스터 ID 로 kubeconfig 를 찾아
// 해당 클러스터의 워크로드를 rolling restart 한다.
//
// 회전 스케줄러는 여러 스택을 다루므로 kubeconfig 가 고정될 수 없다.
// 대상 클러스터를 매번 해석하는 계층을 여기에 둔다.
type ClusterWorkloadRestarter struct {
	kubeconfigProvider KubeconfigDecryptor
}

func NewClusterWorkloadRestarter(provider KubeconfigDecryptor) *ClusterWorkloadRestarter {
	return &ClusterWorkloadRestarter{kubeconfigProvider: provider}
}

func (r *ClusterWorkloadRestarter) RestartForProvider(ctx context.Context, clusterID, namespace, provider string) error {
	if r == nil || r.kubeconfigProvider == nil {
		return nil
	}
	if strings.TrimSpace(clusterID) == "" || strings.TrimSpace(namespace) == "" {
		return nil
	}

	kubeconfig, err := r.kubeconfigProvider.GetKubeconfig(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("클러스터 %s kubeconfig 로드 실패: %w", clusterID, err)
	}
	if len(kubeconfig) == 0 {
		return nil
	}

	restarter := rotation.NewWorkloadRestarter(kubeconfig, runKubectlWithKubeconfig)
	return restarter.RestartForProvider(ctx, namespace, provider)
}

// runKubectlWithKubeconfig 는 임시 kubeconfig 파일로 kubectl 을 실행한다.
func runKubectlWithKubeconfig(ctx context.Context, kubeconfig []byte, args ...string) ([]byte, error) {
	file, err := os.CreateTemp("", "nullus-rotation-kubeconfig-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("임시 kubeconfig 생성 실패: %w", err)
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()

	if _, err := file.Write(kubeconfig); err != nil {
		return nil, fmt.Errorf("kubeconfig 쓰기 실패: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("kubeconfig 닫기 실패: %w", err)
	}

	cmdArgs := append([]string{"--kubeconfig", file.Name()}, args...)
	out, err := exec.CommandContext(ctx, "kubectl", cmdArgs...).CombinedOutput() // #nosec G204 -- 인자는 내부 생성값
	if err != nil {
		return out, fmt.Errorf("kubectl %s 실패: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
