-- SSO 계정과 짝이 되는 사용자 시드.
--
-- 배경: 인증이 OIDC 로 넘어가면서 API 는 토큰의 email 클레임으로 users 행을 찾는다.
-- 그런데 Keycloak 쪽 계정(scripts/setup-keycloak.sh)과 DB 시드가 서로 다른 이메일을
-- 쓰고 있었다.
--
--   Keycloak : admin@nullus.io / devops@nullus.io / dev@nullus.io
--   DB 시드  : admin@nullus.io / kim@nullus.io    / park@nullus.io
--
-- 그래서 admin 을 뺀 두 계정은 로그인은 되지만 사용자 매칭이 되지 않았다.
-- Keycloak 쪽을 정본으로 삼아 DB 에 대응 사용자를 추가한다.
--
-- kim@/park@ 는 화면 샘플 데이터로 여러 시드에서 문자열로 참조되므로 지우지 않는다
-- (000021·000023·000027·000028 등). 이 마이그레이션은 추가만 한다.

INSERT INTO users (id, email, name, role, org_id, is_active, created_at, updated_at)
VALUES
  ('24444444-4444-4444-4444-444444444444', 'devops@nullus.io', 'Devops User',   'devops',    '11111111-1111-1111-1111-111111111111', true, NOW(), NOW()),
  ('25555555-5555-5555-5555-555555555555', 'dev@nullus.io',    'Dev User',      'developer', '11111111-1111-1111-1111-111111111111', true, NOW(), NOW())
ON CONFLICT (email) DO NOTHING;

INSERT INTO org_members (id, org_id, user_id, role, joined_at)
VALUES
  ('44444444-4444-4444-4444-444444444444', '11111111-1111-1111-1111-111111111111', '24444444-4444-4444-4444-444444444444', 'devops',    NOW()),
  ('45555555-5555-5555-5555-555555555555', '11111111-1111-1111-1111-111111111111', '25555555-5555-5555-5555-555555555555', 'developer', NOW())
ON CONFLICT (org_id, user_id) DO NOTHING;
