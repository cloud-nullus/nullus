package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// newHTTPClient 는 모델 API 를 부르는 클라이언트다. 주소는 관리자가 넣으므로 두 가지를 막는다.
//   - 리다이렉트를 따라가지 않는다 — 넣은 주소에서 내부의 다른 곳으로 튀지 않게.
//   - 링크로컬 · 멀티캐스트 · 미지정 주소로 연결하지 않는다 — 클라우드 메타데이터(169.254.169.254)를 찔러 보지
//     못하게. 이름을 푼 뒤의 주소로 보므로 DNS 로 돌려 대도 막힌다. 사내 서버가 있으므로 사설 대역은 막지 않는다.
func newHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: refuseLinkLocal}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func refuseLinkLocal(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("링크로컬 · 멀티캐스트 주소(%s)로는 연결하지 않습니다", ip)
	}
	return nil
}

// errNoChoices 는 응답에 답이 없다는 뜻이다.
var errNoChoices = errors.New("모델 응답에 답이 없다")

// upstreamError 는 2xx 가 아닌 응답의 오류다. 공급자의 JSON 오류 메시지만 싣고, 그 밖의 본문은 싣지 않는다 —
// 연결 테스트로 내부 서버의 내용을 읽어 내지 못하게.
func upstreamError(status int, body []byte) error {
	var e struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	detail := "(본문 생략)"
	if json.Unmarshal(body, &e) == nil && e.Error != nil && e.Error.Message != "" {
		detail = e.Error.Message
		if len(detail) > 300 {
			detail = detail[:300]
		}
	}
	return fmt.Errorf("모델 응답 %d: %s", status, detail)
}
