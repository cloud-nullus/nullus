// Package domain 은 알림 채널과 보낼 메시지다(nullus-plan#63, 설계 docs/11_기능설계/Nullus_알림_채널_설계.md).
package domain

import (
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// ChannelType 은 알림을 보내는 길이다.
type ChannelType string

const (
	ChannelTelegram ChannelType = "telegram"
	// ChannelEmail 은 SMTP 로 보내는 메일이다. 이름은 기존 알림 규칙·설정(email)과 맞춘다.
	ChannelEmail ChannelType = "email"
	ChannelSlack ChannelType = "slack"
)

// ChannelTypes 는 지원하는 채널 종류다.
func ChannelTypes() []ChannelType {
	return []ChannelType{ChannelTelegram, ChannelEmail, ChannelSlack}
}

// SMTPSecurity 는 SMTP 연결 보호 방식이다.
type SMTPSecurity string

const (
	// SMTPStartTLS 는 평문으로 붙어 STARTTLS 로 올린다(587). 서버가 지원하지 않으면 보내지 않는다.
	SMTPStartTLS SMTPSecurity = "starttls"
	// SMTPTLS 는 처음부터 TLS 로 붙는다(465).
	SMTPTLS SMTPSecurity = "tls"
	// SMTPNone 은 암호화하지 않는다 — 인증 없는 사내 릴레이 전용이다.
	SMTPNone SMTPSecurity = "none"
)

const (
	maxNameLength  = 100
	maxRecipients  = 20
	defaultSMTPTLS = 465
)

var (
	// 봇 토큰은 "<봇 ID>:<35자 안팎>" 이다(BotFather 발급 형식).
	telegramTokenPattern = regexp.MustCompile(`^[0-9]{5,15}:[A-Za-z0-9_-]{30,}$`)
	// chat_id 는 숫자(그룹·슈퍼그룹은 음수) 또는 공개 채널의 @이름이다.
	telegramChatPattern = regexp.MustCompile(`^(-?[0-9]{1,20}|@[A-Za-z][A-Za-z0-9_]{4,31})$`)
)

// Settings 는 비밀이 아닌 채널 설정이다. 채널 종류에 맞는 칸만 남는다(Validate 가 나머지를 지운다).
type Settings struct {
	// Telegram
	ChatID string `json:"chat_id,omitempty"`
	// Email(SMTP)
	SMTPHost     string       `json:"smtp_host,omitempty"`
	SMTPPort     int          `json:"smtp_port,omitempty"`
	SMTPSecurity SMTPSecurity `json:"smtp_security,omitempty"`
	Username     string       `json:"username,omitempty"`
	From         string       `json:"from,omitempty"`
	To           []string     `json:"to,omitempty"`
}

// Channel 은 조직의 알림 채널 하나다.
//
// Secret 은 채널마다 하나다 — Telegram 봇 토큰, Slack 웹훅 URL, SMTP 비밀번호. 저장소가 암호화해
// 두고, API 는 내보내지 않는다(SecretHint 만).
type Channel struct {
	ID        string
	OrgID     string
	Name      string
	Type      ChannelType
	Settings  Settings
	Secret    string
	Events    []sharednotify.EventType
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
	// SecretUnreadable 은 저장된 비밀을 풀지 못했다는 뜻이다(ENCRYPTION_KEY 교체 등). 저장하지 않는
	// 상태다 — 저장소가 읽을 때 정한다. 이 채널은 보내지 못하고, 비밀을 다시 입력해야 고쳐진다.
	SecretUnreadable bool
}

// Validate 는 채널을 저장할 수 있는지 보고, 정규화한다(이벤트 중복 제거, 기본 포트, 다른 종류 설정 제거).
func (c *Channel) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return invalid("name", "이름을 입력하세요")
	}
	if utf8.RuneCountInString(c.Name) > maxNameLength {
		return invalid("name", "이름은 %d자까지입니다", maxNameLength)
	}
	if err := c.validateEvents(); err != nil {
		return err
	}
	c.Secret = strings.TrimSpace(c.Secret)
	switch c.Type {
	case ChannelTelegram:
		return c.validateTelegram()
	case ChannelSlack:
		return c.validateSlack()
	case ChannelEmail:
		return c.validateEmail()
	default:
		return invalid("type", "채널 종류는 telegram · email · slack 중 하나입니다")
	}
}

