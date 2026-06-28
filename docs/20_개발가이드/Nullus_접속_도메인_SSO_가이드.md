# Nullus 접속 도메인 · SSO 가이드

> kind(airgap) 클러스터에 설치된 Nullus 플랫폼과 OSS 스택을 `*.nullus.internal` 도메인으로 접속하고, Keycloak SSO로 로그인하는 방법.
> **정본(authoritative source)**: 도메인·라우팅은 `airgap/scripts/23-setup-gateway.sh`(ROUTES 테이블)와 `airgap/scripts/24-register-hosts.sh`(/etc/hosts 등록)에 정의된다. 이 문서는 그 요약·운영 가이드다.

---

## 1. 접속 도메인 (10개)

| 도메인 | 대상 서비스 | 네임스페이스 | SSO 방식 |
|--------|-------------|--------------|----------|
| `nullus.internal` | nullus-web (포털) | nullus | **native OIDC** (Keycloak 로그인) |
| `api.nullus.internal` | nullus-api | nullus | **OIDC** (Keycloak JWT 검증) |
| `keycloak.nullus.internal` | keycloak (IdP) | nullus-auth | — (IdP 자신) |
| `grafana.nullus.internal` | kps-grafana | nullus-monitoring | native OIDC |
| `argocd.nullus.internal` | argo-cd-argocd-server | nullus | native OIDC |
| `harbor.nullus.internal` | harbor (nginx→core) | nullus | native OIDC |
| `gitlab.nullus.internal` | gitlab-webservice-default | gitlab | native OIDC (omniauth) |
| `minio.nullus.internal` | nullus-minio-console | nullus | native OIDC |
| `prometheus.nullus.internal` | kps-…-prometheus | nullus-monitoring | oauth2-proxy |
| `opensearch.nullus.internal` | opensearch-cluster-master | nullus | oauth2-proxy |

> ⚠️ `harbor` 라우트는 반드시 `harbor`(nginx) svc를 향해야 한다. `harbor-portal`(정적 SPA)로 가면 `/c/oidc/login`이 200(HTML)만 반환하고 OIDC가 동작하지 않는다.

---

## 2. 접속 전제

kind 환경에는 실 LoadBalancer/DNS가 없으므로 **2가지**가 필요하다.

1. **`/etc/hosts` 등록** — `*.nullus.internal` → `127.0.0.1`
   ```bash
   sudo bash airgap/scripts/24-register-hosts.sh
   # 해제: sudo REMOVE=1 bash airgap/scripts/24-register-hosts.sh
   ```
2. **게이트웨이 port-forward** — 단일 진입점(Envoy Gateway가 Host 헤더로 분기하므로 1개면 충분)
   ```bash
   # 특권포트 80 (브라우저가 호스트네임만으로 접근 가능, sudo 필요)
   sudo PORT=80 bash airgap/scripts/25-port-forward.sh
   # 또는 비특권 8080 → 접속 시 http://nullus.internal:8080/
   bash airgap/scripts/25-port-forward.sh
   ```

이후 브라우저에서 `http://grafana.nullus.internal/` 등으로 접속한다.

### 2.1 빠른 접속 스크립트 (권장)

`scripts/sso-access.sh` — Envoy Gateway svc 를 자동 탐지해 포워딩하고 접속 URL·계정을 출력한다.

```bash
# SSO 완전동작 (:80, sudo 필요)
sudo ./scripts/sso-access.sh

# 포털 미리보기만 (:8080, sudo 불필요 — SSO 로그인은 :80에서만)
PORT=8080 ./scripts/sso-access.sh
```

> Ctrl+C 로 종료. 창을 켜둔 채 브라우저로 접속한다. `/etc/hosts` 미등록 시 먼저
> `sudo bash airgap/scripts/24-register-hosts.sh` 실행.

**왜 :80 인가** — OIDC redirect 가 `http://keycloak.nullus.internal`(포트 없음 = :80)으로
가므로, :8080 으로 띄우면 포털은 보여도 로그인 버튼에서 `keycloak.nullus.internal:80` 연결거부가 난다.
SSO 무재인증 로그인은 반드시 :80.

### 2.2 SSO 동작 (무재인증 체인)

포털 또는 아무 OIDC 앱에서 한 번 Keycloak 로그인하면 그 브라우저의 Keycloak 세션
쿠키로 **나머지 앱은 재인증 없이** 통과한다. 검증됨(KC 1회 로그인 → 동일 세션):

| 앱 | 무재인증 결과 |
|----|---------------|
| 포털(nullus.internal) | Keycloak 로그인 → 포털 진입 (브라우저 확인) |
| Grafana | `/api/user` 200, admin@nullus.io |
| ArgoCD | `/api/v1/session/userinfo` loggedIn=true |
| Harbor | `/api/v2.0/users/current` 200 |
| MinIO | authorize→무재인증 code→`/oauth_callback` |
| Prometheus | `/` 200 (oauth2-proxy) |
| OpenSearch | `/` 200 (oauth2-proxy) |
| GitLab | 설정완료(sign_in에 Keycloak 버튼) — 브라우저에서 확인 |

> 포털(nullus-web)도 이제 Keycloak OIDC 로그인이다(이전 mock id/pw 아님).
> 포털 로그인이 Keycloak 세션을 생성하고, 이후 OSS 앱들이 SSO 로 통과한다.

---

## 3. SSO 동작 방식

