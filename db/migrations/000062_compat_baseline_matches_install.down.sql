-- 000062_compat_baseline_matches_install.down.sql
--
-- 000042 의 Narwhal baseline v1 버전으로 되돌린다.
-- up 과 같은 방식으로 도구 이름을 찾아 버전 필드만 바꾼다.

BEGIN;

CREATE TEMP TABLE compat_baseline_v1 (
    tool_name    text PRIMARY KEY,
    helm_version text NOT NULL,
    app_version  text NOT NULL
) ON COMMIT DROP;

INSERT INTO compat_baseline_v1 (tool_name, helm_version, app_version) VALUES
    ('GitLab CE',       '9.5.1',  '18.5.1'),
    ('GitLab CI',       '9.5.1',  '18.5.1'),
    ('GitLab Registry', '9.5.1',  '18.5.1'),
    ('Harbor',          '1.15.0', '2.11.0'),
    ('Nexus',           '64.2.0', '3.64.0'),
    ('MinIO',           '5.2.0',  'RELEASE.2024-08-03T04-33-23Z'),
    ('Argo CD',         '6.8.0',  'v2.8.3'),
    ('Prometheus',      '67.0.0', 'v2.54.1'),
    ('Grafana',         '8.5.0',  '11.1.0');

UPDATE compatibility_matrices m
SET tools = (
        SELECT jsonb_object_agg(
                   e.key,
                   CASE
                       WHEN b.tool_name IS NULL THEN e.value
                       ELSE e.value || jsonb_build_object(
                           'HelmVersion', b.helm_version,
                           'AppVersion',  b.app_version)
                   END)
        FROM jsonb_each(m.tools) AS e
        LEFT JOIN compat_baseline_v1 b ON b.tool_name = e.value->>'Name'
    ),
    updated_at = NOW()
WHERE m.tools IS NOT NULL
  AND EXISTS (
        SELECT 1
        FROM jsonb_each(m.tools) AS e
        JOIN compat_baseline_v1 b ON b.tool_name = e.value->>'Name'
  );

UPDATE golden_path_templates t
SET tools = (
        SELECT jsonb_agg(
                   CASE
                       WHEN b.tool_name IS NULL THEN elem.value
                       ELSE elem.value || jsonb_build_object(
                           'helm_version', b.helm_version,
                           'app_version',  b.app_version)
                   END
                   ORDER BY elem.ordinality)
        FROM jsonb_array_elements(t.tools) WITH ORDINALITY AS elem(value, ordinality)
        LEFT JOIN compat_baseline_v1 b ON b.tool_name = elem.value->>'name'
    ),
    updated_at = NOW()
WHERE t.tools IS NOT NULL
  AND EXISTS (
        SELECT 1
        FROM jsonb_array_elements(t.tools) AS elem
        JOIN compat_baseline_v1 b ON b.tool_name = elem->>'name'
  );

COMMIT;
