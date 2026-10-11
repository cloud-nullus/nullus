package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// GatewayAddressReaderFactory 는 대상 클러스터의 kubeconfig 로 게이트웨이 주소 조회기를 만든다.
type GatewayAddressReaderFactory func(kubeconfig []byte) (port.GatewayAddressReader, error)

// GatewayAliasRepin 은 스택 게이트웨이의 지금 주소로 러너·Argo CD 의 hostAliases 를 다시 박는다.
//
// 설치는 게이트웨이 ClusterIP 를 두 릴리스의 values 에 박는다(스택 도구 이름은 클러스터 안에서
// 아무도 풀어 주지 않는다). 복구로 그 주소가 바뀌면 — 다른 클러스터로 되돌렸거나, ClusterIP 를 담지
// 않은 옛 백업이거나 — 두 곳 모두 옛 주소를 가리켜 CI 는 clone 에서, Argo CD 는 매니페스트 조회에서
// 멈춘다. 백업 모듈이 복구 끝에 이것을 부른다.
//
// values 만 바꿔 다시 적용한다. 차트는 릴리스에 저장된 것을 쓰고, 서브차트를 잃은 차트(argo-cd)는
// 같은 버전을 저장소에서 다시 받는다(release-values.go chartForRelease). 그래야 릴리스 Secret·러너
// ConfigMap·Argo CD 파드가 함께 맞고, 나중에 values 를 고쳐도 옛 주소가 돌아오지 않는다. 주소가
// 그대로면 아무것도 하지 않는다. 한 릴리스가 실패해도 나머지는 맞춘다.
type GatewayAliasRepin struct {
	stackRepo          port.StackRepository
	kubeconfigProvider port.KubeconfigProvider
	managerFactory     ReleaseManagerFactory
	readerFactory      GatewayAddressReaderFactory
}

// NewGatewayAliasRepin 은 유스케이스를 조립한다.
func NewGatewayAliasRepin(
	stackRepo port.StackRepository,
	kubeconfigProvider port.KubeconfigProvider,
	managerFactory ReleaseManagerFactory,
	readerFactory GatewayAddressReaderFactory,
) *GatewayAliasRepin {
	return &GatewayAliasRepin{
		stackRepo:          stackRepo,
		kubeconfigProvider: kubeconfigProvider,
		managerFactory:     managerFactory,
		readerFactory:      readerFactory,
	}
}

// 주소를 박는 릴리스의 차트다.
const (
	argoCDChartName       = "argo-cd"
	gitLabRunnerChartName = "gitlab-runner"
)

// RepinGatewayAliases 는 네임스페이스(비우면 스택의 것)에서 게이트웨이 주소를 읽어 러너·Argo CD 의
// hostAliases 를 맞춘다. 게이트웨이 주소와 다시 적용한 릴리스를 돌려준다.
//
// kubeconfig 를 주면 그 클러스터를 본다. full 복구는 플랫폼 DB 를 백업 시점으로 되돌리므로, 그 사이
// 스택을 다른 클러스터로 옮겼다면 DB 의 kubeconfig 는 옛 클러스터다 — 복구가 실제로 되돌린 클러스터를
// 넘긴다. 비우면 스택의 클러스터에서 읽는다.
func (uc *GatewayAliasRepin) RepinGatewayAliases(ctx context.Context, stackID, namespace string, kubeconfig []byte) (string, []string, error) {
	if uc == nil || uc.kubeconfigProvider == nil || uc.managerFactory == nil || uc.readerFactory == nil {
		return "", nil, fmt.Errorf("게이트웨이 주소 재설정이 구성되지 않았다")
	}
	stack, err := uc.stackRepo.GetByID(ctx, strings.TrimSpace(stackID))
	if err != nil {
		return "", nil, fmt.Errorf("get stack: %w", err)
	}
	if stack == nil {
		return "", nil, ErrStackNotFound
	}
	hostnames := domain.GatewayPinnedHostnames(stackConfigOf(stack).AccessDomain)
	if len(hostnames) == 0 {
		return "", nil, nil // 접근 도메인이 없으면 박아 둔 주소도 없다
	}
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = strings.TrimSpace(stack.Namespace)
	}

	if len(kubeconfig) == 0 {
		if kubeconfig, err = uc.kubeconfigProvider.GetKubeconfig(ctx, stack.ClusterID); err != nil {
			return "", nil, fmt.Errorf("load kubeconfig: %w", err)
		}
	}
	reader, err := uc.readerFactory(kubeconfig)
	if err != nil {
		return "", nil, err
	}
	ip, err := reader.GatewayClusterIP(ctx, ns)
	if err != nil {
		return "", nil, fmt.Errorf("게이트웨이 주소를 읽지 못했다: %w", err)
	}

	manager := uc.managerFactory(kubeconfig)
	releases, err := manager.ListReleases(ctx, ns)
	if err != nil {
		return ip, nil, fmt.Errorf("릴리스 목록(%s): %w", ns, err)
	}
	var updated []string
	var errs []error
	for _, rel := range releases {
		var repin func(map[string]any, []string, string) (bool, []string)
		switch rel.ChartName {
		case argoCDChartName:
			repin = domain.RepinArgoCDHostAliases
		case gitLabRunnerChartName:
			repin = domain.RepinRunnerHostAliases
		default:
			continue
		}
		values, err := manager.GetValues(ctx, rel.ReleaseName, ns)
		if err != nil {
			errs = append(errs, fmt.Errorf("values 조회(%s): %w", rel.ReleaseName, err))
			continue
		}
		if changed, _ := repin(values, hostnames, ip); !changed {
			continue
		}
		if _, err := manager.Upgrade(ctx, port.HelmUpgradeRequest{
			ReleaseName: rel.ReleaseName, Namespace: ns, Values: values,
		}); err != nil {
			errs = append(errs, fmt.Errorf("values 적용(%s): %w", rel.ReleaseName, err))
			continue
		}
		updated = append(updated, rel.ReleaseName)
	}
	return ip, updated, errors.Join(errs...)
}
