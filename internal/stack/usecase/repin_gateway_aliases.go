package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

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

	keys, err := uc.repinOverrides(ctx, stack.ID, hostnames, ip)
	if err != nil {
		errs = append(errs, err)
	}
	for _, k := range keys {
		updated = append(updated, "overrides/"+k)
	}
	return ip, updated, errors.Join(errs...)
}

// overrideRepins 는 게이트웨이 주소가 박히는 오버라이드 키와 그 규칙이다. 설치는 단계 이름 → 릴리스 이름 →
// 차트 이름 → "installing_" 을 뗀 이름 순으로 오버라이드를 찾는다(helm-values.go) — 모두 본다.
var overrideRepins = map[string]func(map[string]any, []string, string) (bool, []string){
	"installing_argocd":   domain.RepinArgoCDHostAliases,
	argoCDChartName:       domain.RepinArgoCDHostAliases,
	"argocd":              domain.RepinArgoCDHostAliases,
	"installing_runner":   domain.RepinRunnerHostAliases,
	gitLabRunnerChartName: domain.RepinRunnerHostAliases,
	"runner":              domain.RepinRunnerHostAliases,
}

// yamlOverrideWriter 는 스택 설정의 오버라이드 키 하나만 바꾼다(Postgres·메모리 저장소). 행 전체를
// 덮어쓰는 Update 와 달리 그 사이 들어온 다른 변경을 되돌리지 않는다.
type yamlOverrideWriter interface {
	SetYAMLOverride(ctx context.Context, stackID, key, yamlText string) error
}

// repinOverrides 는 스택 설정의 오버라이드에 굳은 게이트웨이 주소를 바꾼다. 바꾼 키를 돌려준다.
//
// OSS 설정 편집(live)은 배포된 values 를 통째로 오버라이드에 저장해 그 시점의 주소가 굳는다. 오버라이드는
// 재설치 때 맨 마지막에 병합되므로, 릴리스만 고치면 다음 재설치에서 옛 주소가 돌아온다.
//
//   - helm 작업이 끝난 뒤 스택을 새로 읽는다 — Argo CD 차트를 다시 받으면 수 분이 걸린다.
//   - 키 하나씩만 쓴다. 행 전체를 덮어쓰면 그 사이의 상태 전이·다른 키 편집을 되돌린다.
//   - ip 값만 텍스트에서 바꾼다. 다시 직렬화하면 사용자가 쓴 주석·서식이 사라진다.
func (uc *GatewayAliasRepin) repinOverrides(ctx context.Context, stackID string, hostnames []string, ip string) ([]string, error) {
	stack, err := uc.stackRepo.GetByID(ctx, stackID)
	if err != nil || stack == nil {
		return nil, fmt.Errorf("스택 설정 다시 읽기: %w", err)
	}
	cfg, _ := stackConfigFromInterface(stack.Config)
	writer, partial := uc.stackRepo.(yamlOverrideWriter)

	var changed []string
	for key, text := range cfg.YAMLOverrides {
		repin, ok := overrideRepins[key]
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		var values map[string]any
		if err := yaml.Unmarshal([]byte(text), &values); err != nil {
			continue // 읽지 못하는 오버라이드는 설치도 쓰지 못한다 — 여기서 고칠 일이 아니다
		}
		ok, old := repin(values, hostnames, ip)
		if !ok {
			continue
		}
		next := replaceAliasIPs(text, old, ip)
		if next == text {
			continue
		}
		if partial {
			if err := writer.SetYAMLOverride(ctx, stackID, key, next); err != nil {
				return changed, fmt.Errorf("오버라이드 저장(%s): %w", key, err)
			}
		} else {
			cfg.YAMLOverrides[key] = next
		}
		changed = append(changed, key)
	}
	if len(changed) > 0 && !partial {
		stack.Config = cfg
		if err := uc.stackRepo.Update(ctx, stack); err != nil {
			return nil, fmt.Errorf("스택 설정의 오버라이드 저장: %w", err)
		}
	}
	sort.Strings(changed)
	return changed, nil
}

// replaceAliasIPs 는 텍스트에서 ip 필드의 옛 주소만 새 주소로 바꾼다 — YAML `ip: x`·`- ip: "x"`,
// TOML `ip = "x"`. 주석이나 다른 값에 같은 주소가 있어도 건드리지 않는다.
func replaceAliasIPs(text string, old []string, ip string) string {
	for _, o := range old {
		if strings.TrimSpace(o) == "" {
			continue
		}
		re := regexp.MustCompile(`(\bip\s*[:=]\s*["']?)` + regexp.QuoteMeta(o) + `(["']?(?:[^0-9.]|$))`)
		text = re.ReplaceAllString(text, "${1}"+ip+"${2}")
	}
	return text
}
