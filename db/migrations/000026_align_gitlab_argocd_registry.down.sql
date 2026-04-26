UPDATE golden_path_templates
SET
  description = 'GitLab CI와 Harbor 레지스트리를 분리하여 GitOps 패턴을 강화한 구성입니다.',
  tools = (
    SELECT jsonb_agg(
      CASE
        WHEN lower(coalesce(tool_item->>'category', '')) = 'container_registry' THEN
          jsonb_set(
            jsonb_set(
              jsonb_set(tool_item, '{name}', to_jsonb('Harbor'::text), true),
              '{helm_version}',
              to_jsonb('1.15.0'::text),
              true
            ),
            '{app_version}',
            to_jsonb('2.11.0'::text),
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
      jsonb_set(tools, '{container_registry,Name}', to_jsonb('Harbor'::text), true),
      '{container_registry,HelmVersion}',
      to_jsonb('1.15.0'::text),
      true
    ),
    '{container_registry,AppVersion}',
    to_jsonb('2.11.0'::text),
    true
  ),
  updated_at = NOW()
WHERE id = 'gitlab-argocd-v1';
