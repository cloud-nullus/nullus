package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

const testTelegramToken = "123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0"

func telegramChannel() *Channel {
	return &Channel{
		OrgID: "org-1", Name: "운영팀 텔레그램", Type: ChannelTelegram,
		Settings: Settings{ChatID: "-1001234567890"},
		Secret:   testTelegramToken,
		Events:   []sharednotify.EventType{sharednotify.EventBackupFailed},
		Active:   true,
	}
}

func slackChannel() *Channel {
	return &Channel{
		OrgID: "org-1", Name: "slack #ops", Type: ChannelSlack,
		Secret: "https://hooks.slack.com/services/T000/B000/abcdefghijklmnopqrstWXYZ",
		Events: []sharednotify.EventType{sharednotify.EventBackupFailed},
		Active: true,
	}
}

func emailChannel() *Channel {
	return &Channel{
		OrgID: "org-1", Name: "운영 메일", Type: ChannelEmail,
		Settings: Settings{
			SMTPHost: "smtp.example.com", SMTPSecurity: SMTPStartTLS,
			Username: "nullus", From: "Nullus <nullus@example.com>",
			To: []string{"ops@example.com", "dba@example.com"},
		},
		Secret: "app-password",
		Events: []sharednotify.EventType{sharednotify.EventBackupFailed, sharednotify.EventBackupPartial},
		Active: true,
	}
}

func assertInvalid(t *testing.T, ch *Channel, field string) {
	t.Helper()
	err := ch.Validate()
	require.Error(t, err)
	var ve *ValidationError
	require.True(t, errors.As(err, &ve), "ValidationError 여야 한다: %v", err)
	assert.Equal(t, field, ve.Field, ve.Error())
}

func TestChannel_Validate_AcceptsEachChannelType(t *testing.T) {
	for _, ch := range []*Channel{telegramChannel(), slackChannel(), emailChannel()} {
		assert.NoError(t, ch.Validate(), ch.Name)
	}
}

func TestChannel_Validate_CommonFields(t *testing.T) {
	ch := telegramChannel()
	ch.Name = "  "
	assertInvalid(t, ch, "name")

	ch = telegramChannel()
	ch.Name = strings.Repeat("가", 101)
	assertInvalid(t, ch, "name")

	ch = telegramChannel()
	ch.Type = "webhook"
	assertInvalid(t, ch, "type")

	// 받을 이벤트가 없는 채널은 조용히 아무것도 받지 않는다 — 만들 때 막는다.
	ch = telegramChannel()
	ch.Events = nil
	assertInvalid(t, ch, "events")

	ch = telegramChannel()
	ch.Events = []sharednotify.EventType{"pipeline_failure"}
	assertInvalid(t, ch, "events")
}

func TestChannel_Validate_DeduplicatesEvents(t *testing.T) {
	ch := telegramChannel()
	ch.Events = []sharednotify.EventType{sharednotify.EventBackupFailed, sharednotify.EventBackupFailed}
	require.NoError(t, ch.Validate())
	assert.Equal(t, []sharednotify.EventType{sharednotify.EventBackupFailed}, ch.Events)
}

func TestChannel_Validate_Telegram(t *testing.T) {
	ch := telegramChannel()
	ch.Secret = ""
	assertInvalid(t, ch, "secret")

	ch = telegramChannel()
	ch.Secret = "not-a-token"
	assertInvalid(t, ch, "secret")

	ch = telegramChannel()
	ch.Settings.ChatID = ""
	assertInvalid(t, ch, "chat_id")

	ch = telegramChannel()
	ch.Settings.ChatID = "ops-room"
	assertInvalid(t, ch, "chat_id")

	// 공개 채널은 @이름으로도 보낼 수 있다.
	ch = telegramChannel()
	ch.Settings.ChatID = "@nullus_ops"
	assert.NoError(t, ch.Validate())
}