- **IdP**: Keycloak, realm `nullus`. issuer `http://keycloak.nullus.internal/realms/nullus` (KC_HOSTNAME 고정).
- **포털(nullus-web)**: Keycloak **public client** `nullus-web`(PKCE S256) + react-oidc-context. 빌드 시 `VITE_OIDC_*` 주입(web/Dockerfile build-args). 로그인 후 access_token(JWT)을 api에 Bearer 전송.
- **api(nullus-api)**: `auth.mode=oidc`(deploy/helm/nullus/values.yaml). Keycloak JWT(RS256, JWKS) 검증. Bearer 없으면 401.
- **native OIDC (5종)**: grafana / argocd / harbor / gitlab / minio — 각 앱에 Keycloak confidential client + OIDC 설정. 앱이 직접 Keycloak로 redirect.
- **oauth2-proxy (2종)**: prometheus / opensearch — 네이티브 OIDC가 없어 oauth2-proxy를 앞단에 두고 게이트웨이 라우트를 oauth2-proxy로 연결.
- **PKCE**: grafana(`use_pkce=true`)·harbor·gitlab·nullus-web 은 PKCE(S256) 전송. **argocd 웹UI·minio 콘솔은 PKCE 미전송** → 해당 KC 클라이언트는 PKCE 요구 해제(`pkce.code.challenge.method=""`). 안 맞으면 `Missing parameter: code_challenge_method` 로 로그인 실패.
- **issuer 정합**: in-cluster 파드가 `keycloak.nullus.internal`을 해석하도록 **CoreDNS rewrite** 적용(`keycloak.nullus.internal → keycloak.nullus-auth.svc`). 브라우저(auth_url)·앱서버(token/userinfo) 양쪽이 동일 issuer 사용.
- **무재인증 흐름**: 포털 또는 한 앱에서 Keycloak 로그인 후 SSO 세션 쿠키 생성 → 다른 앱 접속 시 재인증 없이 통과.

**프로비저닝 스크립트** (멱등):
- `airgap/scripts/30-provision-sso.sh` — realm + native OIDC 5종 client + **nullus-web public client** + 테스트 사용자(admin@nullus.io, dev@nullus.io) + 시크릿 주입
- `airgap/scripts/31-oauth2-proxy.sh` — oauth2-proxy 2종 배포 + client
- 포털 OIDC 이미지: `.github/workflows/cd.yml` 의 web 빌드 `build-args`(VITE_*) 로 정적 번들에 OIDC 박힘

---

## 4. 테스트 계정

| 계정 | 비밀번호 | 역할 |
|------|----------|------|
| `admin@nullus.io` | `nullus123!` | admin |
| `dev@nullus.io` | `nullus123!` | developer |

(Keycloak `nullus` realm. `setup-keycloak.sh` / `30-provision-sso.sh`로 생성)

---

## 5. 검증 상태 (kind nullus-airgap, 게이트웨이 경유 실측)

| 앱 | request-phase(→Keycloak) | 무재인증 e2e |
|----|:---:|:---:|
| 포털(nullus-web) | ✅ | ✅ (브라우저 로그인 성공) |
| nullus-api | — | ✅ (Bearer 없음 401 / 유효 JWT 200) |
| grafana | ✅ | ✅ (`/api/user` 200) |
| argocd | ✅ | ✅ (`/api/v1/session/userinfo` loggedIn=true) |
| harbor | ✅ | ✅ (`/api/v2.0/users/current` 200) |
| minio | ✅ | ✅ (authorize→무재인증 code) |
| prometheus | ✅ (oauth2-proxy) | ✅ (`/` 200) |
| opensearch | ✅ (oauth2-proxy) | ✅ (`/` 200) |
| gitlab | ✅ (CSRF 포함 POST) | ⚠️ 브라우저 검증 필요(omniauth CSRF로 curl e2e 불가) |

- **무재인증 e2e 6/7 독립 확인** (`curl --connect-to` 로 KC 1회 로그인 → 동일 jar). GitLab 만 curl 한계로 브라우저 검증 필요.
- 검증 스크립트: `scripts/e2e-sso.sh`.

---

## 6. 트러블슈팅

- **`http://nullus.internal` 등 접속 거부(ERR_CONNECTION_REFUSED)**: `/etc/hosts` 매핑은 IP만 가리킬 뿐 서버를 띄우지 않는다. **게이트웨이 :80 port-forward(`sudo ./scripts/sso-access.sh`)가 떠 있어야** 한다. 그 sudo 창이 닫히면 모든 도메인 접속이 끊긴다.
- **포털이 Keycloak이 아니라 id/pw 폼으로 보임**: nullus-web 이 mock 빌드다. OIDC 빌드(`VITE_AUTH_MODE=oidc` 등 build-args)로 만든 이미지로 배포해야 한다(CD build-args 또는 web/Dockerfile build-arg).
- **`Missing parameter: code_challenge_method` (로그인 실패)**: KC 클라이언트가 PKCE 필수인데 앱이 code_challenge 미전송. 앱에서 PKCE 켜거나(grafana use_pkce), KC 클라이언트 PKCE 요구 해제(argocd/minio).
- **Keycloak 500 (`Table CLIENT not found`)**: 인메모리 DB(dev) 소실. Keycloak 재기동 후 realm 재프로비저닝.
- **OIDC issuer 불일치**: 토큰 iss와 앱 검증 URL이 달라 실패. KC_HOSTNAME=`keycloak.nullus.internal` + CoreDNS rewrite로 정합.
- **Harbor 200(리다이렉트 없음)**: harbor-route가 harbor-portal로 연결됨 → harbor(nginx) svc로 수정.
- **oauth2-proxy 이미지 부재**: 에어갭 번들에 미포함 → 온라인 1회 pull 후 `kind load docker-image`.

---

## 7. 관련 문서
- SSO 설계: `docs/20_개발가이드/Nullus_OSS_SSO_자동로그인_설계.md`
- OIDC Provider: `docs/20_개발가이드/Nullus_OIDC_Provider_가이드.md`
