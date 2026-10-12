package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 취소된 잡은 실패가 아니라 취소다. 실패로 옮기면 스캔 판정이 error 로 남는다.
func TestBuildReader_ListBuilds_CancelledJobIsCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/shop/actions/runs":
			_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []map[string]any{
				{"id": 1, "run_number": 1, "status": "completed", "conclusion": "cancelled",
					"run_started_at": "2026-09-01T10:00:00Z", "updated_at": "2026-09-01T10:01:00Z"},
			}})
		case "/repos/acme/shop/actions/runs/1/jobs":
			_ = json.NewEncoder(w).Encode(map[string]any{"jobs": []map[string]any{
				{"id": 2, "name": "image-scan", "status": "completed", "conclusion": "cancelled",
					"started_at": "2026-09-01T10:00:10Z", "completed_at": "2026-09-01T10:00:20Z"},
			}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	builds, err := NewBuildReader(NewClient(srv.URL, "tok"), "acme").
		ListBuilds(context.Background(), "shop", "main", 5)
	require.NoError(t, err)
	require.Len(t, builds, 1)
	require.Len(t, builds[0].Stages, 1)
	assert.Equal(t, port.CIStageCanceled, builds[0].Stages[0].Status)
}
