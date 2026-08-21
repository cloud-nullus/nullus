-- 000074_stack_deploy_logs.up.sql
-- 설치 로그를 프로세스 밖에 남긴다.
--
-- 지금까지 로그는 API 프로세스 메모리에만 있었다. 설치는 20~30분짜리라 그 사이
-- 파드가 재시작되면 로그가 통째로 사라지고, 무엇이 왜 멈췄는지 사후에 알 방법이
-- 없었다 — 2026-08-21 운영에서 그렇게 됐다.
CREATE TABLE IF NOT EXISTS stack_deploy_logs (
    -- seq 는 기록 순서다. 타임스탬프는 같은 밀리초에 여러 줄이 들어올 수 있어
    -- 정렬 기준으로 쓸 수 없다.
    seq           BIGSERIAL PRIMARY KEY,
    deployment_id TEXT        NOT NULL,
    logged_at     TIMESTAMPTZ NOT NULL,
    level         TEXT        NOT NULL,
    step          TEXT        NOT NULL DEFAULT '',
    phase         TEXT        NOT NULL DEFAULT '',
    message       TEXT        NOT NULL
);

-- 조회는 언제나 "이 배포의 로그를 순서대로" 다.
CREATE INDEX IF NOT EXISTS idx_stack_deploy_logs_deployment
    ON stack_deploy_logs (deployment_id, seq);
