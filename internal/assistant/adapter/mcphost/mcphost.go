// Package mcphost 는 어시스턴트의 도구 호스트다. Nullus MCP 서버(internal/mcp)를 요청마다 in-memory
// transport 로 띄워 MCP 클라이언트로 부른다 — 외부 클라이언트(Claude Code)와 같은 도구 표면을 쓴다.
// 도구는 REST 의 얇은 클라이언트이고, 그 요청에 질문한 사용자의 인증 헤더를 싣는다.
package mcphost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/port"
	"github.com/cloud-nullus/draft/internal/mcp"
	"github.com/cloud-nullus/draft/pkg/nullusclient"
)

// Opener 는 요청마다 도구 세션을 연다. serverURL 은 API 자신(루프백)이다.
type Opener struct {
	serverURL string
	version   string
}

var _ port.ToolSessionOpener = (*Opener)(nil)

func NewOpener(serverURL, version string) *Opener {
	return &Opener{serverURL: serverURL, version: version}
}

// Open 은 조회 도구만 있는 MCP 서버를 띄우고 클라이언트를 붙인다. 변경 도구는 목록에서 아예 빠진다
// (MCP AllowWrite=false) — 모델이 부를 방법이 없다.
func (o *Opener) Open(ctx context.Context, creds port.Credentials) (port.ToolSession, error) {
	hc := &http.Client{Timeout: 30 * time.Second, Transport: credentialTransport{base: http.DefaultTransport, headers: creds}}
	client, err := nullusclient.New(nullusclient.Config{Server: o.serverURL}, nullusclient.WithHTTPClient(hc))
	if err != nil {
		return nil, err
	}
	server := mcp.NewServer(client, mcp.Options{AllowWrite: false, Version: o.version})

	ct, st := sdk.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		return nil, fmt.Errorf("MCP 서버 연결: %w", err)
	}
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "nullus-assistant", Version: o.version}, nil).Connect(ctx, ct, nil)
	if err != nil {
		_ = ss.Close()
		return nil, fmt.Errorf("MCP 클라이언트 연결: %w", err)
	}
	return &session{cs: cs, ss: ss}, nil
}

type session struct {
	cs *sdk.ClientSession
	ss *sdk.ServerSession
}

func (s *session) ListTools(ctx context.Context) ([]domain.ToolSpec, error) {
	res, err := s.cs.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	specs := make([]domain.ToolSpec, 0, len(res.Tools))
	for _, t := range res.Tools {
		params := map[string]any{"type": "object", "properties": map[string]any{}}
		if t.InputSchema != nil {
			raw, err := json.Marshal(t.InputSchema)
			if err != nil {
				return nil, fmt.Errorf("%s 인자 스키마: %w", t.Name, err)
			}
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, fmt.Errorf("%s 인자 스키마: %w", t.Name, err)
			}
		}
		specs = append(specs, domain.ToolSpec{Name: t.Name, Description: t.Description, Parameters: params})
	}
	return specs, nil
}

// CallTool 은 도구를 부른다. 모델이 만든 인자가 JSON 이 아니면 도구 실패로 돌려 다시 고치게 한다.
func (s *session) CallTool(ctx context.Context, name, arguments string) (string, bool, error) {
	var args any = map[string]any{}
	if strings.TrimSpace(arguments) != "" {
		if !json.Valid([]byte(arguments)) {
			return "인자가 올바른 JSON 이 아닙니다: " + arguments, true, nil
		}
		args = json.RawMessage(arguments)
	}
	res, err := s.cs.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", false, err
	}
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*sdk.TextContent); ok {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(text.Text)
		}
	}
	return b.String(), res.IsError, nil
}

func (s *session) Close() error {
	err := s.cs.Close()
	_ = s.ss.Close()
	return err
}

// credentialTransport 는 도구의 API 요청에 질문한 사용자의 인증 헤더를 싣는다.
type credentialTransport struct {
	base    http.RoundTripper
	headers port.Credentials
}

func (t credentialTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(t.headers) == 0 {
		return t.base.RoundTrip(req)
	}
	r := req.Clone(req.Context())
	for k, v := range t.headers {
		if v != "" {
			r.Header.Set(k, v)
		}
	}
	return t.base.RoundTrip(r)
}
