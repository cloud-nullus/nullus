package domain

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolImageProfileForStep_Harbor(t *testing.T) {
	p, ok := ToolImageProfileForStep("installing_harbor")
	require.True(t, ok, "Harbor 는 공식 이미지가 amd64 뿐이라 프로파일이 있어야 한다")

	assert.Equal(t, "Harbor", p.Tool)
	assert.Equal(t, []string{ArchAMD64}, p.NativeArchs, "공식 goharbor 이미지는 단일 아키(amd64) 매니페스트다")
	assert.Equal(t, []string{ArchAMD64, ArchARM64}, p.SupportedArchs())
}

// sonatype/nexus3 는 3.80 부터 멀티아키다. 설치하는 3.64.0 은 단일 아키(amd64)이고
// 대체 출처가 없으므로 arm64 노드에서는 설치 전에 막아야 한다.
func TestToolImageProfileForStep_Nexus(t *testing.T) {
	p, ok := ToolImageProfileForStep("installing_nexus")
	require.True(t, ok)

	assert.Equal(t, "Nexus", p.Tool)
	assert.Equal(t, []string{ArchAMD64}, p.SupportedArchs())
	assert.Empty(t, p.Sources)

	got := p.Resolve([]string{ArchARM64})
	assert.Nil(t, got.Source)
	assert.Equal(t, []string{ArchARM64}, got.Unsupported)
}

func TestToolImageProfileForStep_UnknownStep(t *testing.T) {
	_, ok := ToolImageProfileForStep("installing_argocd")
	assert.False(t, ok, "프로파일이 없는 도구는 공식 이미지가 모든 아키텍처를 낸다고 본다")
}

func TestToolImageProfileForTool_CaseInsensitive(t *testing.T) {
	p, ok := ToolImageProfileForTool("  harbor ")
	require.True(t, ok)
	assert.Equal(t, "installing_harbor", p.Step)

	_, ok = ToolImageProfileForTool("GHCR")
	assert.False(t, ok)
}

func TestToolImageProfile_Resolve_AMD64ClusterUsesOfficialImages(t *testing.T) {
	p, _ := ToolImageProfileForStep("installing_harbor")

	got := p.Resolve([]string{ArchAMD64})

	assert.Nil(t, got.Source, "amd64 클러스터는 공식 이미지를 그대로 쓴다")
	assert.Empty(t, got.Unsupported)
}

func TestToolImageProfile_Resolve_ARM64ClusterUsesMultiArchSource(t *testing.T) {
	p, _ := ToolImageProfileForStep("installing_harbor")

	got := p.Resolve([]string{ArchARM64})

	require.NotNil(t, got.Source, "arm64 클러스터에서 공식 이미지는 뜨지 않는다")
	assert.Equal(t, HarborMultiArchImageRegistry, got.Source.Registry)
	assert.Equal(t, HarborMultiArchImageTag, got.Source.Tag)
	assert.Equal(t, "https://github.com/dasomel/harbor", got.Source.Reference)
	assert.Empty(t, got.Unsupported)
}

// 한 벌의 이미지가 모든 노드에 깔린다. amd64·arm64 가 섞인 클러스터에서 공식 이미지를
// 쓰면 arm64 노드에 잡힌 파드가 죽는다 — 양쪽을 다 내는 대체 출처를 써야 한다.
func TestToolImageProfile_Resolve_MixedClusterUsesMultiArchSource(t *testing.T) {
	p, _ := ToolImageProfileForStep("installing_harbor")

	got := p.Resolve([]string{ArchARM64, ArchAMD64})

	require.NotNil(t, got.Source)
	assert.Equal(t, HarborMultiArchImageRegistry, got.Source.Registry)
	assert.Empty(t, got.Unsupported)
}

// 노드를 읽지 못했으면 판단하지 않는다 — 지금까지처럼 공식 이미지를 쓴다.
func TestToolImageProfile_Resolve_UnknownArchsUsesOfficialImages(t *testing.T) {
	p, _ := ToolImageProfileForStep("installing_harbor")

	for _, archs := range [][]string{nil, {}, {"", "  "}} {
		got := p.Resolve(archs)
		assert.Nil(t, got.Source)
		assert.Empty(t, got.Unsupported)
	}
}

func TestToolImageProfile_Resolve_UnsupportedArch(t *testing.T) {
	p, _ := ToolImageProfileForStep("installing_harbor")

	got := p.Resolve([]string{"s390x", ArchAMD64})

	assert.Nil(t, got.Source, "어느 출처도 s390x 를 덮지 못한다")
	assert.Equal(t, []string{"s390x"}, got.Unsupported)
}

func TestToolImageProfile_Resolve_NormalizesInput(t *testing.T) {
	p, _ := ToolImageProfileForStep("installing_harbor")

	got := p.Resolve([]string{" arm64 ", "arm64", ""})

	require.NotNil(t, got.Source)
}

// 대체 출처가 공식 아키텍처를 덮지 못하면 혼합 클러스터에서 한 벌로 끝낼 수 없다.
// 그때는 공식 이미지가 못 뜨는 쪽을 지원 불가로 돌려준다.
func TestToolImageProfile_Resolve_SourceNotCoveringNativeArchsOnMixedCluster(t *testing.T) {
	p := ToolImageProfile{
		Tool:        "Example",
		Step:        "installing_example",
		NativeArchs: []string{ArchAMD64},
		Sources:     []ArchImageSource{{Archs: []string{ArchARM64}, Registry: "example.io/arm", Tag: "v1"}},
	}

	assert.NotNil(t, p.Resolve([]string{ArchARM64}).Source)

	got := p.Resolve([]string{ArchAMD64, ArchARM64})
	assert.Nil(t, got.Source)
	assert.Equal(t, []string{ArchARM64}, got.Unsupported)
}

func TestToolImageProfiles_ReturnsCopies(t *testing.T) {
	first := ToolImageProfiles()
	require.NotEmpty(t, first)
	first[0].NativeArchs[0] = "mutated"
	first[0].Sources[0].Archs[0] = "mutated"

	again, _ := ToolImageProfileForStep(first[0].Step)
	assert.NotEqual(t, "mutated", again.NativeArchs[0])
	assert.NotEqual(t, "mutated", again.Sources[0].Archs[0])
}

// 프로파일의 단계 이름이 설치 순서에 없으면 설치가 그 프로파일을 영영 보지 않는다.
func TestToolImageProfiles_StepsAreInstallSteps(t *testing.T) {
	for _, p := range ToolImageProfiles() {
		assert.Truef(t, slices.Contains(InstallStepOrder, p.Step), "%s 의 단계 %q 가 InstallStepOrder 에 없다", p.Tool, p.Step)
		for _, s := range p.Sources {
			assert.NotEmptyf(t, s.Registry, "%s 대체 출처에 저장소가 없다", p.Tool)
			assert.NotEmptyf(t, s.Tag, "%s 대체 출처에 태그가 없다", p.Tool)
			assert.NotEmptyf(t, s.Archs, "%s 대체 출처에 아키텍처가 없다", p.Tool)
		}
	}
}
