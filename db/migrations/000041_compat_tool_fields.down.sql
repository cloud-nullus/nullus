-- 000041_compat_tool_fields.down.sql
-- tools JSONB 각 엔트리에서 MinK8sVersion / ArchSupport / Tier 키를 제거해 v1 스키마로 복원한다.

WITH stripped AS (
    SELECT
        cm.id,
        jsonb_object_agg(
            entry.key,
            (entry.value - 'MinK8sVersion' - 'ArchSupport' - 'Tier')
        ) AS tools
    FROM compatibility_matrices cm
    CROSS JOIN LATERAL jsonb_each(cm.tools) AS entry
    GROUP BY cm.id
)
UPDATE compatibility_matrices cm
SET
    tools      = s.tools,
    updated_at = NOW()
FROM stripped s
WHERE cm.id = s.id;
