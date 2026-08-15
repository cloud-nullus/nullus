-- 000072_template_planning_profile.up.sql
-- 템플릿에 설치 규모 프로파일을 얹고, 8Gi 노드용 경량 템플릿을 시드한다.
--
-- 도구 선택만으로는 스택이 몇 Gi 를 먹을지 정해지지 않는다. 같은
-- Gitea + Jenkins + Argo CD 라도 설치 마법사의 계획 프로파일이 standard 면
-- 8Gi 노드에 들어가지 않는다. "무엇을 깔지"와 "얼마나 크게 깔지"가 함께
-- 정해져야 템플릿이 자원 약속을 지킬 수 있다.
--
-- 값은 domain.NormalizePlanningProfile 이 아는 넷뿐이다:
-- local / startup / standard / enterprise.
--
-- idempotent 하게 작성해 재실행해도 같은 상태로 수렴한다.

-- ------------------------------------------------------------
-- 1. golden_path_templates.planning_profile
--
-- 기존 행은 지금까지의 동작(마법사 기본값)과 같은 standard 로 채운다.
-- ------------------------------------------------------------

ALTER TABLE golden_path_templates
    ADD COLUMN IF NOT EXISTS planning_profile VARCHAR(20) NOT NULL DEFAULT 'standard';

ALTER TABLE golden_path_templates
    DROP CONSTRAINT IF EXISTS golden_path_templates_planning_profile_check;

ALTER TABLE golden_path_templates
    ADD CONSTRAINT golden_path_templates_planning_profile_check
    CHECK (planning_profile IN ('local', 'startup', 'standard', 'enterprise'));

-- ------------------------------------------------------------
-- 2. 8Gi 노드용 경량 템플릿
--
-- 예산의 대부분은 템플릿이 고르지 않는 것들이 먼저 가져간다 — PostgreSQL 2Gi,
-- 게이트웨이 0.8Gi, OpenBao·ESO 0.6Gi, cert-manager 1.5Gi. 남는 3Gi 남짓에
-- 들어가는 조합만 담는다. GitLab(4.5Gi)·Prometheus(계산된 벡터가 5개 컴포넌트에
-- 그대로 실려 5Gi)·Nexus(JVM 고정 1.5Gi)는 들어가지 않는다.
--
-- 레지스트리는 뺄 수 없다: 없으면 파이프라인 생성이 registry.ResolverFor 에서
-- 막혀 스택은 서는데 아무것도 배포할 수 없다. Harbor 는 core·registry 만
-- 요청을 잡아 512Mi 로 선다.
--
-- 버전은 설치 경로가 쓰는 값과 같아야 한다 (internal/stack/domain/connection.go).
-- (고정: TestChartVersionsMatchCompatibilityMatrix)
-- ------------------------------------------------------------

INSERT INTO golden_path_templates (
    id, name, description, tools, estimated_install_time, recommended_use_case, min_resources, planning_profile
) VALUES
(
    'gitea-jenkins-argocd-lite-v1',
    'Gitea + Jenkins + Argo CD (Lite)',
    '8Gi 노드 하나에 올라가는 최소 구성입니다. Gitea·Jenkins·Harbor·Argo CD 만 세우고 오브젝트 스토리지와 모니터링은 뺐습니다. 로컬 검증이나 소규모 PoC 에 맞습니다.',
    $$[
      {"category":"source_repository","name":"Gitea","helm_version":"12.7.0","app_version":"1.27.0"},
      {"category":"ci_platform","name":"Jenkins","helm_version":"5.9.54","app_version":"2.568.2"},
      {"category":"container_registry","name":"Harbor","helm_version":"1.15.0","app_version":"2.11.0"},
      {"category":"cd_tool","name":"Argo CD","helm_version":"7.7.16","app_version":"v2.13.3"}
    ]$$::jsonb,
    40,
    '단일 노드 로컬 검증, 소규모 PoC',
    '4 vCPU / 8Gi RAM / 60Gi Storage',
    'local'
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
-- 3. compatibility_matrices
--
-- 템플릿만 넣고 매트릭스를 빠뜨리면 Pre-Deploy Gate 가 판정할 근거가 없어
-- 설치 직전에 막힌다. 도구 셋은 gitea-jenkins-argocd-v1 에서 레지스트리·
-- 오브젝트 스토리지·모니터링을 뺀 것이라 버전 판정 기준은 같다.
-- ------------------------------------------------------------

INSERT INTO compatibility_matrices (
    id, name, status, k8s_min, k8s_max, k8s_recommended, tools
) VALUES
(
    'gitea-jenkins-argocd-lite-v1',
    'Gitea + Jenkins + Argo CD (Lite)',
    'verified',
    '1.26', '1.35', '1.35',
    $$
    {
      "source_repository": {"Name": "Gitea",   "HelmVersion": "12.7.0", "AppVersion": "1.27.0",
                            "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "ci_platform":       {"Name": "Jenkins", "HelmVersion": "5.9.54", "AppVersion": "2.568.2",
                            "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "container_registry": {"Name": "Harbor", "HelmVersion": "1.15.0", "AppVersion": "2.11.0",
                            "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"},
      "cd_tool":           {"Name": "Argo CD", "HelmVersion": "7.7.16", "AppVersion": "v2.13.3",
                            "MinK8sVersion": "1.26", "ArchSupport": ["amd64","arm64"], "Tier": "stable"}
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
