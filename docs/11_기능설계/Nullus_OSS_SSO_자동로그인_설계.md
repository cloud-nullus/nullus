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

---

## 7. 일반 설치 경로 적용 설계 (W7)

**작성일**: 2026-07-28
**연관 문서**: `OpenBao_시크릿_평면_구축_설계.md` (P3)

### 7.1 현재 상태 — SSO가 에어갭에만 존재한다

| 항목 | 에어갭 경로 | 일반 설치 경로 |
|---|---|---|
| Keycloak 설치 | `27-install-stacks.sh`가 `NAMESPACE_AUTH`에 설치 | **없음** — 설치 DAG에 `installing_keycloak` 부재 |
| OIDC client 등록 | `30-provision-sso.sh`가 Keycloak API 직접 호출 | **없음** — `ProvisionSSO`가 어디서도 호출되지 않음 |
| OSS values의 OIDC 블록 | `airgap/helm/stack-values/*.yaml`에 존재 | **없음** — 코드 생성 values에 OIDC 설정 없음 |
| client secret | `*-dev-secret` 리터럴 5종을 Keycloak에 push | 해당 없음 |

즉 일반 설치는 IdP도, 클라이언트 등록도, OSS 설정도 없다. 세 가지를 모두 채워야 SSO가 성립한다.

### 7.2 Keycloak 조달 경로

**Keycloak은 스택 구성요소가 아니라 플랫폼 구성요소로 둔다.** 스택마다 IdP를 띄우지 않는다.

근거: Nullus 자체 로그인도 Keycloak을 사용하므로 스택별 설치는 IdP를 이중화한다. 에어갭 경로가 이미 `NAMESPACE_AUTH`로 분리한 구조와도 일치한다. 스택 설치기는 **기존 Keycloak에 클라이언트를 등록만** 한다.

`authentication.provider`는 현재 `'' | 'openbao'`만 받는다. OIDC issuer 정보를 받을 필드를 별도로 추가한다.

| 모드 | 동작 |
|---|---|
| 플랫폼 Keycloak (기본) | Nullus가 관리하는 Keycloak의 issuer를 자동 주입 |
| 외부 IdP (BYO) | 운영자가 issuer/realm을 직접 입력. 기존 IdP를 쓰는 온프레 고객 대응 |
| 미사용 | OIDC 블록을 생성하지 않는다. 각 OSS 자체 로그인 유지 |

**결정: Keycloak을 `deploy/helm/nullus` 차트의 조건부 의존성으로 포함한다.**

현재 차트 의존성은 `postgresql` 하나뿐이며, Keycloak은 에어갭 스크립트(`22-install-platform-stack.sh`)가 `nullus-auth` 네임스페이스에 설치한다. 그 스크립트는 이미 Keycloak을 "기본(필수)"로 분류하고 있으므로, 차트로 올리면 에어갭과 일반 설치가 같은 경로로 통일된다.

```yaml
# deploy/helm/nullus/Chart.yaml
dependencies:
  - name: postgresql
    condition: postgresql.enabled
  - name: keycloak          # 신규
    condition: keycloak.enabled
```

동반 변경:

- `22-install-platform-stack.sh`의 Keycloak 설치 블록을 제거한다 (차트가 흡수)
- `airgap/images/images.txt`는 `helm template deploy/helm/nullus` 결과로 자동 생성되므로, Keycloak 이미지가 "카탈로그" 섹션에서 "차트 의존성" 섹션으로 자연히 이동한다
- **realm/client 부트스트랩은 차트가 아니라 백엔드가 담당한다.** realm 자체는 기동 시 멱등 보장, client는 스택 설치마다 추가되는 런타임 관심사이기 때문이다. `setup-keycloak.sh`가 하던 역할을 백엔드로 옮긴다
- `keycloak.enabled=false`로 두면 BYO 모드가 된다

### 7.3 Client ID 네임스페이싱

`ToolSSOSpec`의 ClientID는 `grafana`, `argocd` 등으로 고정돼 있다. 공용 realm에 여러 스택이 등록하면 **client ID가 충돌**한다 — 두 스택의 Grafana가 같은 clientId를 두고 redirect URI를 서로 덮어쓴다.

**client ID를 스택 단위로 네임스페이싱한다.**

```text
{stack-slug}-{tool}     예) prod-devops-grafana, staging-devops-argocd
```

realm 분리(org별 realm)는 사용자·그룹 관리까지 쪼개져 운영 복잡도가 크므로 후속 과제로 둔다.

### 7.4 Client Secret 생명주기 — P3와의 통합

**생성 주체는 Nullus다.** Keycloak이 생성한 secret을 읽어오지 않고, Nullus가 만든 값을 Keycloak에 push한다.

