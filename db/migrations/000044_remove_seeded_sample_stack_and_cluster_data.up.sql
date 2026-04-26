-- Remove seeded sample Stack/Cluster records so fresh local environments start clean.
-- This migration intentionally targets known seed IDs to avoid touching user-created data.

-- 1) Remove seeded stack history + stacks
DELETE FROM stack_config_versions
WHERE stack_id IN (
  'production-stack',
  'development-stack',
  'staging-environment',
  'microservices-platform',
  'mock-devsecops-enterprise',
  'mock-devsecops-gitops',
  'mock-devsecops-lean'
);

DELETE FROM stacks
WHERE id IN (
  'production-stack',
  'development-stack',
  'staging-environment',
  'microservices-platform',
  'mock-devsecops-enterprise',
  'mock-devsecops-gitops',
  'mock-devsecops-lean'
);

-- 2) Remove seeded pipelines that reference seeded clusters
--    (pipeline_deployments are removed automatically by ON DELETE CASCADE)
DELETE FROM pipelines
WHERE id IN (
  'frontend-web',
  'backend-api',
  'ml-service',
  'batch-runner'
);

-- 3) Remove seeded cluster names from access scope arrays
UPDATE organizations
SET cluster_access_scope = array_remove(
  array_remove(
    array_remove(
      array_remove(
        array_remove(COALESCE(cluster_access_scope, '{}'), 'kind-nullus-test'),
        'app-cluster-prod'
      ),
      'staging-cluster'
    ),
    'legacy-cluster'
  ),
  'acme-prod-cluster'
),
updated_at = NOW()
WHERE id IN (
  '11111111-1111-1111-1111-111111111111',
  '22222222-2222-2222-2222-222222222222'
);

-- 4) Remove seeded clusters
DELETE FROM clusters
WHERE id IN (
  '31111111-1111-1111-1111-111111111111',
  '32222222-2222-2222-2222-222222222222',
  '35555555-5555-5555-5555-555555555555',
  '36666666-6666-6666-6666-666666666666',
  '37777777-7777-7777-7777-777777777777'
);
