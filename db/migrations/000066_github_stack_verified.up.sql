-- 000066_github_stack_verified.up.sql
--
-- github-argocd-v1 을 verified 로 올린다.
--
-- 이 조합만 untested 로 남아 있었다. 실패 지점이 나머지 매트릭스와 달라서다 —
-- 소스·CI·레지스트리가 전부 클러스터 밖이고, 파이프라인 프로비저닝이 GitLab 이
-- 아니라 GitHub 어댑터(internal/cicd/adapter/github/)를 탄다. 그 경로를 확인한
-- 뒤 verified 로 올린다.
--
-- 설치 전 검사(Pre-Deploy Gate)에서 두 가지가 달라진다:
--
--   1. MATRIX_UNTESTED 경고가 사라지고 판정이 warn(70) → pass(100) 가 된다.
--      배포에 명시적 ack 가 더는 필요하지 않다.
--   2. 아키텍처 불일치의 처리가 뒤집힌다. untested 는 pass→warn 으로 낮추고
--      진행을 허용했지만, verified 는 fail 로 막는다. 이 조합의 도구는 전부
--      amd64+arm64 라 실제로 걸리는 경우는 그 둘이 아닌 아키텍처의 노드가
--      섞여 있을 때뿐이고, 그때는 막는 쪽이 맞다.
--
-- Tier 도 함께 올린다. 000041 이 세운 규칙(verified 매트릭스는 stable)을
-- 유지하기 위해서다 — MinIO·Argo CD·Prometheus·Grafana 는 다른 다섯 매트릭스에서
-- 이미 stable 이고, 같은 차트를 같은 버전으로 설치하면서 이 매트릭스에서만
-- beta 로 남을 이유가 없다.
--
-- tools 블롭을 통째로 덮지 않고 Tier 필드만 바꾼다 — 000060 의 GHCR 전환과
-- 000062 의 버전 정렬을 되돌리지 않기 위해서다.

BEGIN;

UPDATE compatibility_matrices m
SET
    status = 'verified',
    tools = (
        SELECT jsonb_object_agg(e.key, e.value || jsonb_build_object('Tier', 'stable'))
        FROM jsonb_each(m.tools) AS e
    ),
    updated_at = NOW()
WHERE id = 'github-argocd-v1';

COMMIT;
