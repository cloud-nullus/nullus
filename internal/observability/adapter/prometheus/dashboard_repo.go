package prometheus

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/cloud-nullus/draft/internal/observability/domain"
)

const (
	queryCPUUsage     = "100 - (avg(rate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100)"
	queryMemoryUsage  = "(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100"
	queryStorageUsage = "(1 - node_filesystem_avail_bytes{mountpoint=\"/\"} / node_filesystem_size_bytes{mountpoint=\"/\"}) * 100"
	queryPodCount     = "count(kube_pod_info)"
	queryRunningPods  = "count(kube_pod_status_phase{phase=\"Running\"})"
)

type DashboardRepository struct {
	client    *Client
	cache     *domain.Dashboard
	cacheTime time.Time
	cacheTTL  time.Duration
	mu        sync.RWMutex
}

func NewDashboardRepository(client *Client) *DashboardRepository {
	return &DashboardRepository{
		client:   client,
		cacheTTL: 10 * time.Second,
	}
}

func (r *DashboardRepository) GetDashboard(ctx context.Context) (*domain.Dashboard, error) {
	r.mu.RLock()
	if r.isCacheFreshLocked() {
		cached := cloneDashboard(r.cache)
		r.mu.RUnlock()
		return cached, nil
	}
	r.mu.RUnlock()

	fresh, fetchErr := r.fetchDashboard(ctx)

	r.mu.Lock()
	r.cache = cloneDashboard(fresh)
	r.cacheTime = time.Now()
	result := cloneDashboard(r.cache)
	r.mu.Unlock()

	return result, fetchErr
}

func (r *DashboardRepository) isCacheFreshLocked() bool {
	return r.cache != nil && time.Since(r.cacheTime) < r.cacheTTL
}

func (r *DashboardRepository) fetchDashboard(ctx context.Context) (*domain.Dashboard, error) {
	dashboard := &domain.Dashboard{}
	var fetchErr error

	if value, err := r.queryFloat(ctx, queryCPUUsage); err == nil {
		dashboard.ClusterMetrics.CPUUsage = value
	} else {
		fetchErr = errors.Join(fetchErr, err)
	}

	if value, err := r.queryFloat(ctx, queryMemoryUsage); err == nil {
		dashboard.ClusterMetrics.MemoryUsage = value
	} else {
		fetchErr = errors.Join(fetchErr, err)
	}

	if value, err := r.queryFloat(ctx, queryStorageUsage); err == nil {
		dashboard.ClusterMetrics.StorageUsage = value
	} else {
		fetchErr = errors.Join(fetchErr, err)
	}

	totalPods, totalPodsErr := r.client.Query(ctx, queryPodCount)
	if totalPodsErr != nil {
		log.Printf("WARN prometheus query failed: query=%q err=%v", queryPodCount, totalPodsErr)
		fetchErr = errors.Join(fetchErr, totalPodsErr)
	}

	runningPods, runningPodsErr := r.client.Query(ctx, queryRunningPods)
	if runningPodsErr != nil {
		log.Printf("WARN prometheus query failed: query=%q err=%v", queryRunningPods, runningPodsErr)
		fetchErr = errors.Join(fetchErr, runningPodsErr)
	}

	if runningPodsErr == nil {
		dashboard.ClusterMetrics.PodCount = int(runningPods)
	} else if totalPodsErr == nil {
		dashboard.ClusterMetrics.PodCount = int(totalPods)
	}

	return dashboard, fetchErr
}

func (r *DashboardRepository) queryFloat(ctx context.Context, query string) (float64, error) {
	value, err := r.client.Query(ctx, query)
	if err != nil {
		log.Printf("WARN prometheus query failed: query=%q err=%v", query, err)
		return 0, err
	}
	return value, nil
}

func cloneDashboard(d *domain.Dashboard) *domain.Dashboard {
	if d == nil {
		return nil
	}
	copyValue := *d
	if d.ToolHealthList != nil {
		copyValue.ToolHealthList = make([]domain.ToolHealth, len(d.ToolHealthList))
		copy(copyValue.ToolHealthList, d.ToolHealthList)
	}
	return &copyValue
}
