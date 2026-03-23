CREATE TABLE IF NOT EXISTS stack_resource_defaults (
    tool_key VARCHAR(100) PRIMARY KEY,
    display_name VARCHAR(255) NOT NULL,
    cpu_request NUMERIC(10,2) NOT NULL,
    cpu_limit NUMERIC(10,2) NOT NULL,
    memory_request_gi NUMERIC(10,2) NOT NULL,
    memory_limit_gi NUMERIC(10,2) NOT NULL,
    storage_request_gi NUMERIC(10,2) NOT NULL,
    storage_limit_gi NUMERIC(10,2) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO stack_resource_defaults (
    tool_key,
    display_name,
    cpu_request,
    cpu_limit,
    memory_request_gi,
    memory_limit_gi,
    storage_request_gi,
    storage_limit_gi,
    is_default
)
VALUES
    ('gitlab-ce', 'GitLab CE', 4.00, 8.00, 8.00, 16.00, 30.00, 60.00, true),
    ('gitlab-runner', 'GitLab Runner', 2.00, 4.00, 4.00, 8.00, 10.00, 20.00, true),
    ('gitlab-registry', 'GitLab Registry', 0.50, 1.00, 1.00, 2.00, 20.00, 40.00, true),
    ('harbor', 'Harbor', 2.00, 4.00, 4.00, 8.00, 40.00, 80.00, true),
    ('minio', 'MinIO', 0.50, 1.00, 1.00, 2.00, 50.00, 100.00, true),
    ('argocd', 'Argo CD', 1.00, 2.00, 2.00, 4.00, 5.00, 10.00, true),
    ('prometheus', 'Prometheus', 1.00, 2.00, 4.00, 8.00, 20.00, 40.00, true),
    ('grafana', 'Grafana', 0.50, 1.00, 1.00, 2.00, 5.00, 10.00, true),
    ('opensearch', 'OpenSearch', 2.00, 4.00, 4.00, 8.00, 30.00, 60.00, true),
    ('opentelemetry', 'OpenTelemetry', 0.50, 1.00, 1.00, 2.00, 0.00, 0.00, true),
    ('cert-manager', 'Cert-Manager', 0.25, 0.50, 0.25, 0.50, 0.00, 0.00, true),
    ('cnpg', 'CloudNativePG', 0.50, 1.00, 1.00, 2.00, 10.00, 20.00, true)
ON CONFLICT (tool_key) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    cpu_request = EXCLUDED.cpu_request,
    cpu_limit = EXCLUDED.cpu_limit,
    memory_request_gi = EXCLUDED.memory_request_gi,
    memory_limit_gi = EXCLUDED.memory_limit_gi,
    storage_request_gi = EXCLUDED.storage_request_gi,
    storage_limit_gi = EXCLUDED.storage_limit_gi,
    is_default = EXCLUDED.is_default,
    updated_at = NOW();
