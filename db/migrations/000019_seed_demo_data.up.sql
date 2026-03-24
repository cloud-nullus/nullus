-- ============================================================================
-- 000019_seed_demo_data.up.sql
-- Demo seed data for Nullus Platform
-- ============================================================================

-- Org ID reference (from 000016): 11111111-1111-1111-1111-111111111111

-- ── Stacks (4개) ────────────────────────────────────────────────────────────

INSERT INTO stacks (id, name, template_id, org_id, cluster_id, state, namespace, config, created_at, updated_at) VALUES
(
  'production-stack',
  'Production Stack',
  'gitlab-allinone-v1',
  '11111111-1111-1111-1111-111111111111',
  (SELECT id FROM clusters WHERE name = 'devops-cluster' LIMIT 1),
  'completed',
  'nullus-prod',
  '{
    "artifacts": {"package_registry": {"name": "GitLab", "version": "8.7.3", "enabled": true}, "source_repository": {"name": "GitLab CE", "version": "17.7.3", "enabled": true}, "container_registry": {"name": "GitLab Registry", "version": "17.7.3", "enabled": true}, "storage_backend": {"name": "MinIO", "version": "5.4.0", "enabled": true}},
    "pipeline": {"ci_platform": {"name": "GitLab CI", "version": "17.7.3", "enabled": true}, "cd_tool": {"name": "Argo CD", "version": "2.14.2", "enabled": true}},
    "monitoring": {"collection": {"name": "Prometheus", "version": "27.3.0", "enabled": true}, "visualization": {"name": "Grafana", "version": "8.8.4", "enabled": true}},
    "logging": {"collection": {"name": "Loki", "version": "6.24.0", "enabled": true}, "search": {"name": "OpenSearch", "version": "2.28.0", "enabled": true}},
    "resources": {"developers": 10, "concurrent_runners": 4, "weekly_commits": 50, "build_frequency": "high"}
  }'::jsonb,
  '2026-01-10T09:00:00Z',
  '2026-03-15T14:30:00Z'
),
(
  'development-stack',
  'Development Stack',
  'gitlab-argocd-v1',
  '11111111-1111-1111-1111-111111111111',
  (SELECT id FROM clusters WHERE name = 'devops-cluster' LIMIT 1),
  'completed',
  'nullus-dev',
  '{
    "artifacts": {"package_registry": {"name": "Nexus", "version": "3.75.0", "enabled": true}, "source_repository": {"name": "GitLab CE", "version": "17.7.3", "enabled": true}, "container_registry": {"name": "Harbor", "version": "1.16.2", "enabled": true}, "storage_backend": {"name": "MinIO", "version": "5.4.0", "enabled": true}},
    "pipeline": {"ci_platform": {"name": "GitLab CI", "version": "17.7.3", "enabled": true}, "cd_tool": {"name": "Argo CD", "version": "2.14.2", "enabled": true}},
    "monitoring": {"collection": {"name": "Prometheus", "version": "27.3.0", "enabled": true}, "visualization": {"name": "Grafana", "version": "8.8.4", "enabled": true}},
    "logging": {"collection": {"name": "Loki", "version": "6.24.0", "enabled": true}, "search": {"name": "Grafana", "version": "8.8.4", "enabled": true}},
    "resources": {"developers": 5, "concurrent_runners": 2, "weekly_commits": 30, "build_frequency": "medium"}
  }'::jsonb,
  '2026-01-15T10:00:00Z',
  '2026-03-10T11:00:00Z'
),
(
  'staging-environment',
  'Staging Environment',
  'github-argocd-v1',
  '11111111-1111-1111-1111-111111111111',
  (SELECT id FROM clusters WHERE name = 'app-cluster-prod' LIMIT 1),
  'installing',
  'nullus-staging',
  '{
    "artifacts": {"package_registry": {"name": "", "version": "", "enabled": false}, "source_repository": {"name": "GitHub", "version": "external", "enabled": true}, "container_registry": {"name": "Harbor", "version": "1.16.2", "enabled": true}, "storage_backend": {"name": "MinIO", "version": "5.4.0", "enabled": true}},
    "pipeline": {"ci_platform": {"name": "GitHub Actions", "version": "external", "enabled": true}, "cd_tool": {"name": "Argo CD", "version": "2.14.2", "enabled": true}},
    "monitoring": {"collection": {"name": "Prometheus", "version": "27.3.0", "enabled": true}, "visualization": {"name": "Grafana", "version": "8.8.4", "enabled": true}},
    "logging": {"collection": {"name": "Loki", "version": "6.24.0", "enabled": true}, "search": {"name": "Grafana", "version": "8.8.4", "enabled": true}},
    "resources": {"developers": 3, "concurrent_runners": 1, "weekly_commits": 15, "build_frequency": "low"}
  }'::jsonb,
  '2026-02-01T08:00:00Z',
  '2026-03-18T16:00:00Z'
),
(
  'microservices-platform',
  'Microservices Platform',
  'gitlab-argocd-v1',
  '11111111-1111-1111-1111-111111111111',
  (SELECT id FROM clusters WHERE name = 'app-cluster-prod' LIMIT 1),
  'failed',
  'nullus-msa',
  '{
    "artifacts": {"package_registry": {"name": "", "version": "", "enabled": false}, "source_repository": {"name": "GitLab CE", "version": "17.7.3", "enabled": true}, "container_registry": {"name": "Harbor", "version": "1.16.2", "enabled": true}, "storage_backend": {"name": "MinIO", "version": "5.4.0", "enabled": true}},
    "pipeline": {"ci_platform": {"name": "GitLab CI", "version": "17.7.3", "enabled": true}, "cd_tool": {"name": "Flux", "version": "2.4.0", "enabled": true}},
    "monitoring": {"collection": {"name": "Thanos", "version": "15.7.0", "enabled": true}, "visualization": {"name": "Grafana", "version": "8.8.4", "enabled": true}},
    "logging": {"collection": {"name": "Loki", "version": "6.24.0", "enabled": true}, "search": {"name": "OpenSearch", "version": "2.28.0", "enabled": true}},
    "resources": {"developers": 15, "concurrent_runners": 6, "weekly_commits": 80, "build_frequency": "high"}
  }'::jsonb,
  '2026-03-01T13:00:00Z',
  '2026-03-20T09:00:00Z'
)
ON CONFLICT (id) DO NOTHING;

