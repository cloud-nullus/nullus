-- 000089_pipeline_template_created_by.up.sql
-- 화면에서 만든 CI/CD 템플릿의 만든 사람을 남긴다.
--
-- API 와 도메인(PipelineTemplate.CreatedBy)은 이 값을 받고 돌려주고 있었지만 열이 없어
-- 저장소가 버렸다. 시드 템플릿은 만든 사람이 없으므로 NULL 을 허용한다.
ALTER TABLE pipeline_templates ADD COLUMN IF NOT EXISTS created_by VARCHAR(255);
