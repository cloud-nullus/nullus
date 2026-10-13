package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
)

// SlackSender 는 Slack Incoming Webhook 으로 보낸다. 웹훅 URL 이 채널의 Secret 이다.
//
// 본문은 text 하나(mrkdwn)다 — 웹훅은 text 를 mrkdwn 으로 그리고, 블록 없이도 알림 미리보기에
// 같은 문구가 뜬다. Slack 호환 웹훅(Mattermost 등)도 text 를 받는다.
type SlackSender struct {
	client *http.Client
}

func NewSlackSender(client *http.Client) *SlackSender {
	return &SlackSender{client: httpClientOrDefault(client)}
}

var _ port.Sender = (*SlackSender)(nil)

var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func (s *SlackSender) Send(ctx context.Context, ch *domain.Channel, msg domain.Message) error {
	payload, err := json.Marshal(map[string]any{"text": slackText(msg)})
	if err != nil {
		return &port.SendError{Err: fmt.Errorf("slack 메시지 만들기: %w", err)}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ch.Secret, bytes.NewReader(payload))
	if err != nil {
		return &port.SendError{Err: fmt.Errorf("slack 웹훅 URL 이 올바르지 않습니다")}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return transportFailure("slack", err, ch.Secret)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	// Slack 은 실패 이유를 본문 한 단어로 준다(no_service · invalid_payload · channel_is_archived).
	return httpFailure(resp.StatusCode, redact(string(body), ch.Secret), parseRetryAfter(resp.Header.Get("Retry-After")))
}

func slackText(msg domain.Message) string {
	summary, fields := fieldsOf(msg)
	var b strings.Builder
	fmt.Fprintf(&b, "%s *%s*\n", severityEmoji(msg.Severity), slackEscaper.Replace(msg.Title))
	if summary != "" {
		b.WriteString(slackEscaper.Replace(summary))
		b.WriteString("\n")
	}
	if len(fields) > 0 {
		b.WriteString("\n")
		for _, f := range fields {
			fmt.Fprintf(&b, "*%s*: %s\n", slackEscaper.Replace(f.Name), slackEscaper.Replace(f.Value))
		}
	}
	fmt.Fprintf(&b, "\n_%s · Nullus_", formatTime(msg.OccurredAt))
	return b.String()
}
