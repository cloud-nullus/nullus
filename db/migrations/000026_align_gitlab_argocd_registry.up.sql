UPDATE golden_path_templates
SET
  description = 'GitLab CI와 GitLab Registry를 사용하고 Argo CD로 GitOps 패턴을 강화한 구성입니다.',
  tools = (
    SELECT jsonb_agg(
      CASE
        WHEN lower(coalesce(tool_item->>'category', '')) = 'container_registry' THEN
          jsonb_set(
            jsonb_set(
              jsonb_set(tool_item, '{name}', to_jsonb('GitLab Registry'::text), true),
              '{helm_version}',
              to_jsonb('9.5.1'::text),
              true
            ),
            '{app_version}',
            to_jsonb('18.5.1'::text),
            true
          )
        ELSE tool_item
      END
    )
    FROM jsonb_array_elements(golden_path_templates.tools) AS tool_item
  ),
  updated_at = NOW()
WHERE id = 'gitlab-argocd-v1';

UPDATE compatibility_matrices
SET
  tools = jsonb_set(
    jsonb_set(
      jsonb_set(tools, '{container_registry,Name}', to_jsonb('GitLab Registry'::text), true),
      '{container_registry,HelmVersion}',
      to_jsonb('9.5.1'::text),
      true
    ),
    '{container_registry,AppVersion}',
    to_jsonb('18.5.1'::text),
    true
  ),
  updated_at = NOW()
WHERE id = 'gitlab-argocd-v1';
