package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const harborTrivySeedMigration = "000084_seed_gitlab_harbor_trivy_template.up.sql"

// 시드 SQL 과 인메모리 저장소는 같은 템플릿을 말해야 한다.
//
// 개발 환경은 인메모리, 실제 설치는 DB 시드를 읽는다. gitlab-harbor-v1 은 둘이 이미
// 갈라져 있다 — 시드는 GitLab 9.5.1 을, 설치는 8.7.2 를 말한다. 새 템플릿은 SQL 의
// 도구 목록과 매트릭스를 인메모리 정의와 맞춰 두어, 한쪽만 고치면 여기서 깨지게 한다.
func TestSeedMigration_GitLabHarborTrivy_MatchesMemory(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "db", "migrations", harborTrivySeedMigration))
	require.NoError(t, err)

	// $$...$$ 로 감싼 JSON 두 덩어리: 템플릿 도구 목록, 매트릭스 도구 맵.
	blocks := regexp.MustCompile(`(?s)\$\$(.*?)\$\$`).FindAllStringSubmatch(string(raw), -1)
	require.Len(t, blocks, 2, "시드는 템플릿 도구 목록과 매트릭스 도구 맵을 하나씩 가진다")

	var seededTools []domain.ToolConfig
	require.NoError(t, json.Unmarshal([]byte(blocks[0][1]), &seededTools))
	var seededMatrix map[string]domain.ToolVersion
	require.NoError(t, json.Unmarshal([]byte(blocks[1][1]), &seededMatrix))
	// 뒤 마이그레이션이 바꾼 값까지 적용해야 지금 DB 와 같다.
	applyToolImageArchSupportMigration(t, seededMatrix)

	tmpl, err := NewMemoryTemplateRepository().GetByID(context.Background(), "gitlab-harbor-trivy-v1")
	require.NoError(t, err)
	assert.Equal(t, tmpl.Tools, seededTools, "시드 템플릿 도구가 인메모리 정의와 다르다")

	matrix, err := NewMemoryCompatibilityRepository().GetByID(context.Background(), "gitlab-harbor-trivy-v1")
	require.NoError(t, err)
	assert.Equal(t, matrix.Tools, seededMatrix, "시드 매트릭스가 인메모리 정의와 다르다")
}
