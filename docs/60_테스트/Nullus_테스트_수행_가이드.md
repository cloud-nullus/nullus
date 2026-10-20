# Nullus 테스트 수행 가이드

> 로컬에서 Nullus 를 실제로 띄워 기능·반응형을 검증하는 방법을 모은다.
> 시나리오 기록지: [Nullus_둘러보기_기능확인_시나리오.md](./Nullus_둘러보기_기능확인_시나리오.md)

작성일: 2026-08-17 (E2E 세션 실측 기반)

---

## 1. 테스트 종류 한눈에

| 종류 | 명령 | 앱 기동 필요 | 용도 |
|---|---|:---:|---|
| Go 단위/통합 | `go test ./... -count=1` | ✗ | 도메인·유스케이스 회귀 |
| React 단위 (Vitest) | `cd web && npx vitest run` | ✗ | 컴포넌트·훅 회귀 |
| **Playwright E2E** (제품 내장) | `cd web && npm run e2e` (`e2e:visual`·`e2e:stack-critical`·`e2e:headed`) | ✓ (5173/8090 기준) | 화면 흐름 회귀 |
| **모바일/반응형 점검** | `cd web && npm run responsive:audit` (`:check` = CI 게이트) | ✓ | 가로 오버플로우 + 사이드바 미collapse 자동 감지 (9화면×3뷰포트) |
| API smoke | `./scripts/runbook_local.sh smoke` | ✓ | 엔드포인트 생존 |
| 둘러보기 기반 기능 시나리오 | 시나리오 기록지 참조 (수동+Playwright 스크립트 혼합) | ✓ | 투어 여정 = 제품 핵심 흐름 검증 |

## 2. 로컬 환경 기동 (전제)

표준 포트(8090/5173)가 점유된 환경에서는 **API=8091, web=5174** 를 쓴다:

```bash
make dev-up && make migrate-up                 # 인프라(5433/8180/6380/9000-9001)
# API — .env.dev 로드 후 포트만 교체
set -a; . ./.env.dev; set +a
NULLUS_SERVER_PORT=8091 go run ./cmd/api       # 또는 빌드본 실행
# web — proxy 대상은 env 로 지정 (vite.config.ts 수정 불필요)
cd web && NULLUS_API_PORT=8091 npm run dev -- --port 5174 --strictPort
```

확인: `curl localhost:8091/health` → `{"db":"connected",...}`, 로그인 `admin@nullus.dev / admin123`(dev mock).

기동 후 **`./scripts/e2e-preflight.sh`** 로 환경 등급을 한 번에 판정한다 — T0(인프라+API+web) / T1(+kind 클러스터) / T2(+도구 스택·PF) 중 어디까지 갖춰졌는지와, 부족한 항목의 해소 명령을 출력한다(F5 함정의 수동 점검을 대체). 포트가 다르면 `NULLUS_API_URL`·`NULLUS_WEB_URL` 로 지정.

### K8s(스택 배포)까지 갈 때

```bash
./scripts/runbook_local.sh kind-up             # 듀얼 kind (platform 1+2 / develop 1+1)
```

- **preflight**: Rancher Desktop VM 에서는 kind 생성 전에 inotify 상향 필수 (기본 128 로는 노드 부팅 실패):
  `rdctl shell sudo sysctl -w fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576` (VM 재시작 시 원복)
- kind 생성 격동 중 docker 데몬이 재시작되며 **compose 인프라가 조용히 내려갈 수 있다** → 생성 후 `/health` 재확인
- 스택 배포는 **Lite 템플릿(`gitea-jenkins-argocd-lite-v1`) 한정** (VM 4 vCPU 기준 GitLab 계열 불가)
- 클러스터 등록: README §5 (API 등록 → `POST /clusters/:id/verify` → `connected`)

### CI/CD 프로비저닝(파이프라인 생성) 테스트 시

API 가 클러스터 안 Gitea/Jenkins 에 닿아야 한다 — 로컬은 port-forward + URL override:

```bash
kubectl --context kind-nullus-platform port-forward -n nullus svc/gitea-http 3100:3000 &
kubectl --context kind-nullus-platform port-forward -n nullus svc/jenkins 8480:8080 &
NULLUS_GITEA_URL=http://localhost:3100 NULLUS_JENKINS_URL=http://localhost:8480 \
  NULLUS_SERVER_PORT=8091 ...  # API 재기동
```

### 메가 프로세스 완주 (빌드→배포까지, 2026-08-17 실증)

전 구간 흐름과 검증 지점:

```mermaid
flowchart LR
    A[클러스터 등록·Verify<br/>kind 듀얼] --> B[스택 설치<br/>기반 6 + 도구 3종]
    B --> C[파이프라인 생성<br/>UI 201 + API]
    C --> D["git clone<br/>(Gitea PF)"]
    D --> E[docker build]
    E --> F["kind load<br/>→ develop"]
    F --> G["apply → 파드 Running<br/>앱 HTTP 응답"]
```

**핵심 요령 — sudo·hosts·레지스트리 전부 불필요**:

1. **샘플 리포 시드**: Gitea 관리자 자격은 k8s secret 에서 추출
   (`kubectl -n nullus get secret nullus-gitea-credentials -o jsonpath='{.data}'` → base64 디코드)
   → `gitea_admin/spring-sample` 리포 생성 후 `Dockerfile`(nginx **8080 리슨** — backend 템플릿의 기대 포트) + 앱 파일 push.
2. **clone URL 을 PF 로 직접 지정**: 파이프라인의 `git_repo_url` 을 게이트웨이 도메인 대신
   `http://localhost:3100/gitea_admin/spring-sample.git` 로 — 도메인 해석(hosts) 문제가 사라진다.
