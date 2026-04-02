ALTER TABLE pipeline_templates ADD COLUMN IF NOT EXISTS git_repo_url VARCHAR(500) DEFAULT '';
ALTER TABLE pipeline_templates ADD COLUMN IF NOT EXISTS dockerfile_path VARCHAR(500) DEFAULT '';
ALTER TABLE pipeline_templates ADD COLUMN IF NOT EXISTS docker_context VARCHAR(500) DEFAULT '';

INSERT INTO pipeline_templates (id, name, description, app_type, stages, git_repo_url, dockerfile_path, docker_context)
VALUES
(
    'nullus-sample-backend-v1',
    'Nullus Sample App — Backend',
    'Go API server for the Nullus platform demo. Builds from backend/Dockerfile and deploys to Kubernetes.',
    'backend',
    '["GitClone", "DockerBuild", "ImageLoad", "Deploy"]'::jsonb,
    'https://github.com/cloud-nullus/nullus-sample-app',
    'backend/Dockerfile',
    'backend/'
),
(
    'nullus-sample-frontend-v1',
    'Nullus Sample App — Frontend',
    'React SPA for the Nullus platform demo. Builds from frontend/Dockerfile (Nginx) and deploys to Kubernetes.',
    'web',
    '["GitClone", "DockerBuild", "ImageLoad", "Deploy"]'::jsonb,
    'https://github.com/cloud-nullus/nullus-sample-app',
    'frontend/Dockerfile',
    'frontend/'
);
