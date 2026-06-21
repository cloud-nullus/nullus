CREATE TABLE IF NOT EXISTS stack_helm_step_configs (
    step_name    VARCHAR(100) PRIMARY KEY,
    release_name VARCHAR(255),
    chart_name   VARCHAR(255) NOT NULL,
    repo_url     VARCHAR(512),
    version      VARCHAR(100),
    namespace    VARCHAR(255),
    phase        VARCHAR(10) NOT NULL,
    sort_order   SMALLINT NOT NULL,
    wait         BOOLEAN NOT NULL DEFAULT false,
    is_enabled   BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO stack_helm_step_configs (
    step_name,
    release_name,
    chart_name,
    repo_url,
    version,
    namespace,
    phase,
    sort_order,
    wait,
    is_enabled
)
VALUES
    ('installing_cert_manager', NULL, 'cert-manager', 'https://charts.jetstack.io', 'v1.16.3', 'cert-manager', 'A', 0, false, true),
    ('installing_metrics_server', NULL, 'metrics-server', 'https://kubernetes-sigs.github.io/metrics-server/', '3.12.2', NULL, 'A', 1, false, true),
    ('installing_postgresql', 'nullus-postgresql', 'postgresql', 'https://charts.bitnami.com/bitnami', NULL, NULL, 'A', 2, false, true),
    ('installing_minio', 'nullus-minio', 'minio', 'https://charts.min.io/', '5.4.0', NULL, 'A', 3, false, true),
    ('installing_gitlab', NULL, 'gitlab', 'https://charts.gitlab.io/', '8.7.2', NULL, 'B', 8, false, true),
    ('installing_argocd', NULL, 'argo-cd', 'https://argoproj.github.io/argo-helm', '7.7.16', NULL, 'B', 9, false, true),
    ('installing_runner', NULL, 'gitlab-runner', 'https://charts.gitlab.io/', '0.72.0', NULL, 'B', 10, false, true),
    ('installing_prometheus', NULL, 'kube-prometheus-stack', 'https://prometheus-community.github.io/helm-charts', '69.3.0', NULL, 'C', 11, false, true),
    ('installing_grafana', NULL, 'grafana', 'https://grafana.github.io/helm-charts', '8.9.0', NULL, 'C', 12, false, true),
    ('installing_logging', NULL, 'loki', 'https://grafana.github.io/helm-charts', '2.10.3', NULL, 'C', 13, false, true),
    ('installing_log_search', NULL, 'opensearch', 'https://opensearch-project.github.io/helm-charts', '2.22.0', NULL, 'C', 14, false, true),
    ('installing_opentelemetry', NULL, 'opentelemetry-collector', 'https://open-telemetry.github.io/opentelemetry-helm-charts', '0.75.0', NULL, 'C', 15, false, true),
    ('installing_gateway', 'eg', 'oci://docker.io/envoyproxy/gateway-helm', NULL, '1.4.3', NULL, 'C', 16, false, true)
ON CONFLICT (step_name) DO UPDATE SET
    release_name = EXCLUDED.release_name,
    chart_name = EXCLUDED.chart_name,
    repo_url = EXCLUDED.repo_url,
    version = EXCLUDED.version,
    namespace = EXCLUDED.namespace,
    phase = EXCLUDED.phase,
    sort_order = EXCLUDED.sort_order,
    wait = EXCLUDED.wait,
    is_enabled = EXCLUDED.is_enabled,
    updated_at = NOW();
