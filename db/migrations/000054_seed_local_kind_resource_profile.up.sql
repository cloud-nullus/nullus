INSERT INTO org_resource_profiles (
    org_id,
    name,
    base_profile,
    option_overrides,
    applied_resource_overrides,
    row_units
)
SELECT
    '11111111-1111-1111-1111-111111111111',
    'Local Kind',
    'local',
    '{}'::jsonb,
    '{
      "artifacts.packageRegistry:gitlab": {"cpuRequest": 0.50, "cpuLimit": 1.00, "memoryRequestGi": 1.00, "memoryLimitGi": 2.00, "storageRequestGi": 5.00, "storageLimitGi": 10.00},
      "artifacts.sourceRepository:gitlab": {"cpuRequest": 0.75, "cpuLimit": 1.50, "memoryRequestGi": 1.50, "memoryLimitGi": 3.00, "storageRequestGi": 10.00, "storageLimitGi": 20.00},
      "artifacts.containerRegistry:gitlab-registry": {"cpuRequest": 0.50, "cpuLimit": 0.50, "memoryRequestGi": 0.50, "memoryLimitGi": 0.60, "storageRequestGi": 5.00, "storageLimitGi": 10.00},
      "artifacts.storageBackend:minio": {"cpuRequest": 0.50, "cpuLimit": 0.50, "memoryRequestGi": 0.50, "memoryLimitGi": 1.00, "storageRequestGi": 20.00, "storageLimitGi": 40.00},
      "pipeline.cicdPlatform:gitlab-ci": {"cpuRequest": 0.50, "cpuLimit": 0.50, "memoryRequestGi": 0.50, "memoryLimitGi": 0.50, "storageRequestGi": 1.00, "storageLimitGi": 2.00},
      "pipeline.cdTool:argocd": {"cpuRequest": 0.50, "cpuLimit": 0.50, "memoryRequestGi": 0.50, "memoryLimitGi": 1.00, "storageRequestGi": 1.00, "storageLimitGi": 2.00},
      "monitoring.collection:prometheus": {"cpuRequest": 0.50, "cpuLimit": 0.50, "memoryRequestGi": 0.50, "memoryLimitGi": 0.50, "storageRequestGi": 2.00, "storageLimitGi": 4.00},
      "monitoring.visualization:grafana": {"cpuRequest": 0.50, "cpuLimit": 0.50, "memoryRequestGi": 0.50, "memoryLimitGi": 0.50, "storageRequestGi": 1.00, "storageLimitGi": 2.00}
    }'::jsonb,
    '{}'::jsonb
WHERE NOT EXISTS (
    SELECT 1
    FROM org_resource_profiles
    WHERE org_id = '11111111-1111-1111-1111-111111111111'
      AND lower(name) = 'local kind'
);
