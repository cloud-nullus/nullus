package repository

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/scaffold"
)

// 스캐폴딩이 만드는 파이프라인과 템플릿이 선언한 단계가 같아야 한다.
//
// 어긋나면 화면이 돌지도 않은 단계를 보여준다 — 실제로 템플릿은
// Build/Test/ImageBuild/Deploy 를 선언했지만 스캐폴딩은 2단계만 만들었고,
// 화면은 4단계를 모두 "Completed" 로 그렸다.
func TestSeededTemplateStages_MatchScaffold(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()
	templates, err := repo.List(context.Background())
	require.NoError(t, err)

	want := scaffold.PipelineStageNames()
	scaffoldBacked := map[string]bool{"web-backend-v1": true, "batch-job-v1": true}

	found := 0
	for _, tpl := range templates {
		if !scaffoldBacked[tpl.ID] {
			continue
		}
		found++
		assert.Equalf(t, want, tpl.Stages,
			"템플릿 %s 가 선언한 단계가 스캐폴딩 결과와 다르다", tpl.ID)
	}
	require.Equal(t, len(scaffoldBacked), found, "대상 템플릿을 찾지 못했다")
}

// 마이그레이션도 같은 값이어야 한다. 메모리만 고치면 실제 배포에서는
// 옛 값이 남아 화면이 그대로 거짓 단계를 보여준다.
func TestMigrationStages_MatchScaffold(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..",
		"db", "migrations", "000070_align_pipeline_template_stages.up.sql"))
	require.NoError(t, err)

	var quoted []string
	for _, stage := range scaffold.PipelineStageNames() {
		quoted = append(quoted, `"`+stage+`"`)
	}
	want := "[" + strings.Join(quoted, ", ") + "]"

	assert.Containsf(t, string(raw), want,
		"마이그레이션이 스캐폴딩 단계(%s)를 시드하지 않는다", want)
}
