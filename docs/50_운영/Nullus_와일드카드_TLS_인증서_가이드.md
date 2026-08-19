# Nullus 와일드카드 TLS 인증서 가이드

> 공인 도메인 `nullus.io` 환경에서 `*.nullus.io` 인증서 한 장으로 플랫폼과 OSS 스택 전체의 HTTPS를 덮는 방법.
> **정본**: `deploy/csp/zadara/setup-tls.sh`. 이 문서는 그 스크립트의 배경·절차·트러블슈팅이다.
> kind/airgap 환경(`*.nullus.internal`, HTTP, `/etc/hosts`)은 `docs/20_개발가이드/Nullus_접속_도메인_SSO_가이드.md` 를 본다.

---

## 1. 무엇을 해결하는가

HTTPS는 선택이 아니다. OIDC Authorization Code + PKCE가 `crypto.subtle`을 쓰는데 브라우저는 secure context에서만 이를 노출한다. 평문 HTTP에서는 로그인 화면이 이렇게 끝난다.

```
로그인 오류: Crypto.subtle is available only in secure contexts (HTTPS).
```

그런데 스택이 노출하는 호스트는 한둘이 아니다. `manifest-builders.go` 가 만드는 것만 해도 열둘이다.

```
argocd. gitlab. gitea. jenkins. grafana. prometheus.
harbor. nexus. minio. openbao. opensearch. registry.
```

호스트마다 인증서를 따로 받으면 **도구를 하나 늘릴 때마다 발급이 하나 늘고**, 그중 하나가 실패하면 그 도구만 조용히 접속 불가가 된다. 와일드카드 한 장이면 이 문제가 사라진다.

---

## 2. 왜 DNS-01인가 — 선택의 여지가 없다

**와일드카드는 HTTP-01로 발급할 수 없다.** ACME 규격이 그렇다. `*.nullus.io` 를 받으려면 DNS-01 뿐이다.

DNS-01은 `_acme-challenge.nullus.io` 에 TXT를 넣고 지우는 검증이므로, **존을 프로그램으로 쓸 수 있어야 한다.** 즉 DNS 제공자 API 자격증명이 필요하다.

### 2.1 왜 lego 웹훅인가 (위임하지 않는 이유)

cert-manager가 내장한 DNS-01 솔버는 여섯뿐이고 우리 등록기관(Spaceship)이 없다.

```
Route53 · Cloudflare · DigitalOcean · AzureDNS · acme-dns · RFC2136
```

흔한 우회는 `_acme-challenge` 를 지원되는 존으로 CNAME **위임**하는 것이다. 이 방법은 채택하지 않았다.

