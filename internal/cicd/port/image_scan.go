package port

import (
	"errors"
	"strings"
)

// ScanSource 는 이 이미지의 스캔 결과를 어디서 얻을지다.
type ScanSource string

const (
	// ScanSourceCentral 은 스택에 설치된 Trivy 서버로 스캔하는 것이다.
	ScanSourceCentral ScanSource = "central"
	// ScanSourceRegistry 는 레지스트리가 이미 가진 결과를 읽는 것이다.
	// 스캔을 한 번 더 돌리지 않는다.
	ScanSourceRegistry ScanSource = "registry"
	// ScanSourceNone 은 이 스택에서 이미지를 스캔할 수단이 없다는 뜻이다.
	//
	// 스캔을 건너뛰라는 뜻이 아니다 — 스캔 단계를 켤 수 없다는 뜻이고,
	// 호출부는 이것을 침묵이 아니라 거부로 다뤄야 한다.
	ScanSourceNone ScanSource = "none"
)

// ScanSourceFor 는 스택 구성과 레지스트리 종류로 스캔 소스를 고른다.
//
// 스택에 스캐너가 설치돼 있으면 그것을 쓴다. 없으면 레지스트리 자체 기능으로
// 떨어지는데, 2026-09 기준 그것이 있는 것은 Harbor 뿐이다 — GitLab Container
// Registry 의 Container Scanning 은 CI 잡이지 레지스트리 기능이 아니고,
// Nexus 는 OSS 에 스캐너가 없으며(Sonatype IQ 는 상용), GHCR 은 네이티브
// 스캔이 없다. 없는 쪽에 스캐너를 꽂을 확장점도 없다.
//
// Harbor 가 있는 스택에 스캐너를 또 깔 필요는 없다. 그 판단을 여기서 한다.
func ScanSourceFor(stack *StackSummary, kind RegistryKind) ScanSource {
	if stack != nil && strings.TrimSpace(stack.ImageScannerEndpoint) != "" {
		return ScanSourceCentral
	}
	if kind == RegistryKindHarbor {
		return ScanSourceRegistry
	}
	return ScanSourceNone
}

// ErrNoScannerAvailable 은 이 스택에 이미지를 스캔할 수단이 없다는 뜻이다.
//
// 스캔 단계를 조용히 빼지 않는 이유는 ErrImageDeletionUnsupported 와 같다 —
// 사용자가 켠 단계가 아무 일도 하지 않으면 스캔된 줄 알고 넘어간다.
var ErrNoScannerAvailable = errors.New(
	"이 스택에는 이미지 스캐너가 없습니다: 스택에 스캐너를 추가하거나 컨테이너 레지스트리를 Harbor 로 고르세요")
