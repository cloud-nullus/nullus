package domain

import (
	"slices"
	"strings"
)

// Harbor 멀티아키 재빌드 이미지.
//
// 공식 goharbor 이미지는 어느 버전도 arm64 를 내지 않는다(단일 아키 amd64 매니페스트).
// arm64 노드에서는 `fatal error: lfstack.push` 로 크래시해, DGX Spark 에서 스택 설치가
// installing_harbor 에서 멈췄다(cloud-nullus/nullus#270).
//
// dasomel/harbor 는 goharbor 를 매일 동기화해 amd64·arm64 로 다시 빌드하는 fork 다.
// 이미지는 cosign 으로 서명되고 SBOM·SLSA 증적이 붙는다. v2.15.x 빌드만 내므로 차트
// 1.15.0(앱 2.11.0) 위에서 앱 버전이 앞선다 — 에어갭 kind(Apple Silicon) 경로가
// 2026-05 부터 써 온 조합이고, 차트 1.15.0 과 앱 2.15.0 의 차트(1.19.0) 템플릿 차이는
// 프로브 값 템플릿화·namespace 명시·redis TLS 옵션 정도다.
//
// 태그는 에어갭 번들 목록(airgap/images/images.txt)과 같아야 한다. `v2.15.0` 은
// 재빌드마다 옮겨 가는 태그라 빌드 번호가 붙은 쪽을 쓴다.
const (
	HarborMultiArchImageRegistry  = "ghcr.io/dasomel/goharbor"
	HarborMultiArchImageTag       = "v2.15.0-build.32"
	HarborMultiArchImageReference = "https://github.com/dasomel/harbor"
)

// ArchImageSource 는 공식 이미지가 내지 않는 아키텍처를 채우는 대체 이미지 출처다.
type ArchImageSource struct {
	// Archs 는 이 출처의 이미지가 내는 아키텍처다. 한 벌의 이미지가 모든 노드에
	// 깔리므로, 이 출처를 쓰려면 클러스터의 노드 아키텍처를 전부 덮어야 한다.
	Archs []string
	// Registry 는 컴포넌트 이미지 이름 앞에 붙는 저장소 경로다.
	// 컴포넌트 이름(harbor-core 등)은 차트를 아는 설치 어댑터가 붙인다.
	Registry string
	Tag      string
	// Reference 는 이미지를 만드는 저장소다. 설치 로그에 출처로 남긴다.
	Reference string
}

// ToolImageProfile 은 도구 하나가 어느 아키텍처의 노드에서 뜰 수 있는지를 말한다.
//
// 공식 이미지가 모든 아키텍처를 내는 도구는 여기에 없다. 여기 선언이 설치(이미지
// 선택·사전검사)와 호환성 매트릭스의 ArchSupport 가 함께 보는 단일 기준이다 —
// 매트릭스가 따로 적으면 설치가 실제로 내는 이미지와 어긋나도 아무것도 깨지지 않았고,
// 그래서 arm64 에서 뜨지 않는 Harbor 를 Pre-Deploy Gate 가 통과시켰다.
type ToolImageProfile struct {
	// Tool 은 호환성 매트릭스의 도구 이름이다.
	Tool string
	// Step 은 이 도구를 설치하는 단계다.
	Step string
	// NativeArchs 는 공식 이미지가 내는 아키텍처다.
	NativeArchs []string
	// Sources 는 공식 이미지가 못 덮는 클러스터에 쓸 대체 출처다. 앞의 것이 먼저다.
	Sources []ArchImageSource
}

// ImageSourceResolution 은 노드 아키텍처에 맞춰 고른 이미지 출처다.
type ImageSourceResolution struct {
	// Source 가 nil 이면 공식 이미지를 그대로 쓴다.
	Source *ArchImageSource
	// Unsupported 는 어느 이미지로도 뜰 수 없는 노드 아키텍처다.
	Unsupported []string
}

var toolImageProfiles = []ToolImageProfile{
	{
		Tool:        "Harbor",
		Step:        "installing_harbor",
		NativeArchs: []string{ArchAMD64},
		Sources: []ArchImageSource{{
			Archs:     []string{ArchAMD64, ArchARM64},
			Registry:  HarborMultiArchImageRegistry,
			Tag:       HarborMultiArchImageTag,
			Reference: HarborMultiArchImageReference,
		}},
	},
	{
		// sonatype/nexus3 는 3.80 부터 멀티아키다. 설치하는 3.64.0(NexusAppVersion)은
		// 단일 아키(amd64) 매니페스트이고 대체 출처가 없어 arm64 노드에서는 막는다.
		// 버전 상향으로 풀리는 문제라 대체 이미지를 두지 않는다(nullus-plan#70).
		Tool:        "Nexus",
		Step:        "installing_nexus",
		NativeArchs: []string{ArchAMD64},
	},
}

// ToolImageProfiles 는 선언된 프로파일 전부의 사본을 돌려준다.
func ToolImageProfiles() []ToolImageProfile {
	out := make([]ToolImageProfile, 0, len(toolImageProfiles))
	for _, p := range toolImageProfiles {
		out = append(out, p.clone())
	}
	return out
}

// ToolImageProfileForStep 은 설치 단계의 프로파일을 찾는다.
func ToolImageProfileForStep(step string) (ToolImageProfile, bool) {
	for _, p := range toolImageProfiles {
		if p.Step == step {
			return p.clone(), true
		}
	}
	return ToolImageProfile{}, false
}

