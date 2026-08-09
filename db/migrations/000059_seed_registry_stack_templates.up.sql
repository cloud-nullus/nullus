-- 000059_seed_registry_stack_templates.up.sql
-- 독립 레지스트리(Harbor / Nexus) 기반 Golden Path 템플릿 2종을 시드한다.
--
-- 인메모리 저장소(internal/stack/adapter/repository/memory_template.go,
-- memory_compatibility.go)에만 있던 구성이라 실제 배포에서는 목록에 뜨지 않았다.
-- 두 저장소가 갈라지면 개발 환경에서는 보이는 템플릿이 운영에서는 없는 상태가 된다.
--
-- 버전은 설치 경로(internal/stack/domain/connection.go 의 HarborChartVersion /
-- NexusChartVersion)와 같아야 한다. 갈라지면 화면이 안내한 버전과 실제 설치된
-- 버전이 달라진다. (고정: TestChartVersionsMatchCompatibilityMatrix)
--
-- idempotent 하게 작성해 재실행해도 같은 상태로 수렴한다.

-- ------------------------------------------------------------
-- 1. golden_path_templates
-- ------------------------------------------------------------

INSERT INTO golden_path_templates (
    id, name, description, tools, estimated_install_time, recommended_use_case, min_resources
) VALUES
(
    'gitlab-harbor-v1',
    'GitLab + Harbor',
    '소스코드와 CI는 GitLab, 컨테이너 이미지는 Harbor에 둡니다. 이미지 보관을 GitLab에서 떼어내 레지스트리를 독립적으로 운영하려는 구성입니다.',
    $$[
      {"category":"source_repository","name":"GitLab CE","helm_version":"9.5.1","app_version":"18.5.1"},
      {"category":"ci_platform","name":"GitLab CI","helm_version":"9.5.1","app_version":"18.5.1"},
      {"category":"container_registry","name":"Harbor","helm_version":"1.15.0","app_version":"2.11.0"},
      {"category":"storage_backend","name":"MinIO","helm_version":"5.2.0","app_version":"RELEASE.2024-08-03T04-33-23Z"},
      {"category":"cd_tool","name":"Argo CD","helm_version":"6.8.0","app_version":"v2.8.3"},
      {"category":"monitoring_collection","name":"Prometheus","helm_version":"67.0.0","app_version":"v2.54.1"},
      {"category":"monitoring_visualization","name":"Grafana","helm_version":"8.5.0","app_version":"11.1.0"}
    ]$$::jsonb,
    110,
    '이미지 스캔·복제 등 레지스트리 기능이 필요한 조직',
    '10 vCPU / 20Gi RAM / 140Gi Storage'
),
(
    'gitlab-nexus-v1',
    'GitLab + Nexus',
    '컨테이너 이미지와 Maven/npm 패키지를 Nexus 한 곳에 모읍니다. 빌드 산출물이 이미지만이 아닌 조직에 맞습니다.',
    $$[
      {"category":"source_repository","name":"GitLab CE","helm_version":"9.5.1","app_version":"18.5.1"},
      {"category":"ci_platform","name":"GitLab CI","helm_version":"9.5.1","app_version":"18.5.1"},
      {"category":"container_registry","name":"Nexus","helm_version":"64.2.0","app_version":"3.64.0"},
      {"category":"package_registry","name":"Nexus","helm_version":"64.2.0","app_version":"3.64.0"},
      {"category":"storage_backend","name":"MinIO","helm_version":"5.2.0","app_version":"RELEASE.2024-08-03T04-33-23Z"},
      {"category":"cd_tool","name":"Argo CD","helm_version":"6.8.0","app_version":"v2.8.3"},
      {"category":"monitoring_collection","name":"Prometheus","helm_version":"67.0.0","app_version":"v2.54.1"},
      {"category":"monitoring_visualization","name":"Grafana","helm_version":"8.5.0","app_version":"11.1.0"}
    ]$$::jsonb,
    110,
    '이미지와 라이브러리 패키지를 함께 관리하는 조직',
    '10 vCPU / 22Gi RAM / 140Gi Storage'
)
ON CONFLICT (id) DO UPDATE SET
    name                   = EXCLUDED.name,
    description            = EXCLUDED.description,
    tools                  = EXCLUDED.tools,
    estimated_install_time = EXCLUDED.estimated_install_time,
    recommended_use_case   = EXCLUDED.recommended_use_case,
    min_resources          = EXCLUDED.min_resources,
    updated_at             = NOW();

