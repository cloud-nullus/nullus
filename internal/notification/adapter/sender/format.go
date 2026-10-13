// Package sender 는 채널 종류마다 메시지를 실제로 보내는 어댑터다 — Slack Incoming Webhook,
// Telegram Bot API, SMTP.
//
// 공통 규칙 두 가지. 하나, 오류 문구에 비밀(웹훅 URL · 봇 토큰 · 비밀번호)을 싣지 않는다 — 문구가
// 발송 이력과 화면에 그대로 남는다. 둘, 실패를 다시 보낼 만한 것(네트워크 · 5xx · 429)과 설정이
// 틀린 것으로 나눠 port.SendError 로 돌려준다.
package sender

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

const (
	// maxFieldValue 는 필드 값 하나의 최대 글자 수다(오류 문구가 통째로 실리는 경우).
	maxFieldValue = 500
	// maxSummary 는 요약의 최대 글자 수다.
	maxSummary = 3000
)

func severityEmoji(s sharednotify.Severity) string {
	switch s {
	case sharednotify.SeverityCritical:
		return "🔴"
	case sharednotify.SeverityWarning:
		return "🟠"
	default:
		return "🔵"
	}
}

func severityLabel(s sharednotify.Severity) string {
	switch s {
	case sharednotify.SeverityCritical:
		return "심각"
	case sharednotify.SeverityWarning:
		return "경고"
	default:
		return "정보"
	}
}

func formatTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05 MST")
}

// truncate 는 글자(rune) 기준으로 자르고 말줄임표를 붙인다.
func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

// httpFailure 는 HTTP 응답 실패를 분류한다. 429 · 5xx 는 다시 보낸다.
func httpFailure(status int, detail string, retryAfter time.Duration) error {
	retryable := status == http.StatusTooManyRequests || status >= 500
	return &port.SendError{
		Err:        fmt.Errorf("HTTP %d: %s", status, truncate(strings.TrimSpace(detail), 300)),
		Retryable:  retryable,
		RetryAfter: retryAfter,
	}
}

// transportFailure 는 요청 자체가 실패한 것(연결 · 시간 초과 · TLS)이다. url.Error 는 요청 URL 을
// 문구에 싣는다 — 웹훅 URL 과 봇 토큰이 거기 있으므로 URL 을 떼고 원인만 남긴다.
//
// 연결이 안 되거나 시간이 지난 것은 다시 보낸다. 인증서를 믿을 수 없는 것은 다시 보내도 같다.
func transportFailure(what string, err error, secrets ...string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	return &port.SendError{
		Err:       fmt.Errorf("%s 요청 실패: %s", what, redact(err.Error(), secrets...)),
		Retryable: !isCertificateError(err),
	}
}

func isCertificateError(err error) bool {
	var verifyErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	return errors.As(err, &verifyErr) || errors.As(err, &unknownAuthority) ||
		errors.As(err, &hostname) || errors.As(err, &invalid)
}

func redact(s string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "***")
		}
	}
	return s
}

func parseRetryAfter(v string) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

// fieldsOf 는 요약과 필드를 자른 사본이다.
func fieldsOf(msg domain.Message) (string, []sharednotify.Field) {
	fields := make([]sharednotify.Field, 0, len(msg.Fields))
	for _, f := range msg.Fields {
		fields = append(fields, sharednotify.Field{Name: f.Name, Value: truncate(f.Value, maxFieldValue)})
	}
	return truncate(msg.Summary, maxSummary), fields
}

func httpClientOrDefault(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	// 기본 Transport 는 HTTPS_PROXY 를 따른다 — 사내 프록시 뒤에서도 보낸다.
	return &http.Client{Timeout: 15 * time.Second}
}
