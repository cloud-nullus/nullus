-- Nexus 앱 버전을 3.64.0 → 3.76.1 로 올린다.
--
-- 3.64.0 은 Sonatype 지원이 끝났고(2025-07-09 일몰) 비인증 경로 탐색(CVE-2024-4956) 등
-- 미패치 취약점이 있다. 3.77.0 부터는 Community Edition EULA 가 제품 내장을 금지하므로
-- 그 직전 판인 3.76.1 로 올린다(internal/stack/domain/connection.go 의 NexusAppVersion).
--
-- 스택이 실제로 까는 이미지 태그는 코드(values.go 의 installing_nexus)가 정하고, 여기 시드된
-- 템플릿 · 호환성 행의 app_version 은 화면과 설치 계획이 보여 주는 값이다. 둘이 어긋나면
-- 사용자는 3.64.0 을 고른 줄 알고 3.76.1 을 받는다. 과거 시드 마이그레이션(000059 · 000062)은
-- 이미 적용된 이력이라 고치지 않고, 새 마이그레이션이 값을 옮긴다.
--
-- 이미 3.64.0 으로 설치된 스택의 Nexus 는 바뀌지 않는다 — 3.71.0 부터 OrientDB 를 열지 못해,
-- 설치 단계가 떠 있는 OrientDB 시대 태그를 그대로 둔다(domain.NexusImageTagForRelease).
--
-- 도구 이름이 Nexus 이고 3.64.0 인 항목만 바꾼다. 다른 도구 · 이미 바뀐 행은 그대로다(idempotent).
-- 배열(템플릿)은 순서를 지켜야 화면의 도구 나열 순서가 바뀌지 않는다.
UPDATE golden_path_templates t
   SET tools = (
           SELECT jsonb_agg(
                      CASE
                          WHEN elem.value->>'name' = 'Nexus' AND elem.value->>'app_version' = '3.64.0'
                              THEN elem.value || jsonb_build_object('app_version', '3.76.1')
                          ELSE elem.value
                      END
                      ORDER BY elem.ordinality)
             FROM jsonb_array_elements(t.tools) WITH ORDINALITY AS elem(value, ordinality)
       ),
       updated_at = NOW()
 WHERE jsonb_typeof(t.tools) = 'array'
   AND EXISTS (
           SELECT 1
             FROM jsonb_array_elements(t.tools) AS elem
            WHERE elem->>'name' = 'Nexus' AND elem->>'app_version' = '3.64.0'
       );

UPDATE compatibility_matrices m
   SET tools = (
           SELECT jsonb_object_agg(
                      e.key,
                      CASE
                          WHEN e.value->>'Name' = 'Nexus' AND e.value->>'AppVersion' = '3.64.0'
                              THEN e.value || jsonb_build_object('AppVersion', '3.76.1')
                          ELSE e.value
                      END)
             FROM jsonb_each(m.tools) AS e
       ),
       updated_at = NOW()
 WHERE jsonb_typeof(m.tools) = 'object'
   AND EXISTS (
           SELECT 1
             FROM jsonb_each(m.tools) AS e
            WHERE e.value->>'Name' = 'Nexus' AND e.value->>'AppVersion' = '3.64.0'
       );
