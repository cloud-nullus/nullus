-- 000101_template_source.up.sql
-- 템플릿이 외부 원본을 새 앱 저장소로 가져온다 (nullus-plan#91, #92).
--
-- 원본은 fork 하지 않고 고정한 커밋의 파일을 복사해 앱 저장소의 첫 커밋으로 둔다.
-- 가져온 뒤에는 원본을 따라가지 않는다 — 새 버전은 템플릿의 source_commit 을 올렸을 때
-- 새로 만드는 파이프라인부터 들어온다.
--
-- source_commit 은 커밋 SHA 다. 태그는 다시 찍힌다(eGov 웹 샘플의 v4.3.0 은 2025-03 릴리스인데
-- 2026-05 커밋을 가리킨다). source_label 은 화면에 보일 버전 이름일 뿐 가져올 대상을 정하지 않는다.
--
-- 기존 git_repo_url 을 재사용하지 않는다. nullus-sample-* 템플릿에도 값이 있어 의미를 바꾸면
-- 그쪽이 가져오기 경로를 탄다(모노레포라 Dockerfile 이 backend/ 아래에 있어 깨진다).
ALTER TABLE pipeline_templates
    ADD COLUMN IF NOT EXISTS source_repo_url      VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_commit        VARCHAR(40)  NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_label         VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_build_profile VARCHAR(50)  NOT NULL DEFAULT '';

-- 파이프라인은 처음 가져온 원본을 기록한다. 템플릿의 source_commit 은 시간이 지나면 올라가므로
-- 저장소가 어느 버전에서 시작했는지는 여기에만 남는다.
ALTER TABLE pipelines
    ADD COLUMN IF NOT EXISTS source_repo_url      VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_commit        VARCHAR(40)  NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_label         VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_build_profile VARCHAR(50)  NOT NULL DEFAULT '';

-- 전자정부 표준프레임워크 샘플 두 개. 둘 다 내장 HSQLDB 라 DB 를 따로 띄우지 않는다.
-- MSA 템플릿은 자원을 많이 써서 넣지 않는다(2026-10-18 회의).
--
-- 단계는 스캐폴딩의 기본 단계다. 이미지 스캔·SAST 같은 선택 단계는 파이프라인이 정한다(000070).
INSERT INTO pipeline_templates
    (id, name, description, app_type, stages,
     source_repo_url, source_commit, source_label, source_build_profile)
VALUES
(
    'egovframe-boot-sample-v1',
    'eGovFrame Boot Sample (Spring Boot)',
    '전자정부 표준프레임워크 부트 기반 심플 게시판(Java Config). 원본 v5.0.0 을 가져와 원본 Dockerfile 로 빌드하고, 내장 HSQLDB 로 바로 실행됩니다. 원본을 받으려면 인터넷 연결이 필요합니다.',
    'backend',
    '["Build", "Deploy"]'::jsonb,
    'https://github.com/eGovFramework/egovframe-boot-sample-java-config',
    'c44e4bc8ea1e1b1e83e0aaab1037fbf04c761feb',
    'v5.0.0',
    'spring-boot'
),
(
    'egovframe-web-sample-v1',
    'eGovFrame Web Sample (JSP)',
    '전자정부 표준프레임워크 웹 기반 심플 게시판(XML 설정, JSP). 원본 v5.0.0 을 가져와 Maven 으로 WAR 를 만들고 Tomcat 에서 내장 HSQLDB 로 실행됩니다. 원본을 받으려면 인터넷 연결이 필요합니다.',
    'backend',
    '["Build", "Deploy"]'::jsonb,
    'https://github.com/eGovFramework/egovframe-web-sample',
    '158daebd2bfdab951e90ad526c3d2a6b6b42513b',
    'v5.0.0',
    'maven-war'
)
ON CONFLICT (id) DO NOTHING;
