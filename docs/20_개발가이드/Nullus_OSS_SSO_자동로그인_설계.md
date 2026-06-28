# Nullus OSS SSO 자동 로그인 설계 (W6)

> 목표: 설치된 OSS(Grafana / ArgoCD / Harbor) 링크 접속 시 Keycloak SSO로 자연스럽게(재인증 없이) 로그인되게 한다.
> 범위: **OSS 링크 핸드오프만**. nullus-web 프론트의 실제 Keycloak 로그인 구현은 별개 작업으로 가정한다(브라우저에 Keycloak 세션이 이미 있다고 전제).
> 검증: 로컬 docker-compose.

---

## 1. 대상 OSS 진입 흐름 정리

설치된 OSS 링크는 두 경로로 노출된다.

| 경로 | 위치 | 비고 |
|------|------|------|
| 통합 엔드포인트 API | `internal/stack/adapter/handler/stack_handler.go:240` `integrationEndpoint()` / `:175` `/api/v1/stacks/:stackId/integrations` | `https://{subdomain}.{accessDomain}` 형식 URL 생성 |
| Gateway 라우팅 | `internal/stack/adapter/helm/manifest-builders.go:128` `defaultGatewayBundleManifest()` | Envoy `HTTPRoute`로 `argocd./grafana./harbor.{accessDomain}` → 서비스 매핑 |

현재 흐름: **사용자 → nullus-web에서 링크 클릭 → OSS 도메인 직접 접속 → 각 OSS의 자체 로그인 페이지**. SSO 개입 지점이 없다.

목표 흐름: **링크 클릭 → OSS 접속 → (앱 세션 없음) → OSS가 Keycloak으로 redirect → Keycloak 기존 SSO 쿠키로 즉시 인증 → code 콜백 → OSS 로그인 완료**. 핵심은 "redirect/callback을 우리가 만드는 게 아니라, **각 OSS의 내장 OIDC 클라이언트가 처리**하도록 구성"하는 것이다.

---

## 2. 현재 로그인 방식과의 충돌 포인트

| # | 충돌 | 현재 상태 | 해소 방향 |
|---|------|-----------|-----------|
| C1 | **OSS가 자체 로그인 사용** | Grafana/ArgoCD/Harbor values에 OIDC 설정 없음 (`airgap/helm/stack-values/*.yaml`) | 각 OSS values에 Keycloak OIDC 블록 추가 |
| C2 | **Keycloak에 OSS 클라이언트 미등록** | realm export·`setup-keycloak.sh`는 `nullus-app`/`nullus-web`만 등록. `sso_provisioner.go`에 grafana/argocd는 있으나 harbor 없음, redirect URI가 `*.nullus.local` 하드코딩 | provisioner에 harbor 추가 + redirect URI를 accessDomain 기반으로, 로컬 setup 스크립트에 3종 클라이언트 추가 |
| C3 | **자동 로그인(무클릭) 미설정** | OIDC를 켜도 Grafana는 기본 로그인 페이지를 먼저 보여줌 | Grafana `oauth_auto_login=true` 등 각 OSS의 auto-login 옵션 활성화 |
| C4 | **세션 전제 불일치** | nullus-web은 현재 mock/sessionStorage(`web/src/stores/auth-store.ts`) → 실제 Keycloak 브라우저 세션 없음 | **범위 밖**(별도 작업). 본 설계는 Keycloak 세션 존재를 전제. 문서에 명시 |
| C5 | **issuer URL 정합성(로컬)** | 브라우저 redirect용 issuer와 OSS 컨테이너의 토큰 검증용 issuer가 다르면 실패(localhost vs compose DNS) | 로컬은 단일 issuer `http://localhost:8180/realms/nullus` 사용 + OSS 컨테이너에 `extra_hosts`/host 접근 보장 |

---

## 3. Keycloak 우선 자동 로그인 설계안

### 3.1 원칙
- **Keycloak이 단일 IdP**. realm `nullus` 하나에 OSS별 confidential client를 등록한다(`grafana`, `argocd`, `harbor`).
- redirect URI는 `accessDomain`(로컬: `localhost`/지정 포트)으로 파라미터화한다 — `*.nullus.local` 하드코딩 제거.
- "자연스러운 로그인" = 각 OSS의 **auto-login 옵션**으로 로그인 페이지를 건너뛰고 Keycloak으로 즉시 redirect. Keycloak에 SSO 쿠키가 있으면 무중단 통과.
- Authentik은 후순위 — provisioner/values 구조는 provider 교체 가능하게 두되 이번엔 Keycloak만 구현.

### 3.2 OSS별 구성

**Grafana** (`generic_oauth`)
```ini
[auth.generic_oauth]
enabled = true
name = Keycloak
auto_login = true            # ← 로그인 페이지 스킵, 핵심
client_id = grafana
client_secret = <provisioned>
scopes = openid profile email
auth_url = http://localhost:8180/realms/nullus/protocol/openid-connect/auth
token_url = http://localhost:8180/realms/nullus/protocol/openid-connect/token
api_url  = http://localhost:8180/realms/nullus/protocol/openid-connect/userinfo
role_attribute_path = contains(realm_access.roles[*], 'admin') && 'Admin' || 'Viewer'
```
- Helm values: `grafana.ini.auth\.generic_oauth.*` + `[auth] oauth_auto_login = true`(구버전 키 호환).
- Redirect URI: `https://grafana.{accessDomain}/login/generic_oauth`.

