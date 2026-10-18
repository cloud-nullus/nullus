-- CI/CD 골든패스 — 화면: /cicd/golden-paths
--
-- 이 목록은 internal/cicd/adapter/repository/memory_cicd_template.go 의 프로세스 메모리에
-- 박혀 있었다. 생성·수정·삭제 API 가 이미 열려 있어서 화면은 성공 응답을 받았지만, API 가
-- 재시작되면 아래 세 건으로 되돌아갔다. 맵을 돌며 목록을 만들어 카드 순서도 요청마다 바뀌었다.
--
-- stack 모듈의 golden_path_templates 와는 다른 표다. 그쪽은 스택 설치 조합(11건)이고
-- 이쪽은 CI/CD 구성이다. 모듈이 자기 표를 소유한다는 규칙(CLAUDE.md)에 따라 따로 둔다.
--
-- sort_order 는 카드가 놓이는 순서다. id 로 정렬하면 github 가 맨 앞에 와서
-- 종전 화면 순서(GitLab All-in-One → GitLab + Argo CD → GitHub + Argo CD)가 뒤집힌다.
CREATE TABLE IF NOT EXISTS cicd_golden_paths (
    id                     VARCHAR(100) PRIMARY KEY,
    name                   VARCHAR(255) NOT NULL,
    description            TEXT         NOT NULL DEFAULT '',
    tools                  JSONB        NOT NULL DEFAULT '[]'::jsonb,
    estimated_install_time INTEGER      NOT NULL DEFAULT 0,
    recommended_use_case   VARCHAR(255) NOT NULL DEFAULT '',
    min_resources          VARCHAR(255) NOT NULL DEFAULT '',
    sort_order             INTEGER      NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cicd_golden_paths_order ON cicd_golden_paths (sort_order, id);

-- 종전 하드코딩 세 건을 그대로 옮긴다. 도구 버전은 손대지 않는다 — 스택 템플릿
-- (db/migrations/000059 이후)과 같은 값이라, 한쪽만 올리면 설치 화면과 어긋난다.
INSERT INTO cicd_golden_paths
    (id, name, description, estimated_install_time, recommended_use_case, min_resources, sort_order, tools)
VALUES
    (
        'gitlab-allinone-v1',
        'GitLab All-in-One',
        'GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.',
        90,
        '중견기업, 단일 플랫폼 선호',
        '8 vCPU / 16Gi RAM / 100Gi Storage',
        1,
        '[
            {"category": "source_repository",        "name": "GitLab CE",       "helm_version": "8.7.2",  "app_version": "17.7.2"},
            {"category": "ci_platform",              "name": "GitLab CI",       "helm_version": "8.7.2",  "app_version": "17.7.2"},
            {"category": "container_registry",       "name": "GitLab Registry", "helm_version": "8.7.2",  "app_version": "17.7.2"},
            {"category": "storage_backend",          "name": "MinIO",           "helm_version": "5.3.0",  "app_version": "2024.11.7"},
            {"category": "cd_tool",                  "name": "Argo CD",         "helm_version": "7.7.2",  "app_version": "2.13.2"},
            {"category": "monitoring_collection",    "name": "Prometheus",      "helm_version": "67.0.0", "app_version": "3.1.0"},
            {"category": "monitoring_visualization", "name": "Grafana",         "helm_version": "8.5.0",  "app_version": "11.4.0"},
            {"category": "log_aggregation",          "name": "Loki",            "helm_version": "6.6.2",  "app_version": "3.0.0"}
        ]'::jsonb
    ),
    (
        'gitlab-argocd-v1',
        'GitLab + Argo CD',
        'GitLab CI와 Harbor 레지스트리를 분리하여 GitOps 패턴을 강화한 구성입니다.',
        120,
        'GitOps 중심 조직',
        '10 vCPU / 20Gi RAM / 130Gi Storage',
        2,
        '[
            {"category": "source_repository",        "name": "GitLab CE",  "helm_version": "8.7.2",  "app_version": "17.7.2"},
            {"category": "ci_platform",              "name": "GitLab CI",  "helm_version": "8.7.2",  "app_version": "17.7.2"},
            {"category": "container_registry",       "name": "Harbor",     "helm_version": "1.14.0", "app_version": "2.11.0"},
            {"category": "storage_backend",          "name": "MinIO",      "helm_version": "5.3.0",  "app_version": "2024.11.7"},
            {"category": "cd_tool",                  "name": "Argo CD",    "helm_version": "7.7.2",  "app_version": "2.13.2"},
            {"category": "monitoring_collection",    "name": "Prometheus", "helm_version": "67.0.0", "app_version": "3.1.0"},
            {"category": "monitoring_visualization", "name": "Grafana",    "helm_version": "8.5.0",  "app_version": "11.4.0"},
            {"category": "log_aggregation",          "name": "Loki",       "helm_version": "6.6.2",  "app_version": "3.0.0"}
        ]'::jsonb
    ),
    (
        'github-argocd-v1',
        'GitHub + Argo CD',
        -- 이미지 레지스트리도 GitHub 쪽(GHCR)이다. GitHub 호스티드 러너는 클러스터 내부
        -- Harbor 에 닿을 수 없어 push 가 불가능하다.
        'GitHub·GitHub Actions·GHCR 을 외부 서비스로 사용하고, 클러스터 내에는 Argo CD + 모니터링만 설치합니다.',
        45,
        'GitHub 사용 조직',
        '4 vCPU / 8Gi RAM / 50Gi Storage',
        3,
        '[
            {"category": "source_repository",        "name": "GitHub",         "helm_version": "external", "app_version": "external"},
            {"category": "ci_platform",              "name": "GitHub Actions", "helm_version": "external", "app_version": "external"},
            {"category": "container_registry",       "name": "GHCR",           "helm_version": "external", "app_version": "external"},
            {"category": "storage_backend",          "name": "MinIO",          "helm_version": "5.3.0",    "app_version": "2024.11.7"},
            {"category": "cd_tool",                  "name": "Argo CD",        "helm_version": "7.7.2",    "app_version": "2.13.2"},
            {"category": "monitoring_collection",    "name": "Prometheus",     "helm_version": "67.0.0",   "app_version": "3.1.0"},
            {"category": "monitoring_visualization", "name": "Grafana",        "helm_version": "8.5.0",    "app_version": "11.4.0"},
            {"category": "log_aggregation",          "name": "Loki",           "helm_version": "6.6.2",    "app_version": "3.0.0"}
        ]'::jsonb
    )
ON CONFLICT (id) DO NOTHING;
