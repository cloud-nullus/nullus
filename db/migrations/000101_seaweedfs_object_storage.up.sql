-- SeaweedFS 를 스택 오브젝트 스토리지의 다른 구현체로 더한다 (nullus-plan#88).
--
-- MinIO 가 오픈소스 배포를 닫아(상용 AIStor 전환) 한 구현체에 묶여 있으면 그
-- 구현체가 막히는 날 스택 설치가 통째로 멈춘다. 소비자(GitLab)는 S3 계약만 보므로
-- 구현체는 storage_backend 선택으로 고른다. 이 마이그레이션은 설치 단계의 차트
-- 설정과 자원 기본값 두 가지를 시드한다.

-- 1) 설치 단계의 차트 설정.
--
-- Go 기본값(helm_step_metadata.go 의 defaultChartSpecForStep)과 같은 값이다. 이
-- 테이블이 있는 환경에서는 여기가 먼저 읽히므로 두 경로가 같은 값을 보게 둔다.
-- sort_order 는 installing_minio(3) 와 같다 — 같은 슬롯의 다른 선택지라 둘은 술어가
-- 배타적이어서 동시에 서지 않는다.
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
    ('installing_seaweedfs', 'nullus-seaweedfs', 'seaweedfs', 'https://seaweedfs.github.io/seaweedfs/helm', '4.48.0', NULL, 'A', 3, false, true)
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

-- 2) 자원 기본값.
--
-- all-in-one 은 master·volume·filer·S3 게이트웨이가 한 파드다. 유휴 상태에서 40Mi
-- 남짓을 쓴다(kind cgroup 실측, 2026-10-19). 요청 0.5Gi 는 GitLab 업로드가 몰릴 때의
-- 버퍼 여유, 상한 2Gi 는 MinIO 와 같은 자리를 주기 위한 값이다. 디스크는 Storage
-- 탭의 크기(small 50Gi)가 PVC 를 정한다.
--
-- 관리자가 먼저 넣은 값이 있으면 덮지 않는다.
INSERT INTO stack_resource_defaults (
    tool_key,
    display_name,
    cpu_request,
    cpu_limit,
    memory_request_gi,
    memory_limit_gi,
    storage_request_gi,
    storage_limit_gi,
    is_default,
    updated_at
)
VALUES
    ('seaweedfs', 'SeaweedFS', 0.25, 1.00, 0.50, 2.00, 50.00, 100.00, true, NOW())
ON CONFLICT (tool_key) DO NOTHING;
