# Nullus 접속 도메인 · SSO 가이드

> kind(airgap) 클러스터에 설치된 Nullus 플랫폼과 OSS 스택을 `*.nullus.internal` 도메인으로 접속하고, Keycloak SSO로 로그인하는 방법.
> **정본(authoritative source)**: 도메인·라우팅은 `airgap/scripts/23-setup-gateway.sh`(ROUTES 테이블)와 `airgap/scripts/24-register-hosts.sh`(/etc/hosts 등록)에 정의된다. 이 문서는 그 요약·운영 가이드다.

---

## 1. 접속 도메인 (10개)

| 도메인 | 대상 서비스 | 네임스페이스 | SSO 방식 |
|--------|-------------|--------------|----------|
| `nullus.internal` | nullus-web (포털) | nullus | (프론트, 별도) |
| `api.nullus.internal` | nullus-api | nullus | DualAuth(세션/OIDC) |
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

한 번 Keycloak 로그인(아무 OIDC 앱, 예: Grafana)하면 그 브라우저의 Keycloak 세션
쿠키로 **나머지 앱은 재인증 없이** 통과한다. 검증됨(KC 1회 로그인 → 동일 세션):

| 앱 | 무재인증 결과 |
|----|---------------|
| Grafana | `/api/user` 200, admin@nullus.io (브라우저 e2e 확인) |
| ArgoCD | `/api/v1/session/userinfo` loggedIn=true, admin@nullus.io |
| Harbor | `/api/v2.0/users/current` 200, admin@nullus.io |
| MinIO | authorize→무재인증 code→`/oauth_callback` |
| Prometheus | `/` 200 (oauth2-proxy) |
| OpenSearch | `/` 200 (oauth2-proxy) |
| GitLab | 설정완료(sign_in에 Keycloak 버튼) — 브라우저에서 확인 |

> 포털(nullus-web)은 현재 mock 모드라 포털 자체가 Keycloak 세션을 만들지 않는다.
> 첫 OIDC 앱 로그인이 세션을 생성하고, 이후 앱들이 SSO 로 통과한다.

---

## 3. SSO 동작 방식

- **IdP**: Keycloak, realm `nullus`. issuer `http://keycloak.nullus.internal/realms/nullus` (KC_HOSTNAME 고정).
- **native OIDC (5종)**: grafana / argocd / harbor / gitlab / minio — 각 앱에 Keycloak confidential client + OIDC 설정. 앱이 직접 Keycloak로 redirect.
- **oauth2-proxy (2종)**: prometheus / opensearch — 네이티브 OIDC가 없어 oauth2-proxy를 앞단에 두고 게이트웨이 라우트를 oauth2-proxy로 연결.
- **issuer 정합**: in-cluster 파드가 `keycloak.nullus.internal`을 해석하도록 **CoreDNS rewrite** 적용(`keycloak.nullus.internal → keycloak.nullus-auth.svc`). 브라우저(auth_url)·앱서버(token/userinfo) 양쪽이 동일 issuer 사용.
- **무재인증 흐름**: 한 앱에서 Keycloak 로그인 후 SSO 세션 쿠키 생성 → 다른 앱 접속 시 재인증 없이 통과.

**프로비저닝 스크립트** (멱등):
- `airgap/scripts/30-provision-sso.sh` — realm + native OIDC 5종 client + 시크릿 주입
- `airgap/scripts/31-oauth2-proxy.sh` — oauth2-proxy 2종 배포 + client

---

## 4. 테스트 계정

| 계정 | 비밀번호 | 역할 |
|------|----------|------|
| `admin@nullus.io` | `nullus123!` | admin |
| `dev@nullus.io` | `nullus123!` | developer |

(Keycloak `nullus` realm. `setup-keycloak.sh` / `30-provision-sso.sh`로 생성)

---

## 5. 검증 상태 (kind nullus-airgap, 게이트웨이 경유 실측)

| 앱 | request-phase(→Keycloak 302) | 무재인증 e2e |
|----|:---:|:---:|
| grafana | ✅ | (브라우저 필요) |
| argocd | ✅ | (브라우저 필요) |
| harbor | ✅ | ✅ (callback→/api/v2.0/users/current 200) |
| gitlab | ✅ (CSRF 토큰 포함 POST) | (브라우저 필요) |
| minio | ✅ (loginStrategy:redirect) | (브라우저 필요) |
| prometheus | ✅ (oauth2-proxy) | (브라우저 필요) |
| opensearch | ✅ (oauth2-proxy) | (브라우저 필요) |

- **request-phase 7/7 독립 확인**(게이트웨이 경유 Host 헤더 curl). 무재인증 e2e는 Harbor만 완주 확인 — 나머지는 `:80` port-forward + 브라우저에서 확인 가능.

---

## 6. 트러블슈팅

- **`http://*.nullus.internal` 접속 거부**: `/etc/hosts` 매핑은 IP만 가리킬 뿐 서버를 띄우지 않는다. `25-port-forward.sh`로 게이트웨이를 `127.0.0.1:80`에 연결해야 한다.
- **Keycloak 500 (`Table CLIENT not found`)**: 인메모리 DB(dev) 소실. Keycloak 재기동 후 realm 재프로비저닝.
- **OIDC issuer 불일치**: 토큰 iss와 앱 검증 URL이 달라 실패. KC_HOSTNAME=`keycloak.nullus.internal` + CoreDNS rewrite로 정합.
- **Harbor 200(리다이렉트 없음)**: harbor-route가 harbor-portal로 연결됨 → harbor(nginx) svc로 수정.
- **oauth2-proxy 이미지 부재**: 에어갭 번들에 미포함 → 온라인 1회 pull 후 `kind load docker-image`.

---

## 7. 관련 문서
- SSO 설계: `docs/20_개발가이드/Nullus_OSS_SSO_자동로그인_설계.md`
- OIDC Provider: `docs/20_개발가이드/Nullus_OIDC_Provider_가이드.md`
