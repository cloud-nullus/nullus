-- 000087_seed_gitlab_argocd_sonarqube_template.up.sql
-- SonarQube(SAST)를 기본으로 고른 Golden Path 템플릿을 시드한다.
--
-- gitlab-argocd-v1 에 SonarQube 를 더한 구성이다. SonarQube 는 스택의 공유 PostgreSQL
-- 안에 전용 DB 를 만들어 쓴다. Keycloak 로그인(SAML)은 뒤따르는 변경에서 붙는다.
--
-- 버전은 설치 경로가 쓰는 값이다 (internal/stack/domain/connection.go). GitLab 은 000048 이
-- gitlab-argocd-v1 에 한 것과 같이 arm64 를 허용한다.
-- (고정: TestSeedMigration_GitLabArgoCDSonarQube_MatchesMemory, TestChartVersionsMatchCompatibilityMatrix)
--
-- idempotent 하게 작성해 재실행해도 같은 상태로 수렴한다.

-- ------------------------------------------------------------
-- 1. golden_path_templates
-- ------------------------------------------------------------

INSERT INTO golden_path_templates (
    id, name, description, tools, estimated_install_time, recommended_use_case, min_resources, planning_profile
) VALUES
(
    'gitlab-argocd-sonarqube-v1',
    'GitLab + Argo CD + SonarQube',
    'GitLab CI와 GitLab Registry, Argo CD GitOps 구성에 SonarQube 정적 분석 서버를 함께 설치합니다. SonarQube는 스택의 PostgreSQL을 함께 씁니다.',
    $$[
      {"category":"source_repository","name":"GitLab CE","helm_version":"8.7.2","app_version":"v17.7.0"},
      {"category":"ci_platform","name":"GitLab CI","helm_version":"8.7.2","app_version":"v17.7.0"},
      {"category":"container_registry","name":"GitLab Registry","helm_version":"8.7.2","app_version":"v17.7.0"},
      {"category":"storage_backend","name":"MinIO","helm_version":"5.4.0","app_version":"RELEASE.2024-12-18T13-15-44Z"},
      {"category":"cd_tool","name":"Argo CD","helm_version":"7.7.16","app_version":"v2.13.3"},
      {"category":"monitoring_collection","name":"Prometheus","helm_version":"69.3.0","app_version":"v3.1.0"},
      {"category":"monitoring_visualization","name":"Grafana","helm_version":"8.9.0","app_version":"11.5.1"},
      {"category":"sast","name":"SonarQube","helm_version":"2026.5.1002","app_version":"26.9.0.129388"}
    ]$$::jsonb,
    130,
    '코드 품질·보안 정적 분석을 함께 운영하려는 GitOps 조직',
    '12 vCPU / 24Gi RAM / 150Gi Storage',
    'standard'
)
ON CONFLICT (id) DO UPDATE SET
    name                   = EXCLUDED.name,
    description            = EXCLUDED.description,
    tools                  = EXCLUDED.tools,
    estimated_install_time = EXCLUDED.estimated_install_time,
    recommended_use_case   = EXCLUDED.recommended_use_case,
    min_resources          = EXCLUDED.min_resources,
    planning_profile       = EXCLUDED.planning_profile,
    updated_at             = NOW();

-- ------------------------------------------------------------
-- 2. compatibility_matrices
--
-- 템플릿만 넣고 매트릭스를 빠뜨리면 Pre-Deploy Gate 가 판정할 근거가 없다.
-- ------------------------------------------------------------

INSERT INTO compatibility_matrices (
    id, name, status, k8s_min, k8s_max, k8s_recommended, tools
) VALUES
(
    'gitlab-argocd-sonarqube-v1',
    'GitLab + Argo CD + SonarQube',
    'verified',
    '1.27', '1.35', '1.35',
    $$
    {
      "source_repository":        {"Name": "GitLab CE",       "HelmVersion": "8.7.2",  "AppVersion": "v17.7.0",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "ci_platform":              {"Name": "GitLab CI",       "HelmVersion": "8.7.2",  "AppVersion": "v17.7.0",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "container_registry":       {"Name": "GitLab Registry", "HelmVersion": "8.7.2",  "AppVersion": "v17.7.0",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "storage_backend":          {"Name": "MinIO",           "HelmVersion": "5.4.0",  "AppVersion": "RELEASE.2024-12-18T13-15-44Z",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "cd_tool":                  {"Name": "Argo CD",         "HelmVersion": "7.7.16", "AppVersion": "v2.13.3",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "monitoring_collection":    {"Name": "Prometheus",      "HelmVersion": "69.3.0", "AppVersion": "v3.1.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "monitoring_visualization": {"Name": "Grafana",         "HelmVersion": "8.9.0",  "AppVersion": "11.5.1",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "image_scanner":            {"Name": "Trivy",           "HelmVersion": "0.26.0", "AppVersion": "0.74.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "beta"},
      "sast":                     {"Name": "SonarQube",       "HelmVersion": "2026.5.1002", "AppVersion": "26.9.0.129388",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "beta"}
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
