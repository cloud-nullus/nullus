CREATE TABLE IF NOT EXISTS org_resource_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    base_profile VARCHAR(32) NOT NULL CHECK (base_profile IN ('local', 'startup', 'standard', 'enterprise')),
    option_overrides JSONB NOT NULL DEFAULT '{}'::jsonb,
    applied_resource_overrides JSONB NOT NULL DEFAULT '{}'::jsonb,
    row_units JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_org_resource_profiles_org_created
    ON org_resource_profiles (org_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uk_org_resource_profiles_org_name
    ON org_resource_profiles (org_id, lower(name));
