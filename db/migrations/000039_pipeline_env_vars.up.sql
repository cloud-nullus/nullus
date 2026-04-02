ALTER TABLE pipeline_templates ADD COLUMN IF NOT EXISTS env_vars JSONB DEFAULT '{}';
ALTER TABLE pipelines ADD COLUMN IF NOT EXISTS env_vars JSONB DEFAULT '{}';

UPDATE pipeline_templates
SET env_vars = '{"BACKEND_HOST": "sample-backend:8080"}'::jsonb
WHERE id = 'nullus-sample-frontend-v1';
