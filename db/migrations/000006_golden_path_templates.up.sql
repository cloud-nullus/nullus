-- Golden Path Templates
CREATE TABLE golden_path_templates (
    id VARCHAR(100) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    tools JSONB NOT NULL,
    estimated_install_time INTEGER NOT NULL,
    recommended_use_case VARCHAR(255) NOT NULL,
    min_resources VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_golden_path_templates_name ON golden_path_templates(name);
