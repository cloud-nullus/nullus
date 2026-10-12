-- 000084_seed_gitlab_harbor_trivy_template.up.sql
-- 이미지 스캐너(Trivy)를 기본으로 고른 Golden Path 템플릿을 시드한다.
--
-- 스캐너는 매트릭스에서 고를 수 있었지만 기본으로 고른 템플릿이 없어, 화면에서
-- 템플릿을 직접 만들어야 했다. 그렇게 만든 템플릿은 DB 를 초기화하면 사라져 스캔
-- 단계가 들어간 스택을 새 설치에서 다시 세울 수 없었다.
--
-- 버전은 설치 경로가 쓰는 값이다 (internal/stack/domain/connection.go). 000059 의
-- gitlab-harbor-v1 은 GitLab 9.5.1 · Argo CD 6.8.0 을 말하는데 실제 설치는 8.7.2 ·
-- 7.7.16 이다 — 새 템플릿은 그 어긋남을 물려받지 않는다.
-- (고정: TestSeedMigration_GitLabHarborTrivy_MatchesMemory, TestChartVersionsMatchCompatibilityMatrix)
--
-- idempotent 하게 작성해 재실행해도 같은 상태로 수렴한다.

-- ------------------------------------------------------------
-- 1. golden_path_templates
-- ------------------------------------------------------------

INSERT INTO golden_path_templates (
    id, name, description, tools, estimated_install_time, recommended_use_case, min_resources, planning_profile
) VALUES
(
    'gitlab-harbor-trivy-v1',
    'GitLab + Harbor + Trivy',
    '소스코드와 CI는 GitLab, 컨테이너 이미지는 Harbor, 이미지 취약점 스캔은 Trivy 서버가 맡습니다. 파이프라인에 이미지 스캔 단계가 들어가 배포 전에 취약점을 거릅니다.',
    $$[
      {"category":"source_repository","name":"GitLab CE","helm_version":"8.7.2","app_version":"v17.7.0"},
      {"category":"ci_platform","name":"GitLab CI","helm_version":"8.7.2","app_version":"v17.7.0"},
      {"category":"container_registry","name":"Harbor","helm_version":"1.15.0","app_version":"2.11.0"},
      {"category":"storage_backend","name":"MinIO","helm_version":"5.4.0","app_version":"RELEASE.2024-12-18T13-15-44Z"},
      {"category":"cd_tool","name":"Argo CD","helm_version":"7.7.16","app_version":"v2.13.3"},
      {"category":"image_scanner","name":"Trivy","helm_version":"0.26.0","app_version":"0.74.0"}
    ]$$::jsonb,
    110,
    '이미지 취약점 스캔을 배포 게이트로 쓰려는 조직',
    '10 vCPU / 20Gi RAM / 140Gi Storage',
    'standard'
)
ON CONFLICT (id) DO UPDATE SET
    name                   = EXCLUDED.name,
    description            = EXCLUDED.description,
    tools                  = EXCLUDED.tools,
    estimated_install_time = EXCLUDED.estimated_install_time,
    recommended_use_case   = EXCLUDED.recommended_use_case,
    min_resources          = EXCLUDED.min_resources,
    planning_profile       = EXCLUDED.planning_profile,
    updated_at             = NOW();

-- ------------------------------------------------------------
-- 2. compatibility_matrices
--
-- 템플릿만 넣고 매트릭스를 빠뜨리면 Pre-Deploy Gate 가 판정할 근거가 없다.
-- 아키텍처 선언은 gitlab-harbor-v1 과 같다 — 공식 Harbor 이미지는 amd64 뿐이라
-- arm64 클러스터에서는 멀티아키 이미지로 덮어써야 선다.
-- ------------------------------------------------------------

INSERT INTO compatibility_matrices (
    id, name, status, k8s_min, k8s_max, k8s_recommended, tools
) VALUES
(
    'gitlab-harbor-trivy-v1',
    'GitLab + Harbor + Trivy',
    'verified',
    '1.27', '1.35', '1.35',
    $$
    {
      "source_repository":  {"Name": "GitLab CE", "HelmVersion": "8.7.2",  "AppVersion": "v17.7.0",
                             "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "stable"},
      "ci_platform":        {"Name": "GitLab CI", "HelmVersion": "8.7.2",  "AppVersion": "v17.7.0",
                             "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "stable"},
      "container_registry": {"Name": "Harbor",    "HelmVersion": "1.15.0", "AppVersion": "2.11.0",
                             "MinK8sVersion": "1.27", "ArchSupport": ["amd64"], "Tier": "beta"},
      "storage_backend":    {"Name": "MinIO",     "HelmVersion": "5.4.0",  "AppVersion": "RELEASE.2024-12-18T13-15-44Z",
                             "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "cd_tool":            {"Name": "Argo CD",   "HelmVersion": "7.7.16", "AppVersion": "v2.13.3",
                             "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "image_scanner":      {"Name": "Trivy",     "HelmVersion": "0.26.0", "AppVersion": "0.74.0",
                             "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "beta"}
    }
    $$::jsonb
)
ON CONFLICT (id) DO UPDATE SET
    name            = EXCLUDED.name,
    status          = EXCLUDED.status,
    k8s_min         = EXCLUDED.k8s_min,
    k8s_max         = EXCLUDED.k8s_max,
    k8s_recommended = EXCLUDED.k8s_recommended,
    tools           = EXCLUDED.tools,
    updated_at      = NOW();
