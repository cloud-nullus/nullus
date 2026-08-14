-- 000071_deployment_steps.up.sql
-- 배포 기록에 단계 정보를 담는다.
--
-- 직접배포 경로는 단계를 메모리 트래커에서 서빙하지만, CI 서버에서 들인 실행은
-- 담을 곳이 없어 버려졌다 — 화면이 단계를 계속 "실행 정보 없음" 으로 표시했다.
ALTER TABLE pipeline_deployments
    ADD COLUMN IF NOT EXISTS steps JSONB NOT NULL DEFAULT '[]'::jsonb;
