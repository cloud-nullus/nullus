package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cloud-nullus/draft/pkg/nullusclient"
)

// EnvSCMToken 은 stack_deploy 가 배포 본문에 주입하는 SCM PAT 의 유일한
// 공급 경로다 — tool 인자로 시크릿을 받지 않는다 (설계 §5). 모델 컨텍스트와
// 대화 로그에 시크릿이 남지 않게 하기 위해서다.
//
// 주입 위치는 스택 생성 본문이 아니라 **deploy 요청 본문**의
// source_control.personal_access_token 이다 — stacks.config 는 평문 JSONB 로
// 저장되므로 생성 본문에 넣으면 DB 를 읽는 누구에게나 노출된다 (서버
// deployRequest 주석 참고).
const EnvSCMToken = "NULLUS_SCM_TOKEN"

type stackDeployIn struct {
	// Stack 은 stack.yaml v1alpha1 과 동일한 구조의 객체다
	// (docs/11_기능설계/Nullus_Stack_YAML_스키마.md). 검증은 서버가 한다.
	Stack map[string]any `json:"stack" jsonschema:"stack.yaml v1alpha1 구조와 동일한 스택 정의 객체 — name(필수)·cluster(필수, 이름 또는 ID)·namespace·template·config"`
	// AcknowledgeWarnings 는 호환성 경고를 인지하고 배포를 강행한다는 표시다
	// — 서버의 DEPLOY_COMPAT_WARN_UNACK 게이트 대응.
	AcknowledgeWarnings bool `json:"acknowledge_warnings,omitempty" jsonschema:"호환성 경고를 인지하고 진행 (기본 false)"`
}

type stackRollbackIn struct {
	StackID   string `json:"stack_id" jsonschema:"롤백할 스택 ID"`
	VersionID string `json:"version_id" jsonschema:"되돌릴 설정(config) 버전 ID"`
	Reason    string `json:"reason,omitempty" jsonschema:"롤백 사유 (감사 로그용)"`
}

type pipelineDeployIn struct {
	PipelineID string `json:"pipeline_id" jsonschema:"배포를 트리거할 파이프라인 ID"`
}

// registerWriteTools 는 변경 tool 3종을 등록한다. --allow-write 옵트인
// 없이는 호출되지 않는다 (NewServer, 설계 §3).
func registerWriteTools(server *sdk.Server, c *nullusclient.Client) {
	sdk.AddTool(server, &sdk.Tool{
		Name:        "stack_deploy",
		Description: "스택 생성 + 배포 — stack.yaml v1alpha1 구조 입력. 호환성 게이트는 서버 판정을 따른다. SCM 토큰은 인자가 아니라 서버 프로세스의 " + EnvSCMToken + " env 에서 읽는다",
	}, func(ctx context.Context, req *sdk.CallToolRequest, in stackDeployIn) (*sdk.CallToolResult, any, error) {
		payload, err := createPayload(ctx, c, in.Stack)
		if err != nil {
			return nil, nil, err
		}
		created, err := createStack(ctx, c, payload)
		if err != nil {
			return nil, nil, err
		}
		deployBody := map[string]any{}
		if in.AcknowledgeWarnings {
			deployBody["acknowledge_warnings"] = true
		}
		// SCM PAT 는 deploy 본문으로만 전달한다 — 생성 본문(stacks.config)은
		// 평문 JSONB 로 저장되기 때문이다.
		if tok := os.Getenv(EnvSCMToken); tok != "" {
			deployBody["source_control"] = map[string]any{"personal_access_token": tok}
		}
		var out any
		path := "/api/v1/stacks/" + url.PathEscape(created.ID) + "/deploy"
		if err := c.Do(ctx, http.MethodPost, path, deployBody, &out); err != nil {
			// 스택은 이미 생성됐다 — stack_deploy 를 다시 부르면 스택이 하나 더 생긴다.
			return nil, nil, fmt.Errorf("스택 %s 은 생성됐지만 배포 요청이 실패했다 — stack_deploy 를 다시 부르지 말고 stack_status 로 상태를 확인하라: %w", created.ID, err)
		}
		return jsonResult(map[string]any{"stack_id": created.ID, "deploy": out})
	})

	sdk.AddTool(server, &sdk.Tool{
		Name:        "stack_rollback",
		Description: "스택 설정(config)을 지정 버전으로 롤백 — Helm 롤백은 설치 엔진이 자동 수행하므로 여기 없다",
	}, func(ctx context.Context, req *sdk.CallToolRequest, in stackRollbackIn) (*sdk.CallToolResult, any, error) {
		// 서버 rollbackRequest 는 camelCase(versionId)다 — 다른 스택 API 의
		// snake_case 와 다르니 주의 (history_handler.go).
		body := map[string]any{"versionId": in.VersionID}
		if in.Reason != "" {
			body["reason"] = in.Reason
		}
		var out any
		path := "/api/v1/stacks/" + url.PathEscape(in.StackID) + "/rollback"
		if err := c.Do(ctx, http.MethodPost, path, body, &out); err != nil {
			return nil, nil, err
		}
		return jsonResult(out)
	})

	sdk.AddTool(server, &sdk.Tool{
		Name:        "pipeline_deploy",
		Description: "CI/CD 파이프라인 배포 트리거",
	}, func(ctx context.Context, req *sdk.CallToolRequest, in pipelineDeployIn) (*sdk.CallToolResult, any, error) {
		var out any
		// 파이프라인 라우트는 /api/v1/cicd 그룹 하위다 (cmd/api/main.go) —
		// 핸들러 주석의 /api/v1/pipelines 는 옛 경로다.
		path := "/api/v1/cicd/pipelines/" + url.PathEscape(in.PipelineID) + "/deploy"
		if err := c.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
			return nil, nil, err
		}
		return jsonResult(out)
	})
}

