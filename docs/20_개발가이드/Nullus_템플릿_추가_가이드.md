# Nullus 템플릿 추가 가이드

**작성일**: 2026-03-22
**최종 갱신**: 2026-10-12 — API 요청·응답 필드, 템플릿 목록, 매트릭스 화면 안내를 지금 코드와 대조해 맞췄다
**범위**: Stack Template + CI/CD Template 추가 방법

---

## 1. 스택 템플릿 추가

### 1.1 데이터 구조

스택 템플릿은 DevSecOps 도구 조합을 정의하는 프리셋이다.

```
Template
├── id                      string     "jenkins-tekton-v1"
├── name                    string     "Jenkins + Tekton"
├── description             string     "Jenkins CI와 Tekton CD를 조합한 ..."
├── tools                   []ToolConfig
│   ├── category            string     "ci_platform"
│   ├── name                string     "Jenkins"
│   ├── helm_version        string     "5.8.0"
│   └── app_version         string     "2.492.1"
├── estimated_install_time  int64      5400000000000 (나노초, 90분)
├── recommended_use_case    string     "Jenkins 기존 사용 조직"
├── min_resources           string     "8 vCPU / 16Gi RAM / 100Gi Storage"
├── planning_profile        string     "standard" (local | startup | standard | enterprise — 설치 규모, §1.2 주의 참조)
└── created_by              string     "admin" (응답에만 있다 — 요청으로 받지 않는다)
```

**도구 카테고리 목록** (category 필드에 사용):

| 카테고리 | 설명 | 예시 도구 |
|---------|------|----------|
| `source_repository` | 소스코드 저장소 | GitLab CE, Gitea |
| `ci_platform` | CI 플랫폼 | GitLab CI, Jenkins, GitHub Actions |
| `cd_tool` | CD 도구 | Argo CD, Flux |
| `container_registry` | 컨테이너 레지스트리 | Harbor, GitLab Registry |
| `storage` | 오브젝트 스토리지 | MinIO |
| `monitoring` | 모니터링 | Prometheus, Thanos |
| `visualization` | 시각화 | Grafana |
| `logging` | 로깅 | Loki, OpenTelemetry |

### 1.2 방법 1: DB 마이그레이션으로 추가 (권장)

프로덕션 환경에 배포할 템플릿은 마이그레이션 파일로 관리한다.

**Step 1**: 마이그레이션 파일 생성

```bash
# 다음 번호를 먼저 확인한다 — 고정 번호를 쓰면 기존 마이그레이션과 충돌한다
ls db/migrations/*.up.sql | tail -1

# 확인한 번호 + 1 로 만든다 (아래 NNNNNN 자리에 채워 넣는다)
touch db/migrations/NNNNNN_seed_template_jenkins.up.sql
touch db/migrations/NNNNNN_seed_template_jenkins.down.sql
```

**Step 2**: UP 마이그레이션 작성

> **주의 (2026-08-31 정정)** — 이 예제는 오래 `created_by` 컬럼을 함께 넣고 있었는데,
> **그 컬럼은 실재하지 않는다.** 테이블을 만드는 것은 `000006` 이고 거기에는 `created_by`
> 가 없다. `000008` 의 `CREATE TABLE IF NOT EXISTS` 에는 있지만 그때는 테이블이 이미
> 있어 그 정의가 실행되지 않는다. 그대로 따라 하면
> `column "created_by" of relation "golden_path_templates" does not exist` 로 죽는다.
>
> 대신 **`planning_profile` 을 넣어야 한다**(`000072` 에서 추가). 없으면 기본값
> `standard` 로 들어가는데, 8Gi 노드를 겨냥한 템플릿이 그 값을 가지면 설치가 자원
> 약속을 지키지 못한다. 값은 `local` / `startup` / `standard` / `enterprise` 넷뿐이며
> `domain.NormalizePlanningProfile` 이 이 목록을 안다.

