-- 000061 이 소프트 삭제한 행만 되살린다.
--
-- metadata 의 pruned_by 표식으로 대상을 좁힌다. deleted_at 만 보고 되살리면
-- 다른 이유로 지워진 행까지 살아난다.
--
-- uk_token_sources_org_provider_path 는 deleted_at IS NULL 인 행에만 걸리는
-- 부분 유니크 인덱스다. 지운 뒤 같은 (org, provider, path) 로 새 행이 생겼다면
-- 되살리는 순간 충돌하므로, 살아 있는 행이 없는 경우에만 복원한다.

UPDATE token_sources AS t
SET deleted_at = NULL,
    updated_at = now(),
    metadata = t.metadata - 'pruned_by'
WHERE t.deleted_at IS NOT NULL
  AND t.metadata->>'pruned_by' = '000061'
  AND NOT EXISTS (
      SELECT 1
      FROM token_sources AS live
      WHERE live.deleted_at IS NULL
        AND live.org_id = t.org_id
        AND live.provider = t.provider
        AND live.path = t.path
  );

-- 충돌 때문에 되살리지 못한 행은 표식만 지운다. 남겨 두면 다음 down 이
-- 같은 행을 다시 시도한다.
UPDATE token_sources
SET metadata = metadata - 'pruned_by',
    updated_at = now()
WHERE metadata->>'pruned_by' = '000061';
