package repository

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 인메모리 시드에 있는 템플릿은 DB 마이그레이션에도 있어야 한다.
//
// 인메모리 저장소는 개발·테스트 경로이고 실제 배포는 PostgreSQL 을 읽는다.
// 한쪽에만 넣으면 개발 환경에서는 보이는 템플릿이 운영에서는 목록에 없다 —
// Harbor/Nexus 템플릿이 정확히 그 상태였다.
func TestSeededTemplatesExistInMigrations(t *testing.T) {
	sql := readMigrationSQL(t)

	templates, err := NewMemoryTemplateRepository().List(context.Background())
	require.NoError(t, err)

	for _, tmpl := range templates {
		if len(tmpl.Tools) == 0 {
			continue // 빈 템플릿은 UI 전용 시작점이라 시드 대상이 아니다.
		}
		assert.Truef(t, strings.Contains(sql, "'"+tmpl.ID+"'"),
			"template %s is seeded in memory but not in any migration", tmpl.ID)
	}
}

// 매트릭스도 같다. 템플릿만 시드하고 매트릭스를 빠뜨리면 Pre-Deploy Gate 가
// 판정할 근거가 없어 설치 직전에 막힌다.
func TestSeededMatricesExistInMigrations(t *testing.T) {
	sql := readMigrationSQL(t)

	matrices, err := NewMemoryCompatibilityRepository().GetAll(context.Background())
	require.NoError(t, err)

	for _, m := range matrices {
		assert.Truef(t, strings.Contains(sql, "'"+m.ID+"'"),
			"matrix %s is seeded in memory but not in any migration", m.ID)
	}
}

// readMigrationSQL 은 모든 up 마이그레이션을 하나로 이어 붙여 돌려준다.
func readMigrationSQL(t *testing.T) string {
	t.Helper()

	root := filepath.Join("..", "..", "..", "..", "db", "migrations")
	entries, err := os.ReadDir(root)
	require.NoErrorf(t, err, "마이그레이션 디렉터리를 읽지 못했습니다: %s", root)

	var sb strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, entry.Name())) // #nosec G304 -- 저장소 내 고정 경로
		require.NoError(t, err)
		sb.Write(body)
		sb.WriteString("\n")
	}
	require.NotEmpty(t, sb.String(), "up 마이그레이션을 찾지 못했습니다")
	return sb.String()
}
