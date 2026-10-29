package domain

import "strings"

// 조회 질문은 첫 라운드에 도구 호출을 강제한다.
//
// 모델이 도구를 건너뛰고 답을 지어낸 적이 있다 — 2026-10-25 시연에서 "등록된 클러스터 목록" 에 도구 없이
// "클러스터가 없다" 고 답했고(실제 2개), 템플릿 개수도 매번 다르게 말했다. 규칙만으로는 막히지 않아,
// 스택 · 클러스터 · 템플릿 · 설치 · 로그를 묻는 질문이면 첫 라운드의 tool_choice 를 required 로 보낸다.
// 인사나 일반 질문까지 강제하면 엉뚱한 도구를 부르므로 낱말로 가른다.
var toolFirstKeywords = []string{
	"스택", "클러스터", "템플릿", "설치", "배포", "로그", "실패", "호환", "상태", "네임스페이스", "골든", "몇 개", "목록",
	"stack", "cluster", "template", "install", "deploy", "log", "fail", "compat", "status", "namespace", "golden", "how many", "list",
}

// RequiresToolFirst 는 질문이 플랫폼 사실을 묻는 조회 질문인지다.
func RequiresToolFirst(question string) bool {
	q := strings.ToLower(strings.TrimSpace(question))
	if q == "" {
		return false
	}
	for _, k := range toolFirstKeywords {
		if strings.Contains(q, k) {
			return true
		}
	}
	return false
}

// LastUserQuestion 은 대화에서 마지막 사용자 발화다. 없으면 빈 문자열.
func LastUserQuestion(turns []Turn) string {
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Role == RoleUser {
			return turns[i].Content
		}
	}
	return ""
}
