package domain

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture 는 kind 의 Trivy 서버(client/server 모드)로 node:16 을 실제로 스캔한
// 리포트를 줄인 것이다. 형식을 추측해서 만들지 않는다.
func loadNode16Report(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/trivy-report-node16.json")
	require.NoError(t, err)
	return raw
}

func TestParseTrivyReport_CountsBySeverity(t *testing.T) {
	s, err := ParseTrivyReport(loadNode16Report(t))
	require.NoError(t, err)

	assert.Equal(t, SeverityCounts{Critical: 5, High: 7, Medium: 6, Low: 6, Unknown: 2}, s.All)
	// 수정본이 있는 것만 센 값. --ignore-unfixed 정책에서 게이트가 보는 숫자다.
	assert.Equal(t, SeverityCounts{Critical: 3, High: 4, Medium: 4, Low: 4, Unknown: 2}, s.Fixable)
}

func TestParseTrivyReport_Identity(t *testing.T) {
	s, err := ParseTrivyReport(loadNode16Report(t))
	require.NoError(t, err)

	assert.Equal(t, "node", s.ImageRepository)
	assert.Equal(t, "16", s.ImageTag)
	// 결과는 태그가 아니라 digest 에 붙인다 — 태그는 움직인다.
	assert.Equal(t, "sha256:f77a1aef2da8d83e45ec990f45df50f1a286c5fe8bbfb8c6e4246c6389705c0b", s.ImageDigest)
	assert.Equal(t, "0.74.0", s.ScannerVersion)
}

// client/server 모드에서는 리포트에 서버의 DB 메타가 실린다. DB 나이를 서버에
// 따로 묻지 않고 CI 잡이 이미 남기는 리포트에서 얻는다(설계 §6.4).
func TestParseTrivyReport_DBUpdatedAtFromServer(t *testing.T) {
	s, err := ParseTrivyReport(loadNode16Report(t))
	require.NoError(t, err)

	require.NotNil(t, s.DBUpdatedAt)
	assert.Equal(t, time.Date(2026, 9, 13, 1, 11, 30, 263571485, time.UTC), s.DBUpdatedAt.UTC())
}

// DB 메타가 없는 리포트(standalone 등)는 "모름" 이다. 0 값 시각을 넣으면
// 1년 넘게 낡은 DB 로 보이거나, 반대로 비교를 건너뛰어 초록불이 된다.
func TestParseTrivyReport_MissingDBMetaIsUnknown(t *testing.T) {
	s, err := ParseTrivyReport([]byte(`{"SchemaVersion":2,"ArtifactName":"alpine:3.18","Results":[]}`))
	require.NoError(t, err)
	assert.Nil(t, s.DBUpdatedAt)
	assert.Equal(t, SeverityCounts{}, s.All)
}

func TestParseTrivyReport_RejectsGarbage(t *testing.T) {
	_, err := ParseTrivyReport([]byte("not json"))
	assert.Error(t, err)
}

func TestSeverityCounts_AddAndPlus(t *testing.T) {
	var c SeverityCounts
	for _, s := range []string{"CRITICAL", "high", " Medium ", "LOW", "NEGLIGIBLE", ""} {
		c.Add(s)
	}
	// 등급을 모르는 것은 Unknown 으로 센다 — 낮음으로 넘겨짚지 않는다.
	assert.Equal(t, SeverityCounts{Critical: 1, High: 1, Medium: 1, Low: 1, Unknown: 2}, c)
	assert.Equal(t, SeverityCounts{Critical: 2, High: 2, Medium: 2, Low: 2, Unknown: 4}, c.Plus(c))
}

// 에어갭에서 DB 는 사람이 가져다 넣는 만큼만 새것이다. 6개월 된 DB 의
// "0건" 은 정보가 아니라 오해다(설계 §6.4 — 임계 30일).
func TestIsDBStale(t *testing.T) {
	now := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	fresh := now.Add(-24 * time.Hour)
	old := now.Add(-31 * 24 * time.Hour)

	assert.False(t, IsDBStale(&fresh, now))
	assert.True(t, IsDBStale(&old, now))
	// 모르면 낡은 것으로 본다 — 초록불을 켜지 않는다.
	assert.True(t, IsDBStale(nil, now))
}
