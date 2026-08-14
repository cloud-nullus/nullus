// Package externalsecret 은 ExternalSecret 매니페스트를 만든다.
//
// 스택 모듈과 CI/CD 모듈이 모두 이 평면을 쓴다 — 스택은 설치 시점에 차트가
// 참조할 Secret 을, CI/CD 는 파이프라인이 쓸 자격증명 Secret 을 만든다.
// 모듈 간 직접 import 가 금지되므로 공유가 필요한 이 렌더러를 shared 에 둔다.
//
// 매니페스트 모양이 두 곳에 각각 있으면 반드시 갈라진다. 갈라진 쪽은 ESO 가
// 조용히 무시하고, 그 Secret 을 참조하는 파드는 FailedMount 로 영원히 기동하지
// 못한다 — 원인이 멀리 떨어진 실패가 된다.
package externalsecret

import (
	"fmt"
	"sort"
	"strings"
)

// DefaultRefreshInterval 은 ESO 가 OpenBao 를 다시 읽는 주기다.
const DefaultRefreshInterval = "5m"

// Entry 는 대상 Secret 안의 키 하나와 그것이 올 OpenBao 경로다.
type Entry struct {
	// SecretKey 는 Kubernetes Secret 안의 키 이름이다.
	// 이 Secret 을 참조하는 쪽(차트의 existingSecret, 파드의 secretKeyRef)이
	// 요구하는 이름과 정확히 일치해야 한다.
	SecretKey string
	// RemotePath 는 OpenBao 의 전체 경로다(접두사 포함).
	RemotePath string
}

// Spec 은 ExternalSecret 하나를 기술한다.
type Spec struct {
	Name      string
	Namespace string
	// SecretStoreName 은 같은 네임스페이스에 있는 SecretStore 이름이다.
	SecretStoreName string
	// RefreshInterval 이 비면 DefaultRefreshInterval 을 쓴다.
	RefreshInterval string
	Entries         []Entry
	// TemplateData 가 있으면 target.template 으로 렌더링한다.
	// 값 안에서 {{ .키 }} 로 Entries 의 SecretKey 를 참조한다.
	TemplateData map[string]string
}

// Render 는 ExternalSecret 매니페스트를 만든다.
//
// 같은 입력이면 같은 결과다 — 키를 정렬해 재적용이 의미 없는 diff 를 만들지
// 않게 한다.
func Render(spec Spec) (string, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return "", fmt.Errorf("externalsecret: 이름이 필요합니다")
	}
	namespace := strings.TrimSpace(spec.Namespace)
	if namespace == "" {
		return "", fmt.Errorf("externalsecret %q: 네임스페이스가 필요합니다", name)
	}
	store := strings.TrimSpace(spec.SecretStoreName)
	if store == "" {
		return "", fmt.Errorf("externalsecret %q: SecretStore 이름이 필요합니다", name)
	}
	if len(spec.Entries) == 0 {
		return "", fmt.Errorf("externalsecret %q: 항목이 하나도 없습니다", name)
	}
	refresh := strings.TrimSpace(spec.RefreshInterval)
	if refresh == "" {
		refresh = DefaultRefreshInterval
	}

	var b strings.Builder
	fmt.Fprintf(&b, `apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: %s
  namespace: %s
spec:
  refreshInterval: %s
  secretStoreRef:
    name: %s
    kind: SecretStore
  target:
    name: %s
    creationPolicy: Owner
`, name, namespace, refresh, store, name)

	if len(spec.TemplateData) > 0 {
		b.WriteString("    template:\n      engineVersion: v2\n      data:\n")
		keys := make([]string, 0, len(spec.TemplateData))
		for k := range spec.TemplateData {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "        %s: |\n%s\n", k, IndentYAML(spec.TemplateData[k], 10))
		}
	}

	b.WriteString("  data:\n")
	for _, entry := range spec.Entries {
		fmt.Fprintf(&b, `    - secretKey: %s
      remoteRef:
        key: %s
        property: token
`, entry.SecretKey, entry.RemotePath)
	}
	return b.String(), nil
}

// IndentYAML 은 여러 줄 값을 블록 스칼라 안에 넣을 수 있게 들여쓴다.
func IndentYAML(value string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	trimmed := strings.TrimRight(value, "\n")
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}
