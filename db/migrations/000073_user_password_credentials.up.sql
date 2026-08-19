-- 포털 ID/PW 로그인용 자격 저장.
--
-- 지금까지 포털은 OIDC 아니면 session 둘 중 하나만 썼고, session 모드는 클라이언트가
-- 보낸 X-User-* 헤더를 그대로 믿어 사실상 무인증이었다. 비밀번호를 담을 자리 자체가
-- 없어 실제 검증이 불가능했다.
--
-- IdP 가 죽어도 들어갈 수단이 있어야 하므로 OIDC 와 나란히 두는 경로다.
-- NULL 은 "비밀번호를 설정하지 않은 계정"(OIDC 전용)이고, 이 값으로는 어떤
-- 비밀번호도 통과하지 않는다.
ALTER TABLE users ADD COLUMN password_hash TEXT;

-- 비밀번호가 마지막으로 바뀐 시각. 유출 대응 시 강제 만료 기준이 된다.
ALTER TABLE users ADD COLUMN password_updated_at TIMESTAMPTZ;

COMMENT ON COLUMN users.password_hash IS
  'bcrypt 해시. NULL 이면 ID/PW 로그인 불가(OIDC 전용 계정).';
