DROP INDEX IF EXISTS idx_stacks_org_id;
DROP INDEX IF EXISTS idx_stacks_cluster_id;
DROP INDEX IF EXISTS idx_stacks_state;

CREATE INDEX IF NOT EXISTS idx_stacks_org_id ON stacks(org_id);
CREATE INDEX IF NOT EXISTS idx_stacks_cluster_id ON stacks(cluster_id);
CREATE INDEX IF NOT EXISTS idx_stacks_state ON stacks(state);

ALTER TABLE stacks
    DROP COLUMN IF EXISTS deleted_at;
