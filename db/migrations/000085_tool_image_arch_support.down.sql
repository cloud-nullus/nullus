-- 시드가 선언했던 값으로 되돌린다. 관리 화면에서 만든 매트릭스의 원래 값은 알 수 없어
-- 시드 매트릭스만 되돌린다. gitea-jenkins-argocd-* 의 Harbor 는 시드부터 ["amd64","arm64"] 였다.
UPDATE compatibility_matrices
SET
    tools = jsonb_set(tools, '{container_registry,ArchSupport}', '["amd64"]'::jsonb, false),
    updated_at = NOW()
WHERE id IN ('gitlab-harbor-v1', 'gitlab-harbor-trivy-v1')
  AND tools->'container_registry'->>'Name' = 'Harbor';

UPDATE compatibility_matrices
SET
    tools = jsonb_set(
        jsonb_set(tools, '{container_registry,ArchSupport}', '["amd64","arm64"]'::jsonb, false),
        '{package_registry,ArchSupport}', '["amd64","arm64"]'::jsonb, false),
    updated_at = NOW()
WHERE id = 'gitlab-nexus-v1'
  AND tools->'container_registry'->>'Name' = 'Nexus'
  AND tools->'package_registry'->>'Name' = 'Nexus';
