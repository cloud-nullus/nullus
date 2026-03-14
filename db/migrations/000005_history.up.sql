CREATE TABLE stack_config_versions (
    id             VARCHAR(100) PRIMARY KEY,
    stack_id       VARCHAR(100) NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,
    version        INTEGER      NOT NULL,
    config         JSONB        NOT NULL DEFAULT '{}',
    changed_by     VARCHAR(255) NOT NULL,
    change_reason  TEXT,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (stack_id, version)
);

CREATE INDEX idx_stack_config_versions_stack_id ON stack_config_versions(stack_id);
CREATE INDEX idx_stack_config_versions_created_at ON stack_config_versions(created_at DESC);