func TestChannel_Validate_Slack(t *testing.T) {
	ch := slackChannel()
	ch.Secret = ""
	assertInvalid(t, ch, "secret")

	ch = slackChannel()
	ch.Secret = "hooks.slack.com/services/x"
	assertInvalid(t, ch, "secret")

	// 웹훅 URL 자체가 자격증명이다 — 평문 HTTP 로 내보내지 않는다.
	ch = slackChannel()
	ch.Secret = "http://hooks.example.com/services/x"
	assertInvalid(t, ch, "secret")

	// 같은 호스트 안의 수신기(로컬 검증·사내 프록시)는 예외다.
	ch = slackChannel()
	ch.Secret = "http://127.0.0.1:18080/hook"
	assert.NoError(t, ch.Validate())
}

func TestChannel_Validate_Email(t *testing.T) {
	ch := emailChannel()
	ch.Settings.SMTPHost = ""
	assertInvalid(t, ch, "smtp_host")

	ch = emailChannel()
	ch.Settings.SMTPPort = 70000
	assertInvalid(t, ch, "smtp_port")

	ch = emailChannel()
	ch.Settings.SMTPSecurity = "ssl3"
	assertInvalid(t, ch, "smtp_security")

	ch = emailChannel()
	ch.Settings.From = "not an address"
	assertInvalid(t, ch, "from")

	ch = emailChannel()
	ch.Settings.To = nil
	assertInvalid(t, ch, "to")

	ch = emailChannel()
	ch.Settings.To = []string{"ops@example.com", "broken"}
	assertInvalid(t, ch, "to")

	ch = emailChannel()
	ch.Secret = ""
	assertInvalid(t, ch, "secret")

	// 암호화하지 않은 연결로 비밀번호를 보내지 않는다.
	ch = emailChannel()
	ch.Settings.SMTPSecurity = SMTPNone
	assertInvalid(t, ch, "smtp_security")

	// 인증 없는 사내 릴레이는 평문을 허용한다.
	ch = emailChannel()
	ch.Settings.SMTPSecurity = SMTPNone
	ch.Settings.Username = ""
	ch.Secret = ""
	assert.NoError(t, ch.Validate())
}

func TestChannel_Validate_EmailDefaultsPortBySecurity(t *testing.T) {
	cases := map[SMTPSecurity]int{SMTPStartTLS: 587, SMTPTLS: 465, SMTPNone: 25}
	for security, port := range cases {
		ch := emailChannel()
		ch.Settings.SMTPSecurity = security
		ch.Settings.SMTPPort = 0
		if security == SMTPNone {
			ch.Settings.Username, ch.Secret = "", ""
		}
		require.NoError(t, ch.Validate(), security)
		assert.Equal(t, port, ch.Settings.SMTPPort, security)
	}
	ch := emailChannel()
	ch.Settings.SMTPSecurity = ""
	require.NoError(t, ch.Validate())
	assert.Equal(t, SMTPStartTLS, ch.Settings.SMTPSecurity)
}

// 다른 채널 종류의 설정이 섞여 저장되지 않는다.
func TestChannel_Validate_DropsSettingsOfOtherTypes(t *testing.T) {
	ch := slackChannel()
	ch.Settings = Settings{ChatID: "123", SMTPHost: "smtp.example.com", To: []string{"a@example.com"}}
	require.NoError(t, ch.Validate())
	assert.Equal(t, Settings{}, ch.Settings)
}

func TestChannel_SecretHint_NeverRevealsSecret(t *testing.T) {
	assert.Equal(t, "bot 123456789", telegramChannel().SecretHint())
	assert.Equal(t, "hooks.slack.com …WXYZ", slackChannel().SecretHint())
	assert.Equal(t, "", emailChannel().SecretHint())

	empty := slackChannel()
	empty.Secret = ""
	assert.Equal(t, "", empty.SecretHint())
}

func TestChannel_Subscribes(t *testing.T) {
	ch := emailChannel()
	assert.True(t, ch.Subscribes(sharednotify.EventBackupPartial))
	assert.False(t, ch.Subscribes(sharednotify.EventBackupSucceeded))
	ch.Active = false
	assert.False(t, ch.Subscribes(sharednotify.EventBackupPartial), "꺼진 채널은 받지 않는다")
}

