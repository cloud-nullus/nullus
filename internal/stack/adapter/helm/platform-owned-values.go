package helm

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 플랫폼이 소유하는 values — 사용자 오버라이드가 덮어써서는 안 되는 값들.
//
// 왜 필요한가: release values 의 live 편집(ManageReleaseValues)은 릴리스에 실제로
// 배포된 values 를 통째로 읽어 YAMLOverrides 에 저장한다. 거기에는 사용자가 적은
// 값뿐 아니라 **플랫폼이 계산해 넣은 값**도 함께 얼어붙는다. 오버라이드는 병합의
// 맨 마지막에 적용되므로, 그 스냅샷이 이후의 재계산을 영원히 이긴다.
//
// 실제로 두 가지가 이 경로로 깨졌다:
//
//   - global.psql.host 가 스냅샷 시점의 네임스페이스에 묶여, 설정을 다른
//     네임스페이스로 옮기면 GitLab 이 **삭제된 스택의 PostgreSQL** 을 가리켰다.
//   - GitLab 번들 Prometheus 의 메모리 한도가 328Mi 로 얼어붙어, 그 값이 OOM 을
//     막으려 둔 자원 하한(resource-defaults.go 의 promScaled)을 이기고
//     CrashLoopBackOff 를 재현했다.
//
// 그래서 병합이 끝난 뒤 이 값들만 다시 못박는다. 나머지는 사용자 의도이므로
// 손대지 않는다 — 자원·프로브 같은 값까지 되돌리면 오버라이드가 무의미해진다.

// platformOwnedValue 는 되돌릴 값 하나다.
type platformOwnedValue struct {
	// path 는 values 트리에서의 위치다 (예: global.psql.host).
	path []string
	// value 는 플랫폼이 정한 값이다.
	value any
	// reason 은 오버라이드를 무시한 이유다. 사용자가 왜 자기 편집이 안 먹는지
	// 알 수 있도록 경고 로그에 싣는다.
	reason string
}

// platformOwnedValuesForStep 은 단계별로 플랫폼이 소유하는 값을 돌려준다.
func (o *Orchestrator) platformOwnedValuesForStep(step string) []platformOwnedValue {
	namespace := strings.TrimSpace(o.namespace)
	if namespace == "" {
		namespace = defaultStackNamespace
	}

	const namespaceScopedReason = "네임스페이스에서 파생되는 주소라 스택 밖의 값을 쓸 수 없습니다"

	switch step {
	case "installing_gitlab":
		return []platformOwnedValue{
			{
				path:   []string{"global", "psql", "host"},
				value:  fmt.Sprintf("%s.%s.svc.cluster.local", domain.PostgresServiceName, namespace),
				reason: namespaceScopedReason,
			},
			{
				// 스택은 이미 kube-prometheus-stack 을 세우고 라우트·대시보드가
				// 모두 그쪽을 본다. 번들 Prometheus 는 아무도 읽지 않으면서
				// 메모리만 먹고, 스택의 자원 계획 아래에서 OOMKilled 로 죽는다.
				path:   []string{"prometheus", "install"},
				value:  false,
				reason: "스택의 메트릭 백엔드는 kube-prometheus-stack 이라 번들 Prometheus 는 설치하지 않습니다",
			},
		}
	case "installing_minio":
		return []platformOwnedValue{{
			path:   []string{"namespace"},
			value:  namespace,
			reason: namespaceScopedReason,
		}}
	case stepInstallingRunner:
		return []platformOwnedValue{{
			path:   []string{"gitlabUrl"},
			value:  fmt.Sprintf("http://gitlab-webservice-default.%s.svc:8181", namespace),
			reason: namespaceScopedReason,
		}}
	}
	return nil
}

// enforcePlatformOwnedValues 는 병합이 끝난 values 위에 플랫폼 소유 값을 다시 쓴다.
//
// 값이 실제로 달랐을 때만 경고를 남긴다 — 대부분의 배포는 같은 값이라
// 매번 로그를 남기면 진짜 신호가 묻힌다.
func (o *Orchestrator) enforcePlatformOwnedValues(step string, values map[string]any) map[string]any {
	owned := o.platformOwnedValuesForStep(step)
	if len(owned) == 0 {
		return values
	}
	if values == nil {
		values = map[string]any{}
	}

	for _, item := range owned {
		if previous, found := lookupValue(values, item.path); found && previous != item.value {
			slog.Warn("사용자 오버라이드 대신 플랫폼 값을 적용합니다",
				"step", step,
				"path", strings.Join(item.path, "."),
				"override", previous,
				"applied", item.value,
				"reason", item.reason)
		}
		setValue(values, item.path, item.value)
	}
	return values
}

// lookupValue 는 중첩 경로의 값을 찾는다. 중간이 매핑이 아니면 없는 것으로 본다.
func lookupValue(values map[string]any, path []string) (any, bool) {
	if len(path) == 0 {
		return nil, false
	}
	current := values
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	found, ok := current[path[len(path)-1]]
	return found, ok
}

// setValue 는 중첩 경로에 값을 쓴다. 중간 매핑이 없거나 매핑이 아니면 새로 만든다
// — 오버라이드가 엉뚱한 타입을 넣어 둔 경우에도 배선은 서야 한다.
func setValue(values map[string]any, path []string, value any) {
	if len(path) == 0 {
		return
	}
	current := values
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[key] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}