근거: OpenBao가 Source of Truth여야 하며(PRD 5.2 "OIDC client secret은 OpenBao 경유로만 주입"), push 방식이면 Keycloak이 유실돼도 OpenBao에서 복원할 수 있다. 현재 에어갭 스크립트도 `"secret": "${secret}"`로 push하는 구조라 방식이 동일하다 — 값의 출처만 리터럴에서 생성값으로 바뀐다.

```text
provisioning_secrets   random(32) → OpenBao write
                       → ExternalSecret → K8s Secret
        ↓
provisioning_sso       OpenBao read → Keycloak upsert(clientId, secret, redirectURIs)
        ↓
installing_{oss}       K8s Secret 참조 (env / secretKeyRef)
```

OpenBao 경로: `kv/nullus/{env}/{org_id}/auth/{client_id}/client-secret`

`provisioning_sso`는 **Keycloak 기동 이후**여야 한다. 설치 스텝 의존성에 이 제약을 반영한다.

### 7.5 Go 프로비저너 보강

현재 `internal/auth/adapter/keycloak/sso_provisioner.go`는 다음 네 가지가 비어 있다.

| # | 항목 | 현재 | 조치 |
|---|---|---|---|
| G1 | client secret | `RegisterOIDCClient`에 secret 파라미터 없음. confidential client(`publicClient: false`)를 만들지만 Keycloak 생성 secret을 되읽지도 않아 값이 어디에도 없다 | secret 파라미터 추가, push 방식 |
| G2 | **upsert 미지원** | `409 Conflict`를 성공으로 처리해 **기존 클라이언트가 갱신되지 않는다**. secret을 회전해도 Keycloak에 반영되지 않아 로그인이 깨진다 | 존재 시 PUT으로 갱신 |
| G3 | PKCE / webOrigins | 설정하지 않음. values는 PKCE를 전제하므로 불일치 | `ToolSSOSpec`에 필드 추가. Grafana/Harbor/GitLab은 `S256`, MinIO/ArgoCD는 미설정 (에어갭 스크립트의 기존 분기와 동일) |
| G4 | 파이프라인 배선 | `ProvisionSSO`가 정의부와 테스트 외에 호출되지 않음 | `provisioning_sso` 스텝에서 호출 |

**G2가 가장 중요하다.** 시크릿 회전 파이프라인과 직결되며, 증상이 "회전 후 SSO 로그인 실패"로만 나타나 원인 추적이 어렵다.

**모듈 경계**: stack usecase가 `internal/auth/adapter/keycloak`을 직접 import하면 모듈 간 직접 의존 금지 규칙에 위배된다. `internal/stack/port`에 인터페이스를 정의하고 `cmd/api/main.go`에서 auth 구현체를 주입한다 — `TokenSourceRegistry`, `secrets.Router`와 동일한 패턴이다.

### 7.6 코드 생성 values에 OIDC 주입

`internal/stack/adapter/helm/`의 values 생성기에 도구별 OIDC 블록을 추가한다. 기준 구성은 `airgap/helm/stack-values/*.yaml`에 이미 검증된 형태로 존재하므로 그대로 옮기되, 두 가지를 바꾼다.

- **issuer/auth_url을 accessDomain 기반으로 생성** — 에어갭 values는 `keycloak.nullus.internal`이 하드코딩돼 있다
- **client secret은 값이 아니라 참조로** — Grafana는 `secretKeyRef` env, ArgoCD는 ESO가 소유하는 `argocd-secret`

| OSS | 주입 위치 | secret 참조 방식 |
|---|---|---|
| Grafana | `grafana.ini.auth\.generic_oauth` + `auto_login` | env `GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET` ← `secretKeyRef` |
| ArgoCD | `configs.cm.oidc.config` | `$oidc.keycloak.clientSecret` ← ESO 소유 `argocd-secret` |
| Harbor | 설치 후 API로 `auth_mode=oidc_auth` 설정 | 부트스트랩 시 K8s Secret에서 로드 |
| MinIO | `oidc.clientSecret` | `secretKeyRef` |
| GitLab | `omniauth` provider 블록 | K8s Secret 마운트 |

### 7.7 argocd-secret 소유권 (P3 예외)

P3는 "`existingSecret` 패턴이면 Helm과 ESO의 Secret 소유권 충돌이 사라진다"를 전제하지만 **ArgoCD는 예외다.**

`argocd-secret` 하나에 admin 비밀번호 해시와 OIDC client secret이 함께 들어간다. 차트는 `configs.secret.extra`로, ESO는 admin 해시로 같은 Secret을 건드리려 하므로 `creationPolicy: Owner`가 차트가 넣은 키를 덮어쓴다.

