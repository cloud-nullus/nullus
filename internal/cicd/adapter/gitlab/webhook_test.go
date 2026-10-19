package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const argoHookURL = "http://argo-cd-argocd-server.devsecops.svc:80/api/webhook"

func TestClient_ImplementsSCMWebhookProvisioner(t *testing.T) {
	var _ port.SCMWebhookProvisioner = (*Client)(nil)
}

// fakeGitLabSettings 는 허용 목록을 들고 있는 GitLab 흉내다.
type fakeGitLabSettings struct {
	allowlist string
	hooks     []map[string]any
	// ignorePut 이면 설정 변경 요청을 받고도 반영하지 않는다(알 수 없는 인자를 무시하는 GitLab).
	ignorePut bool
	forbidden bool
}

func (f *fakeGitLabSettings) handle(w http.ResponseWriter, r *http.Request, body map[string]any) {
	switch {
	case r.URL.Path == "/api/v4/application/settings" && f.forbidden:
		w.WriteHeader(http.StatusForbidden)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v4/application/settings":
		_ = json.NewEncoder(w).Encode(map[string]any{"outbound_local_requests_allowlist_raw": f.allowlist})
	case r.Method == http.MethodPut && r.URL.Path == "/api/v4/application/settings":
		// GitLab 17.7 은 원문(outbound_local_requests_allowlist_raw)만 받는다. 배열 인자
		// (outbound_local_requests_whitelist)를 보내면 "받을 인자가 없다" 는 400 이다.
		raw, ok := body["outbound_local_requests_allowlist_raw"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"abuse_notification_email, admin_mode, ... are missing, at least one parameter must be provided"}`))
			return
		}
		if !f.ignorePut {
			f.allowlist = raw
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/projects/100/hooks":
		// 목록은 per_page 를 붙여 부른다
		_ = json.NewEncoder(w).Encode(f.hooks)
	case r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/v4/projects/100/hooks":
		f.hooks = append(f.hooks, body)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(body)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// 클러스터 안 주소로의 webhook 은 GitLab 기본 설정이 막는다. 전체를 풀지 않고 그
// 호스트 하나만 허용 목록에 더한 뒤 push webhook 을 건다.
func TestEnsureWebhook_AllowsClusterHostThenCreatesPushHook(t *testing.T) {
	gl := &fakeGitLabSettings{allowlist: "10.0.0.5\nexample.internal"}
	srv, _ := newStubGitLab(t, gl.handle)

	require.NoError(t, NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", argoHookURL, ""))

	assert.Equal(t, "10.0.0.5\nexample.internal\nargo-cd-argocd-server.devsecops.svc:80", gl.allowlist,
		"있던 항목은 그대로 두고, 호스트의 모든 포트가 아니라 그 포트만 더한다")
	require.Len(t, gl.hooks, 1)
	assert.Equal(t, argoHookURL, gl.hooks[0]["url"])
	assert.Equal(t, true, gl.hooks[0]["push_events"])
	assert.NotContains(t, gl.hooks[0], "token", "비밀값이 없으면 보내지 않는다")
}

// 이미 걸려 있으면 다시 걸지 않는다 — 파이프라인 정의를 갱신하거나 다시 만들 때도 불린다.
func TestEnsureWebhook_IsIdempotent(t *testing.T) {
	gl := &fakeGitLabSettings{
		allowlist: "argo-cd-argocd-server.devsecops.svc:80",
		hooks:     []map[string]any{{"id": 1, "url": argoHookURL}},
	}
	srv, recorded := newStubGitLab(t, gl.handle)

	require.NoError(t, NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", argoHookURL, ""))

	for _, req := range *recorded {
		assert.Equal(t, http.MethodGet, req.Method, "이미 있으면 아무것도 바꾸지 않는다: %s %s", req.Method, req.Path)
	}
}

// 허용 목록에 이미 있으면 설정을 건드리지 않는다. 같은 호스트가 포트 없이 있으면 모든 포트가
// 열려 있는 것이니 그것으로 충분하다.
func TestEnsureWebhook_LeavesAllowlistWhenHostPresent(t *testing.T) {
	for _, existing := range []string{
		"a.example, argo-cd-argocd-server.devsecops.svc:80 ; b.example",
		"a.example\nargo-cd-argocd-server.devsecops.svc",
	} {
		gl := &fakeGitLabSettings{allowlist: existing}
		srv, recorded := newStubGitLab(t, gl.handle)

		require.NoError(t, NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", argoHookURL, ""))

		for _, req := range *recorded {
			assert.NotEqual(t, http.MethodPut, req.Method, existing)
		}
		require.Len(t, gl.hooks, 1)
	}
}

// 응답에 허용 목록 항목이 아예 없으면 빈 목록으로 보고 덮어쓰지 않는다 — 관리자가 넣어 둔
// 항목이 사라진다.
func TestEnsureWebhook_DoesNotOverwriteWhenAllowlistMissingFromResponse(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/application/settings" {
			_ = json.NewEncoder(w).Encode(map[string]any{"signup_enabled": false})
			return
		}
		(&fakeGitLabSettings{}).handle(w, r, body)
	})

	err := NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", argoHookURL, "")
	require.Error(t, err)
	for _, req := range *recorded {
		assert.NotEqual(t, http.MethodPut, req.Method)
	}
}

// hook 이 많은 저장소에서도 이미 건 것을 찾는다(기본 페이지는 20개다).
func TestEnsureWebhook_ListsHooksWithLargePage(t *testing.T) {
	gl := &fakeGitLabSettings{allowlist: "argo-cd-argocd-server.devsecops.svc:80"}
	srv, recorded := newStubGitLab(t, gl.handle)

	require.NoError(t, NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", argoHookURL, ""))

	assert.Equal(t, "per_page=100", (*recorded)[0].Query)
}

// 클러스터 주소라도 "Invalid url" 이 아닌 422(hook 개수 한도 등)는 기다린다고 풀리지 않는다.
func TestEnsureWebhook_OtherUnprocessableIsFinal(t *testing.T) {
	gl := &fakeGitLabSettings{}
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		if r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/v4/projects/100/hooks" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"Maximum number of project hooks (100) exceeded"}`))
			return
		}
		gl.handle(w, r, body)
	})

	err := NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", argoHookURL, "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, port.ErrWebhookTargetPending)
}

