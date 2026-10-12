-- Jenkins 앱 버전을 2.568.2 → 2.568.3 으로 올린다 (릴리즈 정책 §7.4 — 2026-10-12 기준선 CRITICAL 수정).
--
-- 스택이 실제로 까는 이미지 태그는 코드 상수(internal/stack/adapter/helm/values.go 의 jenkinsImageTag)와
-- 차트 values 가 정하고, 여기 시드된 템플릿·호환성 행의 app_version 은 화면과 설치 계획이 보여 주는 값이다.
-- 둘이 어긋나면 사용자는 2.568.2 를 고른 줄 알고 2.568.3 을 받는다. 과거 시드 마이그레이션(000069·000072)은
-- 이미 적용된 이력이라 고치지 않고, 새 마이그레이션이 값을 옮긴다.
--
-- JSONB 를 텍스트로 바꿔 치환한다 — 두 테이블의 tools 는 모양이 다르다(배열의 "app_version" 과 객체의 "AppVersion").
-- 해당 문자열이 없는 행은 그대로다(idempotent).
UPDATE golden_path_templates
   SET tools = replace(tools::text, '"app_version": "2.568.2"', '"app_version": "2.568.3"')::jsonb
 WHERE tools::text LIKE '%"app_version": "2.568.2"%';

UPDATE compatibility_matrices
   SET tools = replace(tools::text, '"AppVersion": "2.568.2"', '"AppVersion": "2.568.3"')::jsonb
 WHERE tools::text LIKE '%"AppVersion": "2.568.2"%';
