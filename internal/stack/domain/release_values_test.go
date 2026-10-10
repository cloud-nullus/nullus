package domain_test

import (
	"slices"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func TestProtectedValuePaths_KnownStepHasPaths(t *testing.T) {
	paths := domain.ProtectedValuePaths("installing_harbor")
	if len(paths) == 0 {
		t.Fatal("installing_harbor 는 플랫폼이 소유한 경로가 있어야 한다")
	}
	found := false
	for _, p := range paths {
		if p == "externalURL" {
			found = true
		}
	}
	if !found {
		t.Fatalf("externalURL 이 보호 경로에 없다: %v", paths)
	}
}

func TestProtectedValuePaths_UnknownStepIsEmpty(t *testing.T) {
	if paths := domain.ProtectedValuePaths("installing_unknown"); len(paths) != 0 {
		t.Fatalf("모르는 단계는 보호 경로가 없어야 한다: %v", paths)
	}
}

func TestProtectedValueViolations_NoChangeIsClean(t *testing.T) {
	base := map[string]any{"externalURL": "http://harbor.example.com"}
	edited := map[string]any{"externalURL": "http://harbor.example.com", "trivy": map[string]any{"enabled": false}}

	if got := domain.ProtectedValueViolations("installing_harbor", base, edited); len(got) != 0 {
		t.Fatalf("보호 경로를 건드리지 않았는데 경고가 났다: %v", got)
	}
}

func TestProtectedValueViolations_RemovedPathIsReported(t *testing.T) {
	base := map[string]any{"externalURL": "http://harbor.example.com"}
	edited := map[string]any{"trivy": map[string]any{"enabled": false}}

	got := domain.ProtectedValueViolations("installing_harbor", base, edited)
	if len(got) != 1 || got[0].Path != "externalURL" || got[0].Kind != domain.ProtectedValueRemoved {
		t.Fatalf("삭제된 보호 경로가 보고되지 않았다: %+v", got)
	}
}

func TestProtectedValueViolations_ChangedPathIsReported(t *testing.T) {
	base := map[string]any{"externalURL": "http://harbor.example.com"}
	edited := map[string]any{"externalURL": "http://nope.invalid"}

	got := domain.ProtectedValueViolations("installing_harbor", base, edited)
	if len(got) != 1 || got[0].Kind != domain.ProtectedValueChanged {
		t.Fatalf("변경된 보호 경로가 보고되지 않았다: %+v", got)
	}
}

func TestProtectedValueViolations_NestedPath(t *testing.T) {
	base := map[string]any{
		"global": map[string]any{
			"psql": map[string]any{"host": "nullus-postgresql.nullus.svc.cluster.local", "port": 5432},
		},
	}
	edited := map[string]any{
		"global": map[string]any{
			"psql": map[string]any{"host": "localhost", "port": 5432},
		},
	}

	got := domain.ProtectedValueViolations("installing_gitlab", base, edited)
	if len(got) == 0 {
		t.Fatal("중첩된 보호 경로 변경이 잡히지 않았다")
	}
	if got[0].Path != "global.psql" {
		t.Fatalf("보고된 경로가 틀렸다: %s", got[0].Path)
	}
}

// base 에 없던 보호 경로를 편집본이 새로 채우는 건 위반이 아니다. 플랫폼이
// 아직 그 값을 세팅하지 않은 구성(예: accessDomain 미설정)에서 사용자가
// 직접 지정하는 정상 경로다.
func TestProtectedValueViolations_AddedPathIsAllowed(t *testing.T) {
	base := map[string]any{}
	edited := map[string]any{"externalURL": "http://harbor.example.com"}

	if got := domain.ProtectedValueViolations("installing_harbor", base, edited); len(got) != 0 {
		t.Fatalf("없던 값을 채운 것은 위반이 아니다: %+v", got)
	}
}

// values 키에는 점이 들어간다(grafana.ini, sonar.auth.saml.*). 점마다 쪼개 찾으면
// 그런 경로는 영영 못 찾아 보호가 조용히 꺼진다.
func TestProtectedValueViolations_KeysContainingDots(t *testing.T) {
	base := map[string]any{
		"sonarProperties": map[string]any{
			"sonar.auth.saml.certificate.secured": "NEW-CERT",
			"sonar.web.javaOpts":                  "-Xmx1G",
		},
	}
	edited := map[string]any{
		"sonarProperties": map[string]any{
			"sonar.auth.saml.certificate.secured": "OLD-CERT",
			"sonar.web.javaOpts":                  "-Xmx2G",
		},
	}

	got := domain.ProtectedValueViolations("installing_sonarqube", base, edited)
	if len(got) != 1 || got[0].Path != "sonarProperties.sonar.auth.saml.certificate.secured" {
		t.Fatalf("SAML 인증서 변경만 보고해야 한다(사용자 속성은 자유): %+v", got)
	}

	grafanaBase := map[string]any{"grafana.ini": map[string]any{"auth.generic_oauth": map[string]any{"enabled": "true"}}}
	grafanaEdited := map[string]any{"grafana.ini": map[string]any{}}
	if got := domain.ProtectedValueViolations("installing_grafana", grafanaBase, grafanaEdited); len(got) != 1 {
		t.Fatalf("grafana.ini 아래 OIDC 블록을 지운 것이 잡히지 않았다: %+v", got)
	}
}

// SonarQube 는 공유 DB 주소와 서버 주소, Keycloak 로그인 설정을 플랫폼이 계산한다.
func TestProtectedValuePaths_SonarQube(t *testing.T) {
	paths := domain.ProtectedValuePaths("installing_sonarqube")
	for _, want := range []string{
		"jdbcOverwrite.jdbcUrl",
		"sonarProperties.sonar.core.serverBaseURL",
		"sonarProperties.sonar.auth.saml.certificate.secured",
		"sonarProperties.sonar.auth.saml.providerId",
		"sonarProperties.sonar.auth.saml.loginUrl",
		"sonarProperties.sonar.auth.saml.applicationId",
	} {
		if !slices.Contains(paths, want) {
			t.Fatalf("보호 경로에 %s 가 없다: %v", want, paths)
		}
	}
}
