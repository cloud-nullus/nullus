DELETE FROM audit_logs WHERE user_id IN ('admin@nullus.io', 'kim@nullus.io', 'park@nullus.io');
DELETE FROM notification_history WHERE org_id = '11111111-1111-1111-1111-111111111111';
DELETE FROM notification_configs WHERE org_id = '11111111-1111-1111-1111-111111111111';
DELETE FROM alerts WHERE id IN ('al-1', 'al-2', 'al-3', 'al-4', 'al-5', 'al-6');
DELETE FROM alert_rules WHERE id IN ('ar-1', 'ar-2', 'ar-3', 'ar-4', 'ar-5');
DELETE FROM pipeline_deployments WHERE id IN ('d1', 'd2', 'd3', 'd4', 'd5', 'd6', 'd7', 'd8');
DELETE FROM pipelines WHERE id IN ('frontend-web', 'backend-api', 'ml-service', 'batch-runner');
DELETE FROM stack_config_versions WHERE id IN ('h1', 'h2', 'h3', 'h4', 'h5');
DELETE FROM stacks WHERE id IN ('production-stack', 'development-stack', 'staging-environment', 'microservices-platform');
