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

// 11-01 실측 — exec 스트림을 느리게 소비하면 끝부분이 떨어진다. 스풀 디렉터리를 주면 아카이버가 먼저
// 호스트 디스크에 빨리 받아 두고 봉인 · 업로드한다(§4.3 의 "중간 파일 없음" 은 기본값으로 지킨다 — 옵션).
func TestLoadConfig_백업_스풀_디렉터리는_환경변수로_받는다(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  port: 8080\n"), 0o600))
	t.Setenv("NULLUS_BACKUP_SPOOL_DIR", "/var/tmp/nullus-spool")

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "/var/tmp/nullus-spool", cfg.Backup.SpoolDir)
}