-- ── Stack Config Versions (이력 5개) ────────────────────────────────────────

INSERT INTO stack_config_versions (id, stack_id, version, config, changed_by, change_reason, created_at) VALUES
-- production-stack: v1-v4
('h1', 'production-stack', 1,
 '{"artifacts":{"package_registry":{"name":"GitLab","version":"8.5.0","enabled":true},"source_repository":{"name":"GitLab CE","version":"16.7","enabled":true},"container_registry":{"name":"GitLab Registry","version":"16.7","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitLab CI","version":"16.7","enabled":true},"cd_tool":{"name":"Argo CD","version":"2.9.3","enabled":true}},"monitoring":{"collection":{"name":"Prometheus","version":"2.48","enabled":true},"visualization":{"name":"Grafana","version":"10.2","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.20.0","enabled":true},"search":{"name":"OpenSearch","version":"2.28.0","enabled":true}}}'::jsonb,
 'admin@nullus.io', '초기 스택 배포', '2026-01-10T09:00:00Z'),
('h2', 'production-stack', 2,
 '{"artifacts":{"package_registry":{"name":"GitLab","version":"8.5.0","enabled":true},"source_repository":{"name":"GitLab CE","version":"16.7","enabled":true},"container_registry":{"name":"GitLab Registry","version":"16.7","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitLab CI","version":"16.7","enabled":true},"cd_tool":{"name":"Argo CD","version":"2.9.3","enabled":true}},"monitoring":{"collection":{"name":"Prometheus","version":"2.49","enabled":true},"visualization":{"name":"Grafana","version":"10.2","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.20.0","enabled":true},"search":{"name":"OpenSearch","version":"2.28.0","enabled":true}}}'::jsonb,
 'kim@nullus.io', 'Prometheus 버전 업그레이드 (v2.48 → v2.49)', '2026-02-05T11:30:00Z'),
