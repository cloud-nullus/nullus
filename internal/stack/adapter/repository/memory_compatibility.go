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

// 매트릭스가 선언하는 버전. 전부 domain 이 소유한다 — 설치가 쓰는 값과 같아야
// 하기 때문이다.
//
// 처음에는 외부 프로젝트 Narwhal(dasomel/narwhal)의 VERSIONS.md 를 기준선으로
// 삼았다. 그런데 Nullus 의 설치 경로가 독자적으로 올라가면서 두 값이 갈라졌고,
// 화면은 Argo CD 6.8.0 을 "검증된 조합" 이라 안내하는데 클러스터에는 7.7.16 이
// 서 있었다. Harbor/Nexus 만 domain 을 참조해 그 둘만 어긋나지 않았다.
//
// 이제 기준선은 우리가 실제로 설치하는 차트다. 버전을 올릴 때는 domain 상수
// 하나만 고치면 설치·매트릭스가 함께 따라온다.
// (고정: TestChartVersionsMatchCompatibilityMatrix)
const (
	// GitLab 차트 하나가 소스 저장소·CI·레지스트리를 겸한다.
	baselineGitLabHelmVersion = domain.GitLabChartVersion
	baselineGitLabAppVersion  = domain.GitLabAppVersion

	baselineHarborHelmVersion = domain.HarborChartVersion
	baselineHarborAppVersion  = domain.HarborAppVersion
	baselineNexusHelmVersion  = domain.NexusChartVersion
	baselineNexusAppVersion   = domain.NexusAppVersion

	baselineGiteaHelmVersion = domain.GiteaChartVersion
	baselineGiteaAppVersion  = domain.GiteaAppVersion

	baselineJenkinsHelmVersion = domain.JenkinsChartVersion
	baselineJenkinsAppVersion  = domain.JenkinsAppVersion

	baselineMinIOHelmVersion = domain.MinIOChartVersion
	baselineMinIOAppVersion  = domain.MinIOAppVersion

	baselineArgoCDHelmVersion = domain.ArgoCDChartVersion
	baselineArgoCDAppVersion  = domain.ArgoCDAppVersion

	baselinePrometheusHelmVer = domain.PrometheusChartVersion
	baselinePrometheusAppVer  = domain.PrometheusAppVersion

	baselineGrafanaHelmVersion = domain.GrafanaChartVersion
	baselineGrafanaAppVersion  = domain.GrafanaAppVersion

	baselineLokiHelmVersion = "2.10.3"
	baselineLokiAppVersion  = "v2.4.2"

	baselineTempoHelmVersion = domain.TempoChartVersion
	baselineTempoAppVersion  = domain.TempoAppVersion

	baselineOTelCollectorHelmVersion = domain.OTelCollectorChartVersion
	baselineOTelCollectorAppVersion  = domain.OTelCollectorAppVersion

	// 최저 K8s 라인은 차트에서 끌어올 수 없는 편집 값이다.
	baselineMinK8sPlatform = "1.27" // GitLab, GitHub, Harbor
	baselineMinK8sWorkload = "1.26" // MinIO, Argo CD, Prometheus, Grafana
)

