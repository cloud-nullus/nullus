package sender

import (
	"context"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
)

func emailChannelFor(s *fakeSMTP, security domain.SMTPSecurity) *domain.Channel {
	return &domain.Channel{
		Name: "운영 메일", Type: domain.ChannelEmail,
		Settings: domain.Settings{
			SMTPHost: "127.0.0.1", SMTPPort: s.port(), SMTPSecurity: security,
			Username: "nullus", From: "Nullus 알림 <nullus@example.com>",
			To: []string{"ops@example.com", "dba@example.com"},
		},
		Secret: "secret",
	}
}

func testEmailSender(s *fakeSMTP) *EmailSender {
	e := NewEmailSender()
	e.rootCAs = s.certPool
	return e
}

func decodeBody(t *testing.T, data string) (*mail.Message, string) {
	t.Helper()
	m, err := mail.ReadMessage(strings.NewReader(data))
	require.NoError(t, err)
	body, err := io.ReadAll(quotedprintable.NewReader(m.Body))
	require.NoError(t, err)
	return m, string(body)
}

func TestEmailSender_StartTLSWithAuthDeliversUTF8Mail(t *testing.T) {
	srv := newFakeSMTP(t, nil)
	ch := emailChannelFor(srv, domain.SMTPStartTLS)

	require.NoError(t, testEmailSender(srv).Send(context.Background(), ch, sampleMessage()))

	from, rcpts, data, authUser, usedTLS := srv.snapshot()
	assert.Equal(t, "nullus@example.com", from, "봉투 발신자는 주소만이다")
	assert.Equal(t, []string{"ops@example.com", "dba@example.com"}, rcpts)
	assert.Equal(t, "nullus", authUser)
	assert.True(t, usedTLS, "STARTTLS 로 올린 뒤에 인증하고 보낸다")

	m, body := decodeBody(t, data)
	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	require.NoError(t, err)
	assert.Equal(t, "[Nullus][심각] 백업 실패 <platform>", subject)
	assert.Equal(t, "text/plain; charset=UTF-8", m.Header.Get("Content-Type"))
	assert.Equal(t, "quoted-printable", m.Header.Get("Content-Transfer-Encoding"))
	assert.NotEmpty(t, m.Header.Get("Message-ID"))
	assert.NotEmpty(t, m.Header.Get("Date"))
	fromHeader, err := mail.ParseAddress(m.Header.Get("From"))
	require.NoError(t, err)
	assert.Equal(t, "Nullus 알림", fromHeader.Name)
	assert.Contains(t, m.Header.Get("To"), "ops@example.com")
	assert.Contains(t, body, "목적지에 쓸 수 없습니다 & 재시도 필요")
	assert.Contains(t, body, "실행: bkp_1")
}

func TestEmailSender_ImplicitTLS(t *testing.T) {
	srv := newFakeSMTP(t, func(s *fakeSMTP) { s.implicitTLS = true; s.startTLS = false })
	require.NoError(t, testEmailSender(srv).Send(context.Background(), emailChannelFor(srv, domain.SMTPTLS), sampleMessage()))
	_, rcpts, _, authUser, usedTLS := srv.snapshot()
	assert.Len(t, rcpts, 2)
	assert.Equal(t, "nullus", authUser)
	assert.True(t, usedTLS)
}

// Office 365 처럼 AUTH LOGIN 만 여는 서버가 있다.
func TestEmailSender_FallsBackToLoginAuth(t *testing.T) {
	srv := newFakeSMTP(t, func(s *fakeSMTP) { s.authMechs = []string{"LOGIN"} })
	require.NoError(t, testEmailSender(srv).Send(context.Background(), emailChannelFor(srv, domain.SMTPStartTLS), sampleMessage()))
	_, _, _, authUser, _ := srv.snapshot()
	assert.Equal(t, "nullus", authUser)
}

// 인증 없는 사내 릴레이.
func TestEmailSender_PlainRelayWithoutAuth(t *testing.T) {
	srv := newFakeSMTP(t, func(s *fakeSMTP) { s.startTLS = false; s.authMechs = nil; s.user = "" })
	ch := emailChannelFor(srv, domain.SMTPNone)
	ch.Settings.Username, ch.Secret = "", ""
	require.NoError(t, testEmailSender(srv).Send(context.Background(), ch, sampleMessage()))
	_, rcpts, _, _, usedTLS := srv.snapshot()
	assert.Len(t, rcpts, 2)
	assert.False(t, usedTLS)
}

// STARTTLS 를 고른 채널은 서버가 지원하지 않으면 평문으로 내려가지 않는다.
func TestEmailSender_RefusesDowngradeWhenStartTLSMissing(t *testing.T) {
	srv := newFakeSMTP(t, func(s *fakeSMTP) { s.startTLS = false })
	err := testEmailSender(srv).Send(context.Background(), emailChannelFor(srv, domain.SMTPStartTLS), sampleMessage())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STARTTLS")
	se, _ := port.AsSendError(err)
	require.NotNil(t, se)
	assert.False(t, se.Retryable)
	_, rcpts, _, _, _ := srv.snapshot()
	assert.Empty(t, rcpts)
}