// ToolImageProfileForTool 은 매트릭스의 도구 이름으로 프로파일을 찾는다.
func ToolImageProfileForTool(name string) (ToolImageProfile, bool) {
	name = strings.TrimSpace(name)
	for _, p := range toolImageProfiles {
		if strings.EqualFold(p.Tool, name) {
			return p.clone(), true
		}
	}
	return ToolImageProfile{}, false
}

// SupportedArchs 는 공식 이미지나 대체 출처로 뜰 수 있는 아키텍처다(정렬).
func (p ToolImageProfile) SupportedArchs() []string {
	all := slices.Clone(p.NativeArchs)
	for _, s := range p.Sources {
		all = append(all, s.Archs...)
	}
	return normalizeArchList(all)
}

// Resolve 는 클러스터의 노드 아키텍처에서 쓸 이미지 출처를 고른다.
//
//   - 노드를 모르면(빈 목록) 판단하지 않고 공식 이미지를 쓴다.
//   - 공식 이미지가 모든 노드를 덮으면 공식 이미지를 쓴다.
//   - 아니면 모든 노드를 덮는 첫 대체 출처를 쓴다.
//   - 그런 출처가 없으면 못 뜨는 아키텍처를 Unsupported 로 돌려준다.
func (p ToolImageProfile) Resolve(nodeArchs []string) ImageSourceResolution {
	archs := normalizeArchList(nodeArchs)
	if len(archs) == 0 || coversAll(p.NativeArchs, archs) {
		return ImageSourceResolution{}
	}
	for _, s := range p.Sources {
		if coversAll(s.Archs, archs) {
			src := s.clone()
			return ImageSourceResolution{Source: &src}
		}
	}

	supported := p.SupportedArchs()
	var unsupported []string
	for _, a := range archs {
		if !slices.Contains(supported, a) {
			unsupported = append(unsupported, a)
		}
	}
	// 아키텍처마다 뜰 이미지는 있는데 한 벌로 모두를 덮는 출처가 없는 경우다
	// (예: 공식 amd64 + arm64 전용 출처, 혼합 클러스터). 공식 이미지가 못 뜨는 쪽을 알린다.
	if len(unsupported) == 0 {
		for _, a := range archs {
			if !slices.Contains(p.NativeArchs, a) {
				unsupported = append(unsupported, a)
			}
		}
	}
	return ImageSourceResolution{Unsupported: unsupported}
}

func (p ToolImageProfile) clone() ToolImageProfile {
	out := p
	out.NativeArchs = slices.Clone(p.NativeArchs)
	out.Sources = make([]ArchImageSource, 0, len(p.Sources))
	for _, s := range p.Sources {
		out.Sources = append(out.Sources, s.clone())
	}
	return out
}

func (s ArchImageSource) clone() ArchImageSource {
	out := s
	out.Archs = slices.Clone(s.Archs)
	return out
}

func coversAll(have, want []string) bool {
	for _, a := range want {
		if !slices.Contains(have, a) {
			return false
		}
	}
	return true
}

// normalizeArchList 는 공백·빈 값·중복을 걷어내고 정렬한다.
func normalizeArchList(archs []string) []string {
	var out []string
	for _, a := range archs {
		a = strings.TrimSpace(a)
		if a == "" || slices.Contains(out, a) {
			continue
		}
		out = append(out, a)
	}
	slices.Sort(out)
	return out
}

// GitLab Runner 의 Kubernetes executor 는 잡마다 helper 컨테이너를 띄운다. 그
// 이미지 태그에는 아키텍처가 박혀 있고(gitlab-runner-helper:x86_64-v17.7.0),
// 설정하지 않으면 러너가 x86_64 를 고른다 — 러너 자신이 arm64 로 돌고 있어도
// 그렇다. arm64 노드에서는 helper 의 init 컨테이너가 실행되지 못해 그 스택의
// 모든 CI 잡이 runner_system_failure 로 끝난다.
//
// 이 실패는 설치로는 드러나지 않는다. 헬름은 성공하고 러너 파드도 Running 이며
// 스택은 completed 로 끝나고 Pre-Deploy Gate 도 통과한다 — 파이프라인을 실제로
// 돌려 봐야 드러난다. Harbor(#270)와 같은 부류이지만 그 게이트는 차트가 받는
// 컴포넌트 이미지만 보므로 러너 설정 안의 이 이미지는 잡지 못한다.
const GitLabRunnerHelperImageRepository = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper"

// gitLabRunnerHelperArchTokens 는 노드 아키텍처 → helper 이미지 태그의 아키텍처 조각이다.
// 러너가 내는 태그 이름을 그대로 따른다 — amd64 는 x86_64 로 적힌다.
var gitLabRunnerHelperArchTokens = map[string]string{
	ArchAMD64: "x86_64",
	ArchARM64: "arm64",
}

// GitLabRunnerHelperImage 는 이 클러스터의 노드에서 뜨는 helper 이미지를 고른다.
//
// 노드 아키텍처가 한 종류일 때만 고른다. 섞여 있으면 태그 하나로 양쪽을 덮을 수
// 없고, 노드를 모르거나 처음 보는 아키텍처면 지금까지의 동작(러너 기본값)을
// 그대로 둔다 — 잘못 박으면 지금 도는 클러스터를 우리가 깨뜨린다.
func GitLabRunnerHelperImage(nodeArchs []string, version string) (string, bool) {
	archs := normalizeArchList(nodeArchs)
	if len(archs) != 1 {
		return "", false
	}
	token, ok := gitLabRunnerHelperArchTokens[archs[0]]
	if !ok {
		return "", false
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return "", false
	}
	return GitLabRunnerHelperImageRepository + ":" + token + "-" + version, true
}
