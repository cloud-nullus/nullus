INSERT INTO golden_path_templates (
    id,
    name,
    description,
    tools,
    estimated_install_time,
    recommended_use_case,
    min_resources
) VALUES (
    'empty-template-v1',
    'Empty Template',
    'Start from an empty stack configuration with every tool left unselected.',
    '[]'::jsonb,
    300000000000,
    'Blank starting point for custom stack composition',
    'Decide resources after selecting the tools you need'
)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    tools = EXCLUDED.tools,
    estimated_install_time = EXCLUDED.estimated_install_time,
    recommended_use_case = EXCLUDED.recommended_use_case,
    min_resources = EXCLUDED.min_resources,
    updated_at = NOW();
