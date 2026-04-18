UPDATE stack_resource_defaults
SET cpu_request = 0.50,
    cpu_limit = 1.00,
    memory_request_gi = 0.50,
    memory_limit_gi = 1.00,
    updated_at = NOW()
WHERE tool_key = 'cert-manager';

UPDATE stack_resource_defaults
SET cpu_request = 1.00,
    cpu_limit = 2.00,
    memory_request_gi = 2.00,
    memory_limit_gi = 4.00,
    updated_at = NOW()
WHERE tool_key = 'minio';

UPDATE stack_resource_defaults
SET cpu_request = 2.00,
    cpu_limit = 4.00,
    memory_request_gi = 3.00,
    memory_limit_gi = 6.00,
    updated_at = NOW()
WHERE tool_key = 'argocd';

UPDATE stack_resource_defaults
SET cpu_request = 1.00,
    cpu_limit = 2.00,
    memory_request_gi = 2.00,
    memory_limit_gi = 4.00,
    updated_at = NOW()
WHERE tool_key = 'grafana';

UPDATE stack_resource_defaults
SET cpu_request = 1.00,
    cpu_limit = 2.00,
    memory_request_gi = 2.00,
    memory_limit_gi = 4.00,
    updated_at = NOW()
WHERE tool_key = 'opentelemetry';

UPDATE stack_resource_defaults
SET cpu_request = 1.00,
    cpu_limit = 2.00,
    memory_request_gi = 2.00,
    memory_limit_gi = 4.00,
    updated_at = NOW()
WHERE tool_key = 'cnpg';
