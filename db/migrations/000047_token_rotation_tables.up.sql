CREATE TABLE IF NOT EXISTS token_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    module VARCHAR(50) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    path TEXT NOT NULL,
    token_type VARCHAR(30) NOT NULL,
    status VARCHAR(30) NOT NULL,
    expires_at TIMESTAMPTZ,
    last_rotated_at TIMESTAMPTZ,
    next_check_at TIMESTAMPTZ,
    requires_approval BOOLEAN NOT NULL DEFAULT FALSE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_token_sources_org_status
    ON token_sources (org_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_token_sources_next_check
    ON token_sources (next_check_at) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_token_sources_org_provider_path
    ON token_sources (org_id, provider, path) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS token_rotation_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_source_id UUID NOT NULL,
    event_type VARCHAR(30) NOT NULL,
    result VARCHAR(20) NOT NULL,
    reason_code VARCHAR(100),
    detail_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    trace_id VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_token_rotation_events_source
        FOREIGN KEY (token_source_id) REFERENCES token_sources(id)
);

CREATE INDEX IF NOT EXISTS idx_token_rotation_events_source_time
    ON token_rotation_events (token_source_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_token_rotation_events_result
    ON token_rotation_events (result, created_at DESC);