('h3', 'production-stack', 3,
 '{"artifacts":{"package_registry":{"name":"GitLab","version":"8.5.0","enabled":true},"source_repository":{"name":"GitLab CE","version":"16.7","enabled":true},"container_registry":{"name":"GitLab Registry","version":"16.7","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitLab CI","version":"16.7","enabled":true},"cd_tool":{"name":"Argo CD","version":"2.9.3","enabled":true}},"monitoring":{"collection":{"name":"Prometheus","version":"2.49","enabled":true},"visualization":{"name":"Grafana","version":"10.3","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.20.0","enabled":true},"search":{"name":"OpenSearch","version":"2.28.0","enabled":true}}}'::jsonb,
 'kim@nullus.io', 'Grafana 버전 업그레이드 (v10.2 → v10.3)', '2026-02-20T14:00:00Z'),
('h4', 'production-stack', 4,
 '{"artifacts":{"package_registry":{"name":"GitLab","version":"8.7.3","enabled":true},"source_repository":{"name":"GitLab CE","version":"16.8","enabled":true},"container_registry":{"name":"GitLab Registry","version":"16.8","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitLab CI","version":"16.8","enabled":true},"cd_tool":{"name":"Argo CD","version":"2.10.0","enabled":true}},"monitoring":{"collection":{"name":"Prometheus","version":"2.49","enabled":true},"visualization":{"name":"Grafana","version":"10.3","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.24.0","enabled":true},"search":{"name":"OpenSearch","version":"2.28.0","enabled":true}}}'::jsonb,
 'admin@nullus.io', 'GitLab + ArgoCD 마이너 업그레이드', '2026-03-03T14:28:00Z'),
-- development-stack: v1-v2
('h5', 'development-stack', 1,
 '{"artifacts":{"package_registry":{"name":"Nexus","version":"3.75.0","enabled":true},"source_repository":{"name":"GitLab CE","version":"16.7","enabled":true},"container_registry":{"name":"Harbor","version":"1.16.2","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitLab CI","version":"16.7","enabled":true},"cd_tool":{"name":"Argo CD","version":"2.9.3","enabled":true}},"monitoring":{"collection":{"name":"Prometheus","version":"2.48","enabled":true},"visualization":{"name":"Grafana","version":"8.8.4","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.20.0","enabled":true},"search":{"name":"Grafana","version":"8.8.4","enabled":true}}}'::jsonb,
 'kim@nullus.io', '개발 스택 초기 배포', '2026-01-15T10:00:00Z'),
('h6', 'development-stack', 2,
 '{"artifacts":{"package_registry":{"name":"Nexus","version":"3.75.0","enabled":true},"source_repository":{"name":"GitLab CE","version":"16.8","enabled":true},"container_registry":{"name":"Harbor","version":"1.16.2","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitLab CI","version":"16.8","enabled":true},"cd_tool":{"name":"Argo CD","version":"2.10.0","enabled":true}},"monitoring":{"collection":{"name":"Prometheus","version":"2.49","enabled":true},"visualization":{"name":"Grafana","version":"8.8.4","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.24.0","enabled":true},"search":{"name":"Grafana","version":"8.8.4","enabled":true}}}'::jsonb,
 'kim@nullus.io', 'GitLab + ArgoCD + Prometheus 업그레이드', '2026-02-20T09:00:00Z'),
-- staging-environment: v1-v2
('h7', 'staging-environment', 1,
 '{"artifacts":{"package_registry":{"name":"","version":"","enabled":false},"source_repository":{"name":"GitHub","version":"external","enabled":true},"container_registry":{"name":"Harbor","version":"1.16.2","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitHub Actions","version":"external","enabled":true},"cd_tool":{"name":"Argo CD","version":"2.9.3","enabled":true}},"monitoring":{"collection":{"name":"Prometheus","version":"2.48","enabled":true},"visualization":{"name":"Grafana","version":"8.8.4","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.20.0","enabled":true},"search":{"name":"Grafana","version":"8.8.4","enabled":true}}}'::jsonb,
 'admin@nullus.io', '스테이징 환경 초기 배포', '2026-02-01T08:00:00Z'),
('h8', 'staging-environment', 2,
 '{"artifacts":{"package_registry":{"name":"","version":"","enabled":false},"source_repository":{"name":"GitHub","version":"external","enabled":true},"container_registry":{"name":"Harbor","version":"1.16.2","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitHub Actions","version":"external","enabled":true},"cd_tool":{"name":"Argo CD","version":"2.10.0","enabled":true}},"monitoring":{"collection":{"name":"Prometheus","version":"2.49","enabled":true},"visualization":{"name":"Grafana","version":"8.8.4","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.24.0","enabled":true},"search":{"name":"Grafana","version":"8.8.4","enabled":true}}}'::jsonb,
 'kim@nullus.io', 'ArgoCD + Prometheus 버전 업그레이드', '2026-03-05T10:00:00Z'),
