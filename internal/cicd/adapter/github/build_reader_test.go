package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestBuildReader_ImplementsCIBuildReader(t *testing.T) {
	var _ port.CIBuildReader = (*BuildReader)(nil)
}

// GitHub Actions 스택의 파이프라인도 실행 기록과 스캔 게이트 판정을 남겨야 한다.
func TestBuildReader_ListBuilds_MapsRunsAndJobs(t *testing.T) {
	var runsQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/shop/actions/runs":
			runsQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_count": 2,
				"workflow_runs": []map[string]any{
					{"id": 7001, "run_number": 4, "status": "completed", "conclusion": "failure",
						"run_started_at": "2026-09-01T10:00:00Z", "updated_at": "2026-09-01T10:04:00Z"},
					{"id": 7002, "run_number": 5, "status": "in_progress", "conclusion": nil,
						"run_started_at": "2026-09-01T11:00:00Z", "updated_at": "2026-09-01T11:00:30Z"},
				},
			})
		case "/repos/acme/shop/actions/runs/7001/jobs":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jobs": []map[string]any{
					{"id": 81, "name": "build", "status": "completed", "conclusion": "success",
						"started_at": "2026-09-01T10:00:05Z", "completed_at": "2026-09-01T10:01:05Z"},
					{"id": 82, "name": "image-scan", "status": "completed", "conclusion": "failure",
						"started_at": "2026-09-01T10:01:10Z", "completed_at": "2026-09-01T10:02:00Z"},
					{"id": 83, "name": "deploy", "status": "completed", "conclusion": "skipped",
						"started_at": "2026-09-01T10:02:01Z", "completed_at": "2026-09-01T10:02:01Z"},
				},
			})
		case "/repos/acme/shop/actions/runs/7002/jobs":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jobs": []map[string]any{
					{"id": 91, "name": "build", "status": "in_progress", "conclusion": nil,
						"started_at": "2026-09-01T11:00:05Z", "completed_at": nil},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	reader := NewBuildReader(NewClient(srv.URL, "tok"), "acme")
	builds, err := reader.ListBuilds(context.Background(), "shop", "main", 5)
	require.NoError(t, err)
	require.Len(t, builds, 2)

	done := builds[0]
	assert.Equal(t, 4, done.Number)
	assert.Equal(t, "7001", done.ID, "산출물 조회는 run id 로 한다")
	assert.Equal(t, "FAILURE", done.Result)
	assert.False(t, done.Building)
	assert.Equal(t, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), done.StartedAt.UTC())
	assert.Equal(t, 4*time.Minute, done.Duration)

	require.Len(t, done.Stages, 3)
	assert.Equal(t, "image-scan", done.Stages[1].Name)
	assert.Equal(t, port.CIStageSuccess, done.Stages[0].Status)
	assert.Equal(t, port.CIStageFailed, done.Stages[1].Status)
	assert.Equal(t, port.CIStageSkipped, done.Stages[2].Status)
	assert.Equal(t, 50*time.Second, done.Stages[1].Duration)

	running := builds[1]
	assert.True(t, running.Building)
	assert.Empty(t, running.Result)
	require.Len(t, running.Stages, 1)
	assert.Equal(t, port.CIStageRunning, running.Stages[0].Status)
	assert.Zero(t, running.Stages[0].Duration)

	assert.Contains(t, runsQuery, "branch=main")
	assert.Contains(t, runsQuery, "per_page=5")
}

// 시간 초과·시작 실패는 실패다. 모르는 값으로 흘리면 막힌 스캔이 판정 없이 사라진다.
func TestBuildReader_ListBuilds_TimedOutIsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/shop/actions/runs":
			_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []map[string]any{
				{"id": 1, "run_number": 1, "status": "completed", "conclusion": "timed_out",
					"run_started_at": "2026-09-01T10:00:00Z", "updated_at": "2026-09-01T10:30:00Z"},
			}})
		case "/repos/acme/shop/actions/runs/1/jobs":
			_ = json.NewEncoder(w).Encode(map[string]any{"jobs": []map[string]any{
				{"id": 2, "name": "image-scan", "status": "completed", "conclusion": "timed_out",
					"started_at": "2026-09-01T10:00:00Z", "completed_at": "2026-09-01T10:30:00Z"},
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
	assert.Equal(t, "FAILURE", builds[0].Result)
	require.Len(t, builds[0].Stages, 1)
	assert.Equal(t, port.CIStageFailed, builds[0].Stages[0].Status)
}

func TestBuildReader_ListBuilds_MissingRepoIsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	builds, err := NewBuildReader(NewClient(srv.URL, "tok"), "acme").
		ListBuilds(context.Background(), "shop", "main", 5)
	require.NoError(t, err)
	assert.Empty(t, builds)
}
