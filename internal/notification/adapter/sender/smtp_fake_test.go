package sender

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP 는 테스트용 SMTP 서버다. 받은 봉투와 본문, 인증 정보를 적는다.
type fakeSMTP struct {
	t        *testing.T
	ln       net.Listener
	tlsCfg   *tls.Config
	certPool *x509.CertPool

	// 설정
	implicitTLS  bool     // 465 처럼 처음부터 TLS
	startTLS     bool     // STARTTLS 를 광고한다
	authMechs    []string // 광고할 AUTH 방식(TLS 뒤에만)
	user, pass   string
	rejectRcpt   string // 이 주소면 550
	tempFailRcpt string // 이 주소면 451

	mu       sync.Mutex
	from     string
	rcpts    []string
	data     string
	authUser string
	usedTLS  bool
}

func newFakeSMTP(t *testing.T, configure func(*fakeSMTP)) *fakeSMTP {
	t.Helper()
	s := &fakeSMTP{t: t, startTLS: true, authMechs: []string{"PLAIN", "LOGIN"}, user: "nullus", pass: "secret"}
	s.tlsCfg, s.certPool = selfSignedTLS(t)
	if configure != nil {
		configure(s)
	}
	var err error
	if s.implicitTLS {
		s.ln, err = tls.Listen("tcp", "127.0.0.1:0", s.tlsCfg)
	} else {
		s.ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.ln.Close() })
	go s.serve()
	return s
}

func (s *fakeSMTP) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *fakeSMTP) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeSMTP) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	_, isTLS := conn.(*tls.Conn)
	r := bufio.NewReader(conn)
	w := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	w("220 fake.smtp ESMTP")
	authed := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			lines := []string{"250-fake.smtp"}
			if s.startTLS && !isTLS {
				lines = append(lines, "250-STARTTLS")
			}
			if isTLS && len(s.authMechs) > 0 {
				lines = append(lines, "250-AUTH "+strings.Join(s.authMechs, " "))
			}
			lines = append(lines, "250 8BITMIME")
			for _, l := range lines {
				w(l)
			}
		case cmd == "STARTTLS":
			w("220 go ahead")
			tlsConn := tls.Server(conn, s.tlsCfg)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn, isTLS = tlsConn, true
			r = bufio.NewReader(conn)
		case strings.HasPrefix(cmd, "AUTH PLAIN"):
			raw := strings.TrimSpace(line[len("AUTH PLAIN"):])
			if raw == "" {
				w("334 ")
				raw, _ = r.ReadString('\n')
				raw = strings.TrimSpace(raw)
			}
			dec, _ := base64.StdEncoding.DecodeString(raw)
			parts := strings.Split(string(dec), "\x00")
			if len(parts) == 3 && parts[1] == s.user && parts[2] == s.pass {
				authed = true
				s.setAuth(parts[1], isTLS)
				w("235 ok")
			} else {
				w("535 5.7.8 authentication failed")
			}
		case cmd == "AUTH LOGIN":
			w("334 " + base64.StdEncoding.EncodeToString([]byte("Username:")))
			u, _ := r.ReadString('\n')
			w("334 " + base64.StdEncoding.EncodeToString([]byte("Password:")))
			p, _ := r.ReadString('\n')
			user, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(u))
			pass, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(p))
			if string(user) == s.user && string(pass) == s.pass {
				authed = true
				s.setAuth(string(user), isTLS)
				w("235 ok")
			} else {
				w("535 5.7.8 authentication failed")
			}
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			if len(s.authMechs) > 0 && s.user != "" && !authed {
				w("530 5.7.0 authentication required")
				continue
			}
			s.mu.Lock()
			s.from = envelopeAddress(line[len("MAIL FROM:"):])
			s.usedTLS = isTLS
			s.mu.Unlock()
			w("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			rcpt := envelopeAddress(line[len("RCPT TO:"):])
			switch rcpt {
			case s.rejectRcpt:
				w("550 5.1.1 no such user")
			case s.tempFailRcpt:
				w("451 4.3.0 try again later")
			default:
				s.mu.Lock()
				s.rcpts = append(s.rcpts, rcpt)
				s.mu.Unlock()
				w("250 ok")
			}
		case cmd == "DATA":
			w("354 end with .")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			s.mu.Lock()
			s.data = b.String()
			s.mu.Unlock()
			w("250 queued")
		case cmd == "QUIT":
			w("221 bye")
			return
		case cmd == "RSET", cmd == "NOOP":
			w("250 ok")
		default:
			w("502 unknown")
		}
	}
}

func (s *fakeSMTP) setAuth(user string, tls bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authUser = user
	s.usedTLS = tls
}

func (s *fakeSMTP) snapshot() (from string, rcpts []string, data, authUser string, usedTLS bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.from, append([]string(nil), s.rcpts...), s.data, s.authUser, s.usedTLS
}

func selfSignedTLS(t *testing.T) (*tls.Config, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fake.smtp"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}, pool
}

// envelopeAddress 는 "<주소> BODY=8BITMIME" 같은 인자에서 주소만 꺼낸다.
func envelopeAddress(arg string) string {
	arg = strings.TrimSpace(arg)
	if i := strings.Index(arg, ">"); i >= 0 {
		arg = arg[:i]
	}
	return strings.TrimPrefix(arg, "<")
}