-- microservices-platform: v1-v3
('h9', 'microservices-platform', 1,
 '{"artifacts":{"package_registry":{"name":"","version":"","enabled":false},"source_repository":{"name":"GitLab CE","version":"16.7","enabled":true},"container_registry":{"name":"Harbor","version":"1.16.2","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitLab CI","version":"16.7","enabled":true},"cd_tool":{"name":"Flux","version":"2.3.0","enabled":true}},"monitoring":{"collection":{"name":"Thanos","version":"0.34","enabled":true},"visualization":{"name":"Grafana","version":"8.8.4","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.20.0","enabled":true},"search":{"name":"OpenSearch","version":"2.28.0","enabled":true}}}'::jsonb,
 'admin@nullus.io', 'MSA 플랫폼 초기 배포', '2026-03-01T13:00:00Z'),
('h10', 'microservices-platform', 2,
 '{"artifacts":{"package_registry":{"name":"","version":"","enabled":false},"source_repository":{"name":"GitLab CE","version":"16.7","enabled":true},"container_registry":{"name":"Harbor","version":"1.16.2","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitLab CI","version":"16.7","enabled":true},"cd_tool":{"name":"Flux","version":"2.4.0","enabled":true}},"monitoring":{"collection":{"name":"Thanos","version":"0.35","enabled":true},"visualization":{"name":"Grafana","version":"8.8.4","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.24.0","enabled":true},"search":{"name":"OpenSearch","version":"2.28.0","enabled":true}}}'::jsonb,
 'kim@nullus.io', 'Flux + Thanos 버전 업그레이드', '2026-03-10T11:00:00Z'),
('h11', 'microservices-platform', 3,
 '{"artifacts":{"package_registry":{"name":"","version":"","enabled":false},"source_repository":{"name":"GitLab CE","version":"16.8","enabled":true},"container_registry":{"name":"Harbor","version":"1.16.3","enabled":true},"storage_backend":{"name":"MinIO","version":"5.4.0","enabled":true}},"pipeline":{"ci_platform":{"name":"GitLab CI","version":"16.8","enabled":true},"cd_tool":{"name":"Flux","version":"2.4.0","enabled":true}},"monitoring":{"collection":{"name":"Thanos","version":"0.35","enabled":true},"visualization":{"name":"Grafana","version":"8.8.4","enabled":true}},"logging":{"collection":{"name":"Loki","version":"6.24.0","enabled":true},"search":{"name":"OpenSearch","version":"2.28.0","enabled":true}}}'::jsonb,
 'kim@nullus.io', 'GitLab + Harbor 마이너 업그레이드', '2026-03-18T15:00:00Z')
ON CONFLICT (id) DO NOTHING;

-- ── Pipelines (4개) ─────────────────────────────────────────────────────────

INSERT INTO pipelines (id, name, template_id, org_id, cluster_id, namespace, app_type, git_repo_url, status, created_at) VALUES
(
  'frontend-web',
  'Frontend Web App',
  'web-frontend-v1',
  '11111111-1111-1111-1111-111111111111',
  (SELECT id::varchar FROM clusters WHERE name = 'devops-cluster' LIMIT 1),
  'nullus-prod',
  'web',
  'https://github.com/nullus/web-frontend.git',
  'active',
  '2026-01-20T09:00:00Z'
),
(
  'backend-api',
  'Backend API Server',
  'web-backend-v1',
  '11111111-1111-1111-1111-111111111111',
  (SELECT id::varchar FROM clusters WHERE name = 'devops-cluster' LIMIT 1),
  'nullus-prod',
  'backend',
  'https://github.com/nullus/backend-api.git',
  'active',
  '2026-01-20T09:30:00Z'
),
(
  'ml-service',
  'ML Prediction Service',
  'batch-job-v1',
  '11111111-1111-1111-1111-111111111111',
  (SELECT id::varchar FROM clusters WHERE name = 'app-cluster-prod' LIMIT 1),
  'nullus-ml',
  'batch',
  'https://github.com/nullus/ml-service.git',
  'active',
  '2026-02-10T11:00:00Z'
),
(
  'batch-runner',
  'Nightly Batch Runner',
  'batch-job-v1',
  '11111111-1111-1111-1111-111111111111',
  (SELECT id::varchar FROM clusters WHERE name = 'app-cluster-prod' LIMIT 1),
  'nullus-batch',
  'batch',
  'https://github.com/nullus/batch-runner.git',
  'inactive',
  '2026-02-15T14:00:00Z'
)
ON CONFLICT (id) DO NOTHING;

