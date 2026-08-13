-- 000065_helm_step_config_otel_agent.down.sql
-- 에이전트 단계를 카탈로그에서 빼고 게이트웨이 정렬값을 되돌린다.

DELETE FROM stack_helm_step_configs WHERE step_name = 'installing_otel_agent';

UPDATE stack_helm_step_configs SET sort_order = 17, updated_at = NOW()
WHERE step_name = 'installing_gateway';
