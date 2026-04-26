-- Add org_id column to users table for direct organization membership lookup.
-- The existing org_members join table remains for potential multi-org support later.
ALTER TABLE users ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_users_org_id ON users(org_id);