-- ── Pipeline Deployments (CI/CD 이력 8개) ───────────────────────────────────

INSERT INTO pipeline_deployments (id, pipeline_id, version, status, started_at, completed_at, deployed_by) VALUES
('d1', 'frontend-web', 'v1.2.3', 'success', '2026-03-20T14:22:00Z', '2026-03-20T14:30:00Z', 'kim@nullus.io'),
('d2', 'backend-api',  'v2.1.0', 'success', '2026-03-20T13:00:00Z', '2026-03-20T13:15:00Z', 'kim@nullus.io'),
('d3', 'frontend-web', 'v1.2.2', 'failed',  '2026-03-19T10:00:00Z', '2026-03-19T10:08:00Z', 'park@nullus.io'),
('d4', 'batch-runner', 'v1.3.1', 'success', '2026-03-18T22:00:00Z', '2026-03-18T22:12:00Z', 'admin@nullus.io'),
('d5', 'backend-api',  'v2.0.9', 'success', '2026-03-17T16:30:00Z', '2026-03-17T16:42:00Z', 'kim@nullus.io'),
('d6', 'ml-service',   'v0.8.1', 'success', '2026-03-16T09:00:00Z', '2026-03-16T09:25:00Z', 'park@nullus.io'),
('d7', 'frontend-web', 'v1.2.1', 'success', '2026-03-15T11:00:00Z', '2026-03-15T11:10:00Z', 'kim@nullus.io'),
('d8', 'ml-service',   'v0.8.0', 'failed',  '2026-03-14T15:00:00Z', '2026-03-14T15:18:00Z', 'park@nullus.io')
ON CONFLICT (id) DO NOTHING;

-- ── Alert Rules (5개) ───────────────────────────────────────────────────────

INSERT INTO alert_rules (id, name, condition, threshold, channel, enabled, created_at) VALUES
('ar-1', 'Build Failure Rate',      'build_failure_rate > threshold',   10,    'slack', true,  '2026-01-25T09:00:00Z'),
('ar-2', 'Deploy Rollback Count',   'rollback_count > threshold',       2,    'email', true,  '2026-01-25T09:00:00Z'),
('ar-3', 'Pipeline Duration Alert', 'pipeline_duration > threshold',   300,   'slack', true,  '2026-01-25T09:00:00Z'),
('ar-4', 'High CPU Usage',          'cpu_usage > threshold',            85,   'slack', true,  '2026-02-01T10:00:00Z'),
('ar-5', 'Storage Warning',         'storage_usage > threshold',        80,   'email', false, '2026-02-01T10:00:00Z')
ON CONFLICT (id) DO NOTHING;

-- ── Alert History (6개) ─────────────────────────────────────────────────────

INSERT INTO alerts (id, rule_id, severity, message, fired_at, resolved_at) VALUES
('al-1', 'ar-1', 'critical', 'frontend-web 파이프라인 빌드 실패율 15% 초과 (최근 1시간)',           '2026-03-19T10:15:00Z', '2026-03-19T11:30:00Z'),
('al-2', 'ar-3', 'warning',  'backend-api 파이프라인 실행 시간 420초 (임계값 300초 초과)',           '2026-03-18T14:00:00Z', '2026-03-18T14:30:00Z'),
('al-3', 'ar-4', 'info',     'devops-cluster CPU 사용률 88% — 스케일링 검토 필요',                  '2026-03-20T09:00:00Z', NULL),
('al-4', 'ar-1', 'critical', 'ml-service 파이프라인 빌드 3회 연속 실패',                            '2026-03-14T15:20:00Z', '2026-03-14T16:00:00Z'),
('al-5', 'ar-2', 'warning',  'frontend-web v1.2.2 배포 실패 후 자동 롤백 발생',                    '2026-03-19T10:10:00Z', '2026-03-19T10:12:00Z'),
('al-6', 'ar-5', 'info',     'app-cluster-prod 스토리지 사용률 82% — 정리 권장',                    '2026-03-21T08:00:00Z', NULL)
ON CONFLICT (id) DO NOTHING;

