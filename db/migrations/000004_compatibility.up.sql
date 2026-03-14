CREATE TABLE compatibility_matrices (
    id          VARCHAR(100) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    status      VARCHAR(50)  NOT NULL DEFAULT 'untested',
    k8s_min     VARCHAR(20)  NOT NULL,
    k8s_max     VARCHAR(20)  NOT NULL,
    k8s_recommended VARCHAR(20) NOT NULL,
    tools       JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_compatibility_matrices_status ON compatibility_matrices(status);
