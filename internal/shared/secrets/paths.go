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
