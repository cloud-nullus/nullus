-- 자주 사용되는 복합 쿼리용 인덱스

-- stacks: org_id + state 복합 (목록 조회 시 상태 필터)
CREATE INDEX IF NOT EXISTS idx_stacks_org_state ON stacks(org_id, state);

-- stacks: template_id + created_at (템플릿별 최신 스택)
CREATE INDEX IF NOT EXISTS idx_stacks_template_created ON stacks(template_id, created_at DESC);

-- pipeline_deployments: pipeline_id + started_at (이력 조회)
CREATE INDEX IF NOT EXISTS idx_pipeline_deployments_timeline ON pipeline_deployments(pipeline_id, started_at DESC);

-- alerts: severity + fired_at (심각도별 최신 알림)
CREATE INDEX IF NOT EXISTS idx_alerts_severity_time ON alerts(severity, fired_at DESC);

-- stack_config_versions: stack_id + version (버전 조회)
-- 이미 UNIQUE(stack_id, version) 있으므로 추가 불필요

-- users: org_id 통한 org_members 조회 최적화
CREATE INDEX IF NOT EXISTS idx_org_members_role ON org_members(org_id, role);

-- pipelines: org_id + status (조직별 활성 파이프라인)
CREATE INDEX IF NOT EXISTS idx_pipelines_org_status ON pipelines(org_id, status);
