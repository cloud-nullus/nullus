package repository

import (
	"context"
	"fmt"
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
// An optional stackID filters results to pipelines linked to that stack.
func (r *MemoryPipelineRepository) List(_ context.Context, orgID string, stackID ...string) ([]*domain.Pipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	filterStack := len(stackID) > 0 && stackID[0] != ""
	var result []*domain.Pipeline
	for _, p := range r.pipelines {
		if p.OrgID != orgID {
			continue
		}
		if filterStack && p.StackID != stackID[0] {
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