```sql
-- db/migrations/NNNNNN_seed_template_jenkins.up.sql
INSERT INTO golden_path_templates (
  id, name, description, tools,
  estimated_install_time, recommended_use_case, min_resources, planning_profile
)
VALUES (
  'jenkins-tekton-v1',
  'Jenkins + Tekton',
  'Jenkins CI와 Tekton CD를 조합한 엔터프라이즈 파이프라인. 기존 Jenkins 사용 조직에 권장합니다.',
  '[
    {"category":"source_repository","name":"Gitea","helm_version":"10.6.0","app_version":"1.23.5"},
    {"category":"ci_platform","name":"Jenkins","helm_version":"5.8.0","app_version":"2.492.1"},
    {"category":"cd_tool","name":"Tekton","helm_version":"0.54.0","app_version":"0.54.0"},
    {"category":"container_registry","name":"Harbor","helm_version":"1.16.2","app_version":"2.12.2"},
    {"category":"storage","name":"MinIO","helm_version":"5.4.0","app_version":"2024.11.7"},
    {"category":"monitoring","name":"Prometheus","helm_version":"27.3.0","app_version":"3.1.0"},
    {"category":"visualization","name":"Grafana","helm_version":"8.8.4","app_version":"11.4.0"}
  ]'::jsonb,
  6480000000000,
  '기존 Jenkins 사용 조직, 엔터프라이즈 CI/CD',
  '10 vCPU / 20Gi RAM / 120Gi Storage',
  'standard'
)
ON CONFLICT (id) DO NOTHING;
```

**Step 2-b (필수)**: 같은 마이그레이션에 **호환성 매트릭스도 함께 시드한다**

템플릿만 넣고 매트릭스를 빠뜨리면 Pre-Deploy Gate 가 판정할 근거가 없어 **설치 직전에
막힌다.** 매트릭스 `id` 는 템플릿 `id` 와 같은 값을 쓴다.

```sql
INSERT INTO compatibility_matrices (id, name, status, k8s_min, k8s_max, k8s_recommended, tools)
VALUES ('jenkins-tekton-v1', 'Jenkins + Tekton', 'verified', '1.26', '1.35', '1.35', $$
  { "ci_platform": {"Name":"Jenkins","HelmVersion":"5.8.0","AppVersion":"2.492.1",
                    "MinK8sVersion":"1.26","ArchSupport":["amd64","arm64"],"Tier":"stable"} }
$$::jsonb)
ON CONFLICT (id) DO UPDATE SET tools = EXCLUDED.tools, updated_at = NOW();
```

차트 버전은 **설치 경로가 쓰는 값**(`internal/stack/domain/connection.go`)과 같아야 한다.
`TestChartVersionsMatchCompatibilityMatrix` 가 이 일치를 고정하므로, 어긋나면 테스트가 깨진다.

> 매트릭스는 화면(`/admin/stack-versions`)이나 API 로도 만들 수 있다 — `POST/PUT/DELETE
> /api/v1/stacks/compatibility/matrices[/:id]` 가 등록되어 있다(`compatibility_handler.go`,
> 2026-10-12 확인). 다만 **코드의 차트 버전과 어긋나면 `TestChartVersionsMatchCompatibilityMatrix`
> 가 깨지므로**, 템플릿을 코드로 들이는 경우(마이그레이션)에는 매트릭스도 같은 마이그레이션에 둔다.

**Step 3**: DOWN 마이그레이션 작성

```sql
-- db/migrations/NNNNNN_seed_template_jenkins.down.sql
DELETE FROM compatibility_matrices   WHERE id = 'jenkins-tekton-v1';
DELETE FROM golden_path_templates    WHERE id = 'jenkins-tekton-v1';
```

**Step 4**: 마이그레이션 적용

```bash
# Docker Compose 환경
docker exec -i draft-postgres-1 psql -U nullus -d nullus < db/migrations/NNNNNN_seed_template_jenkins.up.sql

# migrate CLI 사용 시
migrate -path db/migrations -database "postgres://nullus:nullus_dev@localhost:5433/nullus?sslmode=disable" up
```

**Step 5**: 확인

```bash
curl -s http://localhost:8090/api/v1/stacks/templates | jq -r '.[] | "\(.id)  \(.planning_profile)  \(.name)"'
# empty-template-v1               standard  Empty Template
# gitea-jenkins-argocd-lite-v1    local     Gitea + Jenkins + Argo CD (Lite)
# gitea-jenkins-argocd-v1         standard  Gitea + Jenkins + Argo CD
# github-argocd-v1                standard  GitHub + Argo CD
# gitlab-allinone-v1              standard  GitLab All-in-One
# gitlab-argocd-otel-v1           standard  GitLab + Argo CD + OpenTelemetry
# gitlab-argocd-sonarqube-v1      standard  GitLab + Argo CD + SonarQube
# gitlab-argocd-v1                standard  GitLab + Argo CD
# gitlab-harbor-trivy-v1          standard  GitLab + Harbor + Trivy
# gitlab-harbor-v1                standard  GitLab + Harbor
# gitlab-nexus-v1                 standard  GitLab + Nexus
# jenkins-tekton-v1               standard  Jenkins + Tekton   ← 새로 추가됨
```

