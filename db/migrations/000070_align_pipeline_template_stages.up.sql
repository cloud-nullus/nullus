-- 000070_align_pipeline_template_stages.up.sql
-- 스캐폴딩이 만드는 파이프라인과 템플릿이 선언한 단계를 맞춘다.
--
-- 템플릿은 Build/Test/ImageBuild/Deploy 를 선언했지만 스캐폴딩(.gitlab-ci.yml,
-- Jenkinsfile)은 Build 와 Deploy 두 단계만 만든다. 그래서 화면이 돌지도 않은
-- Test 를 성공으로 보여줬다 — 실행되지 않은 일을 성공이라 말하면 안 된다.
--
-- 단계 이름의 출처는 렌더러다(scaffold.PipelineStageNames).
-- 고정: TestPipelineStageNames_MatchRenderedJenkinsfile,
--       TestSeededTemplateStages_MatchScaffold
--
-- batch-job-v1 의 CronJobDeploy 도 함께 정리한다. 스캐폴딩은 app_type 과 무관하게
-- 같은 파이프라인을 만들며 CronJob 배포는 아직 구현되어 있지 않다 — 선언만 남겨
-- 두면 없는 기능을 약속하는 셈이 된다.
UPDATE pipeline_templates
SET stages = '["Build", "Deploy"]'::jsonb
WHERE id IN ('web-backend-v1', 'batch-job-v1');
