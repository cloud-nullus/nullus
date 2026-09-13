package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestBuildReader_ImplementsCIBuildReader(t *testing.T) {
	var _ port.CIBuildReader = (*BuildReader)(nil)
}

// GitLab CI 스택의 파이프라인도 실행 기록과 스캔 게이트 판정을 남겨야 한다.
// 읽는 경로가 Jenkins 에만 있어서, 같은 스캔 단계가 돌아도 GitLab 쪽은 기록이
// 영원히 비어 있었다.
func TestBuildReader_ListBuilds_MapsPipelinesAndJobs(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		switch r.URL.EscapedPath() {
		case "/api/v4/projects/nullus%2Fshop/pipelines":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 901, "iid": 12, "status": "failed",
					"created_at": "2026-09-01T10:00:00Z", "updated_at": "2026-09-01T10:05:00Z"},
				{"id": 902, "iid": 13, "status": "running",
					"created_at": "2026-09-01T11:00:00Z", "updated_at": "2026-09-01T11:01:00Z"},
			})
		case "/api/v4/projects/nullus%2Fshop/pipelines/901/jobs":
			// GitLab 은 최근 잡을 먼저 준다.
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 5003, "name": "deploy", "stage": "deploy", "status": "skipped",
					"started_at": nil, "duration": nil},
				{"id": 5002, "name": "image-scan", "stage": "image-scan", "status": "failed",
					"started_at": "2026-09-01T10:02:00Z", "duration": 40.5},
				{"id": 5001, "name": "build", "stage": "build", "status": "success",
					"started_at": "2026-09-01T10:00:10Z", "duration": 100.0},
			})
		case "/api/v4/projects/nullus%2Fshop/pipelines/902/jobs":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 5011, "name": "build", "stage": "build", "status": "running",
					"started_at": "2026-09-01T11:00:05Z", "duration": nil},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	reader := NewBuildReader(NewClient(srv.URL, "tok"), "nullus")
	builds, err := reader.ListBuilds(context.Background(), "shop", "main", 5)
	require.NoError(t, err)
	require.Len(t, builds, 2)

	done := builds[0]
	assert.Equal(t, 12, done.Number, "화면의 실행 번호는 프로젝트 안의 번호(iid)다")
	assert.Equal(t, "901", done.ID, "잡·산출물 조회는 전역 id 로 한다")
	assert.Equal(t, "FAILURE", done.Result)
	assert.False(t, done.Building)
	assert.Equal(t, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), done.StartedAt.UTC())
	assert.Equal(t, 5*time.Minute, done.Duration)

	require.Len(t, done.Stages, 3)
	assert.Equal(t, []string{"build", "image-scan", "deploy"},
		[]string{done.Stages[0].Name, done.Stages[1].Name, done.Stages[2].Name},
		"단계는 실행 순서대로다")
	assert.Equal(t, port.CIStageSuccess, done.Stages[0].Status)
	assert.Equal(t, port.CIStageFailed, done.Stages[1].Status)
	assert.Equal(t, port.CIStageSkipped, done.Stages[2].Status)
	assert.Equal(t, "5002", done.Stages[1].ID)
	assert.Equal(t, 40500*time.Millisecond, done.Stages[1].Duration)
	assert.True(t, done.Stages[2].StartedAt.IsZero(), "돌지 않은 잡에 시각을 지어내지 않는다")

	running := builds[1]
	assert.True(t, running.Building)
	assert.Empty(t, running.Result)
	assert.Zero(t, running.Duration, "실행 중에는 걸린 시간이 없다")

	var listQuery string
	for _, req := range *recorded {
		if strings.HasSuffix(req.Path, "/pipelines") {
			listQuery = req.Query
		}
	}
	assert.Contains(t, listQuery, "ref=main")
	assert.Contains(t, listQuery, "per_page=5")
}

func TestBuildReader_ListBuilds_CanceledIsAborted(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		switch r.URL.EscapedPath() {
		case "/api/v4/projects/nullus%2Fshop/pipelines":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 1, "iid": 1, "status": "canceled",
					"created_at": "2026-09-01T10:00:00Z", "updated_at": "2026-09-01T10:01:00Z"},
			})
		default:
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		}
	})

	builds, err := NewBuildReader(NewClient(srv.URL, "tok"), "nullus").
		ListBuilds(context.Background(), "shop", "main", 5)
	require.NoError(t, err)
	require.Len(t, builds, 1)
	assert.Equal(t, "ABORTED", builds[0].Result)
	assert.False(t, builds[0].Building)
}

// 프로젝트가 아직 없으면(프로비저닝 전) 실행 기록이 없는 것이다. 오류가 아니다.
func TestBuildReader_ListBuilds_MissingProjectIsEmpty(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusNotFound)
	})

	builds, err := NewBuildReader(NewClient(srv.URL, "tok"), "nullus").
		ListBuilds(context.Background(), "shop", "main", 5)
	require.NoError(t, err)
	assert.Empty(t, builds)
}
