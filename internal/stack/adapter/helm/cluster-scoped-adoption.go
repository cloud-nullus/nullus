package helm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// cluster-scoped 리소스 소유권 인수.
//
// Helm 은 cluster-scoped 리소스의 소유권을 meta.helm.sh/release-name 과
// meta.helm.sh/release-namespace 주석으로 판별한다. 네임스페이스 리소스와 달리
// 이들은 스택을 지운 뒤에도 남기 쉽다 — CRD 는 Helm 이 의도적으로 남기고,
// uninstall 이 부분 실패하거나 네임스페이스가 강제 삭제되면 RBAC/webhook 도 남는다.
//
// 그 상태에서 다른 네임스페이스로 재설치하면 Helm 이
//
//	exists and cannot be imported into the current release: invalid ownership metadata
//
// 로 거부한다. 그래서 차트 설치 직전에 소유권을 현재 릴리스로 인수한다.
//
// 삭제가 아니라 인수인 이유: CRD 는 클러스터 전역이라 지우면 다른 스택의 커스텀
// 리소스가 함께 사라진다. 한 클러스터에 여러 스택이 살 수 있으므로 삭제는 위험하다.

// clusterScopedAdoptKinds 는 소유권 충돌이 실제로 발생하는 cluster-scoped 종류다.
var clusterScopedAdoptKinds = []string{
	"crd",
	"clusterrole",
	"clusterrolebinding",
	"validatingwebhookconfiguration",
	"mutatingwebhookconfiguration",
	"apiservice",
}

// foreignNamespaceAdoptTarget 은 스택 네임스페이스 밖에 놓이는 네임스페이스 리소스다.
type foreignNamespaceAdoptTarget struct {
	Kind      string
	Namespace string
}

// foreignNamespaceAdoptTargets 는 cluster-scoped 와 같은 이유로 인수가 필요한, 스택 네임스페이스 밖의
// 네임스페이스 리소스다. 네임스페이스를 회수해도 함께 사라지지 않는다.
//
// kube-prometheus-stack 은 컨트롤 플레인을 긁는 Service(coredns·kube-controller-manager·kube-etcd·
// kube-proxy·kube-scheduler)를 kube-system 에 만든다. 2026-10-12 — 옛 스택 삭제가 그 릴리스를 지우지
// 못해 이것들이 남았고, 재설치가 installing_prometheus 에서 "kube-prometheus-stack-coredns ... cannot
// be imported into the current release" 로 실패했다(삭제 쪽 원인은 domain.HelmReleaseNamesInUninstallOrder).
var foreignNamespaceAdoptTargets = []foreignNamespaceAdoptTarget{
	{Kind: "service", Namespace: "kube-system"},
}

// clusterScopedOwner 는 리소스 하나의 Helm 소유권 정보다.
type clusterScopedOwner struct {
	Name             string
	ReleaseName      string
	ReleaseNamespace string
}

// releaseAliveInNamespace 는 해당 네임스페이스에 그 Helm 릴리스가 아직 살아 있는지 본다.
// 테스트에서 대체할 수 있도록 패키지 변수로 둔다.
var releaseAliveInNamespace = func(ctx context.Context, o *Orchestrator, releaseName, namespace string) bool {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(releaseName) == "" {
		return false
	}
	// Helm 3 는 릴리스를 owner=helm,name=<release> 라벨이 붙은 Secret 으로 저장한다.
	// 네임스페이스가 이미 사라졌다면 조회가 실패하고, 그건 "살아 있지 않다" 로 본다.
	out, err := o.runKubectl(ctx, "get", "secret",
		"-n", namespace,
		"-l", "owner=helm,name="+releaseName,
		"-o", "jsonpath={.items[*].metadata.name}",
	)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// shouldAdoptClusterScopedResource 는 인수 여부를 판단한다.
//
// 핵심 안전 조건: 옛 소유 릴리스가 아직 살아 있으면 인수하지 않는다.
// 인수해 버리면 다른 스택이 쓰는 리소스를 탈취하고, 그 스택을 지울 때 함께 삭제된다.
func shouldAdoptClusterScopedResource(owner clusterScopedOwner, releaseName, targetNamespace string, oldReleaseAlive bool) bool {
	if strings.TrimSpace(owner.ReleaseName) == "" {
		return false // Helm 이 소유하지 않는 리소스는 건드리지 않는다
	}
	if owner.ReleaseName != releaseName {
		return false // 다른 릴리스 소유
	}
	if owner.ReleaseNamespace == targetNamespace {
		return false // 이미 우리 소유
	}
	if oldReleaseAlive {
		return false // 살아 있는 스택 소유 — 탈취 금지
	}
	return true
}

// parseClusterScopedOwners 는 kubectl custom-columns 출력을 파싱한다.
// 형식이 어긋난 줄은 조용히 건너뛴다 — 인수는 부가 보정이라 설치를 막으면 안 된다.
func parseClusterScopedOwners(output []byte) []clusterScopedOwner {
	var owners []clusterScopedOwner
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		owners = append(owners, clusterScopedOwner{
			Name:             fields[0],
			ReleaseName:      normalizeKubectlNone(fields[1]),
			ReleaseNamespace: normalizeKubectlNone(fields[2]),
		})
	}
	return owners
}

// normalizeKubectlNone 은 kubectl 이 값 없음을 표시하는 <none> 을 빈 문자열로 바꾼다.
func normalizeKubectlNone(v string) string {
	v = strings.TrimSpace(v)
	if v == "<none>" {
		return ""
	}
	return v
}