3. **레지스트리 불필요**: `env_vars.IMAGE_REGISTRY_URL` 을 **비우면** 빌더가 push 대신
   **`kind load docker-image` 로 대상 클러스터에 직접 적재**한다(`internal/cicd/adapter/docker/builder.go`).
   등록 클러스터명의 `kind-` 접두사도 서버가 제거해 준다. "이미지 레지스트리를 결정할 수 없습니다" WARN 은
   CI 실행 기록용일 뿐 직접 배포를 막지 않는다.
4. API 로 재현하는 최소 페이로드:

```bash
curl -X POST $API/api/v1/cicd/pipelines -d '{
  "name":"e2e-direct","cluster_id":"<develop-id>","stack_id":"<stack-id>",
  "namespace":"default","app_type":"backend",
  "git_repo_url":"http://localhost:3100/gitea_admin/spring-sample.git",
  "dockerfile_path":"Dockerfile","docker_context":".","env_vars":{},
  "provision_repository":false,"port":8080,"replicas":1}'
curl -X POST $API/api/v1/cicd/pipelines/<pip-id>/deploy -d '{"version":"v1"}'   # version 필수
# 검증: deployments 상태 success → kubectl --context kind-nullus-develop get pods
```

## 3. Playwright 수행 방법

### 3.1 제품 내장 스위트 (`web/e2e`)

```bash
cd web
npm run e2e              # 전체 (기본 5173 — dev 서버 자동 기동)
npm run e2e:headed       # 브라우저 창을 보면서
npm run e2e:report       # 리포트 열기
# 포트 회피 환경(§2)에서는 떠 있는 서버를 지정 — dev 서버를 새로 띄우지 않는다
E2E_BASE_URL=http://localhost:5174 npx playwright test e2e/tour-regression.spec.ts
```

- **`tour-regression.spec.ts`** = 시나리오 기록지의 읽기 전용 구간(S0~S4·S6·S8·S9)을 단정(assert) 스펙으로 옮긴 회귀 게이트. 전제 데이터가 없으면(스택 미배포 등) 해당 스펙이 화면 상태를 보고 스스로 skip 한다 — 어느 등급이 갖춰졌는지는 preflight(§2)로 먼저 본다. **S5·S7(상태 변경 구간)은 제외** — 기록지의 수동 시나리오가 정본.
- `tour-walkthrough.spec.ts` 는 단정 없는 스크린샷 도구(눈검증용)로 역할이 다르다.

### 3.2 반응형 점검 (`scripts/responsive-audit.mjs`)

```bash
RESPONSIVE_BASE_URL=http://localhost:5174 npm run responsive:audit
npm run responsive:audit:check    # 이슈 발견 시 exit 1 (CI 게이트)
```

- 감지 2종: **가로 오버플로우**(`scrollWidth > clientWidth`) + **모바일 사이드바 미collapse**(`<aside>` 가 뷰포트 30% 초과)
- 산출물: `web/.responsive-audit/report-<date>.md`·`.json` + 화면별 스크린샷 (gitignore 됨)
- 뷰포트: 360/390/768px × 9개 화면. 로그인은 mock auth 자동 수행

### 3.3 일회성 시나리오 스크립트 작성 요령

시나리오 기록지의 자동 검증(S1~S4·S8 등)은 이 패턴으로 작성했다:

```js
import { chromium } from '<repo>/web/node_modules/playwright/index.mjs'  // 로컬 설치본 재사용
const browser = await chromium.launch()                       // headless (검증용)
// const browser = await chromium.launch({ headless: false, slowMo: 400 })  // 화면 시연용
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
// mock 로그인 → 라우트 이동 → 상태 검증(innerText/count) → screenshot
```

- **headed + `slowMo`** = 사람이 눈으로 따라가는 시연 모드. 자동 셀렉터가 실패하면 `waitForTimeout` 으로 수동 보조 창을 주는 하이브리드도 유효 (클러스터 선택 등)
- 셀렉터는 투어가 쓰는 `data-tour` 속성(`web/src/features/tour/tour-steps.ts`)을 우선 재사용 — 화면 리팩터링에 가장 강하다
- 클립보드 검증: `newContext({ permissions: ['clipboard-read','clipboard-write'] })` 후 `navigator.clipboard.readText()`
- **상태 변경 주의**: 저장/배포 버튼은 검증 목적을 정하고 누른다. 실험은 일회용 kind 클러스터에서만

## 4. 테스트 데이터·환경 정리

```bash
./scripts/runbook_local.sh kind-down    # 클러스터째 폐기 (스택·도구 포함)
make dev-down                            # 인프라 (볼륨 유지)
make dev-clean                           # 볼륨까지 초기화
```

## 5. 알려진 함정 (실측)

| 함정 | 증상 | 대처 |
|---|---|---|
| inotify 기본값 | kind 노드 "Multi-User System" 타임아웃 | `./scripts/e2e-preflight.sh` 가 kind 부재 시 값 점검·상향 명령 안내 (§2) |
| docker 데몬 재시작 | compose 인프라 조용히 다운 | `./scripts/e2e-preflight.sh` 로 일괄 점검 후 `make dev-up` 재실행 |
| `POST /stacks/:id/config` | **전체 교체형** — 부분 갱신 아님 | 항상 GET 후 전체 config 로 수정·재전송 |
| API 스택 생성 | `golden_path_id` 는 도구 목록을 펼치지 않음 | `config.artifacts/pipeline` 의 `ToolSelection(enabled+name)` 을 채워야 설치됨 |
| Add Tools 화면 | "Confirm & Deploy" 가 설정 저장만 하고 배포 미트리거 (버그 후보 F2) | 시나리오 기록지 발견 사항 참조 |
