package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 백업 봉인 키와 목적지 자격증명은 설정 파일에 없다 — 차트가 Secret 에서
// 환경변수로 넣는다. viper 의 AutomaticEnv 는 아는 키만 보므로 명시적으로
// 묶어야 하고, 그것을 빠뜨리면 값이 **조용히 무시된다.**
//
// kind 인클러스터 리허설에서 실제로 그렇게 됐다. 파드는 떴는데 백업 모듈만
// "키가 32바이트가 아니다" 로 기동을 거부했고, 원인이 설정 파일 어디에도
// 보이지 않아 찾는 데 시간이 걸렸다.
func TestLoadConfig_백업_비밀값은_환경변수로만_들어온다(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// 설정 파일에는 backup.seal_key 가 **없다** — 차트가 렌더하는 모양 그대로다.
	require.NoError(t, os.WriteFile(path, []byte(`
server:
  port: 8080
backup:
  enabled: true
  destination:
    endpoint: "minio.internal:9000"
    bucket: "nullus-backup"
    access_key: "ak"
`), 0o600))

	t.Setenv("NULLUS_BACKUP_SEAL_KEY", "seal-key-32bytes-padding-value!!")
	t.Setenv("NULLUS_BACKUP_DESTINATION_SECRET_KEY", "sk-from-secret")

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	assert.True(t, cfg.Backup.Enabled)
	assert.Equal(t, "seal-key-32bytes-padding-value!!", cfg.Backup.SealKey,
		"설정 파일에 없는 키도 환경변수로 들어와야 한다")
	assert.Equal(t, "sk-from-secret", cfg.Backup.Destination.SecretKey)
	assert.Equal(t, "ak", cfg.Backup.Destination.AccessKey, "파일 값은 그대로 유지")
}

func TestLoadConfig_백업_비밀값이_없으면_빈_값(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  port: 8080\n"), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Empty(t, cfg.Backup.SealKey)
	assert.False(t, cfg.Backup.Enabled)
}
