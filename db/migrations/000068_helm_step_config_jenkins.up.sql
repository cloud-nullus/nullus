-- Jenkins 설치 스텝의 차트 설정.
--
-- 차트 버전을 임의로 내리면 안 된다. Gitea multibranch 스캔에 쓰는 jenkinsci/gitea
-- 플러그인이 Jenkins 2.528.3 이상을 요구하므로, appVersion 이 그 하한을 넘는
-- 차트여야 한다 — 5.9.54 는 2.568.2 라 요건을 만족한다.
--
-- sort_order 는 installing_runner(10) 바로 뒤다. CI 슬롯의 다른 선택지라
-- 둘은 술어가 배타적이어서 동시에 서지 않는다.
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
    ('installing_jenkins', 'jenkins', 'jenkins', 'https://charts.jenkins.io', '5.9.54', NULL, 'B', 11, false, true)
ON CONFLICT (step_name) DO UPDATE SET
    release_name = EXCLUDED.release_name,
    chart_name   = EXCLUDED.chart_name,
    repo_url     = EXCLUDED.repo_url,
    version      = EXCLUDED.version,
    namespace    = EXCLUDED.namespace,
    phase        = EXCLUDED.phase,
    sort_order   = EXCLUDED.sort_order,
    wait         = EXCLUDED.wait,
    is_enabled   = EXCLUDED.is_enabled,
    updated_at   = NOW();
