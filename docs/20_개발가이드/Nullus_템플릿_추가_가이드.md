# Nullus 템플릿 추가 가이드

**작성일**: 2026-03-22
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
└── created_by              string     "admin" (선택)
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
# 현재 최신 번호 + 1을 사용 (예: 000023)
touch db/migrations/000023_seed_template_jenkins.up.sql
touch db/migrations/000023_seed_template_jenkins.down.sql
```

**Step 2**: UP 마이그레이션 작성

```sql
-- db/migrations/000023_seed_template_jenkins.up.sql
INSERT INTO golden_path_templates (id, name, description, tools, estimated_install_time, recommended_use_case, min_resources, created_by)
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
  'admin'
)
ON CONFLICT (id) DO NOTHING;
```

**Step 3**: DOWN 마이그레이션 작성

```sql
-- db/migrations/000023_seed_template_jenkins.down.sql
DELETE FROM golden_path_templates WHERE id = 'jenkins-tekton-v1';
```

**Step 4**: 마이그레이션 적용

```bash
# Docker Compose 환경
docker exec -i draft-postgres-1 psql -U nullus -d nullus < db/migrations/000023_seed_template_jenkins.up.sql

# migrate CLI 사용 시
migrate -path db/migrations -database "postgres://nullus:nullus_dev@localhost:5433/nullus?sslmode=disable" up
```

**Step 5**: 확인

```bash
curl -s http://localhost:8090/api/v1/stacks/templates | jq '.[].name'
# "GitLab All-in-One"
# "GitLab + Argo CD"
# "GitHub + Argo CD"
# "Jenkins + Tekton"  ← 새로 추가됨
```

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
    "min_resources": "10 vCPU / 20Gi RAM / 120Gi Storage"
  }'
```

Admin UI에서도 가능: **Stack Template 페이지 → "New Template" 버튼 → 모달 폼 작성 → Create**

### 1.4 방법 3: 프론트엔드 Admin UI 사용

1. Admin 또는 DevOps 역할로 로그인
2. 데브섹옵스 스택 > 스택 템플릿 이동
3. 우상단 "New Template" 클릭
4. 모달에서 입력:
   - **Template ID**: 영문 소문자 + 하이픈 (예: `jenkins-tekton-v1`)
   - **Name**: 표시 이름
   - **Description**: 설명
   - **Tools**: JSON 배열 (위 구조 참고)
   - **Estimated Install Time**: 나노초 단위 (90분 = `5400000000000`)
   - **Recommended Use Case**: 권장 사용 사례
   - **Minimum Resources**: 최소 리소스
5. "Create Template" 클릭

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
├── id              string      "ml-pipeline-v1"
├── name            string      "ML Training Pipeline"
├── description     string      "머신러닝 모델 학습/배포 파이프라인"
├── app_type        string      "batch" (web | backend | batch)
└── stages          []string    ["DataPrep", "Train", "Evaluate", "Deploy"]
```

**app_type 값과 UI 색상**:

| app_type | UI 라벨 | 배지 색상 |
|----------|---------|----------|
| `web` | Web Frontend | 초록색 |
| `backend` | Web Backend | 파란색 |
| `batch` | Batch Job | 주황색 |

**일반적인 stages 조합**:

| 앱 타입 | 단계 예시 |
|---------|----------|
| Backend | `["Build", "Test", "ImageBuild", "Deploy"]` |
| Frontend | `["Build", "Test", "StaticBuild", "Deploy"]` |
| Batch | `["Build", "ImageBuild", "CronJobDeploy"]` |
| ML | `["DataPrep", "Train", "Evaluate", "Deploy"]` |
| Mobile | `["Build", "Test", "Sign", "Distribute"]` |

### 2.2 방법 1: DB 마이그레이션으로 추가 (권장)

**Step 1**: 마이그레이션 파일 생성

```bash
# 현재 최신 번호 + 1을 사용 (예: 000024)
touch db/migrations/000024_seed_cicd_template_ml.up.sql
touch db/migrations/000024_seed_cicd_template_ml.down.sql
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
curl -s http://localhost:8090/api/v1/cicd/templates | jq '.[].name'
# "Web Backend Pipeline"
# "Web Frontend Pipeline"
# "Batch Job Pipeline"
# "ML Training Pipeline"  ← 새로 추가됨
```

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
    "created_by": "admin"
  }'
```

### 2.4 방법 3: 프론트엔드 Admin UI 사용

1. Admin 또는 DevOps 역할로 로그인
2. CI/CD > CI/CD 템플릿 이동
3. 우상단 "New Template" 클릭
4. 모달에서 입력:
   - **Template ID**: 영문 소문자 + 하이픈 (예: `ml-pipeline-v1`)
   - **Name**: 표시 이름
   - **Description**: 설명
   - **Stages**: 체크박스에서 선택하거나 직접 입력
5. "Create Template" 클릭

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
- [ ] 각 stage 이름이 파이프라인 실행 엔진에서 인식 가능한 이름인지 확인
- [ ] 프론트엔드에서 카드가 정상 렌더링되는지 확인

---

## 5. 참고 자료

| 자료 | 경로 |
|------|------|
| Stack Template 도메인 | `internal/stack/domain/template.go` |
| Stack Template 시드 | `db/migrations/000007_seed_templates.up.sql` |
| Stack Template API | `internal/stack/adapter/handler/template_handler.go` |
| Stack Template 페이지 | `web/src/features/stack/pages/stack-template-page.tsx` |
| CI/CD Template 도메인 | `internal/cicd/domain/pipeline.go` |
| CI/CD Template 시드 | `db/migrations/000009_seed_cicd_templates.up.sql` |
| CI/CD Template API | `internal/cicd/adapter/handler/cicd_template_handler.go` |
| CI/CD Template 페이지 | `web/src/features/cicd/pages/cicd-template-page.tsx` |
