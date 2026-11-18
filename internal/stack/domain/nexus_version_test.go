package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Nexus 앱 버전은 Community Edition 경계를 넘을 수 없다.
//
// Sonatype 은 3.77.0(2025-02-04)부터 무료판을 Community Edition 으로 바꾸고 별도 EULA
// 를 받는다. 그 EULA 는 제품을 "Licensee product or service offering" 에 넣는 것
// (xiv), 제3자를 위한 사용(vii), 재배포(v)를 금지하고 내부 업무 용도로만 허락한다 —
// Nullus 가 고객 클러스터에 설치해 주는 방식과 정면으로 충돌한다. 3.77.0 이상은
// EULA 를 수락해야 컴포넌트를 올리고 받을 수 있고(사용량 한도도 강제된다), 수락하는
// 주체가 고객이어야 하므로 설치 단계가 대신할 수 없다.
//
// 버전을 올려야 하면(보안 패치 등) 이 경계 아래에서 고르거나, 설치 대신 고객이 가진
// Nexus 에 연결하는 방식을 택한다. 근거: airgap/THIRD_PARTY_NOTICES.md 의 Nexus 항목.
func TestNexusAppVersion_StaysBeforeCommunityEditionEULA(t *testing.T) {
	got := parseJenkinsVersion(t, NexusAppVersion)
	boundary := parseJenkinsVersion(t, NexusCommunityEditionFirstAppVersion)

	require.Negativef(t, compareVersions(got, boundary),
		"Nexus appVersion %s 이 Community Edition 경계 %s 이상이다 — CE EULA 가 제품 내장·재배포를 금지한다",
		NexusAppVersion, NexusCommunityEditionFirstAppVersion)
}

// 경계 상수 자체가 바뀌면 위 가드가 의미를 잃는다. 값은 Sonatype 3.77.0 릴리스 노트가 정한다.
func TestNexusCommunityEditionFirstAppVersion_Is3770(t *testing.T) {
	require.Equal(t, "3.77.0", NexusCommunityEditionFirstAppVersion)
}

// 3.71.0 부터 Nexus 는 OrientDB 를 열지 못한다(H2 · PostgreSQL 만). 이미 OrientDB 시대
// 태그로 떠 있는 릴리스를 새 기본 태그로 올리면 파드가 데이터베이스를 열지 못해 서지
// 않는다 — 재시도 · 도구 추가가 설치 단계를 다시 돌릴 때 그 릴리스의 태그를 지킨다.
func TestNexusImageTagForRelease(t *testing.T) {
	cases := []struct {
		name     string
		deployed string
		want     string
	}{
		{"새 설치는 기본 태그", "", NexusAppVersion},
		{"OrientDB 시대(3.64.0)는 그대로", "3.64.0", "3.64.0"},
		{"OrientDB 마지막 줄(3.70.5)도 그대로", "3.70.5", "3.70.5"},
		{"H2 시대는 기본 태그로", "3.71.0", NexusAppVersion},
		{"이미 기본 태그면 그대로", NexusAppVersion, NexusAppVersion},
		{"읽을 수 없는 태그는 기본 태그로", "latest", NexusAppVersion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, NexusImageTagForRelease(tc.deployed))
		})
	}
}

// 기본 태그 자체가 OrientDB 시대면 위 규칙이 거꾸로 동작한다 — 새 설치가 H2 를 써야 한다.
func TestNexusAppVersion_IsOnH2(t *testing.T) {
	got := parseJenkinsVersion(t, NexusAppVersion)
	boundary := parseJenkinsVersion(t, NexusH2FirstAppVersion)
	require.GreaterOrEqualf(t, compareVersions(got, boundary), 0,
		"Nexus appVersion %s 이 OrientDB 시대다 — %s 이상이어야 한다", NexusAppVersion, NexusH2FirstAppVersion)
}
