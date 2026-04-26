DELETE FROM pipeline_templates WHERE id IN ('nullus-sample-backend-v1', 'nullus-sample-frontend-v1');
ALTER TABLE pipeline_templates DROP COLUMN IF EXISTS docker_context;
ALTER TABLE pipeline_templates DROP COLUMN IF EXISTS dockerfile_path;
ALTER TABLE pipeline_templates DROP COLUMN IF EXISTS git_repo_url;