func (c *Channel) validateEvents() error {
	if len(c.Events) == 0 {
		return invalid("events", "받을 이벤트를 하나 이상 고르세요")
	}
	seen := make(map[sharednotify.EventType]bool, len(c.Events))
	events := make([]sharednotify.EventType, 0, len(c.Events))
	for _, e := range c.Events {
		if !IsKnownEvent(e) {
			return invalid("events", "알 수 없는 이벤트입니다: %s", e)
		}
		if !seen[e] {
			seen[e] = true
			events = append(events, e)
		}
	}
	c.Events = events
	return nil
}

func (c *Channel) validateTelegram() error {
	chatID := strings.TrimSpace(c.Settings.ChatID)
	c.Settings = Settings{ChatID: chatID}
	if c.Secret == "" {
		return invalid("secret", "봇 토큰을 입력하세요(BotFather 가 발급한 <숫자>:<문자열>)")
	}
	if !telegramTokenPattern.MatchString(c.Secret) {
		return invalid("secret", "봇 토큰 형식이 아닙니다 — <숫자>:<문자열> 모양이어야 합니다")
	}
	if chatID == "" {
		return invalid("chat_id", "chat_id 를 입력하세요")
	}
	if !telegramChatPattern.MatchString(chatID) {
		return invalid("chat_id", "chat_id 는 숫자(그룹은 음수) 또는 공개 채널의 @이름입니다")
	}
	return nil
}

func (c *Channel) validateSlack() error {
	c.Settings = Settings{}
	if c.Secret == "" {
		return invalid("secret", "Incoming Webhook URL 을 입력하세요")
	}
	if err := validateWebhookURL(c.Secret); err != nil {
		return err
	}
	return nil
}

// validateWebhookURL 은 웹훅 URL 을 본다. URL 자체가 자격증명이라 HTTPS 만 받되, 같은 호스트의
// 수신기(로컬 검증·사이드카 프록시)는 예외로 둔다.
func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return invalid("secret", "웹훅 URL 이 아닙니다 — https:// 로 시작해야 합니다")
	}
	if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
		return invalid("secret", "웹훅 URL 은 https 여야 합니다 — URL 자체가 자격증명이라 평문으로 보내지 않습니다")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c *Channel) validateEmail() error {
	s := c.Settings
	s.ChatID = ""
	s.SMTPHost = strings.TrimSpace(s.SMTPHost)
	s.Username = strings.TrimSpace(s.Username)
	s.From = strings.TrimSpace(s.From)
	if s.SMTPSecurity == "" {
		s.SMTPSecurity = SMTPStartTLS
	}
	recipients := make([]string, 0, len(s.To))
	for _, to := range s.To {
		if to = strings.TrimSpace(to); to != "" {
			recipients = append(recipients, to)
		}
	}
	s.To = recipients
	c.Settings = s

	if s.SMTPHost == "" {
		return invalid("smtp_host", "SMTP 서버 주소를 입력하세요")
	}
	switch s.SMTPSecurity {
	case SMTPStartTLS, SMTPTLS, SMTPNone:
	default:
		return invalid("smtp_security", "보안 방식은 starttls · tls · none 중 하나입니다")
	}
	if s.SMTPPort == 0 {
		c.Settings.SMTPPort = defaultSMTPPort(s.SMTPSecurity)
	} else if s.SMTPPort < 1 || s.SMTPPort > 65535 {
		return invalid("smtp_port", "포트는 1~65535 입니다")
	}
	// mail.ParseAddress 는 주석 안의 CR/LF 를 받아들인다 — 헤더 끼워 넣기를 여기서 막는다.
	if _, err := mail.ParseAddress(s.From); err != nil || hasLineBreak(s.From) {
		return invalid("from", "보내는 주소가 메일 주소가 아닙니다")
	}
	if len(s.To) == 0 {
		return invalid("to", "받는 주소를 하나 이상 입력하세요")
	}
	if len(s.To) > maxRecipients {
		return invalid("to", "받는 주소는 %d개까지입니다", maxRecipients)
	}
	for _, to := range s.To {
		if _, err := mail.ParseAddress(to); err != nil || hasLineBreak(to) {
			return invalid("to", "메일 주소가 아닙니다: %s", strings.NewReplacer("\r", " ", "\n", " ").Replace(to))
		}
	}
	if s.Username != "" {
		if s.SMTPSecurity == SMTPNone {
			return invalid("smtp_security", "인증을 쓰면 starttls 나 tls 를 고르세요 — 비밀번호를 평문으로 보내지 않습니다")
		}
		if c.Secret == "" {
			return invalid("secret", "SMTP 비밀번호를 입력하세요")
		}
	} else {
		// 인증하지 않으면 비밀번호는 쓰이지 않는다 — 저장하지 않는다.
		c.Secret = ""
	}
	return nil
}