// clusterScopedOwnerOf 는 리소스 하나의 Helm 소유권을 조회한다.
// 리소스가 없으면 에러를 돌려준다.
func (o *Orchestrator) clusterScopedOwnerOf(ctx context.Context, kind, name string) (clusterScopedOwner, error) {
	out, err := o.runKubectl(ctx, "get", kind, name,
		"-o", `jsonpath={.metadata.annotations.meta\.helm\.sh/release-name}{"\t"}`+
			`{.metadata.annotations.meta\.helm\.sh/release-namespace}`,
	)
	if err != nil {
		return clusterScopedOwner{}, err
	}

	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	owner := clusterScopedOwner{Name: name}
	if len(parts) > 0 {
		owner.ReleaseName = normalizeKubectlNone(parts[0])
	}
	if len(parts) > 1 {
		owner.ReleaseNamespace = normalizeKubectlNone(parts[1])
	}
	return owner, nil
}

// listClusterScopedOwners 는 한 종류의 cluster-scoped 리소스 소유권을 조회한다.
func (o *Orchestrator) listClusterScopedOwners(ctx context.Context, kind string) ([]clusterScopedOwner, error) {
	return o.listOwners(ctx, kind)
}

// listOwners 는 한 종류의 리소스 소유권을 조회한다. scope 는 "-n <네임스페이스>" 같은 범위 인자다.
func (o *Orchestrator) listOwners(ctx context.Context, kind string, scope ...string) ([]clusterScopedOwner, error) {
	args := append([]string{"get", kind}, scope...)
	out, err := o.runKubectl(ctx, append(args,
		"--no-headers",
		"-o", `custom-columns=NAME:.metadata.name,`+
			`REL:.metadata.annotations.meta\.helm\.sh/release-name,`+
			`NS:.metadata.annotations.meta\.helm\.sh/release-namespace`,
	)...)
	if err != nil {
		return nil, err
	}
	return parseClusterScopedOwners(out), nil
}

// adoptClusterScopedResources 는 해당 릴리스가 남긴 cluster-scoped 리소스의
// 소유권을 현재 네임스페이스로 인수한다.
//
// 최선 노력(best effort)이다 — 조회나 patch 실패가 설치를 막아서는 안 된다.
// 인수하지 못하면 Helm 이 원래 에러를 그대로 돌려주므로 원인은 드러난다.
func (o *Orchestrator) adoptClusterScopedResources(ctx context.Context, releaseName, namespace string) {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return
	}
	releaseName = strings.TrimSpace(releaseName)
	namespace = strings.TrimSpace(namespace)
	if releaseName == "" || namespace == "" {
		return
	}

	for _, kind := range clusterScopedAdoptKinds {
		o.adoptOwnedResources(ctx, kind, releaseName, namespace)
	}
	for _, target := range foreignNamespaceAdoptTargets {
		if target.Namespace == namespace {
			continue // 스택이 그 자리에 깔렸으면 Helm 이 스스로 판단한다
		}
		o.adoptOwnedResources(ctx, target.Kind, releaseName, namespace, "-n", target.Namespace)
	}
}

// adoptOwnedResources 는 한 종류의 리소스 가운데 같은 이름의 죽은 릴리스가 남긴 것을 인수한다.
// scope 는 네임스페이스 리소스일 때의 "-n <네임스페이스>" 다.
func (o *Orchestrator) adoptOwnedResources(ctx context.Context, kind, releaseName, namespace string, scope ...string) {
	owners, err := o.listOwners(ctx, kind, scope...)
	if err != nil {
		// 클러스터에 없는 종류(apiservice 등)일 수 있다.
		return
	}

	for _, owner := range owners {
		if owner.ReleaseName != releaseName || owner.ReleaseNamespace == namespace {
			continue
		}

		alive := releaseAliveInNamespace(ctx, o, owner.ReleaseName, owner.ReleaseNamespace)
		if !shouldAdoptClusterScopedResource(owner, releaseName, namespace, alive) {
			if alive {
				slog.Warn("cluster-scoped 리소스를 인수하지 않습니다 — 다른 네임스페이스의 릴리스가 아직 살아 있습니다",
					"kind", kind, "name", owner.Name, "scope", strings.Join(scope, " "),
					"owner_namespace", owner.ReleaseNamespace, "target_namespace", namespace)
			}
			continue
		}

		if err := o.patchOwnership(ctx, kind, owner.Name, releaseName, namespace, scope...); err != nil {
			slog.Warn("cluster-scoped 리소스 소유권 인수 실패",
				"kind", kind, "name", owner.Name, "scope", strings.Join(scope, " "), "error", err)
			continue
		}
		slog.Info("cluster-scoped 리소스 소유권을 현재 릴리스로 인수했습니다",
			"kind", kind, "name", owner.Name, "scope", strings.Join(scope, " "),
			"from_namespace", owner.ReleaseNamespace, "to_namespace", namespace)
	}
}

// patchClusterScopedOwnership 는 Helm 소유권 주석/라벨을 현재 릴리스로 덮어쓴다.
func (o *Orchestrator) patchClusterScopedOwnership(ctx context.Context, kind, name, releaseName, namespace string) error {
	return o.patchOwnership(ctx, kind, name, releaseName, namespace)
}

// patchOwnership 은 patchClusterScopedOwnership 에 범위 인자(-n <네임스페이스>)를 더한 것이다.
func (o *Orchestrator) patchOwnership(ctx context.Context, kind, name, releaseName, namespace string, scope ...string) error {
	patch := fmt.Sprintf(
		`{"metadata":{"labels":{"app.kubernetes.io/managed-by":"Helm"},`+
			`"annotations":{"meta.helm.sh/release-name":%q,"meta.helm.sh/release-namespace":%q}}}`,
		releaseName, namespace)
	args := append([]string{"patch", kind, name}, scope...)
	_, err := o.runKubectl(ctx, append(args, "--type=merge", "-p", patch)...)
	return err
}