-- ── Notification Configs (2개) ──────────────────────────────────────────────

INSERT INTO notification_configs (id, org_id, channel, config, events, is_active, created_at) VALUES
(
  'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
  '11111111-1111-1111-1111-111111111111',
  'slack',
  '{"webhook_url": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX", "channel": "#nullus-alerts"}'::jsonb,
  ARRAY['pipeline_failure', 'deploy_rollback', 'high_cpu', 'tool_down'],
  true,
  '2026-01-25T09:00:00Z'
),
(
  'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
  '11111111-1111-1111-1111-111111111111',
  'email',
  '{"recipients": ["admin@nullus.io", "kim@nullus.io"], "subject_prefix": "[Nullus Alert]"}'::jsonb,
  ARRAY['deploy_rollback', 'storage_warning'],
  true,
  '2026-01-25T09:00:00Z'
)
ON CONFLICT (id) DO NOTHING;

-- ── Notification History (4개) ──────────────────────────────────────────────

INSERT INTO notification_history (id, org_id, channel, event, status, payload, error, created_at) VALUES
(
  gen_random_uuid(),
  '11111111-1111-1111-1111-111111111111',
  'slack',
  'pipeline_failure',
  'sent',
  '{"pipeline": "frontend-web", "version": "v1.2.2", "message": "빌드 실패"}'::jsonb,
  NULL,
  '2026-03-19T10:10:00Z'
),
(
  gen_random_uuid(),
  '11111111-1111-1111-1111-111111111111',
  'email',
  'deploy_rollback',
  'sent',
  '{"pipeline": "frontend-web", "version": "v1.2.2", "recipients": ["admin@nullus.io"]}'::jsonb,
  NULL,
  '2026-03-19T10:12:00Z'
),
(
  gen_random_uuid(),
  '11111111-1111-1111-1111-111111111111',
  'slack',
  'high_cpu',
  'sent',
  '{"cluster": "devops-cluster", "cpu": 88, "threshold": 85}'::jsonb,
  NULL,
  '2026-03-20T09:01:00Z'
),
(
  gen_random_uuid(),
  '11111111-1111-1111-1111-111111111111',
  'email',
  'storage_warning',
  'failed',
  '{"cluster": "app-cluster-prod", "storage": 82, "threshold": 80}'::jsonb,
  'SMTP connection timeout',
  '2026-03-21T08:01:00Z'
)
ON CONFLICT DO NOTHING;

-- ── Audit Logs (6개) ────────────────────────────────────────────────────────

INSERT INTO audit_logs (id, user_id, action, resource_type, resource_id, details, ip_address, created_at) VALUES
(gen_random_uuid(), 'admin@nullus.io', 'create',    'organization', '11111111-1111-1111-1111-111111111111', '{"name": "Nullus DevOps Team"}'::jsonb,           '192.168.1.10', '2026-01-10T08:00:00Z'),
(gen_random_uuid(), 'admin@nullus.io', 'register',  'cluster',      'devops-cluster',                       '{"name": "devops-cluster", "type": "pipeline"}'::jsonb, '192.168.1.10', '2026-01-10T08:30:00Z'),
(gen_random_uuid(), 'admin@nullus.io', 'deploy',    'stack',        'production-stack',                     '{"template": "gitlab-allinone-v1", "status": "completed"}'::jsonb, '192.168.1.10', '2026-01-10T09:00:00Z'),
(gen_random_uuid(), 'kim@nullus.io',   'deploy',    'pipeline',     'frontend-web',                         '{"version": "v1.2.3", "status": "success"}'::jsonb, '192.168.1.20', '2026-03-20T14:22:00Z'),
(gen_random_uuid(), 'park@nullus.io',  'deploy',    'pipeline',     'frontend-web',                         '{"version": "v1.2.2", "status": "failed"}'::jsonb,  '192.168.1.30', '2026-03-19T10:00:00Z'),
(gen_random_uuid(), 'kim@nullus.io',   'upgrade',   'stack',        'production-stack',                     '{"from": "v2.48", "to": "v2.49", "tool": "prometheus"}'::jsonb, '192.168.1.20', '2026-02-05T11:30:00Z')
ON CONFLICT DO NOTHING;
