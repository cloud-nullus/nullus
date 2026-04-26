ALTER TABLE pipelines
  ALTER COLUMN org_id TYPE UUID USING org_id::uuid,
  ALTER COLUMN cluster_id TYPE UUID USING cluster_id::uuid;

ALTER TABLE pipelines
  ADD COLUMN stack_id VARCHAR(100) REFERENCES stacks(id) ON DELETE SET NULL;

ALTER TABLE pipelines
  ADD CONSTRAINT fk_pipelines_org FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
  ADD CONSTRAINT fk_pipelines_cluster FOREIGN KEY (cluster_id) REFERENCES clusters(id);

CREATE INDEX idx_pipelines_stack_id ON pipelines(stack_id);
