-- Gitea 설치 스텝의 차트 설정.
--
-- Go 기본값(helm_step_metadata.go 의 defaultChartSpecForStep)만 넣으면 이 테이블이
-- 있는 환경에서는 조회가 실패해 폴백에 의존하게 된다. 두 경로가 같은 값을 보도록
-- 명시적으로 시드한다.
--
-- sort_order 는 installing_gitlab(8) 바로 뒤다 — 소스 저장소 슬롯의 다른 선택지라
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
    ('installing_gitea', 'gitea', 'gitea', 'https://dl.gitea.com/charts', '12.7.0', NULL, 'B', 9, false, true)
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
