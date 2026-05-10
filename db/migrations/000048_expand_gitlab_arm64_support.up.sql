-- 000048_expand_gitlab_arm64_support.up.sql
-- 확장정책: GitLab 조합의 arm64 지원을 compatibility seed에 반영한다.

UPDATE compatibility_matrices
SET
    tools = jsonb_set(
        jsonb_set(
            jsonb_set(
                tools,
                '{source_repository,ArchSupport}',
                '["amd64","arm64"]'::jsonb,
                true
            ),
            '{ci_platform,ArchSupport}',
            '["amd64","arm64"]'::jsonb,
            true
        ),
        '{container_registry,ArchSupport}',
        '["amd64","arm64"]'::jsonb,
        true
    ),
    updated_at = NOW()
WHERE id IN ('gitlab-allinone-v1', 'gitlab-argocd-v1');
