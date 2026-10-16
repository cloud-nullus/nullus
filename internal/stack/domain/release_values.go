package domain

import (
	"fmt"
	"reflect"
	"strings"
)

// 배포된 스택의 Helm values 를 사용자가 직접 편집할 때, 플랫폼이 스스로 계산해
// 넣은 값까지 함께 노출된다. 그 값들은 지우거나 바꾸면 스택이 조용히 깨진다 —
// Harbor 의 externalURL 을 되돌리면 노드의 containerd 가 레지스트리 주소를 풀지
// 못해 배포된 앱이 ImagePullBackOff 에서 나오지 못하고, GitLab 의 global.psql 을
// 건드리면 웹서비스가 DB 에 붙지 못한다.
//
// 편집 자체를 막지는 않는다 — 전문가용 탈출구가 이 기능의 존재 이유다.
// 대신 무엇을 건드렸는지 적용 전에 반드시 보여 준다.

// ProtectedValueKind 는 보호 경로가 어떻게 훼손됐는지 구분한다.
type ProtectedValueKind string

const (
	// ProtectedValueRemoved 는 플랫폼이 넣어 둔 경로가 편집본에서 사라진 경우다.
	ProtectedValueRemoved ProtectedValueKind = "removed"
	// ProtectedValueChanged 는 값이 다른 것으로 바뀐 경우다.
	ProtectedValueChanged ProtectedValueKind = "changed"
)

// ProtectedValueViolation 은 편집본이 플랫폼 소유 경로를 훼손한 한 건이다.
type ProtectedValueViolation struct {
	Path    string             `json:"path"`
	Kind    ProtectedValueKind `json:"kind"`
	Message string             `json:"message"`
}

// protectedValuePathsByStep 은 오케스트레이터가 설치 시 직접 계산해 넣는 values
// 경로다. 여기 없는 경로는 사용자 마음대로 바꿔도 플랫폼 배선이 깨지지 않는다.
var protectedValuePathsByStep = map[string][]string{
	// harborExternalURLValues
	"installing_harbor": {"externalURL"},
	// gitlabExternalSharedServiceValues + accessDomain 배선
	"installing_gitlab": {
		"global.psql",
		"global.appConfig.object_store",
		"global.minio.enabled",
		"global.hosts.domain",
		"postgresql.install",
		"registry.storage",
	},
	// sharedPostgresValues — 비밀번호는 values 가 아니라 프로비저닝된 Secret 에서 온다.
	"installing_postgresql": {
		"auth.existingSecret",
		"auth.secretKeys",
		"auth.username",
		"auth.database",
	},
	// 러너는 클러스터 내부 GitLab 주소로 등록된다.
	"installing_runner": {"gitlabUrl"},
	// MinIO 는 네임스페이스와 OIDC 블록을 플랫폼이 채운다.
	"installing_minio": {"namespace", "oidc"},
	// oidcValuesForStep
	"installing_grafana": {"envValueFrom", "grafana.ini.auth.generic_oauth"},
	"installing_argocd":  {"configs.cm", "configs.secret"},
	// openBaoValues — StorageClass 가 비면 PVC 가 Pending 에서 멈춘다.
	"installing_openbao": {"server.dataStorage.storageClass"},
	// sonarqubeSharedServiceValues + sonarQubeSAMLValues — 공유 DB 주소, 서버 주소(ACS 의
	// 바탕), Keycloak 로그인 설정. 인증서가 옛것으로 얼면 모든 SAML 로그인이 서명
	// 검증에서 실패한다.
	"installing_sonarqube": {
		"jdbcOverwrite.jdbcUrl",
		"sonarProperties.sonar.core.serverBaseURL",
		"sonarProperties.sonar.auth.saml.enabled",
		"sonarProperties.sonar.auth.saml.applicationId",
		"sonarProperties.sonar.auth.saml.providerId",
		"sonarProperties.sonar.auth.saml.loginUrl",
		"sonarProperties.sonar.auth.saml.certificate.secured",
	},
}

// ProtectedValuePaths 는 해당 설치 단계에서 플랫폼이 소유한 values 경로다.
// 모르는 단계는 빈 슬라이스를 돌려준다 — 보호할 것이 없다는 뜻이다.
func ProtectedValuePaths(step string) []string {
	paths, ok := protectedValuePathsByStep[strings.TrimSpace(step)]
	if !ok {
		return nil
	}
	out := make([]string, len(paths))
	copy(out, paths)
	return out
}

// ProtectedValueViolations 는 편집본(edited)이 현재 배포값(base)의 보호 경로를
// 지웠거나 바꿨는지 검사한다.
//
// base 에 없던 경로를 편집본이 새로 채우는 것은 위반이 아니다. 플랫폼이 아직
// 그 값을 세팅하지 않은 구성(예: access_domain 미설정)에서 사용자가 직접
// 지정하는 정상 경로이기 때문이다.
func ProtectedValueViolations(step string, base, edited map[string]any) []ProtectedValueViolation {
	var violations []ProtectedValueViolation

	for _, path := range ProtectedValuePaths(step) {
		baseValue, baseFound := lookupValuePath(base, path)
		if !baseFound {
			continue
		}

		editedValue, editedFound := lookupValuePath(edited, path)
		switch {
		case !editedFound:
			violations = append(violations, ProtectedValueViolation{
				Path:    path,
				Kind:    ProtectedValueRemoved,
				Message: fmt.Sprintf("%s 는 플랫폼이 설치 시 계산해 넣는 값이다. 지우면 기본값으로 되돌아간다.", path),
			})
		case !reflect.DeepEqual(baseValue, editedValue):
			violations = append(violations, ProtectedValueViolation{
				Path:    path,
				Kind:    ProtectedValueChanged,
				Message: fmt.Sprintf("%s 는 플랫폼이 설치 시 계산해 넣는 값이다. 바꾸면 스택 배선이 어긋날 수 있다.", path),
			})
		}
	}

	return violations
}

// lookupValuePath 는 점으로 구분된 경로를 values 맵에서 찾는다.
//
// 키 자체에 점이 들어 있을 수 있다(grafana.ini, sonar.auth.saml.*). 점마다 쪼개기만
// 하면 그런 경로는 영영 못 찾아 보호가 조용히 꺼지므로, 각 단계에서 남은 조각을
// 이어 붙인 긴 키부터 맞춰 본다.
func lookupValuePath(values map[string]any, path string) (any, bool) {
	if len(values) == 0 || strings.TrimSpace(path) == "" {
		return nil, false
	}
	return lookupValueSegments(values, strings.Split(path, "."))
}

func lookupValueSegments(current any, segments []string) (any, bool) {
	if len(segments) == 0 {
		return current, true
	}
	node, ok := current.(map[string]any)
	if !ok {
		return nil, false
	}
	for n := len(segments); n >= 1; n-- {
		child, ok := node[strings.Join(segments[:n], ".")]
		if !ok {
			continue
		}
		if found, ok := lookupValueSegments(child, segments[n:]); ok {
			return found, true
		}
	}
	return nil, false
}