- 남의 DNS 계정(AWS 등)이 발급 경로에 하나 더 끼고, 그 계정이 죽으면 갱신이 멈춘다
- cert-manager [#5751](https://github.com/cert-manager/cert-manager/issues/5751)(와일드카드 + `cnameStrategy: Follow`)을 정면으로 밟는다
- Cloudflare 무료 플랜은 서브도메인을 존으로 못 올려, 두 번째 도메인이나 NS 이전이 필요하다

대신 **lego가 Spaceship을 1급 프로바이더로 지원한다**(v4.22.0+). lego를 감싼 웹훅을 붙이면 위임도, 네임서버 이전도, 제3자 계정도 없이 끝난다.

```
cert-manager ──webhook──> lego(spaceship) ──API──> Spaceship 존
```

### 2.2 왜 HTTP-01을 남기는가

`121.78.39.184.nip.io` 처럼 **와일드카드가 덮지 못하는 이름**이 아직 ingress에 있다. 그것까지 DNS-01로 보내면 nip.io 존의 TXT를 우리가 쓸 수 없어 발급이 통째로 막힌다. 그래서 존별로 나눈다 — cert-manager는 더 구체적인 `selector.dnsZones` 를 우선하므로 두 솔버가 한 ClusterIssuer에 공존해도 서로를 가리지 않는다.

```yaml
solvers:
  - http01: {ingress: {class: nginx}}      # nip.io 등 나머지
  - dns01:  {webhook: {...}}
    selector: {dnsZones: [nullus.io]}      # nullus.io 만
```

---

## 3. 사전 준비

### 3.1 DNS 레코드 (Spaceship 콘솔)

Advanced DNS에 A 레코드 하나를 더한다.

| Type | Host | Value |
|---|---|---|
| `A` | `*` | 공인 IP (예: `121.78.39.184`) |

**기존 `@` · `www` · `auth` 는 지우지 않는다.**

- `@`(apex)는 **필수다.** 와일드카드는 apex를 덮지 않는다(RFC 4592 §2.1.3 — `*.nullus.io` 는 앞에 라벨이 최소 하나 있어야 매칭). 지우면 `nullus.io` 자체가 NXDOMAIN이 된다
- `www` · `auth` 는 와일드카드가 덮으므로 기능상 지워도 되지만 남긴다. 와일드카드가 사고로 지워졌을 때 `auth`(Keycloak)가 죽으면 **로그인 전체가 죽는다.** 레코드 한 줄이 그 단일 실패점을 막는다
- 콘솔 목록에 안 보이는 **MX · SPF TXT**(Spaceship 이메일 포워딩)가 별도로 있다. 지우면 `admin@nullus.io` 수신이 끊기고 Let's Encrypt 만료 알림도 사라진다

> **와일드카드의 매칭 범위 — 흔한 오해**
> DNS에서 `*.nullus.io` 는 중간에 다른 노드만 없으면 **몇 단계든** 매칭된다(`a.b.nullus.io` 도 응답한다). 한 레벨 제한은 DNS가 아니라 **TLS 인증서** 쪽 규칙이다(RFC 6125). 그래서 `ACCESS_DOMAIN` 은 `nullus.io` 로만 쓴다 — `poc.nullus.io` 를 주면 라우트가 `argocd.poc.nullus.io`(2단계)가 되어 `*.nullus.io` 인증서가 덮지 못한다.

### 3.2 API 키 (Spaceship API Manager)

**이름**: `certmanager-dns01-<환경>` (예: `certmanager-dns01-nullus-prod`)
소비자를 앞에 두면 목록이 소비자별로 정렬되고, 1년 뒤 이 키를 폐기해도 되는지 이름만으로 판단할 수 있다.

**스코프**: 아래 둘만. 나머지(Domains·Contacts·SellerHub·Hyperlift)는 전부 끈다.

```
dnsrecords:write   ← 필수. Read 만으로는 동작하지 않는다
dnsrecords:read    ← 권장 (client 에 GetRecords 가 있어 향후 버전 대비)
```

lego 소스(`providers/dns/spaceship/`)를 확인한 결과 호출 경로는 셋뿐이고 `/domains` 는 건드리지 않는다. 존 판별은 API가 아니라 SOA 조회(`dns01.FindZoneByFqdn`)로 한다.

```
Present  → PUT    /v1/dns/records/{zone}   (TXT 추가)
CleanUp  → DELETE /v1/dns/records/{zone}   (TXT 제거)
           GET    /v1/dns/records/{zone}
```

> ⚠️ `dnsrecords:write` 는 계정 단위 스코프로 보인다. 같은 Spaceship 계정에 다른 도메인이 있으면 이 키가 그 도메인의 DNS도 바꿀 수 있다. 발급 화면에 도메인 제한 옵션이 있으면 반드시 좁힌다. 이 키가 유출되면 MX 변조를 통한 계정 복구 가로채기까지 간다.

### 3.3 시크릿 등록

**자격증명은 스크립트가 만들지 않는다.** 키가 코드·셸 히스토리·CI 로그를 통과하지 않게 하기 위해서다.

```bash
kubectl -n cert-manager create secret generic spaceship-dns01 \
  --from-literal=SPACESHIP_API_KEY='...' \
  --from-literal=SPACESHIP_API_SECRET='...'
```

**키 이름을 한 글자도 바꾸면 안 된다.** 웹훅은 시크릿의 키를 그 이름 그대로 환경변수로 lego에 주입한다. `api-key` 같은 이름으로 넣으면 lego가 자격증명을 못 찾고 **챌린지가 에러 없이 pending으로 굳는다** — 로그 어디에도 원인이 안 나온다. 스크립트가 이 두 이름을 검사하므로 잘못 넣으면 Let's Encrypt를 건드리기 전에 걸린다.

---

## 4. 발급 절차

### 4.1 staging 먼저

Let's Encrypt는 등록 도메인당 **주 50건**이다. 시행착오로 태우면 일주일을 기다린다. 경로를 처음 뚫을 때는 반드시 staging부터 돌린다.

```bash
export KUBECONFIG=~/.kube/nullus-zadara.conf

DNS01_ZONE=nullus.io ISSUER=letsencrypt-staging ./deploy/csp/zadara/setup-tls.sh install
DNS01_ZONE=nullus.io ISSUER=letsencrypt-staging ./deploy/csp/zadara/setup-tls.sh wildcard
DNS01_ZONE=nullus.io ./deploy/csp/zadara/setup-tls.sh status
```

`install` 은 lego 웹훅(helm, 버전 고정)을 깔고 ClusterIssuer를 만든다. `wildcard` 는 `*.<존>` 과 `<존>` 두 이름을 한 장에 담은 Certificate를 만든다. **둘 다 적는 이유는 와일드카드가 apex를 덮지 않기 때문이다.**

발급되면 인증서 내용을 확인한다.

```bash
kubectl -n nullus get secret nullus-wildcard-tls -o 'jsonpath={.data.tls\.crt}' \
  | base64 -d | openssl x509 -noout -subject -issuer -ext subjectAltName
```

```
subject=CN=*.nullus.io
issuer=C=US, O=Let's Encrypt, CN=(STAGING) Dastardly Durum YR1   ← staging 확인
X509v3 Subject Alternative Name: DNS:*.nullus.io, DNS:nullus.io
```

### 4.2 prod 전환

staging 산출물을 지우고 `ISSUER` 없이 같은 명령을 돌린다.

```bash
kubectl -n nullus delete certificate nullus-wildcard --ignore-not-found
kubectl -n nullus delete secret nullus-wildcard-tls --ignore-not-found

DNS01_ZONE=nullus.io ./deploy/csp/zadara/setup-tls.sh install
DNS01_ZONE=nullus.io ./deploy/csp/zadara/setup-tls.sh wildcard
```

> **prod `install` 은 살아 있는 `letsencrypt` ClusterIssuer를 수정한다.** 먼저 기존 spec과 대조한다 — 특히 `privateKeySecretRef` 가 다르면 **새 ACME 계정이 만들어져** rate limit 이력이 리셋된다.
> ```bash
> kubectl get clusterissuer letsencrypt \
>   -o jsonpath='{.spec.acme.email}{"\n"}{.spec.acme.server}{"\n"}{.spec.acme.privateKeySecretRef.name}{"\n"}'
> ```
> 세 값이 스크립트 기본값(`ACME_EMAIL` / prod 서버 / `letsencrypt-account-key`)과 같으면 dns01 솔버만 추가되고 계정은 보존된다.

### 4.3 Ingress에 연결

**여기까지는 인증서를 만들기만 한 것이다.** 어느 Ingress에도 붙이지 않았으므로 브라우저에는 아직 나타나지 않는다. 연결은 `values-zadara.yaml` 의 `ingress.tls` 를 와일드카드 시크릿으로 바꾸고 `helm upgrade` 를 돌리는 별도 단계이며, **살아 있는 사이트의 인증서를 실제로 갈아끼우는 유일한 지점**이다. 서두를 이유가 없다면 기존 인증서 만료 전에 여유를 두고 한다.

---

## 5. 검증

### 5.1 DNS

```bash
dig +short @launch1.spaceship.net argocd.nullus.io          # → 공인 IP
dig +short @launch1.spaceship.net zzz-없는이름.nullus.io      # → 공인 IP (와일드카드 동작)
dig +noall +answer @launch1.spaceship.net '*.nullus.io' A
```

### 5.2 자격증명·스코프·클러스터 egress (한 번에)

로컬이 아니라 **클러스터 안에서** 확인한다. 웹훅 파드가 `spaceship.dev` 에 닿아야 하므로 egress까지 함께 검증된다. 값은 파드 안에만 머문다.

```bash
kubectl -n cert-manager run spaceship-probe --restart=Never --image=curlimages/curl:8.11.1 \
  --overrides='{"spec":{"containers":[{"name":"spaceship-probe","image":"curlimages/curl:8.11.1",
  "envFrom":[{"secretRef":{"name":"spaceship-dns01"}}],
  "command":["sh","-c","curl -sS -o /dev/null -w \"HTTP=%{http_code}\\n\" -H \"X-API-Key: $SPACESHIP_API_KEY\" -H \"X-API-Secret: $SPACESHIP_API_SECRET\" https://spaceship.dev/api/v1/dns/records/nullus.io?take=1"]}]}}'
kubectl -n cert-manager logs spaceship-probe
kubectl -n cert-manager delete pod spaceship-probe
```

`HTTP=200` 이면 자격증명·읽기 스코프·egress 셋 다 정상이다. 쓰기 스코프는 실제 발급에서 확인된다.

### 5.3 발급 중 실시간 추적

```bash
watch -n3 '
  kubectl -n nullus get certificate nullus-wildcard
  kubectl -n nullus get challenge
  dig +short @launch1.spaceship.net _acme-challenge.nullus.io TXT
'
```

챌린지는 **2건**이 뜬다 — `*.nullus.io` 와 `nullus.io` 가 **같은** `_acme-challenge.nullus.io` 를 쓰기 때문이다. cert-manager가 순차 처리하므로(`valid` → `pending`) 서로 덮어쓰지 않는다. 발급이 끝나면 TXT는 **0건**으로 정리된다. 남아 있으면 CleanUp이 실패한 것이다.

### 5.4 실측 (2026-08-19, zadara platform 클러스터)

| 항목 | 결과 |
|---|---|
| Spaceship API probe (클러스터 내부) | `HTTP=200`, 345ms |
| lego 웹훅 APIService | `v1alpha1.lego.dns-solver` = True |
| staging 발급 | 102초 |
| prod 발급 | 114초 |
| TXT 쓰기 / 정리 | 정상 (발급 후 잔여 0건) |
| 기존 서비스 영향 | 없음 (`nullus.io` 200 · `www` 200 · `auth` 302 · nip.io 200) |

---

## 6. 운영

**갱신은 자동이다.** cert-manager가 만료 30일 전에 같은 경로로 다시 발급한다. 사람이 할 일은 없고, 다만 API 키가 폐기되면 그때 조용히 멈추므로 키를 회전할 때는 시크릿을 함께 갱신한다.

```bash
DNS01_ZONE=nullus.io ./deploy/csp/zadara/setup-tls.sh status     # 상태
./deploy/csp/zadara/setup-tls.sh uninstall                        # 되돌리기
```

`uninstall` 은 cert-manager와 lego 웹훅을 지우되 **자격증명 시크릿은 남긴다.** 재설치 때 키를 다시 발급받게 만들 이유가 없다.

---

## 7. 트러블슈팅

| 증상 | 원인 | 조치 |
|---|---|---|
| 챌린지가 **에러 없이** pending으로 굳음 | 시크릿 키 이름이 `SPACESHIP_API_KEY`/`SPACESHIP_API_SECRET` 이 아님 | 시크릿 재생성. 스크립트 `install` 이 미리 잡아준다 |
| TXT가 존에 안 생김 | `dnsrecords:write` 미부여 (Read만 줌) | API Manager에서 Write 추가 |
| 웹훅 파드는 뜨는데 챌린지 실패 | 클러스터에서 `spaceship.dev` egress 차단 | §5.2 probe로 확인 |
| `nullus.io` 가 NXDOMAIN | apex `@` 레코드를 지움 | 와일드카드는 apex를 안 덮는다. `@` 복구 |
| 2단계 호스트에서 인증서 경고 | `*.nullus.io` 는 한 레벨만 검증 (RFC 6125) | `ACCESS_DOMAIN` 을 `nullus.io` 로 |
| 발급은 됐는데 브라우저에 안 나옴 | Ingress에 연결하지 않음 (§4.3) | `ingress.tls` 교체 + `helm upgrade` |
| 스택 호스트가 404 | 앱이 설치되지 않았거나 라우트 없음 | 인증서와 무관. 스택 설치 상태 확인 |
| `connection refused` on `127.0.0.1:16443` | SSH 터널이 죽음 | `./deploy/csp/zadara/kubeconfig.sh` 재실행 |

---

## 8. 제약과 남은 일

- **Ingress 연결 미실행** — §4.3. 인증서는 발급되어 시크릿에 대기 중이다
- **라우팅 경로가 두 갈래다** — 제품이 만드는 라우트는 Gateway API(HTTPRoute + envoy, `nullus-wildcard-tls` 참조)인데 zadara 클러스터는 nginx Ingress(`ingress.tls`)로 돈다. 스택 호스트가 실제로 뜨려면 이 정합이 먼저다
- **lego 웹훅은 소규모 서드파티다** (star 23, 라이선스 표기 없음). 자체 운영에는 쓰되 **제품 차트에 넣어 재배포하지 않는다** — Apache-2.0 재배포에 걸린다
- **메일 발신 경로 없음** — Spaceship 이메일 포워딩은 수신 전용이다. 초대 메일(`Nullus_사용자_초대_메일_설계.md`)에서 SES/SendGrid를 붙이면 SPF TXT에 `include` 를 추가해야 한다. 안 하면 스팸함으로 간다

---

## 9. 관련 문서

- 스크립트 정본: `deploy/csp/zadara/setup-tls.sh`
- 클러스터 접속(SSH 터널): `deploy/csp/zadara/kubeconfig.sh`
- kind/airgap 도메인·SSO: `docs/20_개발가이드/Nullus_접속_도메인_SSO_가이드.md`
- zadara 환경 전제: `docs/50_운영/zadara_cloud_poc.md`
- 초대 메일 설계: `docs/20_개발가이드/Nullus_사용자_초대_메일_설계.md`
