-- 000042_seed_narwhal_compat_refresh.up.sql
-- Compatibility Matrix Task 2: Narwhal(dasomel/narwhal) VERSIONS.md 기반 Golden Path 3종
-- 조합의 canonical baseline을 확정한다. 이전 마이그레이션 체인
-- (000008 → 000024 → 000026 → 000033 → 000041) 의 결과를 idempotent 하게 재확정하여,
-- 운영 환경에서 seed row가 수동 수정되거나 중간 마이그레이션이 skip 된 경우에도
-- 단일 지점에서 Narwhal baseline v1으로 수렴시킬 수 있도록 한다.
--
-- 동일 마이그레이션이 golden_path_templates 에도 적용되어야 compatibility_matrices 와
-- Install Wizard 가 동일한 버전 pin 위에서 동작한다. 각 버전 출처는
-- docs/20_아키텍처/Narwhal_호환성_Seed_Sources.md 에 기록되어 있다.

-- ------------------------------------------------------------
-- 1. compatibility_matrices 재확정
-- ------------------------------------------------------------

UPDATE compatibility_matrices
SET
    status          = 'verified',
    k8s_min         = '1.27',
    k8s_max         = '1.35',
    k8s_recommended = '1.35',
    tools = $$
    {
      "source_repository":        {"Name": "GitLab CE",       "HelmVersion": "9.5.1",  "AppVersion": "18.5.1",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"],          "Tier": "stable"},
      "ci_platform":              {"Name": "GitLab CI",       "HelmVersion": "9.5.1",  "AppVersion": "18.5.1",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"],          "Tier": "stable"},
      "container_registry":       {"Name": "GitLab Registry", "HelmVersion": "9.5.1",  "AppVersion": "18.5.1",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"],          "Tier": "stable"},
      "storage_backend":          {"Name": "MinIO",           "HelmVersion": "5.2.0",  "AppVersion": "RELEASE.2024-08-03T04-33-23Z",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"],  "Tier": "stable"},
      "cd_tool":                  {"Name": "Argo CD",         "HelmVersion": "6.8.0",  "AppVersion": "v2.8.3",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"],  "Tier": "stable"},
      "monitoring_collection":    {"Name": "Prometheus",      "HelmVersion": "67.0.0", "AppVersion": "v2.54.1",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"],  "Tier": "stable"},
      "monitoring_visualization": {"Name": "Grafana",         "HelmVersion": "8.5.0",  "AppVersion": "11.1.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"],  "Tier": "stable"}
    }
    $$::jsonb,
    updated_at = NOW()
WHERE id = 'gitlab-allinone-v1';

UPDATE compatibility_matrices
SET
    status          = 'verified',
    k8s_min         = '1.27',
    k8s_max         = '1.35',
    k8s_recommended = '1.35',
    tools = $$
    {
      "source_repository":        {"Name": "GitLab CE",       "HelmVersion": "9.5.1",  "AppVersion": "18.5.1",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"],          "Tier": "stable"},
      "ci_platform":              {"Name": "GitLab CI",       "HelmVersion": "9.5.1",  "AppVersion": "18.5.1",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"],          "Tier": "stable"},
      "container_registry":       {"Name": "GitLab Registry", "HelmVersion": "9.5.1",  "AppVersion": "18.5.1",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"],          "Tier": "stable"},
      "storage_backend":          {"Name": "MinIO",           "HelmVersion": "5.2.0",  "AppVersion": "RELEASE.2024-08-03T04-33-23Z",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"],  "Tier": "stable"},
      "cd_tool":                  {"Name": "Argo CD",         "HelmVersion": "6.8.0",  "AppVersion": "v2.8.3",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"],  "Tier": "stable"},
      "monitoring_collection":    {"Name": "Prometheus",      "HelmVersion": "67.0.0", "AppVersion": "v2.54.1",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"],  "Tier": "stable"},
      "monitoring_visualization": {"Name": "Grafana",         "HelmVersion": "8.5.0",  "AppVersion": "11.1.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"],  "Tier": "stable"}
    }
    $$::jsonb,
    updated_at = NOW()
WHERE id = 'gitlab-argocd-v1';

UPDATE compatibility_matrices
SET
    status          = 'untested',
    k8s_min         = '1.27',
    k8s_max         = '1.35',
    k8s_recommended = '1.35',
    tools = $$
    {
      "source_repository":        {"Name": "GitHub",         "HelmVersion": "external", "AppVersion": "external",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64","arm64"], "Tier": "beta"},
      "ci_platform":              {"Name": "GitHub Actions", "HelmVersion": "external", "AppVersion": "external",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64","arm64"], "Tier": "beta"},
      "container_registry":       {"Name": "Harbor",         "HelmVersion": "1.15.0",   "AppVersion": "2.11.0",
                                   "MinK8sVersion": "1.27", "ArchSupport": ["amd64"],         "Tier": "beta"},
      "storage_backend":          {"Name": "MinIO",          "HelmVersion": "5.2.0",    "AppVersion": "RELEASE.2024-08-03T04-33-23Z",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "beta"},
      "cd_tool":                  {"Name": "Argo CD",        "HelmVersion": "6.8.0",    "AppVersion": "v2.8.3",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "beta"},
      "monitoring_collection":    {"Name": "Prometheus",     "HelmVersion": "67.0.0",   "AppVersion": "v2.54.1",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "beta"},
      "monitoring_visualization": {"Name": "Grafana",        "HelmVersion": "8.5.0",    "AppVersion": "11.1.0",
                                   "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "beta"}
    }
    $$::jsonb,
    updated_at = NOW()
WHERE id = 'github-argocd-v1';

-- ------------------------------------------------------------
-- 2. golden_path_templates 재확정 (compatibility_matrices 와 동일한 버전 pin)
--    golden_path_templates.tools 는 배열 형태 + snake_case 키를 사용한다.
-- ------------------------------------------------------------

UPDATE golden_path_templates
SET
    description = 'GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.',
    tools = $$[
      {"category":"source_repository",       "name":"GitLab CE",       "helm_version":"9.5.1",  "app_version":"18.5.1"},
      {"category":"ci_platform",             "name":"GitLab CI",       "helm_version":"9.5.1",  "app_version":"18.5.1"},
      {"category":"container_registry",      "name":"GitLab Registry", "helm_version":"9.5.1",  "app_version":"18.5.1"},
      {"category":"storage_backend",         "name":"MinIO",           "helm_version":"5.2.0",  "app_version":"RELEASE.2024-08-03T04-33-23Z"},
      {"category":"cd_tool",                 "name":"Argo CD",         "helm_version":"6.8.0",  "app_version":"v2.8.3"},
      {"category":"monitoring_collection",   "name":"Prometheus",      "helm_version":"67.0.0", "app_version":"v2.54.1"},
      {"category":"monitoring_visualization","name":"Grafana",         "helm_version":"8.5.0",  "app_version":"11.1.0"}
    ]$$::jsonb,
    updated_at = NOW()
WHERE id = 'gitlab-allinone-v1';

UPDATE golden_path_templates
SET
    description = 'GitLab CI와 GitLab Registry를 사용하고 Argo CD로 GitOps 패턴을 강화한 구성입니다.',
    tools = $$[
      {"category":"source_repository",       "name":"GitLab CE",       "helm_version":"9.5.1",  "app_version":"18.5.1"},
      {"category":"ci_platform",             "name":"GitLab CI",       "helm_version":"9.5.1",  "app_version":"18.5.1"},
      {"category":"container_registry",      "name":"GitLab Registry", "helm_version":"9.5.1",  "app_version":"18.5.1"},
      {"category":"storage_backend",         "name":"MinIO",           "helm_version":"5.2.0",  "app_version":"RELEASE.2024-08-03T04-33-23Z"},
      {"category":"cd_tool",                 "name":"Argo CD",         "helm_version":"6.8.0",  "app_version":"v2.8.3"},
      {"category":"monitoring_collection",   "name":"Prometheus",      "helm_version":"67.0.0", "app_version":"v2.54.1"},
      {"category":"monitoring_visualization","name":"Grafana",         "helm_version":"8.5.0",  "app_version":"11.1.0"}
    ]$$::jsonb,
    updated_at = NOW()
WHERE id = 'gitlab-argocd-v1';

UPDATE golden_path_templates
SET
    description = 'GitHub와 GitHub Actions를 외부 서비스로 사용하고, 클러스터 내에는 Harbor + Argo CD + 모니터링만 설치합니다.',
    tools = $$[
      {"category":"source_repository",       "name":"GitHub",         "helm_version":"external", "app_version":"external"},
      {"category":"ci_platform",             "name":"GitHub Actions", "helm_version":"external", "app_version":"external"},
      {"category":"container_registry",      "name":"Harbor",         "helm_version":"1.15.0",   "app_version":"2.11.0"},
      {"category":"storage_backend",         "name":"MinIO",          "helm_version":"5.2.0",    "app_version":"RELEASE.2024-08-03T04-33-23Z"},
      {"category":"cd_tool",                 "name":"Argo CD",        "helm_version":"6.8.0",    "app_version":"v2.8.3"},
      {"category":"monitoring_collection",   "name":"Prometheus",     "helm_version":"67.0.0",   "app_version":"v2.54.1"},
      {"category":"monitoring_visualization","name":"Grafana",        "helm_version":"8.5.0",    "app_version":"11.1.0"}
    ]$$::jsonb,
    updated_at = NOW()
WHERE id = 'github-argocd-v1';
