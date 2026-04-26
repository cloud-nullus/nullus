package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
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

// Create persists a new matrix. Returns ErrCompatibilityMatrixExists when the
// id collides with an existing row. F8-Phase5 admin CRUD endpoint backing.
func (r *MemoryCompatibilityRepository) Create(_ context.Context, m *domain.CompatibilityMatrix) error {
	if m == nil || strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("compatibility matrix: id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.matrices[m.ID]; ok {
		return port.ErrCompatibilityMatrixExists
	}
	cp := *m
	r.matrices[m.ID] = &cp
	return nil
}

// Update replaces every mutable field on the matrix identified by m.ID.
func (r *MemoryCompatibilityRepository) Update(_ context.Context, m *domain.CompatibilityMatrix) error {
	if m == nil || strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("compatibility matrix: id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.matrices[m.ID]; !ok {
		return port.ErrCompatibilityMatrixNotFound
	}
	cp := *m
	r.matrices[m.ID] = &cp
	return nil
}

// Delete is idempotent — missing row returns nil. Handlers that want strict
// 404 semantics should GetByID beforehand.
func (r *MemoryCompatibilityRepository) Delete(_ context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("compatibility matrix: id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.matrices, id)
	return nil
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

// archAMD64Only is the arch profile for tools that do not publish official arm64 images
// (e.g. Harbor, GitLab CE/CI/Registry as of 2026-Q1). Kept as a package-level slice so
// callers don't accidentally mutate the defaults.
var archAMD64Only = []string{domain.ArchAMD64}

// archMulti is the arch profile for tools that support both amd64 and arm64.
var archMulti = []string{domain.ArchAMD64, domain.ArchARM64}

// Narwhal baseline version pins. These mirror the DB state after migration
// 000042_seed_narwhal_compat_refresh and are sourced from
// docs/20_아키텍처/Narwhal_호환성_Seed_Sources.md. When bumping a version here,
// update both the refresh migration and the Narwhal sources doc in the same
// commit so the three layers (DB / in-memory / docs) never drift.
const (
	narwhalGitLabHelmVersion  = "9.5.1"
	narwhalGitLabAppVersion   = "18.5.1"
	narwhalHarborHelmVersion  = "1.15.0"
	narwhalHarborAppVersion   = "2.11.0"
	narwhalMinIOHelmVersion   = "5.2.0"
	narwhalMinIOAppVersion    = "RELEASE.2024-08-03T04-33-23Z"
	narwhalArgoCDHelmVersion  = "6.8.0"
	narwhalArgoCDAppVersion   = "v2.8.3"
	narwhalPrometheusHelmVer  = "67.0.0"
	narwhalPrometheusAppVer   = "v2.54.1"
	narwhalGrafanaHelmVersion = "8.5.0"
	narwhalGrafanaAppVersion  = "11.1.0"
	narwhalBaseMinK8sPlatform = "1.27" // GitLab, GitHub, Harbor
	narwhalBaseMinK8sWorkload = "1.26" // MinIO, Argo CD, Prometheus, Grafana
)

// defaultCompatibilityMatrices returns the three canonical compatibility matrices.
// Per-tool MinK8sVersion / ArchSupport / Tier values mirror what migrations
// 000041_compat_tool_fields and 000042_seed_narwhal_compat_refresh apply to the
// persisted rows. If this function drifts from the DB state, the Pre-Deploy Gate
// will produce different verdicts in tests vs. real deployments.
func defaultCompatibilityMatrices() []*domain.CompatibilityMatrix {
	return []*domain.CompatibilityMatrix{
		{
			ID:     "gitlab-allinone-v1",
			Name:   "GitLab All-in-One",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         narwhalBaseMinK8sPlatform,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitLab CE", HelmVersion: narwhalGitLabHelmVersion, AppVersion: narwhalGitLabAppVersion, MinK8sVersion: narwhalBaseMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"ci_platform":              {Name: "GitLab CI", HelmVersion: narwhalGitLabHelmVersion, AppVersion: narwhalGitLabAppVersion, MinK8sVersion: narwhalBaseMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"container_registry":       {Name: "GitLab Registry", HelmVersion: narwhalGitLabHelmVersion, AppVersion: narwhalGitLabAppVersion, MinK8sVersion: narwhalBaseMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"storage_backend":          {Name: "MinIO", HelmVersion: narwhalMinIOHelmVersion, AppVersion: narwhalMinIOAppVersion, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: narwhalArgoCDHelmVersion, AppVersion: narwhalArgoCDAppVersion, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: narwhalPrometheusHelmVer, AppVersion: narwhalPrometheusAppVer, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: narwhalGrafanaHelmVersion, AppVersion: narwhalGrafanaAppVersion, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
			},
		},
		{
			ID:     "gitlab-argocd-v1",
			Name:   "GitLab + Argo CD",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         narwhalBaseMinK8sPlatform,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitLab CE", HelmVersion: narwhalGitLabHelmVersion, AppVersion: narwhalGitLabAppVersion, MinK8sVersion: narwhalBaseMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"ci_platform":              {Name: "GitLab CI", HelmVersion: narwhalGitLabHelmVersion, AppVersion: narwhalGitLabAppVersion, MinK8sVersion: narwhalBaseMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"container_registry":       {Name: "GitLab Registry", HelmVersion: narwhalGitLabHelmVersion, AppVersion: narwhalGitLabAppVersion, MinK8sVersion: narwhalBaseMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"storage_backend":          {Name: "MinIO", HelmVersion: narwhalMinIOHelmVersion, AppVersion: narwhalMinIOAppVersion, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: narwhalArgoCDHelmVersion, AppVersion: narwhalArgoCDAppVersion, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: narwhalPrometheusHelmVer, AppVersion: narwhalPrometheusAppVer, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: narwhalGrafanaHelmVersion, AppVersion: narwhalGrafanaAppVersion, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
			},
		},
		{
			ID:     "github-argocd-v1",
			Name:   "GitHub + Argo CD",
			Status: "untested",
			Kubernetes: domain.KubernetesCompat{
				Min:         narwhalBaseMinK8sPlatform,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitHub", HelmVersion: "external", AppVersion: "external", MinK8sVersion: narwhalBaseMinK8sPlatform, ArchSupport: archMulti, Tier: domain.ToolTierBeta},
				"ci_platform":              {Name: "GitHub Actions", HelmVersion: "external", AppVersion: "external", MinK8sVersion: narwhalBaseMinK8sPlatform, ArchSupport: archMulti, Tier: domain.ToolTierBeta},
				"container_registry":       {Name: "Harbor", HelmVersion: narwhalHarborHelmVersion, AppVersion: narwhalHarborAppVersion, MinK8sVersion: narwhalBaseMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierBeta},
				"storage_backend":          {Name: "MinIO", HelmVersion: narwhalMinIOHelmVersion, AppVersion: narwhalMinIOAppVersion, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierBeta},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: narwhalArgoCDHelmVersion, AppVersion: narwhalArgoCDAppVersion, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierBeta},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: narwhalPrometheusHelmVer, AppVersion: narwhalPrometheusAppVer, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierBeta},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: narwhalGrafanaHelmVersion, AppVersion: narwhalGrafanaAppVersion, MinK8sVersion: narwhalBaseMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierBeta},
			},
		},
	}
}
