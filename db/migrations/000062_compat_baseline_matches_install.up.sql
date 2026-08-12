-- 000062_compat_baseline_matches_install.up.sql
--
-- 호환성 매트릭스가 선언하는 버전을 Nullus 가 실제로 설치하는 차트 버전에 맞춘다.
--
-- 000042 는 외부 프로젝트 Narwhal(dasomel/narwhal)의 VERSIONS.md 를 기준선으로
-- 삼았다. 그 뒤 Nullus 의 설치 경로(internal/stack/adapter/helm/helm_step_metadata.go)가
-- 독자적으로 올라가면서 두 값이 갈라졌고, 화면은 "검증된 조합" 이라며 실제로는
-- 깔리지 않는 버전을 안내하게 됐다. 실측한 차이는 다음과 같다.
--
--   도구         매트릭스(000042)        실제 설치
--   GitLab       9.5.1  / 18.5.1         8.7.2  / v17.7.0
--   MinIO        5.2.0  / 2024-08-03     5.4.0  / 2024-12-18
--   Argo CD      6.8.0  / v2.8.3         7.7.16 / v2.13.3
--   Prometheus   67.0.0 / v2.54.1        69.3.0 / v3.1.0
--   Grafana      8.5.0  / 11.1.0         8.9.0  / 11.5.1
--   Harbor/Nexus (일치 — domain 상수를 참조하고 있었다)
--
-- Prometheus 의 app 버전은 차트 appVersion(v0.80.0, prometheus-operator)이 아니라
-- 실제로 서는 Prometheus 서버 버전을 적는다. 사용자가 알아야 하는 값은 그쪽이다.
--
-- 이제 이 값들의 출처는 domain 상수 하나다(internal/stack/domain/connection.go).
-- 버전을 올릴 때는 그 상수와 이 마이그레이션을 같은 커밋에서 함께 고친다.
-- (고정: TestChartVersionsMatchCompatibilityMatrix)
--
-- 000042 처럼 tools 블롭을 통째로 덮지 않고 도구 이름으로 찾아 버전 필드만
-- 바꾼다. 그 사이 다른 마이그레이션이 바꿔 둔 값(000060 의 GHCR 전환, 000041 의
-- Tier/ArchSupport)을 되돌리지 않기 위해서다.

BEGIN;

CREATE TEMP TABLE compat_baseline_v2 (
    tool_name    text PRIMARY KEY,
    helm_version text NOT NULL,
    app_version  text NOT NULL
) ON COMMIT DROP;

INSERT INTO compat_baseline_v2 (tool_name, helm_version, app_version) VALUES
    ('GitLab CE',       '8.7.2',  'v17.7.0'),
    ('GitLab CI',       '8.7.2',  'v17.7.0'),
    ('GitLab Registry', '8.7.2',  'v17.7.0'),
    ('Harbor',          '1.15.0', '2.11.0'),
    ('Nexus',           '64.2.0', '3.64.0'),
    ('MinIO',           '5.4.0',  'RELEASE.2024-12-18T13-15-44Z'),
    ('Argo CD',         '7.7.16', 'v2.13.3'),
    ('Prometheus',      '69.3.0', 'v3.1.0'),
    ('Grafana',         '8.9.0',  '11.5.1');

-- ------------------------------------------------------------
-- 1. compatibility_matrices.tools — 카테고리를 키로 갖는 객체
-- ------------------------------------------------------------

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
        LEFT JOIN compat_baseline_v2 b ON b.tool_name = e.value->>'Name'
    ),
    updated_at = NOW()
WHERE m.tools IS NOT NULL
  AND EXISTS (
        SELECT 1
        FROM jsonb_each(m.tools) AS e
        JOIN compat_baseline_v2 b ON b.tool_name = e.value->>'Name'
  );

-- ------------------------------------------------------------
-- 2. golden_path_templates.tools — 배열 + snake_case 키
--    순서를 지켜야 화면의 도구 나열 순서가 바뀌지 않는다.
-- ------------------------------------------------------------

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
        LEFT JOIN compat_baseline_v2 b ON b.tool_name = elem.value->>'name'
    ),
    updated_at = NOW()
WHERE t.tools IS NOT NULL
  AND EXISTS (
        SELECT 1
        FROM jsonb_array_elements(t.tools) AS elem
        JOIN compat_baseline_v2 b ON b.tool_name = elem->>'name'
  );

COMMIT;
