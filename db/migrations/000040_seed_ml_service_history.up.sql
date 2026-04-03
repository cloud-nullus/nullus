-- Add more deployment history for ML Prediction Service so CI/CD history view has enough entries.
INSERT INTO pipeline_deployments (id, pipeline_id, version, status, started_at, completed_at, deployed_by) VALUES
('ml-d01', 'ml-service', 'v0.8.2',  'success', '2026-03-22T09:00:00Z', '2026-03-22T09:19:00Z', 'park@nullus.io'),
('ml-d02', 'ml-service', 'v0.8.3',  'success', '2026-03-23T09:10:00Z', '2026-03-23T09:27:00Z', 'kim@nullus.io'),
('ml-d03', 'ml-service', 'v0.8.4',  'failed',  '2026-03-24T09:05:00Z', '2026-03-24T09:22:00Z', 'park@nullus.io'),
('ml-d04', 'ml-service', 'v0.8.5',  'success', '2026-03-25T09:12:00Z', '2026-03-25T09:31:00Z', 'kim@nullus.io'),
('ml-d05', 'ml-service', 'v0.8.6',  'success', '2026-03-26T09:08:00Z', '2026-03-26T09:26:00Z', 'admin@nullus.io'),
('ml-d06', 'ml-service', 'v0.8.7',  'failed',  '2026-03-27T09:03:00Z', '2026-03-27T09:20:00Z', 'park@nullus.io'),
('ml-d07', 'ml-service', 'v0.8.8',  'success', '2026-03-28T09:14:00Z', '2026-03-28T09:33:00Z', 'kim@nullus.io'),
('ml-d08', 'ml-service', 'v0.8.9',  'success', '2026-03-29T09:06:00Z', '2026-03-29T09:24:00Z', 'admin@nullus.io'),
('ml-d09', 'ml-service', 'v0.9.0',  'success', '2026-03-30T09:09:00Z', '2026-03-30T09:29:00Z', 'kim@nullus.io'),
('ml-d10', 'ml-service', 'v0.9.1',  'success', '2026-03-31T09:07:00Z', '2026-03-31T09:25:00Z', 'park@nullus.io')
ON CONFLICT (id) DO NOTHING;
