# Authentik OIDC 테스트 가이드

로컬 개발환경에서 Authentik을 OIDC 프로바이더로 사용하여 인증 플로우를 테스트하는 가이드입니다.

## 개요

Nullus는 Keycloak과 Authentik 두 가지 OIDC 프로바이더를 지원합니다. 기본 개발 모드는 Keycloak(session 모드)이며, 이 가이드는 Authentik으로 전환하여 OIDC 인증을 테스트하는 방법을 설명합니다.

## 아키텍처

```
docker-compose.dev.yaml          docker-compose.auth.yaml (분리)
┌─────────────────────┐          ┌─────────────────────────┐
│ PostgreSQL  :5433    │          │ authentik-db (내부)      │
│ Redis       :6380    │          │ authentik-redis (내부)   │
│ MinIO       :9000    │          │ authentik-server  :9090  │
│ Keycloak    :8180    │          │ authentik-worker         │
└─────────────────────┘          └─────────────────────────┘
```

Authentik 서비스는 별도 compose 파일로 분리되어 있으며, 자체 PostgreSQL과 Redis를 사용합니다. 기존 인프라와 포트 충돌이 없습니다.

## Quick Start

### 1. Authentik 포함 전체 기동

```bash
./scripts/runbook_local.sh up --authentik
```

이 명령은 기존 인프라(PostgreSQL, Redis, MinIO, Keycloak) + API + 프론트엔드를 기동한 뒤, Authentik을 추가로 시작하고 자동 설정합니다.

### 2. Authentik만 별도 기동

이미 기본 환경이 실행 중인 경우:

```bash
# Authentik 서비스 기동
docker compose -f docker-compose.dev.yaml -f docker-compose.auth.yaml up -d \
  authentik-db authentik-redis authentik-server authentik-worker

# 초기 설정 (Application, Provider, 테스트 사용자 생성)
./scripts/setup-authentik.sh
```

### 3. 백엔드를 Authentik 모드로 전환

```bash
# config 파일 교체
cp configs/config.authentik.yaml configs/config.yaml

# API 서버 재시작
make run
```

### 4. 프론트엔드를 Authentik 모드로 전환

```bash
cd web
VITE_AUTH_MODE=oidc \
VITE_OIDC_PROVIDER=authentik \
VITE_OIDC_AUTHORITY=http://localhost:9090/application/o/nullus/ \
VITE_OIDC_CLIENT_ID=nullus-app \
npm run dev
```

## 접속 정보

| 서비스 | URL | 비고 |
|--------|-----|------|
| Authentik Admin | http://localhost:9090/if/admin/ | 관리 콘솔 |
| Authentik Login | http://localhost:9090/if/flow/default-authentication-flow/ | 로그인 화면 |

## 테스트 계정

| 이메일 | 비밀번호 | 그룹 (역할) |
|--------|----------|------------|
| admin@nullus.io | nullus123! | admin |
| devops@nullus.io | nullus123! | devops |
| dev@nullus.io | nullus123! | developer |

## 테스트 시나리오

### A. JWT 클레임 구조 차이 확인

Keycloak과 Authentik의 JWT 클레임 구조가 다릅니다:

| | Keycloak | Authentik |
|--|----------|-----------|
| 역할 위치 | `realm_access.roles[]` | `groups[]` |
| 구조 | 중첩 객체 | 플랫 배열 |

백엔드의 `OIDCProvider.ExtractRoles()`가 이 차이를 올바르게 처리하는지 확인합니다.

### B. OIDC 인증 플로우

1. 프론트엔드에서 "Sign in with Authentik" 클릭
2. Authentik 로그인 페이지로 리다이렉트
3. 테스트 계정으로 로그인
4. `http://localhost:5173/`로 콜백, JWT 발급
5. 역할에 따라 적절한 홈 페이지로 라우팅

### C. 프로바이더 전환 테스트

1. Keycloak 모드로 정상 로그인 확인
2. config를 Authentik으로 전환
3. API 서버 재시작 후 Authentik으로 로그인 확인
4. RBAC가 동일하게 동작하는지 검증

## 종료

```bash
# Authentik 포함 전체 중지
./scripts/runbook_local.sh down --authentik

# Authentik만 중지
docker compose -f docker-compose.dev.yaml -f docker-compose.auth.yaml stop \
  authentik-server authentik-worker authentik-db authentik-redis
```

## Keycloak 모드로 복원

```bash
# config 원복
git checkout configs/config.yaml

# API 서버 재시작
make run

# 프론트엔드 재시작 (환경변수 없이)
cd web && npm run dev
```

## 트러블슈팅

### Authentik이 시작되지 않음
```bash
docker compose -f docker-compose.dev.yaml -f docker-compose.auth.yaml logs authentik-server
```
- Authentik은 최초 기동 시 DB 마이그레이션으로 30~60초 소요됩니다.

### setup-authentik.sh 실행 실패
- Authentik health check 확인: `curl http://localhost:9090/-/health/ready/`
- Bootstrap token이 일치하는지 확인 (기본값: `nullus-authentik-bootstrap-token`)

### JWT 검증 실패
- Authentik issuer URL 확인: `http://localhost:9090/application/o/nullus/`
- JWKS 엔드포인트 확인: `curl http://localhost:9090/application/o/nullus/jwks/`
- `config.yaml`의 `auth.oidc.issuer_url`이 정확한지 확인

### 포트 충돌
- Authentik `:9090` — MinIO API(`:9000`)와 분리됨
- MinIO Console(`:9001`)과도 충돌 없음
