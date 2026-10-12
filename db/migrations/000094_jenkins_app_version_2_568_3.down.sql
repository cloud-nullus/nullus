UPDATE golden_path_templates
   SET tools = replace(tools::text, '"app_version": "2.568.3"', '"app_version": "2.568.2"')::jsonb
 WHERE tools::text LIKE '%"app_version": "2.568.3"%';

UPDATE compatibility_matrices
   SET tools = replace(tools::text, '"AppVersion": "2.568.3"', '"AppVersion": "2.568.2"')::jsonb
 WHERE tools::text LIKE '%"AppVersion": "2.568.3"%';
