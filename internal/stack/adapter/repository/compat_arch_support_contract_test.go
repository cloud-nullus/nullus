package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const toolImageArchSupportMigration = "000085_tool_image_arch_support.up.sql"

// 매트릭스의 ArchSupport 는 설치가 실제로 내는 이미지와 같아야 한다.
//
// Harbor 선언이 매트릭스마다 달랐다 — gitlab-harbor-* 는 amd64, gitea-jenkins-argocd-* 는
// amd64·arm64. 설치는 공식 amd64 이미지만 깔았으므로 gitea 템플릿은 arm64 클러스터에서
// Pre-Deploy Gate 를 통과하고 installing_harbor 에서 멈췄다(DGX Spark, #270). Nexus 는
// amd64 뿐인 3.64.0 을 깔면서 amd64·arm64 로 선언돼 있었다.
// 선언은 도메인(ToolImageProfile)이 소유하고 매트릭스는 그것을 따른다.
func TestMemoryCompatibility_ToolArchSupportMatchesImageProfiles(t *testing.T) {
	matrices, err := NewMemoryCompatibilityRepository().GetAll(context.Background())
	require.NoError(t, err)

	checked := map[string]bool{}
	for _, m := range matrices {
		for category, tool := range m.Tools {
			profile, ok := domain.ToolImageProfileForTool(tool.Name)
			if !ok {
				continue
			}
			checked[profile.Tool] = true
			assert.Equalf(t, profile.SupportedArchs(), tool.ArchSupport,
				"%s 매트릭스의 %s(%s) ArchSupport 가 설치 이미지 선언과 다르다", m.ID, tool.Name, category)
		}
	}
	for _, p := range domain.ToolImageProfiles() {
		assert.Truef(t, checked[p.Tool], "%s 를 쓰는 매트릭스가 없어 선언을 확인하지 못했다", p.Tool)
	}
}

// DB 는 마이그레이션이 같은 값으로 맞춘다. 선언된 도구마다 그 도구가 들어가는 모든
// 카테고리를 갱신해야 한다.
func TestToolImageArchSupportMigration_MatchesImageProfiles(t *testing.T) {
	updates := toolImageArchSupportUpdates(t)
	require.NotEmpty(t, updates)

	for _, u := range updates {
		profile, ok := domain.ToolImageProfileForTool(u.tool)
		require.Truef(t, ok, "마이그레이션이 선언에 없는 도구 %s 를 바꾼다", u.tool)
		assert.Equalf(t, profile.SupportedArchs(), u.archs, "%s(%s)", u.tool, u.category)
	}

	// 메모리 매트릭스에서 그 도구가 쓰이는 카테고리는 모두 마이그레이션에 있어야 한다.
	matrices, err := NewMemoryCompatibilityRepository().GetAll(context.Background())
	require.NoError(t, err)
	for _, m := range matrices {
		for category, tool := range m.Tools {
			if _, ok := domain.ToolImageProfileForTool(tool.Name); !ok {
				continue
			}
			found := false
			for _, u := range updates {
				found = found || (u.category == category && strings.EqualFold(u.tool, tool.Name))
			}
			assert.Truef(t, found, "마이그레이션이 %s(%s) 를 갱신하지 않는다", tool.Name, category)
		}
	}
}

type archSupportUpdate struct {
	category string
	tool     string
	archs    []string
}

var archSupportUpdatePattern = regexp.MustCompile(
	`(?s)jsonb_set\(tools, '\{(\w+),ArchSupport\}', \$\$(.*?)\$\$::jsonb, true\).*?WHERE lower\(tools->'(\w+)'->>'Name'\) = '([^']+)'`)

// toolImageArchSupportUpdates 는 000085 의 UPDATE 문을 (카테고리, 도구, 아키텍처) 로 읽는다.
func toolImageArchSupportUpdates(t *testing.T) []archSupportUpdate {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "db", "migrations", toolImageArchSupportMigration))
	require.NoError(t, err)

	var out []archSupportUpdate
	for _, m := range archSupportUpdatePattern.FindAllStringSubmatch(string(raw), -1) {
		require.Equalf(t, m[1], m[3], "갱신하는 카테고리(%s)와 조건의 카테고리(%s)가 다르다", m[1], m[3])
		var archs []string
		require.NoError(t, json.Unmarshal([]byte(m[2]), &archs))
		out = append(out, archSupportUpdate{category: m[1], tool: m[4], archs: archs})
	}
	return out
}

// applyToolImageArchSupportMigration 은 시드 매트릭스에 000085 를 적용한 결과다.
func applyToolImageArchSupportMigration(t *testing.T, tools map[string]domain.ToolVersion) {
	t.Helper()
	for _, u := range toolImageArchSupportUpdates(t) {
		if tv, ok := tools[u.category]; ok && strings.EqualFold(tv.Name, u.tool) {
			tv.ArchSupport = u.archs
			tools[u.category] = tv
		}
	}
}