func TestChannel_ApplyUpdate_KeepsSecretWhenOmitted(t *testing.T) {
	ch := telegramChannel()
	name := "새 이름"
	require.NoError(t, ch.ApplyUpdate(ChannelUpdate{Name: &name}))
	assert.Equal(t, testTelegramToken, ch.Secret)
	assert.Equal(t, "새 이름", ch.Name)

	newToken := "987654321:BBHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0"
	require.NoError(t, ch.ApplyUpdate(ChannelUpdate{Secret: &newToken}))
	assert.Equal(t, newToken, ch.Secret)

	active := false
	require.NoError(t, ch.ApplyUpdate(ChannelUpdate{Active: &active}))
	assert.False(t, ch.Active)
}

func TestChannel_ApplyUpdate_RejectsInvalidResult(t *testing.T) {
	ch := telegramChannel()
	bad := "nope"
	err := ch.ApplyUpdate(ChannelUpdate{Secret: &bad})
	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
	assert.Equal(t, testTelegramToken, ch.Secret, "실패한 갱신은 채널을 바꾸지 않는다")
}

// 저장된 SMTP 비밀번호를 다른 서버로 보내게 만들 수 없다 — 서버·포트·보안·계정을 바꾸면 비밀번호를
// 다시 받는다(리뷰 지적: smtp_host 만 바꾸고 테스트 전송을 누르면 원래 비밀번호가 그 서버로 간다).
func TestChannel_ApplyUpdate_EmailServerChangeRequiresPasswordAgain(t *testing.T) {
	for name, mutate := range map[string]func(*Settings){
		"host":     func(s *Settings) { s.SMTPHost = "smtp.evil.example" },
		"port":     func(s *Settings) { s.SMTPPort = 2525 },
		"security": func(s *Settings) { s.SMTPSecurity = SMTPTLS },
		"username": func(s *Settings) { s.Username = "other" },
	} {
		ch := emailChannel()
		require.NoError(t, ch.Validate())
		settings := ch.Settings
		settings.To = append([]string(nil), ch.Settings.To...)
		mutate(&settings)

		err := ch.ApplyUpdate(ChannelUpdate{Settings: &settings})
		var ve *ValidationError
		require.True(t, errors.As(err, &ve), name)
		assert.Equal(t, "secret", ve.Field, name)

		pw := "new-password"
		assert.NoError(t, ch.ApplyUpdate(ChannelUpdate{Settings: &settings, Secret: &pw}), name)
	}

	// 받는 주소 · 보내는 주소만 바꾸는 것은 비밀번호가 가는 곳을 바꾸지 않는다.
	ch := emailChannel()
	require.NoError(t, ch.Validate())
	settings := ch.Settings
	settings.To = []string{"new@example.com"}
	assert.NoError(t, ch.ApplyUpdate(ChannelUpdate{Settings: &settings}))
}

// mail.ParseAddress 는 주석 안의 CR/LF 를 받아들인다 — 헤더를 끼워 넣지 못하게 막는다.
func TestChannel_Validate_RejectsLineBreaksInAddresses(t *testing.T) {
	ch := emailChannel()
	ch.Settings.To = []string{"a@b.com (x\r\nBcc: evil@x.com)"}
	assertInvalid(t, ch, "to")

	ch = emailChannel()
	ch.Settings.From = "n@example.com (x\nBcc: evil@x.com)"
	assertInvalid(t, ch, "from")
}

// 복호화할 수 없는 비밀(키 교체 등)은 다시 입력하게 한다.
func TestChannel_ApplyUpdate_UnreadableSecretMustBeReentered(t *testing.T) {
	ch := telegramChannel()
	ch.Secret = ""
	ch.SecretUnreadable = true
	name := "이름만"
	err := ch.ApplyUpdate(ChannelUpdate{Name: &name})
	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
	assert.Equal(t, "secret", ve.Field)

	token := testTelegramToken
	require.NoError(t, ch.ApplyUpdate(ChannelUpdate{Secret: &token}))
	assert.False(t, ch.SecretUnreadable)
}