func TestEmailSender_ClassifiesSMTPErrors(t *testing.T) {
	srv := newFakeSMTP(t, nil)
	ch := emailChannelFor(srv, domain.SMTPStartTLS)
	ch.Secret = "wrong"
	err := testEmailSender(srv).Send(context.Background(), ch, sampleMessage())
	require.Error(t, err)
	se, _ := port.AsSendError(err)
	require.NotNil(t, se)
	assert.False(t, se.Retryable, "인증 실패(535)는 다시 보내도 같다")
	assert.NotContains(t, err.Error(), "wrong", "비밀번호는 문구에 없다")

	srv = newFakeSMTP(t, func(s *fakeSMTP) { s.tempFailRcpt = "dba@example.com" })
	err = testEmailSender(srv).Send(context.Background(), emailChannelFor(srv, domain.SMTPStartTLS), sampleMessage())
	se, _ = port.AsSendError(err)
	require.NotNil(t, se)
	assert.True(t, se.Retryable, "4xx 는 잠시 뒤 다시 보낸다")

	srv = newFakeSMTP(t, func(s *fakeSMTP) { s.rejectRcpt = "dba@example.com" })
	err = testEmailSender(srv).Send(context.Background(), emailChannelFor(srv, domain.SMTPStartTLS), sampleMessage())
	se, _ = port.AsSendError(err)
	require.NotNil(t, se)
	assert.False(t, se.Retryable)
	assert.Contains(t, err.Error(), "dba@example.com")
}

// 신뢰할 수 없는 인증서는 다시 보내도 같다.
func TestEmailSender_UntrustedCertificateIsPermanent(t *testing.T) {
	srv := newFakeSMTP(t, nil)
	err := NewEmailSender().Send(context.Background(), emailChannelFor(srv, domain.SMTPStartTLS), sampleMessage())
	require.Error(t, err)
	se, _ := port.AsSendError(err)
	require.NotNil(t, se)
	assert.False(t, se.Retryable)
}

func TestEmailSender_UnreachableServerIsRetryable(t *testing.T) {
	ch := &domain.Channel{Type: domain.ChannelEmail, Settings: domain.Settings{
		SMTPHost: "127.0.0.1", SMTPPort: 1, SMTPSecurity: domain.SMTPNone,
		From: "n@example.com", To: []string{"o@example.com"},
	}}
	err := NewEmailSender().Send(context.Background(), ch, sampleMessage())
	se, _ := port.AsSendError(err)
	require.NotNil(t, se)
	assert.True(t, se.Retryable)
}

// 사용자 문구(채널 이름)로 헤더를 끼워 넣지 못한다.
func TestEmailSender_SubjectCannotInjectHeaders(t *testing.T) {
	srv := newFakeSMTP(t, nil)
	msg := sampleMessage()
	msg.Title = "제목\r\nBcc: evil@example.com"
	require.NoError(t, testEmailSender(srv).Send(context.Background(), emailChannelFor(srv, domain.SMTPStartTLS), msg))
	_, rcpts, data, _, _ := srv.snapshot()
	assert.Len(t, rcpts, 2)
	m, _ := decodeBody(t, data)
	assert.Empty(t, m.Header.Get("Bcc"))
}

// 검증을 거치지 않은 옛 행의 보안 방식이 비었거나 모르는 값이면 평문으로 보내지 않는다.
func TestEmailSender_UnknownSecurityIsRefused(t *testing.T) {
	srv := newFakeSMTP(t, func(s *fakeSMTP) { s.startTLS = false; s.authMechs = nil; s.user = "" })
	ch := emailChannelFor(srv, "")
	ch.Settings.Username, ch.Secret = "", ""
	err := testEmailSender(srv).Send(context.Background(), ch, sampleMessage())
	require.Error(t, err)
	_, rcpts, _, _, _ := srv.snapshot()
	assert.Empty(t, rcpts)
}

// To 헤더는 파싱한 주소로 쓴다 — 원래 문자열을 그대로 쓰면 주석 안의 CR/LF 로 헤더를 끼워 넣는다.
func TestEmailSender_ToHeaderUsesParsedAddresses(t *testing.T) {
	srv := newFakeSMTP(t, nil)
	ch := emailChannelFor(srv, domain.SMTPStartTLS)
	ch.Settings.To = []string{"운영팀 <ops@example.com>", "dba@example.com"}
	require.NoError(t, testEmailSender(srv).Send(context.Background(), ch, sampleMessage()))
	_, rcpts, data, _, _ := srv.snapshot()
	assert.Equal(t, []string{"ops@example.com", "dba@example.com"}, rcpts)
	m, _ := decodeBody(t, data)
	list, err := m.Header.AddressList("To")
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "운영팀", list[0].Name)
}
