package domain

import "regexp"

// redacted 는 가린 자리에 남기는 표시다.
const redacted = "[가림]"

// secretPatterns 는 도구 결과 문자열에서 가릴 것이다. 도구 결과는 외부 LLM 으로 나간다 — MCP 도구가
// kubeconfig 같은 시크릿 필드를 빼도, 설치 로그 같은 문자열 안의 토큰은 남는다. 키 이름은 남겨 무엇이
// 가려졌는지 알 수 있게 한다.
var secretPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), redacted},
	{regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]{8,}`), "${1}" + redacted},
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}`), redacted},
	{regexp.MustCompile(`glpat-[A-Za-z0-9_-]{10,}`), redacted},
	{regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), redacted},
	{regexp.MustCompile(`(?i)((?:password|passwd|secret|token|api[_-]?key|access[_-]?key)[A-Za-z0-9_-]*["']?\s*[:=]\s*["']?)[^\s"',;]+`), "${1}" + redacted},
}

// RedactSecrets 는 문자열 안의 토큰 · 비밀번호 · 개인 키를 가린다.
func RedactSecrets(s string) string {
	for _, p := range secretPatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return s
}
