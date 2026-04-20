-- 000041_compat_tool_fields.up.sql
-- Compatibility Matrix Task 1: tools JSONB 세분화
-- 각 tool 엔트리에 다음 필드를 idempotent 하게 추가한다.
--   * MinK8sVersion  : 해당 도구가 요구하는 최소 K8s 버전 (미지정 시 매트릭스 k8s_min 상속)
--   * ArchSupport    : 지원 아키텍처 배열 (Harbor / GitLab 계열은 amd64 only, 그 외 amd64+arm64)
--   * Tier           : 도구 성숙도 (verified 매트릭스는 stable, 그 외는 beta)
--
-- JSONB 병합 시 기존 값이 있으면 그대로 유지한다 (COALESCE).

WITH arch_map(name_key, arch_support) AS (
    VALUES
        ('harbor',           '["amd64"]'::jsonb),
        ('gitlab ce',        '["amd64"]'::jsonb),
        ('gitlab ci',        '["amd64"]'::jsonb),
        ('gitlab registry',  '["amd64"]'::jsonb)
),
expanded AS (
    SELECT
        cm.id,
        jsonb_object_agg(
            entry.key,
            entry.value || jsonb_build_object(
                'MinK8sVersion',
                    COALESCE(NULLIF(entry.value->>'MinK8sVersion', ''), cm.k8s_min),
                'ArchSupport',
                    COALESCE(
                        entry.value->'ArchSupport',
                        am.arch_support,
                        '["amd64","arm64"]'::jsonb
                    ),
                'Tier',
                    COALESCE(
                        NULLIF(entry.value->>'Tier', ''),
                        CASE
                            WHEN cm.status = 'verified' THEN 'stable'
                            WHEN cm.status = 'unsupported' THEN 'deprecated'
                            ELSE 'beta'
                        END
                    )
            )
        ) AS tools
    FROM compatibility_matrices cm
    CROSS JOIN LATERAL jsonb_each(cm.tools) AS entry
    LEFT JOIN arch_map am
        ON lower(COALESCE(entry.value->>'Name', '')) = am.name_key
    GROUP BY cm.id
)
UPDATE compatibility_matrices cm
SET
    tools      = e.tools,
    updated_at = NOW()
FROM expanded e
WHERE cm.id = e.id;
