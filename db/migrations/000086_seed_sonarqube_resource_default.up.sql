-- SonarQube(SAST 서버)의 자원 기본값.
--
-- SonarQube 는 JVM 셋(웹·Compute Engine·검색)이 한 파드에서 돈다. 각 기본 힙은
-- 512m 이고 검색(Elasticsearch)은 힙 밖에서 mmap 을 더 쓴다. kind 에서 분석 없이
-- 떠 있기만 해도 2.69Gi 를 썼다 — 요청이 그보다 작으면 노드 메모리 압박 때 먼저
-- 축출되므로 요청은 3Gi, 상한 4Gi 는 분석이 몰려 CE 와 색인이 함께 바쁠 때의 여유다.
-- 차트 기본값(요청 4096M · 상한 10240M)은 로컬 kind 에 둘 자리가 없다.
-- 디스크는 검색 색인이 쓴다(DB 데이터는 공유 PostgreSQL 쪽이다).
--
-- 기준 부하는 계획 옵션의 기준값(scansPerDay 40, projectCount 20)이고, 계획 화면이
-- 이 벡터에 배수를 곱한다.
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
    ('sonarqube', 'SonarQube', 0.50, 2.00, 3.00, 4.00, 10.00, 20.00, true, NOW())
ON CONFLICT (tool_key) DO NOTHING;
