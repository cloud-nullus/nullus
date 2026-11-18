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
