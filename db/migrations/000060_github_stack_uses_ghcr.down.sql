-- github-argocd-v1 을 Harbor 기반 구성으로 되돌린다.
-- 000042 가 확정한 값과 같다.

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
    estimated_install_time = 60,
    min_resources = '6 vCPU / 12Gi RAM / 80Gi Storage',
    updated_at = NOW()
WHERE id = 'github-argocd-v1';

UPDATE compatibility_matrices
SET
    tools = jsonb_set(
        tools,
        '{container_registry}',
        $${"Name": "Harbor", "HelmVersion": "1.15.0", "AppVersion": "2.11.0",
           "MinK8sVersion": "1.26", "ArchSupport": ["amd64"], "Tier": "beta"}$$::jsonb,
        true
    ),
    updated_at = NOW()
WHERE id = 'github-argocd-v1';
