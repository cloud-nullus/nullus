package secrets

import (
	"fmt"
	"strings"
)

// DefaultEnv 는 환경 이름이 비었을 때 쓰는 값이다.
const DefaultEnv = "dev"

// 시크릿 경로 규약: kv/nullus/{env}/{org}/{module}/{provider}/{name}
//
// 이 경로들은 쓰는 쪽과 읽는 쪽이 다른 모듈에 있다. 예를 들어 GitHub PAT 는
// stack 모듈이 설치 마지막에 기록하고 cicd 모듈이 파이프라인을 만들 때 읽는다.
// 양쪽에 문자열을 따로 두면 한쪽만 바뀌어도 컴파일은 통과하고 런타임에만
// "등록된 토큰이 없다" 로 드러난다 — 그래서 단일 출처로 모은다.

// GitHubAPITokenPath 는 조직 GitHub PAT 의 경로다.
func GitHubAPITokenPath(env, orgID string) string {
	return fmt.Sprintf("kv/nullus/%s/%s/cicd/github/api-token", normalizeEnv(env), strings.TrimSpace(orgID))
}

// GitLabAPITokenPath 는 스택 GitLab 자동화 토큰의 경로다.
func GitLabAPITokenPath(env, orgID string) string {
	return fmt.Sprintf("kv/nullus/%s/%s/cicd/gitlab/api-token", normalizeEnv(env), strings.TrimSpace(orgID))
}

func normalizeEnv(env string) string {
	env = strings.TrimSpace(env)
	if env == "" {
		return DefaultEnv
	}
	return env
}

// PipelineCredentialsPolicyPath 는 컨트롤러가 지울 수 있는 파이프라인 자격증명의 정책 경로다
// (KV 마운트 뒤). 스택 모듈이 부트스트랩 정책에 쓰고, cicd 모듈은 PipelineCredentialPrefix 로
// 같은 깊이에 값을 쓴다.
//
// 끝을 * 가 아니라 + 로 둔다. OpenBao 는 맞는 규칙 중 가장 구체적인 하나만 쓰는데, 끝이 *
// 이면서 + 가 많은 규칙은 nullus/* 보다 우선순위가 낮아 delete 가 무시된다(OpenBao 2.5.5 확인).
const PipelineCredentialsPolicyPath = "metadata/nullus/+/+/cicd/pipelines/+/+"

// PipelineCredentialPrefix 는 한 파이프라인의 자격증명이 사는 경로다(끝이 /).
//
// 앱 이름은 경로의 한 세그먼트여야 한다. "..", "/" 가 들어가면 경로가 다른 파이프라인의 값을
// 가리킨다 — client-go 의 proxy 요청은 경로를 정리(path.Join)해서 보낸다. 조직이 비면 예전부터
// 쓰던 "default" 다.
func PipelineCredentialPrefix(env, orgID, app string) (string, error) {
	name := strings.TrimSpace(app)
	if !IsPathSegment(name) {
		return "", fmt.Errorf("파이프라인 이름 %q 은 금고 경로의 한 세그먼트로 쓸 수 없습니다", app)
	}
	org := strings.TrimSpace(orgID)
	if org == "" {
		org = "default"
	}
	return fmt.Sprintf("kv/nullus/%s/%s/cicd/pipelines/%s/", normalizeEnv(env), org, name), nil
}

// IsPathSegment 는 경로의 한 세그먼트로 안전한지 본다 — 비어 있지 않고, 구분자가 없고, "." · ".."
// 이 아니다.
func IsPathSegment(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, `/\`)
}
