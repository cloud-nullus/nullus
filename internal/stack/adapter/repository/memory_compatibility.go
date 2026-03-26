package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// MemoryCompatibilityRepository is an in-memory implementation of port.CompatibilityRepository
// with three pre-loaded compatibility matrices.
type MemoryCompatibilityRepository struct {
	mu       sync.RWMutex
	matrices map[string]*domain.CompatibilityMatrix
}

// NewMemoryCompatibilityRepository constructs a MemoryCompatibilityRepository with three
// canonical compatibility matrices pre-loaded.
func NewMemoryCompatibilityRepository() *MemoryCompatibilityRepository {
	r := &MemoryCompatibilityRepository{
		matrices: make(map[string]*domain.CompatibilityMatrix),
	}
	for _, m := range defaultCompatibilityMatrices() {
		r.matrices[m.ID] = m
	}
	return r
}

// GetAll returns all compatibility matrices.
func (r *MemoryCompatibilityRepository) GetAll(_ context.Context) ([]*domain.CompatibilityMatrix, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.CompatibilityMatrix, 0, len(r.matrices))
	for _, m := range r.matrices {
		cp := *m
		result = append(result, &cp)
	}
	return result, nil
}

// GetByID returns the compatibility matrix with the given ID.
func (r *MemoryCompatibilityRepository) GetByID(_ context.Context, id string) (*domain.CompatibilityMatrix, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.matrices[id]
	if !ok {
		return nil, fmt.Errorf("compatibility matrix %q not found", id)
	}
	cp := *m
	return &cp, nil
}

// Validate finds the best matching matrix for the given tool map (tool category -> tool name).
// Returns the first matrix whose tools all match. Returns an error if no match is found.
func (r *MemoryCompatibilityRepository) Validate(_ context.Context, tools map[string]string) (*domain.CompatibilityMatrix, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, m := range r.matrices {
		if matchesMatrix(m, tools) {
			cp := *m
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("no compatible matrix found for the given tool combination")
}

// matchesMatrix returns true when every tool in the request is present in the matrix.
func matchesMatrix(m *domain.CompatibilityMatrix, tools map[string]string) bool {
	for category, name := range tools {
		tv, ok := m.Tools[category]
		if !ok {
			return false
		}
		if !strings.EqualFold(tv.Name, name) {
			return false
		}
	}
	return true
}

// defaultCompatibilityMatrices returns the three canonical compatibility matrices.
func defaultCompatibilityMatrices() []*domain.CompatibilityMatrix {
	return []*domain.CompatibilityMatrix{
		{
			ID:     "gitlab-allinone-v1",
			Name:   "GitLab All-in-One",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         "1.27",
				Max:         "1.32",
				Recommended: "1.30",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitLab CE", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				"ci_platform":              {Name: "GitLab CI", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				"container_registry":       {Name: "GitLab Registry", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				"storage_backend":          {Name: "MinIO", HelmVersion: "5.3.0", AppVersion: "2024.11.7"},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "3.1.0"},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.4.0"},
			},
		},
		{
			ID:     "gitlab-argocd-v1",
			Name:   "GitLab + Argo CD",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         "1.27",
				Max:         "1.32",
				Recommended: "1.30",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitLab CE", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				"ci_platform":              {Name: "GitLab CI", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				"container_registry":       {Name: "GitLab Registry", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				"storage_backend":          {Name: "MinIO", HelmVersion: "5.3.0", AppVersion: "2024.11.7"},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "3.1.0"},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.4.0"},
			},
		},
		{
			ID:     "github-argocd-v1",
			Name:   "GitHub + Argo CD",
			Status: "untested",
			Kubernetes: domain.KubernetesCompat{
				Min:         "1.27",
				Max:         "1.32",
				Recommended: "1.29",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitHub", HelmVersion: "external", AppVersion: "external"},
				"ci_platform":              {Name: "GitHub Actions", HelmVersion: "external", AppVersion: "external"},
				"container_registry":       {Name: "Harbor", HelmVersion: "1.14.0", AppVersion: "2.11.0"},
				"storage_backend":          {Name: "MinIO", HelmVersion: "5.3.0", AppVersion: "2024.11.7"},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "3.1.0"},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.4.0"},
			},
		},
	}
}
