package nullusclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client 는 /api/v1/* REST 의 얇은 클라이언트다. 비즈니스 로직도, 재시도도
// 없다 — 서버가 판정하고, 재시도 여부는 호출한 자동화(스크립트·에이전트)가
// 정한다 (컨셉 문서 §5, Automation 계약).
type Client struct {
	server string
	token  string
	hc     *http.Client
}

// Option 은 Client 생성 옵션이다.
type Option func(*Client)

// WithHTTPClient 는 기본 http.Client 를 교체한다 (테스트·타임아웃 조정용).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.hc = hc }
}

// New 는 해석이 끝난 Config 로 클라이언트를 만든다. 서버 주소는 필수다.
// 토큰은 선택이다 — dev 모드(auth.mode=session)는 토큰 없이 동작한다.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.Server == "" {
		return nil, fmt.Errorf("서버 주소가 없다 — --server 플래그, %s env, 또는 ~/.nullus/config 로 지정하라", EnvServer)
	}
	c := &Client{
		server: strings.TrimRight(cfg.Server, "/"),
		token:  cfg.Token,
		hc:     &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Do 는 API 를 한 번 호출한다. path 는 "/api/v1/..." 절대 경로. in 이 nil 이
// 아니면 JSON body 로 보내고, out 이 nil 이 아니면 응답 JSON 을 디코딩한다.
// 2xx 가 아니면 *APIError 를 반환한다.
func (c *Client) Do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("요청 직렬화: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.server+path, body)
	if err != nil {
		return fmt.Errorf("요청 생성: %w", err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		// context 취소는 호출측 사정이므로 그대로 돌려준다.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return &APIError{Kind: KindServer, Message: err.Error(), cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return apiErrorFromResponse(resp)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("응답 디코딩 (%s %s): %w", method, path, err)
	}
	return nil
}

// apiErrorFromResponse 는 실패 응답을 APIError 로 바꾼다. 서버(echo)는 보통
// {"message": "..."} 를 주므로 그걸 우선 취하고, 아니면 본문을 자른다.
func apiErrorFromResponse(resp *http.Response) *APIError {
	const maxBody = 2 << 10 // 에러 메시지용으로 2KiB 면 충분하다
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))

	msg := ""
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(b, &payload); err == nil && payload.Message != "" {
		msg = payload.Message
	} else {
		msg = strings.TrimSpace(string(b))
	}
	if msg == "" {
		msg = resp.Status
	}
	return &APIError{
		Kind:       kindForStatus(resp.StatusCode),
		StatusCode: resp.StatusCode,
		Message:    msg,
	}
}
