-- 000048_expand_gitlab_arm64_support.down.sql
-- 롤백: GitLab 조합의 ArchSupport를 amd64 전용으로 복원한다.

UPDATE compatibility_matrices
SET
    tools = jsonb_set(
        jsonb_set(
            jsonb_set(
                tools,
                '{source_repository,ArchSupport}',
                '["amd64"]'::jsonb,
                true
            ),
            '{ci_platform,ArchSupport}',
            '["amd64"]'::jsonb,
            true
        ),
        '{container_registry,ArchSupport}',
        '["amd64"]'::jsonb,
        true
    ),
    updated_at = NOW()
WHERE id IN ('gitlab-allinone-v1', 'gitlab-argocd-v1');
