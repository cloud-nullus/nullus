package sender

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

const testToken = "123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0"

type captured struct {
	mu      sync.Mutex
	path    string
	body    map[string]any
	headers http.Header
}

func captureServer(t *testing.T, status int, response string, headers map[string]string) (*httptest.Server, *captured) {
	t.Helper()
	c := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.path = r.URL.Path
		c.headers = r.Header.Clone()
		_ = json.Unmarshal(raw, &c.body)
		c.mu.Unlock()
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

func sampleMessage() domain.Message {
	return domain.Message{
		Event: sharednotify.EventBackupFailed, Severity: sharednotify.SeverityCritical,
		Title: "백업 실패 <platform>", Summary: "목적지에 쓸 수 없습니다 & 재시도 필요",
		Fields:     []sharednotify.Field{{Name: "실행", Value: "bkp_1"}, {Name: "모드", Value: "full"}},
		OccurredAt: time.Date(2026, 10, 13, 1, 2, 3, 0, time.UTC),
	}
}

// ── Slack

func TestSlackSender_PostsEscapedMrkdwnText(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, "ok", nil)
	ch := &domain.Channel{Name: "ops", Type: domain.ChannelSlack, Secret: srv.URL + "/services/T/B/X"}

	require.NoError(t, NewSlackSender(nil).Send(context.Background(), ch, sampleMessage()))

	assert.Equal(t, "/services/T/B/X", got.path)
	assert.Equal(t, "application/json", got.headers.Get("Content-Type"))
	text, _ := got.body["text"].(string)
	assert.Contains(t, text, "*백업 실패 &lt;platform&gt;*", "Slack 의 & < > 는 이스케이프해야 한다")
	assert.Contains(t, text, "목적지에 쓸 수 없습니다 &amp; 재시도 필요")
	assert.Contains(t, text, "*실행*: bkp_1")
	assert.Contains(t, text, "🔴")
}

func TestSlackSender_ClassifiesFailuresWithoutLeakingURL(t *testing.T) {
	cases := []struct {
		status    int
		body      string
		headers   map[string]string
		retryable bool
		after     time.Duration
	}{
		{http.StatusNotFound, "no_service", nil, false, 0},
		{http.StatusBadRequest, "invalid_payload", nil, false, 0},
		{http.StatusTooManyRequests, "rate_limited", map[string]string{"Retry-After": "7"}, true, 7 * time.Second},
		{http.StatusInternalServerError, "rollup_error", nil, true, 0},
	}
	for _, tc := range cases {
		srv, _ := captureServer(t, tc.status, tc.body, tc.headers)
		ch := &domain.Channel{Type: domain.ChannelSlack, Secret: srv.URL + "/services/T/B/SECRETPART"}
		err := NewSlackSender(nil).Send(context.Background(), ch, sampleMessage())
		require.Error(t, err, tc.body)
		se, ok := port.AsSendError(err)
		require.True(t, ok)
		assert.Equal(t, tc.retryable, se.Retryable, tc.body)
		assert.Equal(t, tc.after, se.RetryAfter, tc.body)
		assert.Contains(t, err.Error(), tc.body)
		assert.NotContains(t, err.Error(), "SECRETPART", "웹훅 URL 은 자격증명이다")
	}
}

func TestSlackSender_NetworkErrorIsRetryableAndRedacted(t *testing.T) {
	ch := &domain.Channel{Type: domain.ChannelSlack, Secret: "http://127.0.0.1:1/services/T/B/SECRETPART"}
	err := NewSlackSender(nil).Send(context.Background(), ch, sampleMessage())
	require.Error(t, err)
	se, ok := port.AsSendError(err)
	require.True(t, ok)
	assert.True(t, se.Retryable)
	assert.NotContains(t, err.Error(), "SECRETPART")
}

// ── Telegram

