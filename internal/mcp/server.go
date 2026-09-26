// Package mcp 는 Nullus MCP 서버다 (트랙 B) — AI 어시스턴트(MCP 클라이언트)가
// Nullus 를 조회·조작하는 tool 표면을 stdio 로 제공한다.
//
// 원칙은 CLI 와 같다: /api/v1/* REST 의 얇은 클라이언트이며 서버 쪽 신규
// API 를 만들지 않는다. 설계는 docs/11_기능설계/Nullus_MCP_설계.md.
// stdout 은 프로토콜 전용이다 — 사람용 로그·경고는 전부 stderr 로 나간다
// (Automation 계약 §2).
package mcp

import (
	"context"
	"fmt"
	"io"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cloud-nullus/draft/pkg/nullusclient"
)

// Options 는 tool 표면 구성이다.
type Options struct {
	// AllowWrite 가 참이면 변경 tool 3종(stack_deploy·stack_rollback·
	// pipeline_deploy)을 등록한다. 거짓이면 표면에서 아예 제거된다 —
	// "호출하면 거부"가 아니라 list_tools 에 나타나지 않는다 (설계 §3).
	AllowWrite bool
	// Version 은 nullus 바이너리 버전이다 (R-1 ldflags 주입 값).
	Version string
}

// NewServer 는 tool 이 등록된 MCP 서버를 만든다. transport 연결은 하지 않는다
// — 운영은 Run 이 stdio 로, 테스트는 in-memory transport 로 연결한다.
func NewServer(c *nullusclient.Client, opts Options) *sdk.Server {
	server := sdk.NewServer(&sdk.Implementation{Name: "nullus", Version: opts.Version}, nil)
	registerReadTools(server, c)
	if opts.AllowWrite {
		registerWriteTools(server, c)
	}
	return server
}

// RunOptions 는 Run 의 구성이다. Options 와 달리 프로세스 환경(stderr·배너)
// 까지 포함한다.
type RunOptions struct {
	AllowWrite bool
	Version    string
	// ServerURL 은 시작 배너에 표시할 대상 서버 주소다.
	ServerURL string
	// Stderr 로 시작 안내·버전 스큐 경고가 나간다. stdout 은 쓰지 않는다.
	Stderr io.Writer
}

// Run 은 MCP 서버를 stdio 로 기동하고 클라이언트 종료까지 블록한다 (B-1).
// 설정 해석·토큰 검증·exit code 매핑은 CLI 어댑터(internal/cli) 몫이다 —
// 여기는 이미 준비된 클라이언트로 표면을 서비스할 뿐이다.
//
// 설계 §6: 버전 스큐는 시작 시 1회 검사해 stderr 경고만 남긴다 — 서버 도달
// 실패로 기동을 막지 않는다 (tool 호출이 각자 실패를 보고한다).
func Run(ctx context.Context, c *nullusclient.Client, opts RunOptions) error {
	warnVersionSkew(ctx, c, opts.Stderr)

	surface := "읽기 tool 6종"
	if opts.AllowWrite {
		surface += " + 변경 tool 3종 (--allow-write)"
	}
	fmt.Fprintf(opts.Stderr, "nullus mcp serve %s — %s, 서버 %s\n", opts.Version, surface, opts.ServerURL)

	server := NewServer(c, Options{AllowWrite: opts.AllowWrite, Version: opts.Version})
	return server.Run(ctx, &sdk.StdioTransport{})
}

func warnVersionSkew(ctx context.Context, c *nullusclient.Client, stderr io.Writer) {
	skew, err := c.CheckVersionSkew(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "경고: 서버 버전 확인 실패 — %v\n", err)
		return
	}
	if !skew.Compatible {
		fmt.Fprintf(stderr, "경고: 서버 버전 %s 는 최소 호환 버전 %s 보다 낮다 — tool 호출이 실패할 수 있다\n",
			skew.ServerVersion, skew.MinSupported)
	}
}
