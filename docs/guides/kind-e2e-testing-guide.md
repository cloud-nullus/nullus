# Nullus Platform — kind 클러스터 E2E 테스트 가이드

## 사전 요구사항

- Docker Desktop 실행 중
- kind, kubectl, helm 설치
- Nullus 로컬 인프라 기동 (`./scripts/runbook_local.sh up`)
- API 서버 기동 (ENCRYPTION_KEY 32바이트 필수)

```bash
# 1. 인프라 기동
./scripts/runbook_local.sh up

# 2. kind 클러스터 생성
kind create cluster --config scripts/kind-cluster.yaml

# 3. API 서버 기동
ENCRYPTION_KEY="nullus-dev-key-32bytes-padding!!" \
NULLUS_DATABASE_HOST=localhost \
NULLUS_DATABASE_PORT=5433 \
NULLUS_SERVER_MODE=development \
go run ./cmd/api

# 4. 프론트엔드 개발 서버
cd web && npm run dev
```

## 테스트 계정

| 역할 | 이메일 | 비밀번호 | 홈 페이지 |
|------|--------|----------|-----------|
| Admin | admin@nullus.dev | admin123 | /admin/organization |
| DevOps | devops@nullus.dev | devops123 | /stack/templates |
| Developer | developer@nullus.dev | developer123 | /cicd/developer-deploy |

---

## 시나리오 1: Admin — 조직/클러스터/사용자 관리

### 1.1 로그인 + 조직 설정
1. `http://localhost:5173/login` 접속
2. admin@nullus.dev / admin123 입력 → Sign In
3. `/admin/organization` 페이지로 리다이렉트 확인
4. 조직 정보 폼 필드 확인 (이름, slug, 도메인)

### 1.2 kind 클러스터 등록
```bash
# API로 kind 클러스터 등록
KUBECONFIG_B64=$(kind get kubeconfig --name nullus-test | base64 | tr -d '\n')
curl -s -X POST http://localhost:8080/api/v1/admin/clusters \
  -H 'Content-Type: application/json' \
  -d "{
    \"name\": \"kind-nullus-test\",
    \"type\": \"target\",
    \"endpoint\": \"https://127.0.0.1:$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' --context kind-nullus-test | grep -oP ':\K\d+')\",
    \"org_id\": \"$(curl -s http://localhost:8080/api/v1/admin/organization | python3 -c 'import sys,json; print(json.load(sys.stdin)[\"id\"])')\",
    \"kubeconfig\": \"$KUBECONFIG_B64\"
  }" | python3 -m json.tool
```

### 1.3 클러스터 연결 검증
1. UI: `/admin/clusters` 페이지 → 등록된 클러스터 선택
2. "Verify Connection" 버튼 클릭
3. 예상 결과: `status: connected`, K8s 버전 표시

```bash
# API 검증
CLUSTER_ID="<등록된 클러스터 ID>"
curl -s -X POST "http://localhost:8080/api/v1/admin/clusters/${CLUSTER_ID}/verify"
# 예상: {"status":"connected","version":"v1.35.0"}
```

### 1.4 사용자 관리
1. `/admin/users` 페이지 접근
2. 사용자 목록 확인 (테이블 렌더링)
3. 역할 변경 기능 확인

### 1.5 Known Issues 확인
1. 사이드바에서 "Known Issues" 메뉴 클릭
2. `/admin/known-issues` 페이지에서 3개 이슈 표시 확인

### 1.6 감사 로그 확인
```bash
# 클러스터 등록 후 감사 로그에 기록되었는지 확인
curl -s http://localhost:8080/api/v1/admin/audit-logs | python3 -m json.tool | head -20
```

---

## 시나리오 2: DevOps — 스택 설치 + 모니터링

### 2.1 로그인 + 스택 템플릿 확인
1. devops@nullus.dev / devops123 로그인
2. `/stack/templates` 페이지로 리다이렉트
3. 3개 Golden Path 템플릿 카드 확인:
   - GitLab All-in-One
   - GitHub + ArgoCD
   - Minimal CI/CD

### 2.2 스택 설치 위자드
1. "Use Template" 버튼 클릭 → `/stack/install` 페이지
2. 5개 탭 확인: Artifacts → Pipeline → Monitoring → Logging → Resources
3. 각 탭에서 도구 선택
4. YAML View 탭에서 설정 확인
5. "Deploy" 버튼 클릭

