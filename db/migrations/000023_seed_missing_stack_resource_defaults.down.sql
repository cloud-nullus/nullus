DELETE FROM stack_resource_defaults
WHERE tool_key IN (
  'gitlab',
  'gitlab-ci',
  'nexus',
  'jfrog',
  'github',
  'gitea',
  'docker-hub',
  's3',
  'gcs',
  'github-actions',
  'jenkins',
  'flux',
  'spinnaker',
  'thanos',
  'victoriametrics',
  'kibana',
  'opensearch-dashboards',
  'tempo',
  'jaeger',
  'elasticsearch',
  'loki'
);
