DROP INDEX IF EXISTS idx_pipelines_stack_id;

ALTER TABLE pipelines
  DROP CONSTRAINT IF EXISTS fk_pipelines_cluster,
  DROP CONSTRAINT IF EXISTS fk_pipelines_org;

ALTER TABLE pipelines DROP COLUMN IF EXISTS stack_id;

ALTER TABLE pipelines
  ALTER COLUMN org_id TYPE VARCHAR(100),
  ALTER COLUMN cluster_id TYPE VARCHAR(100);