응답은 배열이다(`{items: …}` 가 아니다). 2026-10-12 시드 기준 11개.

### 1.3 방법 2: API로 동적 추가

운영 중 Admin UI에서 실시간으로 추가할 때 사용한다.

```bash
curl -X POST http://localhost:8090/api/v1/stacks/templates \
  -H "Content-Type: application/json" \
  -d '{
    "id": "jenkins-tekton-v1",
    "name": "Jenkins + Tekton",
    "description": "Jenkins CI와 Tekton CD를 조합한 엔터프라이즈 파이프라인.",
    "tools": [
      {"category":"source_repository","name":"Gitea","helm_version":"10.6.0","app_version":"1.23.5"},
      {"category":"ci_platform","name":"Jenkins","helm_version":"5.8.0","app_version":"2.492.1"},
      {"category":"cd_tool","name":"Tekton","helm_version":"0.54.0","app_version":"0.54.0"},
      {"category":"container_registry","name":"Harbor","helm_version":"1.16.2","app_version":"2.12.2"},
      {"category":"storage","name":"MinIO","helm_version":"5.4.0","app_version":"2024.11.7"},
      {"category":"monitoring","name":"Prometheus","helm_version":"27.3.0","app_version":"3.1.0"},
      {"category":"visualization","name":"Grafana","helm_version":"8.8.4","app_version":"11.4.0"}
    ],
    "estimated_install_time": 6480000000000,
    "recommended_use_case": "기존 Jenkins 사용 조직",
    "min_resources": "10 vCPU / 20Gi RAM / 120Gi Storage",
    "planning_profile": "standard"
  }'
```

요청 필드는 `id` · `name` · `description` · `tools` · `estimated_install_time` · `recommended_use_case` · `min_resources` · `planning_profile` 이다 (`internal/stack/adapter/handler/template_handler.go` 의 `templateRequest`). `created_by` 는 받지 않는다. 호환성 매트릭스는 이 API 가 만들어 주지 않으므로 §1.2 Step 2-b 처럼 따로 넣어야 설치 직전 게이트를 통과한다.

Admin UI에서도 가능: **스택 템플릿 페이지 → "템플릿 생성" 버튼 → 모달 폼 작성 → 생성**

### 1.4 방법 3: 프론트엔드 Admin UI 사용

1. Admin 또는 DevOps 역할로 로그인
2. 데브섹옵스 스택 > 스택 템플릿 이동
3. 우상단 "템플릿 생성"(Create Template) 클릭
4. 모달에서 입력:
   - **Template ID**: 영문 소문자 + 하이픈 (예: `jenkins-tekton-v1`)
   - **Name**: 표시 이름
   - **Description**: 설명
   - **Tools**: 도구 선택 (위 구조의 category · name · 버전)
   - **Estimated Install Time**: 나노초 단위 (90분 = `5400000000000`)
   - **Recommended Use Case**: 권장 사용 사례
   - **Minimum Resources**: 최소 리소스
   - **설치 규모**(planning_profile): local / startup / standard / enterprise
5. "템플릿 생성" 클릭

### 1.5 estimated_install_time 변환 참고

| 시간 | 나노초 값 |
|------|----------|
| 30분 | `1800000000000` |
| 60분 | `3600000000000` |
| 90분 | `5400000000000` |
| 120분 | `7200000000000` |

프론트엔드에서는 자동으로 "약 90분"으로 변환하여 표시한다.

---

## 2. CI/CD 템플릿 추가

### 2.1 데이터 구조

CI/CD 템플릿은 파이프라인 단계 조합을 정의하는 프리셋이다.

```
PipelineTemplate
├── id               string             "ml-pipeline-v1"
├── name             string             "ML Training Pipeline"
├── description      string             "머신러닝 모델 학습/배포 파이프라인"
├── app_type         string             "batch" (web | backend | batch)
├── stages           []string           ["DataPrep", "Train", "Evaluate", "Deploy"]
├── git_repo_url     string             (선택) 스택 없는 파이프라인의 기본 저장소
├── dockerfile_path  string             (선택) 기본 Dockerfile 경로
├── docker_context   string             (선택) 기본 빌드 컨텍스트
├── env_vars         map[string]string  (선택) 기본 환경 변수
└── created_by       string             (선택)
```

