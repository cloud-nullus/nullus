package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestPipelineTrigger_ImplementsCIBuildTrigger(t *testing.T) {
	var _ port.CIBuildTrigger = (*PipelineTrigger)(nil)
}

// 스택에 묶인 파이프라인의 "실행" 은 플랫폼이 빌드하지 않고 CI 에 넘긴다. 이 넘김이
// Jenkins 번들에만 있어 GitLab 스택에서는 "Trigger CI" 단계가 늘 실패로 남았다.
func TestPipelineTrigger_CreatesPipelineOnBranch(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/projects/acme%2Fshop/pipelines":
			// 최근 실행은 끝났다 — 새로 만들어야 한다.
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 70, "iid": 7, "status": "success", "ref": "main"},
			})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/v4/projects/acme%2Fshop/pipeline":
			assert.Equal(t, "main", body["ref"])
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 77, "iid": 8, "status": "created", "ref": "main",
				"web_url": "http://gitlab-webservice-default.devsecops.svc:8181/acme/shop/-/pipelines/77",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	trigger := NewPipelineTrigger(NewClient(srv.URL, "tok"), "acme").
		WithWebBaseURL("https://gitlab.nullus.local/")
	runURL, err := trigger.TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	// 링크는 브라우저가 여는 주소로 만든다. API 가 준 web_url 은 클러스터 안 주소일 수 있다.
	assert.Equal(t, "https://gitlab.nullus.local/acme/shop/-/pipelines/77", runURL)
	posts := 0
	for _, req := range *recorded {
		if req.Method == http.MethodPost {
			posts++
		}
	}
	assert.Equal(t, 1, posts)
}

// 외부 주소를 모르면 API 가 준 web_url 을 그대로 쓴다 — 지어내지 않는다.
func TestPipelineTrigger_FallsBackToAPIWebURL(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 1, "web_url": "http://gl.internal/acme/shop/-/pipelines/1",
			})
		}
	})

	runURL, err := NewPipelineTrigger(NewClient(srv.URL, "tok"), "acme").
		TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.Equal(t, "http://gl.internal/acme/shop/-/pipelines/1", runURL)
}

// 그 브랜치의 최신 파이프라인이 아직 돌고 있으면 새로 만들지 않고 그 실행에 붙는다.
//
// 화면은 파이프라인을 만든 직후 "실행" 을 한 번 더 부른다. 스캐폴딩 커밋이 이미
// 파이프라인을 시작했는데 또 만들면 같은 커밋의 배포 잡 둘이 같은 브랜치에
// 되커밋을 밀어 둘째가 non-fast-forward 로 실패한다.
func TestPipelineTrigger_ReusesInFlightPipeline(t *testing.T) {
	for _, status := range []string{"created", "waiting_for_resource", "preparing", "pending", "running"} {
		t.Run(status, func(t *testing.T) {
			srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode([]map[string]any{
						{"id": 90, "iid": 9, "status": status, "ref": "main",
							"web_url": "http://gl.internal/acme/shop/-/pipelines/90"},
						{"id": 89, "iid": 8, "status": "success", "ref": "main"},
					})
					return
				}
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
			})

			runURL, err := NewPipelineTrigger(NewClient(srv.URL, "tok"), "acme").
				WithWebBaseURL("https://gitlab.nullus.local").
				TriggerBuild(context.Background(), "shop", "main")

			require.NoError(t, err)
			assert.Equal(t, "https://gitlab.nullus.local/acme/shop/-/pipelines/90", runURL)
			for _, req := range *recorded {
				assert.NotEqual(t, http.MethodPost, req.Method)
			}
			require.NotEmpty(t, *recorded)
			assert.Contains(t, (*recorded)[0].Query, "ref=main", "그 브랜치의 파이프라인만 본다")
		})
	}
}

// 최신 파이프라인이 끝났으면(실패·취소 포함) 붙을 것이 없다 — 새로 만든다.
func TestPipelineTrigger_CreatesWhenLatestIsFinished(t *testing.T) {
	for _, status := range []string{"success", "failed", "canceled", "skipped", "manual"} {
		t.Run(status, func(t *testing.T) {
			created := false
			srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
				switch r.Method {
				case http.MethodGet:
					_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 5, "status": status, "ref": "main"}})
				case http.MethodPost:
					created = true
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(map[string]any{"id": 6, "web_url": "http://gl/x/-/pipelines/6"})
				}
			})

			_, err := NewPipelineTrigger(NewClient(srv.URL, "tok"), "acme").
				TriggerBuild(context.Background(), "shop", "main")

			require.NoError(t, err)
			assert.True(t, created)
		})
	}
}

// 이력 조회는 중복을 피하기 위한 것이다. 그것이 실패했다고 실행까지 막지는 않는다.
func TestPipelineTrigger_CreatesWhenListingFails(t *testing.T) {
	created := false
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
		case http.MethodPost:
			created = true
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 6, "web_url": "http://gl/x/-/pipelines/6"})
		}
	})

	_, err := NewPipelineTrigger(NewClient(srv.URL, "tok"), "acme").
		TriggerBuild(context.Background(), "shop", "main")

	require.NoError(t, err)
	assert.True(t, created)
}

// 프로젝트가 없으면 프로비저닝이 끝나지 않은 것이다. 상태 코드가 아니라 무엇을 해야
// 하는지 말해 준다.
func TestPipelineTrigger_ExplainsMissingProject(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Project Not Found"}`))
	})

	_, err := NewPipelineTrigger(NewClient(srv.URL, "tok"), "acme").
		TriggerBuild(context.Background(), "shop", "main")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "acme/shop")
	assert.Contains(t, err.Error(), "프로비저닝")
}

// 그 밖의 거절(브랜치 없음, .gitlab-ci.yml 없음 등)은 GitLab 의 설명을 그대로 싣는다.
func TestPipelineTrigger_PassesThroughOtherRejections(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]map[string]any{})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":{"base":["Missing CI config file"]}}`))
	})

	_, err := NewPipelineTrigger(NewClient(srv.URL, "tok"), "acme").
		TriggerBuild(context.Background(), "shop", "main")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing CI config file")
	assert.Contains(t, err.Error(), "acme/shop")
}

func TestPipelineTrigger_RequiresProjectAndBranch(t *testing.T) {
	trigger := NewPipelineTrigger(NewClient("http://gitlab.local", "tok"), "acme")

	_, err := trigger.TriggerBuild(context.Background(), "", "main")
	assert.Error(t, err)
	_, err = trigger.TriggerBuild(context.Background(), "shop", "")
	assert.Error(t, err)
}
