package helm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// DGX Spark(arm64)에서 Harbor 가 뜨지 않은 원인이다 — 공식 goharbor 이미지는 amd64
// 뿐인데 설치가 노드 아키텍처를 보지 않고 차트 기본 이미지를 깔았다(#270).
func TestParseNodeArchitectures(t *testing.T) {
	archs, err := parseNodeArchitectures([]byte(`{"items":[
		{"status":{"nodeInfo":{"architecture":"arm64"}}},
		{"status":{"nodeInfo":{"architecture":"amd64"}}},
		{"status":{"nodeInfo":{"architecture":"arm64"}}},
		{"status":{"nodeInfo":{"architecture":""}}}
	]}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"amd64", "arm64"}, archs)

	archs, err = parseNodeArchitectures([]byte(`{"items":[]}`))
	require.NoError(t, err)
	assert.Empty(t, archs)

	_, err = parseNodeArchitectures([]byte(`not json`))
	assert.Error(t, err)
}

func TestArchImageValuesForStep_ARM64Harbor(t *testing.T) {
	values := archImageValuesForStep("installing_harbor", []string{domain.ArchARM64})
	require.NotNil(t, values, "arm64 클러스터에서 Harbor 는 공식 이미지로 뜨지 않는다")

	got := map[string]string{}
	collectImageOverrides("", values, got)
	assert.Equal(t, map[string]string{
		"nginx.image":               "ghcr.io/dasomel/goharbor/nginx-photon:v2.15.0-build.32",
		"portal.image":              "ghcr.io/dasomel/goharbor/harbor-portal:v2.15.0-build.32",
		"core.image":                "ghcr.io/dasomel/goharbor/harbor-core:v2.15.0-build.32",
		"jobservice.image":          "ghcr.io/dasomel/goharbor/harbor-jobservice:v2.15.0-build.32",
		"registry.registry.image":   "ghcr.io/dasomel/goharbor/registry-photon:v2.15.0-build.32",
		"registry.controller.image": "ghcr.io/dasomel/goharbor/harbor-registryctl:v2.15.0-build.32",
		"database.internal.image":   "ghcr.io/dasomel/goharbor/harbor-db:v2.15.0-build.32",
		"redis.internal.image":      "ghcr.io/dasomel/goharbor/redis-photon:v2.15.0-build.32",
		"trivy.image":               "ghcr.io/dasomel/goharbor/trivy-adapter-photon:v2.15.0-build.32",
	}, got)
	// exporter 는 바꾸지 않는다 — fork 의 v2.15.x 태그 빌드는 arm64 슬롯에도 amd64
	// 바이너리가 들어 있다. 설치가 Harbor metrics 를 켜지 않아 exporter 파드가 없다.
	assert.NotContains(t, values, "exporter")
}

func TestArchImageValuesForStep_AMD64HarborKeepsOfficialImages(t *testing.T) {
	assert.Nil(t, archImageValuesForStep("installing_harbor", []string{domain.ArchAMD64}))
	assert.Nil(t, archImageValuesForStep("installing_harbor", nil), "노드를 모르면 지금처럼 공식 이미지")
}

func TestArchImageValuesForStep_StepWithoutProfile(t *testing.T) {
	assert.Nil(t, archImageValuesForStep("installing_argocd", []string{domain.ArchARM64}))
}

func TestValuesForStep_ARM64Cluster_HarborUsesMultiArchImages(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("not-a-kubeconfig"), "nullus-demo")
	o.SetStackConfig(domain.StackConfig{AccessDomain: "nullus.internal"})
	o.setNodeArchitectures([]string{domain.ArchARM64})

	spec, ok := DefaultChartSpecForStep("installing_harbor")
	require.True(t, ok)
	values := o.valuesForStep("installing_harbor", spec)

	core := values["core"].(map[string]any)
	assert.Equal(t, map[string]any{
		"repository": "ghcr.io/dasomel/goharbor/harbor-core",
		"tag":        "v2.15.0-build.32",
	}, core["image"])
	assert.NotNil(t, core["resources"], "이미지를 바꾸면서 같은 블록의 다른 값을 지우면 안 된다")
	assert.Equal(t, "https://harbor.nullus.internal", values["externalURL"])
}

// 사용자가 yaml_overrides 로 자기 미러를 가리키면 그쪽이 이긴다. 대체 출처는
// 기본값 자리에 둔다.
func TestValuesForStep_ARM64Cluster_UserOverrideWins(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("not-a-kubeconfig"), "nullus-demo")
	o.SetStackConfig(domain.StackConfig{
		AccessDomain: "nullus.internal",
		YAMLOverrides: map[string]string{
			"installing_harbor": "core:\n  image:\n    repository: mirror.example.com/harbor-core\n    tag: v2.15.0-custom\n",
		},
	})
	o.setNodeArchitectures([]string{domain.ArchARM64})

	spec, _ := DefaultChartSpecForStep("installing_harbor")
	values := o.valuesForStep("installing_harbor", spec)

	got := map[string]string{}
	collectImageOverrides("", values, got)
	assert.Equal(t, "mirror.example.com/harbor-core:v2.15.0-custom", got["core.image"])
	assert.Equal(t, "ghcr.io/dasomel/goharbor/harbor-portal:v2.15.0-build.32", got["portal.image"],
		"사용자가 건드리지 않은 컴포넌트는 대체 출처를 유지한다")
}

func TestValuesForStep_AMD64Cluster_HarborKeepsChartImages(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("not-a-kubeconfig"), "nullus-demo")
	o.SetStackConfig(domain.StackConfig{AccessDomain: "nullus.internal"})
	o.setNodeArchitectures([]string{domain.ArchAMD64})

	spec, _ := DefaultChartSpecForStep("installing_harbor")
	values := o.valuesForStep("installing_harbor", spec)

	got := map[string]string{}
	collectImageOverrides("", values, got)
	assert.Empty(t, got, "amd64 클러스터는 차트 기본(공식 goharbor) 이미지를 쓴다")
}

func TestCheckStepArchitectures_BlocksUnsupportedArch(t *testing.T) {
	notices, err := checkStepArchitectures([]string{"installing_harbor", "installing_argocd"}, []string{"s390x"})

	require.Error(t, err, "어느 이미지로도 뜨지 않으면 헬름 설치 전에 멈춰야 한다")
	msg := err.Error()
	assert.Contains(t, msg, "Harbor")
	assert.Contains(t, msg, "s390x")
	assert.Contains(t, msg, "amd64, arm64", "어느 아키텍처면 되는지 알려 준다")
	assert.NotContains(t, msg, "Argo CD", "프로파일이 없는 도구는 판단하지 않는다")
	assert.Empty(t, notices)
}

// 대체 출처가 없는 도구(Nexus 3.64.0)는 arm64 노드에서 설치 전에 멈춘다.
func TestCheckStepArchitectures_BlocksToolWithoutSource(t *testing.T) {
	_, err := checkStepArchitectures([]string{"installing_nexus"}, []string{domain.ArchARM64})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Nexus(installing_nexus)")
	assert.Contains(t, err.Error(), "arm64")
}

func TestCheckStepArchitectures_NoticesAlternativeSource(t *testing.T) {
	notices, err := checkStepArchitectures([]string{"installing_harbor"}, []string{domain.ArchARM64})

	require.NoError(t, err)
	require.Len(t, notices, 1)
	assert.Contains(t, notices[0], "Harbor")
	assert.Contains(t, notices[0], "arm64")
	assert.Contains(t, notices[0], "ghcr.io/dasomel/goharbor")
	assert.Contains(t, notices[0], "v2.15.0-build.32")
	assert.Contains(t, notices[0], "https://github.com/dasomel/harbor")
}

func TestCheckStepArchitectures_OfficialImagesAreSilent(t *testing.T) {
	notices, err := checkStepArchitectures([]string{"installing_harbor"}, []string{domain.ArchAMD64})
	require.NoError(t, err)
	assert.Empty(t, notices)
}

// 대체 출처는 선언했는데 설치 어댑터가 그 차트의 이미지 자리를 모르면 이미지를
// 바꿀 수 없다 — 그대로 설치하면 공식 이미지로 깔려 같은 실패가 난다.
func TestCheckStepArchitectures_BlocksSourceWithoutChartComponents(t *testing.T) {
	_, err := checkStepArchitecturesWith([]string{"installing_example"}, []string{domain.ArchARM64},
		func(step string) (domain.ToolImageProfile, bool) {
			return domain.ToolImageProfile{
				Tool: "Example", Step: step, NativeArchs: []string{domain.ArchAMD64},
				Sources: []domain.ArchImageSource{{Archs: []string{domain.ArchAMD64, domain.ArchARM64}, Registry: "example.io", Tag: "v1"}},
			}, true
		})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Example")
}

// 설치 유스케이스는 이 모양의 메서드가 있을 때만 검사를 부른다(타입 단언). 시그니처가
// 어긋나면 컴파일은 되고 검사만 조용히 빠지므로 여기서 고정한다.
var _ interface {
	PreflightArchitecture(ctx context.Context, stackID string) ([]string, error)
} = (*Orchestrator)(nil)

func TestPreflightArchitecture_SkipsWithoutKubeconfig(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("not-a-kubeconfig"), "nullus-demo")

	notices, err := o.PreflightArchitecture(t.Context(), "stk_1")
	assert.NoError(t, err)
	assert.Empty(t, notices)
}

func nodesJSON(archs ...string) []byte {
	items := make([]string, 0, len(archs))
	for _, a := range archs {
		items = append(items, `{"status":{"nodeInfo":{"architecture":"`+a+`"}}}`)
	}
	return []byte(`{"items":[` + strings.Join(items, ",") + `]}`)
}

func harborStackConfig() domain.StackConfig {
	return domain.StackConfig{
		AccessDomain: "nullus.internal",
		Artifacts: domain.ArtifactsConfig{
			ContainerRegistry: domain.ToolSelection{Name: "Harbor", Enabled: true},
		},
	}
}

func nexusStackConfig() domain.StackConfig {
	return domain.StackConfig{
		AccessDomain: "nullus.internal",
		Artifacts: domain.ArtifactsConfig{
			ContainerRegistry: domain.ToolSelection{Name: "Nexus", Enabled: true},
		},
	}
}

// 노드를 못 읽었는데 이미지가 노드 아키텍처에 달린 도구를 설치하면 안 된다. 조용히 공식
// 이미지로 가면 arm64 에서 #270 이 그대로 재발한다.
func TestPreflightArchitecture_BlocksWhenNodesUnreadable(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("not-a-kubeconfig"), "nullus-demo")
	o.SetStackConfig(harborStackConfig())
	o.nodeReader = func(context.Context) ([]byte, error) { return nil, errors.New("connection refused") }

	_, err := o.PreflightArchitecture(t.Context(), "stk_1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Harbor(installing_harbor)")
	assert.Contains(t, err.Error(), "connection refused")
}

// 노드에 달린 도구가 없으면 못 읽어도 막을 이유가 없다 — 알림만 남긴다.
func TestPreflightArchitecture_UnreadableNodesWithoutArchDependentSteps(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("not-a-kubeconfig"), "nullus-demo")
	o.SetStackConfig(domain.StackConfig{AccessDomain: "nullus.internal"})
	o.nodeReader = func(context.Context) ([]byte, error) { return nil, errors.New("connection refused") }

	notices, err := o.PreflightArchitecture(t.Context(), "stk_1")

	require.NoError(t, err)
	require.Len(t, notices, 1)
	assert.Contains(t, notices[0], "노드 아키텍처를 읽지 못해")
}

// 이어서 진행할 때 이미 끝난 단계는 다시 설치하지 않으므로 검사하지 않는다. 검사하면
// 이미 떠 있는 도구 때문에 뒤 단계를 영영 이어갈 수 없게 된다.
func TestPreflightArchitecture_SkipsStepsCompletedBeforeResume(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("not-a-kubeconfig"), "nullus-demo")
	o.SetStackConfig(nexusStackConfig())
	o.nodeReader = func(context.Context) ([]byte, error) { return nodesJSON("amd64", "arm64"), nil }

	_, err := o.PreflightArchitecture(t.Context(), "stk_fresh")
	require.Error(t, err, "처음 설치라면 Nexus 는 arm64 노드에서 뜨지 않아 막아야 한다")

	o.ResumeFromStep("stk_resume", "installing_argocd")
	_, err = o.PreflightArchitecture(t.Context(), "stk_resume")
	assert.NoError(t, err, "installing_nexus 는 재개 지점 앞이라 이미 끝났다")
}

// 사전검사가 읽은 노드 아키텍처가 실제 설치 values 까지 이어져야 한다.
func TestExecuteStep_Harbor_InstallsImagesForNodeArchitecture(t *testing.T) {
	installer := &mockInstaller{}
	o := NewOrchestrator(installer, []byte("not-a-kubeconfig"), "nullus-demo")
	o.SetStackConfig(harborStackConfig())
	o.nodeReader = func(context.Context) ([]byte, error) { return nodesJSON("arm64"), nil }

	notices, err := o.PreflightArchitecture(t.Context(), "stk_arm")
	require.NoError(t, err)
	require.Len(t, notices, 1)

	o.ResumeFromStep("stk_arm", "installing_harbor")
	require.NoError(t, o.ExecuteStep(t.Context(), "stk_arm", "installing_harbor", "B"))

	got := map[string]string{}
	collectImageOverrides("", installer.valuesByRelease["harbor"], got)
	assert.Equal(t, "ghcr.io/dasomel/goharbor/harbor-core:v2.15.0-build.32", got["core.image"])
}

// 사전검사를 거치지 않았어도 단계가 스스로 노드를 읽는다. 못 읽으면 설치하지 않는다.
func TestExecuteStep_Harbor_FailsWhenNodesUnreadable(t *testing.T) {
	installer := &mockInstaller{}
	o := NewOrchestrator(installer, []byte("not-a-kubeconfig"), "nullus-demo")
	o.SetStackConfig(harborStackConfig())
	o.nodeReader = func(context.Context) ([]byte, error) { return nil, errors.New("connection refused") }

	o.ResumeFromStep("stk_arm", "installing_harbor")
	err := o.ExecuteStep(t.Context(), "stk_arm", "installing_harbor", "B")

	require.Error(t, err)
	assert.Empty(t, installer.installed)
}

// 도메인이 대체 출처를 선언한 도구는 설치 어댑터가 그 차트의 이미지 자리를 알아야 한다.
func TestArchImageComponents_CoverEveryProfileWithSources(t *testing.T) {
	for _, p := range domain.ToolImageProfiles() {
		if len(p.Sources) == 0 {
			continue
		}
		assert.NotEmptyf(t, archImageComponents[p.Step], "%s(%s) 의 차트 이미지 자리가 없다", p.Tool, p.Step)
	}
}
