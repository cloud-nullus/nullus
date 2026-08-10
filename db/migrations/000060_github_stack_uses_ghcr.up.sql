-- github-argocd-v1 의 컨테이너 레지스트리를 Harbor 에서 GHCR 로 바꾼다.
--
-- 이 스택은 GitHub 호스티드 러너에서 빌드한다. 러너는 GitHub 네트워크에 있어
-- 클러스터 내부 Harbor(harbor.<access_domain>, 보통 .internal)에 닿을 수 없고,
-- 그래서 docker push 가 반드시 실패한다. 이미지는 러너가 닿을 수 있는 곳,
-- 즉 GHCR 에 올려야 한다.
--
-- Harbor 가 빠지면서 최소 자원과 예상 설치 시간도 함께 줄었다.

UPDATE golden_path_templates
SET
    description = 'GitHub·GitHub Actions·GHCR 을 외부 서비스로 사용하고, 클러스터 내에는 Argo CD + 모니터링만 설치합니다.',
    tools = $$[
      {"category":"source_repository",       "name":"GitHub",         "helm_version":"external", "app_version":"external"},
      {"category":"ci_platform",             "name":"GitHub Actions", "helm_version":"external", "app_version":"external"},
      {"category":"container_registry",      "name":"GHCR",           "helm_version":"external", "app_version":"external"},
      {"category":"storage_backend",         "name":"MinIO",          "helm_version":"5.2.0",    "app_version":"RELEASE.2024-08-03T04-33-23Z"},
      {"category":"cd_tool",                 "name":"Argo CD",        "helm_version":"6.8.0",    "app_version":"v2.8.3"},
      {"category":"monitoring_collection",   "name":"Prometheus",     "helm_version":"67.0.0",   "app_version":"v2.54.1"},
      {"category":"monitoring_visualization","name":"Grafana",        "helm_version":"8.5.0",    "app_version":"11.1.0"}
    ]$$::jsonb,
    estimated_install_time = 45,
    min_resources = '4 vCPU / 8Gi RAM / 50Gi Storage',
    updated_at = NOW()
WHERE id = 'github-argocd-v1';

-- 호환성 행렬도 같이 맞춘다. 어긋나면 설치 전 검사에서 존재하지 않는
-- Harbor 버전을 두고 경고가 뜬다.
--
-- GHCR 은 클러스터 밖이라 Harbor 의 amd64 전용 제약을 물려받으면 안 된다 —
-- arm64 클러스터에서 호환성 검사가 잘못 막는다.
UPDATE compatibility_matrices
SET
    tools = jsonb_set(
        tools,
        '{container_registry}',
        $${"Name": "GHCR", "HelmVersion": "external", "AppVersion": "external",
           "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "beta"}$$::jsonb,
        true
    ),
    updated_at = NOW()
WHERE id = 'github-argocd-v1';
