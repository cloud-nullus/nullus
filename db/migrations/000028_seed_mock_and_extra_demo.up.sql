-- ============================================================================
-- 000022_seed_mock_and_extra_demo.up.sql
-- Mock auth 사용자(@nullus.dev) DB 등록 + 추가 조직/사용자/클러스터 데모 데이터
-- ============================================================================

-- ── Mock Auth 사용자 (@nullus.dev, development 모드 전용) ───────────────────
-- login-page.tsx TEST_ACCOUNTS와 ID/이메일 일치 필수

INSERT INTO users (id, email, name, role, org_id, is_active, created_at, updated_at) VALUES
('a1000000-0000-0000-0000-000000000001', 'admin@nullus.dev',     'Admin User',      'admin',     '11111111-1111-1111-1111-111111111111', true,  NOW(), NOW()),
('a2000000-0000-0000-0000-000000000002', 'devops@nullus.dev',    'DevOps Engineer',  'devops',    '11111111-1111-1111-1111-111111111111', true,  NOW(), NOW()),
('a3000000-0000-0000-0000-000000000003', 'developer@nullus.dev', 'Developer',        'developer', '11111111-1111-1111-1111-111111111111', true,  NOW(), NOW())
ON CONFLICT (email) DO NOTHING;

INSERT INTO org_members (org_id, user_id, role, joined_at) VALUES
('11111111-1111-1111-1111-111111111111', 'a1000000-0000-0000-0000-000000000001', 'admin',     NOW()),
('11111111-1111-1111-1111-111111111111', 'a2000000-0000-0000-0000-000000000002', 'devops',    NOW()),
('11111111-1111-1111-1111-111111111111', 'a3000000-0000-0000-0000-000000000003', 'developer', NOW())
ON CONFLICT (org_id, user_id) DO NOTHING;

-- ── 추가 조직 (2개, 총 3개) ─────────────────────────────────────────────────

INSERT INTO organizations (id, name, slug, domain, status, cluster_access_scope, created_at, updated_at) VALUES
(
  '22222222-2222-2222-2222-222222222222',
  'Acme Corp',
  'acme-corp',
  'acme.io',
  'active',
  ARRAY['acme-prod-cluster'],
  '2026-01-05T08:00:00Z',
  '2026-03-10T14:00:00Z'
),
(
  '33333333-3333-3333-3333-333333333333',
  'Startup Labs',
  'startup-labs',
  'startuplabs.dev',
  'inactive',
  '{}',
  '2026-02-20T10:00:00Z',
  '2026-03-01T09:00:00Z'
)
ON CONFLICT (id) DO NOTHING;

-- ── 추가 사용자 (5명, 총 11명) ──────────────────────────────────────────────

INSERT INTO users (id, email, name, role, org_id, is_active, created_at, updated_at) VALUES
('b1000000-0000-0000-0000-000000000001', 'john@acme.io',         'John Smith',   'admin',     '22222222-2222-2222-2222-222222222222', true,  '2026-01-05T08:00:00Z', '2026-03-10T14:00:00Z'),
('b2000000-0000-0000-0000-000000000002', 'jane@acme.io',         'Jane Doe',     'devops',    '22222222-2222-2222-2222-222222222222', true,  '2026-01-10T09:00:00Z', '2026-03-10T14:00:00Z'),
('b3000000-0000-0000-0000-000000000003', 'bob@acme.io',          'Bob Wilson',   'developer', '22222222-2222-2222-2222-222222222222', true,  '2026-02-01T10:00:00Z', '2026-03-10T14:00:00Z'),
('c1000000-0000-0000-0000-000000000001', 'sarah@startuplabs.dev','Sarah Lee',    'admin',     '33333333-3333-3333-3333-333333333333', true,  '2026-02-20T10:00:00Z', '2026-03-01T09:00:00Z'),
('c2000000-0000-0000-0000-000000000002', 'mike@startuplabs.dev', 'Mike Chen',    'developer', '33333333-3333-3333-3333-333333333333', false, '2026-02-25T11:00:00Z', '2026-03-01T09:00:00Z')
ON CONFLICT (email) DO NOTHING;

-- ── 추가 org_members ────────────────────────────────────────────────────────

INSERT INTO org_members (org_id, user_id, role, joined_at) VALUES
('22222222-2222-2222-2222-222222222222', 'b1000000-0000-0000-0000-000000000001', 'admin',     '2026-01-05T08:00:00Z'),
('22222222-2222-2222-2222-222222222222', 'b2000000-0000-0000-0000-000000000002', 'devops',    '2026-01-10T09:00:00Z'),
('22222222-2222-2222-2222-222222222222', 'b3000000-0000-0000-0000-000000000003', 'developer', '2026-02-01T10:00:00Z'),
('33333333-3333-3333-3333-333333333333', 'c1000000-0000-0000-0000-000000000001', 'admin',     '2026-02-20T10:00:00Z'),
('33333333-3333-3333-3333-333333333333', 'c2000000-0000-0000-0000-000000000002', 'developer', '2026-02-25T11:00:00Z')
ON CONFLICT (org_id, user_id) DO NOTHING;

-- ── 추가 클러스터 (3개, 총 5개 — 전체 connection_status enum 커버) ──────────

INSERT INTO clusters (id, name, type, endpoint, connection_status, org_id, created_at, updated_at) VALUES
(
  '35555555-5555-5555-5555-555555555555',
  'staging-cluster',
  'target',
  'https://staging.nullus.io:6443',
  'unreachable',
  '11111111-1111-1111-1111-111111111111',
  '2026-01-20T08:00:00Z',
  '2026-03-18T22:00:00Z'
),
(
  '36666666-6666-6666-6666-666666666666',
  'legacy-cluster',
  'pipeline',
  'https://legacy.nullus.io:6443',
  'auth_failed',
  '11111111-1111-1111-1111-111111111111',
  '2026-02-01T09:00:00Z',
  '2026-03-15T16:00:00Z'
),
(
  '37777777-7777-7777-7777-777777777777',
  'acme-prod-cluster',
  'target',
  'https://prod.acme.io:6443',
  'connected',
  '22222222-2222-2222-2222-222222222222',
  '2026-01-08T08:00:00Z',
  '2026-03-20T10:00:00Z'
)
ON CONFLICT (id) DO NOTHING;

-- ── 기존 조직 cluster_access_scope 업데이트 (새 클러스터 반영) ──────────────

UPDATE organizations
SET cluster_access_scope = ARRAY['kind-nullus-platform', 'kind-nullus-develop', 'staging-cluster', 'legacy-cluster'],
    updated_at = NOW()
WHERE id = '11111111-1111-1111-1111-111111111111';
