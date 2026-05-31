UPDATE pipeline_templates
SET name = 'User Custom Pipeline'
WHERE id = 'web-backend-v1';

DELETE FROM pipeline_templates
WHERE id = 'web-frontend-v1';
