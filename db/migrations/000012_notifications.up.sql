CREATE TABLE IF NOT EXISTS notification_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id VARCHAR(255) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    config JSONB NOT NULL,
    events TEXT[] NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id VARCHAR(255) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    event VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    payload JSONB,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
