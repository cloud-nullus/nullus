-- github-argocd-v1 을 untested / beta 로 되돌린다.
--
-- 000042 가 이 매트릭스의 모든 도구를 beta 로 세워 두었으므로 그 상태로
-- 돌아간다. status 와 Tier 는 000041 의 규칙에서 한 쌍으로 움직인다.

BEGIN;

UPDATE compatibility_matrices m
SET
    status = 'untested',
    tools = (
        SELECT jsonb_object_agg(e.key, e.value || jsonb_build_object('Tier', 'beta'))
        FROM jsonb_each(m.tools) AS e
    ),
    updated_at = NOW()
WHERE id = 'github-argocd-v1';

COMMIT;
