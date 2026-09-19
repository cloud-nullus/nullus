-- 매트릭스의 ArchSupport 를 설치가 실제로 내는 이미지에 맞춘다(cloud-nullus/nullus#270).
--
-- 선언이 매트릭스마다 따로 적혀 있어 실제 이미지와 어긋나도 아무것도 깨지지 않았다.
-- 선언은 이제 도메인(internal/stack/domain/arch_image_source.go)이 소유하고, 여기서
-- 그 도구를 쓰는 모든 매트릭스를 그 값으로 맞춘다. 관리 화면에서 만든 매트릭스도 같은
-- 설치를 쓰므로 함께 맞춘다 — 도구 이름은 대소문자를 가리지 않고(메모리 저장소와 같다),
-- ArchSupport 키가 없는 행에는 만든다.
-- (고정: TestToolImageArchSupportMigration_MatchesImageProfiles)

-- Harbor 는 arm64 노드에서도 선다 — 설치가 노드 아키텍처를 읽어, amd64 뿐인 공식 이미지
-- 대신 멀티아키 재빌드 이미지(ghcr.io/dasomel/goharbor)로 바꾼다.
-- gitlab-harbor-* 는 ["amd64"], gitea-jenkins-argocd-* 는 ["amd64","arm64"] 로 갈려 있었다.
-- 설치가 amd64 이미지만 깔던 때에도 gitea 템플릿은 arm64 클러스터(DGX Spark)에서
-- Pre-Deploy Gate 를 통과하고 installing_harbor 에서 멈췄다.
UPDATE compatibility_matrices
SET
    tools = jsonb_set(tools, '{container_registry,ArchSupport}', $$["amd64","arm64"]$$::jsonb, true),
    updated_at = NOW()
WHERE lower(tools->'container_registry'->>'Name') = 'harbor';

-- Nexus 는 amd64 뿐이다 — 설치하는 sonatype/nexus3 3.64.0 은 단일 아키 매니페스트이고
-- (멀티아키는 3.80 부터) 대체 출처가 없다. 시드(000059)는 ["amd64","arm64"] 였다.
UPDATE compatibility_matrices
SET
    tools = jsonb_set(tools, '{container_registry,ArchSupport}', $$["amd64"]$$::jsonb, true),
    updated_at = NOW()
WHERE lower(tools->'container_registry'->>'Name') = 'nexus';

UPDATE compatibility_matrices
SET
    tools = jsonb_set(tools, '{package_registry,ArchSupport}', $$["amd64"]$$::jsonb, true),
    updated_at = NOW()
WHERE lower(tools->'package_registry'->>'Name') = 'nexus';
