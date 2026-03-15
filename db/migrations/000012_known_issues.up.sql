CREATE TABLE IF NOT EXISTS known_issues (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    severity    VARCHAR(20) NOT NULL DEFAULT 'medium',
    title       VARCHAR(500) NOT NULL,
    description TEXT NOT NULL,
    workaround  TEXT DEFAULT '',
    status      VARCHAR(20) NOT NULL DEFAULT 'open',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO known_issues (severity, title, description, workaround, status) VALUES
('medium', 'Helm install requires cluster admin', 'Helm-based stack installation currently requires cluster-admin role to create CRDs and cluster-scoped resources.', 'Use a temporary cluster-admin service account during installation, then rotate to least-privilege RBAC.', 'open'),
('low', 'Dashboard metrics delay', 'Prometheus cache TTL is 10s', 'Refresh page', 'acknowledged'),
('high', 'No automatic certificate renewal', 'Automatic certificate rotation is not wired into the current stack lifecycle jobs.', 'Manual cert-manager renewal', 'planned');
