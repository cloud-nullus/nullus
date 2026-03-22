# Nullus OIDC Provider 가이드

**작성일**: 2026-03-22
**범위**: Keycloak / Authentik 듀얼 OIDC 인증 지원

---

## 1. 개요

Nullus는 OIDC(OpenID Connect) 기반 인증을 지원하며, **Keycloak**과 **Authentik** 두 가지 Identity Provider를 환경변수 하나로 전환할 수 있다.

- **Frontend**: `oidc-providers.ts` — Provider config factory + role extractor
- **Backend**: `port/oidc_provider.go` — OIDCProvider 인터페이스 + 구현체
- **Mock Auth**: `VITE_AUTH_MODE=mock`으로 OIDC 없이 개발 가능

---

## 2. 빠른 시작 — Provider 전환

### Keycloak (기본)

```bash
# Frontend (.env)
VITE_AUTH_MODE=oidc
VITE_OIDC_PROVIDER=keycloak
VITE_OIDC_AUTHORITY=http://localhost:8180/realms/nullus
VITE_OIDC_CLIENT_ID=nullus-web

# Backend (configs/config.yaml)
auth:
  mode: oidc
oidc:
  provider: keycloak
  issuer_url: http://localhost:8180/realms/nullus
  audience: nullus-app
```

### Authentik

```bash
# Frontend (.env)
VITE_AUTH_MODE=oidc
VITE_OIDC_PROVIDER=authentik
VITE_OIDC_AUTHORITY=http://localhost:9000/application/o/nullus-platform/
VITE_OIDC_CLIENT_ID=nullus-web

# Backend (configs/config.yaml)
oidc:
  provider: authentik
  issuer_url: http://localhost:9000/application/o/nullus-platform/
  audience: nullus-app
```

### Mock Auth (개발용)

```bash
VITE_AUTH_MODE=mock
# OIDC 관련 변수 불필요. 테스트 계정:
# admin@nullus.dev / devops@nullus.dev / developer@nullus.dev
```

---

## 3. 아키텍처 — 추상화 구조

### Frontend

```
web/src/lib/oidc-providers.ts          ← Provider config factory
  ├── OIDCProviderConfig interface
  │     { type, authority, clientId, scope, extractRoles, getLogoutUrl? }
  ├── keycloakExtractRoles(user)       ← user.profile.realm_access.roles
  ├── authentikExtractRoles(user)      ← user.profile.groups
  ├── getProviderConfig()              ← VITE_OIDC_PROVIDER로 자동 선택
  ├── toAuthProviderProps(config)      ← react-oidc-context용 변환
  └── isOidcMode                       ← VITE_AUTH_MODE !== 'mock'

web/src/lib/oidc-config.ts             ← 하위 호환성 re-export

web/src/stores/auth-store.ts
  └── extractRoleFromOidc(user)        ← getProviderConfig().extractRoles 사용

web/src/main.tsx                       ← isOidcMode ? AuthProvider : passthrough
web/src/features/auth/pages/login-page.tsx  ← OidcLoginContent / MockLoginContent
web/src/components/layout/sidebar.tsx       ← provider별 logout 분기
```

### Backend

```
internal/auth/port/oidc_provider.go           ← 인터페이스
  type OIDCProvider interface {
      ExtractRoles(claims jwt.MapClaims) []string
      Name() string
  }

internal/auth/adapter/keycloak/oidc_provider.go   ← realm_access.roles
internal/auth/adapter/authentik/oidc_provider.go   ← groups claim
internal/auth/adapter/provider_factory.go          ← NewOIDCProvider("keycloak"|"authentik")

internal/auth/adapter/middleware/jwt_middleware.go  ← OIDCProvider 주입
  func NewJWTMiddleware(cfg JWTConfig, provider OIDCProvider)
  → m.provider.ExtractRoles(claims) 호출
```

---

## 4. Keycloak vs Authentik 핵심 차이

| 항목 | Keycloak | Authentik |
|------|----------|-----------|
| **Discovery URL** | `/realms/{realm}/.well-known/openid-configuration` | `/application/o/{slug}/.well-known/openid-configuration` |
| **Issuer 단위** | Realm | Application |
| **Role claim 경로** | `realm_access.roles` (중첩 객체) | `groups` (최상위 배열) |
| **Role claim scope** | 별도 mapper 필요 | `profile` scope에 자동 포함 |
| **Logout** | `post_logout_redirect_uri` 즉시 동작 | Flow 설정 필요 (불안정) |
| **PKCE** | ✅ | ✅ |
| **react-oidc-context 호환** | ✅ | ✅ (authority URL만 다름) |
| **Docker 구성** | 2 컨테이너 (app + DB) | 3 컨테이너 (server + worker + DB) |

