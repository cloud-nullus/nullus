package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// MemoryPipelineRepository is an in-memory implementation of port.PipelineRepository.
type MemoryPipelineRepository struct {
	mu        sync.RWMutex
	pipelines map[string]*domain.Pipeline
}

// NewMemoryPipelineRepository constructs an empty MemoryPipelineRepository.
func NewMemoryPipelineRepository() *MemoryPipelineRepository {
	return &MemoryPipelineRepository{
		pipelines: make(map[string]*domain.Pipeline),
	}
}

// Create stores a new pipeline.
func (r *MemoryPipelineRepository) Create(_ context.Context, p *domain.Pipeline) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.pipelines[p.ID]; ok {
		return fmt.Errorf("pipeline %q already exists", p.ID)
	}
	r.pipelines[p.ID] = clonePipeline(p)
	return nil
}

// GetByID retrieves a pipeline by its ID.
func (r *MemoryPipelineRepository) GetByID(_ context.Context, id string) (*domain.Pipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.pipelines[id]
	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", id)
	}
	return clonePipeline(p), nil
}

// List returns all pipelines for an organization.
// ListWithStack 은 스택에 묶인 파이프라인을 조직과 무관하게 돌려준다(주기 동기화용).
func (r *MemoryPipelineRepository) ListWithStack(_ context.Context) ([]*domain.Pipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Pipeline
	for _, p := range r.pipelines {
		if strings.TrimSpace(p.StackID) == "" {
			continue
		}
		result = append(result, clonePipeline(p))
	}
	return result, nil
}

func (r *MemoryPipelineRepository) List(_ context.Context, orgID string) ([]*domain.Pipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Pipeline
	for _, p := range r.pipelines {
		if p.OrgID != orgID {
			continue
		}
		result = append(result, clonePipeline(p))
	}
	return result, nil
}

// ListByStackID returns all pipelines linked to a specific stack.
func (r *MemoryPipelineRepository) ListByStackID(_ context.Context, stackID string) ([]*domain.Pipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Pipeline
	for _, p := range r.pipelines {
		if p.StackID == stackID {
			result = append(result, clonePipeline(p))
		}
	}
	return result, nil
}

// Update persists changes to an existing pipeline.
func (r *MemoryPipelineRepository) Update(_ context.Context, p *domain.Pipeline) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.pipelines[p.ID]; !ok {
		return fmt.Errorf("pipeline %q not found", p.ID)
	}
	r.pipelines[p.ID] = clonePipeline(p)
	return nil
}

// Delete removes a pipeline by ID. No-op when the pipeline does not exist so
// callers can idempotently clean up.
func (r *MemoryPipelineRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pipelines, id)
	return nil
}

// MemoryDeploymentRepository is an in-memory implementation of port.DeploymentRepository.
type MemoryDeploymentRepository struct {
	mu          sync.RWMutex
	deployments map[string]*domain.Deployment
}

// NewMemoryDeploymentRepository constructs an empty MemoryDeploymentRepository.
func NewMemoryDeploymentRepository() *MemoryDeploymentRepository {
	return &MemoryDeploymentRepository{
		deployments: make(map[string]*domain.Deployment),
	}
}

// Create stores a new deployment.
func (r *MemoryDeploymentRepository) Create(_ context.Context, d *domain.Deployment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.deployments[d.ID]; ok {
		return fmt.Errorf("deployment %q already exists", d.ID)
	}
	r.deployments[d.ID] = cloneDeployment(d)
	return nil
}

// GetByID retrieves a deployment by its ID.
func (r *MemoryDeploymentRepository) GetByID(_ context.Context, id string) (*domain.Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.deployments[id]
	if !ok {
		return nil, fmt.Errorf("deployment %q not found", id)
	}
	return cloneDeployment(d), nil
}

// ListByPipelineID returns all deployments for a given pipeline.
func (r *MemoryDeploymentRepository) ListByPipelineID(_ context.Context, pipelineID string) ([]*domain.Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Deployment
	for _, d := range r.deployments {
		if d.PipelineID == pipelineID {
			result = append(result, cloneDeployment(d))
		}
	}
	return result, nil
}

// Update persists changes to an existing deployment.
func (r *MemoryDeploymentRepository) Update(_ context.Context, d *domain.Deployment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.deployments[d.ID]; !ok {
		return fmt.Errorf("deployment %q not found", d.ID)
	}
	r.deployments[d.ID] = cloneDeployment(d)
	return nil
}

func clonePipeline(p *domain.Pipeline) *domain.Pipeline {
	if p == nil {
		return nil
	}
	cp := *p
	// 슬라이스를 공유하면 호출부가 고친 값이 저장된 기록까지 바꾼다.
	// DB 저장소에서는 일어나지 않는 일이라 테스트가 거짓 초록이 된다.
	if p.Stages != nil {
		cp.Stages = append([]string(nil), p.Stages...)
	}
	return &cp
}

func cloneDeployment(d *domain.Deployment) *domain.Deployment {
	if d == nil {
		return nil
	}
	cp := *d
	if d.CompletedAt != nil {
		completedAt := *d.CompletedAt
		cp.CompletedAt = &completedAt
	}
	return &cp
}
