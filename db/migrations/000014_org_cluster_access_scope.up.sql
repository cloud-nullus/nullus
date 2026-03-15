ALTER TABLE organizations ADD COLUMN IF NOT EXISTS cluster_access_scope TEXT[] DEFAULT '{}';