### Role Claim 추출 비교

```go
// Keycloak — 중첩 객체 접근
claims["realm_access"].(map[string]any)["roles"].([]any)
// → ["admin", "devops", "developer"]

// Authentik — 최상위 평탄 배열
claims["groups"].([]any)
// → ["admin", "devops", "developer"]
```

```typescript
// Frontend — provider에 따라 자동 분기
function keycloakExtractRoles(user: User): string[] {
  return user.profile?.realm_access?.roles ?? []
}
function authentikExtractRoles(user: User): string[] {
  return (user.profile?.groups as string[]) ?? []
}
```

---

## 5. 새 Provider 추가 방법

### Step 1: Frontend — Role Extractor 추가

`web/src/lib/oidc-providers.ts`:
```typescript
function newProviderExtractRoles(user: User): string[] {
  const profile = user.profile as Record<string, unknown>
  return (profile?.custom_roles_claim as string[]) ?? []
}
```

`getProviderConfig()` 함수에 case 추가:
```typescript
if (provider === 'new-provider') {
  return { type: 'new-provider', authority, clientId, scope: 'openid profile email',
    extractRoles: newProviderExtractRoles }
}
```

### Step 2: Backend — Go Provider 구현체

`internal/auth/adapter/newprovider/oidc_provider.go`:
```go
package newprovider

type OIDCProvider struct{}

func NewOIDCProvider() *OIDCProvider { return &OIDCProvider{} }
func (p *OIDCProvider) Name() string { return "new-provider" }
func (p *OIDCProvider) ExtractRoles(claims jwt.MapClaims) []string {
    roles, ok := claims["custom_roles_claim"].([]any)
    if !ok { return nil }
    // ... 변환 로직
}
```

### Step 3: Factory에 등록

`internal/auth/adapter/provider_factory.go`:
```go
case "new-provider":
    return newprovider.NewOIDCProvider(), nil
```

### Step 4: 환경변수 문서 업데이트

`.env.example`과 `configs/config.yaml`에 새 provider 예시 추가.

---

## 6. Authentik Logout 주의사항

Authentik의 `end-session` 엔드포인트는 `post_logout_redirect_uri`를 무시하고 자체 Invalidation Flow UI를 표시할 수 있다.

**해결 방법** (`sidebar.tsx`에 구현됨):

```typescript
const config = getProviderConfig()
if (config.getLogoutUrl && auth.user?.id_token) {
  // Authentik: 로컬 세션 먼저 제거 후 수동 리다이렉트
  await auth.removeUser()
  window.location.href = config.getLogoutUrl(auth.user.id_token, window.location.origin)
} else {
  // Keycloak: 표준 signoutRedirect 사용
  await auth.signoutRedirect()
}
```

**Authentik Admin 설정**: Flows → `default-provider-invalidation-flow` → `default-invalidation-logout` stage 바인딩 필요.

---

## 7. 트러블슈팅

| 증상 | 원인 | 해결 |
|------|------|------|
| `realm_access` claim 없음 | Keycloak Client Scope에 `realm_access` mapper 미설정 | Keycloak Admin → Client Scopes → `roles` scope 확인 |
| `groups` claim 없음 | Authentik에서 `profile` scope 미요청 | OIDC scope에 `openid profile email` 포함 확인 |
| Logout 후 리다이렉트 안 됨 | Authentik Flow 미설정 | Invalidation Flow에 logout stage 바인딩 |
| CORS 에러 | Web Origins 미설정 | Keycloak: Client → Web Origins에 `http://localhost:5173` 추가 |
| Silent renew 실패 | Keycloak refresh token 만료 | Realm Settings → Tokens → SSO Session Max 확인 |
| 무한 리다이렉트 | Authority URL 오류 | Discovery endpoint 직접 접근하여 확인 |

---

## 8. 참고 자료

| 자료 | 경로/URL |
|------|----------|
| OIDC Provider 코드 | `web/src/lib/oidc-providers.ts` |
| Backend Provider 인터페이스 | `internal/auth/port/oidc_provider.go` |
| JWT 미들웨어 | `internal/auth/adapter/middleware/jwt_middleware.go` |
| Keycloak 공식 문서 | https://www.keycloak.org/documentation |
| Authentik 공식 문서 | https://docs.goauthentik.io |
| react-oidc-context | https://github.com/authts/react-oidc-context |
