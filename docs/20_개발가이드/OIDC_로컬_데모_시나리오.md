# OIDC 설치 옵션화 1차 — 로컬 데모 시나리오

> EPIC: [cloud-nullus/nullus-plan#31](https://github.com/cloud-nullus/nullus-plan/issues/31) · PR: cloud-nullus/nullus#93 (merged)
> 관련 문서: [OIDC Provider 선정 기준](./OIDC_Provider_선정기준.md)
> 실행 기준 브랜치: `test/auth/oidc-provider-smoke` (provider별 smoke 검증 커밋 `6993461` 포함 — origin/main 미머지분이므로 반드시 이 브랜치의 worktree에서 데모할 것)

## 1. 배경 (왜 이 데모인가)

기존에는 로컬 개발환경(`runbook_local.sh up`)이 Keycloak을 **항상** 기동했고, realm 셋업은 수동이었으며, Authentik은 별도 플래그로만 추가 기동됐다. 이번 EPIC으로:

- IdP가 **선택 옵션**이 된다: `--auth=<keycloak|authentik|none>` (기본 `keycloak`, 현행 동작 100% 보존)
- Keycloak realm/클라이언트/계정이 **부팅 시 자동 셋업**된다 (KC_DB=dev-mem 휘발성 대응)
- `smoke`가 provider별 **부팅(OIDC discovery) + 로그인(password grant)** 을 검증한다
- 기존 `--authentik` 플래그는 하위호환 별칭으로 유지된다

이 데모는 위 4가지를 이해관계자 앞에서 그대로 재현한다.

## 2. 데모 환경 요약 (스크립트 기준 확정값)

| 항목 | 값 | 출처 |
|---|---|---|
| 웹 UI | http://localhost:5173 (`WEB_PORT`) | `scripts/runbook_local.sh:17` |
| API | http://localhost:8090 (`API_PORT`), 헬스 `/health` | `scripts/runbook_local.sh:16` |
| Keycloak | http://localhost:8180 (`KEYCLOAK_PORT`), admin / admin | `scripts/runbook_local.sh:22` |
| Keycloak realm / client | `nullus` / `nullus-app` (public, PKCE S256, redirect `http://localhost:5173/*`) | `scripts/setup-keycloak.sh` |
| OIDC 테스트 계정 | admin@nullus.io / devops@nullus.io / dev@nullus.io — 비밀번호 공통 `nullus123!` | `scripts/setup-keycloak.sh` |
| 프론트 mock 계정 (`--auth=none`용) | admin@nullus.dev / admin123 등 | `usage()` 출력 |
| Authentik | http://localhost:9090 (`AUTHENTIK_PORT`) | `scripts/runbook_local.sh:28` |
| 기타 인프라 | PostgreSQL 5433, Redis 6380, MinIO 9000/9001 | `scripts/runbook_local.sh:18-21` |

모든 명령은 worktree 루트(`~/workspace-test/nullus/oidc-option`)에서 실행한다.

## 3. 사전조건 확인 (데모 시작 전)

| # | 실행 | 기대 결과 | 실패 시 확인 |
|---|---|---|---|
| 0-1 | `docker info >/dev/null && echo docker-ok` | `docker-ok` | Docker Desktop/Colima 기동 (`open -a Docker` 등) |
| 0-2 | `lsof -tiTCP:5173 -sTCP:LISTEN` | **출력 없음** (포트 비어 있음) | ⚠️ 포트 5173은 다른 프로젝트(bc-agentcell/ui vite)가 점유 중일 수 있음. PID가 나오면 해당 프로세스 종료 후 재확인. `up`은 5173/8090 점유 시 즉시 중단됨(`require_port_free`) |
| 0-3 | `lsof -tiTCP:8090 -sTCP:LISTEN; lsof -tiTCP:8180 -sTCP:LISTEN` | 출력 없음 | 이전 세션 잔재면 `./scripts/runbook_local.sh down` 먼저 |
| 0-4 | `git -C . status -sb` | `## test/auth/oidc-provider-smoke...` | 다른 브랜치면 데모용 smoke 분기 코드가 없음 |
| 0-5 | `./scripts/runbook_local.sh preflight` | `preflight OK` (go/node/docker 버전 출력) | 누락 toolchain 설치 |

## 4. 시연 단계

### S1. 기본 provider(Keycloak)로 기동 — `up --auth=keycloak`

> 인자 없는 `up`과 동일(기본값 keycloak)임을 구두로 언급. 최초 실행은 npm ci 포함 2~4분.

| 실행 | 기대 결과 (화면에서 확인) |
|---|---|
| `./scripts/runbook_local.sh up --auth=keycloak` | ① `starting docker infra (postgres redis minio keycloak) [auth=keycloak]...` — **provider가 인프라 구성에 반영됨** ② `waiting for keycloak...` → `keycloak is ready, running realm setup...` → `Keycloak realm 'nullus' configured.` — **realm 자동 셋업** ③ 요약 박스에 `Keycloak      http://localhost:8180  (admin/admin)` ④ `wrote web/.env.development.local (VITE_AUTH_MODE=oidc, provider=keycloak)` — **프런트가 자동으로 OIDC 모드로 전환됨** ⑤ 요약에 `Web auth      OIDC (web/.env.development.local 자동 생성됨)` 과 `SSO 프로비저닝  ON` |

실패 시 확인: `keycloak realm setup failed (non-blocking...)`가 뜨면 Keycloak 기동 지연 — `scripts/setup-keycloak.sh` 수동 재실행. API 실패는 `.runbook-logs/api.log`.

### S2. Keycloak 화면/헬스 + realm 자동 셋업 확인

| # | 실행 | 기대 결과 |
|---|---|---|
| 2-1 | 브라우저: http://localhost:8180 → Administration Console → admin / admin | Keycloak 26 관리 콘솔 로그인 성공 |
| 2-2 | 좌상단 realm 선택 드롭다운 | **`nullus` realm 존재** (자동 생성됨) |
| 2-3 | nullus realm → Clients | **`nullus-app`** 존재, redirect URI `http://localhost:5173/*` |
| 2-4 | nullus realm → Users | admin@nullus.io / devops@nullus.io / dev@nullus.io 3계정 |
| 2-5 | (터미널) `curl -fsS http://localhost:8180/realms/nullus/.well-known/openid-configuration \| python3 -m json.tool \| head -5` | JSON에 `"issuer": "http://localhost:8180/realms/nullus"` |
| 2-6 | (터미널) API env 주입 확인: `ps eww $(pgrep -f bin/api) \| tr ' ' '\n' \| grep NULLUS_AUTH_OIDC` | `NULLUS_AUTH_OIDC_PROVIDER=keycloak`, `NULLUS_AUTH_OIDC_ISSUER_URL=http://localhost:8180/realms/nullus` |

### S3. smoke — OIDC discovery + 로그인 검증

| 실행 | 기대 결과 |
|---|---|
| `./scripts/runbook_local.sh smoke --auth=keycloak` | 기존 API smoke 14건 OK에 이어: `[auth smoke: provider=keycloak]` 아래 ① `GET keycloak openid-configuration   OK (200)` ② `POST keycloak token (login smoke)   OK` — admin@nullus.io password grant로 access_token 발급 성공. 마지막 줄 `[nullus] smoke: 16 passed, 0 failed`, exit 0 |

실패 시 확인: token FAIL이면 realm 셋업 여부(S2) 및 client `directAccessGrantsEnabled` 확인 (`setup-keycloak.sh`가 `true`로 생성).

### S4. 브라우저 관점 로그인 플로우 (Authorization Code + PKCE)

S1의 `up --auth=keycloak`이 `web/.env.development.local`을 이미 만들어 뒀으므로 손으로 고칠 것이 없다.
(추적 파일인 `web/.env.development`는 그대로 `VITE_AUTH_MODE=mock`으로 남는다 — Vite가 `.local`을 더 높은
우선순위로 읽는다. 그래서 워킹트리가 더러워지지 않는다.)

| # | 실행 | 기대 결과 |
|---|---|---|
| 4-1 | `cat web/.env.development.local` | `VITE_AUTH_MODE=oidc` / `VITE_OIDC_PROVIDER=keycloak` / `VITE_OIDC_AUTHORITY=http://localhost:8180/realms/nullus` / `VITE_OIDC_CLIENT_ID=nullus-app` — runbook이 생성한 값. 반영이 안 됐으면 `./scripts/runbook_local.sh refresh` |
| 4-2 | 브라우저: http://localhost:5173 접속 → 로그인 진입 | **Keycloak 로그인 페이지로 redirect** — 주소창이 `http://localhost:8180/realms/nullus/protocol/openid-connect/auth?...client_id=nullus-app...` |
| 4-3 | admin@nullus.io / `nullus123!` 입력 | 인증 성공 → **`http://localhost:5173/...`으로 redirect back**, 웹 UI에 로그인된 상태로 진입 |
| 4-4 | (선택) 개발자도구 Network 탭 | `.../protocol/openid-connect/token` 요청에 `code_verifier` 포함 (PKCE) |

실패 시 확인: redirect_uri 오류면 client `nullus-app`의 redirect `http://localhost:5173/*` 확인(2-3). CORS 오류면 webOrigins `http://localhost:5173`.

### S5. 실패 케이스 — 잘못된 provider 즉시 차단

| 실행 | 기대 결과 |
|---|---|
| `./scripts/runbook_local.sh up --auth=bogus; echo "exit=$?"` | `[nullus] invalid auth provider: 'bogus' (allowed: keycloak \| authentik \| none)` + `exit=1`. **docker 호출 전에 즉시 종료** — 기동 중이던 환경에 부작용 없음 |

### S6. `--auth=none` 경로 — IdP 없이 mock auth

| # | 실행 | 기대 결과 |
|---|---|---|
| 6-1 | `./scripts/runbook_local.sh down` | api/web 프로세스 중지 + `stopping docker infra...` + `all stopped` (keycloak 컨테이너 포함 정리) |
| 6-2 | `./scripts/runbook_local.sh up --auth=none` | ① `starting docker infra (postgres redis minio) [auth=none]...` — **keycloak 미포함** ② `removed web/.env.development.local (mock auth 로 되돌림)` — **원복도 자동** ③ 요약에 `Auth          none (frontend mock auth: VITE_AUTH_MODE=mock)` ④ Web auth/SSO 블록 **미출력** |
| 6-3 | `docker ps --format '{{.Names}}' \| grep -c keycloak; true` | `0` — Keycloak 컨테이너 없음 (리소스 절약 확인) |
| 6-4 | `./scripts/runbook_local.sh smoke --auth=none` | API smoke 14건 OK + `[auth smoke: provider=none]` 아래 `auth=none   SKIPPED (mock auth)`, `14 passed, 0 failed` |
| 6-5 | (선택) 브라우저 http://localhost:5173 → admin@nullus.dev / admin123 | Keycloak 없이 mock 로그인 성공 |

### S7. `--authentik` 하위호환 별칭 확인 + 최종 정리 — `down`

| # | 실행 | 기대 결과 |
|---|---|---|
| 7-1 | `./scripts/runbook_local.sh down --authentik` | 별칭이 `--auth=authentik`으로 파싱되어 `stopping docker infra (incl. authentik)...` — **compose에 `docker-compose.auth.yaml`이 포함**된 down 경로 실행 (authentik 미기동 상태여도 무해). 마지막 `all stopped` |
| 7-2 | `docker compose -f docker-compose.dev.yaml ps -q \| wc -l` | `0` — 컨테이너 전부 정리 |
| 7-3 | `lsof -tiTCP:8090 -sTCP:LISTEN; lsof -tiTCP:5173 -sTCP:LISTEN; true` | 출력 없음 — api/web 프로세스 정리 완료 |

> (선택 부록) Authentik 전체 기동 데모: `./scripts/runbook_local.sh up --auth=authentik` — 컨테이너 4종(db/redis/server/worker) 추가 기동, 대기 최대 180s, 요약에 `Authentik     http://localhost:9090`. smoke는 `smoke --auth=authentik`으로 health + openid-configuration 2건 검증. 시간 관계상 본 데모 기본 흐름에서는 제외.

## 5. 데모 불가 / 범위 제외 항목

- **Authentik 웹 로그인 end-to-end**: JWT 미들웨어의 JWKS 경로가 Keycloak 형태로 고정되어 있어 Authentik issuer와 불일치 — 별도 WIP 브랜치(`feat/auth/local-oidc-test-env`)에서 진행 중, 본 EPIC 범위 제외 ([선정 기준 문서](./OIDC_Provider_선정기준.md) 참조). 본 데모의 Authentik 검증은 discovery/health smoke까지.
- **README/CHANGELOG 현행화 (Task 7)**: 본 시나리오 문서와 함께 후속 문서화 PR에서 처리.

## 6. 소요 시간 가이드

| 구간 | 예상 |
|---|---|
| 사전조건 + S1 (최초 npm ci 포함) | 3~5분 |
| S2~S4 (Keycloak 확인 + smoke + 브라우저 로그인) | 3~4분 |
| S5~S7 (실패 케이스 + none + 별칭/정리) | 2~3분 |
| 총 | 약 10분 |
