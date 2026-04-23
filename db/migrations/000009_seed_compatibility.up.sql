INSERT INTO compatibility_matrices (
    id,
    name,
    status,
    k8s_min,
    k8s_max,
    k8s_recommended,
    tools
) VALUES
(
    'gitlab-allinone-v1',
    'GitLab All-in-One',
    'verified',
    '1.27',
    '1.32',
    '1.30',
    $$
    {
      "source_repository": {"Name": "GitLab CE", "HelmVersion": "8.7.2", "AppVersion": "17.7.2"},
      "ci_platform": {"Name": "GitLab CI", "HelmVersion": "8.7.2", "AppVersion": "17.7.2"},
      "container_registry": {"Name": "GitLab Registry", "HelmVersion": "8.7.2", "AppVersion": "17.7.2"},
      "storage_backend": {"Name": "MinIO", "HelmVersion": "5.3.0", "AppVersion": "2024.11.7"},
      "cd_tool": {"Name": "Argo CD", "HelmVersion": "7.7.2", "AppVersion": "2.13.2"},
      "monitoring_collection": {"Name": "Prometheus", "HelmVersion": "67.0.0", "AppVersion": "3.1.0"},
      "monitoring_visualization": {"Name": "Grafana", "HelmVersion": "8.5.0", "AppVersion": "11.4.0"}
    }
    $$::jsonb
),
(
    'gitlab-argocd-v1',
    'GitLab + Argo CD',
    'verified',
    '1.27',
    '1.32',
    '1.30',
    $$
    {
      "source_repository": {"Name": "GitLab CE", "HelmVersion": "8.7.2", "AppVersion": "17.7.2"},
      "ci_platform": {"Name": "GitLab CI", "HelmVersion": "8.7.2", "AppVersion": "17.7.2"},
      "container_registry": {"Name": "Harbor", "HelmVersion": "1.14.0", "AppVersion": "2.11.0"},
      "storage_backend": {"Name": "MinIO", "HelmVersion": "5.3.0", "AppVersion": "2024.11.7"},
      "cd_tool": {"Name": "Argo CD", "HelmVersion": "7.7.2", "AppVersion": "2.13.2"},
      "monitoring_collection": {"Name": "Prometheus", "HelmVersion": "67.0.0", "AppVersion": "3.1.0"},
      "monitoring_visualization": {"Name": "Grafana", "HelmVersion": "8.5.0", "AppVersion": "11.4.0"}
    }
    $$::jsonb
),
(
    'github-argocd-v1',
    'GitHub + Argo CD',
    'untested',
    '1.27',
    '1.32',
    '1.29',
    $$
    {
      "source_repository": {"Name": "GitHub", "HelmVersion": "external", "AppVersion": "external"},
      "ci_platform": {"Name": "GitHub Actions", "HelmVersion": "external", "AppVersion": "external"},
      "container_registry": {"Name": "Harbor", "HelmVersion": "1.14.0", "AppVersion": "2.11.0"},
      "storage_backend": {"Name": "MinIO", "HelmVersion": "5.3.0", "AppVersion": "2024.11.7"},
      "cd_tool": {"Name": "Argo CD", "HelmVersion": "7.7.2", "AppVersion": "2.13.2"},
      "monitoring_collection": {"Name": "Prometheus", "HelmVersion": "67.0.0", "AppVersion": "3.1.0"},
      "monitoring_visualization": {"Name": "Grafana", "HelmVersion": "8.5.0", "AppVersion": "11.4.0"}
    }
    $$::jsonb
);
