-- 000063_seed_otel_stack_template.up.sql
-- OpenTelemetry 수집기를 포함한 Golden Path 템플릿을 시드한다.
--
-- 이 템플릿은 CI/CD(GitLab + Argo CD) 위에 관측 계층을 얹는다. 기존 템플릿과
-- 다른 점은 애플리케이션 텔레메트리를 받는 지점이 하나로 모인다는 것이다 —
-- OpenTelemetry Collector 가 OTLP 를 받아 추적은 Tempo, 메트릭은 Prometheus,
-- 로그는 Loki 로 나눠 보낸다.
--
-- 버전은 설치 경로가 쓰는 값과 같아야 한다 (internal/stack/domain/connection.go).
-- 갈라지면 화면이 안내한 버전과 실제 설치된 버전이 달라진다.
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
    'gitlab-argocd-otel-v1',
    'GitLab + Argo CD + OpenTelemetry',
    'GitLab CI 와 Argo CD 위에 OpenTelemetry Collector 를 세웁니다. 애플리케이션은 OTLP 한 곳으로만 보내고, 수집기가 추적은 Tempo, 메트릭은 Prometheus, 로그는 Loki 로 나눠 보냅니다.',
    $$[
      {"category":"source_repository","name":"GitLab CE","helm_version":"8.7.2","app_version":"v17.7.0"},
      {"category":"ci_platform","name":"GitLab CI","helm_version":"8.7.2","app_version":"v17.7.0"},
      {"category":"container_registry","name":"GitLab Registry","helm_version":"8.7.2","app_version":"v17.7.0"},
      {"category":"storage_backend","name":"MinIO","helm_version":"5.4.0","app_version":"RELEASE.2024-12-18T13-15-44Z"},
      {"category":"cd_tool","name":"Argo CD","helm_version":"7.7.16","app_version":"v2.13.3"},
      {"category":"monitoring_collection","name":"Prometheus","helm_version":"69.3.0","app_version":"v3.1.0"},
      {"category":"monitoring_visualization","name":"Grafana","helm_version":"8.9.0","app_version":"11.5.1"},
      {"category":"log_search","name":"Loki","helm_version":"2.10.3","app_version":"v2.4.2"},
      {"category":"trace_layer","name":"Tempo","helm_version":"1.18.1","app_version":"2.7.0"},
      {"category":"agent","name":"OpenTelemetry Collector","helm_version":"0.75.0","app_version":"0.90.0"}
    ]$$::jsonb,
    130,
    '추적·메트릭·로그를 한 수집기로 모으려는 조직',
    '12 vCPU / 24Gi RAM / 150Gi Storage'
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
    'gitlab-argocd-otel-v1',
    'GitLab + Argo CD + OpenTelemetry',
    'verified',
    '1.27', '1.35', '1.35',
    $$
    {
      "source_repository":        {"Name": "GitLab CE",  "HelmVersion": "8.7.2",  "AppVersion": "v17.7.0",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "stable"},
      "ci_platform":              {"Name": "GitLab CI",  "HelmVersion": "8.7.2",  "AppVersion": "v17.7.0",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "stable"},
      "container_registry":       {"Name": "GitLab Registry", "HelmVersion": "8.7.2", "AppVersion": "v17.7.0",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "stable"},
      "storage_backend":          {"Name": "MinIO",      "HelmVersion": "5.4.0",  "AppVersion": "RELEASE.2024-12-18T13-15-44Z",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "cd_tool":                  {"Name": "Argo CD",    "HelmVersion": "7.7.16", "AppVersion": "v2.13.3",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "monitoring_collection":    {"Name": "Prometheus", "HelmVersion": "69.3.0", "AppVersion": "v3.1.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "monitoring_visualization": {"Name": "Grafana",    "HelmVersion": "8.9.0",  "AppVersion": "11.5.1",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "log_search":               {"Name": "Loki",       "HelmVersion": "2.10.3", "AppVersion": "v2.4.2",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "trace_layer":              {"Name": "Tempo",      "HelmVersion": "1.18.1", "AppVersion": "2.7.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "agent":                    {"Name": "OpenTelemetry Collector", "HelmVersion": "0.75.0", "AppVersion": "0.90.0",
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
