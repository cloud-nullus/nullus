-- 000064_helm_step_config_otel_collector.up.sql
-- OpenTelemetry Collector 설치 단계를 Helm 단계 카탈로그에 등록한다.
--
-- 이 표가 없으면 설치는 코드의 기본 스펙으로 계속 돌지만(chartSpecForStep 이
-- 폴백한다), 관리자가 차트 버전을 표에서 고쳐도 수집기만 반영되지 않는다.
-- 두 출처가 갈라지는 상태를 남기지 않기 위해 함께 넣는다.
--
-- release_name 은 코드의 domain.OTelCollectorReleaseName 과 반드시 같아야 한다.
-- 다르면 설치는 이 이름으로 만들고 삭제는 저 이름을 찾아 릴리스가 고아로 남는다.

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
    ('installing_otel_collector', 'otel-collector', 'opentelemetry-collector', 'https://open-telemetry.github.io/opentelemetry-helm-charts', '0.75.0', NULL, 'C', 16, false, true)
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

-- 게이트웨이는 수집기 뒤에 온다. 목록을 sort_order 로 정렬해 보여주므로
-- 같은 값이 둘이면 설치 순서를 잘못 읽게 된다.
UPDATE stack_helm_step_configs SET sort_order = 17, updated_at = NOW()
WHERE step_name = 'installing_gateway';