type createdStack struct {
	ID string `json:"id"`
}

// createPayload 는 stack.yaml v1alpha1 을 생성 API payload 로 매핑한다
// (docs/11_기능설계/Nullus_Stack_YAML_스키마.md §1.2). config 는 변환 없이
// 넘기되, 섞인 시크릿성 필드는 제거한다 — stacks.config 는 평문 JSONB 라
// 생성 본문의 토큰은 DB 를 읽는 누구에게나 노출된다. 자격증명은 deploy
// 본문(EnvSCMToken)으로만 나간다.
func createPayload(ctx context.Context, c *nullusclient.Client, stack map[string]any) (map[string]any, error) {
	name := stringField(stack, "name")
	if name == "" {
		return nil, errors.New("stack.name 은 필수다")
	}
	cluster := stringField(stack, "cluster")
	if cluster == "" {
		return nil, errors.New("stack.cluster 는 필수다 — 클러스터 이름 또는 ID")
	}
	clusterID, err := resolveClusterID(ctx, c, cluster)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"name":           name,
		"cluster_id":     clusterID,
		"namespace":      stringField(stack, "namespace"),
		"golden_path_id": stringField(stack, "template"),
	}
	if cfg, ok := stack["config"]; ok {
		payload["config"] = redactSecrets(cfg)
	}
	return payload, nil
}

// resolveClusterID 는 클러스터 이름을 ID 로 해석한다. 이미 ID(UUID)면 조회하지
// 않는다 — 목록 API(/admin/clusters)는 admin 전용이라 조회를 강제하면 devops
// 사용자는 배포할 수 없다.
func resolveClusterID(ctx context.Context, c *nullusclient.Client, cluster string) (string, error) {
	if _, err := uuid.Parse(cluster); err == nil {
		return cluster, nil
	}

	var resp struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := c.Do(ctx, http.MethodGet, "/api/v1/admin/clusters", nil, &resp); err != nil {
		return "", fmt.Errorf("클러스터 %q 의 ID 해석 실패 (ID 를 직접 넣으면 조회하지 않는다): %w", cluster, err)
	}
	var ids []string
	for _, it := range resp.Items {
		if it.Name == cluster {
			ids = append(ids, it.ID)
		}
	}
	switch len(ids) {
	case 0:
		return "", fmt.Errorf("클러스터 %q 를 찾지 못했다 — cluster_list 로 이름을 확인하라", cluster)
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("클러스터 이름 %q 가 %d개다 — 이름 대신 ID 를 넣어라: %v", cluster, len(ids), ids)
	}
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// createStack 은 생성 API payload 로 스택을 만들고 ID 를 돌려받는다.
func createStack(ctx context.Context, c *nullusclient.Client, payload map[string]any) (createdStack, error) {
	var created createdStack
	if err := c.Do(ctx, http.MethodPost, "/api/v1/stacks", payload, &created); err != nil {
		return createdStack{}, err
	}
	return created, nil
}
