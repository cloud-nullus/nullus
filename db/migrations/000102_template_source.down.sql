-- 000101_template_source.down.sql
DELETE FROM pipeline_templates
WHERE id IN ('egovframe-boot-sample-v1', 'egovframe-web-sample-v1');

ALTER TABLE pipelines
    DROP COLUMN IF EXISTS source_build_profile,
    DROP COLUMN IF EXISTS source_label,
    DROP COLUMN IF EXISTS source_commit,
    DROP COLUMN IF EXISTS source_repo_url;

ALTER TABLE pipeline_templates
    DROP COLUMN IF EXISTS source_build_profile,
    DROP COLUMN IF EXISTS source_label,
    DROP COLUMN IF EXISTS source_commit,
    DROP COLUMN IF EXISTS source_repo_url;