func TestTelegramSender_SendsHTMLMessageToChat(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, `{"ok":true,"result":{"message_id":1}}`, nil)
	ch := &domain.Channel{Type: domain.ChannelTelegram, Secret: testToken, Settings: domain.Settings{ChatID: "-100123"}}

	require.NoError(t, NewTelegramSender(srv.URL, nil).Send(context.Background(), ch, sampleMessage()))

	assert.Equal(t, "/bot"+testToken+"/sendMessage", got.path)
	assert.Equal(t, "-100123", got.body["chat_id"])
	assert.Equal(t, "HTML", got.body["parse_mode"])
	assert.Equal(t, map[string]any{"is_disabled": true}, got.body["link_preview_options"])
	text, _ := got.body["text"].(string)
	assert.Contains(t, text, "<b>백업 실패 &lt;platform&gt;</b>")
	assert.Contains(t, text, "목적지에 쓸 수 없습니다 &amp; 재시도 필요")
	assert.Contains(t, text, "<b>실행</b>: bkp_1")
}

func TestTelegramSender_DefaultsToOfficialAPI(t *testing.T) {
	assert.Equal(t, "https://api.telegram.org", NewTelegramSender("", nil).baseURL)
	assert.Equal(t, "http://proxy.local:8081", NewTelegramSender("http://proxy.local:8081/", nil).baseURL)
}

func TestTelegramSender_ClassifiesAPIErrorsWithoutLeakingToken(t *testing.T) {
	cases := []struct {
		status    int
		body      string
		want      string
		retryable bool
		after     time.Duration
	}{
		{http.StatusBadRequest, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`, "chat not found", false, 0},
		{http.StatusUnauthorized, `{"ok":false,"error_code":401,"description":"Unauthorized"}`, "Unauthorized", false, 0},
		{http.StatusTooManyRequests, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 5","parameters":{"retry_after":5}}`, "Too Many Requests", true, 5 * time.Second},
		{http.StatusBadGateway, `bad gateway`, "502", true, 0},
	}
	for _, tc := range cases {
		srv, _ := captureServer(t, tc.status, tc.body, nil)
		ch := &domain.Channel{Type: domain.ChannelTelegram, Secret: testToken, Settings: domain.Settings{ChatID: "1"}}
		err := NewTelegramSender(srv.URL, nil).Send(context.Background(), ch, sampleMessage())
		require.Error(t, err, tc.want)
		se, ok := port.AsSendError(err)
		require.True(t, ok)
		assert.Equal(t, tc.retryable, se.Retryable, tc.want)
		assert.Equal(t, tc.after, se.RetryAfter, tc.want)
		assert.Contains(t, err.Error(), tc.want)
		assert.NotContains(t, err.Error(), "AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0", "봇 토큰은 URL 경로에 있다 — 오류에 실리면 안 된다")
	}
}

func TestTelegramSender_NetworkErrorRedactsToken(t *testing.T) {
	ch := &domain.Channel{Type: domain.ChannelTelegram, Secret: testToken, Settings: domain.Settings{ChatID: "1"}}
	err := NewTelegramSender("http://127.0.0.1:1", nil).Send(context.Background(), ch, sampleMessage())
	require.Error(t, err)
	se, _ := port.AsSendError(err)
	require.NotNil(t, se)
	assert.True(t, se.Retryable)
	assert.NotContains(t, err.Error(), "AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0")
}

// Telegram 은 한 메시지 4096자까지다. 긴 오류 문구가 와도 잘라서 보낸다.
func TestTelegramSender_TruncatesLongText(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, `{"ok":true}`, nil)
	ch := &domain.Channel{Type: domain.ChannelTelegram, Secret: testToken, Settings: domain.Settings{ChatID: "1"}}
	msg := sampleMessage()
	msg.Summary = strings.Repeat("가", 6000)

	require.NoError(t, NewTelegramSender(srv.URL, nil).Send(context.Background(), ch, msg))
	text, _ := got.body["text"].(string)
	assert.LessOrEqual(t, len([]rune(text)), 4096)
	assert.Contains(t, text, "…")
}

// 필드가 아주 많아도 4096자를 넘지 않고, 요약이 비어 있어도 무한히 돌지 않는다.
func TestTelegramText_FitsEvenWithManyFields(t *testing.T) {
	msg := sampleMessage()
	msg.Summary = ""
	for i := 0; i < 40; i++ {
		msg.Fields = append(msg.Fields, sharednotify.Field{Name: "필드", Value: strings.Repeat("나", 400)})
	}
	text := telegramText(msg)
	assert.LessOrEqual(t, len([]rune(text)), 4096)
	assert.Contains(t, text, "<b>백업 실패 &lt;platform&gt;</b>")
}
