-- Pipeline Templates
CREATE TABLE pipeline_templates (
    id          VARCHAR(100) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    app_type    VARCHAR(50)  NOT NULL,
    stages      JSONB        NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Pipelines
CREATE TYPE pipeline_status AS ENUM ('active', 'inactive');
CREATE TYPE pipeline_app_type AS ENUM ('web', 'backend', 'batch');

CREATE TABLE pipelines (
    id           VARCHAR(100) PRIMARY KEY,
    name         VARCHAR(255) NOT NULL,
    template_id  VARCHAR(100),
    org_id       VARCHAR(100) NOT NULL,
    cluster_id   VARCHAR(100) NOT NULL,
    namespace    VARCHAR(255) NOT NULL DEFAULT 'default',
    app_type     pipeline_app_type NOT NULL,
    git_repo_url VARCHAR(512),
    status       pipeline_status NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pipelines_org_id ON pipelines(org_id);
CREATE INDEX idx_pipelines_status ON pipelines(status);

-- Pipeline Deployments
CREATE TYPE deployment_status AS ENUM ('pending', 'running', 'success', 'failed', 'rolled_back');

CREATE TABLE pipeline_deployments (
    id           VARCHAR(100) PRIMARY KEY,
    pipeline_id  VARCHAR(100) NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    version      VARCHAR(255) NOT NULL,
    status       deployment_status NOT NULL DEFAULT 'pending',
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    deployed_by  VARCHAR(255)
);

CREATE INDEX idx_pipeline_deployments_pipeline_id ON pipeline_deployments(pipeline_id);
CREATE INDEX idx_pipeline_deployments_status ON pipeline_deployments(status);
