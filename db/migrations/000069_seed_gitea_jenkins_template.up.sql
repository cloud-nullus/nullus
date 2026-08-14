-- 000069_seed_gitea_jenkins_template.up.sql
-- Gitea + Jenkins + Argo CD Golden Path 템플릿을 시드한다.
--
-- Gitea 는 소스 저장소만 담당한다 — GitLab 처럼 CI 도 레지스트리도 겸하지
-- 않으므로 Jenkins(CI)와 Harbor(레지스트리)를 따로 세운다.
--
-- Jenkins 차트 버전은 임의로 내릴 수 없다: Gitea multibranch 스캔에 쓰는
-- jenkinsci/gitea 플러그인이 Jenkins 2.528.3 이상을 요구한다.
-- (고정: TestJenkinsAppVersion_SatisfiesGiteaPluginMinimum)
--
-- 버전은 설치 경로가 쓰는 값과 같아야 한다 (internal/stack/domain/connection.go).
-- (고정: TestChartVersionsMatchCompatibilityMatrix)
--
-- idempotent 하게 작성해 재실행해도 같은 상태로 수렴한다.

-- ------------------------------------------------------------
-- 1. golden_path_templates
-- ------------------------------------------------------------

INSERT INTO golden_path_templates (
    id, name, description, tools, estimated_install_time, recommended_use_case, min_resources
) VALUES
(
    'gitea-jenkins-argocd-v1',
    'Gitea + Jenkins + Argo CD',
    '가벼운 Git 서버(Gitea)와 익숙한 CI(Jenkins)를 Argo CD GitOps에 연결합니다. 이미 Jenkins를 운영 중이거나 GitLab이 부담스러운 조직에 맞습니다.',
    $$[
      {"category":"source_repository","name":"Gitea","helm_version":"12.7.0","app_version":"1.27.0"},
      {"category":"ci_platform","name":"Jenkins","helm_version":"5.9.54","app_version":"2.568.2"},
      {"category":"container_registry","name":"Harbor","helm_version":"1.15.0","app_version":"2.11.0"},
      {"category":"storage_backend","name":"MinIO","helm_version":"5.4.0","app_version":"RELEASE.2024-12-18T13-15-44Z"},
      {"category":"cd_tool","name":"Argo CD","helm_version":"7.7.16","app_version":"v2.13.3"},
      {"category":"monitoring_collection","name":"Prometheus","helm_version":"69.3.0","app_version":"v3.1.0"},
      {"category":"monitoring_visualization","name":"Grafana","helm_version":"8.9.0","app_version":"11.5.1"}
    ]$$::jsonb,
    100,
    '기존 Jenkins 운영 조직, 경량 Git 서버 선호',
    '10 vCPU / 20Gi RAM / 120Gi Storage'
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
    'gitea-jenkins-argocd-v1',
    'Gitea + Jenkins + Argo CD',
    'verified',
    '1.26', '1.35', '1.35',
    $$
    {
      "source_repository":        {"Name": "Gitea",      "HelmVersion": "12.7.0", "AppVersion": "1.27.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "ci_platform":              {"Name": "Jenkins",    "HelmVersion": "5.9.54", "AppVersion": "2.568.2",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "container_registry":       {"Name": "Harbor",     "HelmVersion": "1.15.0", "AppVersion": "2.11.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "storage_backend":          {"Name": "MinIO",      "HelmVersion": "5.4.0",  "AppVersion": "RELEASE.2024-12-18T13-15-44Z",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "cd_tool":                  {"Name": "Argo CD",    "HelmVersion": "7.7.16", "AppVersion": "v2.13.3",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "monitoring_collection":    {"Name": "Prometheus", "HelmVersion": "69.3.0", "AppVersion": "v3.1.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "monitoring_visualization": {"Name": "Grafana",    "HelmVersion": "8.9.0",  "AppVersion": "11.5.1",
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
