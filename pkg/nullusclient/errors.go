package nullusclient

import "fmt"

// Kind 는 API 호출 실패의 분류다. 값은 Automation 계약 §1 의 exit code 와
// 일치한다 — CLI 는 Kind.ExitCode() 를 그대로 프로세스 종료 코드로 쓰고,
// MCP 는 isError 메시지의 카테고리로 쓴다.
type Kind int

const (
	KindUsage    Kind = 2 // 잘못된 요청 — 400, 409 등 4xx (아래 예외 제외)
	KindAuth     Kind = 3 // 인증·권한 — 401, 403
	KindNotFound Kind = 4 // 대상 없음 — 404
	KindServer   Kind = 5 // 서버 오류·연결 실패 — 5xx, transport 오류
)

// ExitCode 는 automation 계약의 프로세스 종료 코드를 반환한다.
func (k Kind) ExitCode() int { return int(k) }

func (k Kind) String() string {
	switch k {
	case KindUsage:
		return "usage"
	case KindAuth:
		return "auth"
	case KindNotFound:
		return "not-found"
	case KindServer:
		return "server"
	}
	return fmt.Sprintf("kind(%d)", int(k))
}

// APIError 는 실패한 API 호출 하나를 설명한다. HTTP 응답 실패와 transport
// 실패(연결 불가 등) 모두 이 타입으로 돌아온다 — 일반 분기는 Kind 로 하고,
// 같은 상태 코드 안에서 갈라야 하는 경우(예: 400 중 DEPLOY_COMPAT_WARN_UNACK
// 를 --ack-warnings 안내로 바꾸는 A-5)만 Code 를 본다.
type APIError struct {
	Kind       Kind
	StatusCode int    // transport 실패면 0
	Code       string // 서버 error envelope 의 도메인 코드 (예: DEPLOY_COMPAT_WARN_UNACK)
	Message    string // envelope 의 message, 없으면 본문/원인 요약
	TraceID    string // 서버 request id — 지원 문의·로그 대조용
	cause      error
}

func (e *APIError) Error() string {
	if e.StatusCode == 0 {
		return fmt.Sprintf("nullus API 연결 실패: %s", e.Message)
	}
	s := fmt.Sprintf("nullus API %d (%s): %s", e.StatusCode, e.Kind, e.Message)
	if e.TraceID != "" {
		s += " [trace_id=" + e.TraceID + "]"
	}
	return s
}

func (e *APIError) Unwrap() error { return e.cause }

func kindForStatus(status int) Kind {
	switch {
	case status == 401 || status == 403:
		return KindAuth
	case status == 404:
		return KindNotFound
	case status >= 500:
		return KindServer
	default:
		return KindUsage
	}
}