// 클러스터 밖 주소는 허용 목록과 상관없다 — 관리자 설정을 건드리지 않는다.
func TestEnsureWebhook_ExternalTargetSkipsAllowlist(t *testing.T) {
	gl := &fakeGitLabSettings{}
	srv, recorded := newStubGitLab(t, gl.handle)

	require.NoError(t, NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", "https://argocd.example.com/api/webhook", "s3cret"))

	for _, req := range *recorded {
		assert.NotContains(t, req.Path, "application/settings")
	}
	require.Len(t, gl.hooks, 1)
	assert.Equal(t, "s3cret", gl.hooks[0]["token"])
}

// GitLab 은 모르는 인자를 조용히 무시한다. 바꿨다고 믿고 넘어가면 webhook 생성이
// "local network" 로 막힌 이유를 알 수 없으므로, 반영됐는지 다시 읽어 본다.
func TestEnsureWebhook_FailsWhenAllowlistChangeNotApplied(t *testing.T) {
	gl := &fakeGitLabSettings{ignorePut: true}
	srv, _ := newStubGitLab(t, gl.handle)

	err := NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", argoHookURL, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "허용 목록")
	assert.Empty(t, gl.hooks)
}

func TestEnsureWebhook_ReportsMissingAdminRight(t *testing.T) {
	gl := &fakeGitLabSettings{forbidden: true}
	srv, _ := newStubGitLab(t, gl.handle)

	err := NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", argoHookURL, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
	assert.Contains(t, err.Error(), "관리자 권한")
}

// GitLab 의 400 응답은 받을 수 있는 설정 이름 수백 개를 늘어놓는다. 그대로 경고에 실으면
// 화면에서 읽을 수 없으므로 줄인다.
func TestEnsureWebhook_KeepsErrorShort(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.URL.Path == "/api/v4/application/settings" && r.Method == http.MethodPut {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"` + strings.Repeat("setting_name, ", 300) + `"}`))
			return
		}
		(&fakeGitLabSettings{}).handle(w, r, nil)
	})

	err := NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", argoHookURL, "")
	require.Error(t, err)
	assert.Less(t, len(err.Error()), 600)
	assert.Contains(t, err.Error(), "400")
}

func TestEnsureWebhook_RequiresProjectAndTarget(t *testing.T) {
	c := NewClient("http://unused", "tok")
	assert.Error(t, c.EnsureWebhook(context.Background(), "", argoHookURL, ""))
	assert.Error(t, c.EnsureWebhook(context.Background(), "100", " ", ""))
}

// 허용 목록에 넣은 클러스터 주소가 422 로 거부되면 GitLab 이 아직 옛 설정(최대 1분 캐시)으로
// 검사한 것이다. 호출부가 잠시 뒤 다시 걸 수 있게 그렇다고 알린다.
func TestEnsureWebhook_ReportsPendingWhenSettingsNotYetApplied(t *testing.T) {
	gl := &fakeGitLabSettings{}
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		if r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/v4/projects/100/hooks" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":"Invalid url given"}`))
			return
		}
		gl.handle(w, r, body)
	})

	err := NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", argoHookURL, "")
	require.ErrorIs(t, err, port.ErrWebhookTargetPending)
}

// 클러스터 밖 주소의 422 는 기다린다고 풀리지 않는다.
func TestEnsureWebhook_ExternalTargetRejectionIsFinal(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":"Invalid url given"}`))
			return
		}
		(&fakeGitLabSettings{}).handle(w, r, body)
	})

	err := NewClient(srv.URL, "tok").EnsureWebhook(context.Background(), "100", "https://argocd.example.com/api/webhook", "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, port.ErrWebhookTargetPending)
}
