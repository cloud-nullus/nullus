CREATE TABLE upgrade_bundles (
    id TEXT PRIMARY KEY,
    tool TEXT NOT NULL,
    release_name TEXT NOT NULL,
    chart_name TEXT NOT NULL,
    repo_url TEXT NOT NULL,
    source_chart_version TEXT NOT NULL,
    source_app_version TEXT NOT NULL DEFAULT '',
    target_chart_version TEXT NOT NULL,
    target_app_version TEXT NOT NULL DEFAULT '',
    risk_level TEXT NOT NULL CHECK (risk_level IN ('low', 'medium', 'high')),
    expected_disruption TEXT NOT NULL DEFAULT '',
    release_notes TEXT NOT NULL DEFAULT '',
    requires_backup BOOLEAN NOT NULL DEFAULT TRUE,
    supported_architectures JSONB NOT NULL DEFAULT '[]',
    status TEXT NOT NULL CHECK (status IN ('draft', 'verified', 'deprecated', 'revoked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tool, source_chart_version, target_chart_version)
);

CREATE TABLE stack_upgrade_runs (
    id TEXT PRIMARY KEY,
    stack_id TEXT NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,
    bundle_id TEXT NOT NULL REFERENCES upgrade_bundles(id),
    tool TEXT NOT NULL,
    status TEXT NOT NULL,
    current_step TEXT NOT NULL,
    requested_by TEXT NOT NULL,
    reason TEXT NOT NULL,
    idempotency_key TEXT,
    source_chart_version TEXT NOT NULL,
    target_chart_version TEXT NOT NULL,
    previous_revision INTEGER NOT NULL DEFAULT 0,
    result_revision INTEGER NOT NULL DEFAULT 0,
    checks JSONB NOT NULL DEFAULT '[]',
    error TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    UNIQUE (stack_id, idempotency_key)
);

CREATE INDEX idx_upgrade_bundles_path ON upgrade_bundles(tool, source_chart_version, status);
CREATE INDEX idx_stack_upgrade_runs_stack_started ON stack_upgrade_runs(stack_id, started_at DESC);
CREATE UNIQUE INDEX idx_stack_upgrade_runs_one_active
    ON stack_upgrade_runs(stack_id)
    WHERE status IN ('preflighting', 'upgrading', 'rolling_back');

-- A bundle becomes verified only after the Phase 0 real-cluster rehearsal.
-- Deliberately seed no executable version path here.
