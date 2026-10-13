package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// DefaultTelegramAPIURL 은 Telegram Bot API 주소다. 사내 프록시나 Local Bot API Server 를 쓰면
// NULLUS_TELEGRAM_API_URL 로 바꾼다.
const DefaultTelegramAPIURL = "https://api.telegram.org"

// telegramMaxText 는 sendMessage 한 번의 최대 글자 수다(Bot API).
const telegramMaxText = 4096

// TelegramSender 는 Bot API 의 sendMessage 로 보낸다. 봇 토큰이 채널의 Secret, chat_id 가 설정이다.
type TelegramSender struct {
	baseURL string
	client  *http.Client
}

func NewTelegramSender(baseURL string, client *http.Client) *TelegramSender {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultTelegramAPIURL
	}
	return &TelegramSender{baseURL: baseURL, client: httpClientOrDefault(client)}
}

var _ port.Sender = (*TelegramSender)(nil)

type telegramResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func (s *TelegramSender) Send(ctx context.Context, ch *domain.Channel, msg domain.Message) error {
	payload, err := json.Marshal(map[string]any{
		"chat_id":              ch.Settings.ChatID,
		"text":                 telegramText(msg),
		"parse_mode":           "HTML",
		"link_preview_options": map[string]any{"is_disabled": true},
	})
	if err != nil {
		return &port.SendError{Err: fmt.Errorf("telegram 메시지 만들기: %w", err)}
	}
	endpoint := s.baseURL + "/bot" + ch.Secret + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return &port.SendError{Err: fmt.Errorf("telegram 요청 만들기: %s", redact(err.Error(), ch.Secret))}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return transportFailure("telegram", err, ch.Secret)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

	var parsed telegramResponse
	if jsonErr := json.Unmarshal(body, &parsed); jsonErr != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		return httpFailure(resp.StatusCode, redact(string(body), ch.Secret), 0)
	}
	if parsed.OK {
		return nil
	}
	status := parsed.ErrorCode
	if status == 0 {
		status = resp.StatusCode
	}
	// Bot API 는 실패 이유를 description 에 준다(chat not found · bot was blocked by the user ·
	// Unauthorized). 429 면 parameters.retry_after 초 뒤에 다시 보내라는 뜻이다.
	return httpFailure(status, redact(parsed.Description, ch.Secret),
		time.Duration(parsed.Parameters.RetryAfter)*time.Second)
}

// telegramText 는 HTML parse_mode 본문이다. 사용자 문구는 이스케이프한다. 4096자를 넘으면 태그
// 한가운데를 자르지 않게 요약을 줄이고, 그래도 넘으면 뒤쪽 필드를 뺀다.
func telegramText(msg domain.Message) string {
	summary, fields := fieldsOf(msg)
	text := renderTelegram(msg, summary, fields)
	for over := runeLen(text) - telegramMaxText; over > 0; over = runeLen(text) - telegramMaxText {
		if keep := runeLen(summary) - over - 1; keep > 0 {
			summary = truncate(summary, keep)
		} else if len(fields) > 0 {
			summary = truncate(summary, 1)
			fields = fields[:len(fields)-1]
		} else {
			return truncate(html.EscapeString(msg.Title), telegramMaxText)
		}
		text = renderTelegram(msg, summary, fields)
	}
	return text
}

func renderTelegram(msg domain.Message, summary string, fields []sharednotify.Field) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s <b>%s</b>\n", severityEmoji(msg.Severity), html.EscapeString(msg.Title))
	if summary != "" {
		b.WriteString(html.EscapeString(summary))
		b.WriteString("\n")
	}
	if len(fields) > 0 {
		b.WriteString("\n")
		for _, f := range fields {
			fmt.Fprintf(&b, "<b>%s</b>: %s\n", html.EscapeString(f.Name), html.EscapeString(f.Value))
		}
	}
	fmt.Fprintf(&b, "\n<i>%s · Nullus</i>", html.EscapeString(formatTime(msg.OccurredAt)))
	return b.String()
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }
