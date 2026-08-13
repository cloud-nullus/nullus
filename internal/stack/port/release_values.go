package port

import "context"

// 배포가 끝난 스택의 Helm 릴리스를 values 단위로 다루기 위한 포트다.
//
// HelmInstaller 에 메서드를 더하지 않고 따로 둔다. 설치는 "차트를 새로 올린다",
// 이쪽은 "이미 올라간 릴리스의 values 만 바꾼다" 로 관심사가 다르고, 설치
// 경로의 대역(fake) 들이 이 메서드까지 구현할 이유가 없기 때문이다.

// ReleaseInfo 는 클러스터에 실제로 올라가 있는 Helm 릴리스 한 건이다.
type ReleaseInfo struct {
	ReleaseName string `json:"release_name"`
	// StepName 은 이 릴리스를 만든 설치 단계다. YAML 오버라이드를 어느 키로
	// 저장할지 결정하므로 비어 있으면 편집 결과가 재배포 때 유실된다.
	StepName     string `json:"step_name,omitempty"`
	ChartName    string `json:"chart_name,omitempty"`
	ChartVersion string `json:"chart_version,omitempty"`
	AppVersion   string `json:"app_version,omitempty"`
	Namespace    string `json:"namespace"`
	Revision     int    `json:"revision"`
	Status       string `json:"status"`
}

// HelmUpgradeRequest 는 values 만 교체하는 업그레이드 요청이다.
// 차트는 릴리스에 저장된 것을 그대로 재사용하므로 여기서 받지 않는다 —
// 설정 변경이 의도치 않은 차트 버전 업그레이드를 끌고 오면 안 된다.
type HelmUpgradeRequest struct {
	ReleaseName string
	Namespace   string
	Values      map[string]any
	// DryRun 이면 클러스터를 건드리지 않고 렌더 결과만 돌려준다.
	DryRun bool
}

// HelmUpgradeResult 는 업그레이드(또는 드라이런) 결과다.
type HelmUpgradeResult struct {
	ReleaseName string
	Namespace   string
	Revision    int
	Status      string
	// Manifest 는 렌더된 매니페스트다. 드라이런에서 변경 내용을 보여주는 데 쓴다.
	Manifest string
}

// HelmReleaseManager 는 배포된 릴리스의 values 를 읽고 다시 적용한다.
type HelmReleaseManager interface {
	ListReleases(ctx context.Context, namespace string) ([]ReleaseInfo, error)
	// GetValues 는 사용자가 지정한 values(= 설치 때 우리가 넘긴 값)를 돌려준다.
	// 차트 기본값까지 합친 전체가 아니라, 실제로 배포에 쓰인 우리 입력이다.
	GetValues(ctx context.Context, releaseName, namespace string) (map[string]any, error)
	Upgrade(ctx context.Context, req HelmUpgradeRequest) (*HelmUpgradeResult, error)
}
