package prometheus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardRepository_ReturnsCachedResultWithinTTL(t *testing.T) {
	var reqCount atomic.Int64
	var value atomic.Int64
	value.Store(10)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		v := strconv.FormatInt(value.Load(), 10)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []any{
					map[string]any{"metric": map[string]string{}, "value": []any{1234567890, v}},
				},
			},
		}))
	}))
	defer srv.Close()

	repo := NewDashboardRepository(NewClient(srv.URL))
	repo.cacheTTL = time.Second

	first, err := repo.GetDashboard(context.Background())
	require.NoError(t, err)
	require.Equal(t, 10.0, first.ClusterMetrics.CPUUsage)

	value.Store(99)
	second, err := repo.GetDashboard(context.Background())
	require.NoError(t, err)
	require.Equal(t, 10.0, second.ClusterMetrics.CPUUsage)
	require.Equal(t, int64(5), reqCount.Load())
}

func TestDashboardRepository_RefreshesAfterTTL(t *testing.T) {
	var value atomic.Int64
	value.Store(10)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := strconv.FormatInt(value.Load(), 10)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []any{
					map[string]any{"metric": map[string]string{}, "value": []any{1234567890, v}},
				},
			},
		}))
	}))
	defer srv.Close()

	repo := NewDashboardRepository(NewClient(srv.URL))
	repo.cacheTTL = 20 * time.Millisecond

	first, err := repo.GetDashboard(context.Background())
	require.NoError(t, err)
	require.Equal(t, 10.0, first.ClusterMetrics.CPUUsage)

	time.Sleep(30 * time.Millisecond)
	value.Store(20)

	second, err := repo.GetDashboard(context.Background())
	require.NoError(t, err)
	require.Equal(t, 20.0, second.ClusterMetrics.CPUUsage)
}

func TestDashboardRepository_GracefulFallbackWhenPrometheusDown(t *testing.T) {
	repo := NewDashboardRepository(NewClient("http://127.0.0.1:1"))

	dashboard, err := repo.GetDashboard(context.Background())
	require.NoError(t, err)
	require.NotNil(t, dashboard)
	assert.Equal(t, 0.0, dashboard.ClusterMetrics.CPUUsage)
	assert.Equal(t, 0.0, dashboard.ClusterMetrics.MemoryUsage)
	assert.Equal(t, 0.0, dashboard.ClusterMetrics.StorageUsage)
	assert.Equal(t, 0, dashboard.ClusterMetrics.PodCount)
	assert.Equal(t, 0, dashboard.PipelineMetrics.TotalRuns)
	assert.Empty(t, dashboard.ToolHealthList)
}

func TestDashboardRepository_ConcurrentAccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []any{
					map[string]any{"metric": map[string]string{}, "value": []any{1234567890, "10"}},
				},
			},
		}))
	}))
	defer srv.Close()

	repo := NewDashboardRepository(NewClient(srv.URL))
	repo.cacheTTL = time.Second

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := repo.GetDashboard(context.Background())
			require.NoError(t, err)
			require.NotNil(t, d)
		}()
	}
	wg.Wait()
}
