-- 설치된 OSS 업그레이드(설계: docs/11_기능설계/Nullus_OSS_업그레이드_설계.md).
--
-- upgrade_bundles 는 실클러스터 리허설(설계 §7 Phase 0)을 거친 업그레이드 경로다. verified 만
-- 실행할 수 있다. 여기서는 일부러 아무 경로도 넣지 않는다 — 리허설 없이 실행 가능한 경로가
-- 생기면 안 된다. 등록 방법은 설계 §11.1.
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

CREATE INDEX idx_upgrade_bundles_path ON upgrade_bundles(tool, source_chart_version, status);

-- stack_upgrade_runs 는 실행 하나다 — 막힌 시도까지 감사 기록으로 남긴다.
-- updated_at 은 실행 중인 프로세스의 심장 박동이다(1분마다). 진행 중인데 박동이 끊기면 API 가
-- 작업 중에 죽은 것으로 보고 수동 개입 필요로 닫는다(domain.UpgradeHeartbeatTimeout).
CREATE TABLE stack_upgrade_runs (
    id TEXT PRIMARY KEY,
    stack_id VARCHAR(100) NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,
    bundle_id TEXT NOT NULL REFERENCES upgrade_bundles(id),
    tool TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('blocked', 'upgrading', 'failed', 'succeeded', 'rolling_back', 'rolled_back', 'manual_intervention_required')),
    current_step TEXT NOT NULL,
    requested_by TEXT NOT NULL,
    reason TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    source_chart_version TEXT NOT NULL,
    target_chart_version TEXT NOT NULL,
    previous_revision INTEGER NOT NULL DEFAULT 0,
    result_revision INTEGER NOT NULL DEFAULT 0,
    checks JSONB NOT NULL DEFAULT '[]',
    error TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    CONSTRAINT uq_stack_upgrade_runs_idempotency UNIQUE (stack_id, idempotency_key)
);

CREATE INDEX idx_stack_upgrade_runs_stack_started ON stack_upgrade_runs(stack_id, started_at DESC);

-- 같은 스택에 진행 중 실행은 하나뿐이다. 레플리카 둘에 동시에 들어온 요청도 여기서 갈린다.
CREATE UNIQUE INDEX uq_stack_upgrade_runs_active
    ON stack_upgrade_runs(stack_id)
    WHERE status IN ('upgrading', 'rolling_back');
