-- ============================================================================
-- 000022_seed_mock_and_extra_demo.up.sql
-- Mock auth 사용자(@nullus.dev) DB 등록 + 로컬 단일 조직 데모 데이터
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

-- ── 추가 클러스터 (2개, 총 4개 — 전체 connection_status enum 커버) ──────────

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
)
ON CONFLICT (id) DO NOTHING;

-- ── 기존 조직 cluster_access_scope 업데이트 (새 클러스터 반영) ──────────────

UPDATE organizations
SET cluster_access_scope = ARRAY['kind-nullus-platform', 'kind-nullus-develop', 'staging-cluster', 'legacy-cluster'],
    updated_at = NOW()
WHERE id = '11111111-1111-1111-1111-111111111111';
