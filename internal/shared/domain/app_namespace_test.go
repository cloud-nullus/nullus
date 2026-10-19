package domain

import "testing"

// 한 네임스페이스에 앱이 여럿이면 pull 자격증명도 앱마다 따로다. GitLab 프로젝트 레지스트리의 토큰은
// 그 프로젝트 이미지만 읽는다 — 이름이 하나면 나중에 만든 앱이 앞 앱의 토큰을 덮어 앞 앱의 pull 이
// insufficient_scope 로 막힌다(2026-10-19 kind, eGov 템플릿 두 개를 한 네임스페이스에 배포하다 실측).
func TestAppImagePullSecretNameFor(t *testing.T) {
	if got := AppImagePullSecretNameFor("egov-boot"); got != "nullus-registry-egov-boot" {
		t.Fatalf("AppImagePullSecretNameFor = %q", got)
	}
	if AppImagePullSecretNameFor("egov-boot") == AppImagePullSecretNameFor("egov-web") {
		t.Fatal("앱마다 이름이 달라야 한다")
	}
}
