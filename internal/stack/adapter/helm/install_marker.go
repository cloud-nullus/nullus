package helm

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 클러스터 공용 자원의 설치 표시.
//
// 클러스터의 마지막 스택을 지울 때 Nullus 가 설치한 공용 자원을 회수한다(usecase/
// delete_shared_resources.go). 설치기는 클러스터에 원래 있던 것을 재사용하므로, 새로 깐 것에만
// 표시를 남겨 둘을 가른다. cert-manager·metrics-server 는 릴리스 values 에 표시가 실린다
// (platform-owned-values.go). 여기는 Helm 릴리스가 소유하지 않는 게이트웨이 쪽이다.

// gatewayCRDSnapshot 은 게이트웨이 단계 전의 Gateway CRD 목록이다. 읽지 못하면 nil 이고, 그러면
// 표시하지 않는다 — 원래 있던 CRD 에 표시를 다는 것보다 새로 깐 CRD 를 남기는 편이 낫다.
func (o *Orchestrator) gatewayCRDSnapshot(ctx context.Context) map[string]bool {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	out, err := o.runKubectlStdout(ctx, "get", "crd", "-o", "name")
	if err != nil {
		slog.Warn("CRD 목록을 읽지 못해 게이트웨이 단계가 까는 CRD 에 설치 표시를 달지 않습니다 — 스택을 지울 때 그 CRD 는 남습니다",
			"error", err)
		return nil
	}
	return crdNameSet(string(out))
}

// markGatewayCRDsCreatedSince 는 before 뒤에 새로 생긴 Gateway CRD 에 설치 표시를 단다.
//
// 이름 목록으로 고르지 않는다. Envoy Gateway 차트는 실험용 Gateway API CRD(*.x-k8s.io)와 자기
// CRD(*.gateway.envoyproxy.io)를 crds/ 로 덤으로 깐다 — 목록으로 고르면 그것을 놓친다. 단계가
// 실패하거나 취소돼도 깔린 CRD 는 남으므로 취소되지 않는 컨텍스트로 단다.
func (o *Orchestrator) markGatewayCRDsCreatedSince(ctx context.Context, before map[string]bool) {
	if before == nil {
		return
	}
	markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
	defer cancel()
	out, err := o.runKubectlStdout(markCtx, "get", "crd", "-o", "name")
	if err != nil {
		slog.Warn("CRD 목록을 읽지 못해 설치 표시를 달지 못했습니다 — 스택을 지울 때 이 CRD 는 남습니다", "error", err)
		return
	}
	var created []string
	for name := range crdNameSet(string(out)) {
		if !before[name] && domain.IsGatewayCRD(name) {
			created = append(created, name)
		}
	}
	if len(created) == 0 {
		return
	}
	sort.Strings(created)
	args := append([]string{"label", "crd"}, created...)
	args = append(args, domain.LabelInstalledBy+"="+domain.InstalledByNullus, "--overwrite")
	if _, err := o.runKubectl(markCtx, args...); err != nil {
		slog.Warn("Gateway CRD 에 설치 표시를 달지 못했습니다 — 스택을 지울 때 이 CRD 는 남습니다",
			"crds", created, "error", err)
	}
}

// ensureEnvoyGatewayClass 는 GatewayClass envoy 가 없을 때만 설치 표시와 함께 만든다.
//
// 이미 있으면 건드리지 않는다. 고객이 같은 이름으로 만든 것일 수 있고, 지난 스택이 만든 것이면 그대로
// 쓰면 된다. 예전에는 설치마다 apply 로 덮어써서 남의 GatewayClass 였어도 이 설치기의 것으로 바뀌었다.
// "있는지 조회 → 없으면 apply" 로 하지 않는다 — 조회가 실패하면 있는 것을 덮어쓰고 표시까지 단다.
func (o *Orchestrator) ensureEnvoyGatewayClass(ctx context.Context) error {
	out, err := o.runKubectlWithStdin(ctx, defaultEnvoyGatewayClassManifest(), "create", "-f", "-")
	if err != nil && strings.Contains(string(out)+err.Error(), "AlreadyExists") {
		return nil
	}
	return err
}

// releaseValuesGetter 는 이미 있는 릴리스의 values 를 읽는다(HelmInstaller 가 구현한다).
type releaseValuesGetter interface {
	GetValues(ctx context.Context, releaseName, namespace string) (map[string]any, error)
}

// withInstallMarkerIfOwned 는 공용 릴리스의 설치 표시를 Nullus 가 새로 까는 릴리스에만 남긴다.
//
// Install 은 이미 있는 릴리스를 업그레이드한다. 재사용 판정(checkExistingCertManagerInstallation)은
// Deployment 셋이 다 있어야 알아보므로, cainjector 를 끈 고객의 cert-manager 같은 것은 놓치고 업그레이드
// 한다. 그때 표시를 달면 스택을 지울 때 고객 것을 회수한다. 이미 있는 릴리스면 원래 Nullus 것(표시나 옛
// 설치 지문)일 때만 표시를 이어 간다. 확인하지 못하면 달지 않는다 — 표시 없는 Nullus 릴리스는 삭제 때
// 남을 뿐이다.
func (o *Orchestrator) withInstallMarkerIfOwned(ctx context.Context, releaseName, namespace string, values map[string]any) map[string]any {
	path, ok := domain.InstallMarkerValuePath(releaseName)
	if !ok || o.installer == nil {
		return values
	}
	_, statusErr := o.installer.Status(ctx, releaseName, namespace)
	switch {
	case statusErr != nil && isReleaseNotFoundError(statusErr):
		return values
	case statusErr == nil:
		if getter, ok := o.installer.(releaseValuesGetter); ok {
			if existing, err := getter.GetValues(ctx, releaseName, namespace); err == nil &&
				domain.IsNullusInstalledRelease(releaseName, existing) {
				return values
			}
		}
	}
	slog.Warn("Nullus 가 새로 까는 릴리스가 아니라 설치 표시를 달지 않습니다 — 스택을 지울 때 이 릴리스는 남습니다",
		"release", releaseName, "namespace", namespace, "status_error", statusErr)
	return withoutValue(values, path)
}

// withoutValue 는 중첩 경로의 값을 지우고, 그래서 비게 된 상위 매핑도 지운다.
func withoutValue(values map[string]any, path []string) map[string]any {
	if len(path) == 0 || values == nil {
		return values
	}
	if len(path) == 1 {
		delete(values, path[0])
		return values
	}
	child, ok := values[path[0]].(map[string]any)
	if !ok {
		return values
	}
	withoutValue(child, path[1:])
	if len(child) == 0 {
		delete(values, path[0])
	}
	return values
}

// crdNameSet 은 `kubectl get crd -o name` 출력의 CRD 이름이다. 공백이 든 줄은 이름이 아니다(안내 문구).
func crdNameSet(output string) map[string]bool {
	names := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		names[name] = true
	}
	return names
}