(`internal/cicd/domain/pipeline.go` 의 `PipelineTemplate`.) **`stages` 는 카드 표시와 스택 없는 파이프라인의 단계 기본값이다.** 스택에 붙여 만든 파이프라인은 저장소 프로비저닝이 CI 정의(`.gitlab-ci.yml` · Jenkinsfile · 워크플로)에서 단계를 정하므로(Build · SAST · ImageScan · Deploy 등) 템플릿의 `stages` 가 그대로 실행 단계가 되지 않는다.

**app_type 값과 UI 색상**:

| app_type | UI 라벨 | 배지 색상 |
|----------|---------|----------|
| `web` | Web Frontend | 초록색 |
| `backend` | Web Backend | 파란색 |
| `batch` | Batch Job | 주황색 |

**시드된 템플릿의 stages** (2026-10-12, `GET /api/v1/cicd/templates`):

| ID | 이름 | 앱 타입 | stages |
|----|------|---------|--------|
| `nullus-sample-backend-v1` | Nullus Sample App — Backend | backend | `["GitClone", "DockerBuild", "ImageLoad", "Deploy"]` |
| `nullus-sample-frontend-v1` | Nullus Sample App — Frontend | web | `["GitClone", "DockerBuild", "ImageLoad", "Deploy"]` |
| `web-backend-v1` | User Custom Pipeline | backend | `["Build", "Deploy"]` |
| `batch-job-v1` | Batch Job Pipeline | batch | `["Build", "Deploy"]` |

### 2.2 방법 1: DB 마이그레이션으로 추가 (권장)

**Step 1**: 마이그레이션 파일 생성

```bash
# 위와 같이 다음 번호를 확인한 뒤 만든다
ls db/migrations/*.up.sql | tail -1
touch db/migrations/NNNNNN_seed_cicd_template_ml.up.sql
touch db/migrations/NNNNNN_seed_cicd_template_ml.down.sql
```

**Step 2**: UP 마이그레이션 작성

```sql
-- db/migrations/000024_seed_cicd_template_ml.up.sql
INSERT INTO pipeline_templates (id, name, description, app_type, stages)
VALUES (
  'ml-pipeline-v1',
  'ML Training Pipeline',
  '머신러닝 모델 학습, 평가, 배포를 자동화하는 파이프라인. GPU 워크로드에 최적화되어 있습니다.',
  'batch',
  '["DataPrep", "Train", "Evaluate", "ModelRegistry", "Deploy"]'::jsonb
)
ON CONFLICT (id) DO NOTHING;
```

**Step 3**: DOWN 마이그레이션 작성

```sql
-- db/migrations/000024_seed_cicd_template_ml.down.sql
DELETE FROM pipeline_templates WHERE id = 'ml-pipeline-v1';
```

**Step 4**: 마이그레이션 적용

```bash
docker exec -i draft-postgres-1 psql -U nullus -d nullus < db/migrations/000024_seed_cicd_template_ml.up.sql
```

**Step 5**: 확인

```bash
curl -s http://localhost:8090/api/v1/cicd/templates | jq -r '.[] | "\(.id)  \(.app_type)  \(.name)"'
# batch-job-v1               batch    Batch Job Pipeline
# nullus-sample-backend-v1   backend  Nullus Sample App — Backend
# nullus-sample-frontend-v1  web      Nullus Sample App — Frontend
# web-backend-v1             backend  User Custom Pipeline
# ml-pipeline-v1             batch    ML Training Pipeline   ← 새로 추가됨
```

응답은 배열이다.

### 2.3 방법 2: API로 동적 추가

```bash
curl -X POST http://localhost:8090/api/v1/cicd/templates \
  -H "Content-Type: application/json" \
  -d '{
    "id": "ml-pipeline-v1",
    "name": "ML Training Pipeline",
    "description": "머신러닝 모델 학습, 평가, 배포를 자동화하는 파이프라인.",
    "app_type": "batch",
    "stages": ["DataPrep", "Train", "Evaluate", "ModelRegistry", "Deploy"],
    "git_repo_url": "https://github.com/acme/ml-pipeline",
    "dockerfile_path": "Dockerfile",
    "docker_context": ".",
    "env_vars": {"MODEL_BUCKET": "models"},
    "created_by": "admin"
  }'
```

