-- Seed initial admin-facing sample data for Organization/Cluster/Member screens.
-- This keeps UI sample content sourced from DB records, not frontend hardcoded fallbacks.

INSERT INTO organizations (id, name, slug, domain, status, cluster_access_scope, created_at, updated_at)
VALUES (
  '11111111-1111-1111-1111-111111111111',
  'Nullus DevOps Team',
  'nullus-devops',
  'nullus.io',
  'active',
  ARRAY['devops-cluster'],
  NOW(),
  NOW()
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (id, email, name, role, org_id, is_active, created_at, updated_at)
VALUES
  ('21111111-1111-1111-1111-111111111111', 'admin@nullus.io', 'Admin User', 'admin', '11111111-1111-1111-1111-111111111111', true, NOW(), NOW()),
  ('22222222-2222-2222-2222-222222222222', 'kim@nullus.io', 'Kim DevOps', 'devops', '11111111-1111-1111-1111-111111111111', true, NOW(), NOW()),
  ('23333333-3333-3333-3333-333333333333', 'park@nullus.io', 'Park Developer', 'developer', '11111111-1111-1111-1111-111111111111', false, NOW(), NOW())
ON CONFLICT (email) DO NOTHING;

UPDATE organizations
SET default_admin_id = '21111111-1111-1111-1111-111111111111', updated_at = NOW()
WHERE id = '11111111-1111-1111-1111-111111111111';

INSERT INTO clusters (id, name, type, endpoint, connection_status, org_id, created_at, updated_at)
VALUES
  ('31111111-1111-1111-1111-111111111111', 'devops-cluster', 'pipeline', 'https://devops-cluster.nullus.io', 'connected', '11111111-1111-1111-1111-111111111111', NOW(), NOW()),
  ('32222222-2222-2222-2222-222222222222', 'app-cluster-prod', 'target', 'https://app-cluster-prod.nullus.io', 'pending', '11111111-1111-1111-1111-111111111111', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO org_members (id, org_id, user_id, role, joined_at)
VALUES
  ('41111111-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', '21111111-1111-1111-1111-111111111111', 'admin', NOW()),
  ('42222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222', 'devops', NOW()),
  ('43333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111', '23333333-3333-3333-3333-333333333333', 'developer', NOW())
ON CONFLICT (org_id, user_id) DO NOTHING;