// defaultCompatibilityMatrices returns the three canonical compatibility matrices.
// Per-tool MinK8sVersion / ArchSupport / Tier values mirror what migrations
// 000041_compat_tool_fields / 000042_seed_narwhal_compat_refresh /
// 000062_compat_baseline_matches_install apply to the persisted rows. If this
// function drifts from the DB state, the Pre-Deploy Gate will produce different
// verdicts in tests vs. real deployments.
func defaultCompatibilityMatrices() []*domain.CompatibilityMatrix {
	return []*domain.CompatibilityMatrix{
		{
			// Gitea 는 소스 저장소만 담당한다 — GitLab 처럼 CI 도 레지스트리도
			// 겸하지 않으므로 Jenkins 와 Harbor 를 따로 세운다.
			//
			// Jenkins 차트 버전은 임의로 내릴 수 없다: Gitea multibranch 스캔에
			// 쓰는 gitea 플러그인이 Jenkins 2.528.3 이상을 요구한다.
			ID:     "gitea-jenkins-argocd-v1",
			Name:   "Gitea + Jenkins + Argo CD",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         baselineMinK8sWorkload,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "Gitea", HelmVersion: baselineGiteaHelmVersion, AppVersion: baselineGiteaAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"ci_platform":              {Name: "Jenkins", HelmVersion: baselineJenkinsHelmVersion, AppVersion: baselineJenkinsAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"container_registry":       {Name: "Harbor", HelmVersion: baselineHarborHelmVersion, AppVersion: baselineHarborAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"storage_backend":          {Name: "MinIO", HelmVersion: baselineMinIOHelmVersion, AppVersion: baselineMinIOAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: baselineArgoCDHelmVersion, AppVersion: baselineArgoCDAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: baselinePrometheusHelmVer, AppVersion: baselinePrometheusAppVer, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: baselineGrafanaHelmVersion, AppVersion: baselineGrafanaAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
			},
		},
		{
			// 8Gi 노드용 최소 구성. 도구 셋은 gitea-jenkins-argocd-v1 에서
			// 레지스트리·오브젝트 스토리지·모니터링을 뺀 것이라 버전 판정 기준은
			// 같다 — 같은 baseline 상수를 쓴다.
			ID:     "gitea-jenkins-argocd-lite-v1",
			Name:   "Gitea + Jenkins + Argo CD (Lite)",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         baselineMinK8sWorkload,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository": {Name: "Gitea", HelmVersion: baselineGiteaHelmVersion, AppVersion: baselineGiteaAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"ci_platform":       {Name: "Jenkins", HelmVersion: baselineJenkinsHelmVersion, AppVersion: baselineJenkinsAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"container_registry": {Name: "Harbor", HelmVersion: baselineHarborHelmVersion, AppVersion: baselineHarborAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"cd_tool":           {Name: "Argo CD", HelmVersion: baselineArgoCDHelmVersion, AppVersion: baselineArgoCDAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
			},
		},
		{
			ID:     "gitlab-allinone-v1",
			Name:   "GitLab All-in-One",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         baselineMinK8sPlatform,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitLab CE", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"ci_platform":              {Name: "GitLab CI", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"container_registry":       {Name: "GitLab Registry", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"storage_backend":          {Name: "MinIO", HelmVersion: baselineMinIOHelmVersion, AppVersion: baselineMinIOAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: baselineArgoCDHelmVersion, AppVersion: baselineArgoCDAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: baselinePrometheusHelmVer, AppVersion: baselinePrometheusAppVer, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: baselineGrafanaHelmVersion, AppVersion: baselineGrafanaAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
			},
		},
		{
			ID:     "gitlab-argocd-v1",
			Name:   "GitLab + Argo CD",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         baselineMinK8sPlatform,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitLab CE", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"ci_platform":              {Name: "GitLab CI", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"container_registry":       {Name: "GitLab Registry", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"storage_backend":          {Name: "MinIO", HelmVersion: baselineMinIOHelmVersion, AppVersion: baselineMinIOAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: baselineArgoCDHelmVersion, AppVersion: baselineArgoCDAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: baselinePrometheusHelmVer, AppVersion: baselinePrometheusAppVer, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: baselineGrafanaHelmVersion, AppVersion: baselineGrafanaAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
			},
		},
		{
			ID:     "gitlab-harbor-v1",
			Name:   "GitLab + Harbor",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         baselineMinK8sPlatform,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitLab CE", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"ci_platform":              {Name: "GitLab CI", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"container_registry":       {Name: "Harbor", HelmVersion: baselineHarborHelmVersion, AppVersion: baselineHarborAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierBeta},
				"storage_backend":          {Name: "MinIO", HelmVersion: baselineMinIOHelmVersion, AppVersion: baselineMinIOAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: baselineArgoCDHelmVersion, AppVersion: baselineArgoCDAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: baselinePrometheusHelmVer, AppVersion: baselinePrometheusAppVer, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: baselineGrafanaHelmVersion, AppVersion: baselineGrafanaAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
			},
		},
		{
			ID:     "gitlab-nexus-v1",
			Name:   "GitLab + Nexus",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         baselineMinK8sPlatform,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitLab CE", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"ci_platform":              {Name: "GitLab CI", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"container_registry":       {Name: "Nexus", HelmVersion: baselineNexusHelmVersion, AppVersion: baselineNexusAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierBeta},
				"package_registry":         {Name: "Nexus", HelmVersion: baselineNexusHelmVersion, AppVersion: baselineNexusAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierBeta},
				"storage_backend":          {Name: "MinIO", HelmVersion: baselineMinIOHelmVersion, AppVersion: baselineMinIOAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: baselineArgoCDHelmVersion, AppVersion: baselineArgoCDAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: baselinePrometheusHelmVer, AppVersion: baselinePrometheusAppVer, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: baselineGrafanaHelmVersion, AppVersion: baselineGrafanaAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
			},
		},
		{
			ID:     "gitlab-argocd-otel-v1",
			Name:   "GitLab + Argo CD + OpenTelemetry",
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         baselineMinK8sPlatform,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository":        {Name: "GitLab CE", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"ci_platform":              {Name: "GitLab CI", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"container_registry":       {Name: "GitLab Registry", HelmVersion: baselineGitLabHelmVersion, AppVersion: baselineGitLabAppVersion, MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archAMD64Only, Tier: domain.ToolTierStable},
				"storage_backend":          {Name: "MinIO", HelmVersion: baselineMinIOHelmVersion, AppVersion: baselineMinIOAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: baselineArgoCDHelmVersion, AppVersion: baselineArgoCDAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: baselinePrometheusHelmVer, AppVersion: baselinePrometheusAppVer, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: baselineGrafanaHelmVersion, AppVersion: baselineGrafanaAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"log_search":               {Name: "Loki", HelmVersion: baselineLokiHelmVersion, AppVersion: baselineLokiAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"trace_layer":              {Name: "Tempo", HelmVersion: baselineTempoHelmVersion, AppVersion: baselineTempoAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"agent":                    {Name: "OpenTelemetry Collector", HelmVersion: baselineOTelCollectorHelmVersion, AppVersion: baselineOTelCollectorAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierBeta},
			},
		},
		{
			ID:   "github-argocd-v1",
			Name: "GitHub + Argo CD",
			// 검증을 마쳤다. 이 조합은 클러스터 밖(GitHub·Actions·GHCR)과 안
			// (Argo CD·모니터링)이 갈려 있어 나머지 매트릭스와 실패 지점이
			// 다르다 — 파이프라인 프로비저닝이 GitHub 어댑터를 타고
			// (internal/cicd/adapter/github/), 이미지가 GHCR 로 나간다.
			// 그 경로까지 확인한 뒤 verified 로 올렸다.
			Status: "verified",
			Kubernetes: domain.KubernetesCompat{
				Min:         baselineMinK8sPlatform,
				Max:         "1.35",
				Recommended: "1.35",
			},
			Tools: map[string]domain.ToolVersion{
				"source_repository": {Name: "GitHub", HelmVersion: "external", AppVersion: "external", MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"ci_platform":       {Name: "GitHub Actions", HelmVersion: "external", AppVersion: "external", MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				// GHCR 은 클러스터 밖이라 아키텍처 제약이 없다 — Harbor 의 amd64 전용
				// 제약을 물려받으면 arm64 클러스터에서 호환성 검사가 잘못 막는다.
				"container_registry":       {Name: "GHCR", HelmVersion: "external", AppVersion: "external", MinK8sVersion: baselineMinK8sPlatform, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"storage_backend":          {Name: "MinIO", HelmVersion: baselineMinIOHelmVersion, AppVersion: baselineMinIOAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"cd_tool":                  {Name: "Argo CD", HelmVersion: baselineArgoCDHelmVersion, AppVersion: baselineArgoCDAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_collection":    {Name: "Prometheus", HelmVersion: baselinePrometheusHelmVer, AppVersion: baselinePrometheusAppVer, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
				"monitoring_visualization": {Name: "Grafana", HelmVersion: baselineGrafanaHelmVersion, AppVersion: baselineGrafanaAppVersion, MinK8sVersion: baselineMinK8sWorkload, ArchSupport: archMulti, Tier: domain.ToolTierStable},
			},
		},
	}
}
