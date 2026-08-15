package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// MemoryTemplateRepository is an in-memory implementation of port.TemplateRepository
// with canonical Golden Path templates.
type MemoryTemplateRepository struct {
	templates map[string]*domain.Template
}

// NewMemoryTemplateRepository constructs a MemoryTemplateRepository with canonical
// Golden Path templates pre-loaded.
func NewMemoryTemplateRepository() *MemoryTemplateRepository {
	repo := &MemoryTemplateRepository{
		templates: make(map[string]*domain.Template),
	}
	for _, t := range goldenPathTemplates() {
		// 프로파일을 적지 않은 템플릿은 standard 로 읽는다. 시드 표 일곱 줄에
		// 같은 값을 되풀이해 적는 대신 여기 한 곳에서 채운다.
		t.PlanningProfile = domain.NormalizePlanningProfile(t.PlanningProfile)
		repo.templates[t.ID] = t
	}
	return repo
}

// GetByID returns the template with the given ID.
func (r *MemoryTemplateRepository) GetByID(_ context.Context, id string) (*domain.Template, error) {
	t, ok := r.templates[id]
	if !ok {
		return nil, fmt.Errorf("template %q not found", id)
	}
	return t, nil
}

// List returns all available templates.
func (r *MemoryTemplateRepository) List(_ context.Context) ([]*domain.Template, error) {
	result := make([]*domain.Template, 0, len(r.templates))
	for _, t := range r.templates {
		result = append(result, t)
	}
	return result, nil
}

func (r *MemoryTemplateRepository) Create(_ context.Context, template *domain.Template) error {
	if _, exists := r.templates[template.ID]; exists {
		return fmt.Errorf("template %q already exists", template.ID)
	}
	r.templates[template.ID] = template
	return nil
}

func (r *MemoryTemplateRepository) Update(_ context.Context, template *domain.Template) error {
	if _, exists := r.templates[template.ID]; !exists {
		return fmt.Errorf("template %q not found", template.ID)
	}
	r.templates[template.ID] = template
	return nil
}

func (r *MemoryTemplateRepository) Delete(_ context.Context, id string) error {
	if _, exists := r.templates[id]; !exists {
		return fmt.Errorf("template %q not found", id)
	}
	delete(r.templates, id)
	return nil
}

