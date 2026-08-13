-- 000065_helm_step_config_otel_agent.up.sql
-- 노드 로그 수집 에이전트 단계를 Helm 단계 카탈로그에 등록한다.
--
-- Loki 는 저장소일 뿐 파드 로그를 스스로 긁어오지 않는다. 컨테이너 로그는 노드의
-- /var/log/pods 아래 파일이라 그것을 읽는 주체가 노드마다 있어야 하고, 그래서
-- 게이트웨이(Deployment)와 별개로 DaemonSet 을 하나 더 세운다.
--
-- release_name 은 코드의 domain.OTelAgentReleaseName 과 반드시 같아야 한다.
-- 다르면 설치는 이 이름으로 만들고 삭제는 저 이름을 찾아 DaemonSet 과
-- ClusterRole 이 고아로 남는다.

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
    ('installing_otel_agent', 'otel-agent', 'opentelemetry-collector', 'https://open-telemetry.github.io/opentelemetry-helm-charts', '0.75.0', NULL, 'C', 17, false, true)
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

-- 게이트웨이는 에이전트 뒤로 밀린다. 목록을 sort_order 로 정렬해 보여주므로
-- 값이 겹치면 설치 순서를 잘못 읽게 된다.
UPDATE stack_helm_step_configs SET sort_order = 18, updated_at = NOW()
WHERE step_name = 'installing_gateway';