**ArgoCD** (`oidc.config`, argocd-cm)
```yaml
oidc.config: |
  name: Keycloak
  issuer: http://localhost:8180/realms/nullus
  clientID: argocd
  clientSecret: $oidc.keycloak.clientSecret
  requestedScopes: ["openid","profile","email","groups"]
```
- auto-login: ArgoCD는 OIDC 단독 구성 시 로그인 화면에 "LOG IN VIA KEYCLOAK"만 노출. 완전 무클릭이 필요하면 `users.anonymous`가 아닌 redirect 옵션 검토(범위 내 최소: OIDC 버튼 1클릭 허용, 문서화).
- Redirect URI: `https://argocd.{accessDomain}/auth/callback`.

**Harbor** (OIDC auth_mode)
```yaml
# values: harbor.yaml — 설치 후 Harbor는 DB에 auth_mode 저장.
# 부트스트랩: configmap/initContainer 또는 setup 스크립트의 Harbor API 호출로
#   auth_mode=oidc_auth, oidc_endpoint, oidc_client_id=harbor 설정.
```
- auto-login: Harbor도 OIDC 버튼 노출 방식. 최소 범위는 OIDC 모드 활성 + 버튼 1클릭, 무중단 통과는 Keycloak 세션 재사용으로 달성.
- Redirect URI: `https://harbor.{accessDomain}/c/oidc/callback`.

### 3.3 백엔드 변경
- `internal/auth/adapter/keycloak/sso_provisioner.go`: `installing_harbor` 스펙 추가, redirect URI를 `accessDomain` 인자로 생성하도록 시그니처/구성 보강(하드코딩 제거). TDD: provisioner 스펙 테이블·URI 생성 단위 테스트.
- `setup-keycloak.sh` / `keycloak-realm-export.json`: 로컬 검증용으로 `grafana`/`argocd`/`harbor` confidential client 3종 등록(redirect URI·client secret 포함).
- 링크 핸드오프: `integrationEndpoint()`는 이미 `https://{subdomain}.{accessDomain}`를 반환하므로 URL 자체 변경은 불필요. 단, Grafana는 `auto_login`으로 처리되므로 **링크는 그대로 OSS 루트 URL을 가리키면 됨**(추가 쿼리 불필요). 회귀 테스트로 grafana/harbor/argocd subdomain 매핑 확인.

---

## 4. 로컬 docker-compose 검증 설계

- 신규 오버레이 `docker-compose.sso.yaml`: 기존 `docker-compose.dev.yaml`(Keycloak 8180)에 **Grafana** 컨테이너 추가(가장 가벼운 PoC 대상).
  - Grafana env로 `GF_AUTH_GENERIC_OAUTH_*` + `GF_AUTH_GENERIC_OAUTH_AUTO_LOGIN=true` 주입, issuer=`http://localhost:8180/realms/nullus`.
  - issuer 정합(C5): Grafana 컨테이너에 `extra_hosts` 또는 `network_mode`로 `localhost:8180` 도달 보장.
- 스모크 절차(`scripts/smoke-sso.sh`):
  1. `docker compose -f docker-compose.dev.yaml -f docker-compose.sso.yaml up -d`
  2. `setup-keycloak.sh`로 realm + grafana client 등록.
  3. Keycloak에 직접 로그인하여 SSO 쿠키 확보(세션 전제 시뮬레이션).
  4. 같은 쿠키 자(cookie jar)로 Grafana 루트 GET → `302 → keycloak/auth` → `302 → grafana/login/generic_oauth?code=...` → Grafana 세션 쿠키 발급까지 **재인증 없이** 도달하는지 `curl -L -c/-b` 체인으로 확인.
  5. 최종 `200` + Grafana 사용자 API(`/api/user`)가 Keycloak 사용자 반환 → PASS.

---

## 5. 범위 밖(명시)
- nullus-web 프론트의 실제 Keycloak 로그인(react-oidc-context) 구현 — 본 설계의 전제, 별도 작업.
- ArgoCD/Harbor의 docker-compose 실행 — 무겁고 핸드오프 검증은 Grafana로 충분. 두 앱은 values/Keycloak client 구성 + 설정 단위 검증까지.
- GitLab/MinIO/OpenSearch 등 나머지 OSS — 후순위.

## 6. 결정 로그
- **D1**: 핸드오프 검증 PoC = Grafana 1종(docker-compose). 이유: 가장 가벼움·`auto_login` 무중단 통과를 가장 명확히 증명. 비용: ArgoCD/Harbor는 설정 레벨 검증에 그침. 탈출구: 후속 W에서 k8s e2e.
- **D2**: redirect/callback은 신규 구현하지 않고 OSS 내장 OIDC에 위임. 이유: "핸드오프만" 범위·중복 회피. 비용: OSS별 옵션 차이 흡수 필요. 탈출구: 무중단이 부족한 앱은 후속에서 게이트웨이 보강.