func hasLineBreak(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

func defaultSMTPPort(security SMTPSecurity) int {
	switch security {
	case SMTPTLS:
		return defaultSMTPTLS
	case SMTPNone:
		return 25
	default:
		return 587
	}
}

// SecretHint 는 화면이 "어떤 자격증명이 들어 있는지" 알아볼 만큼만 보여 준다. 비밀 자체는 담지 않는다.
func (c *Channel) SecretHint() string {
	if c.Secret == "" {
		return ""
	}
	switch c.Type {
	case ChannelTelegram:
		// 앞쪽 봇 ID 는 비밀이 아니다(봇 프로필에 보인다). 뒤쪽이 비밀이다.
		if id, _, ok := strings.Cut(c.Secret, ":"); ok {
			return "bot " + id
		}
	case ChannelSlack:
		if u, err := url.Parse(c.Secret); err == nil && len(c.Secret) > 4 {
			return u.Host + " …" + c.Secret[len(c.Secret)-4:]
		}
	}
	return ""
}

// Subscribes 는 이 채널이 그 이벤트를 받는지다.
func (c *Channel) Subscribes(e sharednotify.EventType) bool {
	if !c.Active {
		return false
	}
	for _, sub := range c.Events {
		if sub == e {
			return true
		}
	}
	return false
}

// ChannelUpdate 는 바꿀 칸만 담는다. nil 은 그대로 둔다 — 특히 Secret 은 화면이 모르는 값이라
// 비워 보내면 기존 값을 지킨다.
type ChannelUpdate struct {
	Name     *string
	Settings *Settings
	Secret   *string
	Events   *[]sharednotify.EventType
	Active   *bool
}

// ApplyUpdate 는 갱신을 적용하고 검증한다. 실패하면 채널을 바꾸지 않는다. 채널 종류는 바꿀 수 없다.
func (c *Channel) ApplyUpdate(u ChannelUpdate) error {
	next := *c
	next.Settings.To = append([]string(nil), c.Settings.To...)
	next.Events = append([]sharednotify.EventType(nil), c.Events...)
	if u.Name != nil {
		next.Name = *u.Name
	}
	if u.Settings != nil {
		next.Settings = *u.Settings
	}
	secretGiven := u.Secret != nil && strings.TrimSpace(*u.Secret) != ""
	if secretGiven {
		next.Secret = *u.Secret
		next.SecretUnreadable = false
	}
	if u.Events != nil {
		next.Events = *u.Events
	}
	if u.Active != nil {
		next.Active = *u.Active
	}
	if err := next.Validate(); err != nil {
		return err
	}
	// 저장된 SMTP 비밀번호는 그 서버로만 간다. 서버·포트·보안·계정을 바꾸면서 비밀번호를 그대로 두면,
	// 비밀번호를 볼 수 없는 사람도 smtp_host 를 자기 서버로 돌려 테스트 전송 한 번에 빼낼 수 있다.
	if c.Type == ChannelEmail && next.Settings.Username != "" && !secretGiven && smtpEndpointChanged(c.Settings, next.Settings) {
		return invalid("secret", "SMTP 서버나 계정을 바꾸면 비밀번호를 다시 입력하세요")
	}
	*c = next
	return nil
}

func smtpEndpointChanged(before, after Settings) bool {
	return before.SMTPHost != after.SMTPHost || before.SMTPPort != after.SMTPPort ||
		before.SMTPSecurity != after.SMTPSecurity || before.Username != after.Username
}