// goldenPathTemplates returns the canonical Golden Path templates.
func goldenPathTemplates() []*domain.Template {
	// Templates surface the tested matrix snapshot. The install runtime still
	// reads chart versions from stack_helm_step_configs.
	return []*domain.Template{
		{
			ID:                   "empty-template-v1",
			Name:                 "Empty Template",
			Description:          "Start from an empty stack configuration with every tool left unselected.",
			Tools:                []domain.ToolConfig{},
			EstimatedInstallTime: 5 * time.Minute,
			RecommendedUseCase:   "Blank starting point for custom stack composition",
			MinResources:         "Decide resources after selecting the tools you need",
		},
		{
			ID:          "gitlab-allinone-v1",
			Name:        "GitLab All-in-One",
			Description: "GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "GitLab CE", HelmVersion: "9.5.1", AppVersion: "18.5.1"},
				{Category: "ci_platform", Name: "GitLab CI", HelmVersion: "9.5.1", AppVersion: "18.5.1"},
				{Category: "container_registry", Name: "GitLab Registry", HelmVersion: "9.5.1", AppVersion: "18.5.1"},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.2.0", AppVersion: "RELEASE.2024-08-03T04-33-23Z"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "6.8.0", AppVersion: "v2.8.3"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "v2.54.1"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.1.0"},
			},
			EstimatedInstallTime: 90 * time.Minute,
			RecommendedUseCase:   "중견기업, 단일 플랫폼 선호",
			MinResources:         "8 vCPU / 16Gi RAM / 100Gi Storage",
		},
		{
			ID:          "gitlab-argocd-v1",
			Name:        "GitLab + Argo CD",
			Description: "GitLab CI와 GitLab Registry를 사용하고 Argo CD로 GitOps 패턴을 강화한 구성입니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "GitLab CE", HelmVersion: "9.5.1", AppVersion: "18.5.1"},
				{Category: "ci_platform", Name: "GitLab CI", HelmVersion: "9.5.1", AppVersion: "18.5.1"},
				{Category: "container_registry", Name: "GitLab Registry", HelmVersion: "9.5.1", AppVersion: "18.5.1"},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.2.0", AppVersion: "RELEASE.2024-08-03T04-33-23Z"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "6.8.0", AppVersion: "v2.8.3"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "v2.54.1"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.1.0"},
			},
			EstimatedInstallTime: 120 * time.Minute,
			RecommendedUseCase:   "GitOps 중심 조직",
			MinResources:         "10 vCPU / 20Gi RAM / 130Gi Storage",
		},
		{
			// Gitea 는 소스 저장소만 담당한다 — GitLab 처럼 CI 도 레지스트리도
			// 겸하지 않으므로 Jenkins 와 Harbor 를 따로 세운다. 그래서 도구 수는
			// 많지만 개별 도구가 가벼워 GitLab 올인원과 자원 요구가 비슷하다.
			ID:          "gitea-jenkins-argocd-v1",
			Name:        "Gitea + Jenkins + Argo CD",
			Description: "가벼운 Git 서버(Gitea)와 익숙한 CI(Jenkins)를 Argo CD GitOps에 연결합니다. 이미 Jenkins를 운영 중이거나 GitLab이 부담스러운 조직에 맞습니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "Gitea", HelmVersion: domain.GiteaChartVersion, AppVersion: domain.GiteaAppVersion},
				{Category: "ci_platform", Name: "Jenkins", HelmVersion: domain.JenkinsChartVersion, AppVersion: domain.JenkinsAppVersion},
				{Category: "container_registry", Name: "Harbor", HelmVersion: domain.HarborChartVersion, AppVersion: domain.HarborAppVersion},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: domain.MinIOChartVersion, AppVersion: domain.MinIOAppVersion},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: domain.ArgoCDChartVersion, AppVersion: domain.ArgoCDAppVersion},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: domain.PrometheusChartVersion, AppVersion: domain.PrometheusAppVersion},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: domain.GrafanaChartVersion, AppVersion: domain.GrafanaAppVersion},
			},
			EstimatedInstallTime: 100 * time.Minute,
			RecommendedUseCase:   "기존 Jenkins 운영 조직, 경량 Git 서버 선호",
			MinResources:         "10 vCPU / 20Gi RAM / 120Gi Storage",
		},
		{
			ID:          "gitlab-harbor-v1",
			Name:        "GitLab + Harbor",
			Description: "소스코드와 CI는 GitLab, 컨테이너 이미지는 Harbor에 둡니다. 이미지 보관을 GitLab에서 떼어내 레지스트리를 독립적으로 운영하려는 구성입니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "GitLab CE", HelmVersion: "9.5.1", AppVersion: "18.5.1"},
				{Category: "ci_platform", Name: "GitLab CI", HelmVersion: "9.5.1", AppVersion: "18.5.1"},
				{Category: "container_registry", Name: "Harbor", HelmVersion: domain.HarborChartVersion, AppVersion: domain.HarborAppVersion},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.2.0", AppVersion: "RELEASE.2024-08-03T04-33-23Z"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "6.8.0", AppVersion: "v2.8.3"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "v2.54.1"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.1.0"},
			},
			EstimatedInstallTime: 110 * time.Minute,
			RecommendedUseCase:   "이미지 스캔·복제 등 레지스트리 기능이 필요한 조직",
			MinResources:         "10 vCPU / 20Gi RAM / 140Gi Storage",
		},
		{
			ID:          "gitlab-nexus-v1",
			Name:        "GitLab + Nexus",
			Description: "컨테이너 이미지와 Maven/npm 패키지를 Nexus 한 곳에 모읍니다. 빌드 산출물이 이미지만이 아닌 조직에 맞습니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "GitLab CE", HelmVersion: "9.5.1", AppVersion: "18.5.1"},
				{Category: "ci_platform", Name: "GitLab CI", HelmVersion: "9.5.1", AppVersion: "18.5.1"},
				{Category: "container_registry", Name: "Nexus", HelmVersion: domain.NexusChartVersion, AppVersion: domain.NexusAppVersion},
				{Category: "package_registry", Name: "Nexus", HelmVersion: domain.NexusChartVersion, AppVersion: domain.NexusAppVersion},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.2.0", AppVersion: "RELEASE.2024-08-03T04-33-23Z"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "6.8.0", AppVersion: "v2.8.3"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "v2.54.1"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.1.0"},
			},
			EstimatedInstallTime: 110 * time.Minute,
			RecommendedUseCase:   "이미지와 라이브러리 패키지를 함께 관리하는 조직",
			MinResources:         "10 vCPU / 22Gi RAM / 140Gi Storage",
		},
		{
			ID:          "gitlab-argocd-otel-v1",
			Name:        "GitLab + Argo CD + OpenTelemetry",
			Description: "GitLab CI 와 Argo CD 위에 OpenTelemetry Collector 를 세웁니다. 애플리케이션은 OTLP 한 곳으로만 보내고, 수집기가 추적은 Tempo, 메트릭은 Prometheus, 로그는 Loki 로 나눠 보냅니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "GitLab CE", HelmVersion: domain.GitLabChartVersion, AppVersion: domain.GitLabAppVersion},
				{Category: "ci_platform", Name: "GitLab CI", HelmVersion: domain.GitLabChartVersion, AppVersion: domain.GitLabAppVersion},
				{Category: "container_registry", Name: "GitLab Registry", HelmVersion: domain.GitLabChartVersion, AppVersion: domain.GitLabAppVersion},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: domain.MinIOChartVersion, AppVersion: domain.MinIOAppVersion},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: domain.ArgoCDChartVersion, AppVersion: domain.ArgoCDAppVersion},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: domain.PrometheusChartVersion, AppVersion: domain.PrometheusAppVersion},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: domain.GrafanaChartVersion, AppVersion: domain.GrafanaAppVersion},
				{Category: "log_search", Name: "Loki", HelmVersion: "2.10.3", AppVersion: "v2.4.2"},
				{Category: "trace_layer", Name: "Tempo", HelmVersion: domain.TempoChartVersion, AppVersion: domain.TempoAppVersion},
				// 수집기는 추적 저장소가 아니라 그 앞에 서는 수집 계층이다.
				// 화면의 Observability > Agent 칸에 대응한다.
				{Category: "agent", Name: "OpenTelemetry Collector", HelmVersion: domain.OTelCollectorChartVersion, AppVersion: domain.OTelCollectorAppVersion},
			},
			EstimatedInstallTime: 130 * time.Minute,
			RecommendedUseCase:   "추적·메트릭·로그를 한 수집기로 모으려는 조직",
			MinResources:         "12 vCPU / 24Gi RAM / 150Gi Storage",
		},
		{
			// 8Gi 노드 하나에 들어가는 스택.
			//
			// 예산의 대부분은 템플릿이 고르지 않는 것들이 먼저 가져간다 —
			// cert-manager 세 파드에 1.5Gi, PostgreSQL 2Gi, OpenBao·ESO·
			// metrics-server·게이트웨이가 1Gi. 남는 2Gi 남짓에 들어가는 조합만
			// 여기 담는다. GitLab(4.5Gi)·Prometheus(계산된 벡터가 5개 컴포넌트에
			// 그대로 실려 5Gi)·Nexus(1.5Gi 고정)·Harbor 는 들어가지 않는다.
			//
			// 레지스트리는 뺄 수 없다. 없으면 파이프라인을 만드는 순간
			// registry.ResolverFor 가 "이미지 레지스트리를 결정할 수 없습니다" 로
			// 막아, 스택은 서는데 아무것도 배포할 수 없는 템플릿이 된다(실측 확인).
			// 그 예산에 들어가는 레지스트리는 Harbor 하나뿐이다 — core·registry 만
			// 요청을 잡아 512Mi 로 서고, Nexus 는 JVM 고정으로 1.5Gi 를 요구한다.
			ID:          "gitea-jenkins-argocd-lite-v1",
			Name:        "Gitea + Jenkins + Argo CD (Lite)",
			Description: "8Gi 노드 하나에 올라가는 최소 구성입니다. Gitea·Jenkins·Argo CD 만 세우고 레지스트리와 모니터링은 뺐습니다. 로컬 검증이나 소규모 PoC 에 맞습니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "Gitea", HelmVersion: domain.GiteaChartVersion, AppVersion: domain.GiteaAppVersion},
				{Category: "ci_platform", Name: "Jenkins", HelmVersion: domain.JenkinsChartVersion, AppVersion: domain.JenkinsAppVersion},
				{Category: "container_registry", Name: "Harbor", HelmVersion: domain.HarborChartVersion, AppVersion: domain.HarborAppVersion},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: domain.ArgoCDChartVersion, AppVersion: domain.ArgoCDAppVersion},
			},
			EstimatedInstallTime: 40 * time.Minute,
			RecommendedUseCase:   "단일 노드 로컬 검증, 소규모 PoC",
			MinResources:         "4 vCPU / 8Gi RAM / 60Gi Storage",
			PlanningProfile:      domain.PlanningProfileLocal,
		},
		{
			ID:   "github-argocd-v1",
			Name: "GitHub + Argo CD",
			// 이미지 레지스트리도 GitHub 쪽(GHCR)이다. GitHub 호스티드 러너는
			// 클러스터 내부 Harbor 에 닿을 수 없어 push 가 불가능하다.
			Description: "GitHub·GitHub Actions·GHCR 을 외부 서비스로 사용하고, 클러스터 내에는 Argo CD + 모니터링만 설치합니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "GitHub", HelmVersion: "external", AppVersion: "external"},
				{Category: "ci_platform", Name: "GitHub Actions", HelmVersion: "external", AppVersion: "external"},
				{Category: "container_registry", Name: "GHCR", HelmVersion: "external", AppVersion: "external"},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.3.0", AppVersion: "2024.11.7"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "3.1.0"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.4.0"},
			},
			// Harbor 가 빠져 설치 시간과 최소 자원이 함께 줄었다.
			EstimatedInstallTime: 45 * time.Minute,
			RecommendedUseCase:   "GitHub 사용 조직",
			MinResources:         "4 vCPU / 8Gi RAM / 50Gi Storage",
		},
	}
}
