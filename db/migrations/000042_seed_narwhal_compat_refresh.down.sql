-- 000042_seed_narwhal_compat_refresh.down.sql
-- Narwhal baseline reassert를 롤백한다. 되돌릴 상태는 000041 직전, 즉 000024 + 000026 의
-- 결과다 (Task 1에서 추가한 v2 필드가 같이 제거되지 않도록 down은 신중하게 값만 교체한다).
-- 실무적으로는 000041을 먼저 down 하고 000042를 down 하는 경우가 정상 경로이므로,
-- 아래 down은 "000042가 덮어쓴 값을 이전 canonical 상태로 복원"만 수행한다.

UPDATE compatibility_matrices
SET
    tools = $$
    {
      "source_repository":        {"Name": "GitLab CE",       "HelmVersion": "9.5.1",  "AppVersion": "18.5.1"},
      "ci_platform":              {"Name": "GitLab CI",       "HelmVersion": "9.5.1",  "AppVersion": "18.5.1"},
      "container_registry":       {"Name": "GitLab Registry", "HelmVersion": "9.5.1",  "AppVersion": "18.5.1"},
      "storage_backend":          {"Name": "MinIO",           "HelmVersion": "5.2.0",  "AppVersion": "RELEASE.2024-08-03T04-33-23Z"},
      "cd_tool":                  {"Name": "Argo CD",         "HelmVersion": "6.8.0",  "AppVersion": "v2.8.3"},
      "monitoring_collection":    {"Name": "Prometheus",      "HelmVersion": "67.0.0", "AppVersion": "v2.54.1"},
      "monitoring_visualization": {"Name": "Grafana",         "HelmVersion": "8.5.0",  "AppVersion": "11.1.0"}
    }
    $$::jsonb,
    updated_at = NOW()
WHERE id IN ('gitlab-allinone-v1', 'gitlab-argocd-v1');

UPDATE compatibility_matrices
SET
    tools = $$
    {
      "source_repository":        {"Name": "GitHub",         "HelmVersion": "external", "AppVersion": "external"},
      "ci_platform":              {"Name": "GitHub Actions", "HelmVersion": "external", "AppVersion": "external"},
      "container_registry":       {"Name": "Harbor",         "HelmVersion": "1.15.0",   "AppVersion": "2.11.0"},
      "storage_backend":          {"Name": "MinIO",          "HelmVersion": "5.2.0",    "AppVersion": "RELEASE.2024-08-03T04-33-23Z"},
      "cd_tool":                  {"Name": "Argo CD",        "HelmVersion": "6.8.0",    "AppVersion": "v2.8.3"},
      "monitoring_collection":    {"Name": "Prometheus",     "HelmVersion": "67.0.0",   "AppVersion": "v2.54.1"},
      "monitoring_visualization": {"Name": "Grafana",        "HelmVersion": "8.5.0",    "AppVersion": "11.1.0"}
    }
    $$::jsonb,
    updated_at = NOW()
WHERE id = 'github-argocd-v1';

-- golden_path_templates 는 000024/000026 이후 상태가 이미 canonical 과 동일하므로
-- 내용 변경 없이 updated_at 만 touch 한다 (no-op 수준).
UPDATE golden_path_templates
SET updated_at = NOW()
WHERE id IN ('gitlab-allinone-v1', 'gitlab-argocd-v1', 'github-argocd-v1');
