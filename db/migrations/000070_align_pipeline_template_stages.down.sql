UPDATE pipeline_templates SET stages = '["Build", "Test", "ImageBuild", "Deploy"]'::jsonb WHERE id = 'web-backend-v1';
UPDATE pipeline_templates SET stages = '["Build", "ImageBuild", "CronJobDeploy"]'::jsonb WHERE id = 'batch-job-v1';