**해결**: `argocd-secret`을 ESO가 통째로 소유하고 차트의 Secret 생성을 끈다(`configs.secret.createSecret: false`). admin 해시와 OIDC client secret을 **하나의 ExternalSecret에 함께** 담는다.

### 7.8 에어갭 경로 통합

**결정: 에어갭도 Nullus 백엔드 설치 경로를 사용한다. 설치 구현은 하나로 통일한다.**

현재 에어갭에는 백엔드를 우회하는 **중복 설치 구현**이 존재한다.

| 스크립트 | 현재 역할 | 조치 |
|---|---|---|
| `21-install-nullus.sh` | 플랫폼 Helm 설치 | 유지 (정본) |
| `22-install-platform-stack.sh` | Keycloak 설치 | Keycloak 블록 제거 → 차트가 흡수 (7.2) |
| `27-install-stacks.sh` | **스택 15종을 helm으로 직접 설치** | helm 직접 호출 → **백엔드 API 호출로 재작성** |
| `30-provision-sso.sh` | SSO 클라이언트 등록 | **폐기** → 백엔드 `provisioning_sso`가 대체 |

`22-install-platform-stack.sh`는 이미 주석에 *"Harbor / MinIO / ArgoCD는 Nullus UI의 Stack 설치 기능에서 사용자가 선택 시 설치된다"* 고 정본 경로를 명시하고 있다. `27`은 백엔드 없이 검증하기 위한 보조 수단이었으므로, 무인 검증 목적만 남기고 호출 방식을 API로 바꾼다.

**선행 작업 2가지**

**① 자기 클러스터 등록(self-registration)** — 현재 스택 설치는 kubeconfig가 등록된 클러스터를 대상으로 한다. 에어갭에서는 Nullus가 자기가 떠 있는 클러스터에 설치해야 하므로, in-cluster ServiceAccount 기반으로 자신을 대상 클러스터로 자동 등록하는 경로가 필요하다.

현재 `ClusterType`은 `pipeline` / `target` 두 가지뿐이며 self 개념이 없다. 이 기능은 에어갭 전용이 아니라 **단일 클러스터 설치 시나리오 전반에 유용**하므로 제품 기능으로 설계한다.

**② 부트스트랩 인증** — 스크립트가 Admin API를 호출할 자격이 필요하다. Keycloak service account(client credentials) 방식을 사용하고, 부트스트랩 전용 자격은 설치 완료 후 폐기한다.

**통합 후 에어갭 파이프라인**

```text
21  플랫폼 Helm 설치 (Keycloak 포함)
26  DB 마이그레이션
--  자기 클러스터 등록 (신규)
27  스택 설치 요청 → 백엔드 API  ← helm 직접 호출에서 전환
    └ 백엔드가 P1~P3 스텝을 수행 (OpenBao init/unseal → auth → ESO → provisioning_secrets → provisioning_sso → OSS 설치)
23  게이트웨이 구성
99  검증
```

하드코딩 secret 5종(`grafana-dev-secret` 등)은 `provisioning_secrets`가 생성값으로 대체하므로 자연히 제거된다.

> 통합 완료 전까지는 두 경로가 병존한다. 그 기간에는 에어갭 경로와 일반 경로가 동일한 구성을 산출하는지 확인하는 회귀 검증이 필요하다.

### 7.9 작업 순서

```text
[플랫폼]
 1. Keycloak을 nullus 차트 조건부 의존성으로 추가 (7.2)
 2. realm 부트스트랩을 백엔드 기동 경로로 이관
 3. authentication에 OIDC issuer 필드 추가 (플랫폼 / BYO / 미사용)

[프로비저닝]
 4. Go 프로비저너 보강 G1~G3 + 단위 테스트
 5. provisioning_secrets에 OIDC client secret 추가 (P3)
 6. provisioning_sso 스텝 신설 + port 인터페이스 정의 + main.go 주입 (G4)
 7. 코드 생성 values에 OIDC 블록 추가 (7.6)
 8. argocd-secret 소유권 전환 (7.7)

[에어갭 통합]
 9. 자기 클러스터 등록(self-registration) 구현
10. 부트스트랩 인증 경로 구현
11. 27-install-stacks.sh를 API 호출로 재작성, 30-provision-sso.sh 폐기
12. 22-install-platform-stack.sh의 Keycloak 블록 제거
```

4~8은 P3와 같은 릴리스에 묶인다. client secret이 생성 방식으로 바뀌는 breaking change를 한 번에 처리하기 위해서다.

9~12(에어갭 통합)은 **P3 완료 후 별도 단계**로 진행한다. 자기 클러스터 등록이 제품 기능이라 범위가 크고, P1~P3가 끝나야 백엔드 경로가 에어갭에서 실제로 동작하기 때문이다.
