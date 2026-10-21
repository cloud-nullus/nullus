# 환경 변수

서버는 `configs/config.yaml` 을 읽고 그 위에 환경변수를 덮어씁니다.

## 이름 규칙

설정 키 `a.b.c` → 환경변수 **`NULLUS_A_B_C`**.

!!! warning "설정 파일에 없는 키는 환경변수만으로 덮어쓸 수 없습니다"
    자동 매핑이 **설정 파일에 있는 키만** 봅니다. 없는 키는 코드에서 따로 묶어
    줘야 하고, 그러지 않으면 환경변수를 넣어도 **조용히 무시**됩니다 — 백업 봉인
    키가 그 함정에 걸려 기동이 막힌 적이 있습니다.

    따로 묶여 있는 키: `platform.namespace` · `backup.seal_key` ·
    `backup.destination.secret_key` · `backup.keycloak_database.password` ·
    `server.rate_limit.*`.

## 서버·데이터베이스

| 환경변수 | 기본값 |
|----------|--------|
| `NULLUS_SERVER_PORT` | 8090 |
| `NULLUS_SERVER_MODE` | development |
| `NULLUS_SERVER_RATE_LIMIT_AUTHENTICATED` | 0 → 사용자당 분당 300 |
| `NULLUS_SERVER_RATE_LIMIT_UNAUTHENTICATED` | 0 → IP 당 분당 30 (development 는 사용자 한도와 같습니다 — 명시하면 그 값) |
| `NULLUS_SERVER_RATE_LIMIT_IP_CEILING` | 0 → IP 당 분당 600 (사용자 · 미인증 한도를 올리면 따라 오릅니다 — 명시하면 그 값) |
| `NULLUS_DATABASE_HOST` | localhost |
| `NULLUS_DATABASE_PORT` | 5433 |
| `NULLUS_DATABASE_NAME` | nullus |
| `NULLUS_DATABASE_USER` | nullus |
| `NULLUS_DATABASE_PASSWORD` | nullus_dev |
| `NULLUS_DATABASE_SSLMODE` | disable |
| `NULLUS_DATABASE_MAX_OPEN_CONNS` / `_MAX_IDLE_CONNS` | 25 / 10 |
| `NULLUS_DATABASE_CONN_MAX_LIFETIME` / `_CONN_MAX_IDLE_TIME` | 5m / 1m |

요청 한도는 넘으면 429 로 거절합니다. 0 이하는 기본값입니다. 한 IP 에서 여러 화면을 한꺼번에 여는
자동화(e2e 등)는 개발 모드에서도 분당 300 을 넘길 수 있어 CI 의 e2e 는 한도를 넉넉히 올려 띄웁니다
(`AUTHENTICATED=6000` · `IP_CEILING=12000`). 로컬에서 Playwright 를 돌릴 때도 같은 값을 줍니다.
명시한 IP 상한이 사용자 한도보다 낮으면 기동 로그에 경고가 남습니다. 로그인 · 배포 한도(분당 10)는 바꿀 수
없습니다. 한도는 요청 시각을 모두 기록해 세므로 필요 이상으로 크게 잡지 않습니다.

!!! note "옛 `NULLUS_DB_*` 는 먹지 않습니다"
    `NULLUS_DB_HOST` 처럼 줄여 쓴 이름은 어느 설정 키와도 맞지 않아 무시됩니다.
    `.env.example` · `.env.dev` · `Makefile` 은 올바른 이름으로 고쳤지만, 전에
    복사해 둔 `.env` 가 있으면 그쪽을 바꿔야 합니다.

## 인증

| 환경변수 | 기본값 | 비고 |
|----------|--------|------|
| `NULLUS_AUTH_MODE` | **session** | `oidc` 또는 그 외. 운영은 `oidc` |
| `NULLUS_AUTH_SESSION_SECRET` | dev-secret-change-in-production | 로컬 토큰 발급 HMAC 키. **비면 `/auth/login` 라우트가 등록되지 않습니다** |
| `NULLUS_AUTH_SESSION_MAX_AGE` | 86400 | 0 이하면 24시간으로 되돌립니다 |
| `NULLUS_AUTH_OIDC_PROVIDER` | keycloak | |
| `NULLUS_AUTH_OIDC_ISSUER_URL` | `http://localhost:8180/realms/nullus` | `oidc` 모드에서 비면 **기동 거부** |
| `NULLUS_AUTH_OIDC_AUDIENCE` | nullus-app | |

## Keycloak (별칭이 있는 것들)

