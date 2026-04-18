ALTER TABLE stacks
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

DROP INDEX IF EXISTS idx_stacks_org_id;
DROP INDEX IF EXISTS idx_stacks_cluster_id;
DROP INDEX IF EXISTS idx_stacks_state;

CREATE INDEX IF NOT EXISTS idx_stacks_org_id ON stacks(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_stacks_cluster_id ON stacks(cluster_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_stacks_state ON stacks(state) WHERE deleted_at IS NULL;
