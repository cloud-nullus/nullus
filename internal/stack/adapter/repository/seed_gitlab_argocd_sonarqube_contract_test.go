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

const gitlabArgoCDSonarQubeSeedMigration = "000087_seed_gitlab_argocd_sonarqube_template.up.sql"

// 시드 SQL 과 인메모리 저장소는 같은 템플릿을 말해야 한다. 개발 환경은 인메모리, 실제
// 설치는 DB 시드를 읽으므로 한쪽만 고치면 화면과 설치가 갈라진다.
func TestSeedMigration_GitLabArgoCDSonarQube_MatchesMemory(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "db", "migrations", gitlabArgoCDSonarQubeSeedMigration))
	require.NoError(t, err)

	blocks := regexp.MustCompile(`(?s)\$\$(.*?)\$\$`).FindAllStringSubmatch(string(raw), -1)
	require.Len(t, blocks, 2, "시드는 템플릿 도구 목록과 매트릭스 도구 맵을 하나씩 가진다")

	var seededTools []domain.ToolConfig
	require.NoError(t, json.Unmarshal([]byte(blocks[0][1]), &seededTools))
	var seededMatrix map[string]domain.ToolVersion
	require.NoError(t, json.Unmarshal([]byte(blocks[1][1]), &seededMatrix))

	tmpl, err := NewMemoryTemplateRepository().GetByID(context.Background(), "gitlab-argocd-sonarqube-v1")
	require.NoError(t, err)
	assert.Equal(t, tmpl.Tools, seededTools, "시드 템플릿 도구가 인메모리 정의와 다르다")

	matrix, err := NewMemoryCompatibilityRepository().GetByID(context.Background(), "gitlab-argocd-sonarqube-v1")
	require.NoError(t, err)
	assert.Equal(t, matrix.Tools, seededMatrix, "시드 매트릭스가 인메모리 정의와 다르다")
}
