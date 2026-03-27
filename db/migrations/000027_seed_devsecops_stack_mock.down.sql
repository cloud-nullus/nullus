DELETE FROM stack_config_versions
WHERE stack_id IN (
  'mock-devsecops-enterprise',
  'mock-devsecops-gitops',
  'mock-devsecops-lean'
);

DELETE FROM stacks
WHERE id IN (
  'mock-devsecops-enterprise',
  'mock-devsecops-gitops',
  'mock-devsecops-lean'
);