-- ------------------------------------------------------------
-- 2. compatibility_matrices
--
-- 템플릿만 넣고 매트릭스를 빠뜨리면 Pre-Deploy Gate 가 판정할 근거가 없어
-- 설치 직전에 막힌다.
-- ------------------------------------------------------------

INSERT INTO compatibility_matrices (
    id, name, status, k8s_min, k8s_max, k8s_recommended, tools
) VALUES
(
    'gitlab-harbor-v1',
    'GitLab + Harbor',
    'verified',
    '1.27', '1.35', '1.35',
    $$
    {
      "source_repository":        {"Name": "GitLab CE",  "HelmVersion": "9.5.1",  "AppVersion": "18.5.1",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "stable"},
      "ci_platform":              {"Name": "GitLab CI",  "HelmVersion": "9.5.1",  "AppVersion": "18.5.1",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "stable"},
      "container_registry":       {"Name": "Harbor",     "HelmVersion": "1.15.0", "AppVersion": "2.11.0",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "beta"},
      "storage_backend":          {"Name": "MinIO",      "HelmVersion": "5.2.0",  "AppVersion": "RELEASE.2024-08-03T04-33-23Z",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "cd_tool":                  {"Name": "Argo CD",    "HelmVersion": "6.8.0",  "AppVersion": "v2.8.3",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "monitoring_collection":    {"Name": "Prometheus", "HelmVersion": "67.0.0", "AppVersion": "v2.54.1",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "monitoring_visualization": {"Name": "Grafana",    "HelmVersion": "8.5.0",  "AppVersion": "11.1.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"}
    }
    $$::jsonb
),
(
    'gitlab-nexus-v1',
    'GitLab + Nexus',
    'verified',
    '1.27', '1.35', '1.35',
    $$
    {
      "source_repository":        {"Name": "GitLab CE",  "HelmVersion": "9.5.1",  "AppVersion": "18.5.1",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "stable"},
      "ci_platform":              {"Name": "GitLab CI",  "HelmVersion": "9.5.1",  "AppVersion": "18.5.1",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "stable"},
      "container_registry":       {"Name": "Nexus",      "HelmVersion": "64.2.0", "AppVersion": "3.64.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "beta"},
      "package_registry":         {"Name": "Nexus",      "HelmVersion": "64.2.0", "AppVersion": "3.64.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "beta"},
      "storage_backend":          {"Name": "MinIO",      "HelmVersion": "5.2.0",  "AppVersion": "RELEASE.2024-08-03T04-33-23Z",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "cd_tool":                  {"Name": "Argo CD",    "HelmVersion": "6.8.0",  "AppVersion": "v2.8.3",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "monitoring_collection":    {"Name": "Prometheus", "HelmVersion": "67.0.0", "AppVersion": "v2.54.1",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "monitoring_visualization": {"Name": "Grafana",    "HelmVersion": "8.5.0",  "AppVersion": "11.1.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"}
    }
    $$::jsonb
)
ON CONFLICT (id) DO UPDATE SET
    name            = EXCLUDED.name,
    status          = EXCLUDED.status,
    k8s_min         = EXCLUDED.k8s_min,
    k8s_max         = EXCLUDED.k8s_max,
    k8s_recommended = EXCLUDED.k8s_recommended,
    tools           = EXCLUDED.tools,
    updated_at      = NOW();