### 2.3 스택 설치 (API 테스트)
```bash
# 스택 생성
STACK_RESP=$(curl -s -X POST http://localhost:8080/api/v1/stacks \
  -H 'Content-Type: application/json' \
  -d "{
    \"name\": \"test-devops-stack\",
    \"template_id\": \"$(curl -s http://localhost:8080/api/v1/stacks/templates | python3 -c 'import sys,json; print(json.load(sys.stdin)[\"items\"][0][\"id\"])')\",
    \"cluster_id\": \"<CLUSTER_ID>\",
    \"config\": {\"tools\": [\"gitlab\", \"argocd\", \"prometheus\"]}
  }")
echo "$STACK_RESP" | python3 -m json.tool

# 스택 배포 트리거
STACK_ID=$(echo "$STACK_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
curl -s -X POST "http://localhost:8080/api/v1/stacks/${STACK_ID}/deploy"
# Helm executor가 연결된 경우: 실제 Helm 차트 설치 시작
# 미연결 시: 시뮬레이션 (상태 전환만 진행)
```

### 2.4 스택 목록 + 이력 확인
1. `/stack/list` 페이지에서 생성된 스택 확인
2. 스택 클릭 → 이력 페이지에서 배포 로그 확인

### 2.5 호환성 매트릭스
1. `/stack/version` 페이지 접근
2. 도구 간 호환성 매트릭스 확인
3. "Validate Current Stack" 버튼 클릭

### 2.6 모니터링 대시보드
1. `/observability/monitoring` 페이지 접근
2. KPI 카드 (CPU, Memory, Pod, Pipeline) 표시 확인
3. Recharts 차트 렌더링 확인

### 2.7 알림 규칙 관리
1. `/observability/alert-rules` 페이지 접근
2. 알림 규칙 CRUD 확인 (생성, 수정, 삭제)

---

## 시나리오 3: Developer — CI/CD 파이프라인 + 셀프서비스 배포

### 3.1 로그인 + 접근 제한 확인
1. developer@nullus.dev / developer123 로그인
2. 리다이렉트: `/cicd/developer-deploy`
3. 사이드바 확인:
   - ❌ DevSecOps Stack 메뉴 없음
   - ❌ Admin 메뉴 없음
   - ✅ CI/CD 메뉴 있음
   - ✅ Observability 메뉴 있음 (읽기 전용)

### 3.2 CI/CD 파이프라인 목록
1. `/cicd/list` 페이지에서 파이프라인 목록 확인

### 3.3 셀프서비스 배포
1. `/cicd/developer-deploy` 페이지
2. 앱 템플릿 선택 (e.g., Spring Boot)
3. 클러스터/네임스페이스 선택
4. 배포 실행

### 3.4 모니터링 (읽기 전용)
1. `/observability/monitoring` 페이지 접근 가능 확인
2. Alert Rules 메뉴가 사이드바에서 숨겨져 있는지 확인

---

## 시나리오 4: 크로스 역할 접근 제어

### 4.1 Developer가 Admin 페이지 접근 시도
```bash
# Developer는 admin API에 접근 불가 (프로덕션 모드에서)
# Dev 모드에서는 인증 미들웨어 비활성화 상태
curl -s http://localhost:8080/api/v1/admin/organization
# Dev mode: 200 (인증 우회)
# Prod mode: 403 Forbidden
```

### 4.2 Developer가 Stack 페이지 접근 시도
- UI에서 직접 `/stack/templates` URL 입력
- 사이드바에서 메뉴 미표시 확인

---

## API 엔드포인트 전체 Smoke Test

```bash
BASE="http://localhost:8080"
ENDPOINTS=(
  "GET /health"
  "GET /api/v1/stacks"
  "GET /api/v1/stacks/templates"
  "GET /api/v1/stacks/compatibility"
  "GET /api/v1/admin/organization"
  "GET /api/v1/admin/clusters"
  "GET /api/v1/admin/known-issues"
  "GET /api/v1/admin/audit-logs"
  "GET /api/v1/cicd/templates"
  "GET /api/v1/cicd/pipelines"
  "GET /api/v1/observability/dashboard"
  "GET /api/v1/observability/alert-rules"
  "GET /api/v1/admin/notifications/configs"
)

for ep in "${ENDPOINTS[@]}"; do
  PATH=$(echo "$ep" | cut -d' ' -f2)
  CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE}${PATH}")
  [ "$CODE" -ge 200 ] && [ "$CODE" -lt 300 ] && echo "✅ $CODE $ep" || echo "❌ $CODE $ep"
done
```

---

## kind 클러스터 정리

```bash
kind delete cluster --name nullus-test
```
