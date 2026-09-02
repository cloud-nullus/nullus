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

func TestSelectDumpImage_같은_major_안에서는_높은_minor_를_고른다(t *testing.T) {
	// Keycloak DB 는 17.2 다. 같은 major 안에서는 어차피 호환이므로,
	// 버그 수정이 더 들어간 17.5 를 쓴다.
	got, err := SelectDumpImage("17.2", bundled())
	require.NoError(t, err)
	assert.Equal(t, "17.5.0", got.Version)
}

// 리허설에서 걸린 조합이다 — minor 로 막으면 서버가 패치만 받아도 백업이
// 멈춘다. 막으려던 것과 정반대 방향의 사고다.
func TestIsCompatible_minor_차이는_막지_않는다(t *testing.T) {
	ok, err := IsCompatible("18.3", "18.4")
	require.NoError(t, err)
	assert.True(t, ok, "같은 major 안의 minor 차이는 호환이다")
}

func TestIsCompatible_major_가_낮으면_거부한다(t *testing.T) {
	ok, err := IsCompatible("16.9", "17.0")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestIsCompatible_major_가_높으면_허용한다(t *testing.T) {
	ok, err := IsCompatible("18.0", "17.5")
	require.NoError(t, err)
	assert.True(t, ok, "pg_dump 는 자기보다 낮은 major 의 서버를 덤프할 수 있다")
}

func TestIsCompatible_해석_불가(t *testing.T) {
	_, err := IsCompatible("latest", "17.5")
	require.Error(t, err)
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
	assert.Equal(t, "17.5.0", got.Version, "major 17 중 가장 높은 minor")
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
