package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 에어갭 번들에 실제로 들어 있는 이미지들 (airgap/images/images.txt).
func bundled() []DumpImage {
	return []DumpImage{
		{Ref: "docker.io/bitnamilegacy/postgresql:14.8.0", Version: "14.8.0"},
		{Ref: "docker.io/bitnamilegacy/postgresql:17.2.0-debian-12-r6", Version: "17.2.0"},
		{Ref: "docker.io/bitnamilegacy/postgresql:17.5.0-debian-12-r20", Version: "17.5.0"},
	}
}

func TestSelectDumpImage_서버보다_낮은_클라이언트를_고르지_않는다(t *testing.T) {
	// 플랫폼 DB 는 17.5 다. 14.8 을 고르면 pg_dump 가 거부한다.
	got, err := SelectDumpImage("17.5", bundled())
	require.NoError(t, err)
	assert.Equal(t, "17.5.0", got.Version)
}

func TestSelectDumpImage_충분한_것_중_가장_낮은_것을_고른다(t *testing.T) {
	// Keycloak DB 는 17.2 다. 17.2 로 충분하므로 17.5 를 쓰지 않는다.
	got, err := SelectDumpImage("17.2", bundled())
	require.NoError(t, err)
	assert.Equal(t, "17.2.0", got.Version)
}

func TestSelectDumpImage_구버전_서버(t *testing.T) {
	got, err := SelectDumpImage("14.8", bundled())
	require.NoError(t, err)
	assert.Equal(t, "14.8.0", got.Version)
}

func TestSelectDumpImage_맞는_것이_없으면_명확히_실패한다(t *testing.T) {
	// 조용히 낮은 버전을 고르는 것보다 실패가 낫다.
	_, err := SelectDumpImage("18.0", bundled())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "번들에 추가")
}

func TestSelectDumpImage_태그_접미사를_무시한다(t *testing.T) {
	got, err := SelectDumpImage("17.5.0-debian-12-r20", bundled())
	require.NoError(t, err)
	assert.Equal(t, "17.5.0", got.Version)
}

func TestSelectDumpImage_메이저만_있는_버전(t *testing.T) {
	got, err := SelectDumpImage("17", bundled())
	require.NoError(t, err)
	assert.Equal(t, "17.2.0", got.Version, "17.0 이상이면 되므로 17.2 로 충분하다")
}

func TestSelectDumpImage_잘못된_서버_버전(t *testing.T) {
	_, err := SelectDumpImage("", bundled())
	require.Error(t, err)
	_, err = SelectDumpImage("abc", bundled())
	require.Error(t, err)
}

func TestSelectDumpImage_해석불가_후보는_건너뛴다(t *testing.T) {
	got, err := SelectDumpImage("17.0", []DumpImage{
		{Ref: "broken", Version: "latest"},
		{Ref: "ok", Version: "17.5.0"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", got.Ref)
}

func TestSelectDumpImage_후보가_없으면_실패(t *testing.T) {
	_, err := SelectDumpImage("17.0", nil)
	require.Error(t, err)
}
