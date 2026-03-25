WITH version_map(name_key, helm_version, app_version) AS (
  VALUES
    ('gitlab ce', '8.7.2', '17.7.2'),
    ('gitlab ci', '8.7.2', '17.7.2'),
    ('gitlab registry', '8.7.2', '17.7.2'),
    ('argo cd', '7.7.2', '2.13.2'),
    ('harbor', '1.14.0', '2.11.0'),
    ('minio', '5.3.0', '2024.11.7'),
    ('prometheus', '67.0.0', '3.1.0'),
    ('grafana', '8.5.0', '11.4.0')
),
updated_templates AS (
  SELECT
    gpt.id,
    (
      SELECT jsonb_agg(
        CASE
          WHEN vm.name_key IS NULL THEN tool_item
          ELSE jsonb_set(
            jsonb_set(tool_item, '{helm_version}', to_jsonb(vm.helm_version), true),
            '{app_version}',
            to_jsonb(vm.app_version),
            true
          )
        END
      )
      FROM jsonb_array_elements(gpt.tools) AS tool_item
      LEFT JOIN version_map vm
        ON lower(coalesce(tool_item->>'name', '')) = vm.name_key
    ) AS tools
  FROM golden_path_templates gpt
)
UPDATE golden_path_templates gpt
SET tools = ut.tools,
    updated_at = NOW()
FROM updated_templates ut
WHERE gpt.id = ut.id
  AND ut.tools IS NOT NULL;

WITH version_map(name_key, helm_version, app_version) AS (
  VALUES
    ('gitlab ce', '8.7.2', '17.7.2'),
    ('gitlab ci', '8.7.2', '17.7.2'),
    ('gitlab registry', '8.7.2', '17.7.2'),
    ('argo cd', '7.7.2', '2.13.2'),
    ('harbor', '1.14.0', '2.11.0'),
    ('minio', '5.3.0', '2024.11.7'),
    ('prometheus', '67.0.0', '3.1.0'),
    ('grafana', '8.5.0', '11.4.0')
),
updated_compatibility AS (
  SELECT
    cm.id,
    (
      SELECT jsonb_object_agg(
        entry.key,
        CASE
          WHEN vm.name_key IS NULL THEN entry.value
          ELSE jsonb_set(
            jsonb_set(entry.value, '{HelmVersion}', to_jsonb(vm.helm_version), true),
            '{AppVersion}',
            to_jsonb(vm.app_version),
            true
          )
        END
      )
      FROM jsonb_each(cm.tools) AS entry
      LEFT JOIN version_map vm
        ON lower(coalesce(entry.value->>'Name', entry.value->>'name', '')) = vm.name_key
    ) AS tools
  FROM compatibility_matrices cm
)
UPDATE compatibility_matrices cm
SET tools = uc.tools,
    updated_at = NOW()
FROM updated_compatibility uc
WHERE cm.id = uc.id
  AND uc.tools IS NOT NULL;
