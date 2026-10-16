package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cloud-nullus/draft/pkg/nullusclient"
)

// jsonResult 는 서버 응답을 들여쓴 JSON text content 로 감싼다 — 모든 tool
// 출력은 JSON 이다 (설계 §2). 실패는 핸들러가 error 를 반환하면 SDK 가
// isError 로 포장하고, APIError.Error() 가 HTTP 상태·trace_id 를 담는다.
func jsonResult(v any) (*sdk.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("응답 직렬화: %w", err)
	}
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
}

// getJSON 은 GET 응답을 원문 구조 그대로 받는다.
func getJSON(ctx context.Context, c *nullusclient.Client, path string) (any, error) {
	var out any
	if err := c.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type stackStatusIn struct {
	StackID string `json:"stack_id" jsonschema:"조회할 스택 ID"`
}

type compatCheckIn struct {
	StackID string            `json:"stack_id" jsonschema:"검증할 스택 ID"`
	Tools   map[string]string `json:"tools,omitempty" jsonschema:"도구별 버전 오버라이드 (예: {\"jenkins\": \"2.452.1\"}) — 생략하면 현재 구성으로 검증"`
}

type stackLogsTailIn struct {
	StackID string `json:"stack_id" jsonschema:"스택 ID"`
	Lines   int    `json:"lines,omitempty" jsonschema:"가져올 최근 로그 줄 수 (기본 100)"`
}

func registerReadTools(server *sdk.Server, c *nullusclient.Client) {
	sdk.AddTool(server, &sdk.Tool{
		Name:        "stack_list",
		Description: "Nullus 스택 목록 조회 — 이름·상태·클러스터·네임스페이스",
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
		out, err := getJSON(ctx, c, "/api/v1/stacks")
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(out)
	})

	sdk.AddTool(server, &sdk.Tool{
		Name:        "stack_status",
		Description: "스택 배포 상태·진행률·실패 스텝과 사유 조회",
	}, func(ctx context.Context, req *sdk.CallToolRequest, in stackStatusIn) (*sdk.CallToolResult, any, error) {
		out, err := getJSON(ctx, c, "/api/v1/stacks/"+url.PathEscape(in.StackID)+"/status")
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(out)
	})

	sdk.AddTool(server, &sdk.Tool{
		Name:        "cluster_list",
		Description: "등록된 Kubernetes 클러스터 목록 조회 (kubeconfig 등 시크릿 제외)",
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
		out, err := getJSON(ctx, c, "/api/v1/admin/clusters")
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(redactSecrets(out))
	})

	sdk.AddTool(server, &sdk.Tool{
		Name:        "template_list",
		Description: "스택 템플릿(Golden Path) 카탈로그 조회",
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
		out, err := getJSON(ctx, c, "/api/v1/stacks/templates")
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(out)
	})

	sdk.AddTool(server, &sdk.Tool{
		Name:        "compat_check",
		Description: "스택 도구 조합의 호환성 검증 — POST 지만 시맨틱은 조회다",
	}, func(ctx context.Context, req *sdk.CallToolRequest, in compatCheckIn) (*sdk.CallToolResult, any, error) {
		var body any
		if len(in.Tools) > 0 {
			body = map[string]any{"tools": in.Tools}
		}
		var out any
		path := "/api/v1/stacks/" + url.PathEscape(in.StackID) + "/validate"
		if err := c.Do(ctx, http.MethodPost, path, body, &out); err != nil {
			return nil, nil, err
		}
		return jsonResult(out)
	})

	sdk.AddTool(server, &sdk.Tool{
		Name:        "stack_logs_tail",
		Description: "스택 설치 로그의 최근 N줄 조회 (기본 100줄)",
	}, func(ctx context.Context, req *sdk.CallToolRequest, in stackLogsTailIn) (*sdk.CallToolResult, any, error) {
		lines := in.Lines
		if lines <= 0 {
			lines = 100
		}
		path := "/api/v1/stacks/" + url.PathEscape(in.StackID) + "/deploy/logs/tail?lines=" + strconv.Itoa(lines)
		out, err := getJSON(ctx, c, path)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(out)
	})
}

// secretKeys 는 읽기 tool 응답에서 제거하는 필드다 (설계 §5 — 시크릿성
// 필드는 제거 후 반환). 값 마스킹이 아니라 키 자체를 지운다.
var secretKeys = map[string]bool{
	"kubeconfig":           true,
	"kubeconfig_encrypted": true,
	"token":                true,
	"password":             true,
	"secret":               true,
}

// redactSecrets 는 디코딩된 JSON 트리를 내려가며 시크릿 키를 제거한다.
func redactSecrets(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if secretKeys[k] {
				delete(t, k)
				continue
			}
			t[k] = redactSecrets(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = redactSecrets(val)
		}
		return t
	default:
		return v
	}
}
