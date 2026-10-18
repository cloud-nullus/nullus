package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DB 접속 정보를 환경변수로 덮어쓰는 이름은 설정 키(`database.*`)를 그대로 따른
// NULLUS_DATABASE_* 다. `.env.example`·`.env.dev`·Makefile 이 오래 NULLUS_DB_* 를
// 적어 두었는데 그 이름은 어느 키와도 맞지 않아 **조용히 무시됐다** — 포트를 바꿔
// 적어도 설정 파일의 5433 으로 붙었다.
//
// 이름을 고친 뒤 다시 어긋나지 않도록, 먹는 이름과 먹지 않는 이름을 함께 못 박는다.
func TestLoadConfig_DB접속정보는_NULLUS_DATABASE_로_덮어쓴다(t *testing.T) {
	path := writeDatabaseConfig(t)

	t.Setenv("NULLUS_DATABASE_HOST", "db.internal")
	t.Setenv("NULLUS_DATABASE_PORT", "6543")
	t.Setenv("NULLUS_DATABASE_NAME", "nullus_prod")
	t.Setenv("NULLUS_DATABASE_USER", "svc_nullus")
	t.Setenv("NULLUS_DATABASE_PASSWORD", "from-secret")
	t.Setenv("NULLUS_DATABASE_SSLMODE", "require")

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "db.internal", cfg.Database.Host)
	assert.Equal(t, 6543, cfg.Database.Port)
	assert.Equal(t, "nullus_prod", cfg.Database.Name)
	assert.Equal(t, "svc_nullus", cfg.Database.User)
	assert.Equal(t, "from-secret", cfg.Database.Password)
	assert.Equal(t, "require", cfg.Database.SSLMode)
}

// 옛 이름은 먹지 않는다. 별칭으로 되살리지 않는 이유는 그 이름을 적어 둔 파일
// (`.env.example` 등)을 고쳤기 때문이고, 두 이름을 함께 두면 어느 쪽이 이기는지를
// 또 설명해야 한다.
func TestLoadConfig_옛_NULLUS_DB_이름은_먹지_않는다(t *testing.T) {
	path := writeDatabaseConfig(t)

	t.Setenv("NULLUS_DB_HOST", "db.internal")
	t.Setenv("NULLUS_DB_PORT", "6543")

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "localhost", cfg.Database.Host, "설정 파일 값이 그대로 남는다")
	assert.Equal(t, 5433, cfg.Database.Port)
}

func writeDatabaseConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
server:
  port: 8080
database:
  host: localhost
  port: 5433
  name: nullus
  user: nullus
  password: nullus_dev
  sslmode: disable
`), 0o600))

	return path
}
