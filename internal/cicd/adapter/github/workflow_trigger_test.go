package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestWorkflowTrigger_ImplementsCIBuildTrigger(t *testing.T) {
	var _ port.CIBuildTrigger = (*WorkflowTrigger)(nil)
}

func noWait() {}

// GitHub 스택의 "실행" 도 CI 에 넘어가야 한다. 트리거가 없어 Trigger CI 단계가 늘
// "CI 플랫폼이 없다" 로 끝났다. GitHub 은 실행 id 를 돌려주지 않으므로 dispatch 뒤
// 새로 생긴 실행을 찾아 그 주소를 준다.
func TestWorkflowTrigger_DispatchesWorkflowAndFindsRun(t *testing.T) {
	listCalls := 0
	srv, recorded := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/shop/actions/runs":
			listCalls++
			if listCalls == 1 {
				// dispatch 전: 최근 실행은 끝났다.
				_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []map[string]any{
					{"id": 70, "status": "completed", "conclusion": "success", "event": "push",
						"html_url": "https://github.com/acme/shop/actions/runs/70"},
				}})
				return
			}
			// dispatch 뒤: 새 실행이 생겼다.
			assert.Contains(t, r.URL.RawQuery, "event=workflow_dispatch")
			_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []map[string]any{
				{"id": 71, "status": "queued", "event": "workflow_dispatch",
					"html_url": "https://github.com/acme/shop/actions/runs/71"},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/shop/actions/workflows/nullus-ci.yml/dispatches":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	trigger := NewWorkflowTrigger(NewClient(srv.URL, "ghp"), "acme", "nullus-ci.yml").withWait(noWait)
	runURL, err := trigger.TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.Equal(t, "https://github.com/acme/shop/actions/runs/71", runURL)
	var dispatched bool
	for _, req := range *recorded {
		if req.Method == http.MethodPost {
			dispatched = true
			assert.Equal(t, "main", req.Body["ref"])
		}
	}
	assert.True(t, dispatched)
}

// 새 실행이 아직 목록에 보이지 않으면 워크플로 페이지 주소를 준다 — 지어낸 실행 id 는
// 열리지 않는 링크가 된다.
func TestWorkflowTrigger_FallsBackToWorkflowPageWhenRunNotYetListed(t *testing.T) {
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []map[string]any{}})
		case http.MethodPost:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	trigger := NewWorkflowTrigger(NewClient(srv.URL, "ghp"), "acme", "nullus-ci.yml").
		WithWebBaseURL("https://github.com").withWait(noWait)
	runURL, err := trigger.TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.Equal(t, "https://github.com/acme/shop/actions/workflows/nullus-ci.yml", runURL)
}

// 그 브랜치의 최신 실행이 아직 돌고 있으면 새로 시작하지 않고 그 실행에 붙는다 —
// 생성 직후 화면이 부르는 "실행" 이 스캐폴딩 커밋의 실행과 겹치지 않게.
func TestWorkflowTrigger_ReusesInFlightRun(t *testing.T) {
	for _, status := range []string{"queued", "in_progress", "waiting", "pending", "requested"} {
		t.Run(status, func(t *testing.T) {
			srv, recorded := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					assert.Contains(t, r.URL.RawQuery, "branch=main")
					_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []map[string]any{
						{"id": 90, "status": status, "html_url": "https://github.com/acme/shop/actions/runs/90"},
						{"id": 89, "status": "completed", "conclusion": "success"},
					}})
					return
				}
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
			})

			runURL, err := NewWorkflowTrigger(NewClient(srv.URL, "ghp"), "acme", "nullus-ci.yml").withWait(noWait).
				TriggerBuild(context.Background(), "shop", "main")

			require.NoError(t, err)
			assert.Equal(t, "https://github.com/acme/shop/actions/runs/90", runURL)
			for _, req := range *recorded {
				assert.NotEqual(t, http.MethodPost, req.Method)
			}
		})
	}
}

// 이력 조회 실패는 실행을 막지 않는다.
func TestWorkflowTrigger_DispatchesWhenListingFails(t *testing.T) {
	dispatched := false
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
		case http.MethodPost:
			dispatched = true
			w.WriteHeader(http.StatusNoContent)
		}
	})

	_, err := NewWorkflowTrigger(NewClient(srv.URL, "ghp"), "acme", "nullus-ci.yml").withWait(noWait).
		TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.True(t, dispatched)
}

// 리포나 워크플로 파일이 없으면 프로비저닝이 끝나지 않은 것이다.
func TestWorkflowTrigger_ExplainsMissingRepositoryOrWorkflow(t *testing.T) {
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})

	_, err := NewWorkflowTrigger(NewClient(srv.URL, "ghp"), "acme", "nullus-ci.yml").withWait(noWait).
		TriggerBuild(context.Background(), "shop", "main")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "acme/shop")
	assert.Contains(t, err.Error(), "프로비저닝")
}

// workflow_dispatch 가 없는 옛 워크플로는 GitHub 이 422 로 거절한다. 무엇을 고쳐야
// 하는지 말해 준다 — 이 변경 전에 스캐폴딩된 리포가 그렇다.
func TestWorkflowTrigger_ExplainsWorkflowWithoutDispatchTrigger(t *testing.T) {
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []map[string]any{}})
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Workflow does not have 'workflow_dispatch' trigger"}`))
	})

	_, err := NewWorkflowTrigger(NewClient(srv.URL, "ghp"), "acme", "nullus-ci.yml").withWait(noWait).
		TriggerBuild(context.Background(), "shop", "main")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "workflow_dispatch")
	assert.Contains(t, err.Error(), ".github/workflows/nullus-ci.yml")
}

func TestWorkflowTrigger_RequiresRepoAndBranch(t *testing.T) {
	trigger := NewWorkflowTrigger(NewClient("http://gh.local", "ghp"), "acme", "nullus-ci.yml")

	_, err := trigger.TriggerBuild(context.Background(), "", "main")
	assert.Error(t, err)
	_, err = trigger.TriggerBuild(context.Background(), "shop", "")
	assert.Error(t, err)
}