요청 필드는 `id` · `name` · `description` · `app_type` · `stages` 와 선택인 `git_repo_url` · `dockerfile_path` · `docker_context` · `env_vars` · `created_by` 다 (`internal/cicd/adapter/handler/cicd_template_handler.go` 의 `cicdTemplateRequest`). `id` 는 100자, `name` 은 255자까지다.

### 2.4 방법 3: 프론트엔드 Admin UI 사용

1. Admin 또는 DevOps 역할로 로그인
2. CI/CD > CI/CD 템플릿 이동
3. 우상단 "템플릿 생성"(Create Template) 클릭
4. 모달에서 입력:
   - **Template ID**: 영문 소문자 + 하이픈 (예: `ml-pipeline-v1`)
   - **Name**: 표시 이름
   - **Description**: 설명
   - **App Type**: web / backend / batch
   - **Stages**: 체크박스에서 선택하거나 직접 입력
5. "템플릿 생성" 클릭

---

## 3. 수정 및 삭제

### API

```bash
# 스택 템플릿 수정
curl -X PUT http://localhost:8090/api/v1/stacks/templates/jenkins-tekton-v1 \
  -H "Content-Type: application/json" \
  -d '{ "name": "Jenkins + Tekton (Updated)", ... }'

# 스택 템플릿 삭제
curl -X DELETE http://localhost:8090/api/v1/stacks/templates/jenkins-tekton-v1

# CI/CD 템플릿 수정
curl -X PUT http://localhost:8090/api/v1/cicd/templates/ml-pipeline-v1 \
  -H "Content-Type: application/json" \
  -d '{ "name": "ML Pipeline v2", ... }'

# CI/CD 템플릿 삭제
curl -X DELETE http://localhost:8090/api/v1/cicd/templates/ml-pipeline-v1
```

### Admin UI

각 템플릿 카드의 Edit/Delete 버튼으로 수정/삭제 가능 (Admin, DevOps 역할만).

---

## 4. 체크리스트

### 스택 템플릿 추가 시

- [ ] 모든 도구에 `category`, `name`, `helm_version`, `app_version` 필드 포함
- [ ] `id`가 기존 템플릿과 중복되지 않음
- [ ] `estimated_install_time`이 나노초 단위로 올바르게 설정됨
- [ ] `min_resources`가 도구 수에 맞게 현실적으로 설정됨
- [ ] Helm 차트 버전이 실제 존재하는 버전인지 확인
- [ ] 프론트엔드에서 카드가 정상 렌더링되는지 확인

### CI/CD 템플릿 추가 시

- [ ] `app_type`이 `web`, `backend`, `batch` 중 하나
- [ ] `stages` 배열이 비어있지 않음
- [ ] `id`가 기존 템플릿과 중복되지 않음
- [ ] 스택 없는 파이프라인에 쓸 템플릿이면 `stages` 가 플랫폼 직접 배포 단계(Build · Deploy)와 맞는지 확인 — 스택 통합 파이프라인은 CI 정의가 단계를 정한다
- [ ] 프론트엔드에서 카드가 정상 렌더링되는지 확인

---

## 5. 참고 자료

| 자료 | 경로 |
|------|------|
| Stack Template 도메인 | `internal/stack/domain/template.go` |
| Stack Template 테이블 | `db/migrations/000006_golden_path_templates.up.sql` (컬럼의 실제 정의는 여기다) |
| Stack Template 시드 | `db/migrations/000008_seed_templates.up.sql`, 최근 예시는 `000069`·`000072` |
| 설치 규모 프로파일 | `db/migrations/000072_template_planning_profile.up.sql`, `domain.NormalizePlanningProfile`(`internal/stack/domain/template.go`), 계산은 `internal/stack/domain/planning.go` |
| 호환성 매트릭스 시드 | `db/migrations/000009_seed_compatibility.up.sql`, `000072` |
| Stack Template API | `internal/stack/adapter/handler/template_handler.go` |
| Stack Template 페이지 | `web/src/features/stack/pages/stack-template-page.tsx` |
| CI/CD Template 도메인 | `internal/cicd/domain/pipeline.go` |
| CI/CD Template 시드 | `db/migrations/000010_seed_cicd_templates.up.sql` |
| CI/CD Template API | `internal/cicd/adapter/handler/cicd_template_handler.go` |
| CI/CD Template 페이지 | `web/src/features/cicd/pages/cicd-template-page.tsx` |
