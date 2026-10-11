package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

// EnsureSharedComponents 는 클러스터 공용 컨트롤러 — cert-manager·metrics-server — 를 스택 없이 세운다.
//
// 클러스터의 마지막 스택을 지우면 delete_shared_resources 가 두 컨트롤러와 내부 CA·Gateway CRD 를
// 회수한다. 백업은 CA·CRD 같은 **자원**은 담지만 컨트롤러는 **설치물**이라 담지 않는다. 그래서 그
// 뒤의 복구는 cert-manager 가 없어 ClusterIssuer·Certificate 를 되살리지 못했고, 운영자가 손으로
// 먼저 깔아야 했다(백업복구 설계 §11.3 Q16). 이 유스케이스가 그 손작업을 대신한다 — 설치기의 같은
// 단계를 그대로 돌리므로 설치와 복구가 한 코드로 같은 결과를 낸다.
//
// 스택 ID 를 비우고 부른다. 설치기는 스택 ID 가 비면 단계 순서 검사를 하지 않고, 이미 깔린
// cert-manager 가 있으면 다시 쓴다 — 다른 스택이 쓰는 클러스터에 두 번째로 불러도 안전하다.
// 백업 모듈이 복구 중 공용 자원을 되살리기 직전에 부른다.
type EnsureSharedComponents struct {
	factory func(kubeconfig []byte) port.StepExecutor
}

// NewEnsureSharedComponents 는 설치기 팩토리로 유스케이스를 만든다. 스택 설치가 쓰는 것과 같은
// 팩토리를 넘겨야 차트 메타데이터·자원 기본값·이미지 프로젝트가 설치와 같다.
func NewEnsureSharedComponents(factory func(kubeconfig []byte) port.StepExecutor) *EnsureSharedComponents {
	return &EnsureSharedComponents{factory: factory}
}

// 복구가 다시 세우는 단계. 설치 순서(domain.InstallStepOrder)와 같게 cert-manager 가 먼저다.
const (
	sharedStepCertManager   = "installing_cert_manager"
	sharedStepMetricsServer = "installing_metrics_server"
	sharedStepPhase         = "A"
)

// EnsureSharedComponents 는 돌린 단계와 넘긴 실패를 돌려준다. cert-manager 실패는 오류다 — 그것 없이는
// 내부 CA 를 되살릴 수 없고, 되살리지 못한 채 볼륨까지 되돌리면 되돌릴 길이 없다. metrics-server 는
// HPA·자원 지표에만 쓰이므로 실패해도 경고로 넘긴다.
func (uc *EnsureSharedComponents) EnsureSharedComponents(ctx context.Context, kubeconfig []byte, namespace string) ([]string, []string, error) {
	if uc == nil || uc.factory == nil {
		return nil, nil, errors.New("공용 컨트롤러 설치기가 배선되지 않았습니다")
	}
	if len(kubeconfig) == 0 {
		return nil, nil, errors.New("공용 컨트롤러를 세울 클러스터의 kubeconfig 가 없습니다")
	}
	exec := uc.factory(kubeconfig)
	if exec == nil {
		return nil, nil, errors.New("공용 컨트롤러 설치기를 만들지 못했습니다")
	}
	if aware, ok := exec.(namespaceAwareExecutor); ok && namespace != "" {
		aware.SetNamespace(namespace)
	}

	var steps, warnings []string
	if err := exec.ExecuteStep(ctx, "", sharedStepCertManager, sharedStepPhase); err != nil {
		return steps, warnings, fmt.Errorf("cert-manager 를 다시 세우지 못했습니다: %w", err)
	}
	steps = append(steps, sharedStepCertManager)

	if err := exec.ExecuteStep(ctx, "", sharedStepMetricsServer, sharedStepPhase); err != nil {
		warnings = append(warnings, fmt.Sprintf("metrics-server 를 다시 세우지 못했습니다(HPA·자원 지표만 영향): %v", err))
		return steps, warnings, nil
	}
	steps = append(steps, sharedStepMetricsServer)
	return steps, warnings, nil
}
