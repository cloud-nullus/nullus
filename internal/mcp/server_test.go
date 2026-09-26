package mcp

import (
	"context"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cloud-nullus/draft/pkg/nullusclient"
)

// connect 는 in-memory transport 로 서버-클라이언트 세션을 연결한다
// (설계 §8 — DB·실서버 없이 tool 표면을 검증한다).
func connect(t *testing.T, server *sdk.Server) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()

	ct, st := sdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("서버 연결 실패: %v", err)
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "test"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("클라이언트 연결 실패: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func listToolNames(t *testing.T, cs *sdk.ClientSession) map[string]bool {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list_tools 실패: %v", err)
	}
	names := make(map[string]bool, len(res.Tools))
	for _, tool := range res.Tools {
		names[tool.Name] = true
	}
	return names
}

func testClient(t *testing.T, server string) *nullusclient.Client {
	t.Helper()
	c, err := nullusclient.New(nullusclient.Config{Server: server, Token: "test-token"})
	if err != nil {
		t.Fatalf("클라이언트 생성 실패: %v", err)
	}
	return c
}

var readToolNames = []string{
	"stack_list", "stack_status", "cluster_list",
	"template_list", "compat_check", "stack_logs_tail",
}

var writeToolNames = []string{"stack_deploy", "stack_rollback", "pipeline_deploy"}

func TestNewServer_DefaultSurface_ReadOnly(t *testing.T) {
	server := NewServer(testClient(t, "http://unused.invalid"), Options{Version: "test"})
	names := listToolNames(t, connect(t, server))

	for _, want := range readToolNames {
		if !names[want] {
			t.Errorf("읽기 tool %q 이 기본 표면에 없다", want)
		}
	}
	// 변경 tool 은 "호출하면 거부"가 아니라 표면에서 제거돼야 한다 (설계 §3).
	for _, banned := range writeToolNames {
		if names[banned] {
			t.Errorf("변경 tool %q 이 --allow-write 없이 노출됐다", banned)
		}
	}
	if len(names) != len(readToolNames) {
		t.Errorf("기본 표면 tool 수 = %d, want %d (%v)", len(names), len(readToolNames), names)
	}
}

func TestNewServer_AllowWriteSurface(t *testing.T) {
	server := NewServer(testClient(t, "http://unused.invalid"), Options{Version: "test", AllowWrite: true})
	names := listToolNames(t, connect(t, server))

	for _, want := range append(append([]string{}, readToolNames...), writeToolNames...) {
		if !names[want] {
			t.Errorf("tool %q 이 allow-write 표면에 없다", want)
		}
	}
	if len(names) != len(readToolNames)+len(writeToolNames) {
		t.Errorf("allow-write 표면 tool 수 = %d, want %d", len(names), len(readToolNames)+len(writeToolNames))
	}
}
