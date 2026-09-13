package gitlab

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestClient_ImplementsPipelineVariableWriter(t *testing.T) {
	var _ port.PipelineVariableWriter = (*Client)(nil)
}

// 정책 변수는 프로젝트 CI/CD 변수다 — .gitlab-ci.yml 의 variables 보다 앞서므로
// 이미 스캐폴딩한 파이프라인도 다시 커밋하지 않고 정책이 바뀐다.
//
// 가리지 않는다. GitLab 은 8자 미만 값(true·allow)의 마스킹 등록을 통째로 거부한다.
func TestSetPipelineVariable_RegistersUnmaskedProjectVariable(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	})

	err := NewClient(srv.URL, "tok").SetPipelineVariable(context.Background(),
		"nullus/shop", port.ScanIgnoreUnfixedVariable, "true")
	require.NoError(t, err)

	require.Len(t, *recorded, 1)
	req := (*recorded)[0]
	assert.Equal(t, "/api/v4/projects/nullus%2Fshop/variables", req.Path)
	assert.Equal(t, "NULLUS_SCAN_IGNORE_UNFIXED", req.Body["key"])
	assert.Equal(t, "true", req.Body["value"])
	assert.Equal(t, false, req.Body["masked"])
}
