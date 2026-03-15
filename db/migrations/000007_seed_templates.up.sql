INSERT INTO golden_path_templates (
    id,
    name,
    description,
    tools,
    estimated_install_time,
    recommended_use_case,
    min_resources
) VALUES
(
    'gitlab-allinone-v1',
    'GitLab All-in-One',
    'GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.',
    $$[
      {"category":"source_repository","name":"GitLab CE","helm_version":"8.7.2","app_version":"17.7.2"},
      {"category":"ci_platform","name":"GitLab CI","helm_version":"8.7.2","app_version":"17.7.2"},
      {"category":"container_registry","name":"GitLab Registry","helm_version":"8.7.2","app_version":"17.7.2"},
      {"category":"storage_backend","name":"MinIO","helm_version":"5.3.0","app_version":"2024.11.7"},
      {"category":"cd_tool","name":"Argo CD","helm_version":"7.7.2","app_version":"2.13.2"},
      {"category":"monitoring_collection","name":"Prometheus","helm_version":"67.0.0","app_version":"3.1.0"},
      {"category":"monitoring_visualization","name":"Grafana","helm_version":"8.5.0","app_version":"11.4.0"}
    ]$$::jsonb,
    90,
    '중견기업, 단일 플랫폼 선호',
    '8 vCPU / 16Gi RAM / 100Gi Storage'
),
(
    'gitlab-argocd-v1',
    'GitLab + Argo CD',
    'GitLab CI와 Harbor 레지스트리를 분리하여 GitOps 패턴을 강화한 구성입니다.',
    $$[
      {"category":"source_repository","name":"GitLab CE","helm_version":"8.7.2","app_version":"17.7.2"},
      {"category":"ci_platform","name":"GitLab CI","helm_version":"8.7.2","app_version":"17.7.2"},
      {"category":"container_registry","name":"Harbor","helm_version":"1.14.0","app_version":"2.11.0"},
      {"category":"storage_backend","name":"MinIO","helm_version":"5.3.0","app_version":"2024.11.7"},
      {"category":"cd_tool","name":"Argo CD","helm_version":"7.7.2","app_version":"2.13.2"},
      {"category":"monitoring_collection","name":"Prometheus","helm_version":"67.0.0","app_version":"3.1.0"},
      {"category":"monitoring_visualization","name":"Grafana","helm_version":"8.5.0","app_version":"11.4.0"}
    ]$$::jsonb,
    120,
    'GitOps 중심 조직',
    '10 vCPU / 20Gi RAM / 130Gi Storage'
),
(
    'github-argocd-v1',
    'GitHub + Argo CD',
    'GitHub와 GitHub Actions를 외부 서비스로 사용하고, 클러스터 내에는 Harbor + Argo CD + 모니터링만 설치합니다.',
    $$[
      {"category":"source_repository","name":"GitHub","helm_version":"external","app_version":"external"},
      {"category":"ci_platform","name":"GitHub Actions","helm_version":"external","app_version":"external"},
      {"category":"container_registry","name":"Harbor","helm_version":"1.14.0","app_version":"2.11.0"},
      {"category":"storage_backend","name":"MinIO","helm_version":"5.3.0","app_version":"2024.11.7"},
      {"category":"cd_tool","name":"Argo CD","helm_version":"7.7.2","app_version":"2.13.2"},
      {"category":"monitoring_collection","name":"Prometheus","helm_version":"67.0.0","app_version":"3.1.0"},
      {"category":"monitoring_visualization","name":"Grafana","helm_version":"8.5.0","app_version":"11.4.0"}
    ]$$::jsonb,
    60,
    'GitHub 사용 조직',
    '6 vCPU / 12Gi RAM / 80Gi Storage'
);
