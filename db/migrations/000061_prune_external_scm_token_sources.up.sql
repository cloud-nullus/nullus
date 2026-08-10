-- 외부 SaaS 도구로 만들어진 회전 대상 토큰 소스를 정리한다.
--
-- GitHub·GitHub Actions·GHCR 은 우리가 토큰을 발급할 수 없는데도
-- kv/.../artifacts/github/token 같은 항목이 만들어져 있었다. 값이 영영 채워지지
-- 않는 죽은 행이라 회전 컨트롤러가 주기마다 실패하고 로그만 쌓는다(실측한 행은
-- retry_count 19, status=failed_manual).
--
-- 더 나쁜 것은 조회 쪽이다. 사용자가 등록한 GitHub PAT 항목과 provider 가 같아
-- 소유자 정보가 없는 행이 하나 더 존재하게 되고, SCM 연동 설정을 읽는 쿼리가
-- 둘 중 어느 것을 집을지 알 수 없어진다.
--
-- 새로 만들지 않는 것은 코드에서 막았지만(isExternalSCMProvider) 이미 만들어진
-- 행은 그대로 남으므로 여기서 치운다.
--
-- token_type='reissue' 로 좁히는 것이 핵심이다. 사용자가 마법사에 넣은 PAT 는
-- token_type='pat' 이고 provider 도 'github' 이라, 이 조건이 없으면 살아 있는
-- 자격증명까지 함께 지운다.
--
-- 하드 삭제가 아니라 소프트 삭제인 이유는 두 가지다. 되돌릴 수 있어야 하고,
-- token_rotation_events 가 token_sources 를 참조하는 외래키를 들고 있어 행을
-- 지우면 이력까지 끊긴다. 회전 컨트롤러와 조회 쿼리는 모두 deleted_at IS NULL
-- 로 거르므로 소프트 삭제만으로 증상이 멎는다.

-- metadata 에 표식을 남긴다. 이것이 없으면 down 이 "예전에 다른 이유로 지워진
-- 행" 까지 되살린다 — deleted_at 만으로는 누가 언제 왜 지웠는지 구분할 수 없다.
UPDATE token_sources
SET deleted_at = now(),
    updated_at = now(),
    metadata = metadata || jsonb_build_object('pruned_by', '000061')
WHERE deleted_at IS NULL
  AND token_type = 'reissue'
  AND provider IN ('github', 'github-actions', 'ghcr');