| 환경변수 | 별칭 | 기본값 |
|----------|------|--------|
| `NULLUS_KEYCLOAK_ADMIN_URL` | `KEYCLOAK_URL` | 빈 값 — 비우면 OSS SSO 연동 전체를 건너뜁니다 |
| `NULLUS_KEYCLOAK_PUBLIC_URL` | `KEYCLOAK_PUBLIC_URL` | 비면 issuer 를 물려받습니다 |
| `NULLUS_KEYCLOAK_REALM` | `KEYCLOAK_REALM` | nullus |
| `NULLUS_KEYCLOAK_ADMIN_USER` | `KEYCLOAK_ADMIN_USER` | admin |
| `NULLUS_KEYCLOAK_ADMIN_PASSWORD` | `KEYCLOAK_ADMIN_PASSWORD` | 빈 값 |

앞쪽(접두사가 붙은) 이름이 우선합니다.

## 그 밖의 설정 키

| 환경변수 | 기본값 |
|----------|--------|
| `NULLUS_HELM_TIMEOUT` / `NULLUS_HELM_NAMESPACE_PREFIX` | 600s / `nullus-` |
| `NULLUS_PROMETHEUS_URL` | 빈 값 |
| `NULLUS_LOG_LEVEL` / `NULLUS_LOG_FORMAT` | debug / text |
| `NULLUS_PLATFORM_NAMESPACE` | 차트가 주입 |
| `NULLUS_BACKUP_*` | [백업 문서](../features/admin/backup.md) 참고 |

## 접두사 규칙을 따르지 않는 변수

이들은 설정 로더를 거치지 않고 코드가 직접 읽습니다.

| 환경변수 | 하는 일 | 기본값 |
|----------|---------|--------|
| **`ENCRYPTION_KEY`** | kubeconfig·토큰 암복호. **정확히 32바이트** | 없음 — 비면 런타임 500 |
| `OPENBAO_ADDR` / `OPENBAO_TOKEN` | 로컬 전용 전역 OpenBao | 둘 다 있어야 등록 |
| `NULLUS_GITLAB_URL` / `NULLUS_GITEA_URL` / `NULLUS_JENKINS_URL` | 도구 주소 덮어쓰기 | 빈 값 |
| `NULLUS_HELM_OCI_REGISTRY` | 에어갭 OCI 레지스트리. 설치 이미지 스캔도 이 호스트에서 이미지를 받는다(노드의 containerd 미러와 같은 레지스트리) | 빈 값 |
| `NULLUS_TELEGRAM_API_URL` | 알림 테스트용 | 빈 값 |
| `NULLUS_STACK_ALERT_INTERVAL` | 경보 수집 주기 | 1m (`off`·`0` 이면 끔) |
| `TOKEN_ROTATION_INTERVAL` | 토큰 회전 점검 주기 | **5m** |
| `CICD_RUN_SYNC_INTERVAL` | CI 실행 동기화 주기 | **10m** |
| `STACK_IMAGE_RESCAN_INTERVAL` | 설치 이미지 재스캔 주기(에어갭 포함). 새로 발견된 수정판 있는 CRITICAL 은 알림 `stack_cve_found` 로 간다 | **24h** |
| `NULLUS_SCM_GROUP` | 파이프라인용 GitLab 그룹 | **`nullus`** |
| `NULLUS_DEFAULT_ORG_ID` | 기본 조직 ID | 코드의 고정 UUID |

### CLI·MCP 전용

`NULLUS_SERVER` · `NULLUS_TOKEN` · `NULLUS_CONFIG_DIR` ·
`NULLUS_MCP_ALLOW_WRITE` · `NULLUS_SCM_TOKEN`

### `nullus-bootstrap` 전용

`KEYCLOAK_URL`(필수) · `KEYCLOAK_REALM`(nullus) · `KEYCLOAK_ADMIN_USER`(admin) ·
`KEYCLOAK_ADMIN_PASSWORD` · `BOOTSTRAP_CLIENT_ID`(nullus-bootstrap) ·
`BOOTSTRAP_ROLE`(admin)

### `token-source-sync` 전용

`DATABASE_URL` · `OPENBAO_ADDR`(필수) · `OPENBAO_TOKEN`(필수) ·
`TOKEN_SOURCE_ENV`(기본 `dev`)

## 설정 파일 경로는 바꿀 수 없습니다

서버가 `configs/config.yaml` 을 **하드코딩**해 읽습니다. 경로를 바꾸는 플래그도
환경변수도 없습니다. 컨테이너에서는 차트가 ConfigMap 을 알맞은 위치에 마운트해
맞춰 둡니다.

`configs/config.dev.yaml` 은 Makefile·스크립트·워크플로·코드 어디에서도
참조하지 않습니다(고아 파일로 보입니다).
