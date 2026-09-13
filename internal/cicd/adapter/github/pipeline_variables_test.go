package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestClient_ImplementsPipelineVariableWriter(t *testing.T) {
	var _ port.PipelineVariableWriter = (*Client)(nil)
}

type seenRequest struct {
	method, path string
	body         map[string]any
}

func variablesServer(t *testing.T, createStatus int) (*httptest.Server, *[]seenRequest) {
	t.Helper()
	seen := &[]seenRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		*seen = append(*seen, seenRequest{method: r.Method, path: r.URL.Path, body: body})
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/shop/actions/variables":
			w.WriteHeader(createStatus)
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/acme/shop/actions/variables/NULLUS_SCAN_SEVERITY":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}

// 정책은 Actions 시크릿이 아니라 변수(vars)로 싣는다. 워크플로가 ${{ vars.X }} 로
// 읽으므로 시크릿에 넣으면 빈 값이 되고, 로그에서도 정책이 *** 로 가려진다.
func TestSetPipelineVariable_CreatesRepositoryVariable(t *testing.T) {
	srv, seen := variablesServer(t, http.StatusCreated)

	err := NewClient(srv.URL, "tok").SetPipelineVariable(context.Background(), "acme/shop",
		port.ScanSeverityVariable, "HIGH,CRITICAL")
	require.NoError(t, err)

	require.Len(t, *seen, 1)
	assert.Equal(t, http.MethodPost, (*seen)[0].method)
	assert.Equal(t, map[string]any{"name": "NULLUS_SCAN_SEVERITY", "value": "HIGH,CRITICAL"}, (*seen)[0].body)
}

// 이미 있으면 GitHub 은 409 를 준다. 그때 갱신한다 — 정책 변경의 대부분이 이 경로다.
func TestSetPipelineVariable_UpdatesExistingVariable(t *testing.T) {
	srv, seen := variablesServer(t, http.StatusConflict)

	err := NewClient(srv.URL, "tok").SetPipelineVariable(context.Background(), "acme/shop",
		port.ScanSeverityVariable, "CRITICAL")
	require.NoError(t, err)

	require.Len(t, *seen, 2)
	assert.Equal(t, http.MethodPatch, (*seen)[1].method)
	assert.Equal(t, map[string]any{"name": "NULLUS_SCAN_SEVERITY", "value": "CRITICAL"}, (*seen)[1].body)
}

func TestSetPipelineVariable_PropagatesOtherFailures(t *testing.T) {
	srv, _ := variablesServer(t, http.StatusForbidden)

	err := NewClient(srv.URL, "tok").SetPipelineVariable(context.Background(), "acme/shop",
		port.ScanSeverityVariable, "CRITICAL")
	assert.Error(t, err, "권한 부족을 성공으로 넘기면 정책이 반영됐다고 착각한다")
}
