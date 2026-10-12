package port

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 스캔 소스는 스택에 무엇이 설치됐는지와 레지스트리 종류를 함께 봐야 정해진다.
//
// 지원 레지스트리 5종 중 자체 스캔 기능이 있는 것은 Harbor 뿐이다 — GitLab
// Container Registry 의 Container Scanning 은 CI 잡이지 레지스트리 기능이
// 아니고, Nexus 는 OSS 에 스캐너가 없으며, GHCR 은 네이티브 스캔이 없다.
func TestScanSourceFor(t *testing.T) {
	withScanner := &StackSummary{ImageScannerEndpoint: "http://trivy.ns.svc.cluster.local:4954"}
	noScanner := &StackSummary{}

	tests := []struct {
		name  string
		stack *StackSummary
		kind  RegistryKind
		want  ScanSource
	}{
		{"스캐너가 있으면 그것을 쓴다", withScanner, RegistryKindGHCR, ScanSourceCentral},
		{"스캐너가 있으면 Harbor 여도 또 깔지 않는다", withScanner, RegistryKindHarbor, ScanSourceCentral},
		{"스캐너가 없고 Harbor 면 내장 스캐너", noScanner, RegistryKindHarbor, ScanSourceRegistry},
		{"스캐너도 Harbor 도 없으면 수단이 없다", noScanner, RegistryKindNexus, ScanSourceNone},
		{"GitLab 레지스트리도 수단이 없다", noScanner, RegistryKindSCMProject, ScanSourceNone},
		{"GHCR 도 수단이 없다", noScanner, RegistryKindGHCR, ScanSourceNone},
		{"외부 레지스트리는 보증할 수 없다", noScanner, RegistryKindExternal, ScanSourceNone},
		// 레지스트리 종류는 스택 정보와 무관하게 ImageTarget 에서 온다.
		// 스택을 못 읽었다고 Harbor 라는 사실이 사라지지는 않는다.
		{"스택을 못 읽어도 Harbor 는 여전히 Harbor 다", nil, RegistryKindHarbor, ScanSourceRegistry},
		{"스택도 못 읽고 Harbor 도 아니면 수단이 없다", nil, RegistryKindNexus, ScanSourceNone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ScanSourceFor(tc.stack, tc.kind))
		})
	}
}

// 새 RegistryKind 를 추가하면 이 테스트가 먼저 깨진다.
// 스캔 소스를 정하지 않은 채 레지스트리를 늘리지 못하게 막는 장치다.
func TestScanSourceFor_CoversAllKinds(t *testing.T) {
	all := []RegistryKind{
		RegistryKindSCMProject, RegistryKindHarbor, RegistryKindNexus,
		RegistryKindGHCR, RegistryKindExternal,
	}
	assert.Len(t, all, 5, "RegistryKind 가 늘면 여기와 ScanSourceFor 를 함께 고쳐야 한다")

	for _, k := range all {
		got := ScanSourceFor(&StackSummary{}, k)
		assert.Containsf(t, []ScanSource{ScanSourceRegistry, ScanSourceNone}, got,
			"%s 의 스캔 소스가 정해지지 않았다", k)
	}
}

// 스캔 단계를 조용히 빼지 않는다 — 사용자가 켠 단계가 아무 일도 하지 않으면
// 스캔된 줄 알고 넘어간다. 무엇을 하면 되는지까지 말한다.
func TestErrNoScannerAvailable_TellsWhatToDo(t *testing.T) {
	assert.Contains(t, ErrNoScannerAvailable.Error(), "스캐너")
	assert.Contains(t, ErrNoScannerAvailable.Error(), "Harbor")
}
