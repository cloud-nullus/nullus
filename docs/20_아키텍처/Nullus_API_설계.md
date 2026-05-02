# Nullus Platform REST API 설계

**작성일**: 2026-03-14
**버전**: 1.0
**기반 문서**: nullus_PRD_1.3.md, Nullus_기능목록.md, Nullus 상세 기능 명세 및 시스템 아키텍처.md
**대상 독자**: Backend 엔지니어, Frontend 엔지니어, API 소비자

---

## 목차

1. [API 개요](#1-api-개요)
2. [공통 규격](#2-공통-규격)
3. [인증 체계](#3-인증-체계)
4. [API 버전 관리 전략](#4-api-버전-관리-전략)
5. [Rate Limiting 정책](#5-rate-limiting-정책)
6. [모듈별 엔드포인트](#6-모듈별-엔드포인트)
   - 6.1 Auth 모듈
   - 6.2 Organization 모듈
   - 6.3 Cluster 모듈
   - 6.4 Stack 모듈
   - 6.5 Template 모듈
   - 6.6 Installation (배포) 모듈
   - 6.7 Pipeline (CI/CD) 모듈
   - 6.8 Monitoring (Observability) 모듈
   - 6.9 Compatibility 모듈
   - 6.10 Resource 모듈
   - 6.11 User (RBAC) 모듈
7. [WebSocket 엔드포인트](#7-websocket-엔드포인트)
8. [OpenAPI 3.0 공통 스키마](#8-openapi-30-공통-스키마)

---

## 1. API 개요

### 1.1 기본 정보

| 항목 | 값 |
|------|-----|
| Base URL | `https://{host}/api/v1` |
| 프로토콜 | HTTPS (TLS 1.3) |
| 데이터 형식 | JSON (`application/json`) |
| 문자 인코딩 | UTF-8 |
| API 문서 생성 도구 | swaggo/swag (Go 구조체 → OpenAPI 3.0 자동 생성) |
| WebSocket URL | `wss://{host}/ws` |

### 1.2 OpenAPI 3.0 메타정보

```yaml
openapi: "3.0.3"
info:
  title: Nullus Platform API
  version: "1.0.0"
  description: >
    Kubernetes 기반 DevSecOps 자동화 플랫폼 API.
    Golden Path 템플릿 기반으로 검증된 CI/CD 도구 조합을 노코드 UI로 설정하고,
    한 번의 배포로 전체 DevSecOps 스택을 자동 설치합니다.
  contact:
    name: Nullus 팀
    url: https://github.com/cloud-nullus
  license:
    name: Apache 2.0
    url: https://www.apache.org/licenses/LICENSE-2.0
servers:
  - url: https://nullus.example.com/api/v1
    description: Production
  - url: http://localhost:8090/api/v1
    description: Local Development
```

### 1.3 설계 원칙

- **RESTful**: 리소스 중심 URL 설계, HTTP 메서드로 행위 표현
- **일관성**: 모든 엔드포인트에 동일한 요청/응답 패턴 적용
- **점진적 공개**: Alpha → Beta → v1 단계별로 엔드포인트 활성화
- **하위 호환**: 기존 필드 제거 없이 추가만 허용, Breaking Change 시 새 버전(`/api/v2`) 신설

---

## 2. 공통 규격

### 2.1 HTTP 상태 코드

| 코드 | 의미 | 사용 시점 |
|------|------|-----------|
| `200 OK` | 성공 | GET, PUT 요청 성공 |
| `201 Created` | 리소스 생성 | POST 요청으로 리소스 생성 성공 |
| `204 No Content` | 본문 없는 성공 | DELETE 요청 성공 |
| `400 Bad Request` | 잘못된 요청 | 요청 body 유효성 검증 실패 |
| `401 Unauthorized` | 인증 필요 | 세션/토큰 없거나 만료 |
| `403 Forbidden` | 권한 부족 | RBAC 권한 미충족 |
| `404 Not Found` | 리소스 없음 | 존재하지 않는 리소스 접근 |
| `409 Conflict` | 충돌 | 중복 리소스 생성 시도 (slug 중복 등) |
| `422 Unprocessable Entity` | 비즈니스 규칙 위반 | 호환성 검증 실패 등 |
| `429 Too Many Requests` | 요청 제한 초과 | Rate Limit 초과 |
| `500 Internal Server Error` | 서버 오류 | 예기치 않은 서버 오류 |
| `504 Gateway Timeout` | 타임아웃 | Helm 설치 등 장시간 작업 타임아웃 |

### 2.2 표준 에러 응답 형식

모든 에러 응답은 아래 형식을 따릅니다.

```json
{
  "error": {
    "code": "CLUSTER_VERIFY_UNREACHABLE",
    "http_status": 422,
    "message": "클러스터에 연결할 수 없습니다",
    "detail": "엔드포인트 https://35.x.x.x:6443에 TCP 연결 실패 (timeout 10s)",
    "retryable": true,
    "trace_id": "tr_a1b2c3d4e5f6"
  }
}
```

| 필드 | 타입 | 설명 |
|------|------|------|
| `code` | `string` | 머신이 읽을 수 있는 에러 코드. 네이밍: `{DOMAIN}_{ACTION}_{REASON}` |
| `http_status` | `integer` | HTTP 상태 코드 |
| `message` | `string` | 사용자에게 표시할 메시지 (i18n 지원, `Accept-Language` 헤더 기반) |
| `detail` | `string` | 디버깅용 상세 정보 (프로덕션에서는 선택적 노출) |
| `retryable` | `boolean` | 클라이언트가 자동 재시도 가능 여부 |
| `trace_id` | `string` | 서버 로그 추적용 고유 ID (`tr_` 접두사) |

#### 에러 코드 목록 (주요)

| 에러 코드 | HTTP | 설명 |
|-----------|------|------|
| `AUTH_LOGIN_INVALID_CREDENTIALS` | 401 | 잘못된 이메일/비밀번호 |
| `AUTH_SESSION_EXPIRED` | 401 | 세션 만료 |
| `AUTH_FORBIDDEN` | 403 | RBAC 권한 부족 |
| `ORG_CREATE_SLUG_DUPLICATE` | 409 | Organization 슬러그 중복 |
| `ORG_STATUS_INACTIVE` | 403 | 비활성 Organization 접근 |
| `CLUSTER_VERIFY_UNREACHABLE` | 422 | 클러스터 연결 불가 |
| `CLUSTER_VERIFY_AUTH_FAILED` | 422 | kubeconfig 인증 실패 |
| `STACK_CONFIG_INVALID` | 400 | 스택 설정 유효성 검증 실패 |
| `STACK_DEPLOY_CLUSTER_NOT_READY` | 422 | 클러스터 미연결 상태에서 배포 시도 |
| `INSTALL_HELM_TIMEOUT` | 504 | Helm 차트 설치 시간 초과 |
| `INSTALL_HELM_FAILED` | 500 | Helm 차트 설치 실패 |
| `INSTALL_ROLLBACK_FAILED` | 500 | 롤백 중 오류 발생 |
| `COMPATIBILITY_UNTESTED` | 422 | 비검증 도구 조합 |
| `RESOURCE_INVALID_VALUE` | 400 | 리소스 값 범위 초과 또는 음수 |
| `PIPELINE_DEPLOY_FAILED` | 500 | 파이프라인 배포 실패 |
| `RATE_LIMIT_EXCEEDED` | 429 | 요청 제한 초과 |

### 2.3 표준 성공 응답 형식

#### 단일 리소스 응답

```json
{
  "data": {
    "id": "org_a1b2c3d4",
    "name": "Nullus 팀",
    "slug": "nullus-team",
    "status": "active",
    "created_at": "2026-03-14T09:00:00Z"
  }
}
```

#### 목록 응답 (페이지네이션 포함)

```json
{
  "data": [
    { "id": "cls_001", "name": "production-gke" },
    { "id": "cls_002", "name": "staging-eks" }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total_items": 42,
    "total_pages": 3,
    "has_next": true,
    "has_prev": false
  }
}
```

### 2.4 페이지네이션

모든 목록 API는 커서 기반 또는 오프셋 기반 페이지네이션을 지원합니다. Phase 1에서는 오프셋 기반을 사용합니다.

| 파라미터 | 타입 | 기본값 | 설명 |
|----------|------|--------|------|
| `page` | `integer` | `1` | 페이지 번호 (1부터 시작) |
| `page_size` | `integer` | `20` | 페이지당 항목 수 (최대 100) |

```
GET /api/v1/admin/clusters?page=2&page_size=10
```

### 2.5 필터링

쿼리 파라미터로 필터링합니다. 리소스별로 지원하는 필터가 다릅니다.

```
GET /api/v1/admin/clusters?status=connected&type=pipeline
GET /api/v1/cicd/pipelines?status=running&template_id=web-backend-v1
GET /api/v1/cicd/deployments?status=success
```

### 2.6 정렬

`sort` 파라미터로 정렬합니다. `-` 접두사는 내림차순을 의미합니다.

```
GET /api/v1/admin/clusters?sort=-created_at
GET /api/v1/stacks?sort=name
GET /api/v1/cicd/deployments?sort=-deployed_at
```

### 2.7 공통 요청 헤더

| 헤더 | 필수 | 설명 |
|------|------|------|
| `Content-Type` | O (POST/PUT) | `application/json` |
| `Accept` | X | `application/json` (기본값) |
| `Accept-Language` | X | `ko`, `en` (에러 메시지 언어, 기본 `en`) |
| `Cookie` | O (Alpha/Beta) | 세션 쿠키 (`nullus_session`) |
| `Authorization` | O (v1) | `Bearer {access_token}` (Keycloak OIDC) |
| `X-Request-ID` | X | 클라이언트 요청 추적 ID (없으면 서버 자동 생성) |

### 2.8 공통 응답 헤더

| 헤더 | 설명 |
|------|------|
| `X-Request-ID` | 요청 추적 ID (요청의 `X-Request-ID` 또는 서버 자동 생성) |
| `X-RateLimit-Limit` | 시간 윈도우 내 최대 요청 수 |
| `X-RateLimit-Remaining` | 남은 요청 수 |
| `X-RateLimit-Reset` | 제한 초기화 Unix 타임스탬프 |
| `Deprecation` | 더 이상 사용되지 않는 엔드포인트일 때 `true` |
| `Sunset` | 엔드포인트 제거 예정 날짜 (RFC 7231) |

---

## 3. 인증 체계

### 3.1 Alpha/Beta: 세션 기반 인증

Alpha/Beta 단계에서는 빠른 구현을 위해 세션 기반 인증을 사용합니다.

```
POST /api/v1/auth/login
→ Set-Cookie: nullus_session=abc123; HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=86400

이후 요청:
Cookie: nullus_session=abc123
```

| 항목 | 설정 |
|------|------|
| 세션 저장소 | PostgreSQL (gorilla/sessions) |
| 쿠키 속성 | `HttpOnly`, `Secure`, `SameSite=Strict` |
| 세션 만료 | 24시간 (슬라이딩 윈도우) |
| CSRF 보호 | `SameSite=Strict` + Double Submit Cookie |

### 3.2 v1 GA: Keycloak OIDC

v1 정식 릴리스에서는 Keycloak OIDC로 전환합니다.

```
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
```

| 항목 | 설정 |
|------|------|
| IdP | Keycloak (Nullus 스택에 포함 설치) |
| 프로토콜 | OpenID Connect (Authorization Code Flow + PKCE) |
| 액세스 토큰 만료 | 15분 |
| 리프레시 토큰 만료 | 7일 |
| Token 검증 | Keycloak JWKS 엔드포인트에서 공개키로 서명 검증 |

#### OIDC 인증 흐름

```
1. Browser → Nullus Web → Keycloak /auth (Authorization Code + PKCE)
2. Keycloak → 사용자 로그인 → Authorization Code 반환
3. Nullus Web → Keycloak /token (Code + Code Verifier 교환)
4. Keycloak → ID Token + Access Token + Refresh Token 발급
5. Nullus Web → Nullus API (Authorization: Bearer {access_token})
6. Nullus API → Keycloak JWKS로 토큰 서명 검증
7. Nullus API → DB에서 org_members.role 조회 → RBAC 적용
```

#### Keycloak 자동 설정 (Narwhal 레퍼런스)

Nullus 스택 설치 시 Keycloak이 자동 구성됩니다.

1. Keycloak realm (`nullus`) 생성
2. `groups` client scope 생성 + group membership mapper 추가
3. 앱별 OIDC 클라이언트 생성 (ArgoCD, Grafana, Gitea 등)
4. 전체 클라이언트에 `groups` scope를 default scope로 할당
5. 각 앱의 Helm values에 OIDC 설정 주입
6. K8s API Server OIDC 연동 (선택)

#### OpenBao-first 시크릿 연계 (신규)

Nullus v1.x에서는 토큰/비밀번호/client secret의 원문 저장소를 OpenBao로 통일합니다.

1. Stack 배포 시작 전 OpenBao 준비 상태를 서버가 사전 검증
2. 앱별 OIDC/SCM/Registry/Webhook credential은 OpenBao path에서 읽어 주입
3. API 응답 본문/로그/에러 detail에 비밀 원문을 포함하지 않음
4. Kubernetes Secret은 애플리케이션 런타임 참조를 위한 파생 리소스로만 사용

권장 연계 방식:

- `auth/kubernetes` + short-lived token
- External Secrets Operator(ESO) 또는 Secrets Store CSI Driver

#### OSS별 권한 매핑

```
Keycloak Role "admin"     → GitLab Admin + Argo CD Admin + Grafana Admin
Keycloak Role "devops"    → GitLab Maintainer + Argo CD Read-only + Grafana Editor
Keycloak Role "developer" → GitLab Reporter + Argo CD Read-only + Grafana Viewer
```

### 3.3 RBAC 권한 매트릭스

PRD v1.3 확정 3역할 체계: **Admin / DevOps Engineer / Developer**

| 리소스 | 행위 | Admin | DevOps Engineer | Developer |
|--------|------|-------|-----------------|-----------|
| Organization | 생성/수정/삭제 | O | X | X |
| Organization | 조회 | O | O | O |
| User | 관리 (역할 부여, 비활성화) | O | X | X |
| Cluster | 등록/수정/삭제 | O | O | X |
| Cluster | 조회 | O | O | X |
| Stack Config | 생성/수정 | O | O | X |
| Stack Config | 조회 | O | O | X |
| Stack Deploy | 실행/롤백 | O | O | X |
| Golden Path Template | 조회 | O | O | X |
| Pipeline | 생성/배포 | O | O | O |
| Pipeline | 조회/이력 | O | O | O |
| Pipeline | 롤백 | O | O | X |
| Monitoring | 조회 | O | O | O |
| Alert Config | 설정 | O | O | X |
| Compatibility | 조회/검증 | O | O | X |
| Resource Estimate | 계산 | O | O | X |

---

## 4. API 버전 관리 전략

### 4.1 버전 정책

- **URL Path 기반 버전**: `/api/v1/`, `/api/v2/`
- **현재 버전**: `v1` (Alpha ~ v1 GA까지 동일 경로 사용)
- **Breaking Change 기준**: 기존 필드 제거, 필드 타입 변경, 필수 파라미터 추가

### 4.2 하위 호환 변경 (Non-Breaking)

아래 변경은 버전 증가 없이 수행합니다.

- 응답에 새 필드 추가
- 새 엔드포인트 추가
- 선택적(optional) 요청 파라미터 추가
- 새 enum 값 추가 (클라이언트는 unknown enum을 무시해야 함)

### 4.3 Deprecation 정책

기능 제거 시 최소 **4주(2 스프린트)** 전에 공지합니다.

```
HTTP/1.1 200 OK
Deprecation: true
Sunset: Sat, 30 May 2026 00:00:00 GMT
Link: </api/v2/clusters>; rel="successor-version"
```

### 4.4 릴리스별 엔드포인트 활성화

| 엔드포인트 그룹 | Alpha | Beta | v1 GA |
|-----------------|-------|------|-------|
| Auth (세션) | O | O | O (Keycloak 병행) |
| Auth (OIDC) | X | X | O |
| Organization (기본) | O | O | O |
| Organization (멤버 관리) | X | O | O |
| Cluster CRUD + Verify | O | O | O |
| Stack CRUD | O | O | O |
| Stack History/Diff | X | X | O |
| Golden Path Template | O | O | O |
| Installation (Deploy) | O | O | O |
| Installation (Rollback) | X | O | O |
| Installation (Retry) | X | O | O |
| Pipeline CRUD + Deploy | X | O | O |
| Pipeline Rollback/Diff | X | X | O |
| Monitoring Dashboard | X | O | O |
| Alert Config | X | O | O |
| Compatibility Matrix | O | O | O |
| Resource Estimate | O | O | O |
| User/RBAC | X | X | O |
| WebSocket (설치 로그) | O | O | O |

### 4.5 Secret Delivery 정책 (신규)

- `POST /api/v1/stacks/:id/deploy` 및 retry 계열 엔드포인트는 OpenBao 연계 상태를 필수 검증한다.
- OpenBao 미연결/권한 오류 시 배포를 차단하고, 재시도 가능한 표준 에러코드를 반환한다.
- 시크릿 회전은 배포 파이프라인과 분리된 운영 액션으로 수행 가능해야 하며, Audit 로그를 남긴다.

---

## 5. Rate Limiting 정책

### 5.1 기본 정책

Rate Limiting은 **토큰 버킷(Token Bucket)** 알고리즘을 사용합니다.

| 대상 | 제한 | 윈도우 | 식별자 |
|------|------|--------|--------|
| 인증된 사용자 | 300 요청 | 1분 | 세션 ID 또는 사용자 ID |
| 미인증 요청 | 30 요청 | 1분 | IP 주소 |
| 로그인 시도 | 10 요청 | 1분 | IP 주소 |
| WebSocket 연결 | 5 연결 | 동시 | 사용자 ID |
| 설치/배포 요청 | 10 요청 | 1시간 | Organization ID |

### 5.2 응답 헤더

```
HTTP/1.1 200 OK
X-RateLimit-Limit: 300
X-RateLimit-Remaining: 247
X-RateLimit-Reset: 1711000860
```

### 5.3 제한 초과 응답

```json
HTTP/1.1 429 Too Many Requests
Retry-After: 32

{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "http_status": 429,
    "message": "요청 제한을 초과했습니다. 32초 후 재시도하세요.",
    "detail": "현재 윈도우에서 300/300 요청 사용",
    "retryable": true,
    "trace_id": "tr_rate_001"
  }
}
```

---

## 6. 모듈별 엔드포인트

---

### 6.1 Auth 모듈

인증/인가를 담당합니다. Alpha/Beta는 세션 기반, v1에서 Keycloak OIDC로 전환합니다.

#### POST /api/v1/auth/login

세션 기반 로그인 (Alpha/Beta)

- **릴리스**: Alpha
- **인증**: 불필요
- **Rate Limit**: 10회/분 (IP 기준)

**요청**:
```json
{
  "email": "admin@nullus.io",
  "password": "securepassword123"
}
```

**응답** (`200 OK`):
```json
{
  "data": {
    "user": {
      "id": "usr_a1b2c3",
      "email": "admin@nullus.io",
      "display_name": "관리자",
      "role": "admin",
      "org_id": "org_x1y2z3"
    }
  }
}
```
```
Set-Cookie: nullus_session=s%3A...; Path=/; HttpOnly; Secure; SameSite=Strict; Max-Age=86400
```

**에러**:
| 코드 | 상황 |
|------|------|
| `AUTH_LOGIN_INVALID_CREDENTIALS` (401) | 이메일 또는 비밀번호 불일치 |
| `AUTH_LOGIN_ACCOUNT_LOCKED` (423) | 5회 연속 실패로 계정 잠금 |

---

#### POST /api/v1/auth/logout

세션 종료

- **릴리스**: Alpha
- **인증**: 세션 필요

**응답** (`204 No Content`):
```
Set-Cookie: nullus_session=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT
```

---

#### GET /api/v1/auth/me

현재 로그인 사용자 정보 및 권한 조회

- **릴리스**: Alpha
- **인증**: 필요

**응답** (`200 OK`):
```json
{
  "data": {
    "id": "usr_a1b2c3",
    "email": "admin@nullus.io",
    "display_name": "관리자",
    "role": "admin",
    "org": {
      "id": "org_x1y2z3",
      "name": "Nullus 팀",
      "slug": "nullus-team"
    },
    "permissions": [
      "org:read", "org:write",
      "cluster:read", "cluster:write",
      "stack:read", "stack:write", "stack:deploy",
      "pipeline:read", "pipeline:write", "pipeline:deploy",
      "monitoring:read",
      "alert:write",
      "user:read", "user:write"
    ]
  }
}
```

---

#### POST /api/v1/auth/token/refresh

Keycloak 리프레시 토큰으로 액세스 토큰 갱신 (v1)

- **릴리스**: v1
- **인증**: Refresh Token 필요

**요청**:
```json
{
  "refresh_token": "eyJhbGciOiJSUzI1NiIs..."
}
```

**응답** (`200 OK`):
```json
{
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIs...",
    "token_type": "Bearer",
    "expires_in": 900,
    "refresh_token": "eyJhbGciOiJSUzI1NiIs..."
  }
}
```

---

### 6.2 Organization 모듈

Organization (조직) 관리를 담당합니다.

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/api/v1/admin/organization` | 현재 Organization 조회 |
| PATCH | `/api/v1/admin/organization` | Organization 수정 |
| POST | `/api/v1/admin/orgs` | Organization 생성 |
| GET | `/api/v1/admin/known-issues` | Known Issues 목록 조회 |
| GET | `/api/v1/admin/audit-logs` | 감사 로그 조회 |
| GET | `/api/v1/admin/notifications/configs` | 알림 설정 목록 조회 |
| POST | `/api/v1/admin/notifications/configs` | 알림 설정 생성 |
| DELETE | `/api/v1/admin/notifications/configs/:id` | 알림 설정 삭제 |
| GET | `/api/v1/admin/notifications/history` | 알림 이력 조회 |
| GET | `/api/v1/admin/organizations/:orgId/members` | 멤버 목록 조회 |
| POST | `/api/v1/admin/organizations/:orgId/members` | 멤버 초대 |
| DELETE | `/api/v1/admin/organizations/:orgId/members/:id` | 멤버 제거 |
| PATCH | `/api/v1/admin/organizations/:orgId/members/:id` | 멤버 역할 변경 |

---

### 6.3 Cluster 모듈

Kubernetes 클러스터 등록 및 관리를 담당합니다.

#### POST /api/v1/admin/clusters

클러스터 등록 (kubeconfig 업로드)

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**요청** (`multipart/form-data`):
```
kubeconfig: (파일 업로드)
name: production-gke
type: pipeline
context: gke_project_zone_cluster
```

| 필드 | 타입 | 필수 | 검증 규칙 |
|------|------|------|-----------|
| `kubeconfig` | `file` | O | 유효한 kubeconfig YAML |
| `name` | `string` | O | 1-100자, Organization 내 UNIQUE |
| `type` | `string` | O | `pipeline` (도구 설치용) 또는 `target` (앱 배포용) |
| `context` | `string` | X | kubeconfig 내 context 이름 (미지정 시 current-context 사용) |

**처리 흐름**:
1. kubeconfig 파일 파싱, context 추출
2. AES-256-GCM 암호화 후 DB 저장
3. 클러스터 이름 유니크 검증
4. 상태 `pending`으로 저장

**응답** (`201 Created`):
```json
{
  "data": {
    "id": "cls_abc123",
    "org_id": "org_x1y2z3",
    "name": "production-gke",
    "type": "pipeline",
    "endpoint": "https://35.x.x.x:6443",
    "kubernetes_version": null,
    "status": "pending",
    "namespace_count": null,
    "last_verified_at": null,
    "created_at": "2026-03-14T09:30:00Z",
    "updated_at": "2026-03-14T09:30:00Z"
  }
}
```

---

#### GET /api/v1/admin/clusters

클러스터 목록 조회

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer
- **페이지네이션**: 지원
- **필터**: `status` (`connected`, `pending`, `unreachable`, `auth_failed`), `type` (`pipeline`, `target`)
- **정렬**: `name`, `status`, `created_at`, `-created_at`

**응답** (`200 OK`):
```json
{
  "data": [
    {
      "id": "cls_abc123",
      "name": "production-gke",
      "type": "pipeline",
      "endpoint": "https://35.x.x.x:6443",
      "kubernetes_version": "1.28.5",
      "status": "connected",
      "namespace_count": 12,
      "last_verified_at": "2026-03-14T09:35:00Z",
      "created_at": "2026-03-14T09:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total_items": 3,
    "total_pages": 1,
    "has_next": false,
    "has_prev": false
  }
}
```

---

#### GET /api/v1/admin/clusters/:id

클러스터 상세 조회

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**응답** (`200 OK`):
```json
{
  "data": {
    "id": "cls_abc123",
    "org_id": "org_x1y2z3",
    "name": "production-gke",
    "type": "pipeline",
    "endpoint": "https://35.x.x.x:6443",
    "kubernetes_version": "1.28.5",
    "auth_method": "kubeconfig",
    "status": "connected",
    "namespace_count": 12,
    "namespaces": ["default", "kube-system", "nullus-system"],
    "last_verified_at": "2026-03-14T09:35:00Z",
    "created_at": "2026-03-14T09:30:00Z",
    "updated_at": "2026-03-14T09:35:00Z"
  }
}
```

---

#### PATCH /api/v1/admin/clusters/:id

클러스터 정보 수정

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**요청**:
```json
{
  "name": "production-gke-main",
  "type": "target"
}
```

> kubeconfig 변경 시 파일 재업로드 필요 (별도 `multipart/form-data` 요청)

**응답** (`200 OK`): 수정된 클러스터 객체 반환

---

#### DELETE /api/v1/admin/clusters/:id

클러스터 삭제

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin

**응답** (`204 No Content`)

**에러**:
| 코드 | 상황 |
|------|------|
| `CLUSTER_DELETE_HAS_STACKS` (409) | 해당 클러스터에 배포된 스택이 존재 |

---

#### POST /api/v1/admin/clusters/:id/verify

클러스터 연결 검증

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**처리 흐름**:
1. DB에서 kubeconfig 복호화 (메모리에서만 처리)
2. `kubectl version` 실행
3. K8s 버전 확인, API 접근 가능 여부 판단
4. 상태 업데이트 (`connected` / `unreachable` / `auth_failed`)
5. 연결 성공 시 네임스페이스 목록 캐시

**응답** (`200 OK`):
```json
{
  "data": {
    "cluster_id": "cls_abc123",
    "status": "connected",
    "kubernetes_version": "1.28.5",
    "node_count": 3,
    "node_architectures": ["amd64"],
    "namespace_count": 12,
    "verified_at": "2026-03-14T09:35:00Z"
  }
}
```

**에러**:
| 코드 | 상황 |
|------|------|
| `CLUSTER_VERIFY_UNREACHABLE` (422) | 네트워크 연결 불가 |
| `CLUSTER_VERIFY_AUTH_FAILED` (422) | kubeconfig 인증 실패 |

---

#### GET /api/v1/admin/clusters/:id/namespaces

클러스터 네임스페이스 목록 조회

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**응답** (`200 OK`):
```json
{
  "data": [
    { "name": "default", "status": "Active" },
    { "name": "kube-system", "status": "Active" },
    { "name": "nullus-system", "status": "Active" }
  ]
}
```

---

### 6.4 Stack 모듈

DevSecOps Stack 설정(노코드 UI 5단계 워크플로우) 관리를 담당합니다.

#### POST /api/v1/stacks

스택 설정 생성

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**요청**:
```json
{
  "name": "프로덕션 DevOps 스택",
  "cluster_id": "cls_abc123",
  "golden_path_id": "gitlab-allinone-v1",
  "config": {
    "artifacts": {
      "package_registry": { "tool": "gitlab", "version": "17.7.2" },
      "source_repository": { "tool": "gitlab", "version": "17.7.2" },
      "container_registry": { "tool": "gitlab-registry", "version": "17.7.2" },
      "storage_backend": { "tool": "minio", "version": "2024.11.7" }
    },
    "pipeline": {
      "ci_platform": { "tool": "gitlab-ci", "version": "17.7.2" },
      "cd_tool": { "tool": "argocd", "version": "2.13.2" }
    },
    "monitoring": {
      "collection": { "tool": "prometheus", "version": "3.1.0" },
      "visualization": { "tool": "grafana", "version": "11.4.0" }
    },
    "logging": {
      "collection": { "tool": "opentelemetry", "version": "0.115.0" },
      "search": { "tool": "opensearch", "version": "2.18.0" }
    },
    "resources": {
      "developers": 20,
      "concurrent_runners": 5,
      "weekly_commits": 100,
      "build_frequency": "hourly"
    }
  }
}
```

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `name` | `string` | O | 스택 이름 (1-200자) |
| `cluster_id` | `string` | O | 대상 클러스터 ID |
| `golden_path_id` | `string` | X | Golden Path 템플릿 ID (선택 시 config 자동 채움) |
| `config` | `StackConfigSpec` | O | 5단계 워크플로우 설정 전체 |
| `config.artifacts` | `ArtifactsConfig` | O | Step 1: Artifacts 설정 |
| `config.pipeline` | `PipelineConfig` | O | Step 2: Pipeline Tools 설정 |
| `config.monitoring` | `MonitoringConfig` | O | Step 3: Monitoring Tools 설정 |
| `config.logging` | `LoggingConfig` | O | Step 4: Logging Tools 설정 |
| `config.resources` | `ResourcesConfig` | O | Step 5: Resources 설정 |

**응답** (`201 Created`):
```json
{
  "data": {
    "id": "stk_m1n2o3",
    "name": "프로덕션 DevOps 스택",
    "cluster_id": "cls_abc123",
    "org_id": "org_x1y2z3",
    "golden_path_id": "gitlab-allinone-v1",
    "config": { "..." },
    "status": "configured",
    "current_version": 1,
    "created_at": "2026-03-14T10:00:00Z",
    "updated_at": "2026-03-14T10:00:00Z"
  }
}
```

---

#### GET /api/v1/stacks

스택 설정 목록 조회

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer
- **페이지네이션**: 지원
- **필터**: `status` (`configured`, `deploying`, `deployed`, `failed`), `cluster_id`
- **정렬**: `name`, `status`, `created_at`, `updated_at`

**응답** (`200 OK`):
```json
{
  "data": [
    {
      "id": "stk_m1n2o3",
      "name": "프로덕션 DevOps 스택",
      "cluster_id": "cls_abc123",
      "golden_path_id": "gitlab-allinone-v1",
      "status": "deployed",
      "current_version": 3,
      "tool_summary": {
        "source_repository": "GitLab CE 17.7.2",
        "cd_tool": "Argo CD 2.13.2",
        "monitoring": "Prometheus 3.1.0 + Grafana 11.4.0"
      },
      "created_at": "2026-03-14T10:00:00Z",
      "updated_at": "2026-03-20T14:30:00Z"
    }
  ],
  "pagination": { "..." }
}
```

---

#### GET /api/v1/stacks/:id

스택 설정 상세 조회

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**응답** (`200 OK`): 스택 설정 전체 (config JSONB 포함) 반환

---

#### PUT /api/v1/stacks/:id

스택 설정 수정

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**요청**: 변경할 필드만 포함 (PATCH 시맨틱)

```json
{
  "config": {
    "pipeline": {
      "cd_tool": { "tool": "argocd", "version": "2.14.0" }
    }
  }
}
```

**응답** (`200 OK`): 수정된 스택 설정 반환 (version 자동 증가)

---

#### GET /api/v1/stacks/:id/history

스택 설정 변경 이력 조회

- **릴리스**: v1
- **인증**: 필요
- **권한**: Admin, DevOps Engineer
- **페이지네이션**: 지원
- **정렬**: `-created_at` (기본: 최신순)

**응답** (`200 OK`):
```json
{
  "data": [
    {
      "version_id": "ver_p1q2r3",
      "version_number": 3,
      "change_reason": "Argo CD 보안 패치 적용",
      "changed_by": {
        "id": "usr_a1b2c3",
        "display_name": "김민수"
      },
      "created_at": "2026-05-20T14:30:00Z"
    },
    {
      "version_id": "ver_s4t5u6",
      "version_number": 2,
      "change_reason": "러너 동시 실행 수 증가",
      "changed_by": {
        "id": "usr_d4e5f6",
        "display_name": "이미정"
      },
      "created_at": "2026-05-15T09:15:00Z"
    }
  ],
  "pagination": { "..." }
}
```

---

#### GET /api/v1/stacks/:id/history/:versionId/diff

특정 버전과 이전 버전 간 diff 조회

- **릴리스**: v1
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**응답** (`200 OK`):
```json
{
  "data": {
    "version_from": 2,
    "version_to": 3,
    "changes": [
      {
        "path": "config.pipeline.cd_tool.version",
        "old_value": "2.13.2",
        "new_value": "2.14.0"
      }
    ],
    "diff_text": "--- v2\n+++ v3\n@@ pipeline.cd_tool @@\n- version: 2.13.2\n+ version: 2.14.0"
  }
}
```

---

#### POST /api/v1/stacks/:id/rollback/:versionId

특정 버전으로 스택 설정 롤백

- **릴리스**: v1
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**요청**:
```json
{
  "reason": "v3 적용 후 Argo CD 동기화 오류 발생"
}
```

**응답** (`200 OK`): 롤백된 스택 설정 반환 (새로운 version으로 저장)

---

### 6.5 Template 모듈

Golden Path 템플릿 및 CI/CD 파이프라인 템플릿을 조회합니다.

#### GET /api/v1/stacks/templates

Golden Path 목록 조회

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**응답** (`200 OK`):
```json
{
  "data": [
    {
      "id": "gitlab-allinone-v1",
      "name": "GitLab All-in-One",
      "description": "GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.",
      "tools": [
        { "category": "source_repository", "name": "GitLab CE", "version": "17.7.2" },
        { "category": "ci_platform", "name": "GitLab CI", "version": "17.7.2" },
        { "category": "container_registry", "name": "GitLab Registry", "version": "17.7.2" },
        { "category": "storage_backend", "name": "MinIO", "version": "2024.11.7" },
        { "category": "cd_tool", "name": "Argo CD", "version": "2.13.2" },
        { "category": "monitoring_collection", "name": "Prometheus", "version": "3.1.0" },
        { "category": "monitoring_visualization", "name": "Grafana", "version": "11.4.0" }
      ],
      "target_audience": "중견기업, 단일 플랫폼 선호",
      "estimated_install_time_minutes": 90,
      "resource_baseline": {
        "cpu_cores": 8,
        "memory_gi": 16,
        "storage_gi": 100
      },
      "status": "verified",
      "release": "Alpha"
    },
    {
      "id": "gitlab-argocd-v1",
      "name": "GitLab + Argo CD",
      "description": "GitLab CI와 Harbor 레지스트리를 분리하여 GitOps 패턴을 강화한 구성입니다.",
      "tools": [
        { "category": "source_repository", "name": "GitLab CE", "version": "17.7.2" },
        { "category": "ci_platform", "name": "GitLab CI", "version": "17.7.2" },
        { "category": "container_registry", "name": "Harbor", "version": "2.11.0" },
        { "category": "storage_backend", "name": "MinIO", "version": "2024.11.7" },
        { "category": "cd_tool", "name": "Argo CD", "version": "2.13.2" },
        { "category": "monitoring_collection", "name": "Prometheus", "version": "3.1.0" },
        { "category": "monitoring_visualization", "name": "Grafana", "version": "11.4.0" }
      ],
      "target_audience": "GitOps 중심 조직",
      "estimated_install_time_minutes": 120,
      "resource_baseline": {
        "cpu_cores": 10,
        "memory_gi": 20,
        "storage_gi": 130
      },
      "status": "verified",
      "release": "Beta"
    },
    {
      "id": "github-argocd-v1",
      "name": "GitHub + Argo CD",
      "description": "GitHub와 GitHub Actions를 외부 서비스로 사용하고, 클러스터 내에는 Harbor + Argo CD + 모니터링만 설치합니다.",
      "tools": [
        { "category": "source_repository", "name": "GitHub", "version": "external" },
        { "category": "ci_platform", "name": "GitHub Actions", "version": "external" },
        { "category": "container_registry", "name": "Harbor", "version": "2.11.0" },
        { "category": "storage_backend", "name": "MinIO", "version": "2024.11.7" },
        { "category": "cd_tool", "name": "Argo CD", "version": "2.13.2" },
        { "category": "monitoring_collection", "name": "Prometheus", "version": "3.1.0" },
        { "category": "monitoring_visualization", "name": "Grafana", "version": "11.4.0" }
      ],
      "target_audience": "GitHub 사용 조직",
      "estimated_install_time_minutes": 60,
      "resource_baseline": {
        "cpu_cores": 6,
        "memory_gi": 12,
        "storage_gi": 80
      },
      "status": "verified",
      "release": "v1"
    }
  ]
}
```

---

#### GET /api/v1/stacks/templates/:id

Golden Path 상세 조회

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**응답** (`200 OK`): 단일 Golden Path 객체 (위 목록의 개별 항목과 동일 구조 + `config_preset` 필드 추가)

```json
{
  "data": {
    "id": "gitlab-allinone-v1",
    "name": "GitLab All-in-One",
    "...": "...",
    "config_preset": {
      "artifacts": {
        "package_registry": { "tool": "gitlab", "version": "17.7.2" },
        "source_repository": { "tool": "gitlab", "version": "17.7.2" },
        "container_registry": { "tool": "gitlab-registry", "version": "17.7.2" },
        "storage_backend": { "tool": "minio", "version": "2024.11.7" }
      },
      "pipeline": {
        "ci_platform": { "tool": "gitlab-ci", "version": "17.7.2" },
        "cd_tool": { "tool": "argocd", "version": "2.13.2" }
      },
      "monitoring": {
        "collection": { "tool": "prometheus", "version": "3.1.0" },
        "visualization": { "tool": "grafana", "version": "11.4.0" }
      },
      "logging": {
        "collection": { "tool": "opentelemetry", "version": "0.115.0" },
        "search": { "tool": "opensearch", "version": "2.18.0" }
      }
    }
  }
}
```

---

#### GET /api/v1/cicd/templates

CI/CD 파이프라인 템플릿 목록 조회

- **릴리스**: Beta
- **인증**: 필요
- **권한**: Admin, DevOps Engineer, Developer
- **필터**: `category` (`backend`, `frontend`, `batch`)

**응답** (`200 OK`):
```json
{
  "data": [
    {
      "id": "web-backend-v1",
      "name": "Web Backend",
      "description": "Spring Boot, Express, Django 등 백엔드 애플리케이션용 파이프라인",
      "category": "backend",
      "stages": ["Build", "Test", "Image Build", "Deploy"],
      "supported_frameworks": ["Spring Boot", "Express", "Django", "FastAPI"],
      "parameters": [
        { "name": "repo_url", "type": "string", "required": true, "description": "Git Repository URL" },
        { "name": "image_name", "type": "string", "required": true, "description": "컨테이너 이미지 이름" },
        { "name": "target_namespace", "type": "string", "required": true, "description": "배포 네임스페이스" },
        { "name": "replicas", "type": "integer", "required": false, "default": 2, "description": "Pod 복제본 수" },
        { "name": "env_vars", "type": "object", "required": false, "description": "환경 변수 (Key-Value)" }
      ],
      "release": "Beta"
    },
    {
      "id": "web-frontend-v1",
      "name": "Web Frontend",
      "description": "React, Vue, Next.js 등 프론트엔드 애플리케이션용 파이프라인",
      "category": "frontend",
      "stages": ["Build", "Test", "Static Build", "Deploy (Nginx)"],
      "supported_frameworks": ["React", "Vue", "Next.js", "Nuxt"],
      "parameters": [
        { "name": "repo_url", "type": "string", "required": true, "description": "Git Repository URL" },
        { "name": "build_command", "type": "string", "required": false, "default": "npm run build", "description": "빌드 명령어" },
        { "name": "target_namespace", "type": "string", "required": true, "description": "배포 네임스페이스" }
      ],
      "release": "v1"
    },
    {
      "id": "batch-job-v1",
      "name": "Batch Job",
      "description": "크론 작업, 데이터 처리 등 배치 작업용 파이프라인",
      "category": "batch",
      "stages": ["Build", "Image Build", "CronJob Deploy"],
      "supported_frameworks": ["Python", "Java", "Go"],
      "parameters": [
        { "name": "repo_url", "type": "string", "required": true, "description": "Git Repository URL" },
        { "name": "schedule", "type": "string", "required": true, "description": "Cron 표현식 (예: '0 2 * * *')" },
        { "name": "target_namespace", "type": "string", "required": true, "description": "배포 네임스페이스" }
      ],
      "release": "v1"
    }
  ]
}
```

---

#### GET /api/v1/cicd/templates/:id

CI/CD 파이프라인 템플릿 상세 조회

- **릴리스**: Beta
- **인증**: 필요
- **권한**: Admin, DevOps Engineer, Developer

**응답** (`200 OK`): 단일 파이프라인 템플릿 객체 반환

---

### 6.6 Installation (배포) 모듈

DevSecOps Stack 설치/배포를 관리합니다. Install Engine (Orchestrator, State Machine, Step Runner, Rollback Manager, Log Streamer)과 연동됩니다.

#### POST /api/v1/installations

스택 설치(배포) 시작

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer
- **Rate Limit**: 10회/시간 (Organization 기준)

**요청**:
```json
{
  "stack_id": "stk_m1n2o3",
  "rollback_mode": "safe"
}
```

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `stack_id` | `string` | O | 배포할 스택 설정 ID |
| `rollback_mode` | `string` | X | `safe` (기본, PVC 보존) 또는 `destructive` (PVC 포함 삭제) |

**전제 조건**:
- 대상 클러스터 상태가 `connected`여야 함
- 동일 스택에 `INSTALLING` 상태 배포가 없어야 함

**응답** (`201 Created`):
```json
{
  "data": {
    "id": "dep_g1h2i3",
    "stack_id": "stk_m1n2o3",
    "cluster_id": "cls_abc123",
    "type": "stack",
    "status": "PENDING",
    "rollback_mode": "safe",
    "steps": [
      { "name": "cert-manager", "phase": "A", "status": "pending", "order": 1 },
      { "name": "cnpg-postgresql", "phase": "A", "status": "pending", "order": 2 },
      { "name": "minio", "phase": "A", "status": "pending", "order": 3 },
      { "name": "gitlab-ce", "phase": "B", "status": "pending", "order": 4 },
      { "name": "gitlab-registry", "phase": "B", "status": "pending", "order": 5 },
      { "name": "argocd", "phase": "B", "status": "pending", "order": 6 },
      { "name": "prometheus-grafana", "phase": "C", "status": "pending", "order": 7 },
      { "name": "otel-opensearch", "phase": "C", "status": "pending", "order": 8 },
      { "name": "integration", "phase": "C", "status": "pending", "order": 9 }
    ],
    "started_by": "usr_a1b2c3",
    "started_at": "2026-03-14T10:30:00Z",
    "completed_at": null,
    "websocket_url": "wss://nullus.example.com/ws/deployments/dep_g1h2i3/logs"
  }
}
```

**에러**:
| 코드 | 상황 |
|------|------|
| `STACK_DEPLOY_CLUSTER_NOT_READY` (422) | 클러스터 상태가 `connected`가 아님 |
| `STACK_DEPLOY_ALREADY_RUNNING` (409) | 동일 스택에 이미 진행 중인 설치 존재 |

#### 상태 머신 (State Machine)

```
PENDING → VALIDATING → INSTALLING → CONFIGURING → HEALTHCHECK → COMPLETED
                            │             │
                            ▼             ▼
                         FAILED ←─── (실패 시)
                            │
                       ┌────┴────┐
                       ▼         ▼
                   RETRYING  ROLLING_BACK → ROLLED_BACK
```

#### 설치 순서 (3-Phase DAG)

```
Phase A (인프라 기반):
  Step 1: cert-manager → Step 2: CNPG (PostgreSQL) → Step 3: MinIO

Phase B (핵심 서비스, Phase A 완료 후):
  Step 4: GitLab CE → Step 5: Container Registry → Step 6: Argo CD

Phase C (보조 서비스, Phase B 완료 후):
  Step 7: Prometheus + Grafana → Step 8: OTel + OpenSearch

Step 9: Integration (모든 Phase 완료 후)
  - GitLab ↔ Argo CD Webhook
  - Prometheus ServiceMonitor
  - Grafana Dashboard Provisioning
  - GitLab Runner ↔ Container Registry Auth
```

---

#### GET /api/v1/installations/:id/status

설치 상태 조회

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**응답** (`200 OK`):
```json
{
  "data": {
    "id": "dep_g1h2i3",
    "status": "INSTALLING",
    "progress_percent": 37,
    "current_step": "gitlab-ci-runner",
    "current_phase": "B",
    "steps": [
      { "name": "cert-manager", "phase": "A", "status": "completed", "duration_seconds": 45 },
      { "name": "cnpg-postgresql", "phase": "A", "status": "completed", "duration_seconds": 120 },
      { "name": "minio", "phase": "A", "status": "completed", "duration_seconds": 150 },
      { "name": "gitlab-ce", "phase": "B", "status": "completed", "duration_seconds": 735 },
      { "name": "gitlab-registry", "phase": "B", "status": "completed", "duration_seconds": 105 },
      { "name": "gitlab-ci-runner", "phase": "B", "status": "installing", "duration_seconds": null },
      { "name": "argocd", "phase": "B", "status": "pending", "duration_seconds": null },
      { "name": "prometheus-grafana", "phase": "C", "status": "pending", "duration_seconds": null },
      { "name": "otel-opensearch", "phase": "C", "status": "pending", "duration_seconds": null }
    ],
    "started_at": "2026-03-14T10:30:00Z",
    "elapsed_seconds": 1155,
    "estimated_remaining_seconds": 2400
  }
}
```

---

#### DELETE /api/v1/installations/:id

설치 취소 (진행 중인 설치 중단)

- **릴리스**: Beta
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**응답** (`200 OK`):
```json
{
  "data": {
    "id": "dep_g1h2i3",
    "status": "CANCELLED",
    "cancelled_at": "2026-03-14T11:00:00Z"
  }
}
```

---

#### POST /api/v1/installations/:id/retry

실패 단계부터 재시도

- **릴리스**: Beta
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**응답** (`200 OK`): 새로운 배포 상태 객체 반환 (상태: `RETRYING`)

---

#### POST /api/v1/installations/:id/rollback

설치 롤백

- **릴리스**: Beta
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**요청**:
```json
{
  "mode": "safe"
}
```

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `mode` | `string` | X | `safe` (기본, PVC 보존) 또는 `destructive` (PVC 포함 삭제, 명시적 확인 필요) |

**응답** (`200 OK`):
```json
{
  "data": {
    "id": "dep_g1h2i3",
    "status": "ROLLING_BACK",
    "rollback_mode": "safe",
    "rollback_steps": [
      { "name": "gitlab-ci-runner", "status": "rolling_back" },
      { "name": "gitlab-registry", "status": "pending" },
      { "name": "gitlab-ce", "status": "pending" },
      { "name": "minio", "status": "pending" }
    ]
  }
}
```

#### 롤백 전략 (릴리스별)

| 릴리스 | 모드 | 설명 |
|--------|------|------|
| Alpha | FULL | 전체 롤백만 지원 (설치된 모든 컴포넌트 제거 후 재설치) |
| Beta | FULL + RETRY | 전체 롤백 + 실패 단계 재시도 |
| v1 | FULL + PARTIAL + RETRY | 부분 롤백 지원 (실패 컴포넌트만 선택적 롤백) |

---

#### GET /api/v1/installations/:id/logs

설치 로그 조회 (HTTP 기반, 과거 로그)

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer
- **필터**: `step` (특정 단계 이름), `level` (`info`, `warn`, `error`)

**응답** (`200 OK`):
```json
{
  "data": [
    {
      "timestamp": "2026-03-14T10:30:05Z",
      "step": "cert-manager",
      "level": "info",
      "message": "Installing cert-manager helm chart v1.14.0..."
    },
    {
      "timestamp": "2026-03-14T10:30:45Z",
      "step": "cert-manager",
      "level": "info",
      "message": "cert-manager pods are ready (3/3)"
    },
    {
      "timestamp": "2026-03-14T10:47:12Z",
      "step": "gitlab-ci-runner",
      "level": "error",
      "message": "Helm install timeout after 300s: gitlab-runner pod CrashLoopBackOff"
    }
  ],
  "pagination": { "..." }
}
```

---

### 6.7 Pipeline (CI/CD) 모듈

CI/CD 파이프라인 생성, 배포, 이력 관리를 담당합니다.

#### POST /api/v1/cicd/pipelines

파이프라인 생성

- **릴리스**: Beta
- **인증**: 필요
- **권한**: Admin, DevOps Engineer, Developer

**요청**:
```json
{
  "name": "user-service",
  "template_id": "web-backend-v1",
  "stack_id": "stk_m1n2o3",
  "params": {
    "repo_url": "https://github.com/org/user-service.git",
    "image_name": "user-service",
    "target_namespace": "app-user-service",
    "target_cluster_id": "cls_abc123",
    "replicas": 3,
    "cpu_request": "500m",
    "memory_request": "512Mi",
    "env_vars": {
      "DATABASE_URL": "postgresql://...",
      "LOG_LEVEL": "info"
    }
  }
}
```

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `name` | `string` | O | 파이프라인 이름 (1-200자) |
| `template_id` | `string` | O | CI/CD 템플릿 ID |
| `stack_id` | `string` | O | 연결할 DevSecOps 스택 ID |
| `params` | `object` | O | 템플릿 파라미터 (템플릿별 상이) |

**응답** (`201 Created`):
```json
{
  "data": {
    "id": "pipe_j1k2l3",
    "name": "user-service",
    "template_id": "web-backend-v1",
    "stack_id": "stk_m1n2o3",
    "params": { "..." },
    "status": "created",
    "deployment_count": 0,
    "created_at": "2026-04-01T09:00:00Z",
    "updated_at": "2026-04-01T09:00:00Z"
  }
}
```

---

#### GET /api/v1/cicd/pipelines

파이프라인 목록 조회

- **릴리스**: Beta
- **인증**: 필요
- **권한**: Admin, DevOps Engineer, Developer
- **페이지네이션**: 지원
- **필터**: `status` (`created`, `running`, `success`, `failed`), `template_id`, `stack_id`
- **정렬**: `name`, `status`, `created_at`, `updated_at`

**응답** (`200 OK`): 파이프라인 목록 + 페이지네이션

---

#### GET /api/v1/cicd/pipelines/:id

파이프라인 상세 조회

- **릴리스**: Beta
- **인증**: 필요
- **권한**: Admin, DevOps Engineer, Developer

**응답** (`200 OK`): 파이프라인 전체 정보 반환

---

#### POST /api/v1/cicd/pipelines/:id/deploy

파이프라인 배포 실행

- **릴리스**: Beta
- **인증**: 필요
- **권한**: Admin, DevOps Engineer, Developer

**요청**:
```json
{
  "change_reason": "v1.2.0 릴리스 배포"
}
```

**처리 흐름**:
1. 템플릿 + 파라미터 기반으로 K8s 매니페스트 생성
2. 필수 K8s Object 자동 생성: Namespace, Deployment, Service, Ingress, Secret, PVC, ServiceAccount
3. 배포 실행 및 이력 저장

**응답** (`201 Created`):
```json
{
  "data": {
    "deployment_id": "pdep_v1w2x3",
    "pipeline_id": "pipe_j1k2l3",
    "version": 1,
    "status": "deploying",
    "k8s_objects": [
      { "kind": "Namespace", "name": "app-user-service", "status": "created" },
      { "kind": "Deployment", "name": "user-service-deployment", "status": "creating" },
      { "kind": "Service", "name": "user-service-service", "status": "creating" },
      { "kind": "Ingress", "name": "user-service-ingress", "status": "creating" },
      { "kind": "Secret", "name": "user-service-registry-secret", "status": "created" },
      { "kind": "ServiceAccount", "name": "user-service-sa", "status": "created" }
    ],
    "deployed_by": "usr_d4e5f6",
    "deployed_at": "2026-04-01T10:00:00Z"
  }
}
```

---

#### GET /api/v1/cicd/deployments

파이프라인 배포 이력 조회

- **릴리스**: Beta
- **인증**: 필요
- **권한**: Admin, DevOps Engineer, Developer
- **페이지네이션**: 지원
- **필터**: `status` (`success`, `failed`, `deploying`, `rolled_back`)
- **정렬**: `-deployed_at` (기본: 최신순)

**응답** (`200 OK`):
```json
{
  "data": [
    {
      "deployment_id": "pdep_v1w2x3",
      "version": 3,
      "status": "success",
      "change_reason": "v1.2.0 릴리스 배포",
      "deployed_by": {
        "id": "usr_d4e5f6",
        "display_name": "김민수"
      },
      "deployed_at": "2026-04-15T10:00:00Z",
      "completed_at": "2026-04-15T10:02:30Z"
    }
  ],
  "pagination": { "..." }
}
```

---

#### GET /api/v1/cicd/deployments/:did

배포 상세 조회 (생성된 K8s 오브젝트 포함)

- **릴리스**: v1
- **인증**: 필요
- **권한**: Admin, DevOps Engineer, Developer

**응답** (`200 OK`):
```json
{
  "data": {
    "deployment_id": "pdep_v1w2x3",
    "version": 3,
    "status": "success",
    "k8s_objects": [
      {
        "kind": "Deployment",
        "name": "user-service-deployment",
        "namespace": "app-user-service",
        "status": "Available",
        "replicas": { "desired": 3, "ready": 3 }
      },
      {
        "kind": "Service",
        "name": "user-service-service",
        "namespace": "app-user-service",
        "type": "ClusterIP",
        "cluster_ip": "10.96.100.50",
        "ports": [{ "port": 80, "target_port": 8080 }]
      },
      {
        "kind": "Ingress",
        "name": "user-service-ingress",
        "namespace": "app-user-service",
        "host": "user-service.example.com"
      }
    ],
    "deployed_by": { "id": "usr_d4e5f6", "display_name": "김민수" },
    "deployed_at": "2026-04-15T10:00:00Z"
  }
}
```

---

#### POST /api/v1/cicd/pipelines/:id/rollback/:did

특정 배포 버전으로 롤백

- **릴리스**: v1
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**요청**:
```json
{
  "reason": "v1.2.0에서 메모리 누수 발견, v1.1.0으로 롤백"
}
```

**응답** (`200 OK`): 새로운 배포 이력 객체 반환 (롤백된 버전의 설정으로 배포)

---

#### GET /api/v1/cicd/deployments/:did/diff

이전 버전과의 diff 조회

- **릴리스**: v1
- **인증**: 필요
- **권한**: Admin, DevOps Engineer, Developer

**응답** (`200 OK`):
```json
{
  "data": {
    "version_from": 2,
    "version_to": 3,
    "changes": [
      {
        "path": "params.replicas",
        "old_value": 2,
        "new_value": 3
      },
      {
        "path": "params.env_vars.LOG_LEVEL",
        "old_value": "debug",
        "new_value": "info"
      }
    ],
    "k8s_manifest_diff": "--- v2\n+++ v3\n@@ deployment.spec.replicas @@\n- replicas: 2\n+ replicas: 3"
  }
}
```

---

### 6.8 Monitoring (Observability) 모듈

클러스터, 도구, 파이프라인 모니터링 및 알림 설정을 담당합니다.

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/api/v1/observability/dashboard` | 모니터링 대시보드 조회 |
| GET | `/api/v1/observability/alert-rules` | 알림 규칙 목록 조회 |
| POST | `/api/v1/observability/alert-rules` | 알림 규칙 생성 |
| PATCH | `/api/v1/observability/alert-rules/:id` | 알림 규칙 수정 |
| DELETE | `/api/v1/observability/alert-rules/:id` | 알림 규칙 삭제 |
| GET | `/api/v1/observability/alert-history` | 알림 이력 조회 |

---

### 6.9 Compatibility 모듈

DevSecOps Stack OSS 버전 호환성 검증을 담당합니다.

#### GET /api/v1/stacks/compatibility

호환성 매트릭스 전체 조회

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**응답** (`200 OK`):
```json
{
  "data": [
    {
      "id": "gitlab-allinone-v1",
      "name": "GitLab All-in-One",
      "status": "verified",
      "tested_at": "2026-03-15T00:00:00Z",
      "kubernetes": {
        "min": "1.26",
        "max": "1.30",
        "recommended": "1.28"
      },
      "tools": {
        "source_repository": {
          "name": "gitlab-ce",
          "app_version": "17.7.x",
          "helm_chart": "gitlab/gitlab",
          "helm_version": "8.7.x"
        },
        "ci_platform": {
          "name": "gitlab-ci",
          "app_version": "17.7.x"
        },
        "cd_tool": {
          "name": "argocd",
          "app_version": "2.13.x",
          "helm_chart": "argo/argo-cd",
          "helm_version": "7.7.x"
        },
        "monitoring_collection": {
          "name": "prometheus",
          "app_version": "3.1.x",
          "helm_chart": "prometheus-community/kube-prometheus-stack",
          "helm_version": "67.x"
        },
        "monitoring_visualization": {
          "name": "grafana",
          "app_version": "11.4.x"
        },
        "storage_backend": {
          "name": "minio",
          "app_version": "2024.x",
          "helm_chart": "minio/minio",
          "helm_version": "5.3.x"
        }
      },
      "integration_tests": [
        "gitlab-argocd-webhook",
        "prometheus-service-discovery",
        "grafana-dashboard-provisioning"
      ],
      "resource_baseline": {
        "cpu_cores": 8,
        "memory_gi": 16,
        "storage_gi": 100
      }
    }
  ]
}
```

---

#### POST /api/v1/stacks/:stackId/validate

도구 조합 호환성 검증

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**요청**:
```json
{
  "tools": [
    { "name": "gitlab-ce", "version": "17.7.2" },
    { "name": "argocd", "version": "2.13.2" },
    { "name": "prometheus", "version": "3.1.0" },
    { "name": "grafana", "version": "11.4.0" },
    { "name": "minio", "version": "2024.11.7" }
  ],
  "kubernetes_version": "1.28"
}
```

**응답 (검증 성공)** (`200 OK`):
```json
{
  "data": {
    "compatible": true,
    "matrix_id": "gitlab-allinone-v1",
    "status": "verified",
    "warnings": [],
    "recommendations": [
      {
        "tool": "argocd",
        "current": "2.13.2",
        "recommended": "2.14.0",
        "reason": "보안 패치 포함"
      }
    ]
  }
}
```

**응답 (비검증 조합)** (`200 OK`):
```json
{
  "data": {
    "compatible": false,
    "status": "untested",
    "warnings": [
      "GitLab CE 18.0 + Argo CD 2.13 조합은 테스트되지 않았습니다. 설치 시 호환성 문제가 발생할 수 있습니다."
    ],
    "closest_verified": "gitlab-allinone-v1",
    "recommendations": [
      {
        "tool": "gitlab-ce",
        "current": "18.0.0",
        "recommended": "17.7.2",
        "reason": "검증 완료된 최신 버전"
      }
    ]
  }
}
```

---

### 6.10 Resource 모듈

DevSecOps Stack 리소스 예상량 계산을 담당합니다.

#### POST /api/v1/resources/estimate

리소스 예상량 계산

- **릴리스**: Alpha
- **인증**: 필요
- **권한**: Admin, DevOps Engineer

**요청**:
```json
{
  "tools": [
    { "name": "gitlab-ce", "instances": 1 },
    { "name": "gitlab-runner", "instances": 4 },
    { "name": "argocd", "instances": 1 },
    { "name": "prometheus", "instances": 1 },
    { "name": "grafana", "instances": 1 },
    { "name": "minio", "instances": 1 },
    { "name": "opentelemetry", "instances": 1 },
    { "name": "opensearch", "instances": 1 }
  ],
  "workload": {
    "developers": 20,
    "concurrent_runners": 5,
    "weekly_commits": 100,
    "build_frequency": "hourly"
  },
  "currency": "USD"
}
```

| 필드 | 타입 | 필수 | 검증 규칙 |
|------|------|------|-----------|
| `tools` | `ToolInstance[]` | O | 도구 목록 + 인스턴스 수 |
| `workload` | `WorkloadInput` | O | 워크로드 입력값 |
| `workload.developers` | `integer` | O | 1-10000 |
| `workload.concurrent_runners` | `integer` | O | 1-100 |
| `workload.weekly_commits` | `integer` | O | 1-10000 |
| `workload.build_frequency` | `string` | O | `hourly`, `daily`, `on-push` |
| `currency` | `string` | X | `USD` (기본), `KRW`, `CNY` |

**응답** (`200 OK`):
```json
{
  "data": {
    "summary": {
      "cpu_cores": 14.5,
      "memory_gi": 33,
      "storage_gi": 180,
      "estimated_monthly_cost": {
        "amount": 187.50,
        "currency": "USD",
        "cloud_provider": "AWS",
        "breakdown": {
          "compute": 125.00,
          "storage": 18.00,
          "network": 44.50
        }
      }
    },
    "per_tool": [
      {
        "name": "GitLab CE",
        "instances": 1,
        "cpu_cores": 4.0,
        "memory_gi": 8,
        "storage_gi": 30
      },
      {
        "name": "GitLab Runner",
        "instances": 4,
        "cpu_cores": 8.0,
        "memory_gi": 16,
        "storage_gi": 40
      },
      {
        "name": "Argo CD",
        "instances": 1,
        "cpu_cores": 1.0,
        "memory_gi": 2,
        "storage_gi": 5
      },
      {
        "name": "Prometheus",
        "instances": 1,
        "cpu_cores": 1.0,
        "memory_gi": 4,
        "storage_gi": 20
      },
      {
        "name": "Grafana",
        "instances": 1,
        "cpu_cores": 0.5,
        "memory_gi": 1,
        "storage_gi": 5
      },
      {
        "name": "MinIO",
        "instances": 1,
        "cpu_cores": 0.5,
        "memory_gi": 1,
        "storage_gi": 50
      },
      {
        "name": "OpenTelemetry",
        "instances": 1,
        "cpu_cores": 0.5,
        "memory_gi": 1,
        "storage_gi": 0
      },
      {
        "name": "OpenSearch",
        "instances": 1,
        "cpu_cores": 2.0,
        "memory_gi": 4,
        "storage_gi": 30
      }
    ],
    "scaling_notes": [
      "GitLab Runner 인스턴스 4개: 기본 대비 CPU +6, Memory +12Gi 추가",
      "주간 커밋 100회 기준 Storage 20Gi 추가 (빌드 아티팩트)"
    ]
  }
}
```

**에러**:
| 코드 | 상황 |
|------|------|
| `RESOURCE_INVALID_VALUE` (400) | 입력값 범위 초과 (vCPU 0.1-1000, Memory 128MiB-1TiB, Storage 1GiB-100TiB) |

---

### 6.11 User (RBAC) 모듈

사용자 관리 및 역할 기반 접근 제어를 담당합니다 (v1에서 전체 구현).

#### GET /api/v1/users

사용자 목록 조회

- **릴리스**: v1
- **인증**: 필요
- **권한**: Admin
- **페이지네이션**: 지원
- **필터**: `role` (`admin`, `devops`, `developer`), `status` (`active`, `inactive`)
- **정렬**: `display_name`, `email`, `role`, `created_at`

**응답** (`200 OK`):
```json
{
  "data": [
    {
      "id": "usr_a1b2c3",
      "email": "admin@nullus.io",
      "display_name": "관리자",
      "role": "admin",
      "status": "active",
      "last_login_at": "2026-03-14T08:00:00Z",
      "created_at": "2026-03-01T00:00:00Z"
    }
  ],
  "pagination": { "..." }
}
```

---

### 6.12 Token Rotation (OpenBao) 모듈

OpenBao-first 시크릿 정책에 따라 OSS 토큰의 만료 전 갱신/재발급을 담당합니다.

#### GET /api/v1/admin/token-sources

토큰 소스 목록 조회

- **릴리스**: v1.x
- **인증**: Admin

지원 필터:

`status`, `module`, `provider`, `org_id`, `page`, `page_size`

#### GET /api/v1/admin/token-sources/:id/events

토큰 갱신 이력 조회

- **릴리스**: v1.x
- **인증**: Admin

#### POST /api/v1/admin/token-sources/:id/rotate

즉시 갱신(renew/reissue) 트리거

- **릴리스**: v1.x
- **인증**: Admin

요청 예시:

```json
{
  "reason": "manual-rotation",
  "force": false
}
```

#### POST /api/v1/admin/token-sources/:id/approve

수동 승인 필요 상태(`FAILED_MANUAL`)에서 갱신 재개

- **릴리스**: v1.x
- **인증**: Admin

#### POST /api/v1/admin/token-sources/:id/re-auth

고위험 조회(step-up)용 재인증

- **릴리스**: v1.x
- **인증**: Admin

요청 예시:

```json
{
  "method": "password",
  "password": "********"
}
```

또는

```json
{
  "method": "oidc_stepup",
  "challenge_token": "..."
}
```

응답 예시:

```json
{
  "step_up_token": "stepup_xxx",
  "expires_in_seconds": 300
}
```

#### POST /api/v1/admin/token-sources/:id/reveal

토큰/시크릿 조회(원문 또는 마스킹 해제)

- **릴리스**: v1.x
- **인증**: Admin + 유효한 `step_up_token`

요청 예시:

```json
{
  "step_up_token": "stepup_xxx",
  "mode": "masked" 
}
```

`mode`:
- `masked`: 일부 마스킹 값
- `full`: 정책상 허용된 경우에만 전체 표시

#### POST /api/v1/admin/token-sources/:id/pause

자동 갱신 일시 정지

- **릴리스**: v1.x
- **인증**: Admin

#### POST /api/v1/admin/token-sources/:id/resume

자동 갱신 재개

- **릴리스**: v1.x
- **인증**: Admin

#### 에러 코드 (추가)

| 코드 | HTTP | 설명 |
|---|---|---|
| `TOKEN_ROTATE_PROVIDER_UNAVAILABLE` | 503 | 외부 provider 응답 불가 |
| `TOKEN_ROTATE_RATE_LIMITED` | 429 | provider rate limit 도달 |
| `TOKEN_ROTATE_APPROVAL_REQUIRED` | 409 | 수동 승인 필요 상태 |
| `TOKEN_ROTATE_POLICY_DENIED` | 403 | OpenBao policy 권한 부족 |
| `TOKEN_ROTATE_EXPIRED` | 422 | 토큰 만료로 긴급 조치 필요 |

#### 정책 메모

- 운영/스테이징은 OpenBao를 원문 시크릿 저장소로 사용합니다.
- Kubernetes Secret은 파생 주입 리소스로만 사용합니다.
- API 응답/로그는 시크릿 원문을 반환하지 않습니다.
- `reveal` 액션은 step-up 재인증 세션에서만 허용하며 TTL(권장 5분)을 강제합니다.
- `reveal` 성공/실패는 감사 로그(`audit_logs`)에 필수 기록합니다.

#### PUT /api/v1/users/:userId/role

사용자 역할 변경

- **릴리스**: v1
- **인증**: 필요
- **권한**: Admin

**요청**:
```json
{
  "role": "devops"
}
```

| 필드 | 타입 | 필수 | 검증 규칙 |
|------|------|------|-----------|
| `role` | `string` | O | `admin`, `devops`, `developer` |

**응답** (`200 OK`): 변경된 사용자 객체 반환

---

#### DELETE /api/v1/users/:userId

사용자 비활성화

- **릴리스**: v1
- **인증**: 필요
- **권한**: Admin

**응답** (`204 No Content`)

**에러**:
| 코드 | 상황 |
|------|------|
| `USER_DELETE_SELF` (422) | 자기 자신 삭제 시도 |
| `USER_DELETE_LAST_ADMIN` (422) | 마지막 Admin 삭제 시도 |

---

## 7. WebSocket 엔드포인트

### WSS /ws/deployments/:id/logs

설치 로그 실시간 스트리밍

- **릴리스**: Alpha
- **인증**: 필요 (연결 시 세션 쿠키 또는 `token` 쿼리 파라미터)
- **동시 연결 제한**: 5개/사용자

#### 연결

```
wss://nullus.example.com/ws/deployments/dep_g1h2i3/logs?token=session_abc123
```

#### 서버 → 클라이언트 메시지 형식

```json
{
  "type": "log",
  "data": {
    "timestamp": "2026-03-14T10:47:12.345Z",
    "step": "gitlab-ce",
    "phase": "B",
    "level": "info",
    "message": "Installing GitLab CE helm chart v17.7.2..."
  }
}
```

```json
{
  "type": "status",
  "data": {
    "deployment_id": "dep_g1h2i3",
    "status": "INSTALLING",
    "progress_percent": 45,
    "current_step": "gitlab-ce",
    "current_phase": "B"
  }
}
```

```json
{
  "type": "step_complete",
  "data": {
    "step": "gitlab-ce",
    "phase": "B",
    "status": "completed",
    "duration_seconds": 735
  }
}
```

```json
{
  "type": "deployment_complete",
  "data": {
    "deployment_id": "dep_g1h2i3",
    "status": "COMPLETED",
    "total_duration_seconds": 5400,
    "healthcheck_passed": true
  }
}
```

```json
{
  "type": "error",
  "data": {
    "step": "gitlab-ci-runner",
    "phase": "B",
    "error_code": "INSTALL_HELM_TIMEOUT",
    "message": "Helm install timeout after 300s",
    "retryable": true
  }
}
```

#### 클라이언트 → 서버 메시지

```json
{
  "type": "ping"
}
```

#### 연결 유지

| 항목 | 설정 |
|------|------|
| Ping 주기 | 30초 |
| Pong 타임아웃 | 10초 |
| 재연결 | 클라이언트 측 자동 재연결 (최대 5회, 지수 백오프) |
| 로그 유실 방지 | 재연결 시 마지막 수신 타임스탬프 기준으로 누락 로그 전송 |

#### 메시지 타입 요약

| Type | 방향 | 설명 |
|------|------|------|
| `log` | Server → Client | 설치 로그 라인 |
| `status` | Server → Client | 배포 상태 변경 |
| `step_complete` | Server → Client | 단계 완료 알림 |
| `deployment_complete` | Server → Client | 전체 배포 완료 |
| `error` | Server → Client | 에러 발생 |
| `ping` | Client → Server | 연결 유지 확인 |
| `pong` | Server → Client | 연결 유지 응답 |

---

## 8. OpenAPI 3.0 공통 스키마

### 8.1 공통 타입 정의

```yaml
components:
  schemas:
    # --- 공통 응답 래퍼 ---
    ApiResponse:
      type: object
      properties:
        data:
          description: 응답 데이터
        pagination:
          $ref: '#/components/schemas/Pagination'

    Pagination:
      type: object
      properties:
        page:
          type: integer
          example: 1
        page_size:
          type: integer
          example: 20
        total_items:
          type: integer
          example: 42
        total_pages:
          type: integer
          example: 3
        has_next:
          type: boolean
        has_prev:
          type: boolean

    ApiError:
      type: object
      required: [error]
      properties:
        error:
          type: object
          required: [code, http_status, message, retryable, trace_id]
          properties:
            code:
              type: string
              example: "CLUSTER_VERIFY_UNREACHABLE"
            http_status:
              type: integer
              example: 422
            message:
              type: string
              example: "클러스터에 연결할 수 없습니다"
            detail:
              type: string
              example: "TCP 연결 실패"
            retryable:
              type: boolean
              example: true
            trace_id:
              type: string
              example: "tr_a1b2c3d4e5f6"

    # --- 도메인 객체 ---
    Organization:
      type: object
      properties:
        id:
          type: string
          example: "org_x1y2z3"
        name:
          type: string
          example: "Nullus 팀"
        slug:
          type: string
          example: "nullus-team"
        domain:
          type: string
          nullable: true
          example: "nullus.io"
        status:
          type: string
          enum: [active, inactive]
        member_count:
          type: integer
        cluster_count:
          type: integer
        created_by:
          type: string
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time

    Cluster:
      type: object
      properties:
        id:
          type: string
          example: "cls_abc123"
        org_id:
          type: string
        name:
          type: string
          example: "production-gke"
        type:
          type: string
          enum: [pipeline, target]
        endpoint:
          type: string
          example: "https://35.x.x.x:6443"
        kubernetes_version:
          type: string
          nullable: true
          example: "1.28.5"
        auth_method:
          type: string
          example: "kubeconfig"
        status:
          type: string
          enum: [connected, pending, unreachable, auth_failed]
        namespace_count:
          type: integer
          nullable: true
        last_verified_at:
          type: string
          format: date-time
          nullable: true
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time

    ToolConfig:
      type: object
      required: [tool, version]
      properties:
        tool:
          type: string
          example: "gitlab"
        version:
          type: string
          example: "17.7.2"

    StackConfigSpec:
      type: object
      required: [artifacts, pipeline, monitoring, logging, resources]
      properties:
        artifacts:
          type: object
          properties:
            package_registry:
              $ref: '#/components/schemas/ToolConfig'
            source_repository:
              $ref: '#/components/schemas/ToolConfig'
            container_registry:
              $ref: '#/components/schemas/ToolConfig'
            storage_backend:
              $ref: '#/components/schemas/ToolConfig'
        pipeline:
          type: object
          properties:
            ci_platform:
              $ref: '#/components/schemas/ToolConfig'
            cd_tool:
              $ref: '#/components/schemas/ToolConfig'
        monitoring:
          type: object
          properties:
            collection:
              $ref: '#/components/schemas/ToolConfig'
            visualization:
              $ref: '#/components/schemas/ToolConfig'
        logging:
          type: object
          properties:
            collection:
              $ref: '#/components/schemas/ToolConfig'
            search:
              $ref: '#/components/schemas/ToolConfig'
        resources:
          $ref: '#/components/schemas/WorkloadInput'

    StackConfig:
      type: object
      properties:
        id:
          type: string
          example: "stk_m1n2o3"
        name:
          type: string
        cluster_id:
          type: string
        org_id:
          type: string
        golden_path_id:
          type: string
          nullable: true
        config:
          $ref: '#/components/schemas/StackConfigSpec'
        status:
          type: string
          enum: [configured, deploying, deployed, failed]
        current_version:
          type: integer
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time

    WorkloadInput:
      type: object
      properties:
        developers:
          type: integer
          minimum: 1
          maximum: 10000
          example: 20
        concurrent_runners:
          type: integer
          minimum: 1
          maximum: 100
          example: 5
        weekly_commits:
          type: integer
          minimum: 1
          maximum: 10000
          example: 100
        build_frequency:
          type: string
          enum: [hourly, daily, on-push]
          example: "hourly"

    DeploymentStep:
      type: object
      properties:
        name:
          type: string
          example: "gitlab-ce"
        phase:
          type: string
          enum: [A, B, C]
        status:
          type: string
          enum: [pending, installing, completed, failed, rolling_back, skipped]
        order:
          type: integer
        duration_seconds:
          type: integer
          nullable: true

    Deployment:
      type: object
      properties:
        id:
          type: string
          example: "dep_g1h2i3"
        stack_id:
          type: string
        cluster_id:
          type: string
        type:
          type: string
          enum: [stack, pipeline]
        status:
          type: string
          enum: [PENDING, VALIDATING, INSTALLING, CONFIGURING, HEALTHCHECK, COMPLETED, FAILED, RETRYING, ROLLING_BACK, ROLLED_BACK, CANCELLED, TIMEOUT]
        rollback_mode:
          type: string
          enum: [safe, destructive]
        progress_percent:
          type: integer
        steps:
          type: array
          items:
            $ref: '#/components/schemas/DeploymentStep'
        started_by:
          type: string
        started_at:
          type: string
          format: date-time
        completed_at:
          type: string
          format: date-time
          nullable: true
        websocket_url:
          type: string

    # --- 인증 ---
    LoginRequest:
      type: object
      required: [email, password]
      properties:
        email:
          type: string
          format: email
        password:
          type: string
          format: password
          minLength: 8

    UserContext:
      type: object
      properties:
        id:
          type: string
        email:
          type: string
        display_name:
          type: string
        role:
          type: string
          enum: [admin, devops, developer]
        org:
          $ref: '#/components/schemas/Organization'
        permissions:
          type: array
          items:
            type: string

  # --- 보안 ---
  securitySchemes:
    sessionAuth:
      type: apiKey
      in: cookie
      name: nullus_session
      description: "세션 기반 인증 (Alpha/Beta)"

    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
      description: "Keycloak OIDC Bearer Token (v1)"

  # --- 공통 파라미터 ---
  parameters:
    PageParam:
      name: page
      in: query
      schema:
        type: integer
        default: 1
        minimum: 1
    PageSizeParam:
      name: page_size
      in: query
      schema:
        type: integer
        default: 20
        minimum: 1
        maximum: 100
    SortParam:
      name: sort
      in: query
      schema:
        type: string
      description: "정렬 필드. '-' 접두사로 내림차순. 예: '-created_at'"
    AcceptLanguageParam:
      name: Accept-Language
      in: header
      schema:
        type: string
        enum: [ko, en]
        default: en
      description: "에러 메시지 언어"
```

### 8.2 엔드포인트 전체 요약

| # | Method | Path | 설명 | 릴리스 |
|---|--------|------|------|--------|
| 1 | POST | `/api/v1/auth/login` | 세션 로그인 | Alpha |
| 2 | POST | `/api/v1/auth/logout` | 세션 로그아웃 | Alpha |
| 3 | GET | `/api/v1/auth/me` | 내 정보 조회 | Alpha |
| 4 | POST | `/api/v1/auth/token/refresh` | 토큰 갱신 (OIDC) | v1 |
| 5 | GET | `/api/v1/admin/organization` | Organization 조회 | Alpha |
| 6 | PATCH | `/api/v1/admin/organization` | Organization 수정 | Alpha |
| 7 | POST | `/api/v1/admin/orgs` | Organization 생성 | Alpha |
| 8 | POST | `/api/v1/admin/clusters` | 클러스터 등록 | Alpha |
| 9 | GET | `/api/v1/admin/clusters` | 클러스터 목록 | Alpha |
| 10 | GET | `/api/v1/admin/clusters/:id` | 클러스터 상세 | Alpha |
| 11 | PATCH | `/api/v1/admin/clusters/:id` | 클러스터 수정 | Alpha |
| 12 | DELETE | `/api/v1/admin/clusters/:id` | 클러스터 삭제 | Alpha |
| 13 | POST | `/api/v1/admin/clusters/:id/verify` | 연결 검증 | Alpha |
| 14 | GET | `/api/v1/admin/known-issues` | Known Issues 목록 | Alpha |
| 15 | GET | `/api/v1/admin/audit-logs` | 감사 로그 조회 | Alpha |
| 16 | GET | `/api/v1/admin/notifications/configs` | 알림 설정 목록 | Alpha |
| 17 | POST | `/api/v1/admin/notifications/configs` | 알림 설정 생성 | Alpha |
| 18 | GET | `/api/v1/admin/notifications/history` | 알림 전송 이력 | Alpha |
| 19 | GET | `/api/v1/admin/organizations/:orgId/members` | 조직 멤버 목록 | Beta |
| 20 | POST | `/api/v1/admin/organizations/:orgId/members` | 조직 멤버 초대 | Beta |
| 21 | DELETE | `/api/v1/admin/organizations/:orgId/members/:id` | 조직 멤버 제거 | v1 |
| 22 | POST | `/api/v1/stacks` | 스택 설정 생성 | Alpha |
| 23 | GET | `/api/v1/stacks` | 스택 목록 | Alpha |
| 24 | GET | `/api/v1/stacks/:id` | 스택 상세 | Alpha |
| 25 | PUT | `/api/v1/stacks/:id` | 스택 수정 | Alpha |
| 26 | GET | `/api/v1/stacks/:id/history` | 변경 이력 | v1 |
| 27 | GET | `/api/v1/stacks/:id/history/:versionId/diff` | 버전 diff | v1 |
| 28 | POST | `/api/v1/stacks/:id/rollback/:versionId` | 버전 롤백 | v1 |
| 29 | GET | `/api/v1/stacks/templates` | Golden Path 목록 | Alpha |
| 30 | GET | `/api/v1/stacks/templates/:id` | Golden Path 상세 | Alpha |
| 31 | GET | `/api/v1/stacks/compatibility` | 호환성 매트릭스 | Alpha |
| 32 | POST | `/api/v1/stacks/:stackId/validate` | 호환성 검증 | Alpha |
| 33 | POST | `/api/v1/installations` | 스택 설치 시작 | Alpha |
| 34 | GET | `/api/v1/installations/:id/status` | 설치 상태 조회 | Alpha |
| 35 | DELETE | `/api/v1/installations/:id` | 설치 취소 | Beta |
| 36 | POST | `/api/v1/installations/:id/retry` | 실패 재시도 | Beta |
| 37 | POST | `/api/v1/installations/:id/rollback` | 설치 롤백 | Beta |
| 38 | GET | `/api/v1/installations/:id/logs` | 설치 로그 (HTTP) | Alpha |
| 39 | GET | `/api/v1/cicd/templates` | CI/CD 템플릿 목록 | Beta |
| 40 | GET | `/api/v1/cicd/templates/:id` | CI/CD 템플릿 상세 | Beta |
| 41 | POST | `/api/v1/cicd/pipelines` | 파이프라인 생성 | Beta |
| 42 | GET | `/api/v1/cicd/pipelines` | 파이프라인 목록 | Beta |
| 43 | POST | `/api/v1/cicd/pipelines/:id/deploy` | 파이프라인 배포 | Beta |
| 44 | GET | `/api/v1/cicd/deployments/:did` | 배포 상세 | v1 |
| 45 | POST | `/api/v1/cicd/pipelines/:id/rollback/:did` | 배포 롤백 | v1 |
| 46 | GET | `/api/v1/cicd/deployments/:did/diff` | 배포 diff | v1 |
| 47 | GET | `/api/v1/observability/dashboard` | 대시보드 데이터 | Beta |
| 48 | GET | `/api/v1/observability/alert-rules` | 알림 규칙 목록 | Beta |
| 49 | POST | `/api/v1/observability/alert-rules` | 알림 규칙 생성 | Beta |
| 50 | GET | `/api/v1/observability/alert-history` | 알림 이력 | Beta |
| 51 | GET | `/api/v1/cicd/app-templates` | 앱 템플릿 목록 | Beta |
| 52 | POST | `/api/v1/cicd/deploy-app` | 앱 배포 | Beta |
| 53 | POST | `/api/v1/resources/estimate` | 리소스 예상량 | Alpha |
| 54 | GET | `/api/v1/stacks/resource-defaults` | OSS 리소스 request/limit 기본값 목록 조회 | Alpha |
| 55 | POST | `/api/v1/stacks/resource-defaults` | OSS 리소스 request/limit 업서트 (`tool_key` 기준, idempotent) | Alpha |
| 56 | GET | `/api/v1/users` | 사용자 목록 | v1 |
| 57 | PUT | `/api/v1/users/:userId/role` | 역할 변경 | v1 |
| 58 | DELETE | `/api/v1/users/:userId` | 사용자 비활성화 | v1 |
| 59 | WSS | `/ws/deployments/:id/logs` | 설치 로그 스트리밍 | Alpha |
