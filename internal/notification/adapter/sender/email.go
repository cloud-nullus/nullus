package sender

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
)

// EmailSender 는 SMTP 로 보낸다. 서버 · 보안 방식 · 계정은 채널 설정, 비밀번호가 Secret 이다.
//
//   - starttls: 평문으로 붙어 STARTTLS 로 올린다. 서버가 STARTTLS 를 광고하지 않으면 **보내지 않는다**
//     (평문으로 내려가면 비밀번호와 본문이 그대로 흐른다).
//   - tls: 처음부터 TLS 로 붙는다(465).
//   - none: 암호화하지 않는다. 도메인 검증이 인증과 함께 쓰지 못하게 막는다(사내 릴레이 전용).
//
// 인증은 PLAIN 을 먼저, 없으면 LOGIN 을 쓴다(Office 365 는 LOGIN 만 연다).
type EmailSender struct {
	// rootCAs 는 서버 인증서를 검증할 CA 다. nil 이면 시스템 CA 다(테스트가 바꾼다).
	rootCAs *x509.CertPool
	helo    string
}

func NewEmailSender() *EmailSender {
	helo := "localhost"
	if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
		helo = h
	}
	return &EmailSender{helo: helo}
}

var _ port.Sender = (*EmailSender)(nil)

func (s *EmailSender) Send(ctx context.Context, ch *domain.Channel, msg domain.Message) error {
	st := ch.Settings
	from, err := mail.ParseAddress(st.From)
	if err != nil {
		return &port.SendError{Err: fmt.Errorf("보내는 주소가 올바르지 않습니다: %s", st.From)}
	}
	raw, err := buildEmail(from, st.To, ch, msg)
	if err != nil {
		return &port.SendError{Err: err}
	}

	client, err := s.connect(ctx, st)
	if err != nil {
		return err
	}
	defer client.Close()

	if st.Username != "" {
		auth, err := chooseAuth(client, st, ch.Secret)
		if err != nil {
			return err
		}
		if err := client.Auth(auth); err != nil {
			return smtpFailure("인증", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return smtpFailure("발신자", err)
	}
	for _, to := range st.To {
		addr, err := mail.ParseAddress(to)
		if err != nil {
			return &port.SendError{Err: fmt.Errorf("받는 주소가 올바르지 않습니다: %s", to)}
		}
		if err := client.Rcpt(addr.Address); err != nil {
			return smtpFailure("받는 주소 "+addr.Address, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return smtpFailure("본문", err)
	}
	if _, err := w.Write(raw); err != nil {
		return smtpFailure("본문", err)
	}
	if err := w.Close(); err != nil {
		return smtpFailure("본문", err)
	}
	// QUIT 실패는 이미 접수된 메일에 영향이 없다.
	_ = client.Quit()
	return nil
}

func (s *EmailSender) connect(ctx context.Context, st domain.Settings) (*smtp.Client, error) {
	addr := net.JoinHostPort(st.SMTPHost, strconv.Itoa(st.SMTPPort))
	tlsCfg := &tls.Config{ServerName: st.SMTPHost, MinVersion: tls.VersionTLS12, RootCAs: s.rootCAs}
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	var conn net.Conn
	var err error
	if st.SMTPSecurity == domain.SMTPTLS {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, smtpFailure("연결", err)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}
	_ = conn.SetDeadline(deadline)

	client, err := smtp.NewClient(conn, st.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return nil, smtpFailure("인사", err)
	}
	if err := client.Hello(s.helo); err != nil {
		_ = client.Close()
		return nil, smtpFailure("인사", err)
	}
	if st.SMTPSecurity == domain.SMTPStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			return nil, &port.SendError{Err: errors.New("서버가 STARTTLS 를 지원하지 않습니다 — 보안 방식을 tls 로 바꾸거나 서버 설정을 확인하세요(평문으로 내려가지 않습니다)")}
		}
		if err := client.StartTLS(tlsCfg); err != nil {
			_ = client.Close()
			return nil, smtpFailure("STARTTLS", err)
		}
	}
	return client, nil
}

func chooseAuth(client *smtp.Client, st domain.Settings, password string) (smtp.Auth, error) {
	_, mechs := client.Extension("AUTH")
	upper := " " + strings.ToUpper(mechs) + " "
	switch {
	case strings.Contains(upper, " PLAIN "):
		return smtp.PlainAuth("", st.Username, password, st.SMTPHost), nil
	case strings.Contains(upper, " LOGIN "):
		return &loginAuth{username: st.Username, password: password}, nil
	default:
		return nil, &port.SendError{Err: fmt.Errorf("서버가 PLAIN · LOGIN 인증을 열지 않습니다(광고: %q)", strings.TrimSpace(mechs))}
	}
}

// loginAuth 는 AUTH LOGIN 이다. net/smtp 의 PlainAuth 와 같이 TLS 위에서만 쓴다.
type loginAuth struct {
	username, password string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS {
		return "", nil, errors.New("암호화하지 않은 연결로는 인증하지 않습니다")
	}
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	prompt := strings.ToLower(string(fromServer))
	switch {
	case strings.Contains(prompt, "username"):
		return []byte(a.username), nil
	case strings.Contains(prompt, "password"):
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("알 수 없는 AUTH LOGIN 요청: %q", string(fromServer))
	}
}

// smtpFailure 는 SMTP 단계의 실패를 분류한다. 4xx 는 잠시 뒤 다시 보낸다(서버가 그렇게 말한다).
// 5xx · 인증서 문제는 다시 보내도 같다. 연결 실패 · 시간 초과는 다시 보낸다.
func smtpFailure(stage string, err error) error {
	var tpErr *textproto.Error
	if errors.As(err, &tpErr) {
		return &port.SendError{
			Err:       fmt.Errorf("SMTP %s 실패: %d %s", stage, tpErr.Code, tpErr.Msg),
			Retryable: tpErr.Code >= 400 && tpErr.Code < 500,
		}
	}
	if isCertificateError(err) {
		return &port.SendError{Err: fmt.Errorf("SMTP %s 실패 — 서버 인증서를 믿을 수 없습니다: %v", stage, err)}
	}
	return &port.SendError{Err: fmt.Errorf("SMTP %s 실패: %v", stage, err), Retryable: true}
}

// buildEmail 은 UTF-8 평문 메일이다. 헤더는 RFC 2047 로, 본문은 quoted-printable 로 싣는다.
func buildEmail(from *mail.Address, to []string, ch *domain.Channel, msg domain.Message) ([]byte, error) {
	subject := fmt.Sprintf("[Nullus][%s] %s", severityLabel(msg.Severity), oneLine(msg.Title))
	var h bytes.Buffer
	writeHeader := func(k, v string) { fmt.Fprintf(&h, "%s: %s\r\n", k, v) }
	writeHeader("From", from.String())
	writeHeader("To", strings.Join(to, ", "))
	writeHeader("Subject", mime.QEncoding.Encode("UTF-8", subject))
	writeHeader("Date", time.Now().Format(time.RFC1123Z))
	writeHeader("Message-ID", messageID(from.Address))
	writeHeader("MIME-Version", "1.0")
	writeHeader("Content-Type", "text/plain; charset=UTF-8")
	writeHeader("Content-Transfer-Encoding", "quoted-printable")
	// 자동 발송임을 알린다(RFC 3834) — 부재 중 자동 응답이 되돌아오지 않게.
	writeHeader("Auto-Submitted", "auto-generated")
	h.WriteString("\r\n")

	qp := quotedprintable.NewWriter(&h)
	if _, err := qp.Write([]byte(emailBody(ch, msg))); err != nil {
		return nil, fmt.Errorf("메일 본문 만들기: %w", err)
	}
	if err := qp.Close(); err != nil {
		return nil, fmt.Errorf("메일 본문 만들기: %w", err)
	}
	return h.Bytes(), nil
}

func emailBody(ch *domain.Channel, msg domain.Message) string {
	summary, fields := fieldsOf(msg)
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s\n\n", severityLabel(msg.Severity), msg.Title)
	if summary != "" {
		b.WriteString(summary)
		b.WriteString("\n\n")
	}
	for _, f := range fields {
		fmt.Fprintf(&b, "%s: %s\n", f.Name, f.Value)
	}
	fmt.Fprintf(&b, "\n발생 시각: %s\n", formatTime(msg.OccurredAt))
	b.WriteString("\n-- \n")
	fmt.Fprintf(&b, "Nullus 알림 — '%s' 채널이 구독한 이벤트입니다. 구독은 관리 > 알림 채널에서 바꿉니다.\n", ch.Name)
	return b.String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.NewReplacer("\r", " ", "\n", " ").Replace(s)), " ")
}

func messageID(fromAddress string) string {
	domainPart := "nullus.local"
	if _, d, ok := strings.Cut(fromAddress, "@"); ok && d != "" {
		domainPart = d
	}
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("<%s.%d@%s>", hex.EncodeToString(buf), time.Now().Unix(), domainPart)
}
