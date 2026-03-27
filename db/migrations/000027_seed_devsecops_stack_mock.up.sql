INSERT INTO stacks (id, name, template_id, org_id, cluster_id, state, namespace, config, created_at, updated_at)
VALUES
(
  'mock-devsecops-enterprise',
  'Mock DevSecOps Enterprise',
  'gitlab-allinone-v1',
  '11111111-1111-1111-1111-111111111111',
  '31111111-1111-1111-1111-111111111111',
  'completed',
  'nullus-enterprise',
  '{
    "artifacts": {
      "package_registry": {"name": "GitLab Package Registry", "version": "17.7.2", "enabled": true},
      "source_repository": {"name": "GitLab CE", "version": "17.7.2", "enabled": true},
      "container_registry": {"name": "GitLab Registry", "version": "17.7.2", "enabled": true},
      "storage_backend": {"name": "MinIO", "version": "2024.11.7", "enabled": true}
    },
    "pipeline": {
      "ci_platform": {"name": "GitLab CI", "version": "17.7.2", "enabled": true},
      "cd_tool": {"name": "Argo CD", "version": "2.13.2", "enabled": true}
    },
    "monitoring": {
      "collection": {"name": "Prometheus", "version": "3.1.0", "enabled": true},
      "visualization": {"name": "Grafana", "version": "11.4.0", "enabled": true}
    },
    "logging": {
      "collection": {"name": "Loki", "version": "3.0.0", "enabled": true},
      "search": {"name": "OpenSearch", "version": "2.14.0", "enabled": true}
    },
    "resources": {
      "developers": 40,
      "concurrent_runners": 16,
      "weekly_commits": 600,
      "build_frequency": "high"
    }
  }'::jsonb,
  '2026-03-22T09:00:00Z',
  '2026-03-22T09:30:00Z'
),
(
  'mock-devsecops-gitops',
  'Mock DevSecOps GitOps',
  'gitlab-argocd-v1',
  '11111111-1111-1111-1111-111111111111',
  '32222222-2222-2222-2222-222222222222',
  'installing',
  'nullus-gitops',
  '{
    "artifacts": {
      "package_registry": {"name": "Harbor", "version": "2.11.0", "enabled": true},
      "source_repository": {"name": "GitLab CE", "version": "17.7.2", "enabled": true},
      "container_registry": {"name": "Harbor", "version": "2.11.0", "enabled": true},
      "storage_backend": {"name": "MinIO", "version": "2024.11.7", "enabled": true}
    },
    "pipeline": {
      "ci_platform": {"name": "GitLab CI", "version": "17.7.2", "enabled": true},
      "cd_tool": {"name": "Argo CD", "version": "2.13.2", "enabled": true}
    },
    "monitoring": {
      "collection": {"name": "Prometheus", "version": "3.1.0", "enabled": true},
      "visualization": {"name": "Grafana", "version": "11.4.0", "enabled": true}
    },
    "logging": {
      "collection": {"name": "Loki", "version": "3.0.0", "enabled": true},
      "search": {"name": "OpenSearch", "version": "2.14.0", "enabled": false}
    },
    "resources": {
      "developers": 18,
      "concurrent_runners": 8,
      "weekly_commits": 240,
      "build_frequency": "medium"
    }
  }'::jsonb,
  '2026-03-22T10:00:00Z',
  '2026-03-22T10:20:00Z'
),
(
  'mock-devsecops-lean',
  'Mock DevSecOps Lean',
  'github-argocd-v1',
  '11111111-1111-1111-1111-111111111111',
  '31111111-1111-1111-1111-111111111111',
  'pending',
  'nullus-lean',
  '{
    "artifacts": {
      "package_registry": {"name": "GitHub Packages", "version": "external", "enabled": true},
      "source_repository": {"name": "GitHub", "version": "external", "enabled": true},
      "container_registry": {"name": "Harbor", "version": "2.11.0", "enabled": true},
      "storage_backend": {"name": "MinIO", "version": "2024.11.7", "enabled": true}
    },
    "pipeline": {
      "ci_platform": {"name": "GitHub Actions", "version": "external", "enabled": true},
      "cd_tool": {"name": "Argo CD", "version": "2.13.2", "enabled": true}
    },
    "monitoring": {
      "collection": {"name": "Prometheus", "version": "3.1.0", "enabled": true},
      "visualization": {"name": "Grafana", "version": "11.4.0", "enabled": true}
    },
    "logging": {
      "collection": {"name": "Loki", "version": "3.0.0", "enabled": false},
      "search": {"name": "OpenSearch", "version": "2.14.0", "enabled": false}
    },
    "resources": {
      "developers": 8,
      "concurrent_runners": 3,
      "weekly_commits": 90,
      "build_frequency": "low"
    }
  }'::jsonb,
  '2026-03-22T11:00:00Z',
  '2026-03-22T11:00:00Z'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO stack_config_versions (id, stack_id, version, config, changed_by, change_reason, created_at)
VALUES
(
  'm22-h1',
  'mock-devsecops-enterprise',
  1,
  '{"gitlab": "17.7.2", "argocd": "2.13.2", "prometheus": "3.1.0", "grafana": "11.4.0"}'::jsonb,
  'admin@nullus.io',
  '목업 엔터프라이즈 스택 초기 데이터 시드',
  '2026-03-22T09:30:00Z'
),
(
  'm22-h2',
  'mock-devsecops-gitops',
  1,
  '{"gitlab": "17.7.2", "harbor": "2.11.0", "argocd": "2.13.2", "status": "installing"}'::jsonb,
  'kim@nullus.io',
  'GitOps 중심 목업 스택 생성',
  '2026-03-22T10:20:00Z'
),
(
  'm22-h3',
  'mock-devsecops-lean',
  1,
  '{"github": "external", "actions": "external", "argocd": "2.13.2", "status": "pending"}'::jsonb,
  'park@nullus.io',
  'Lean 목업 스택 생성',
  '2026-03-22T11:00:00Z'
)
ON CONFLICT (id) DO NOTHING;
