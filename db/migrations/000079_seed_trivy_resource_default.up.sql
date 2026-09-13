-- Trivy 서버(이미지 스캐너)의 자원 기본값.
--
-- 실측(kind, trivy chart 0.26.0 / Trivy 0.74.0, cgroup v2 직접 판독, 4 동시 스캔 × 2회):
--   anon 메모리  idle 29Mi → 부하 후 317Mi
--   file cache   1.29~1.31Gi (취약점 DB mmap — 회수 가능, 상한 1Gi 에서도 OOM 없이 8건 통과)
--   CPU          1초 최대 927m, 스로틀 0 (상한 4 core 조건)
--   DB 디스크    1.3G (갱신 중에는 새 DB 를 옆에 받아 잠시 두 배)
--
-- 메모리 상한 2Gi 는 anon + DB 캐시가 들어가는 크기다. 1Gi 로도 돌지만 캐시가
-- 밀려나 매칭마다 디스크를 다시 읽는다. 기준 부하는 계획 옵션의 기준값
-- (scansPerDay 40, concurrentScans 2)이고, 계획 화면이 이 벡터에 배수를 곱한다.
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
    ('trivy', 'Trivy', 0.25, 1.00, 0.50, 2.00, 5.00, 10.00, true, NOW())
ON CONFLICT (tool_key) DO NOTHING;
