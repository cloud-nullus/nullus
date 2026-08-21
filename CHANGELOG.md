# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **설치 로그가 재시작을 견딘다** (`db/migrations/000074_stack_deploy_logs.up.sql` 신규, `internal/stack/adapter/log/persistent_streamer.go` 신규, `internal/stack/adapter/repository/postgres_deploy_log.go` 신규): 지금까지 설치 로그는 API 프로세스 메모리에만 있었다. 파드가 재시작되면 통째로 사라져, 무엇이 왜 멈췄는지 사후에 알 방법이 없었다 — 설치는 20~30분짜리라 그 사이 재시작이 겹칠 확률이 낮지 않다.

  이제 같은 항목을 DB 에도 남긴다. 실시간 팬아웃은 그대로 메모리가 맡고, 재접속했을 때 메모리 이력이 비어 있으면 — 즉 재시작 뒤에만 — 저장소에서 읽어 재생한다. 이 프로세스가 그 배포를 스트리밍했으면 메모리가 진실이다. 겹쳐 읽으면 같은 줄이 두 번 보인다.

  저장 실패는 스트리밍을 막지 않는다. 로그를 남기지 못하는 것이 설치를 멈출 이유는 아니다 — 실패 사실만 서버 로그에 남기고 설치는 계속된다. 저장소 호출에는 3초 상한을 둔다. 곁가지가 본체를 멈추면 안 된다.

  `ClearHistory` 가 실제로 동작하게 됐다. 메모리 스트리머에 그 메서드가 없어 호출부의 타입 단언이 늘 실패했고, 그래서 새 실행이 이전 실행의 로그 위에 겹쳐 쌓였다. 이제 메모리와 저장소 양쪽을 지운다.

- **배포 진행 막대가 차오르고, 끝나면 로켓이 날아간다** (`web/src/features/stack/utils/deploy-progress.ts` 신규, `web/src/features/stack/components/deploy-progress-bar.tsx` 신규, `web/src/index.css`): 배포 화면의 진행 막대가 멈춰 있는 것처럼 보였다. 애니메이션이 아니라 **데이터**가 문제였다 — 서버 진행률은 단계가 바뀔 때만 뛴다(5 → 15 → 90 → 96 → 100). 설치는 15 와 90 사이에서 대부분의 시간을 보내므로, 잘 돌고 있는 배포가 몇 분 동안 한 픽셀도 움직이지 않았다.

  그래서 표시용 값을 따로 둔다. 실제 값이 앞서면 그쪽으로 빠르게 따라붙고, 값이 멈춰 있는 동안에는 다음 이정표를 향해 천천히 차오른다. **다만 이정표에는 닿지 않는다** — 닿으면 아직 시작하지도 않은 단계를 끝난 것처럼 보여 주는 거짓말이 된다. 되감기지 않고, 실패하면 그 자리에 멈춘다. 이정표는 화면의 단계 정의에서 끌어온다(따로 적으면 단계를 늘렸을 때 막대만 거짓말을 한다).

  막대도 1% 짜리 칸 100 개를 색만 바꿔 칠하던 것에서 한 덩어리가 부드럽게 늘어나는 형태로 바꿨고, 채워진 면 위로 빛이 한 번씩 지나가 잠깐 멈춰도 살아 있어 보인다.

  **로켓은 막대 앞머리를 타고 간다.** 배포가 끝나면 오른쪽 위로 회전하며 사라진다 — 100% 에서 그냥 멈추는 것보다 끝났다는 것이 한눈에 읽힌다. 동체는 막대와 같은 그라디언트라 막대에서 뽑혀 나온 것처럼 보이고, 불꽃은 바깥에서 안으로 갈수록 뜨거워지는 3겹(빨강 → 주황 → 흰 심지)이다. 실패하면 불꽃이 꺼지고 회색으로 멈춘다.

  불꽃에 `--color-error` 를 쓰지 않았다. 그 빨강은 이 화면에서 "실패" 를 뜻하고 실패한 배포의 막대가 바로 그 색이다 — 잘 가고 있는 로켓의 불꽃이 같은 빨강이면 오류처럼 읽힌다. 금속 하이라이트·엔진 링에도 `--color-surface-card` 대신 흰빛을 쓴다. 그 토큰은 다크 테마에서 near-black 이라 하이라이트가 아니라 **검은 이음매**로 보였다.

  접근성도 함께 챙겼다 — 옛 막대에는 `role="progressbar"` 가 없어 스크린리더가 진행률을 읽지 못했다. `prefers-reduced-motion` 에서는 불꽃·빛·비행을 모두 끈다.

- **스택 도구가 밖에서 열린다** (`internal/stack/adapter/helm/gateway-bridge.go` 신규): 운영에서 `gitlab.nullus.io` 같은 스택 도구 주소가 404 로 끝났다. Zadara 에는 LoadBalancer 연동이 없어 Gateway 의 Service 가 외부 IP 를 영원히 받지 못하고, 밖에서 유일하게 열린 ingress-nginx 에는 그 호스트 규칙이 없었다. `*.nullus.io` DNS 는 이미 같은 공인 IP 로 잡혀 있어 **요청은 도착하는데 받아 줄 규칙이 없었다.**

  설치가 게이트웨이를 세운 뒤 **브리지 Ingress 를 스택 네임스페이스에 함께 만든다.** 와일드카드 호스트를 받아 그 스택의 Envoy 데이터플레인으로 넘기고, 원래 Host 는 `upstream-vhost` 로 보존한다 — 게이트웨이의 HTTPRoute 가 호스트로 도구를 갈라내기 때문이다. Envoy Gateway 가 만드는 Service 이름에는 해시가 붙어 미리 적을 수 없으므로, 실제로 생긴 것을 조회해 쓴다.

  **게이트웨이·Envoy·라우트·인증서·브리지가 모두 스택 네임스페이스 한자리에 모인다.** Ingress 의 백엔드는 같은 네임스페이스여야 하는데 그 제약이 저절로 풀리고, 스택을 지우면 배선도 함께 사라진다. TLS 는 ingress-nginx 의 기본 인증서가 처리하므로 스택마다 시크릿을 복사할 필요가 없다.

  접속 도메인이 `.internal` 이면 만들지 않는다 — 로컬(kind)에는 ingress 컨트롤러가 없고 포트포워드로 게이트웨이에 직접 붙는다. 브리지를 걸지 못해도 설치는 멈추지 않는다. 클러스터 안에서는 스택이 정상 동작하고, 그 배선은 나중에 손으로도 걸 수 있다.

- **모니터링 대시보드가 스택 컴포넌트 주소를 미리 채운다** (`internal/stack/domain/tool_access_url.go` 신규, `internal/stack/adapter/handler/monitoring_handler.go`, `web/src/features/observability/components/monitoring-connect-panel.tsx`): "Connect Stack Components" 는 `Tools detected in <스택>` 이라고 써 놓고 실제로는 아무것도 감지하지 않았다. 도구 6개와 그 상태·버전이 화면에 상수로 박혀 있어, 깔리지도 않은 Kibana 가 running 으로 뜨고 Grafana 는 무슨 스택을 골라도 warning 이었다. 주소도 설치할 때 이미 받아 둔 접속 도메인을 두고 사용자에게 다시 받아 적게 했다.

  목록은 이제 스택 모니터링 응답에서 온다 — 무엇이 설치되는지는 `domain.InstalledToolWorkloads` 한 곳이 정하므로 스택 상세 화면과 같은 답을 본다. 상태·버전도 실제 값이고, 주소는 서버가 접속 도메인에서 만들어 준 값으로 미리 채운 뒤 고쳐 쓸 수 있다 — 게이트웨이 앞에 다른 주소를 두는 설치가 있으므로 미리 채운 값은 출발점이지 강제가 아니다.

  **주소 규칙을 한 곳으로 모았다.** 같은 질문("Grafana 는 어디로 들어가는가")에 화면마다 다르게 답하고 있었다 — 스택 상세는 `http://grafana.<도메인>`, 임베드 탭은 스킴 없는 값에 `https` 를 덧붙이고, 설치 경로(`toolURLScheme()`)는 SSO 여부로 갈랐다. 접속 도메인은 게이트웨이 TLS 리스너 뒤에 서므로 `domain.ToolAccessURL` 에서 **https 로 고정**하고, 서버가 `oss_statuses[].url` 로 확정해 내려준다. 화면의 `toolLaunchURL` 은 서버가 아직 답하지 못할 때의 대비책으로만 남는다. 설치 목록에 오르는 OSS 가 전부 주소를 갖는지는 테스트로 고정했다 — `InstalledToolWorkloads` 에 도구를 추가하면서 호스트 규칙을 빠뜨리면 화면에는 뜨는데 주소만 빈 항목이 생긴다.

  **기본 동작은 새 창으로 여는 것이다.** Grafana·Argo CD·Harbor 처럼 대부분의 OSS 는 `X-Frame-Options` 로 iframe 을 막아서, 주소를 누가 넣든 임베드 탭은 빈 화면이 된다. 그래서 임베드는 체크한 도구만 탭으로 만든다. 임베드가 실제로 되게 하려면 차트에 `allow_embedding` 을 켜야 하고, 그건 아직 없다.
- **로그인 화면 파비콘을 Nullus 마크로** (`.../login/resources/img/favicon.ico` 신규, `scripts/make-favicon.mjs` 신규, ConfigMap 템플릿·`values.yaml`): 탭에 키클록 로고가 떴다. 부모 테마(keycloak.v2)의 템플릿이 `<link rel="icon" href="${resourcesPath}/img/favicon.ico">` 를 박아 넣는데, 우리 테마에 그 파일이 없어 Keycloak 이 부모 테마의 것으로 폴백하기 때문이다. 템플릿은 복사하지 않는 방침이라, 링크가 가리키는 그 자리에 파일을 두는 것이 유일하게 자바스크립트 없이 듣는 방법이다.

  `knot.svg` 를 16·32·48px PNG 로 구워 ICO 한 장에 담는다(`node scripts/make-favicon.mjs`). 결과물은 커밋한다 — 아이콘 하나 때문에 브라우저를 CI 의 필수 의존성으로 만들 이유가 없다.

  **두 가지가 걸림돌이었고 둘 다 배선으로 풀었다.** (1) `.ico` 는 바이너리라 ConfigMap 의 `data`(문자열)로는 못 싣는다 → `binaryData` 로 base64 로 싣는다. (2) ConfigMap 키에는 `/` 를 못 넣어 `img/` 를 만들려면 볼륨을 중첩 마운트해야 한다 → `projected` 볼륨으로 두 ConfigMap 을 합치고 `items[].path` 에 `img/favicon.ico` 를 준다. 마운트 지점은 그대로 하나다.

  **테스트도 그 자리까지 따라가게 고쳤다.** 종전에는 마운트가 있는지만 봤는데, 그러면 하위 폴더의 파일이 한 단 위에 평평하게 떨어져도 통과한다. 이제 마운트 → 볼륨 → ConfigMap 키/`items[].path` 를 따라가 파드 안에 실제로 생길 경로를 만들어 대조한다. `items` 를 빼거나, ConfigMap 을 볼륨에서 빼거나, `.ico` 를 문자열로 실으면 각각 실패한다(셋 다 확인).

- **로그인 테마를 Keycloak 이 보지 않는 자리에 얹고 있던 것** (`deploy/helm/nullus/values.yaml`, `deploy/helm/keycloak_theme_test.go`): realm 에 `loginTheme=nullus` 를 걸어도 화면은 기본 테마 그대로였다. 차트는 테마를 `/opt/keycloak/themes/` 에 마운트하는데, 차트가 띄우는 이미지는 `bitnamilegacy/keycloak` 이라 테마를 `/opt/bitnami/keycloak/themes/` 에서 찾는다. 파일은 컨테이너 안에 있는데 Keycloak 이 보지 않는 자리였고, 못 찾은 테마는 오류 없이 기본 화면으로 되돌아간다.

  로컬에서 멀쩡했던 건 `docker-compose.dev.yaml` 이 공식 `quay.io/keycloak` 이미지를 쓰기 때문이다 — 그쪽은 `/opt/keycloak` 이 맞다. 두 환경이 서로 다른 이미지 계열을 쓰는 줄 모르고 로컬 경로를 차트에 그대로 옮겼다.

  **테스트가 이걸 놓친 이유도 같이 고쳤다.** `keycloak_theme_test.go` 는 컨테이너 경로를 상수로 박아 두고 있었다. 그러면 ConfigMap 과 마운트가 *서로만* 맞으면 초록불이 나고, 정작 실제 이미지와 어긋난 것은 잡지 못한다. 이제 `keycloak.image.repository` 에서 경로를 끌어낸다 — 경로를 되돌리거나 이미지 계열만 바꿔도 실패한다(양쪽 다 확인).

- **로그인 테마가 배포에 따라 붙는다** (`.github/workflows/cd.yml`, `deploy/csp/zadara/setup-keycloak-realm.sh` 에 `theme` 하위 명령, `scripts/setup-keycloak.sh` 의 `KEYCLOAK_SSL_REQUIRED`): 차트를 배포해도 `https://auth.nullus.io` 는 기본 Keycloak 화면 그대로였다. 차트는 테마 **파일**을 ConfigMap 으로 실어 줄 뿐이고, "그 테마를 쓰라" 고 realm 에 적는 것은 별개인데 그건 사람이 `setup-keycloak-realm.sh` 를 손으로 돌릴 때만 일어났기 때문이다. 실제로 서빙되는 페이지가 `login/keycloak.v2` 를 참조하는 것으로 확인했다.

  `helm upgrade` 뒤에 realm 의 `loginTheme` 과 언어를 거는 단계를 CD 에 넣었다. setup 전체를 돌리면 클라이언트·사용자까지 건드리므로 그 부분만 `theme` 하위 명령으로 떼어 냈다 — 여러 번 돌려도 결과가 같다. realm 은 차트가 만들지 않으므로 아직 없으면 건너뛰고, Keycloak 파드가 없는 구성(외부 Keycloak)에서도 배포를 세우지 않는다.

  **덤으로 평문 허용 되돌림을 막았다.** `setup-keycloak.sh` 는 realm 을 `sslRequired=none` 으로 PUT 한다 — 로컬 dev 전제다. 그런데 zadara 스크립트가 이걸 그대로 호출하고 있어서, 배포 realm 을 손볼 때마다 평문 허용으로 되돌아가고 있었다. `KEYCLOAK_SSL_REQUIRED` 로 밖에서 정하게 하고 배포 경로는 `external` 을 넘긴다.

- **Keycloak 로그인 화면을 제품 화면으로 바꾼다** (`deploy/helm/nullus/files/keycloak-theme/` 신규, `scripts/emit-keycloak-theme-art.py` 신규, `keycloak-theme-configmap.yaml` 신규): 기본 Keycloak 로그인 화면은 이 제품이 무엇인지 한 글자도 말해 주지 않는다. 왼쪽에 파이프라인을 그림으로 설명하고 오른쪽에 로그인 폼을 두는 두 단 화면으로 바꿨다 — 검증된 OSS 를 컨베이어에 실어 Nullus 게이트로 넣으면 반대편으로 CI/CD 스택이 나오고 그 위에 애플리케이션이 얹힌다.

  **그림은 손으로 그리지 않고 생성기가 뽑는다.** 아이소메트릭은 좌표 하나가 어긋나면 조각들이 서로 떠 버리는데, SVG 를 손으로 고치면 CSS 가 얹는 움직이는 조각의 좌표와 조용히 어긋난다. 그래서 `emit-keycloak-theme-art.py` 가 정지 그림(SVG)과 조각 자리·안무(CSS 변수·keyframes)를 **같은 계산에서** 함께 뽑는다. 장면을 옮기면 조각이 따라온다.

  생성기는 눈으로 봐야만 알 수 있는 것들을 불변식으로 들고 있다 — 집게 가로대가 컨테이너 뚜껑을 가로지르지 않는지, 집게발이 옆변 한가운데에 붙는지, 표시등이 화물이 지나는 길·모니터와 화면에서 겹치지 않는지. 겹침은 경계상자가 아니라 **실루엣(볼록 육각형) + 분리축 정리**로 잰다 — 경계상자로 재면 아이소메트릭 상자의 빈 모서리까지 겹친 것으로 쳐서 벨트 옆에는 놓을 자리가 없다고 나온다.

  **움직이는 것은 전부 DOM 요소다.** WebKit 은 CSS 배경 이미지 속 SVG 애니메이션을 정지 프레임으로 렌더해서, SVG 안에 넣으면 사파리에서만 멈춘 채로 보인다.

  테마 파일은 ConfigMap 세 개로 나눠 실어 Keycloak 에 마운트한다(root/resources/messages). `deploy/helm/keycloak_theme_test.go` 가 `helm template` 결과를 뜯어 **파일이 하나도 빠지지 않고 Keycloak 이 찾는 자리에 그대로 도착하는지**, 볼륨이 실재하는 ConfigMap 을 가리키는지, 그림이 최신인지(생성기를 다시 돌려 대조) 검사한다.

  realm 기본 언어를 `en` 으로 바꿨다(`KEYCLOAK_DEFAULT_LOCALE` 로 덮어쓸 수 있다). 한국어·영어 두 벌을 다 싣는다.

- **HTTP 를 HTTPS 로 강제한다** (`deploy/csp/zadara/setup-tls.sh`, `default-cert` → `ingress-https` 로 이름 변경): 기본 인증서를 걸면 새 호스트도 인증서를 받지만, **평문 HTTP 가 그대로 200 을 반환**한다. tls 섹션 없는 Ingress 를 실제로 만들어 확인했다 — `https://` 는 200 에 `*.nullus.io`, `http://` 는 200 에 리다이렉트 없음.

  ingress-nginx 의 `ssl-redirect` 는 **그 Ingress 에 tls 항목이 있을 때만** 켜지기 때문이다. 기존 호스트(`nullus.io`·`auth`)는 tls 항목이 있어 308 이 나가고 있어서, 새 호스트에서만 조용히 빠진다.

  평문으로 열리면 **SSO 로그인이 깨진다.** OIDC PKCE 가 쓰는 `crypto.subtle` 은 secure context 에서만 노출되고, 이건 이 저장소가 `setup-tls.sh` 머리말에 이미 적어 둔 이유다. 인증서를 붙여 놓고 이 구멍을 남기면 "HTTPS 로 들어가면 되는데 왜 안 되지" 가 된다.

  ConfigMap 에 `force-ssl-redirect` 를 넣어 전역으로 건다. Ingress 마다 어노테이션을 붙여도 되지만 그건 도구를 늘릴 때마다 손대는 일이라, 기본 인증서를 쓰는 이유와 정면으로 어긋난다. ACME HTTP-01 은 영향받지 않는다 — Let's Encrypt 는 검증 시 http→https 리다이렉트를 따라가고 리다이렉트 대상의 인증서는 검사하지 않는다.

  하는 일이 인증서 하나가 아니게 되어 `default-cert` 를 `ingress-https` 로 바꿨다. 이전 이름도 같은 동작으로 남겨 둔다.

- **환경별 값을 저장소에서 걷어낸다** (`deploy/csp/zadara/env.example` 신규, `expose-apiserver.sh` · `expose-web.sh` · `kubeconfig.sh` · `tunnel.sh` · `setup-tls.sh`): bastion 주소가 네 스크립트에 기본값으로 박혀 있었다(`BASTION="${BASTION:-ubuntu@<공인IP>}"`). 이건 저장소가 아니라 환경에 속하는 값이다 — 오픈소스로 받은 사람이 우리 인프라 주소가 들어간 스크립트를 받게 되고, 다른 환경에 올릴 때마다 코드를 고쳐야 한다.

  `deploy/csp/zadara/.env`(gitignore, `.env` 패턴이 이미 있었다)에서 읽고, 없으면 환경변수로 받는다. **기본값은 두지 않는다** — 특정 환경의 주소를 코드에 박으면 다른 환경에서 조용히 엉뚱한 곳에 붙는다. 틀린 기본값보다 멈추는 편이 낫고, 멈출 때 무엇을 어떻게 채우는지 함께 출력한다.

  `.env` 의 값은 `"${VAR:-...}"` 형태로 쓴다. 스크립트가 이 파일을 `source` 하므로 평범하게 `VAR=...` 로 쓰면 **명령줄에서 준 환경변수를 조용히 덮어쓴다.** 다른 환경에 한 번만 붙는 경우가 실제로 있어서 이 우선순위가 필요하다. `env.example` 에 그 이유를 적어 두었다.

  `BASTION` 하나만 옮기면 품이 아까워 `DNS01_ZONE` · `ACCESS_DOMAIN` 도 같은 파일에 모았다. 다만 **비밀은 여기 두지 않는다** — API 키·비밀번호는 쿠버네티스 시크릿이고, 여기 있는 값(공인 IP)은 DNS 로 이미 공개된 것이다. 그래서 이 변경이 얻는 것은 기밀 유지가 아니라 이식성과 저장소 위생이다. git 히스토리에는 그대로 남아 있고, 공개 저장소 히스토리 재작성은 비용 대비 얻는 게 없어 하지 않았다.

- **와일드카드 인증서를 실제 서비스에 연결한다** (`deploy/csp/zadara/values-zadara.yaml`, `deploy/csp/zadara/setup-tls.sh`): 인증서는 발급됐지만 어느 Ingress에도 붙지 않아 브라우저에는 여전히 자체서명이 나가고 있었다. 연결하면서 세 가지가 드러났다.

  **`cert-manager.io/cluster-issuer` 어노테이션을 빼야 한다.** 두면 ingress-shim 이 tls 항목마다 Certificate 를 자동 생성하는데, `nullus-wildcard-tls` 는 `setup-tls.sh` 가 만든 Certificate 가 이미 소유한다. 소유자가 둘이 되면 같은 시크릿을 서로 덮어쓰며 재발급을 반복하고 **Let's Encrypt 주 50 건을 태운다.** 조용히 벌어져 한참 뒤에나 드러나는 종류다. 실제로 기존 인증서 3 장이 전부 `owner=Ingress` 로 shim 소유였다.

  **Ingress 는 하나가 아니다.** 플랫폼(`nullus`)과 Keycloak(`nullus-keycloak`)이 별개 리소스라 한쪽만 고치면 `auth.nullus.io` 만 옛 인증서로 남는다. Bitnami keycloak 차트는 `extraTls` 만 있어도 `tls:` 블록을 렌더하므로(`templates/ingress.yaml:50`), `tls:false` + `extraTls` 로 와일드카드를 직접 지정한다.

  **`www` 는 규칙 대신 리다이렉트로 바꾼다.** 같은 내용을 두 호스트에 서비스하면 중복 콘텐츠가 되고 OIDC redirect_uri 도 호스트마다 등록해야 한다. 그렇다고 규칙만 지우면 `www` 를 친 사람이 404 를 본다. `from-to-www-redirect` 로 apex 에 301 보낸다. `tls` 에는 남긴다 — 리다이렉트도 HTTPS 로 받아야 하고, 인증서가 없으면 브라우저가 301 을 보기 전에 경고부터 띄운다.

  전환이 끝나 `<IP>.nip.io` 호스트도 함께 걷어냈다. 원래 주석이 "도메인 전환 중 끊기지 않도록 남겨 둔다, 안정화되면 지워도 된다" 고 적어 둔 것이고, 공인 IP 가 저장소에서 하나 줄어든다.

- **`setup-tls.sh default-cert` — ingress-nginx 기본 인증서** (`deploy/csp/zadara/setup-tls.sh`): Ingress 의 `tls` 항목은 거기 적힌 호스트만 덮는다. 스택이 깔리며 생기는 `argocd` `grafana` `harbor` 등 열둘은 그 목록에 없어, 연결을 마쳐도 여전히 컨트롤러 자체서명(`CN=Kubernetes Ingress Controller Fake Certificate`)이 나갔다.

  `--default-ssl-certificate` 는 tls 항목이 없는 **모든** 호스트에 폴백 인증서를 씌운다. 도구를 하나 늘릴 때마다 values 를 고치는 일이 사라진다 — 라우트만 생기면 인증서는 이미 붙어 있다. 실제로 지정 후 열 개 호스트 전부가 `-k` 없이 curl 을 통과했다.

  시크릿이 없는 채로 지정하면 컨트롤러가 그것을 찾다 실패하고 **그동안 모든 호스트가 자체서명으로 떨어지므로**, 존재를 먼저 확인하고 없으면 멈춘다. `ingress-nginx` 는 이 저장소가 관리하지 않는 릴리스라 `--reuse-values` 로 인자 하나만 더한다 — 스크립트가 남의 릴리스를 건드리는 유일한 자리이고, 그래서 상태 출력에도 현재 지정값을 함께 보여준다.

- **와일드카드 인증서를 DNS-01 로 발급한다** (`deploy/csp/zadara/setup-tls.sh`): `airgap/scripts/23-setup-gateway.sh` 는 이미 `*.<도메인>` HTTPS 리스너와 `nullus-wildcard-tls` 시크릿을 전제하는데, 정작 그 시크릿을 만들 방법이 없었다. `setup-tls.sh` 가 HTTP-01 전용이었고 **와일드카드는 HTTP-01 로 발급할 수 없기 때문이다** — ACME 규격이 그렇다. 그래서 호스트마다 인증서를 따로 받고 있었고, 스택에 도구를 하나 늘릴 때마다 발급이 하나 늘었다. 그중 하나가 실패하면 그 도구만 조용히 접속 불가가 된다.

  `DNS01_ZONE` 을 주면 그 존만 DNS-01 로 검증한다. **HTTP-01 은 남긴다.** 도메인 전환기의 `<IP>.nip.io` 처럼 와일드카드가 덮지 못하는 이름이 섞일 수 있고, 그것까지 DNS-01 로 보내면 nip.io 존의 TXT 를 우리가 쓸 수 없어 발급이 통째로 막힌다. cert-manager 는 더 구체적인 `selector.dnsZones` 를 우선하므로 두 솔버가 한 ClusterIssuer 에 공존해도 서로를 가리지 않는다.

  솔버는 lego 를 감싼 웹훅으로 붙인다. cert-manager 가 내장한 DNS-01 솔버는 Route53·Cloudflare·DigitalOcean·AzureDNS·acme-dns·RFC2136 뿐이고 우리 등록기관(Spaceship)이 없다. 흔한 우회는 `_acme-challenge` 를 지원되는 존으로 CNAME 위임하는 것인데, 그러면 남의 DNS 계정이 발급 경로에 하나 더 끼고 cert-manager #5751(와일드카드 + `cnameStrategy: Follow`)을 정면으로 밟는다. lego 는 v4.22.0 부터 Spaceship 을 1급 프로바이더로 지원하므로(차트 1.4.0 이 lego v4.30.1 을 벤더) 위임도 네임서버 이전도 없이 끝난다. 차트 버전은 고정한다 — 떠 있으면 어제 통하던 발급이 오늘 조용히 깨진다.

  **API 키는 이 스크립트가 만들지 않는다.** 있는지만 보고, 없으면 넣는 방법을 출력하고 멈춘다. 키가 코드나 셸 히스토리, CI 로그를 통과하지 않게 하려는 것이다. 대신 **키 이름을 검사한다** — 웹훅은 시크릿의 키를 그 이름의 환경변수로 그대로 lego 에 주입하므로 `SPACESHIP_API_KEY` 가 아니라 `api-key` 로 넣으면 자격증명을 못 찾는다. 그때 챌린지는 에러 없이 pending 으로 굳어 원인이 어디에도 보이지 않는다. 설계 중 실제로 저지른 실수라 검사로 굳혔다.

  `wildcard` 서브커맨드가 `*.<존>` 과 `<존>` 두 이름을 한 장에 담은 Certificate 를 만든다. 둘 다 적는 이유는 **와일드카드가 apex 를 덮지 않기** 때문이다. 이 한 장이 지금의 `nullus.io+www` 인증서와 `auth` 인증서를 모두 대체한다. 경로를 처음 뚫을 때는 `ISSUER=letsencrypt-staging` 으로 돌려라 — Let's Encrypt 는 등록 도메인당 주 50건이라 시행착오로 태우면 일주일을 기다린다.

- **설치되는 OSS 를 Keycloak 으로 로그인시킨다** (`internal/auth/adapter/keycloak/`, `internal/stack/adapter/helm/oidc-values.go`, `internal/stack/adapter/helm/{harbor,gitea}-provisioning.go`): SSO 프로비저닝 코드는 처음부터 있었지만 **한 번도 돈 적이 없었다**. `KEYCLOAK_URL` 을 API 프로세스에 넣어 주는 곳이 리포 어디에도 없어(런북도, 차트 deployment 의 env 27개도) 팩토리가 항상 nil 이었고, `provisioning_sso` 는 로그 한 줄만 남기고 성공으로 마킹됐다. 설치는 초록불로 끝나는데 도구는 전부 로컬 admin 계정으로 뜨는, 실패가 아니라 **조용한 누락**이었다.

  `configs/config.yaml` 에는 `keycloak` 블록이 처음부터 있었는데 `cfg.Keycloak` 소비처가 0건이었다. 새 환경변수를 파는 대신 그 죽은 설정을 잇는다. 건너뛸 때도 기동 로그에 남긴다 — 같은 누락이 다시 생겨도 이번에는 보이게.

  ArgoCD·Harbor·Gitea·Jenkins·GitLab·Grafana·MinIO 일곱 도구를 붙였다. 도구마다 설정을 받는 방식이 달라 한 가지 규칙으로 덮이지 않는다: Helm values 로 받는 것(ArgoCD/Grafana/MinIO), 자기 API 로만 받는 것(Harbor), CLI 로만 받는 것(Gitea — app.ini 로 DB 를 찾으므로 별도 Job 이 아니라 기동된 파드에 exec 한다), JCasC 로 받는 것(Jenkins), provider 블록 전체를 담은 Secret 으로 받는 것(GitLab).

- **포털에 ID/PW 로그인 경로** (`internal/auth/{domain,usecase,port}/`, `internal/auth/adapter/{token,handler,repository}/`, `web/src/features/auth/`): 포털은 OIDC 아니면 session 둘 중 하나만 썼다. `DualAuthMiddleware` 는 이름과 달리 authMode 로 하나를 골라 반환할 뿐이었고, session 모드는 클라이언트가 보낸 `X-User-*` 헤더를 그대로 믿어 사실상 무인증이었다. 비밀번호를 담을 컬럼도 로그인 엔드포인트도 없어 실제 자격 검증이 어디에도 없었다.

  그래서 OIDC 배포에는 ID/PW 로 들어갈 방법이 아예 없고, IdP 가 죽으면 아무도 들어갈 수 없었다. 두 경로를 함께 세운다. 토큰의 issuer 로 갈래를 정하되, 우리 것이 아니면 기존 검증기로 넘긴다(가로채면 IdP 사용자가 전부 401 이 된다). 반대로 우리 issuer 를 자칭했는데 서명이 틀리면 넘기지 않는다 — 넘기면 위조 토큰이 두 번째 기회를 얻는다.

  없는 이메일·비활성 계정·틀린 비밀번호를 같은 응답으로 답한다(구분하면 가입 여부가 샌다). 저장소 장애는 401 이 아니라 500 이다 — 401 로 답하면 DB 가 죽었을 때 전 사용자가 "비밀번호가 틀렸다" 는 답을 받고 원인을 못 찾는다.

  같은 이유로 ArgoCD 와 Jenkins 에도 비밀번호 경로를 남긴다. 둘은 SSO 를 켜면 보안 영역이 통째로 교체되어 로컬 계정이 막히기 때문이다(Harbor·Gitea 는 admin 이 살아 있어 별도 조치가 필요 없었다).

- **로컬에서 도구 간 SSO 가 성립하도록 단일 Keycloak 주소 배선** (`scripts/setup-local-domain.sh`, `scripts/runbook_local.sh`): Keycloak SSO 세션 쿠키는 호스트 단위다. 포털이 로그인한 주소와 도구가 브라우저를 보내는 주소가 다르면 쿠키가 실리지 않아 도구마다 다시 로그인해야 한다. 게다가 도구는 issuer 로 JWKS 를 직접 가져오는데, 기본값 `http://localhost:8180` 은 파드 안에서 파드 자신을 가리킨다(실측 확인).

  브라우저는 `/etc/hosts`, 클러스터는 CoreDNS 로 같은 이름에 닿게 한다. 호스트 IP 는 노드 안의 `host.docker.internal` 을 읽어서 얻는다 — 박아 두면 Docker 버전이나 네트워크 구성이 바뀔 때 조용히 깨진다. `NULLUS_LOCAL_DOMAIN` 으로 옵트인이라 기존 로컬 구성은 그대로다.

- **플러그인을 구운 Jenkins 이미지** (`deploy/images/jenkins/`, `scripts/build-jenkins-image.sh`, `.github/workflows/cd.yml`): 기본 차트는 파드가 뜰 때마다 `updates.jenkins.io` 에서 플러그인을 받는다. SSO 용 `oic-auth` 를 더하자 의존성 해석에 대용량 메타데이터가 필요해져 준비 검사 600초를 넘겼고(init 컨테이너가 11분째 `Cache miss for: plugin-versions` 에 머문 것을 실측), 에어갭에서는 애초에 나갈 수 없다. 타임아웃을 늘리는 건 해법이 아니다 — 설치 시간이 네트워크에 좌우되고 실패를 늦게 발견할 뿐이다.

  느린 다운로드를 매 설치에서 한 번의 빌드로 옮긴다. 태그는 브랜치가 아니라 Jenkins 버전이다 — 차트가 그 값을 고정 참조하므로 브랜치 태그로 밀면 스택 설치가 없는 이미지를 가리킨다.


- **설치 규모 계획을 서버가 계산한다** (`internal/stack/domain/planning.go`, `internal/stack/domain/planning_plan.go`, `internal/stack/usecase/create_stack.go`): 템플릿은 `planning_profile` 을 들고 있었지만, 그 값을 자원 요청으로 옮기는 계산은 설치 마법사(`web/src/features/stack/utils/install-planning-utils.ts`)에만 있었다. 그래서 API·CLI·에어갭으로 만든 스택은 프로파일을 저장만 하고 크기에는 반영하지 않아, 8Gi 노드용 Lite 템플릿도 standard 크기로 깔렸다. 계산을 도메인으로 옮겨 두 경로가 같은 크기를 내게 한다.

  이식의 성공 기준은 "동작한다" 가 아니라 **"같다"** 다. 숫자가 갈리면 UI 설치와 API 설치가 서로 다른 크기로 깔리는데, 그건 조용히 벌어져 한참 뒤에야 드러난다. 그래서 마법사가 내놓는 값을 그대로 기대값으로 박은 테스트를 둔다(`TestPlanResourceVector_MatchesInstallWizard` — 프로파일 넷 × 슬롯 여섯의 정확값). `standard` 는 배수가 정확히 1 이라 관리자 기본값이 그대로 나와야 한다는 것도 따로 고정했다 — 여기가 어긋나면 프로파일을 고르지 않은 기존 스택의 설치 크기가 조용히 바뀐다.

  마법사가 계획을 실어 보내면 그 값이 이긴다. 사용자가 화면에서 조정한 값을 서버가 덮어쓰면 계획 화면이 무의미해진다.

- **런북의 스택 생성·삭제 명령** (`scripts/runbook_local.sh`): 런북에는 인프라를 올리고 내리는 명령만 있었고, 스택을 설치하거나 지우는 길은 없었다. 그래서 "지우고 처음부터 다시 깔아 본다" 를 하려면 `kind delete cluster` 로 클러스터를 통째로 버리는 수밖에 없었는데, 그 길로는 제품이 실제로 쓰는 삭제 경로(`DeleteStack` 의 helm uninstall + CRD 정리)를 **한 번도 밟지 않는다**. 커뮤니티 사용자는 클러스터를 버리지 않고 스택만 지우므로, 검증되지 않은 채 공개되는 경로가 된다.

  `stack-up` / `stack-status` / `stack-down` / `pipeline-down` / `purge` 를 추가했다. 생성도 삭제도 helm 을 직접 부르지 않고 **백엔드 API 를 통한다** — 스크립트가 helm 을 직접 부르면 설치 구현이 백엔드와 둘로 갈라져 같은 템플릿이 경로마다 다른 결과를 낸다(`airgap/scripts/29-install-stacks-via-api.sh` 가 같은 판단을 적어 두었다). 도구 선택도 템플릿 API 응답에서 가져온다. 차트 버전 표를 스크립트에 복사해 두면 마이그레이션이 버전을 올릴 때마다 스크립트만 낡는다.

  `purge` 는 파이프라인 → 스택 → Nullus/백킹 → kind 순으로 지운다. 파이프라인이 먼저인 이유는 그것이 스택을 참조하기 때문이다 — 스택을 먼저 지우면 남은 파이프라인이 사라진 네임스페이스를 가리켜 화면에 유령 행으로 남는다. 기본값은 DB 볼륨까지 지우므로 다음 `up` 이 빈 DB에서 마이그레이션을 처음부터 돌린다. `stack-down` 은 끝나고 클러스터에 남은 helm 릴리스를 보고한다 — 남은 것을 보여주지 않으면 "지웠다" 고 믿은 채 다음 설치가 ownership 충돌로 깨진다.

- **제품 둘러보기(투어)** (`web/src/features/tour/`, `web/src/stores/tour-store.ts`, `web/src/components/layout/header.tsx`): 처음 들어온 사람이 "무엇부터 눌러야 하는지" 를 알 방법이 화면 어디에도 없었다. 헤더의 언어 버튼 옆 튜토리얼 버튼을 누르면 클러스터 등록 팝업 → 경량 템플릿 상세 → 기본 템플릿 사용 → 설치 마법사 일곱 탭(Authentication·Artifacts·CI/CD·Observability·Storage·Resources·Dry Run) → 배포 → 스택 목록·워크로드·연결 정보 → CI/CD 여섯 단계(기본 정보·코드 체크아웃·빌드·테스트·보안·생성) → 모니터링·알림 규칙까지 **스물아홉 걸음**을 화면과 팝업과 탭을 실제로 열어 가며 훑는다.

  걸음을 이만큼 잘게 나눈 이유는 이 제품의 어려움이 "어느 메뉴에 있는가" 가 아니라 "무엇을 어떤 순서로 정해야 하는가" 에 있기 때문이다. 특히 설치 뒤의 **Gateway PF Copy / /etc/hosts Copy** 는 이름만 보고는 무엇을 하는 버튼인지 알 수 없어 각각 한 걸음을 준다 — 인그레스 없이 스택 화면에 닿는 방법과, 클러스터와 같은 주소로 이미지 pull·브라우저 링크를 맞추는 방법이다.

  **강조는 덮개에 구멍을 뚫어 만든다.** SVG 마스크 하나로 덮지 않고 사각형 넷을 두르는 이유는 그 구멍이 클릭을 그대로 통과시켜야 하기 때문이다 — 마스크로 덮으면 "여기를 눌러 보세요" 라고 해 놓고 누를 수 없게 된다. 강조된 요소를 실제로 누르면 다음 걸음으로 넘어가고, 읽기만 하려는 사람을 위해 앞·뒤 버튼은 우측 하단 한자리에 고정한다. 팝업 안이나 다른 탭에 있는 것을 가리키는 걸음은 그것을 **먼저 눌러 연다**(`activate`). 다만 대상이 이미 보이면 누르지 않는다 — 사용자가 직접 눌러 왔다면 이미 열려 있고, 거기서 한 번 더 누르면 팝업이 닫힌다.

  탭은 라벨이 아니라 id 로 집는다(`data-tab`). 라벨은 번역되면 바뀌므로 문구를 고치는 순간 투어가 조용히 대상을 잃는다. 탭을 가리키는 걸음은 **탭 버튼과 그 안의 본문을 함께 감싼다** — 버튼만 강조하면 그 탭에서 무엇을 고르는지가 화면에서 잘려 나간다.

  걸음이 가리키는 것은 **화면 안으로 끌어온다**(`scrollIntoView`). 스크롤 아래에 있는 탭 본문을 강조하면 화면 밖에 테두리를 그리게 되어 아무것도 보이지 않았다. 강조 사각형은 뷰포트로 자른다 — 탭 + 긴 본문은 화면보다 커서 테두리가 잘린 채 남았다. 설명 상자는 대상의 **중심축**에 맞춘다: 모서리에 맞추면 팝업처럼 큰 대상에서 설명이 반대편 구석으로 떨어져 무엇을 가리키는지 읽히지 않았다.

  `activate` 는 두 종류를 구분한다. **대상과 누를 곳이 같으면(탭) 늘 누른다** — 탭 버튼은 어느 탭이 열려 있든 항상 화면에 있어서 "대상이 보이면 건너뛴다" 로 두면 탭이 영영 바뀌지 않는다(실제로 Artifacts 에 멈춰 있었다). 다르면(팝업) 대상이 없을 때만 누른다 — 열려 있는데 또 누르면 닫힌다.

  **화면을 직접 찍어 가며 맞췄다**(`web/e2e/tour-walkthrough.spec.ts`). 강조가 엉뚱한 것을 잡거나 설명 상자가 대상을 덮는 것은 단정으로 잡기 어렵고 그림을 봐야 안다. 그렇게 찾은 것들:

  - **앱 팝업이 열리면 투어가 그 뒤로 숨었다.** MUI 모달은 z-index 1300 이고 나머지에 `aria-hidden` 을 건다. 투어를 body 로 포털해 그 위(1400)에 두고, 붙는 `aria-hidden` 은 즉시 떼어낸다 — 투어는 그 팝업을 설명하는 중이라 보조기기에서 사라지면 안 된다.
  - **탭을 화면 위로 올리면 sticky 헤더 밑에 깔렸다.** 좌표상으로는 보이는데 화면에는 없는, 찾기 어려운 상태다. `PageHeader` 에 손잡이를 두고 그 높이만큼 비켜 세운다.
  - **부드러운 스크롤이 겹쳐 과하게 밀렸다.** 애니메이션이 끝나기 전에 다음 보정이 위치를 재면서 값이 겹쳤다. 즉시 스크롤로 바꾸고 얼마나 움직일지를 직접 계산한다(`scrollIntoView` 는 어느 상자를 얼마나 움직일지 브라우저가 정한다).
  - **설명 상자가 대상을 덮었다.** 높이를 150px 로 가정했는데 문구가 길어진 걸음에서 그보다 커졌다. 실제로 그려진 높이를 재서 자리를 잡는다.

  걸음은 여러 곳을 순서대로 눌러 열 수 있다. 스택 상세는 목록에서 행을 골라야 탭이 생기므로, 행을 먼저 누르고 탭을 누른다 — 한 곳만 누르던 동안에는 그 걸음들이 강조할 것을 찾지 못해 설명만 떠 있었다. **투어가 스스로 누른 클릭은 "사용자가 눌렀다" 로 세지 않는다**: 그러지 않으면 탭을 열려고 누른 클릭이 곧바로 다음 걸음을 부르고, 보여 주려던 화면을 아무도 보지 못한 채 지나간다(워크로드·모니터링 걸음이 실제로 그렇게 건너뛰어졌다).

  강조와 설명 상자는 걸음 사이를 이어서 움직인다(`motion-safe`). **배경막은 예외로 즉시 옮긴다** — 자리를 250ms 마다 다시 재는데 전환이 300ms 면 배경막이 영원히 뒤따라오며 구멍을 덮어, 강조된 것을 누를 수 없게 된다(Playwright 가 "backdrop intercepts pointer events" 로 잡았다).

  마지막 걸음은 설명이 아니라 **처음 눌러야 할 버튼**으로 돌려보낸다. 투어가 끝나는 자리가 곧 시작하는 자리여야 한다.

  강조가 **아무것도 가리키지 않게 되는 두 경우**를 막는다. 크기가 0 인 것은 없는 것으로 친다 — 조건부로 그려지는 영역(기능을 켜야 생기는 Test·Security 섹션)은 선택자에 걸리지만 실체가 없어 한 줄짜리 빈 조각을 강조하게 된다. 합집합이 화면을 거의 다 덮으면 합치지 않고 누르는 곳만 남긴다 — 화면 전체를 두른 테두리는 강조가 아니다.

  대상을 찾을 때 **투어가 그린 것은 건너뛴다.** 설명 카드도 `role="dialog"` 라, 팝업을 가리키는 걸음이 자기 자신을 강조하고 → 강조가 카드를 움직이고 → 그 움직임이 다시 강조를 옮기는 되먹임으로 화면이 요동쳤다. 앱의 팝업만 가리키도록 `Modal` 에 `data-modal` 손잡이를 따로 두었다.

  **투어 중에는 목록 응답만 목업으로 갈아 끼운다** (`tour-mock-adapter.ts`). 투어를 처음 도는 사람의 계정에는 클러스터도 스택도 파이프라인도 없어 빈 표를 강조하며 흐름을 설명할 수 없다. 화면 코드에 "투어일 때는 이 값" 분기를 넣지 않고 axios 경계 한 곳에서 어댑터를 바꿔 끼운다. **읽기(GET)만 가로챈다** — 투어 중에 실수로 눌린 생성·삭제가 성공한 것처럼 보이면 사용자는 있지도 않은 변경을 믿게 된다. 상세는 투어가 직접 열어 보겠다고 약속한 워크로드·모니터링 둘만 열어 준다. **투어가 끝나면 목업을 캐시에서 걷어 낸다**: 남겨 두면 새로고침 전까지 가짜 스택이 목록에 그대로 남아, 사용자는 자기 계정에 없는 것을 있다고 믿게 된다.

- **8Gi 노드 하나에 들어가는 경량 템플릿과, 규모를 들고 다니는 템플릿** (`internal/stack/domain/template.go`, `internal/stack/adapter/repository/memory_template.go`, `db/migrations/000072`, `web/src/features/`): 출하되던 템플릿 중 클러스터 안에 소스·CI·CD 를 모두 세우는 조합은 전부 최소 `20Gi RAM` 이었다. **단일 노드에서 스택 전체를 돌려 보려는 사람이 고를 것이 없었다.**

  `Gitea + Jenkins + Harbor + Argo CD (Lite)` 를 시드한다. 예산의 대부분은 템플릿이 고르지 않는 것들이 먼저 가져간다 — 실측하면 PostgreSQL 2Gi, Envoy 게이트웨이 0.8Gi(컨트롤러+프록시), OpenBao 0.3Gi, ESO 0.25Gi, 그리고 cert-manager 1.5Gi(관리자 기본값 0.5Gi 가 controller·webhook·cainjector 세 곳에 그대로 실린다). 남는 예산에 들어가는 조합만 담았다. GitLab(4.5Gi)·Prometheus(계산된 벡터가 prometheusSpec·alertmanager·kube-state-metrics·operator·node-exporter **다섯 곳에 그대로** 실려 Local 프로파일에서도 5Gi)·Nexus(JVM 고정 1.5Gi)는 들어가지 않는다.

  **레지스트리는 뺄 수 없다.** 처음에는 Harbor 도 빼서 Gitea·Jenkins·Argo CD 만 담았는데, 스택은 정상 설치되고 파이프라인을 만드는 순간 `registry.ResolverFor` 가 "이미지 레지스트리를 결정할 수 없습니다" 로 400 을 냈다. 폴백인 `ExternalRegistryPrefix` 는 조립 지점 어디에서도 채워지지 않아 실질적인 선택지가 아니다 — **스택은 서는데 아무것도 배포할 수 없는 템플릿**이 된다. 8Gi 안에서 세울 수 있는 레지스트리는 Harbor 뿐이다: core·registry 만 요청을 잡아 512Mi 로 서고, Nexus 는 JVM 을 고정해 1.5Gi 를 요구한다.

  **도구 선택만으로는 스택 크기가 정해지지 않아** 템플릿에 `planning_profile` 을 얹었다. 같은 도구 넷이라도 설치 마법사의 기본값인 standard 로 계획하면 8Gi 에 들어가지 않는다 — 실측에서 Argo CD 한 도구가 관리자 기본값(3Gi 벡터)으로 7개 파드에 3.07Gi 를 잡았다. 지금까지 템플릿은 "무엇을 깔지" 만 담고 "얼마나 크게 깔지" 는 마법사가 매번 standard 에서 시작했으므로, 자원을 약속하는 템플릿은 만들 수 없었다. 템플릿을 고르면 프로파일도 함께 실린다.

  모르는 프로파일 값은 핸들러가 400 으로 막는다 — 조용히 기본값으로 바꾸면 8Gi 를 노린 템플릿이 그 두 배로 설치되고 만든 사람은 그것을 알 길이 없다. 반대로 **빈 값은 standard 로 채운다**: 컬럼이 생기기 전에 만들어진 행과 프로파일을 보내지 않는 기존 클라이언트가 지금까지 하던 동작이 그것이다. 편집 모달에도 프로파일 칸을 두었다 — 폼이 그 값을 들고 있지 않으면 이름만 고쳐 저장해도 프로파일이 기본값으로 되돌아간다.

- **메인 화면의 퀵스타트** (`web/src/features/home/pages/home-page.tsx`): 경량 템플릿 설치로 곧장 보내는 CTA 를 히어로에 더한다. **클러스터가 하나도 등록되지 않았으면 누를 수 없다** — 마법사는 설치할 곳을 찾지 못하는데, 누를 수 있게 두면 사용자는 그 사실을 마법사 마지막 단계에 가서야 알게 된다. 비활성 버튼만 두면 왜 안 눌리는지도 알 수 없으므로 막힌 이유와 다음 걸음("클러스터를 먼저 등록하세요")을 같은 자리에 적는다. 스택을 설치할 수 없는 역할에게는 다른 CTA 와 같은 기준으로 비활성이다.

- **Gitea + Jenkins + Argo CD 파이프라인** (`internal/stack/adapter/helm/`, `internal/cicd/adapter/{gitea,jenkins}/`, `db/migrations/000067~000071`): Gitea·Jenkins 는 **설치 마법사에서 고를 수는 있었지만 배포해도 아무것도 설치되지 않았다** — 저장소가 그 사실을 `NOT_INSTALLABLE` 로 명시하고 있었고 Go 프로덕션 코드의 관련 히트는 0건이었다. 두 도구를 실제 설치 경로와 CI/CD 프로비저닝 경로에 배선한다.

  **Jenkins 는 GitLab CI 와 트리거 모델이 근본적으로 다르다**: `.gitlab-ci.yml` 은 푸시하면 자동 감지되지만 Jenkins 는 job 이 먼저 존재해야 한다. 그래서 렌더러에 case 를 하나 더 다는 것으로 끝나지 않고 `port.CIJobProvisioner` 와 multibranch job 생성 단계가 새로 필요했다. 러너도 다르다 — GitLab 은 `gitlab-runner` 차트를 세우지만 Jenkins 는 kubernetes 플러그인이 빌드마다 agent 파드를 직접 띄운다. 그래서 health check 의 러너 하드 게이트를 CI 플랫폼별로 좁혔다: 좁히지 않으면 Jenkins 스택은 설치가 다 끝나도 `completed` 에 도달하지 못하고, 그러면 bundle factory 가 파이프라인 생성을 거부한다.

  **CI 자격증명은 OpenBao → ESO → K8s Secret 평면에 얹는다.** Gitea 에는 GitLab 같은 프로젝트 CI 변수 저장소가 없다. Jenkins Credentials 를 1차 저장소로 쓰지 않는 이유는 자격증명 사본이 하나 더 생기고 회전 경로가 둘로 갈리기 때문이다 — OpenBao 단일 출처 원칙이 깨진다. 다만 소비자가 둘이고 주입 방식이 다르다: agent 파드는 `secretKeyRef` 로 env 를 받으면 되지만, **multibranch 스캔은 컨트롤러가 하므로 실제 credential 객체가 필요하다**. ESO 가 만든 Secret 을 컨트롤러에 마운트하고 JCasC `${name-keyName}` 보간으로 선언한다.

  자격증명은 발급 시점으로 두 부류다. 사전 생성 가능한 admin 비밀번호는 `provisioning_secrets`(phase A)가 만들지만, **Gitea 액세스 토큰·레지스트리 자격증명은 해당 OSS 가 뜬 뒤에만 얻을 수 있어** 그 단계에 넣을 수 없다 — 파이프라인 프로비저닝 시점과 회전 컨트롤러가 맡는다.

  Jenkins 차트 버전은 자유롭게 내릴 수 없다: Gitea multibranch 스캔에 쓰는 `gitea` 플러그인이 Jenkins 2.528.3 이상을 요구한다. 하한 아래로 내리면 실패가 설치 시점이 아니라 **첫 빌드 시점**에 나타나 원인을 찾기 어렵다.

- **CI 단계를 OSS 무관하게 표현한다** (`internal/cicd/port/ci_stage.go`): CI 마다 단계 어휘가 다르다 — Jenkins 는 `SUCCESS/FAILED/IN_PROGRESS/NOT_EXECUTED`, GitLab 은 `success/failed/running/pending/skipped`, GitHub Actions 는 `status` 와 `conclusion` 두 필드로 나눠 표현한다. 이 변환을 위쪽 계층에서 하면 OSS 를 하나 늘릴 때마다 도메인과 화면이 모두 바뀐다. **변환을 어댑터 경계에서 끝낸다**: port 에 정규화된 어휘를 두고 각 어댑터가 자기 표현을 옮기며, 유스케이스는 CI 종류를 모른 채 도메인 스텝으로 넘긴다. 모르는 값은 `unknown` 이다 — 성공으로 넘겨짚으면 돌지 않은 단계가 성공으로 보인다. 건너뛴 단계는 실패와 구분한다: 건너뛴 것은 잘못된 것이 아니다.

- **CI 서버의 빌드 이력을 실행 통계로 들인다** (`internal/cicd/usecase/sync_pipeline_runs.go`): GitOps 경로에서는 플랫폼이 배포를 실행하지 않는다 — CI 가 빌드하고 Argo CD 가 동기화한다. 그래서 실행 기록이 CI 에만 있었고 `pipeline_deployments` 에 행을 넣는 경로는 플랫폼이 직접 배포하는 두 곳뿐이었다. 빌드가 성공해도 화면의 Success Rate·Total Runs 가 영원히 0 이었다. 배포 ID 를 빌드 번호에서 만들어 멱등하며, 동기화는 특정 파이프라인을 조회할 때만 돈다 — 목록 전체에서 하면 비용이 파이프라인 수에 비례해 CI 서버를 찾는다.

- **Harbor 프로젝트를 만드는 `provisioning_harbor` 단계** (`internal/stack/adapter/helm/harbor-provisioning.go`): Harbor 는 push 전에 프로젝트가 존재해야 하는데 그것을 만드는 경로가 없어 첫 이미지 push 가 `unauthorized: project not found` 로 죽었다. 스택은 정상 설치됐고 빌드도 성공한 뒤라 원인이 멀리 떨어진 실패였다. 기존 `gitlab-harbor-v1` 템플릿도 같은 지점을 겪는다. Nexus 가 `provisioning_nexus` 로 커넥터·저장소를 맞추는 것과 같은 자리에 둔다.

- **OpenTelemetry 수집기를 스택의 관측 계층으로** (`internal/stack/domain/config.go`, `internal/stack/adapter/helm/otel-collector.go`, `db/migrations/000063_seed_otel_stack_template.up.sql`): 설치 마법사의 **Exporter / Agent 선택이 아무 일도 하지 않고 있었다** — 프론트는 배포 요청에 `trace_exporter` 를 실어 보냈지만 도메인 설정에 받는 칸이 없어 조용히 버려졌다. `trace_layer`(추적 저장소)와 칸을 나눈다: 역할이 다르기 때문이다. `trace_layer` 는 추적을 저장·조회하는 백엔드(Tempo/Jaeger)이고, `trace_exporter` 는 그 앞에 서서 OTLP 를 받아 추적·메트릭·로그를 각각의 저장소로 나눠 보낸다 — 한 칸에 묶으면 둘 중 하나만 설치할 수 있어 "수집기를 통해 관측한다" 는 구성 자체가 불가능하다.

  수집기는 **둘로 나눈다**. OpenTelemetry 의 표준 배치다. 게이트웨이(Deployment)는 OTLP 를 받아 Tempo/Prometheus/Loki 로 내보내고 — 앱이 붙는 주소가 하나로 고정되어야 한다 — 에이전트(DaemonSet)는 노드의 `/var/log/pods` 를 읽어 게이트웨이로 넘긴다(로그 파일은 노드마다 있으므로 모든 노드에 떠야 한다). 에이전트가 저장소로 직접 보내지 않는 이유는 출구를 하나로 모아야 저장소를 바꿀 때 고칠 곳이 한 군데로 남기 때문이다. 게이트웨이가 없거나 로그를 받아 줄 저장소가 없으면 에이전트를 설치하지 않는다 — 갈 곳도 없는 로그를 위해 노드마다 DaemonSet 을 띄우지 않는다. **고르지 않은 백엔드로 보내는 exporter 는 만들지 않는다**: 주소가 풀리지 않아 수집기는 뜨는데 추적만 조용히 사라지는, 원인을 찾기 어려운 상태가 된다.

  Loki 는 라벨로만 스트림을 가르므로 `resource/loki` 프로세서로 `k8s.namespace.name`·`k8s.container.name` 을 라벨로 승격한다. **파드 이름은 넣지 않는다** — 재생성마다 새 스트림이 생겨 카디널리티가 터진다(본문 속성으로는 남아 검색된다). 릴리스명은 차트 이름과 다르게 둔다(`otel-collector` / `otel-agent`): 추적 계층 단계가 같은 차트를 설치할 수 있어 이름이 겹치면 Helm 이 소유권 충돌로 거부한다. 이름 규칙은 `shared` 가 소유한다 — cicd 가 배포되는 앱에 이 주소를 넣어 줘야 하는데 모듈끼리 서로의 internal 을 참조할 수 없다. 연결정보에는 OTLP 주소를 안내한다: 수집기는 자격증명이 없고, 안내의 본체는 "어디로 보내야 하는가" 다.

  Golden Path 템플릿 `gitlab-argocd-otel-v1` 을 인메모리·마이그레이션 양쪽에 시드하고 호환성 매트릭스와 단계 카탈로그도 함께 넣는다.

- **스택이 설치한 OSS 가 자기 메트릭을 Prometheus 에 내준다** (`internal/stack/adapter/helm/service-monitors.go`): 모니터링을 설치해도 **도구 자신이 아는 것은 어디에도 남지 않았다** — 파드의 CPU·메모리는 cadvisor 로 보이지만 Argo CD 의 동기화 실패 수, Loki 의 수집 지연, MinIO 의 버킷 사용량은 수집되지 않았다. 차트가 ServiceMonitor 를 만들도록 켜고 거기에 kube-prometheus-stack 의 `release` 라벨을 붙인다 — 그 차트는 기본값에서 자기 라벨이 붙은 모니터만 고르므로, 라벨이 없으면 **리소스는 생기는데 스크랩은 안 된다**. 키는 차트마다 다르다(grafana 만 `labels`, 나머지는 `additionalLabels`, argo-cd 는 컴포넌트 5개 각각). minio 는 템플릿 조건이 `and .enabled .includeNode` 라 `enabled` 만 켜면 values 에는 실리는데 ServiceMonitor 는 만들어지지 않는다 — 켠 줄 알고 넘어가기 딱 좋은 함정이라 실제로 한 번 밟았다. Prometheus 를 고르지 않은 스택에서는 아무것도 켜지 않는다: ServiceMonitor 는 Operator 의 CRD 라 없으면 설치가 통째로 멈춘다. GitLab 은 차트에 `serviceMonitor` 값 자체가 없어 제외했다.

- **배포되는 앱에 스택 수집기 주소를 넣어 준다** (`internal/cicd/usecase/otlp_endpoint.go`, `internal/cicd/adapter/manifests/generator.go`): 수집기가 떠 있어도 앱이 주소를 모르면 아무것도 내보내지 않는다. OTel SDK 가 표준으로 읽는 `OTEL_EXPORTER_OTLP_ENDPOINT`·`OTEL_EXPORTER_OTLP_PROTOCOL`·`OTEL_SERVICE_NAME` 을 매니페스트에 넣는다. **프로토콜을 명시하는 이유**는 SDK 기본값이 `http/protobuf` 인데 수집기의 4317 은 gRPC 포트라 두면 전송이 실패하기 때문이다. 수집기가 없는 스택에는 넣지 않는다 — 닿지 않는 주소를 박으면 앱이 영원히 재시도하며 실패 로그만 쌓는다. 사용자가 같은 키를 직접 지정했으면 건드리지 않는다: 외부 수집기로 보내려는 선택을 덮어쓰면 안 되고 같은 이름을 두 번 선언하면 어느 값이 적용될지도 알 수 없다. 배포 경로가 둘이라(파이프라인 배포·직접 배포) 판단을 한 곳으로 모았다 — 각자 구현하면 한쪽만 고쳐져 "어떤 경로로 배포했느냐에 따라 추적이 갈리는" 상태가 된다(실제로 직접 배포 쪽이 빠져 있었다).

- **클러스터 상세의 가용 리소스와 조직 이름** (`internal/admin/usecase/cluster_usecase.go`, `web/src/features/admin/pages/cluster-page.tsx`): 조직 접근 칸이 UUID 만 보여 주어 사람이 어느 조직인지 알 수 없었다. 서버가 이름을 함께 내려주되 조회에 실패해도 오류를 올리지 않는다 — 이름은 부가 정보이고 그것 때문에 목록이 뜨지 않으면 더 나쁘다(못 찾으면 화면이 ID 로 되돌아간다). 가용 리소스는 데이터가 이미 있었는데 화면에서만 쓰이지 않았다: 스택을 올리기 전에 알아야 하는 것은 "노드가 몇 대인가" 가 아니라 "지금 얼마가 남아 있는가" 이므로 allocatable 에서 이미 예약된 request 를 뺀 값을 보여 준다. **limit 이 아니라 request 를 기준으로 삼는다** — 스케줄러가 자리를 잡을 때 보는 값이 request 이고, limit 합계는 오버커밋 때문에 가용량보다 큰 것이 정상이다.

- **배포된 스택의 OSS 설정을 values.yaml 수준에서 고쳐 다시 적용** (`internal/stack/port/release_values.go`, `internal/stack/adapter/helm/release-values.go`, `internal/stack/usecase/manage_release_values.go`, `web/src/features/stack/components/stack-config-tab.tsx`): 기능분해도의 `NULLUS_DSS_040_040`(스택 설정 수정 및 재배포)이 비어 있었다. Monaco 에디터는 설치 마법사에만 있었고, 거기 뜨는 YAML 도 프론트가 만든 매니페스트라 백엔드가 실제로 helm 에 넘긴 values 와 달랐다. 배포가 끝난 스택에는 설정을 고칠 진입점 자체가 없었고, DB 만 갱신하는 `POST /stacks/:id/config` 는 클러스터에 재적용하지 않는 데다 프론트에서 호출하는 곳조차 없었다. 이제 스택 상세의 **Config 탭**에서 릴리스를 골라 편집한다.

  편집 단위를 **두 가지 중에 고른다**. `live` 는 릴리스에 실제로 배포된 values 전체이고(보이는 그대로가 배포값이지만 플랫폼이 계산해 넣은 값까지 노출된다), `override` 는 사용자가 얹은 커스텀만이다(안전한 대신 지금 무엇이 적용돼 있는지는 보이지 않는다). 어느 쪽이 맞는지는 상황마다 다르므로 하나로 정하지 않았다. `override` 는 배포값 위에 **누적**된다 — 키를 지워도 이미 적용된 값은 되돌아가지 않는다(되돌리려면 `live` 에서 지운다). 대신 플랫폼이 계산해 넣은 값을 이 경로가 실수로 날려 버리는 일은 없다.

  어느 쪽으로 편집하든 결과는 **두 곳에** 남는다. 클러스터에는 `helm upgrade` 로, 스택 설정에는 `config.yaml_overrides` 로. 후자가 빠지면 다음 재배포·재시도에서 설치 경로가 값을 처음부터 다시 계산하면서 편집이 조용히 사라진다. 순서는 **helm 이 먼저**다 — 적용에 실패한 설정이 DB 에 남으면 다음 배포가 검증된 적 없는 값을 들고 나간다. 차트는 배포된 버전에 고정한다: 설정 한 줄 바꾸려다 차트 버전이 함께 올라가면 안 된다.

  **플랫폼이 소유한 values 경로는 미리 표시하고, 건드리면 경고한다**(`internal/stack/domain/release_values.go`). Harbor 의 `externalURL`, GitLab 의 `global.psql`·`registry.storage`, PostgreSQL 의 `auth.existingSecret`, OIDC 블록 따위다 — 지우거나 바꾸면 스택이 조용히 깨지는데, 원인은 언제나 멀리 떨어져서 드러난다(`externalURL` 을 되돌리면 노드의 containerd 가 레지스트리 주소를 풀지 못해 배포된 앱이 ImagePullBackOff 에서 나오지 못한다). 막지는 않는다 — 전문가용 탈출구가 이 기능의 존재 이유다. 대신 무엇을 건드렸는지 적용 전에 반드시 보여 준다. 미리보기는 `helm upgrade --dry-run` 으로 렌더까지 검증하고 실제 적용될 values 를 함께 돌려준다.

  실제 클러스터에서 값을 바꿔 가며 검증하는 과정에서 **Helm SDK 의 함정 셋**을 만났고 셋 다 고쳤다. (1) 업그레이드가 실패하면 Helm 은 **실패한 리비전을 최신으로 남긴다** — `helm get values` 는 그 리비전을 돌려주므로 적용된 적 없는 값이 "현재 값" 으로 편집기에 뜨고, `override` 모드에서는 그 위에 병합까지 된다. 마지막 `deployed` 리비전을 읽는다. (2) Helm 은 릴리스에 차트를 저장할 때 **의존 서브차트를 직렬화하지 않는다**(`chart.Chart` 의 `Raw` 는 `json:"-"`, `dependencies` 는 비공개 필드다). 저장본을 그대로 재사용하면 bitnami `common` 에 기대는 PostgreSQL·MinIO·Harbor 가 전부 `no template "common.names.fullname"` 으로 깨진다 — 의존성이 없는 Grafana 만 통과해서 처음엔 멀쩡해 보였다. 저장본이 렌더 가능하면 그대로 쓰고, 의존성이 유실됐을 때만 **같은 버전으로** 다시 받아온다. (3) 기본 드라이런은 템플릿의 `lookup` 을 빈 값으로 만드는데, bitnami 차트들은 그 `lookup` 으로 기존 비밀번호 Secret 을 확인한 뒤 렌더를 거부한다 — 실제 적용은 되는데 **미리보기만 실패하는** 오답이 된다. `DryRunOption="server"`(읽기 전용)로 돌린다.

  미리보기에서 차트가 렌더되지 않는 것은 서버 오류가 아니라 편집 결과이므로 200 + `render_error` 로 돌려준다. 에러로 던지면 함께 계산해 둔 보호 경로 경고까지 사라져, 사용자는 무엇을 잘못 건드렸는지 알 길이 없어진다.

- **설정 변경의 이력과 감사** (`internal/shared/middleware/actor.go`, `internal/stack/adapter/handler/release_values_handler.go`): 적용 직전 설정이 `stack_config_versions` 에 스냅샷되어 기존 diff·롤백 경로로 되짚을 수 있고, `update_release_values` 감사 항목이 남는다. 신원을 푸는 자리를 `middleware.ActorFromContext` 하나로 모았다 — 핸들러들이 `X-User-ID` 헤더만 보고 있어서 **OIDC 모드에서는 감사 로그의 `user_id` 가 통째로 비었다**(신원이 헤더가 아니라 인증 미들웨어가 심은 컨텍스트로 오기 때문이다). 사용자 타입을 직접 참조하지 않고 필드 이름으로 읽어 모듈 경계를 넘지 않는다 — 같은 패키지의 rate limiter·org context 가 이미 쓰는 방식이다. **실패한 적용도 남긴다**: 성공만 기록하는 감사는 "누가 무엇을 시도했나" 에 답하지 못한다. 다만 남기는 것은 **건드린 키뿐이고 값은 남기지 않는다** — values 에는 사용자가 적어 넣은 자격증명이 들어갈 수 있고 감사 로그는 그보다 넓게 읽힌다(값 자체는 설정 이력에 남으므로 되짚을 수 있다). 오류 메시지는 500자에서 자른다: Kubernetes 의 패치 실패는 요청 본문 전체를 메시지에 담아 되돌려주는데 실측 한 건이 9.8KB 였다.

- **아이콘 체계 — 크기 4단계·상태 한 벌·타일 하나** (`web/DESIGN.md`, `web/src/components/ui/icon.ts`, `web/src/components/ui/status-icon.tsx`, `web/src/components/ui/icon-tile.tsx`): 아이콘 93종이 354곳에 쓰이는데 규칙이 없었다. 전수조사(AST)로 드러난 것은 세 가지다. (1) **크기가 12가지**로 흩어져 있었다 — 10·11·12·13·14·15·16·18·20·24·28·32px, 그중 13~16px 이 213곳이다. 13px 과 14px 은 눈으로 구분되지 않으니 규칙이 아니라 그때그때 고른 흔적이다. (2) **`strokeWidth` 를 지정한 곳이 0곳**이었다. lucide 는 24 격자에 stroke 2 로 그려지므로 `size` 만 줄이면 렌더 굵기가 `2 × (size/24)` 로 함께 줄어든다 — 12px 에서 1.0px, 28px 에서 2.33px. 크기를 고르는 순간 굵기가 딸려 변해서 작은 아이콘은 흐리고 큰 아이콘은 뭉툭했다. (3) **같은 뜻에 여러 글리프**가 동시에 쓰였다: 성공 하나에 `CheckCircle`·`CheckCircle2`·`Check` 셋(29곳), 진행중에 `Loader`·`Loader2`·`RefreshCw`·`CircleDashed` 넷, 차트에 `BarChart2`·`BarChart3`·`ChartNoAxesColumn` 셋. 고르는 자리가 화면마다 있었기 때문이다.

  이제 크기는 `xs 12 / sm 16 / md 20 / lg 28` 네 단계뿐이고 **굵기가 거기 딸려 온다**(2.25 / 2 / 1.75 / 1.5). 값은 DESIGN.md 의 `icon` 블록이 단일 출처이고 `generate-theme.mjs` 가 굽는다. 화면은 `iconProps('sm')` 으로 한 쌍을 함께 받는다 — `size` 만 집어 가면 굵기가 따라오지 않기 때문이다. 렌더 굵기를 완전히 같게 맞추지는 않았다: 12px 에서 stroke 3(렌더 1.5px)까지 올리면 톱니가 촘촘한 글리프(`Settings`)의 안쪽이 메워진다. 실제로 그려 보고 고른 값이다.

  상태 아이콘은 `status-icon.tsx` 한 곳이 DESIGN.md 의 표를 그대로 돌려준다. **여섯이 같은 원 실루엣을 쓰고 경고만 삼각형이다** — 상태가 한 벌로 읽히게 하려는 것이고, 경고는 그 벌에서 튀어나와야 하므로 유일하게 모양을 깬다(`CircleCheck` · `LoaderCircle` · `CircleDashed` · `CircleX` · `CircleMinus` · `Info` · `TriangleAlert`). lucide 의 `CheckCircle` 은 이름과 달리 체크가 원을 뚫고 나오는 변형이라 옆에 서는 `CircleX`·`CircleMinus` 와 실루엣이 어긋난다 — 담긴 형태는 `CircleCheck` 다. 표에서 **`Running` 을 `Success` 에서, `Pending` 을 `Warning` 에서 뗐다**: 설치·파이프라인 화면은 "도는 중"과 "끝남"을 반드시 구별해야 하고, 대기는 시간의 문제고 경고는 주의의 문제라 같은 색·같은 모양을 줄 이유가 없다. 상태색으로 `--color-primary` 는 쓰지 않는다 — CTA 의 색이라 얹으면 "도는 중"과 "누르세요"가 같은 파랑이 된다.

  아이콘에 바탕을 까는 타일도 하나로 모았다(`IconTile`). 개편 전에는 KPI 가 `h-9 w-9 rounded-lg`, 기능 카드가 `h-9 w-9 rounded-[10px]`, 제목 옆이 `--icon-size` 로 세 벌이었다. 색은 클래스가 아니라 인라인 `style` 로 토큰을 참조한다 — Tailwind 임의값은 소스 문자열을 스캔해 만들어지므로 토큰 이름을 조립하면 그 클래스가 생성되지 않고 색이 조용히 빠진다.


- **브랜드 마크와 그 생성기** (`docs/40_UI_UX/logo/emit.mjs`, `web/src/components/brand/`, `web/public/favicon.svg`): 브랜드가 놓이는 자리가 전부 임시값이었다 — 사이드바와 홈 히어로는 lucide 의 `Box` 아이콘, 로그인 카드는 금색 타일 위의 글자 "N", 파비콘은 프로젝트와 무관한 보라색 도형, 탭 제목은 `web`. 네 자리가 서로 다른 그림이라 같은 제품으로 보이지 않았다. 세잎 매듭(trefoil) 하나로 통일하고, 세 가닥에 쿠버네티스 파랑 → 인프라 보라 → 애플리케이션 청록을 흘렸다 — 세 층이 따로 놀지 않고 한 흐름으로 묶인다는 뜻이다. **도형은 손으로 그리지 않는다**: 매듭은 세 교차점에서 어느 가닥이 위인지가 일관되지 않으면 매듭이 아니라 그냥 겹친 선이 되는데, 그 판정을 눈으로 하면 반드시 틀린다. `emit.mjs` 가 매개변수식에서 점을 뽑고 자기교차점을 수치로 찾아 **파비콘 SVG 와 컴포넌트가 읽는 경로 데이터를 함께** 굽는다. 앱은 이 스크립트를 실행하지 않는다 — 좌표가 박힌 산출물만 쓴다. 아래로 지나가는 가닥을 끊는 간격은 좌표로 따로 계산하지 않고 **위 가닥과 같은 경로를 굵게 그은 마스크**로 낸다. 따로 계산하면 굵기를 고칠 때마다 간격이 위 가닥과 어긋나지만, 같은 경로에서 파생시키면 언제나 나란하다. 마크의 3색은 `web/DESIGN.md` 토큰이 **아니다** — 테마를 따라 회색이 되는 로고는 로고가 아니라 아이콘이다. 그래서 `src/components/brand/**` 만 hex 금지 규칙에서 면제했고(외부 도구 브랜드 색을 한 파일에 모아 둔 `tool-brand-colors.ts` 와 같은 이유), 색이 정해진 바탕 위에 얹어야 할 때를 위해 `tone="current"` 로 단색 한 붓이 되는 길을 열어 뒀다. **어두운 바탕용으로는 밝기만 올린 두 번째 벌을 함께 굽는다**: HSL 의 lightness 는 사람이 느끼는 밝기가 아니라서 파랑과 보라 사이를 HSL 로 이으면 중간이 양 끝보다 어두운 남색으로 꺼지는데(휘도 .176 → .071 → .118), 밝은 바탕에서는 그 구간이 오히려 또렷하지만 어두운 바탕에서는 대비가 2.1:1 까지 떨어져 매듭의 그쪽 절반이 배경에 잠긴다 — 청록 쪽은 7.3:1 이라 한 마크 안에서 3.5배가 벌어졌다. 다크 벌은 색상·채도를 그대로 두고 목표에 못 미치는 색만 lightness 를 이분법으로 올려 `--color-surface-base` 기준 4.5:1 을 보장한다(편차 1.81배). 파랑·청록은 거의 그대로이고 보라만 `#6b3fd4` → `#8663dc` 로 올라간다. 바탕을 판별하는 신호는 두 곳이 다르다 — 앱은 `theme-store` 를 읽고(사용자가 OS 와 무관하게 테마를 고르므로 `prefers-color-scheme` 을 쓰면 강제로 라이트를 켠 사용자에게 어긋난다), React 밖의 독립 문서인 파비콘은 브라우저 크롬이 OS 를 따르므로 SVG 안에 `prefers-color-scheme` 규칙을 넣었다. 대비 약속은 테스트가 `tokens.generated.css` 의 실제 바탕 토큰을 읽어 검사한다 — 바탕이 밝아지면 마크만 조용히 과하게 밝은 채로 남기 때문이다. 홈 히어로에서는 금색 타일을 없앴다 — 마크가 스스로 3색을 가지므로 금색 바탕에 얹으면 색이 싸우고, 바로 아래 제목이 이미 이름을 말한다. 파비콘은 번들을 타지 않는 정적 파일이라 컴포넌트와 따로 노는데, 한쪽만 다시 굽고 커밋하는 사고는 테스트가 경로 대조로 막는다. 한 화면에 마크가 둘 이상 놓일 때 mask id 가 겹쳐 한쪽이 통째로 사라지는 것도 `useId` 로 막고 테스트로 고정했다.

- **배포된 애플리케이션의 실시간 자원 그래프와 컨테이너 로그** (`internal/stack/adapter/handler/workloads.go`, `internal/stack/adapter/handler/workload_logs.go`, `web/src/features/observability/components/app-runtime-panels.tsx`): CI/CD 로 배포한 앱은 "Running" 이라는 상태 문자열뿐이었다 — 그것만으로는 파드가 메모리 한계에 붙어 있는지 놀고 있는지, 무엇을 하다 죽었는지 알 수 없어 결국 `kubectl` 로 넘어가게 된다. `/workloads` 가 metrics-server 에서 파드별 사용량을 함께 읽고, 새로 만든 `GET /stacks/:id/workloads/logs` 가 스택 라벨로 찾은 파드들의 컨테이너 로그를 타임스탬프로 섞어 준다(파드별로 나누면 요청이 어느 파드로 갔는지를 사람이 맞춰 봐야 한다 — `kubectl logs -l` 이 섞는 이유와 같다). 백엔드는 "지금" 만 주므로 폴링(5초) 결과를 화면에서 쌓아 시계열을 만든다. **값을 못 읽은 시점은 0 이 아니라 선을 끊는다** — metrics-server 는 선택 설치라 없는 클러스터가 정상인데 0 으로 이으면 선이 바닥을 기어 "안 쓰는 앱" 으로 읽힌다. 모니터링 대시보드(스택 단위)와 CI/CD 목록 상세(파이프라인 단위)가 같은 컴포넌트를 쓰고 보는 범위만 좁힌다.
- **스택 상세의 Workloads 탭** (`web/src/features/stack/components/stack-workloads-tab.tsx`): "상세 설치 카드는 숨김 처리되었습니다" 라는 안내만 있던 자리에 실제 파드 목록(이름·도구·상태·재시작·CPU·메모리·노드)을 넣었다. 조회는 스택별로 동적이다.
- **배포 매니페스트에 스택·CI/CD 템플릿 라벨** (`internal/cicd/adapter/scaffold/renderer.go`): 배포된 워크로드가 어느 스택·어느 템플릿에서 나왔는지 클러스터만 보고 알 수 있어야 한다. 네임스페이스로는 판별할 수 없다 — 파이프라인이 `default` 에 깔 수도 있고 여러 스택이 한 네임스페이스를 공유할 수도 있다. `nullus.io/stack-id` 와 `nullus.io/cicd-template-id` 를 Deployment·파드 템플릿·Service 에 붙인다. 라벨 값은 항상 id 다 — 이름("Nullus Sample App — Backend")은 공백과 em dash 때문에 라벨 값으로 유효하지 않고, 바뀌면 이미 떠 있는 파드와 어긋난다.


- **디자인 단일 출처 `web/DESIGN.md` 와 토큰 파생 파이프라인** (`web/DESIGN.md`, `web/scripts/generate-theme.mjs`, `web/src/theme/`): 색·타입·간격·모양·깊이의 값을 고칠 곳이 한 곳도 아니었다 — 문서(`docs/40_UI_UX/Nullus_디자인시스템.md`)와 `index.css` 가 서로 다른 값을 갖고 있었고(문서는 라이트 보더를 `#e2e8f0`, 구현은 `#1f2937`), 정작 화면은 둘 다 무시하고 TSX 에 색을 직접 박고 있었다(hex 767곳 + rgba 750곳). 이제 [google-labs-code/design.md](https://github.com/google-labs-code/design.md) 스펙을 따르는 `web/DESIGN.md` 가 유일한 출처이고 `npm run theme:generate` 가 거기서 MUI 테마·Tailwind 토큰·AG Grid 테마를 굽는다. 런타임에 `--mui-palette-*` 를 참조하지 않고 빌드 시점에 굽는 이유는 `index.css` 가 JS 보다 먼저 로드되기 때문이다 — 의존하면 첫 페인트에서 값이 비어 색이 무너진다. CI 가 세 가지를 막는다: 생성물 신선도(`npm run theme:check`), DESIGN.md 유효성(`@google/design.md lint`), 토큰 대비 AA(`contrast-audit.test.ts` 45건). Tailwind v4 와는 `@layer theme, base, mui, components, utilities` 순서 + `StyledEngineProvider enableCssLayer` + `@theme inline` 로 붙였다 — 순서를 선언하지 않으면 MUI 의 emotion 런타임 주입이 Tailwind 유틸리티를 덮어쓴다.
- **화면 정보 인벤토리 대조와 시각 회귀 게이트** (`web/scripts/extract-ui-inventory.mjs`, `web/e2e/visual/screens.spec.ts`, `.github/workflows/ci.yml`): UI 를 대규모로 손볼 때 필드·컬럼·라벨이 조용히 사라지는 것을 막는다. 전자는 TypeScript AST 로 각 화면의 i18n 키·표시 문자열·표 컬럼·라벨을 뽑아 스냅샷과 대조한다(스타일은 의도적으로 추적하지 않는다 — 그건 바뀌어야 하는 것이다). 판정 기준은 "이 문자열이 소스 어딘가에 아직 있는가" 라서 파일과 필드 종류를 넘나들며 찾는다 — 컴포넌트 추출이나 하드코딩 라벨을 `t()` 폴백으로 옮기는 리팩터링을 막지 않는다. 후자는 화면 28개 × 두 테마 = 58장을 고정한다. 로그인이 프론트엔드 목 인증이라 백엔드 없이 돌고, `/api/v1/*` 를 빈 응답으로 스텁해 빈 상태까지 렌더한다. 스텁 경로를 정확히 맞춰야 한다 — `**/api/**` 같은 글롭은 Vite 가 서빙하는 소스 모듈(`/src/features/admin/api/*.ts`)까지 가로채 lazy 라우트를 깨뜨린다.

- **GitHub 스택에서 파이프라인 프로비저닝 지원** (`internal/cicd/adapter/github/`, `internal/cicd/adapter/provisioning/bundle_factory.go`): `github-argocd-v1` 템플릿은 예전부터 설치는 됐지만 파이프라인을 만들 수 없었다 — 번들 팩토리가 self-hosted GitLab 이 아니면 그 자리에서 거절했기 때문이다. 이제 소스 저장소 도구에 따라 GitLab/GitHub 어댑터를 골라 조립한다. GitHub 은 SaaS 라 GitLab 과 두 가지가 근본적으로 다르다. 하나, Organization 을 API 로 만들 수 없어 `EnsureGroup` 은 존재 확인만 하고 없으면 "먼저 만들라"고 끊는다(개인 계정이면 `POST /user/repos` 로 간다). 둘, 리포 범위 토큰 API 가 없어 워크플로는 내장 `GITHUB_TOKEN`(`contents: write`)으로 매니페스트를 되쓰고, Argo CD·이미지 pull 인증에는 조직 PAT 를 재사용한다. 스캐폴딩은 `.github/workflows/nullus-ci.yml` 로 나가며 dind 배관과 `[skip ci]` 마커가 없다 — 호스티드 러너에는 Docker 데몬이 이미 있고, `GITHUB_TOKEN` 으로 만든 push 는 워크플로를 재트리거하지 않는다. Actions 시크릿은 평문을 받지 않으므로 리포 공개키로 sealed box 암호화해서 올린다. PAT 와 organization·API 주소는 `token_sources`(provider=`github`, metadata 의 `owner`/`api_base_url`)에서 읽는다.
- **설치 마법사에서 GitHub 연동 정보를 입력** (`web/.../stack-install-page.tsx`, `internal/stack/usecase/token_source_inputs.go`): 소스 저장소로 GitHub 을 고르면 Organization·API 주소·PAT 입력이 나타난다. **PAT 는 스택 구성에 저장하지 않는다** — `stacks.config` 는 평문 JSONB 로 저장되고 조회 API 로 다시 내려오므로, 토큰을 넣으면 스택을 볼 수 있는 누구에게나 노출된다. 대신 배포 요청 본문(`source_control.personal_access_token`)으로만 흘려보내고, 설치가 끝나는 시점에 그 스택의 OpenBao 로 옮긴다(`kv/nullus/{env}/{org}/cicd/github/api-token`). Organization 은 비밀이 아니라서 구성에 남고 `token_sources.metadata` 로 전달된다 — 토큰만으로는 어느 org 에 리포를 만들지 알 수 없기 때문이다. 재시도·이어하기도 같은 본문을 받는다: 첫 설치가 등록 직전에 실패하면 OpenBao 에 아무것도 없어, 여기서 다시 받지 않으면 복구할 방법이 없다. 경로 문자열은 쓰는 쪽(stack)과 읽는 쪽(cicd)이 다른 모듈이라 `internal/shared/secrets/paths.go` 로 단일화했다 — 한쪽만 바뀌면 컴파일은 통과하고 파이프라인 생성에서만 "등록된 PAT 가 없다" 로 드러난다. 이 등록은 `authentication.provider` 선택과 **무관하게** 일어난다: 시크릿 평면(OpenBao)은 그 값과 상관없이 항상 설치되는데(PostgreSQL·MinIO 가 `provisioning_secrets` 가 만든 Secret 을 참조한다) 마법사 기본값은 `provider=''` 라, 회전 대상 항목과 같은 게이트에 두면 사용자가 입력한 토큰이 조용히 사라진다 — 등록 실패가 아니라 "등록할 것이 없음" 이라 설치 로그에 경고도 남지 않는다. 반대로 회전 대상 항목은 종전 조건을 유지한다. 그쪽은 회전 컨트롤러가 값을 채우는 빈 경로라 범위를 넓히면 영영 채워지지 않는 행만 늘어난다.
- **GHCR 을 컨테이너 레지스트리 선택지로 추가** (`internal/cicd/adapter/registry/resolver.go`): 다른 레지스트리와 달리 사용자가 등록할 자격증명이 없다 — GitHub Actions 잡이 `packages: write` 권한의 내장 토큰으로 자기 리포 패키지에 push 할 수 있기 때문이다. 경로는 항상 소문자로 만든다. GHCR 은 대문자가 섞인 경로를 거부하는데 그 오류가 권한 문제처럼 보여 원인을 찾기 어렵다.
- **플랫폼 도구 상태를 실측으로 채우고 모니터링 화면에 노출** (`internal/observability/adapter/toolhealth/`, `web/.../platform-tool-health.tsx`): `/observability/dashboard` 의 `tool_health` 는 지금까지 죽은 필드였다 — 프로덕션 경로인 Prometheus 리포지토리는 목록을 아예 채우지 않았고, Prometheus 미설정 시 폴백만 하드코딩된 가짜 값(Harbor 는 늘 `running`, Nexus 는 아예 없음)을 돌려줬으며, 이 값을 그리는 화면도 없었다. 이제 설치된 스택의 실제 파드에서 상태를 뽑는다. 같은 도구가 여러 스택에 있으면 한 줄로 합치고 가장 나쁜 상태를 남긴다. 실측 조회에 실패하면 시뮬레이션 값으로 되돌아가지 않고 목록을 비운다 — 죽은 도구가 `running` 으로 보이는 것이 가장 나쁜 오답이기 때문이다. 화면은 Monitoring 페이지 상단 카드로, 클러스터·스택 선택과 무관하게 항상 보인다.

- **파이프라인 삭제 시 부수 리소스를 골라서 함께 정리** (`internal/cicd/usecase/delete_pipeline.go`, `web/.../delete-pipeline-dialog.tsx`): 지금까지 파이프라인 삭제는 `DELETE FROM pipelines` 한 줄이 전부였다. Argo CD Application 이 고아로 남아 계속 동기화하므로 **목록에서는 사라졌는데 앱은 계속 돌았고**, 저장소·이미지도 그대로 남았다. 이제 확인 대화상자에서 클러스터 리소스·컨테이너 이미지·소스 저장소를 각각 고른다. 셋 다 기본은 꺼짐이다 — 종전 동작을 유지하고, 되돌릴 수 없는 일과 서비스가 멈추는 일은 명시적으로 요청받는다. 저장소를 고르면 이름을 그대로 입력해야 확인 버튼이 열린다. 클러스터 정리는 Application 에 `resources-finalizer.argocd.argoproj.io` 를 붙인 뒤 삭제해 Argo CD 가 워크로드까지 걷어내게 한다 — 생성 시점에 넣지 않는 이유는 그러면 Application 을 손으로 지울 때도 항상 배포가 함께 사라지기 때문이다. 요청한 삭제가 하나라도 실패하면 레코드를 남긴다. 레코드가 사라지면 목록에서 안 보이는데 리소스는 남아 다시 시도할 방법조차 없어진다. 이미지 삭제 수단이 없는 레지스트리(Harbor·Nexus)는 조용히 건너뛰지 않고 `IMAGE_DELETION_UNSUPPORTED` 로 끊는다 — 넘어가면 사용자는 지워진 줄 안다. 공용 `common` 저장소는 여러 앱이 공유하므로 대상이 아니다.

### Changed

- **템플릿 편집 모달을 설치 마법사와 같은 구조로 바꾼다** (`web/src/features/stack/components/template-tool-editor.tsx`, `web/src/components/ui/modal.tsx`): 같은 일(스택을 이루는 OSS 고르기)을 하는 두 화면이 서로 다르게 생겨 있었다. 편집 모달은 섹션이 아코디언으로 세로로 쌓이고 그 안에 한 줄짜리 6열 그리드가 들어 있었는데, 카테고리 칸이 `0.7fr` 고정이라 "Package Registry"·"Source Repository" 가 두 줄로 접혔고, **고를 수 있는 도구는 좁은 `<select>` 안에 갇혀 열어 보기 전에는 무엇이 있는지 알 수 없었다.** 이제 섹션이 탭으로 갈리고 탭 안에서 도구가 전폭 카드로 펼쳐진다 — 설치 마법사의 `ToolSelector` 와 같은 읽는 순서다. 마법사와 다른 것은 버전뿐이다: 템플릿은 helm/app 버전을 함께 pin 해야 하므로 그 둘은 카드 아래 '상세' 줄로 내리고 적용은 명시적 버튼으로 남긴다(관리자가 버전을 타이핑하는 도중에 저장이 일어나면 안 된다). 모달에 `size="xl"`(1080) 을 더했다 — 기존 `wide`(800) 를 키우지 않은 이유는 그 값을 쓰는 자리가 열 곳이 넘고 대부분은 폼 두 칸짜리라 넓히면 오히려 휑해지기 때문이다.

  **편집을 열 때 선택이 반쪽만 살아 있었다.** 화면에는 템플릿의 도구가 골라진 채로 떴지만 그것은 "적용된 값"이 draft 를 이기게 해 둔 결과였고, 그 대가로 **이미 템플릿에 담긴 카테고리는 다른 도구를 눌러도 선택이 움직이지 않았다** — `<select>` 시절부터 있던 것이 카드로 바뀌며 드러났다. 이제 모달을 열 때 draft 에 템플릿 값을 심고 draft 를 단일 출처로 쓴다. 초기 선택은 그대로이고 클릭이 즉시 반영된다.

  도구 설명은 **id 가 한 제품만 가리킬 때만** 붙인다. 문구 키가 `stackAddTools.tools.<id>` 하나뿐인데 GitLab 은 소스 저장소이자 패키지 레지스트리라, id 로 찾으면 "GitLab Package Registry" 밑에 "GitLab source code management" 가 붙는다(실제로 그렇게 떴다). 이름이 정확히 맞아도 부족하다 — 문구는 `t()` 로 읽고 그 키가 id 기반이라 번역이 있는 한 언제나 엉뚱한 쪽이 이긴다. 설명이 빠지는 항목이 생기지만 틀린 설명보다 낫다. 제대로 고치려면 키를 (카테고리, id) 쌍으로 나눠야 하고 그때는 설치 마법사도 함께 고쳐야 한다 — **그 화면에도 같은 오표기가 남아 있다.**

- **스택 상세 Config 탭을 좌우 2단으로 바꾼다** (`web/src/features/stack/components/stack-config-tab.tsx`): 릴리스 선택이 가로로 깔려 있어 OSS 가 열댓 개인 스택에서는 그것만으로 패널 위쪽 절반을 먹었고, 정작 편집기는 바깥 스크롤 아래로 밀려났다 — 설정을 한 줄 고치려면 매번 스크롤을 내려야 했다. 릴리스를 왼쪽 레일(240px)로 세우고 오른쪽을 편집기에 준다. 레일 너비는 가장 긴 이름(`kube-prometheus-stack`)에 맞췄고 이름은 자르지 않는다 — 잘리면 어느 OSS 인지가 사라진다. 좁은 화면에서는 레일이 위로 올라오므로 여러 열로 눕히고 높이를 묶는다.

  **탭이 패널 높이를 그대로 채우고 남는 높이는 편집기가 전부 가져간다**(고정 440px 폐기). 세로를 되찾은 만큼 바깥 스크롤이 사라진다. 다만 편집기에 최소 높이를 둔다 — 화면이 아주 낮을 때 편집기를 짜부라뜨리는 것보다 바깥이 스크롤되는 편이 낫다. 목록과 미리보기는 자기 안에서만 스크롤한다: 어느 쪽이든 바깥으로 넘치면 편집기가 밀려 원래 문제로 돌아간다. `min-h-0` 이 빠지면 flex 자식이 제 콘텐츠 높이 아래로 줄지 못해 목록이 레일 밖으로 흐르는데, 실제로 한 번 밟고 브라우저에서 높이를 재서 찾았다.

- **`github-argocd-v1` 호환성 매트릭스를 verified 로 올린다** (`internal/stack/adapter/repository/memory_compatibility.go`, `db/migrations/000066_github_stack_verified.up.sql`): 출하되는 여섯 매트릭스 중 이것만 `untested` 로 남아 있었다. 실패 지점이 나머지와 다르기 때문이다 — 소스·CI·레지스트리가 전부 클러스터 밖이고, 파이프라인 프로비저닝이 GitLab 이 아니라 GitHub 어댑터(`internal/cicd/adapter/github/`)를 탄다. 그 경로를 확인해 verified 로 올린다. 설치 전 검사에서 둘이 달라진다: `MATRIX_UNTESTED` 경고가 사라져 판정이 warn(70) → pass(100) 가 되고 배포에 명시적 ack 가 필요 없어지며, 아키텍처 불일치의 처리가 warn 유지에서 fail 로 뒤집힌다(이 조합의 도구는 전부 amd64+arm64 라 실제로 걸리는 것은 그 둘이 아닌 노드가 섞였을 때뿐이고, 그때는 막는 쪽이 맞다). Tier 도 stable 로 함께 올린다 — 000041 이 세운 "verified 매트릭스는 stable" 규칙을 유지하기 위해서고, MinIO·Argo CD·Prometheus·Grafana 는 같은 차트를 같은 버전으로 설치하면서 여기서만 beta 로 남을 이유가 없다.

  **게이트의 warn 분기 테스트를 전용 행렬로 옮겼다** (`deploy_handler_compat_test.go`, `retry_handler_test.go`). 그 분기를 `github-argocd-v1` 로 시험하고 있어서, 이 조합이 verified 로 올라가는 순간 게이트 로직은 그대로인데 테스트 다섯 개가 함께 죽었다. 출하되는 Golden Path 를 빌려 쓰면 그 조합의 검증 상태가 바뀔 때마다 관계없는 테스트가 깨진다 — 시드 데이터 모양에서 떼어 놓는다. (usecase 쪽은 이미 같은 이유로 전용 행렬을 쓰고 있었다.)

- **메인 화면의 지원 OSS 목록을 실제 배선이 있는 것으로 좁힌다** (`web/src/features/home/utils/support-tools.ts`): 목록의 출처가 설치 마법사의 선택지여서 23개가 전부 떴는데, 그중 Gitea·Jenkins·Flux CD·Spinnaker·JFrog·Docker Hub·Thanos·VictoriaMetrics·OpenSearch Dashboards 는 **고를 수는 있지만 배포해도 아무것도 설치되지 않는다** — helm 단계 카탈로그에도, `resolveChartSpecForStep` 의 차트 분기에도 없다. 프론트의 `TOOL_HELM_META` 에 차트 이름만 적혀 있어 지원하는 것처럼 보였을 뿐이다(`docker-hub` 의 `dockerhub/proxy-cache` 는 실재하지 않는 이름이다). 이제 기준은 "설치되거나, 클러스터 밖이지만 연동 구현체가 있거나" 이고 15장이 남는다. 테스트의 방향도 뒤집었다 — 예전 규칙("마법사의 모든 선택지가 카드여야 한다")이 바로 이 상태를 만든 원인이었다. 이제 마법사 선택지는 **카드이거나 `NOT_INSTALLABLE` 에 이유와 함께 선언되어 있거나** 둘 중 하나여야 통과한다: 새 OSS 가 조용히 빠지는 것도, 배선 없는 도구가 조용히 "지원" 으로 붙는 것도 막는다.

- **아이콘을 기본적으로 읽는 도구에서 숨긴다** (`web/src/components/ui/icon.ts`): 아이콘 314곳 중 `aria-label` 을 가진 것이 **0곳**이었다. lucide 는 아무 것도 붙이지 않은 `<svg>` 를 내보내므로 이름 없는 그래픽 300여 개가 그대로 읽혔다 — 화면을 소리로 듣는 사람에게는 잡음이다. 거의 모든 자리에서 아이콘은 옆 글자를 거드는 장식이고 아이콘만 있는 버튼은 버튼 쪽에 이름이 붙으므로, `iconProps()` 가 `aria-hidden` 을 기본으로 준다. 아이콘이 홀로 뜻을 지는 자리만 `StatusIcon` 의 `label` 로 이름을 준다. 진짜 이름이 없던 아이콘 전용 버튼 5곳에는 `aria-label` 을 붙였다(AST 로 훑어 찾았다 — 정규식으로 보면 `{t(...)}` 를 글자로 세지 못해 오탐이 난다).
- **사이드바 메뉴 아이콘의 뜻을 바로잡고 스택/CI/CD 를 구별한다** (`web/src/components/layout/nav-model.tsx`): 크기·굵기를 통일하는 것과 **어떤 글리프가 그 메뉴를 뜻하는지**는 다른 문제다. 메뉴 22개를 다시 보니 8곳이 어긋나 있었다. 같은 그룹 안에서 겹친 것 — 스택 버전과 스택 버전 관리가 둘 다 `Shield`, 조직이 "관리" 그룹 헤더와 같은 `Settings`, 모니터링 대시보드가 "관측성" 헤더와 같은 `ChartColumn`(그룹을 접으면 구분되지 않는다). 뜻이 틀린 것 — **알림 이력이 `BellOff`** 였는데 그건 "알림 끔" 이고, 스택 이력·CI/CD 이력이 둘 다 `History` 인데 여기만 어긋났다. **알려진 이슈는 `TriangleAlert`** 라 경고 *상태* 글리프와 같은 모양이어서 상태로 오해됐다(`Bug` 로 바꾸면서 래칫 예외도 하나 사라졌다). 스택 버전의 `Shield` 는 보안으로 읽혀 `Tag` 로, 호환성 매트릭스를 다루는 스택 버전 관리만 `ShieldCheck` 로 남겼다.

  **템플릿·목록은 스택과 CI/CD 가 같은 글리프를 쓰고 있어 갈랐다**: CI/CD 템플릿은 `FileCode2`(파이프라인 템플릿은 실제로 워크플로 정의 파일이고, 이미 CI/CD 화면들이 그 뜻으로 쓰고 있다), CI/CD 목록은 `Workflow`(파이프라인은 줄이 아니라 흐름이다). 기준은 **아이콘이 그 화면의 '대상'을 그린다** 는 것이고, 스택 쪽이 기본형을 갖고 CI/CD 가 도메인 고유 글리프를 갖는다. 같은 이유로 이력 3곳은 `History` 로 남는다 — 셋 다 대상이 시간이다. `Layers` 는 후보에서 뺐다. 모니터링이 "파이프라인 수" 에 이미 쓰고 있어 뜻이 겹친다.
- **화면이 각자 들고 있던 상태→아이콘 표를 레지스트리로 걷어낸다** (`web/src/features/admin/pages/cluster-page.tsx`, `web/src/features/observability/components/monitoring-chart-widgets.tsx`): 앱을 실제로 띄워 보고 드러났다 — `StatusBadge` 를 쓰지 않는 화면들이 자기 `STATUS_CONFIG` 를 들고 있어서, 레지스트리를 세워도 거기만 옛 매핑으로 남았다. 클러스터 관리 화면은 **대기가 시계(`Clock`)**, 연결 불가와 인증 실패가 둘 다 `CircleAlert` 였고, 도구 상태 카드는 **경고가 원(`CircleAlert`)** 이었다 — 다른 화면에서는 각각 `CircleDashed` · `TriangleAlert` · `CircleX` 다. 이제 두 화면 모두 tone 만 정하고 글리프와 색은 레지스트리가 준다. 앞으로의 드리프트는 **래칫 테스트**가 막는다: 상태 전용 글리프를 새로 import 하는 파일이 생기면 실패하고, 기존 16곳은 목록에 적어 두되 그 목록이 낡으면(이미 옮겼는데 안 지웠으면) 그것도 실패한다. 목록에 남는 것 중 일부는 상태가 아니라 영영 남는다 — `nav-model` 의 `TriangleAlert` 는 "Known Issues" 메뉴 아이콘이고, `confirm-dialog` 의 것은 파괴적 동작 경고다.
- **Tailwind 기본 팔레트 직접 사용을 토큰으로 옮기고 클래스명 우회를 막는다** (`web/src/features/observability/`, `web/eslint.config.js`): `text-emerald-400` · `bg-amber-500/15` 처럼 팔레트를 직접 쓴 곳이 17곳 있었다. 그 초록은 `--color-success` 와 다른 값이라 **같은 "정상"이 화면마다 다른 초록**이었다. hex 금지 규칙이 이걸 못 잡은 이유는 클래스명이 그냥 문자열이기 때문이다. 전부 토큰으로 옮기고, 팔레트 클래스와 숫자 아이콘 크기를 각각 막는 `no-restricted-syntax` 규칙을 추가했다(둘 다 실제로 걸리는지 확인했다). 로고 `NullusMark` 는 크기 규칙에서 뺀다 — 로고는 아이콘이 아니라서 자리마다 크기가 따로 정해진다(로그인 52px, 홈 히어로 80px).

- **화면 정보 인벤토리 스냅샷 갱신** (`web/e2e/inventory/ui-inventory.json`, `docs/40_UI_UX/화면_정보_인벤토리.md`): 로그인 카드의 임시 로고였던 글자 "N" 이 마크로 바뀌면서 게이트에 걸렸다. 다시 구우면서 **이번 브랜치와 무관한 드리프트 2건이 함께 흡수됐다** — 스냅샷이 직전 개편(#134) 이후로 갱신되지 않아 이미 어긋나 있던 것이라 남겨 둘 수가 없었다. (1) `monitoring-cicd-view.tsx` 의 안내 문구와 라이브 패널 항목들은 `app-runtime-panels.tsx` 로 컴포넌트가 분리되면서 옮겨 간 것이다 — 화면에는 그대로 나온다(추출기가 지역 문자열 객체의 값은 훑지 않아 유실로 잡혔다). (2) `stack-history-page.tsx` 는 Stack Name·Cluster 컬럼과 `stackHistoryPage.table.stackName` 키를 실제로 잃었다. 페이지가 스택을 드롭다운으로 고르는 형태라 행마다 스택 이름을 반복할 이유는 없어 보이지만, **이 브랜치가 판단할 일이 아니므로 의도된 제거인지는 확인이 필요하다.**

- **호환성 매트릭스의 기준선을 외부 프로젝트에서 실제 설치 경로로 이관** (`internal/stack/domain/connection.go`, `db/migrations/000062_compat_baseline_matches_install.up.sql`): 매트릭스는 Pre-Deploy Gate 에서 "검증된 조합" 으로 화면에 뜨는데, 그 값이 실제로 깔리는 버전과 갈라져 있었다(GitLab 9.5.1 vs 8.7.2, Argo CD 6.8.0 vs 7.7.16, Prometheus 67.0.0 vs 69.3.0, Grafana 8.5.0 vs 8.9.0, MinIO 5.2.0 vs 5.4.0). 원인은 기준선을 외부 프로젝트 Narwhal(`dasomel/narwhal`)의 `VERSIONS.md` 에 둔 것이다 — Nullus 의 설치 경로가 독자적으로 올라가면서 따라갈 이유가 사라졌는데 값만 남았다. Harbor·Nexus 만 어긋나지 않았는데 그 둘만 `domain` 상수를 참조했기 때문이다. 이제 출처는 `domain` 상수 하나이고 차트 스펙과 매트릭스가 같은 상수를 본다. 드리프트 테스트를 클러스터에 설치되는 전 도구로 넓혔고(2건 → 11건), 매트릭스 테스트도 리터럴 대신 상수를 참조하게 해 세 번째 출처가 생기지 않게 했다. Prometheus 의 app 버전만 차트 `appVersion` 을 그대로 쓰지 않는다 — 그 값은 prometheus-operator 버전이라 화면에 오퍼레이터 버전이 Prometheus 인 양 뜬다. 화면 문구에서도 외부 프로젝트 이름을 뺐다.
- **셀렉트 펼침 목록을 브라우저 기본에서 테마 목록으로** (`web/src/components/ui/select.tsx`): `NativeSelect` 는 진짜 `<select>` 라 펼친 목록이 OS 위젯이었다 — 다크 테마에서 흰 목록이 떴다. MUI `Select`(포털 Menu)로 바꾸면서 `<option>`/`<optgroup>` 을 `MenuItem`/`ListSubheader` 로 변환하는 어댑터를 유지해 소비자 코드를 그대로 뒀다. react-hook-form `register()` 는 DOM `.value` 를 직접 쓰므로 `reset()` 후 표시가 조용히 어긋난다 — 6곳을 `Controller` 로 옮겼다. 이름이 더 이상 네이티브가 아니게 되어 `Select` 로 바꿨다.
- **파이프라인 토폴로지를 역할별 스테이지 레일로** (`web/src/features/stack/components/pipeline-topology.tsx`): 도구를 나열하던 그림을 스테이지 레일 + 로고 형태로 바꾸고, Artifacts 를 용도(소스·패키지·이미지·스토리지)별로 분리했다. GitLab 처럼 한 도구가 세 역할을 겸하면 `shared` 로 묶어 한 번만 그린다. 모니터링 대시보드에 있던 "플랫폼 도구 상태" 카드를 걷어내고 그 정보를 이 그림 안에 넣었다 — 배치와 동작을 같은 그림에서 읽는 편이 낫고, 그 카드는 클러스터·스택을 고르기도 전에 떠 있어 "선택해서 시작하라" 는 화면 흐름과 어긋났다.
- **화면 껍데기와 제목을 뷰포트에 고정** (`web/src/app/layout.tsx`, `web/src/components/layout/page-header.tsx`): 본문을 스크롤해도 사이드바와 화면 제목이 제자리에 있다. 사이드바는 본문 길이를 따라 늘어나 로그아웃 줄이 화면 밖으로 밀려 있었다(1938px). 제목은 `position: sticky` 로 고정하되 본문이 그 위로 스쳐 지나가지 않게 배경을 full-bleed 로 깔았고, 스크롤 컨테이너의 `padding-top` 을 걷었다 — `top: 0` 은 콘텐츠 박스를 기준으로 붙으므로 패딩이 남아 있으면 그만큼 틈이 생긴다. 상단 경로도 `상위메뉴 › 하위메뉴` 로 바꿨다.

- **공용 UI 프리미티브 6종의 내부를 MUI 로 교체** (`web/src/components/ui/`): Button·Input·NativeSelect·Skeleton·Modal·Card 를 MUI v9 로 바꿨다. **파일 경로·export 이름·prop 시그니처를 그대로 둔 어댑터 방식**이라 28개 화면 18,823 LOC 을 한 줄도 고치지 않고 Button 124곳·Input 95곳·Modal 11곳이 한 번에 통일됐다. 색은 전부 토큰을 참조해 테마 전환에 따라간다 — 이전 구현은 `#a5b4fc`·`#f87171` 같은 다크 기준 hex 를 박아서 라이트 테마에서 1.91:1 까지 무너졌다. Select 가 아니라 **NativeSelect** 를 쓴 이유는 MUI `Select` 가 진짜 `<select>` 대신 listbox 를 렌더해서 소비자가 넘기는 `<option>` children 과 테스트의 `getByRole('combobox')`·`fireEvent.change` 가 전부 깨지기 때문이다. Modal 은 손으로 만든 포커스 트랩 ~70줄을 지웠다(MUI Modal 이 첫 요소 포커스·Tab 순환·닫힐 때 복원·Esc·스크롤 락을 모두 한다). 다만 `if (!open) return null` 로 즉시 언마운트하는 동작은 유지했다 — 이전 구현에 퇴장 애니메이션이 없었고 소비자 테스트 3곳이 그 동기적 제거에 의존한다(Cancel 클릭 직후 `queryByText(...).not.toBeInTheDocument()`). MUI 전환에 맡기면 전환 시간만큼 DOM 에 남아 그 계약이 깨진다.
- **배포 이력의 행 확장을 좌우 분할 상세로 대체** (`web/src/features/cicd/pages/cicd-history-page.tsx`, `web/src/components/shared/data-table.tsx`): 행을 펼쳐 보여주던 상세 패널의 6개 필드가 **메인 테이블 컬럼과 완전히 같았다** — 같은 행을 세로로 다시 그린 것이라 새 정보가 0 이었다. 게다가 행 확장은 28개 화면 중 이 한 곳뿐인 일회성 패턴인데, 좌우 분할 상세는 `list-detail-panel` 로 컴포넌트화돼 3화면이 쓴다. 이제 행을 선택하면 표 **오른쪽**에 상세가 뜬다 — 처음에는 표 아래에 두었다가 좌우로 옮겼다. 아래로 펼치면 행을 고를 때마다 표가 밀려 내려가 방금 고른 행을 잃고, 배포가 몇 건뿐일 때는 그 아래로 화면 절반이 빈 채 남는다. 선택 행이 필터 결과에서 빠지면 상세도 함께 사라진다 — 목록에 없는 항목의 상세가 남아 있으면 안 된다. `DataTable` 에서 아무도 쓰지 않게 된 `expandedRowId`/`renderExpanded` prop 을 제거했다. 실제 하위 엔티티(배포 단계 `steps`)는 `GET /cicd/deployments/{id}` 에 이미 있지만 그걸 붙이는 것은 기능 추가라 별 티켓으로 분리했다.
- **테마 소유권을 theme-store 하나로 통일** (`web/src/theme/theme-sync.tsx`, `web/src/stores/theme-store.ts`): MUI 테마에 `cssVariables` + `colorSchemeSelector: '[data-theme=%s]'` 를 주면 MUI 가 `<html data-theme>` 를 직접 관리한다. 그런데 이 앱은 이미 zustand `theme-store` 가 같은 속성을 쓰고 있어서, **MUI 의 `defaultMode` 가 스토어 값을 덮어써 라이트 테마가 다크로 렌더됐다**(시각 회귀 스냅샷을 눈으로 보고 발견했다). 스토어를 단일 소유자로 두고 영속화 키를 MUI `modeStorageKey` 와 공유하고, `ThemeSync` 가 스토어 → MUI 로 mode 를 밀어 넣는다.
- **⚠️ `github-argocd-v1` 의 컨테이너 레지스트리를 Harbor 에서 GHCR 로 교체** (`db/migrations/000060_github_stack_uses_ghcr.up.sql`, `internal/stack/adapter/repository/memory_template.go`): 이 스택은 GitHub 호스티드 러너에서 빌드하는데, 러너는 GitHub 네트워크에 있어 클러스터 내부 Harbor(`harbor.<access_domain>`, 보통 `.internal`)에 닿을 수 없다 — 즉 `docker push` 가 반드시 실패하는 조합이었다. 이미지는 러너가 닿을 수 있는 GHCR 에 올린다. Harbor 가 빠지면서 최소 자원은 `6 vCPU / 12Gi / 80Gi` → `4 vCPU / 8Gi / 50Gi`, 예상 설치 시간은 60분 → 45분으로 줄었다. 호환성 행렬에서도 Harbor 의 amd64 전용 제약이 사라져, arm64 클러스터에서 설치할 수도 없는 도구를 이유로 게이트가 막던 문제가 함께 해소된다. **기존에 이 템플릿으로 만든 스택은 영향을 받지 않는다**(설치 시점의 선택이 스택에 저장되므로).
- **토큰 소스 등록이 스택 범위 시크릿 저장소를 쓰도록 수정** (`internal/stack/adapter/repository/postgres_token_source_registry.go`): `Upsert` 가 토큰 값을 전역 저장소에 기록했는데, OpenBao 는 스택마다 배포되므로 스택 범위로 읽는 쪽(cicd 모듈)이 값을 찾지 못한다. `StackID` 가 실린 항목은 그 스택의 저장소에 쓴다. 지금까지는 설치 경로가 값을 실어 보내지 않아(회전 컨트롤러가 나중에 채우는 구조) 드러나지 않았다. 아울러 `token_sources.metadata` 에 호출자 값을 병합할 수 있게 했다 — 고정 필드 위에 얹으므로 `secret_manager` 같은 값이 덮이지 않는다.
- **외부 SaaS 도구가 회전 대상 토큰 소스로 등록되지 않도록 제외** (`internal/stack/usecase/token_source_inputs.go`): GitHub·GitHub Actions·GHCR 은 우리가 토큰을 발급할 수 없는데도 `kv/.../artifacts/github/token` 같은 항목이 만들어지고 있었다. 값이 영영 채워지지 않는 죽은 행인 데다, GitHub PAT 항목과 provider 가 같아 소유자 정보가 없는 행이 하나 더 생겨 연동 설정 조회가 둘 중 어느 것을 집을지 알 수 없게 된다.
- **GitHub·GitHub Actions 에 걸려 있던 actions-runner-controller 차트 매핑 제거** (`web/src/features/stack/utils/install-constants.ts`): 프론트는 두 도구를 ARC 차트로 설치할 것처럼 계획을 세웠지만 백엔드는 `external` 로 표시해 설치를 건너뛰고 있었다 — 설치 계획이 실제 동작과 어긋나 있었다. 러너는 GitHub 에서 돌므로 클러스터에 설치할 것이 없다.
- **⚠️ 차트 기본 인증 모드를 `session` 에서 `oidc` 로 변경** (`deploy/helm/nullus/values.yaml`): `session` 은 클라이언트가 보낸 `X-User-ID`·`X-User-Role` 헤더를 그대로 믿는 알파 시절 방식이라 아무나 `X-User-Role: admin` 을 붙이면 관리자가 된다(코드 주석도 "simplified for alpha" 라고 인정). 그런데 그것이 차트 기본값이었다. SPA 폴백 기본값도 `session` 이라 한쪽만 바꾸면 프론트는 세션 헤더를 보내는데 API 는 JWT 를 기다려 전부 401 이 되므로 `config.auth.mode` 와 `web.auth.mode` 를 함께 옮긴다. **기본값으로 설치하려면 이제 IdP 설정이 필요하다** — `config.auth.oidcIssuerUrl`, `web.auth.{oidcProvider,oidcAuthority,oidcClientId}`. 기존에 `oidc` 를 명시하던 배포(zadara 등)는 영향이 없다.
- **레이트리밋을 IP 상한 + 사용자 한도 2단으로 분리** (`internal/shared/middleware/rate_limiter.go`): 전역은 IP 기준 폭주 상한(600/분), 인증 그룹에는 사용자 키 리미터(300/분)를 붙인다. 사용자 리미터는 `RequireRole` 앞에 둬 403 으로 튕기는 요청도 사용량에 잡힌다. development 모드는 인증 미들웨어를 아예 켜지 않으므로 익명 한도를 인증 한도와 같게 둔다 — 5초마다 폴링하는 화면 하나만 열어도 429 가 나던 문제가 사라진다.

### Fixed

- **잘 돌고 있는 설치가 실패로 뒤집히고, 로그는 초반에서 끊기고, 진행률은 5% 에서 굳던 것** (`internal/stack/usecase/install_stack.go`, `internal/stack/adapter/log/memory_streamer.go`): 세 증상이 함께 나타났다. 설치가 "실패" 로 표시됐는데 오류 로그가 하나도 없고, 나중에 보면 게이트웨이는 만들어져 있었다.

  **하나 — 리퍼가 느린 단계를 죽은 설치로 오인했다.** 끊긴 설치를 찾는 리퍼는 갱신 시각만 본다. 그런데 그 시각은 단계가 시작·완료될 때만 움직인다. 한 단계가 임계값(30분)보다 오래 걸리면 — Harbor·GitLab 이미지 풀은 흔히 그렇다 — 멀쩡히 도는 설치가 끊긴 것으로 표시된다. 고루틴은 계속 돌고 있으니 게이트웨이는 "실패" 표시 뒤에 만들어졌고, 진짜 오류가 없었으니 오류 로그도 없었다.

  이제 설치가 도는 동안 2분마다 갱신 시각을 찍는다. 그러면 "시각이 멈췄다" 가 "설치를 돌리던 것이 사라졌다" 를 뜻하게 된다 — 리퍼가 원래 판정하려던 그것이다. 하트비트는 `TouchUpdatedAt` 으로 시각만 찍는다. `Update` 는 메모리에 든 스택 전체를 다시 쓰므로, 오래된 사본으로 하트비트를 찍으면 그 사이 다른 경로가 바꾼 값을 되돌린다.

  **둘 — 재접속하면 로그가 64줄에서 잘렸다.** 구독 채널을 고정 크기(64)로 잡고 히스토리를 `default:` 로 흘려보내, 버퍼가 차는 순간 이후가 전부 조용히 버려졌다. 설치 로그는 그보다 훨씬 길어서 화면에는 초반(cert-manager 언저리)까지만 남았다. 이제 히스토리 전체가 들어갈 만큼 채널을 잡는다.

  **셋 — 그래서 진행률이 5% 로 굳었다.** 화면은 마지막으로 받은 로그 항목의 진행률을 쓰는데, 그 항목이 초반에서 잘린 것이었다. 서버는 `/status` 로 저장된 스텝에서 되살린 올바른 값을 함께 내려주지만, 화면은 스트림 값이 0보다 크면 그쪽을 우선한다. 둘 다 옳게 만들어야 했고, 잘림을 고치면 스트림 값이 최신이 된다.

  히스토리에 상한(5000줄)도 뒀다. 버려야 한다면 오래된 쪽을 버린다 — 화면이 필요로 하는 것은 최근이고, 진행률은 마지막 항목에서 복원되기 때문이다.

- **UI 로 만든 파이프라인에 저장소도 CI job 도 없던 것** (`internal/cicd/usecase/create_pipeline.go`): 스택에 묶인 파이프라인을 만들어도 Gitea 저장소, Jenkinsfile, Jenkins job, Argo CD Application 이 하나도 만들어지지 않았다. 배포를 누르면 넘길 job 이 없다.

  만드는 코드는 있었다 — `ProvisionAppProject` 가 저장소를 만들고 스캐폴딩을 커밋하고 job 과 webhook 과 Argo CD Application 까지 만든다. **부르는 쪽이 없었다.** 그 경로는 `provision_repository` 플래그로 열리는데 프론트는 그 필드를 한 번도 보내지 않았다.

  이제 스택에 묶인 파이프라인은 플래그 없이도 준비한다. 통합모드가 그것들이 있다는 전제 위에 서 있기 때문이다 — 러너가 실행할 Jenkinsfile 도, Argo CD 가 동기화할 매니페스트도, 배포 실행을 넘길 job 도 전부 이 단계에서 만들어진다. `EnsureProject` 는 멱등해서 이미 있는 저장소는 그대로 쓴다.

  긴급 직접 배포(`emergency_direct`)와 스택에 묶이지 않은 파이프라인은 예전 그대로다. 프로비저닝 배선이 없는 구성에서는 자동 준비만 건너뛰고 파이프라인 생성은 그대로 된다 — 요청하지 않은 부가 작업이 본래 작업을 무너뜨리면 안 된다(명시적으로 요청했는데 배선이 없으면 예전처럼 오류다).

- **스택 파이프라인의 배포가 플랫폼 안에서 빌드하려다 죽던 것** (`internal/cicd/usecase/deploy_pipeline.go`, `internal/cicd/adapter/runner/` 신규, `internal/cicd/adapter/jenkins/client.go`): 설치된 스택에서 CI/CD 템플릿을 실행하면 첫 단계 `git clone` 에서 끝났다. API 파드에는 git 도 도커 데몬도 없다 — 이 경로는 API 서버가 host 에서 `go run` 으로 돌던 시절에 만들어졌다.

  git 을 이미지에 넣으면 clone 은 지나가지만 다음 단계 `docker build` 가 같은 이유로 멈춘다. 파드 안에 도커 데몬을 들이는 것이 답이 아니다 — 통합모드 설계가 이미 다른 답을 정해 두었다: *"Nullus API 서버가 정상 실행 과정에서 직접 git clone, docker build, kind load, kubectl apply 를 수행하지 않는다."*

  그런데 실행 경로는 `ExecutionMode` 를 보지 않았다. `DockerfilePath` 만 있으면 무조건 플랫폼이 빌드했다.

  이제 **스택에 묶인 파이프라인의 배포 실행은 스택의 CI 러너에게 넘어간다.** 플랫폼은 job 을 실행시키는 것으로 끝나고, 이어지는 일 — 빌드, 이미지 push, 매니페스트 태그 되커밋, 클러스터 동기화 — 는 Jenkins 와 Argo CD 가 한다. 그 결과는 이미 있던 `SyncPipelineRuns` 가 빌드 이력으로 들여와 화면에 보여 준다.

  매니페스트도 플랫폼이 적용하지 않는다. 러너가 되커밋한 매니페스트를 Argo CD 가 동기화하는데 플랫폼이 같은 리소스를 따로 적용하면 둘이 서로를 덮어쓴다.

  긴급 직접 배포(`emergency_direct`)와 스택에 묶이지 않은 파이프라인은 예전 경로 그대로다. 위임 대상인데 CI 러너가 없으면 조용히 직접 빌드로 되돌아가지 않고 무엇이 없는지 말하고 멈춘다 — 되돌아가 봐야 성공할 수 없는 경로이고, 그러면 사용자는 왜 실패했는지 알 수 없다.
- **CI/CD 템플릿 실행이 `error:` 한 줄만 남기고 죽던 것** (`Dockerfile`, `internal/cicd/adapter/docker/builder.go`): 설치된 스택에서 파이프라인을 돌리면 첫 단계에서 끝났다.

  ```
  Git Clone [failed]
  $ git clone --depth=1 https://gitea.nullus.io/root/spring-sample.git
  error:
  ```

  `error:` 뒤가 빈 것이 곧 단서였다. `CombinedOutput()` 은 **실행 파일 자체가 없으면 출력 없이 에러만** 돌려주는데, 호출부는 출력만 찍고 `err` 를 버렸다. 그래서 화면에는 원인이 한 글자도 남지 않았다.

  없던 것은 git 이다. API 이미지에는 helm·kubectl·migrate 는 넣어 두고 git 은 없었다 — 이 빌더는 host 에서 `go run` 하던 시절에 쓰였고, 그때는 git 이 늘 있었다.

  이미지에 git 을 넣고, 실패 메시지가 스스로를 설명하게 했다: 출력이 있으면 출력을, 없으면 `err` 를, 실행 파일이 없어서 실패한 것이면 무엇이 없는지 이름을 대고 끝낸다.

  **남은 것**: 두 번째 단계(`docker build`)는 아직 이 컨테이너 안에서 도커 데몬을 찾는데, 파드에는 데몬이 없다. 인클러스터 빌드를 어떤 방식으로 할지(dind 사이드카 / buildkit / kaniko / 스택의 러너에 위임)는 따로 정해야 한다.
- **Jenkins 만 SSO 로그인이 `invalid_scope` 로 튕기던 것** (`internal/stack/adapter/helm/oidc-values.go`): 같은 realm 에서 Argo CD·Harbor·Gitea 는 들어가지는데 Jenkins 만 로그인이 끝에서 실패했다.

  ```
  /securityRealm/finishLogin?error=invalid_scope&error_description=Invalid+scopes:
  openid offline_access profile email roles phone address service_account basic acr
  organization web-origins microprofile-jwt
  → Could not extract credentials from request
  ```

  요청한 스코프 목록이 곧 원인이다. 이것은 Jenkins 가 고른 값이 아니라 **realm 이 지원하는 전부**다 — oic-auth 는 스코프를 지정받지 못하면 "request all" 로 동작해 디스커버리 문서의 `scopes_supported` 를 그대로 요청한다. 그중 `service_account`·`basic`·`acr`·`organization` 은 이 클라이언트에 할당된 적이 없고, Keycloak 은 할당되지 않은 스코프를 보면 인가 요청 자체를 거절한다.

  기본값이면 충분하다고 보고 지정하지 않았던 것인데, 그 기본값이 "적당한 셋" 이 아니라 "전부" 였다.

  이제 `openid email profile` 로 좁힌다. 속성 이름은 `scopes` 가 아니라 `scopesOverride` 다 — `scopes` 는 manual 설정용이라 `wellKnown` 아래에 두면 JCasC 가 부팅을 막아 Jenkins 가 통째로 못 뜬다.
- **스택을 지워도 PVC 와 네임스페이스가 남던 것** (`internal/stack/usecase/delete_stack.go`, `internal/stack/adapter/handler/stack_handler.go`): 삭제가 성공했는데도 볼륨과 네임스페이스가 그대로 남았다. 다음 설치는 그 볼륨을 물려받아 옛 비밀번호를 쓰는 데이터베이스로 올라오고, 그 사실은 한참 뒤 Gitea 의 28P01 이나 Harbor 의 401 로 드러난다.

  정리가 **HTTP 요청 컨텍스트에 매달려 있었다.** 릴리스 uninstall 20여 개와 PVC 재시도(최대 6 × 10초)를 합치면 몇 분인데, 게이트웨이는 그만큼 기다려 주지 않는다. 연결이 끊기는 순간 컨텍스트가 죽고 kubectl 호출이 전부 즉시 실패한다 — 그리고 볼륨·네임스페이스 회수는 그 **다음** 단계라 아예 실행되지 않는다. 둘이 함께 남은 이유가 이것이다.

  설치는 이미 `context.WithoutCancel` 로 요청에서 떼어 놓고 있었다(`install_stack.go`). 삭제만 그렇지 않았다.

  이제 `ExecuteAsync` 가 스택 레코드까지만 요청 안에서 지우고, 클러스터 정리는 떼어낸 컨텍스트로 넘긴다(상한 30분). 레코드를 요청 안에서 지우는 이유는, 목록 새로고침이 방금 지운 스택을 다시 보여주면 사용자가 삭제가 실패한 줄 알기 때문이다. 정리 진행 상황은 이벤트 스트림으로 계속 나간다.

- **도구는 SSO 로 설정되는데 그 OIDC 클라이언트를 아무도 만들지 않던 것** (`internal/stack/adapter/helm/orchestrator.go`): Argo CD 에서 로그인하면 Keycloak 이 **`Client not found`** 를 돌려줬다. 리다이렉트는 정상이었고 `client_id=nullus-devsecops-stack-argocd` 도 제대로 실려 있었는데, 그 클라이언트가 realm 에 없었다.

  두 판단이 서로 다른 근거를 봤기 때문이다:

  | 무엇 | 조건 |
  |---|---|
  | 도구에 OIDC 설정을 넣는다 | 플랫폼이 Keycloak 을 가리키는가(provisioner + issuer) |
  | Keycloak 에 클라이언트를 만든다 | 스택 설정의 `authentication.provider` 가 `openbao` 인가 |

  스택이 인증 공급자를 고르지 않으면 도구만 SSO 로 설정되고 클라이언트는 만들어지지 않는다. 설치 로그에는 `skipping disabled stack install step step=provisioning_sso` 한 줄만 남아, 초록불 뒤에 숨는다.

  그 게이트가 지키려던 의존성(클라이언트 시크릿을 OpenBao 에서 읽는다)은 **이미 항상 충족된다** — 시크릿 평면은 `authentication.provider` 와 무관하게 늘 선다. 이제 클라이언트 등록도 도구 쪽 OIDC 값과 **같은 근거**로 판단한다. 도구에 OIDC 를 넣으면서 등록을 건너뛰는 조합은 테스트로 막았다.
- **설치는 성공했는데 gitea·harbor 주소만 열리지 않던 것** (`internal/stack/domain/gateway_backends.go` 신규, `internal/stack/adapter/helm/gateway-tls.go`, `web/src/features/stack/utils/install-manifest-builders.ts`): minio·argocd 는 열리는데 gitea·harbor 는 닫혀 있었다. 설치 마법사가 게이트웨이 라우트의 백엔드를 **`<도구>-svc:80` 으로 지어냈기** 때문이다. 실제 서비스 이름과 포트는 도구마다 다르다 — `gitea-http:3000`, `jenkins:8080`, `nexus:8081`.

  서버에 이것을 바로잡는 장치가 있었지만 **`grafana-svc` 와 `prometheus-svc` 둘만** 고쳤다. 같은 문제를 도구 하나씩 땜질해 온 자리다. 게다가 이름만 고치고 포트는 그대로 두어, 포트가 다른 도구는 이름을 고쳐도 연결되지 않았다.

  이제 도구별 백엔드가 `domain.GatewayBackendForTool` 한 곳에 있다. 서버가 마법사의 매니페스트를 받아 **이름과 포트를 함께** 바로잡고, 마법사도 같은 표를 쓴다. 두 값이 갈라지면 **서버 테스트가 마법사 파일을 직접 읽어** 걸러낸다(`TestGatewayBackends_MatchInstallWizard`) — 그 검사가 없어서 이 문제가 배포까지 갔다.

- **서버가 재시작되면 설치가 흔적 없이 멈추고, 이어서 진행조차 막히던 것** (`internal/stack/usecase/reap_stale_installs.go` 신규, `internal/stack/domain/deploy_steps.go`, `cmd/api/main.go`): 설치는 API 프로세스 안의 고루틴이 돌린다. 파드가 교체되면 그 고루틴이 사라지는데 **아무도 실패를 기록하지 않는다** — 스택은 `installing` 인 채로 남고, 화면은 진행 중처럼 보인다.

  더 나쁜 것은 그 상태에서 **이어서 진행(continue)이 막힌다**는 점이다. `continue` 는 `failed`/`pending` 만 받으므로 `409 STACK_CONTINUE_INVALID_STATE` 가 난다. 사용자에게는 지우고 다시 까는 길밖에 없다 — 2026-08-20 운영에서 `installing_gateway` 에서 그렇게 갇혔고, 몇 시간 뒤에야 발견됐다.

  이제 5분마다 훑어 **30분째 진전이 없는 설치를 실패로 옮긴다.** 실패 사유에 "서버가 재시작되면 진행이 이어지지 않는다, 이어서 진행하면 그 자리부터 계속된다" 까지 적는다 — "실패" 만 적으면 설치가 잘못된 줄 안다.

  기동 시 한 번이 아니라 주기적으로 도는 이유가 있다. API 는 레플리카가 여럿이라 "내가 재시작했다" 가 "아무도 안 돌리고 있다" 를 뜻하지 않는다. 판단 근거는 시간뿐이고, 한 스텝이 오래 걸릴 수 있어(GitLab 은 helm `--wait` 만 15분) 여유를 크게 잡았다.

- **스택을 지워도 네임스페이스가 남던 것** (`internal/stack/usecase/delete_stack.go`): 삭제는 리소스를 종류별로 하나씩 지웠고 **네임스페이스는 건드리지 않았다.** 스택이 플랫폼과 같은 `nullus` 에 살던 시절의 규칙인데, 이제 스택은 자기 네임스페이스를 갖는다. 하나씩 지우는 방식은 놓치는 것이 생기고, 그것이 오늘의 사고들(Gitea `28P01`, Harbor `401`)의 뿌리였다.

  스택 몫으로 만들어진 자리(`nullus-<스택명>`)는 통째로 회수한다. 플랫폼이 사는 곳, `default`, 옛 공용 기본값 `nullus`, 사용자가 직접 고른 네임스페이스는 지우지 않는다 — 다른 것과 함께 쓰고 있을 수 있고, `nullus` 를 지우면 플랫폼이 사라진다.

- **이전 설치의 볼륨이 남은 채 다시 설치하면 스무 단계 뒤에야 알던 것** (`internal/stack/adapter/helm/preflight.go` 신규, `internal/stack/usecase/install_stack.go`): 남은 볼륨은 옛 데이터베이스를 물려주고, 그 안의 비밀번호는 이번에 새로 만든 Secret 과 다르다. 그 사실이 드러나는 자리는 원인에서 멀다 — **Gitea 의 `28P01` 과 Harbor 의 `401` 로 두 번 나왔고, 매번 20분을 태운 뒤였다.**

  이제 설치를 시작할 때 대상 네임스페이스에 볼륨이 남아 있는지 먼저 본다. 있으면 몇 초 만에 멈추고, 무엇이 남았는지·왜 문제인지·어떻게 지우는지를 함께 알린다. 이어서 진행(continue)할 때는 검사하지 않는다 — 그때 남아 있는 볼륨은 지금 하고 있는 설치가 만든 것이다.

- **스택을 지워도 볼륨이 남아 다음 설치의 자격증명이 어긋나던 것** (`internal/stack/usecase/delete_stack.go`, `internal/stack/adapter/helm/harbor-provisioning.go`): PVC 는 그것을 마운트한 파드가 살아 있는 동안 `pvc-protection` finalizer 로 남는다. 삭제는 릴리스를 먼저 걷어내므로 파드가 곧 사라지지만, **첫 삭제 시도는 그 전에 끝나 타임아웃**이 났고 거기서 포기했다.

  남은 볼륨은 조용한 실패가 아니다. 다음 설치가 옛 데이터베이스를 물려받고, 그 안의 비밀번호는 새로 만든 Secret 과 다르다. **PostgreSQL 은 Gitea 의 `28P01` 로, Harbor 는 프로비저닝 `401` 로 드러났다** — 둘 다 원인에서 여섯 단계쯤 떨어진 자리다.

  이제 파드가 빠지기를 기다렸다가 최대 여섯 번까지 다시 지운다. 그래도 남으면 무엇이 남았는지·다음 설치에 무엇을 하는지·어떻게 지우는지를 error 로 남긴다.

  Harbor 의 401 도 읽을 수 있게 고쳤다. 예전에는 `harbor project 생성 실패 (HTTP 401)` 한 줄이 전부라, Harbor 가 관리자 비밀번호를 **자기 데이터베이스에 굽는다**는 사실을 모르면 원인을 찾을 수 없었다.

- **배포 진행률이 실제로 한 일과 어긋나고, 새로고침하면 값이 튀던 것** (`internal/stack/domain/deploy_steps.go` 신규, `internal/stack/adapter/handler/deploy_handler.go`, `web/src/features/stack/utils/deploy-progress.ts`): 시크릿까지만 깔린 시점에 막대가 **50%** 를 넘겼고, 새로고침하면 퍼센트가 다른 값으로 바뀌었다.

  진행률의 출처가 셋이었다. 서버에는 손으로 적은 스텝→퍼센트 표가 있었는데 `provisioning_secrets` · `installing_postgresql` · `installing_gitea` 처럼 **실제로 밟는 스텝의 절반이 빠져 있어** 그 스텝에서는 0 이 나갔다. 화면은 0 을 받으면 상태(`installing`)를 뭉뚱그린 자기 표로 떨어졌고, 그 위에서 시간 기반 크리프가 다음 단계(90%)를 향해 기어올랐다 — 그래서 초반 스텝에서 절반을 넘겼다. 새로고침하면 스트림이 처음부터 다시 붙어 그 뭉뚱그린 표만 남으므로 값이 튀었다.

  이제 **오케스트레이터가 실제로 도는 스텝 순서(31개)가 단일 출처**다(`domain.InstallStepOrder`). 설치 스텝들이 5~90 을 균등하게 나눠 갖고, 스텝 하나가 끝나면 딱 그만큼 오른다 — 시간이 아니라 한 일에 맞춰 움직인다. 모르는 스텝은 0 이 아니라 -1 이라 "아직 시작 전" 과 구분된다.

  서버는 **그 스텝이 끝났을 때 닿을 값(상한)도 함께 보낸다.** 화면은 그 안에서만 조금씩 움직이므로 다음 스텝의 몫을 미리 채우지 않는다. 막대가 살아 있어 보이는 것은 이제 폭이 아니라 표면의 빛과 로켓이 맡는다.

  새로고침도 같은 값이다. `GET /stacks/:id/status` 가 저장된 스텝(`current_step` / `last_completed_step`)에서 같은 계산을 한 번 더 해 내려주고, 표시 값은 첫 값에 애니메이션 없이 앉는다 — 이미 40% 인 배포가 0 부터 다시 차오르지 않는다.

- **배포 진행 막대가 초반에 확 달리던 것** (`web/src/features/stack/utils/deploy-progress.ts`): 표시용 진행률이 남은 거리에 **비례해서만** 움직였다. `install` 단계는 15%에서 다음 이정표 90%까지 75 포인트가 남아 있어서, 비례식이 초당 6%p 넘는 속도를 냈다 — 막대가 시작하자마자 절반을 넘겼다.

  비율에 더해 **한 틱에 움직일 수 있는 최대 폭**을 뒀다. 거리가 멀어도 걸음 폭은 같고, 가까워지면 비율이 줄면서 자연히 느려진다. 멈춰 있을 때는 초당 약 0.36%p(1분에 21 포인트)로, 분 단위로 도는 설치에 맞춘 걸음이다. 실제 값이 뛰었을 때의 도약도 순간이동 대신 몇 초에 걸쳐 미끄러진다 — 값이 뛰었다는 것은 보여 주되 눈이 따라갈 수 있게.

- **볼륨이 남은 채 다시 설치하면 Gitea 가 DB 인증 실패로 못 뜨던 것** (`internal/stack/adapter/helm/postgres-role-sync.go` 신규, `internal/stack/usecase/delete_stack.go`): 운영에서 Gitea 의 `configure-gitea` 가 `password authentication failed for user "gitlab" (28P01)` 로 CrashLoopBackOff 에 빠졌다. DB 주소·이름·사용자는 전부 정상이었고 **비밀번호만 어긋나 있었다.**

  비밀번호의 출처와 그것이 구워지는 곳의 수명이 다르기 때문이다. 출처는 OpenBao(→ ExternalSecrets → Secret)이고, PostgreSQL 은 **데이터 디렉터리가 비어 있을 때 딱 한 번** 그 값으로 사용자를 만든다. 실제 타임스탬프가 그대로 보여 준다:

  ```
  data-nullus-postgresql-0        09:45   ← 이전 설치의 PVC 가 살아남음
  data-openbao-0                  12:41   ← 금고는 새로 초기화 → 새 비밀번호 생성
  nullus-postgresql-credentials   12:45   ← 새 비밀번호
  nullus-postgresql-0 (pod)       12:45   ← 옛 데이터로 기동 (옛 비밀번호 유지)
  ```

  설치는 여기서 멈추지 않는다. 여섯 단계쯤 더 간 뒤 Gitea 에서야 드러나므로, 원인에서 가장 먼 자리에서 증상을 보게 된다.

  이제 PostgreSQL 을 세운 직후 앱 사용자의 비밀번호를 Secret 값과 맞춘다. 비밀번호는 매니페스트에 적지 않고(적으면 helm 히스토리와 이벤트에 평문으로 남는다) Secret 참조로 주입하며, psql 의 변수 인용(`:'pw'`)을 써서 따옴표가 든 비밀번호에서도 구문이 깨지지 않는다. Job 이미지는 차트가 쓰는 것과 같은 상수를 본다 — 갈라지면 에어갭 번들에 없는 이미지를 끌어온다.

- **스택을 지웠는데 볼륨이 남은 것을 알려 주지 않던 것** (`internal/stack/usecase/delete_stack.go`): PVC 삭제가 타임아웃하면(파드가 아직 물고 있으면 그렇게 된다) 경고 한 줄만 남기고 삭제가 성공으로 끝났다. 사용자는 깨끗이 지워진 줄 알고 같은 네임스페이스에 다시 설치했고, 위 사고가 그렇게 시작됐다.

  이제 삭제 뒤 남은 볼륨을 다시 확인해서, 무엇이 남았는지·그것이 다음 설치에 무엇을 하는지·어떻게 지우는지를 error 로 남긴다.

- **스택 삭제가 플랫폼 네임스페이스를 훑던 것** (`internal/stack/usecase/delete_stack.go`): 삭제는 Envoy Gateway 릴리스(`eg`)를 찾겠다고 스택 네임스페이스뿐 아니라 **플랫폼 네임스페이스까지** 훑었다. 그 경로가 2026-08-20 에 플랫폼을 지운 스윕이 지나간 길이다. 이제 스택은 자기 자리와 `default` 만 정리한다 — 게이트웨이는 스택 것이므로 스택 네임스페이스에서 정상적으로 회수된다.

- **Jenkins 가 받을 수 없는 이미지를 가리켜 스택마다 뜨지 않던 것** (`internal/stack/adapter/helm/values.go`, `scripts/build-jenkins-image.sh`, `airgap/scripts/00-generate-images.sh`): 저장소가 `cloud-nullus/draft` → `cloud-nullus/nullus` 로 리네임됐을 때 `nullus-api` · `nullus-web` 은 새 경로로 고쳐졌지만(#78) **Jenkins 이미지만 옛 경로에 남았다.** CD 는 `ghcr.io/${{ github.repository }}/nullus-jenkins` 로 push 하므로 이미지는 새 경로에만 올라간다.

  ```
  ghcr.io/cloud-nullus/draft/nullus-jenkins:2.568.2   →  403   ← 스택이 받으려던 것
  ghcr.io/cloud-nullus/nullus/nullus-jenkins:2.568.2  →  200   ← CD 가 올리는 것
  ```

  받을 수 없는 이미지라 `jenkins-0` 이 `Init:ImagePullBackOff` 로 멈춘다 — Jenkins 차트의 init 컨테이너가 컨트롤러와 같은 이미지를 쓰므로 본 컨테이너는 시작조차 못 하고 `0/2` 로 남는다. 클러스터와 무관하게 **Jenkins 를 포함한 템플릿은 어디에 깔아도 Jenkins 가 뜨지 않았다.** 에어갭 번들도 같은 경로를 담고 있었다.

  **테스트가 틀린 값을 고정하고 있어서 아무도 못 잡았다.** `TestJenkinsImage_FollowsRegistryConvention` 은 "CI 가 push 하는 경로와 같아야 한다" 는 주석을 달고 `cloud-nullus/draft/nullus-jenkins` 를 그대로 단언했다 — 지키려던 불변식은 옳았는데 비교 대상을 CI 가 아니라 자기 자신에 맞춰 놓은 것이다.

  이제 경로를 테스트에 다시 적지 않는다. 같은 CI 가 올리는 `nullus-api` 의 경로를 차트 기본값(`deploy/helm/nullus/values.yaml`)에서 끌어와 접두사를 맞춘다 — 플랫폼이 실제로 그 이미지로 떠 있으니 옳다는 것이 증명된 값이다. 다음 리네임에서 한쪽만 고치면 여기서 걸린다.
- **스택 삭제가 플랫폼 리소스를 이름만 보고 지우던 것** (`internal/stack/usecase/delete_stack.go`, `internal/shared/config/config.go`, `deploy/helm/nullus/templates/deployment.yaml`): 삭제의 마지막 단계는 네임스페이스를 훑어 "레거시 잔재" 를 지운다. 그 판정이 이름 문자열이었다 — 리소스 이름에 스택 이름이 들어 있거나 `nullus-` 로 시작하면 지웠고, 소유자가 누구인지는 보지 않았다.

  2026-08-20, 이름이 `nullus` 이고 네임스페이스도 플랫폼과 같은 `nullus` 인 스택을 지웠다. 그 규칙에 `nullus-api` · `nullus-web` · `nullus-keycloak` · `nullus-postgresql` 이 전부 걸려 **플랫폼 자신이 지워졌다.** nullus.io 가 전면 503 이 됐고, Helm 릴리스 기록만 남아 다음 배포가 `deployments.apps "nullus-api" not found` 로 막혔다.

  **StatefulSet 과 PVC 가 살아남은 건 운이다.** 나열 순서가 `deploy,svc,cm,sa,pod,rs,sts,...` 라서, `deploy` 를 지운 순간 그 스윕을 실행하던 API 파드가 자기 자신을 죽이고 `sts` 차례 전에 멈췄다. 한 칸만 뒤였으면 데이터까지 갔다.

  이제 소유자를 읽고 판단한다. 쿠버네티스는 이미 답을 갖고 있다 — Helm 은 `meta.helm.sh/release-name` 을, 이 플랫폼은 `nullus.io/stack-name` 라벨을 붙인다. 목록을 `-o name` 대신 `-o json` 으로 읽어 소유자와 함께 가져오고, **라벨 → Helm 릴리스 → (고아일 때만) 이름 규칙** 순서로 본다. `nullus-` 접두사는 뺐다(스택이 만드는 `nullus-*` 는 이름을 알고 있으므로 domain 상수로 하나씩 적었다). 스택 이름 부분일치 규칙은 없앴다.

  마지막 안전망으로 **플랫폼이 사는 네임스페이스에서는 이름 기반 정리를 아예 하지 않는다.** 자기 자리는 차트가 Downward API 로 알려준다(`NULLUS_PLATFORM_NAMESPACE`) — 값을 차트에 적지 않으므로 릴리스를 다른 네임스페이스에 깔아도 저절로 맞는다.

  스택 이름이 들어간 와일드카드 TLS 시크릿은 더 이상 지우지 않는다. 스택이 플랫폼의 공용 인증서를 그대로 지정할 수 있기 때문이다 — 이번 스택 설정이 실제로 `secret_name=nullus-wildcard-tls` 를 가리키고 있었다. 고아가 조금 남는 편이 남의 것을 지우는 것보다 낫다.

- **스택이 플랫폼과 같은 네임스페이스에 설치되던 것** (`internal/stack/domain/stack_namespace.go` 신규, `internal/stack/usecase/create_stack.go`, `web/src/features/stack/utils/install-manifest-builders.ts`): 스택 네임스페이스 기본값이 `nullus` 였고 그것은 플랫폼 자신이 사는 곳이었다. 자기 클러스터에 스택을 까는 순간 두 가지가 동시에 깨진다.

  **설치는 실패한다.** 스택의 PostgreSQL 릴리스(`nullus-postgresql`)가 플랫폼 차트의 bitnami postgresql 서브차트와 같은 이름의 리소스를 요구하는데, Helm 은 남의 릴리스 소유물을 인수하지 않는다. `installing_postgresql` 에서 `invalid ownership metadata` 로 멈춘다. **삭제는 더 나쁘다** — 위 항목이 그 결과다.

  이제 스택마다 자기 네임스페이스를 갖는다. 비워 두면 스택 이름에서 만든다(`gitea-jenkins-v1` → `nullus-gitea-jenkins-v1`). RFC1123 라벨 규칙과 63자 제한에 맞춰 자르고, 쓸 수 없는 이름은 `nullus-stack` 으로 떨어진다. 사용자가 직접 플랫폼 네임스페이스를 골라도 막는다.

  화면도 같은 규칙을 쓴다. 설치 마법사가 네임스페이스를 비우면 `nullus` 로 떨어지고 있었으므로 서버만 고치면 UI 설치가 전부 거부당한다. 두 규칙이 갈리지 않도록 테스트로 묶었다. **기존 스택은 자기 네임스페이스를 이미 저장하고 있어 영향받지 않는다.**

- **형식이 깨진 접속 도메인을 그대로 받던 것** (`internal/stack/domain/access_domain.go` 신규, `internal/stack/usecase/create_stack.go`): 접속 도메인 하나가 스택의 모든 주소가 된다 — HTTPRoute 의 hostname, 게이트웨이 인증서의 commonName 과 dnsNames, 도구들의 redirect_uri. 그런데 생성 경로는 값이 비었는지만 보고 형식은 보지 않았다.

  2026-08-20 운영 스택은 `access_domain` 이 `.io` 로 저장돼 있었고, 만들어진 매니페스트는 hostname `jenkins..io`, 인증서 `commonName: .io` / `dnsNames: [.io, *..io]` 였다. 라우팅도 발급도 될 수 없는 값인데 설치는 그대로 시작했다. 이제 스킴·경로·와일드카드·공백을 거부하고, 점으로 나뉜 조각이 둘 이상인지, 빈 조각이나 하이픈으로 시작·끝나는 조각이 없는지 본다. 값을 고쳐 주지는 않는다 — 무엇을 의도했는지는 사람만 안다.

- **한 스택의 모니터링 탭이 다른 스택에도 뜨던 것** (`web/src/features/observability/utils/monitoring-utils.ts`, `monitoring-tab-layout.tsx`): 탭 저장 키가 뷰 단위(`nullus_tabs_stack_v1`)라 스택 A 에서 등록한 탭이 스택 B 를 골라도 그대로 떴다. cluster·cicd 뷰도 같았다. 사용자가 손으로 넣은 주소일 때는 "내가 넣은 게 남아 있네" 로 넘어갔지만, 스택 접속 도메인에서 주소를 미리 채우기 시작하면 그대로 남의 도메인을 안내하는 오작동이 된다.

  키를 스코프별(v2)로 나눴다 — stack 뷰는 stackId, cluster·cicd 뷰는 clusterId 를 쓰고 skip 플래그도 같은 단위로 저장한다. **키만 나누면 부족하다**: 컴포넌트가 마운트된 채 스택만 바뀌면 state 에 남은 이전 스택의 탭이 계속 보인다. `DashboardTabLayout` 이 `key` 로 내부를 갈아끼워 열린 탭·관리 모드·skip 여부까지 함께 초기화한다. v1 키는 지우지 않고 남겨 둔다 — 기존에 등록한 탭은 화면에서 사라지지만 데이터는 남아 있다.
- **배포가 DB 마이그레이션을 건너뛰던 것** (`deploy/helm/nullus/templates/migration-job.yaml`, `deploy/helm/nullus/values.yaml`, `Dockerfile`, `deploy/helm/migration_job_test.go` 신규): 배포된 nullus.io 에 스택 템플릿이 하나도 없었다. 템플릿은 seed 마이그레이션(`000008`/`000031`/`000059`/`000063`/`000069`)으로만 들어오는데, CD(`.github/workflows/cd.yml`)는 `helm upgrade` 와 `rollout restart` 만 돌린다 — 워크플로 전체에 `migrate` 라는 단어가 없다. 차트의 `migration-job.yaml` 은 "마이그레이션은 밖에서 처리한다"는 주석뿐인 빈 파일이었고, 밖에서 돌리는 쪽은 airgap 설치기와 vm-cluster 런북뿐이라 zadara 경로만 비어 있었다. 그래서 배포 DB 는 누군가 손으로 `migrate up` 을 돌린 시점에 멈춰 있었다. 템플릿만 없는 게 아니었다 — `users.password_hash`(`000073`)가 없어 ID/PW 로그인은 500 을 냈다.

  **아무도 실패를 못 본 이유는 실패가 없었기 때문이다.** helm 은 초록불로 끝나고 파드도 Ready 다. 새 코드가 아직 없는 컬럼을 읽을 때가 되어서야 드러난다. 그래서 배포가 스스로 실패하는 자리에 넣는다 — 차트가 `post-install,pre-upgrade` 훅 Job 으로 마이그레이션을 돌리고, 실패하면 `helm upgrade --wait` 가 그 자리에서 멈춘다. 설치 때 `pre-install` 이 아닌 것은 훅이 차트 리소스보다 먼저 돌아 아직 만들어지지도 않은 PostgreSQL 을 기다리게 되기 때문이고, 업그레이드 때 `pre-upgrade` 인 것은 새 코드가 옛 스키마 위에서 도는 창을 없애기 위해서다.

  **Job 이 `migrate` 를 부르는 것과 이미지에 `migrate` 가 있는 것은 별개다.** `deploy/csp/vm-cluster/runbook_csp.sh` 의 Job 이 이미 그 상태였다 — api 이미지 안에서 `migrate` 를 실행하는데 Dockerfile 은 helm 과 kubectl 만 싣고 있어, 돌리면 `migrate: not found` 로 끝난다. golang-migrate CLI 를 이미지에 실어 그 Job 도 같이 살아난다. SQL 과 그것을 적용할 CLI 가 같은 이미지에 있으므로 코드와 스키마의 세대가 어긋날 수 없다.

  DB 접속값은 api Deployment 와 **같은 헬퍼·같은 시크릿**에서 끌어온다(따로 적으면 한쪽만 고쳤을 때 마이그레이션은 성공했는데 API 가 보는 DB 에는 반영이 없다). 비밀번호는 URL 에 끼우기 전에 퍼센트 인코딩한다 — `@ : / ? #` 가 든 비밀번호면 접속 URL 이 갈라진다. 밖에서 돌리는 환경은 `migration.enabled=false` 로 끈다.

- **설치되는 OSS 가 자기 주소를 몰라 OIDC 로그인이 막히던 것** (`internal/stack/adapter/helm/{helm-values,oidc-values}.go`): 도구는 저마다 자기 기본 주소 설정에서 `redirect_uri` 를 만든다. 그 스킴이 Keycloak 에 등록된 redirect 와 다르면 로그인이 `Invalid parameter: redirect_uri` 로 막힌다. Harbor(`externalURL`)·Gitea(`ROOT_URL`)·Jenkins(`jenkinsUrl` — 아예 미설정)·GitLab(`global.hosts.https`) 에서 **같은 실패가 네 번 반복됐다**.

  스킴을 도구마다 하드코딩하면 도구를 추가할 때마다 이 실수가 돌아온다. `toolURLScheme()` 하나로 모으고, 네 도구가 같은 판단을 쓰는지 한 테스트로 묶는다. 갈라지면 그 도구만 로그인이 깨지고, 원인은 인증이 아니라 주소 설정에 있어 찾기 어렵다. 네 번째(GitLab)는 그 뒤라 설치 전에 코드에서 잡았다.

- **Prometheus Operator CRD 가 그것을 쓰는 리소스보다 20단계 뒤에 설치되던 것** (`internal/stack/adapter/helm/orchestrator.go`, `internal/stack/usecase/install_stack.go`): Prometheus 를 고른 스택은 각 도구의 ServiceMonitor 를 켜는데, 그 CRD 는 kube-prometheus-stack 이 가져오고 그 설치는 한참 뒤다. MinIO 에서 `no matches for kind "Probe"` 로 죽었다. ArgoCD 도 같은 상황이었고, MinIO 에서 먼저 죽어 거기까지 가지 못했을 뿐이다. **Prometheus 를 포함한 템플릿은 설치 자체가 불가능한 상태였다** — Prometheus 가 없는 템플릿에서는 ServiceMonitor 를 켜지 않아 드러나지 않았다.

- **스택을 지워도 Keycloak OIDC 클라이언트가 남던 것** (`internal/stack/port/sso_provisioner.go`, `internal/stack/usecase/delete_stack.go`): `DeprovisionSSO` 구현은 진작 있었는데 포트 인터페이스에 없어 stack 모듈이 부를 방법이 없었다. 이미 고친 PVC·Gateway 누수와 같은 모양이고, 프로비저닝 배선이 끊겨 있던 동안에는 만들어지는 것이 없어 보이지 않았다.

  client ID 슬러그도 접속 도메인 대신 네임스페이스에서 뽑는다. 로컬처럼 모든 스택이 같은 도메인을 쓰면 서로의 클라이언트 등록을 덮어쓴다.

- **cert-manager 단계가 매번 2분씩 헛기다리던 것** (`internal/stack/adapter/helm/{cert-manager,kubectl}.go`): `startupapicheck` 은 Helm 훅 Job 이라 설치가 끝나면 사라진다. 재설치 때는 영원히 없는데 `waitForKubectlGet` 이 "없음" 도 재시도 대상으로 보고 60회 × 2초를 꼬박 돈 뒤에야 건너뛴다고 판단했다(실측: 파드는 14시간째 Running, Job 은 없음). 설치 시간이 18분에서 5분 30초로 줄어든 주된 이유다.

  `waitForKubectlGet` 자체는 그대로 둔다. CRD 처럼 곧 만들어질 리소스를 기다리는 경로가 여럿이라, 거기서 "없음" 을 즉시 실패로 보면 설치가 깨진다.


- **스택을 지워도 PVC 가 남아 디스크가 쌓였다** (`internal/stack/usecase/delete_stack.go`): `helm uninstall` 은 PVC 를 지우지 않는다 — StatefulSet 의 `volumeClaimTemplate` 이 만든 것은 애초에 릴리스 소유가 아니고, 차트가 직접 만든 것도 대개 남긴다. 라벨 기반 정리 목록에 `pvc` 가 이미 들어 있었지만 **하나도 지워지지 않았다**. 그 PVC 들에 `nullus.io/stack-name` 라벨이 붙지 않기 때문이다. 실측한 라벨은 Helm 차트 것뿐이었고, 릴리스 라벨조차 없는 것이 있었다 — `data-harbor-redis-0` 는 `release=harbor`, `gitea-shared-storage` 는 `app.kubernetes.io/managed-by=Helm` 하나뿐이다. 그래서 릴리스 라벨로도 전부 잡을 수 없다.

  스택 네임스페이스를 통째로 훑는다. 범위는 스택 자신의 네임스페이스뿐이다 — `cleanupNamespacesForStack` 이 함께 도는 `default`·`nullus`·`envoy-gateway-system` 은 다른 스택이나 사용자의 볼륨이 있을 수 있고, 거기서 `--all` 을 던지면 남의 데이터를 파기한다. 네임스페이스를 모르는 스택은 건너뛴다. `--all` 이 현재 컨텍스트의 기본 네임스페이스를 향하기 때문이다.

  삭제 순서의 맨 끝에 둔다. 릴리스와 StatefulSet 이 살아 있는 동안 PVC 를 지우면 컨트롤러가 곧바로 다시 만든다 — 아래 Gateway 건과 같은 실패 방식이다.

  **되돌릴 수 없는 동작이다.** 스택을 지우면 그 스택의 볼륨 데이터도 함께 사라진다.

- **스택을 지워도 Gateway 가 남아 envoy 파드가 계속 떴다** (`internal/stack/usecase/delete_stack.go`): 삭제 경로는 Gateway 가 **소유한** 리소스(envoy Deployment·Service 등)를 지웠지만, Gateway 와 HTTPRoute 커스텀 리소스 자체를 지우는 코드가 없었다. Gateway 가 살아 있으면 Envoy Gateway 컨트롤러가 그것을 보고 데이터플레인 Deployment 를 곧바로 다시 만든다 — 그래서 지우는 단계가 있어도 파드는 계속 떠 있었다. `helm list` 는 깨끗하게 나오므로 발견도 늦었다(실측: 삭제 2시간 뒤에도 `envoy-<stack>-gateway` 파드가 2/2 Running). 설치·삭제를 반복하면 envoy 파드가 그만큼 쌓인다.

  Gateway·HTTPRoute·GRPCRoute·TCP/TLS/UDPRoute·ReferenceGrant 를 지운다. 순서가 핵심이라 **관리 리소스 삭제보다 먼저** 둔다 — 뒤에 두면 그 사이 컨트롤러가 데이터플레인을 복구해, 방금 지운 Deployment 가 되살아난 채로 삭제가 끝난다. 범위는 스택 자신의 네임스페이스뿐이다. 함께 훑는 `default`·`nullus`·`envoy-gateway-system` 은 다른 스택과 공유될 수 있어 `--all` 로 지우면 남의 게이트웨이가 날아간다.

  `stack-down` 의 잔여물 보고도 helm 릴리스만 보던 것을 고쳤다. 이 누수가 바로 "helm 은 깨끗한데 파드는 살아 있는" 유형이라, 릴리스만 보여주면 지웠다는 잘못된 확신을 준다.

- **Gitea·Jenkins·Harbor·Nexus 는 자원 계획을 아예 받지 못했다** (`internal/stack/adapter/helm/applied-resources.go`, `internal/stack/adapter/helm/resource-defaults.go`): 계획을 세워도 그 단계가 계획을 찾아보지 않으면 아무 일도 일어나지 않는다. `plannedSlotForStep` 에 슬롯 매핑이 없고 `resourceDefaultKeyForStep` 에 자원 키가 없어, 이 넷은 관리자 기본값조차 실리지 않고 차트 기본값으로 깔렸다. Lite 템플릿(Gitea + Jenkins + Harbor + Argo CD)은 도구 넷 중 **셋**이 여기 해당해, `local` 프로파일이 실제로는 Argo CD 하나에만 적용되고 있었다.

  Harbor 는 릴리스 하나가 7개 컴포넌트로 갈라지므로 계획 벡터를 비율로 나눈다 — 그대로 모든 칸에 실으면 릴리스 하나가 계획의 7배를 요청한다(GitLab 이 이미 같은 이유로 비율을 쓴다). 실측: Lite 를 다시 깔았을 때 Gitea 는 requests 없음 → 500m/512Mi, Harbor core 는 하드코딩 100m/256Mi → 계획값 기반 130m/256Mi 로 바뀌었다.

- **Lite 템플릿 설명이 자기 도구 목록과 어긋났다** (`internal/stack/adapter/repository/memory_template.go`): "레지스트리와 모니터링은 뺐습니다" 라고 적혀 있었지만 도구 목록에는 Harbor 가 들어 있다. 마이그레이션 `000072` 의 시드 문구와도 갈렸는데, 프론트의 `TEMPLATE_DESCRIPTION_LOCALE_OVERRIDES` 가 설명 원문을 키로 영문을 찾으므로 문구가 갈리면 영어 화면에서만 번역이 조용히 빠진다.

- **에어갭 API 설치 스크립트가 400 으로 죽었다** (`airgap/scripts/29-install-stacks-via-api.sh`): 스택 생성 요청의 `storage` 에 `plan_mode` 만 담고 있었다. 검증은 `integrated-create` 이면 `database`·`object_storage` 가 **둘 다** `mode=create` 여야 하고, `create` 모드는 `provider_or_engine` 과 `size`(Gi, 0 초과)를 요구한다. 그래서 이 스크립트는 스택을 만드는 첫 호출부터 `STACK_CONFIG_INVALID` 로 실패했다.

  더 큰 문제는 그 앞에 있었다 — 스크립트에 **템플릿 선택이 아예 없었다**. 검증을 통과했더라도 도구를 하나도 고르지 않은 빈 스택을 만들었을 것이다. `TEMPLATE_ID` 를 받아 템플릿 응답의 `tools[]` 를 `StackConfig` 슬롯으로 옮긴다. 버전 표를 스크립트에 복사하지 않는 이유는 그러면 마이그레이션이 차트 버전을 올릴 때마다 에어갭 경로만 낡기 때문이다.

  배포 요청에는 `acknowledge_warnings` 를 실어 보낸다. Pre-Deploy Gate 가 warn 을 내면 명시적 동의 없이 `DEPLOY_COMPAT_WARN_UNACK` 로 막히는데, 무인 설치에는 동의할 사람이 없다. block 판정은 이 값과 무관하게 그대로 막힌다.

- **모바일 폭에서 사이드바가 본문을 짓눌렀다** (`web/src/stores/sidebar-store.ts`): 사이드바(`<aside>`)는 `shrink-0` 로 항상 240px 를 차지하고 접힘은 수동 토글뿐이라 뷰포트를 몰랐다 — 390px 화면에서 본문이 ~150px 로 눌려 히어로가 한 단어씩 쪼개지고 폼·버튼이 잘렸다. 문서 폭은 넘지 않아(가로 스크롤 없음) 오버플로우 지표로는 잡히지 않는 유형이라, 인증 페이지 전역에서 조용히 깨져 있었다. 768px 미만으로 진입하면 사이드바를 기본 collapse(48px 레일)로 두어 본문 폭을 확보한다. 진입 시점 값은 localStorage 에 쓰지 않아 데스크톱의 접힘/펼침 취향을 덮지 않는다. 리사이즈 중 재판정과 오프캔버스 드로어는 후속으로 남긴다(EPIC 모바일/반응형 점검 1차).

- **도구가 없는 템플릿을 편집하면 도구 편집기가 통째로 사라졌다** (`web/src/features/stack/pages/stack-template-page.tsx`): 편집은 "템플릿이 실제로 쓰는 섹션만 편다" 는 규칙을 따르는데, `Empty Template` 은 도구가 하나도 없어 **모든** 섹션이 감춤 대상이 됐다. 그러면 탭 편집기가 `null` 을 반환해 화면에는 그 아래 점선 `+ Add Section` 버튼만 남는다 — 탭 구조로 바꾸기 전(4c2cdb6) 모달처럼 보이지만 실제로는 편집기가 없는 상태다. 도구가 없는 템플릿은 아무 섹션도 감추지 않는다(만들기 모달과 같은 화면이 된다).

  같은 뿌리의 문제가 하나 더 있었다: 감출 섹션을 편집 중의 `form.tools` 로 매 렌더 다시 고르고 있어, **섹션의 마지막 도구를 지우면 그 탭까지 사라져** 방금 지운 것을 되돌릴 수 없었다(GitLab + Argo CD 에서 CD Tool 을 지우면 CI/CD 탭이 증발한다). 감출 목록은 모달을 열 때 한 번만 정한다.

- **화면이 안내하는 도구 버전이 실제 설치되는 버전과 달랐다** (`web/src/features/stack/stores/stack-config-store.ts`): 설치되는 버전은 `internal/stack/domain/connection.go` 하나가 소유하고 백엔드는 그 값이 호환성 매트릭스와 어긋나지 않는지 이미 검사한다(`TestChartVersionsMatchCompatibilityMatrix`). 정작 화면이 보는 표는 그것을 손으로 베껴 온 것이라 지킴이가 없었고, **대조한 11개 중 9개가 갈라져 있었다** — 설치는 Argo CD 7.7.16 을 올리는데 화면은 6.8.0 을, GitLab 은 8.7.2 인데 9.5.1 을 말했다. Prometheus·Grafana·Tempo 는 차트 버전이 아예 없어 템플릿의 helm 버전 칸이 빈칸으로 채워졌다.

  표시 오류로 끝나지 않는 이유는 **템플릿 편집기가 기본값을 이 표에서 가져가기** 때문이다. 관리자가 버전 칸을 손대지 않으면 존재하지 않는 버전이 그대로 템플릿에 pin 되고, 그 템플릿으로 스택을 만들면 설치 전 검사가 매트릭스에 없는 조합이라며 막는다. 값을 맞추고, **Go 상수를 직접 읽어 대조하는 테스트**를 붙였다(`tool-version-catalog.test.ts`). 두 언어 사이에 값을 나르는 생성 단계를 만들 수도 있지만 표가 열 줄 남짓이라 그 배관이 값보다 커진다. 테스트 두 곳에 박혀 있던 리터럴 버전도 표를 참조하게 바꿨다 — 리터럴로 적어 두면 표와 테스트가 각자 출처가 되어 조용히 갈라지고, 실제로 그 숫자들이 뒤처진 상태를 "정상" 으로 굳히고 있었다.

- **역할이 다른데 같은 설명이 붙던 도구** (`web/src/features/stack/utils/tool-copy.ts`): 문구 키가 `stackAddTools.tools.<id>` 하나뿐이라 **한 도구가 역할마다 다른 것을 뜻하는 경우를 표현하지 못했다.** GitLab 은 소스 저장소이면서 패키지 레지스트리인데 둘 다 id 가 `gitlab` 이라, 설치 마법사와 템플릿 편집기 양쪽에서 "GitLab Package Registry" 밑에 "GitLab 소스 코드 관리" 가 붙어 있었다. 키를 **(슬롯, id) 쌍**으로 넓히고 없으면 종전 id 키로 떨어뜨린다 — 실제로 갈라지는 id 는 `gitlab` 하나뿐이라 나머지 27개의 번역을 옮겨 적을 이유가 없다. 직전 커밋이 템플릿 편집기에서 쓰던 "모호하면 설명을 통째로 지운다" 는 임시 방편도 걷어냈다: 그것은 맞는 설명(GitLab CE 의 소스 저장소 설명)까지 함께 지우고 있었다.

  테스트 목이 i18next 의 키 배열·`defaultValue` 를 몰라 배열을 넘기는 순간 `key.split is not a function` 으로 화면이 통째로 죽었다(43건). 목을 실제 동작에 맞췄다 — 목과 실물이 갈라지면 테스트는 화면이 아니라 목을 검사하게 된다.

- **생성한 비밀번호가 CLI 인자로 안전하지 않던 문제** (`internal/stack/adapter/helm/secret-provisioning.go`): `base64.RawURLEncoding` 알파벳(`A-Za-z0-9-_`)은 `-` 로 시작하는 값을 만들 수 있다. 이 비밀번호들은 CLI 인자로 그대로 넘어가므로(`mc`, `gitea admin user`, Nexus 프로비저닝) 그런 값이 나오면 CLI 가 플래그로 파싱해 죽는다 — MinIO post-install 잡이 이렇게 실패해 스택 설치가 phase A 에서 멈췄다. **64분의 1 확률이라 어떤 설치는 통과하고 어떤 설치는 실패해 재현이 어렵다**(같은 코드로 첫 설치는 통과하고 두 번째가 걸렸다). 영숫자만 쓰고 나머지 연산의 편향을 피하려 rejection sampling 을 쓴다. 길이 43 으로 엔트로피는 유지한다.

- **실행되지 않은 파이프라인 단계를 성공으로 그리던 문제** (`web/src/features/cicd/utils/stage-states.ts`): History 탭이 배포 상태 하나로 템플릿에 적힌 모든 단계를 초록 체크로 칠했다. 같은 화면이 바로 아래에서 `0 steps` 라고 밝히면서. 실제로 Jenkinsfile 은 Build·Deploy 2단계인데 템플릿의 4단계가 모두 완료로 보였다 — **돌지도 않은 Test 가 성공했다고 표시된다**. 실제 스텝 결과에서 단계 상태를 만들고, 스텝 정보가 없으면 초록 체크 대신 모른다고 말한다. 단계 목록의 출처도 렌더러로 옮겨(`scaffold.PipelineStageNames`) 템플릿 선언과 스캐폴딩 결과가 어긋나지 않게 계약 테스트로 묶었다.

- **Gitea 재배포가 교착되던 문제** (`internal/stack/adapter/helm/values.go`): Gitea 는 RWO 볼륨 하나에 leveldb 큐 락을 잡는데 차트 기본값이 `RollingUpdate`(maxUnavailable=0)다. 새 파드는 옛 파드가 쥔 락 때문에 뜨지 못하고, 옛 파드는 새 파드가 Ready 가 되어야 내려가므로 영원히 안 내려간다. 첫 설치는 성공하므로 드러나지 않다가 values 를 바꾸는 첫 재배포에서 걸린다.

- **되커밋 자격증명이 빌드 로그에 남던 문제** (`internal/cicd/adapter/scaffold/jenkins_renderer.go`): deploy 단계가 토큰을 URL 에 박아 push 하는데 셸 트레이스가 그 명령을 그대로 찍었다. GitLab 은 CI 변수를 masked 로 등록해 가려 주지만 Jenkins 는 K8s Secret 에서 env 로 온 값을 마스킹하지 않는다 — 빌드 로그를 볼 수 있는 사람이 저장소 쓰기 토큰을 그대로 얻는다.

- **클러스터 밖 API 서버가 스택 내부 주소를 쓰던 문제** (`internal/cicd/adapter/provisioning/bundle_factory.go`): Gitea·Jenkins 의 기본 주소는 서비스 DNS 라 API 서버가 클러스터 안에서 돌 때만 해석된다. 더 중요한 것은 **job 과 Argo CD 에 넘기는 주소가 API 서버의 주소와 다른 관심사**라는 점이다 — 그 둘은 클러스터 안에서 도는 소비자다. 같은 값을 쓰면 job 이 `Unknown server` 로 스캔을 거부하고 Argo 는 `connection refused` 로 동기화에 실패한다. bundle 에 in-cluster 주소를 따로 둔다.

- **Gitea 파드를 워크로드 종류로 찾던 문제** (`internal/cicd/adapter/gitea/token_issuer.go`): 차트 12.7.0 은 Deployment 로 배포하는데 StatefulSet 에 exec 하려 해서 파이프라인 생성만 실패했다. 차트는 버전·설정에 따라 종류가 달라지므로 레이블로 파드를 찾는다. 토큰 폐기도 Gitea CLI 에 없는 명령(`delete-access-token`)을 부르고 있어 API 로 바꿨다.

- **Jenkins CSRF crumb 세션이 유지되지 않던 문제** (`internal/cicd/adapter/jenkins/client.go`): Jenkins 는 crumb 을 세션에 묶어 검증하는데 쿠키 jar 가 없어 매번 세션이 갈렸다 — crumb 이 유효해도 403 이 나고 job 생성이 전부 실패했다. multibranch job 의 `traits` 누락(Jenkins NPE 500)과 Gitea DB 호스트가 기본 네임스페이스를 가리키던 문제도 함께 고쳤다.

- **모달 첫 줄 입력의 라벨 위쪽이 잘리던 문제** (`web/src/components/ui/modal.tsx`): 템플릿 편집 모달의 "Template ID" 가 위가 잘린 채로 떠 있었다. MUI 는 `DialogTitle` 바로 뒤에 오는 `DialogContent` 의 `padding-top` 을 0 으로 만드는데, 이 본문은 `overflow-y: auto` 라 그 경계에서 잘린다. 첫 줄이 외곽선 입력이면 떠 있는 라벨이 상자 위로 나가 있으므로 **라벨의 위쪽 절반이 사라진다**. 여백을 되돌려 준다 — 모든 모달에 적용된다.

- **탭 스트립에 세로 스크롤바가 상시로 뜨던 문제** (`web/src/components/ui/tabs.tsx`): 탭이 화면에 다 들어오는데도 스트립 오른쪽에 스크롤바가 붙어 있었다. 가로 스크롤 때문이 아니다 — `overflow-x: auto` 는 CSS 규칙상 반대 축이 `visible` 이면 그쪽을 `auto` 로 끌어올리는데, 탭 버튼이 활성 밑줄을 구분선 위에 겹치려고 `-mb-px` 로 상자를 **1px** 넘어가 있었다. 그 1px 이 세로 넘침으로 잡혀 스크롤바가 생겼다. 밑줄(`border-b`)을 스크롤 상자 바깥으로 빼고 상자 자체를 `-mb-px` 로 끌어내린다 — 겹침은 그대로 유지되고 넘칠 것은 없어진다. 시각 회귀 58장이 스냅샷 갱신 없이 통과해 픽셀이 같음을 확인했다.

- **직접 배포가 클러스터에 적용하지 않고 성공으로 기록하던 문제** (`internal/cicd/adapter/handler/deploy_app_apply.go`): `POST /cicd/deploy-app` 은 매니페스트를 만들어 응답에 담고 Deployment 레코드를 `status=success` 로 저장하면서 **클러스터에는 아무것도 적용하지 않았다**(완료 시각까지 "지금 + 3초" 로 지어냈다). 화면에는 성공한 배포로 보이는데 실체가 없어, 배포 목록과 클러스터가 어긋나고 사용자는 존재하지 않는 앱의 로그와 파드를 찾게 된다. 이제 생성한 매니페스트를 실제로 적용하고 **상태는 적용 결과를 따른다** — 시작 시점에 `running`, 끝난 뒤 `success`/`failed`. 적용기가 배선되지 않았으면 오류를 돌려준다: 조용히 건너뛰면 예전 상태로 되돌아간다.

- **파이프라인을 지워도 직접 배포한 워크로드가 남던 문제** (`internal/cicd/adapter/kube/workload_deleter.go`): 파이프라인 삭제의 `cluster_resources=true` 는 Argo CD Application 만 지웠다. 매니페스트를 직접 적용하는 경로에는 Application 이 없고 삭제기는 없는 Application 을 성공으로 보므로 **Deployment·Service·Ingress 가 그대로 남았다** — 목록에서는 사라졌는데 클러스터에는 앱이 계속 돈다. 매니페스트 생성기가 모든 리소스에 붙이는 `app` 라벨로 찾아 함께 지운다. 지울 것이 없는 경우(Argo CD 경로)는 성공으로 본다. 정리에 실패하면 파이프라인 레코드는 남긴다 — 레코드가 사라지면 남은 리소스를 찾을 방법도, 다시 시도할 방법도 없어진다.

- **얼어붙은 오버라이드가 플랫폼 계산값을 이기던 문제** (`internal/stack/adapter/helm/platform-owned-values.go`): release values 의 `live` 편집은 배포된 values 를 통째로 읽어 `YAMLOverrides` 에 저장하는데, 거기에는 사용자가 적은 값뿐 아니라 **플랫폼이 계산해 넣은 값도 함께 얼어붙는다**. 오버라이드는 병합의 맨 마지막이라 그 스냅샷이 이후의 재계산을 영원히 이긴다. 실제로 두 가지가 이 경로로 깨졌다. (1) `global.psql.host` 가 스냅샷 시점의 네임스페이스에 묶여, 설정을 다른 네임스페이스로 옮기면 GitLab 이 **삭제된 스택의 PostgreSQL** 을 가리켰다 — export/import 도 이 값을 다시 쓰지 않으므로 그대로 재현된다. (2) GitLab 번들 Prometheus 의 메모리 한도 328Mi 가 얼어붙어, OOM 을 막으려 둔 자원 하한을 이기고 CrashLoopBackOff 를 재현했다(예전에 한 번 고친 증상이 되살아난 것이다). 병합이 끝난 뒤 **배선에 해당하는 값만** 다시 못박는다 — 자원·프로브 같은 실제 사용자 의도까지 되돌리면 오버라이드 기능이 무의미해진다. 값이 실제로 달랐을 때만 이유와 함께 경고를 남긴다.

- **GitLab 번들 Prometheus 를 설치하지 않는다** (`internal/stack/adapter/helm/resource-defaults.go`): 스택은 이미 kube-prometheus-stack 을 세우고 라우트·대시보드·모니터링 화면이 모두 그쪽을 본다 — 번들 쪽은 아무도 읽지 않으면서 메모리만 먹고 스택의 자원 계획 아래에서 OOMKilled 로 죽었다. 자원 하한으로 증상만 가리던 코드를 걷어내고 아예 끈다.

- **로그 저장소로 Loki 를 골라도 OpenSearch 가 설치되던 문제** (`internal/stack/adapter/helm/helm-values.go`): 화면은 Loki 를 고를 수 있게 열어 두는데 `resolveChartSpecForStep` 에 `loki` 분기가 없어 default(OpenSearch)로 떨어졌다. 함께, 모니터링의 워크로드 접두사가 `opensearch` 로 고정돼 있어 Loki 를 고르면 **설치는 정상인데 화면에서만 "0 파드 warning"** 으로 남던 것도 고쳤다(고른 제품에 따라 접두사를 정한다). 수집과 검색이 같은 제품이면 릴리스도 하나이므로 한 번만 센다.

- **차트가 상태색을 계열색으로 쓰던 문제** (`web/DESIGN.md`, `web/src/features/observability/components/monitoring-chart-widgets.tsx`): CPU/Memory 차트가 Limit 을 `--color-warning`(갈색), Current 를 `--color-success`(초록)로 그렸다. 한도와 현재값은 **정상 설정값과 측정값인데도** 색이 "경고"·"성공" 이라고 말해, 아무 문제 없는 차트가 문제 있는 것처럼 읽혔다. 계열 팔레트(`chart-1/2/3`)를 DESIGN.md 에 두고 상태색은 상태에만 남긴다 — 값은 색각 이상 분리도와 표면 대비를 라이트·다크 양쪽에서 검증한 것이고, 라이트의 `chart-3` 은 흰 면 대비가 3:1 미만이라 범례·직접 라벨이 반드시 함께 있어야 한다는 조건까지 문서에 적었다. 함께: 점선 전면 격자를 가로선만·10% 농도로(격자가 데이터보다 먼저 보였다), 도넛의 잉크색 굵은 테두리를 면 색 2px 로(조각보다 테두리가 도드라졌다), 툴팁·범례 글자가 테두리 토큰을 입어 배경에 묻히던 것을 글자 토큰으로.

- **OpenTelemetry 아이콘이 비활성처럼 보이던 문제** (`web/public/tool-icons/opentelemetry.svg`): `fill="#FFFFFF"` 라 흰 카드 위에서 보이지 않았다. 브랜드색으로 교체했다. (같은 이유로 `github.svg` 도 흰색이지만, 검정으로 바꾸면 다크 테마에서 반대로 사라져 테마별 처리가 필요하므로 남겨 뒀다.)

- **스택 상세 STORAGE 칸의 이름이 세로로 접히던 문제** (`web/src/features/stack/components/pipeline-topology.tsx`): 버전 칸이 `shrink-0` 이라 MinIO 의 `RELEASE.2024-12-18T13-15-44Z` 가 폭을 다 가져가 이름 칸이 0 이 되고, 고정 폭 카드 안에서 "minio" 가 한 글자씩 세로로 접혔다. 이름이 읽히는 폭을 먼저 보장한다.

- **설치 리소스 값이 Helm 에 한 번도 실리지 않던 문제** (`cmd/api/main.go`, `internal/stack/adapter/helm/resource-defaults.go`): 마법사에서 고른 규모(Local/Startup/…)와 OSS별 리소스 조정이 클러스터에 반영되지 않고 있었다. 원인이 둘이었다 — `WithResourceDefaultRepository` 가 `main.go` 에서 배선되지 않았고, 사용자가 조정한 `AppliedResourceOverrides` 는 저장만 되고 읽는 곳이 없었다. 배선 누락이 다시 생기지 않도록 `OrchestratorOption` 이 전부 `main.go` 에서 쓰이는지 AST 로 검사하는 테스트를 뒀다. 아울러 소수 Gi 값이 파드 스펙에 `257698037760m`(밀리바이트)로 남던 문제도 고쳤다 — 0.24Gi 같은 값이 `246Mi` 로 나간다.
- **⚠️ metrics-server 가 스택 네임스페이스에 설치돼 클러스터의 모든 네임스페이스 삭제를 교착시키던 문제** (`internal/stack/adapter/helm/helm_step_metadata.go`): 이 차트가 만드는 `APIService v1beta1.metrics.k8s.io` 는 cluster-scoped 라, 스택을 지우면 Service 만 사라지고 APIService 는 죽은 대상을 계속 가리킨다. 그러면 API discovery 가 실패해 **무관한 네임스페이스까지 전부 Terminating 에 갇힌다** — 실제로 스택 셋이 이틀 넘게 갇혀 있었다. `kube-system` 으로 못박는다. cert-manager 가 같은 이유로 이미 자기 네임스페이스를 고정하고 있었다.
- **GitLab 번들 Prometheus 가 OOM 으로 죽던 문제** (`internal/stack/adapter/helm/resource-defaults.go`): 자원 규모를 낮춘 구성에서 메모리 한도가 328Mi 가 되어 기동 중 OOMKilled(exit 137)로 34번 재시작하며 CrashLoopBackOff 에 갇혔다 — 스택은 "실행 중" 인데 파드 하나가 영원히 안 뜨는 상태다. Prometheus 의 메모리는 스택 크기가 아니라 긁는 대상 수와 WAL 재생에 좌우되므로 비율만으로는 맞출 수 없다. webservice·sidekiq·redis 가 이미 쓰고 있던 하한/상한 방식을 적용했다(한도 1~2Gi). 한도를 1Gi 로 올린 뒤 사용량은 311Mi 로 안정됐다 — 옛 한도 바로 아래였으니 조금만 튀어도 죽던 값이다.
- **모니터링 대시보드의 CI/CD 탭이 파이프라인이 있는 클러스터에서 잠기던 문제** (`web/src/features/observability/pages/monitoring-page.tsx`): 탭을 클러스터가 선언한 타입으로 열고 있었다. 파이프라인이 사는 클러스터는 `types=['pipeline']` 이라 탭이 잠기고, 탭이 열리는 `target` 클러스터에는 파이프라인이 하나도 없어 정상 구성에서 지표가 어디서도 안 나왔다. 등록 타입은 사람이 손으로 적는 값이지만 파이프라인의 `cluster_id` 는 이 화면이 그리려는 데이터 자체다 — 그것으로 연다. 첫 화면이 `clusters[0]` 을 골라 빈 클러스터가 잡히던 것도 스택이 있는 첫 클러스터로 바꿨다.
- **id 를 이름 자리에 넣어 보여주던 곳들** (`internal/stack/adapter/handler/stack_handler.go`, `web/src/features/stack/api/stack-normalizers.ts`, `web/src/features/cicd/pages/cicd-list-page.tsx`): 스택 이력의 "클러스터" 열에 `c75747e4-…`, CI/CD 목록의 "스택" 열에 `stk_c073…` 이 떴다. 이름을 못 찾으면 id 로 대신하고 있었는데, 이름이 아닌 것을 이름 자리에 두면 사용자는 그게 이름인 줄 알고 실제 상황(스택이 지워졌다)을 놓친다. 목록 응답에 `cluster_name` 을 넣고, 못 찾으면 빈 값을 줘서 화면이 `-` 나 "삭제됨" 으로 그리게 했다.
- **스택 이력 화면의 잘못된 정보** (`web/src/features/stack/pages/stack-history-page.tsx`): "현재" 배지와 롤백 버튼이 정확히 거꾸로였다 — 현재 버전을 목록 첫 행으로 봤는데 API 가 오름차순이라, 지금 돌고 있는 버전을 롤백 대상으로 내놓고 되돌아갈 버전에는 버튼이 없었다. 설정 스냅샷은 중첩 객체를 `String()` 으로 찍어 전부 `[object Object]` 였다. 그리고 한 스택의 이력만 보는 화면인데 스택 이름·클러스터를 행마다 반복해 252px 를 먹어 "작업" 열이 칸 밖으로 밀렸다.
- **라이트 테마에서 회색 덩어리로 뜨던 패널 6곳** (`web/src/__tests__/surface-tokens.test.ts` 외): 면을 글자색으로 만들고 있었다(`color-mix(--color-text-primary 45%)`). 다크에서는 흰색 45% 라 적당한 회색이지만 라이트에서는 검정 45% 라 진회색 면이 되고 그 위의 글자가 묻힌다 — 본문 대비 1.97:1 이었다. `--color-surface-sunken` 으로 바꿔 5.44:1 이 됐다. `bg` 에 글자색을 12% 넘게 쓰면 실패하는 소스 스캔 테스트를 뒀다.
- **좁은 창에서 상세 레일이 목록을 짓누르던 문제** (`web/src/components/shared/list-detail-panel.tsx`): 상세가 `shrink-0` 고정폭이라 창이 좁아지면 상세는 폭을 지키고 목록만 줄었다 — 960px 창에서 목록 290px / 상세 380px 로 뒤집혀 7개 컬럼짜리 표가 290px 안에서 가로 스크롤됐다. `xl` 아래에서는 위아래로 쌓는다. 폭은 인라인 style 이 아니라 CSS 변수로 넘긴다 — style 로 박으면 미디어 쿼리로 되돌릴 수 없다.

- **⚠️ 라이트 테마가 와이어프레임처럼 보이던 문제** (`web/src/theme/tokens.generated.css`, `web/DESIGN.md`): 배포본을 본 두 사람이 "흰색 배경에서 스켈레톤 같은 느낌이고 가독성이 떨어진다"고 지적했다. 원인은 셋이었다. 하나, 카드 배경(`--color-surface-card: #f8fafc`)과 페이지 배경(`body #f8fafc`)이 **같은 색**이었다 — 대비 1.00:1 이라 카드가 배경에 녹아 사라졌다. 둘, 면을 나누는 유일한 수단이 된 보더가 `#1f2937`(거의 검정, 카드 대비 14.03:1)이라 본문 텍스트보다 강하게 튀었다. 흰 종이에 검은 선만 남은 상태다. 셋, 그림자·elevation 토큰이 **하나도 없어서** 라이트 테마에는 깊이를 표현할 수단이 아예 없었다(다크는 표면 밝기 차로 버티고 있었다). 이제 카드 `#ffffff` / 페이지 `#f4f6f8` 로 면을 나누고(1.08:1), 보더는 `#cbd5e1`(1.48:1)로 낮추고, elevation 3단을 신설해 라이트는 그림자로 다크는 표면 밝기 차로 깊이를 만든다. 페이지 배경을 낮추면서 AA 를 놓치게 된 `--color-text-muted` 도 함께 조정했다(`#64748b` → `#5f6f85`).
- **다크 테마 보조 텍스트가 WCAG AA 에 미달** (`web/src/theme/tokens.generated.css`): `--color-text-muted: #64748b` 가 카드에서 3.89:1, 페이지 배경에서 4.16:1 로 둘 다 4.5:1 을 넘지 못했다. `#8496a9` 로 올려 6.10 / 6.52 가 됐다.
- **정의 없이 참조되던 CSS 토큰 3개** (`web/src/theme/tokens.generated.css`): `--color-primary`(5곳)·`--color-border`(2곳)·`--color-text-tertiary`(1곳)를 코드가 쓰는데 `index.css` 에 선언이 없었다. 값이 비면 해당 선언이 통째로 무효가 된다. 특히 `--color-primary` 는 모니터링 탭의 "흰 텍스트 + primary 배경" 버튼의 배경이라, **라이트 테마에서 사실상 보이지 않는 버튼**이었다. 생성기가 별칭으로 정의해 셋 다 살렸다.
- **라이트 테마만 surface 토큰의 의미가 뒤집혀 있던 문제** (`web/src/theme/tokens.generated.css`): 다크는 `surface-base` 가 페이지 배경이고 `surface-card` 가 올라온 표면인데, 라이트만 base=`#ffffff` / card=`#f8fafc` 로 반대였다. 두 테마를 같은 의미로 통일했다(라이트 base=`#f4f6f8`, card=`#ffffff`).
- **한국어 사용자에게 영문이 노출되던 번역 키 9개** (`web/src/i18n/ko.json`): `stackTemplatePage.actions.duplicateTemplate`, `stackList.table.cluster` 등이 `en.json` 에만 있어서 `t()` 의 인라인 폴백(영문)이 그대로 화면에 나왔다. 9개를 채우고 정합 테스트(양방향 키 일치·죽은 키·보간 변수 일치)로 고정했다.
- **`Card` 의 `iconBg`/`iconColor` prop 이 동작하지 않던 문제** (`web/src/components/ui/card.tsx`): 삼항의 양쪽 분기가 같은 값이어서 무엇을 넘겨도 항상 indigo 였다. 디자인시스템의 "기능별 색상 매핑"(스택=indigo, 템플릿=emerald, CI/CD=amber …)이 문서에만 있고 코드에 반영되지 않고 있었다.
- **만들어졌지만 접근할 수 없던 화면 4개 배선** (`web/src/app/routes.tsx`, `web/.../sidebar.tsx`): 컴포넌트는 있는데 라우트에도 사이드바에도 연결돼 있지 않아 어디에서도 열 수 없었다. 특히 토큰 관리는 백엔드에 API 가 9개(조회·회전·승인·일시중지·재인증·이벤트) 있는데 화면만 끊겨 있어 **회전 실패나 승인 대기 상태를 UI 로 확인할 방법이 없었다**. `/cicd/golden-paths`, `/admin/token-management`, `/stack/deployments/:deploymentId/retry-history` 를 새로 연결하고, 재배포 기록은 스택 목록 이력 패널에서 진입할 수 있게 버튼을 붙였다. 그리고 `/cicd/create` 는 개발자 셀프서비스 화면을 렌더링하고 있었다 — 정작 파이프라인 생성 화면이 자기 안에서 이 경로로 이동하므로 템플릿을 고르면 엉뚱한 화면으로 튕겼다. 원래 페이지로 바로잡았다.
- **역할별 시작 경로가 로그인과 홈에서 달랐다** (`web/src/features/auth/role-landing.ts`): 로그인은 developer 를 `/cicd/developer-deploy` 로 보내는데 홈의 시작 버튼은 `/cicd/templates` 로 보냈다. 사이드바는 CI/CD 템플릿을 developer 에게 숨기므로 **홈에서 한 번 들어가면 메뉴로 다시 찾아갈 수 없었다**. 두 곳이 각자 목록을 들고 있던 것이 원인이라 단일 출처 모듈로 모았다.
- **라우트 등록기가 정의만 되고 배선되지 않은 것 2건** (`cmd/api/main.go`): `PipelineHandler.RegisterStackRoutes`(`GET /api/v1/stacks/:stackId/pipelines`)와 `RetryHistoryHandler.RegisterRoutes`(`GET /api/v1/stacks/:id/retry-history`)가 호출되지 않아 404 였다. 앞의 것은 프론트가 `?stack_id=` 쿼리 방식을 써서, 뒤의 것은 그 API 를 쓰는 화면 자체가 라우팅돼 있지 않아 드러나지 않았다. 이런 누락은 컴파일도 테스트도 통과하고 해당 엔드포인트만 404 가 되므로, `main.go` 를 파싱해 정의된 등록기가 전부 호출되는지 확인하는 테스트(`cmd/api/wiring_test.go`)를 함께 넣었다. 의도적으로 등록하지 않는 것은 사유와 함께 목록에 적고, 그 목록이 낡는 것도 같은 테스트가 잡는다.

- **이미 있는 저장소에 다시 프로비저닝하면 스캐폴딩이 기존 파일을 덮어쓰던 문제** (`internal/cicd/usecase/provision_app_project.go`, `internal/cicd/port/scm.go`): `CommitFiles` 는 upsert 라 재실행 때마다 `deploy/deployment.yaml` 의 이미지 태그가 초기값(`:bootstrap`)으로 되돌아갔다. 그 태그는 레지스트리에 없으므로 **돌던 배포가 ImagePullBackOff 로 떨어졌다가 CI 가 한 바퀴 더 돌아야 복구된다**(실측 약 3분). 사용자가 고친 Dockerfile·워크플로·매니페스트도 함께 사라지고, 자동화 토큰으로 만든 커밋이라 파이프라인까지 매번 다시 돈다. `EnsureProject` 가 새로 만들었는지(`SCMProject.Created`)를 돌려주고, 기존 저장소면 스캐폴딩과 공용 프로젝트 README 를 쓰지 않는다. 건너뛴 사실은 응답의 `scaffold_skipped` 와 경고로 알린다 — 조용히 넘기면 파일이 갱신된 줄 알고 배포가 예전 매니페스트로 도는 이유를 엉뚱한 데서 찾게 된다.
- **자리표시자 Dockerfile 이 고른 포트에서 듣지 않던 문제** (`internal/cicd/adapter/scaffold/renderer.go`): `FROM nginx:alpine` + `EXPOSE <포트>` 만 넣었는데 nginx 는 80 에서 듣고 `EXPOSE` 는 바인딩을 바꾸지 않는다. Service·Deployment·HTTPRoute 는 사용자가 고른 포트를 가리키므로 **첫 배포가 "파드는 Running 인데 아무도 응답하지 않는" 상태로 끝난다** — 자리표시자에 readinessProbe 가 없어 Argo CD 도 Healthy 로 보고한다. 이제 그 포트를 듣도록 nginx 설정을 함께 쓴다(80 이면 기본값 그대로 둔다).
- **외부 SaaS 도구로 만들어진 죽은 토큰 소스 정리** (`db/migrations/000061_prune_external_scm_token_sources.up.sql`): 새로 만들지 않도록 막았지만 이미 만들어진 행은 남아, 회전 컨트롤러가 주기마다 실패하고(실측 `retry_count` 19, `status=failed_manual`) 사용자 PAT 항목과 provider 가 같아 연동 조회를 모호하게 만든다. `token_type='reissue'` 인 `github`·`github-actions`·`ghcr` 행만 소프트 삭제한다 — 마법사에 넣은 PAT 는 `token_type='pat'` 이라 이 조건이 없으면 살아 있는 자격증명까지 지운다. `token_rotation_events` 외래키 때문에 하드 삭제는 이력을 끊으므로 `deleted_at` 만 채우고, `metadata.pruned_by` 표식으로 down 이 이 마이그레이션이 지운 행만 되살린다.
- **배포는 성공했는데 공개 엔드포인트 검증만 실패하던 문제** (`cd.yml` 의 `deploy-zadara`): `v0.4.1` 태그 배포에서 `helm upgrade` 와 두 deployment 의 `rollout status` 가 모두 성공한 직후 `https://nullus.io/` 가 502 를 돌려줘 잡이 실패로 끝났다. 잡이 끝난 뒤에도 서비스는 멀쩡했다 — 파드가 Ready 인 것과 ingress 가 새 엔드포인트로 수렴하고 옛 파드가 빠지는 것은 다른 일이고, 그 짧은 틈에 한 번뿐인 curl 이 걸린 것이다. 배포가 아니라 검증이 불안정했다. 12초 간격으로 최대 10회까지 기다린다. 이미 열려 있으면 첫 시도에 통과하므로 정상 배포가 느려지지는 않는다.
- **Harbor·Nexus 가 모니터링에서 통째로 빠지던 문제** (`internal/stack/domain/tool_workload.go`): 설치 도구 목록을 만드는 함수가 9개 카테고리만 열거하고 `container_registry`·`package_registry` 를 누락해, 파드가 정상이어도 두 도구가 어느 화면에도 보이지 않았다. 이 함수 하나가 OSS 목록·요약 KPI·Tool Health 표를 모두 먹이므로 해당 파드는 CPU·메모리·Ready Pods 합계에서도 빠져 있었다. 판단 규칙을 `domain.InstalledToolWorkloads` 로 끌어올려 스택 화면과 플랫폼 대시보드가 같은 기준을 쓰게 한다. Nexus 는 컨테이너 레지스트리와 패키지 저장소를 겸할 수 있어 그대로 두면 같은 파드가 두 번 잡히므로 백엔드·프론트 양쪽에서 이름 기준으로 한 번만 남긴다.
- **배포 API 5개가 인증 없이 열려 있던 문제** (`internal/stack/adapter/handler/deploy_handler.go`): `deployHandler` 가 인자로 받은 `v1` 에서 `v1.Group("/stacks")` 를 새로 만들어, 경로만 같고 `main.go` 가 구성한 `stacks` 그룹의 인증 체인을 타지 않았다. `deploy`·`retry`·`continue`·`status`·`deploy/logs` 가 운영 모드에서 미인증으로 호출 가능했다. 웹 파드가 `/api/` 를 백엔드로 프록시하므로 ingress 를 켠 배포에서는 인터넷에서 닿는다.
- **레이트리밋의 인증 한도가 적용되지 않던 문제**: `RateLimiter` 를 전역 `e.Use` 로만 붙였는데 인증 미들웨어는 그룹에만 붙어, Echo 실행 순서상 리미터가 돌 때 사용자를 알 수 없었다. `Authenticated`(300/분) 는 도달 불가능한 죽은 설정이었고 운영에서도 전원이 IP당 30/분을 나눠 썼다.
- **브라우저 WebSocket 이 인증을 통과할 수 없던 문제** (`internal/shared/middleware/ws_subprotocol.go`): 브라우저는 WebSocket 에 `Authorization` 헤더를 붙일 수 없다. `Sec-WebSocket-Protocol` 로 받은 토큰을 헤더로 옮긴 뒤 기존 검증을 그대로 태운다. 쿼리 파라미터를 쓰지 않은 것은 토큰이 액세스 로그·프록시 로그에 남기 때문이다.
- **스택 삭제가 Harbor·Nexus 릴리스와 Argo CD CRD 를 남기던 문제** (`internal/stack/usecase/delete_stack.go`, `internal/stack/domain/helm_releases.go`): 삭제 대상 릴리스 목록에 `harbor`·`nexus` 가 없어 파드와 PVC 가 그대로 남았다 — 아래 `external-secrets`·`metrics-server` 누락과 같은 원인이라 두 릴리스도 단일 출처 `InstalledHelmReleaseNames` 에 등록했다. 또 Argo CD 가 기동하며 스스로 만드는 `default` AppProject 는 `helm uninstall` 로 지워지지 않는데, CRD 정리 가드가 이를 "다른 스택이 사용 중" 으로 오인해 정리를 영구히 건너뛰었다 — 남은 CRD 가 이전 네임스페이스 소유권을 물고 있어 다음 스택 설치가 `invalid ownership metadata` 로 실패했다.
- **`auth.mode=oidc` 인데 issuer 가 비면 조용히 전부 401 이 되던 문제** (`internal/shared/config/auth_validation.go`): JWKS 를 받을 수 없어 모든 요청이 거절되는데 원인이 로그에 드러나지 않았다. 기동 시점에 명시적으로 실패시킨다. `session` 을 운영 모드로 켜면 "인증이 강제되지 않는다" 는 경고를 남긴다. development 는 인증 미들웨어를 붙이지 않으므로 두 검사에서 제외한다.

## [0.4.1] - 2026-08-10

### Added

- **v0.4.1 릴리즈 파이프라인 검증**: `-alpha` 접미사를 뗀 `v0.4.1` 패치 릴리즈 자동화 파이프라인(멀티 아키텍처 이미지 빌드·Helm 차트 게시·Release 본문 자동 렌더링·Zadara Cloud 배포) 검증.

## [0.4.0-alpha] - 2026-08-09

### Added

- **CHANGELOG 누락과 릴리즈 중복을 CI 가 차단** (`scripts/check_changelog.py`, `Lint Review` 의 `📝 CHANGELOG Check`): 릴리즈 정책 §4.2 는 CHANGELOG 갱신을 PR 작성자의 의무로 규정하고 PR 템플릿 체크박스를 두기로 했으나, 체크박스는 차단 장치가 아니라 실제로 #114 가 CHANGELOG 없이 머지됐다 — GitLab 저장소 프로비저닝과 Argo CD 연동이라는 기능 본체가 릴리즈 노트에서 통째로 빠졌고, 릴리즈를 자를 때 커밋 36건을 손으로 대조해 17건을 백필해야 했다. 이제 동작이 바뀌는 파일을 고치고 `CHANGELOG.md` 를 건드리지 않으면 CI 가 실패한다(`no-changelog` 라벨로 면제, 문서·테스트 전용은 자동 면제). 같은 검사가 릴리즈를 자른 뒤 `main` 을 되머지할 때 생기는 `[Unreleased]`↔릴리즈 섹션 중복도 잡는다 — 이 역시 실제로 발생해 8건이 양쪽에 남아 있었다. 검사기는 로컬에서도 그대로 돈다.

- **Zadara Cloud PoC 배포 산출물** (`deploy/csp/zadara/`): `values-zadara.yaml`(워커 1대·NodePort ingress·local-path 스토리지 기준)과 배포 런북 `README.md`.

- **태그 push 로 릴리즈·배포까지 자동화** (`cd.yml`): `create-release` 가 CHANGELOG 의 해당 버전 섹션을 릴리즈 본문으로 쓰고(없으면 자동 생성 노트) GA 이전 버전은 프리릴리즈로 표시한다. `deploy-zadara` 가 bastion SSH 로 Zadara 클러스터에 `helm upgrade` 를 수행하고 공개 엔드포인트 2곳이 200 인지까지 확인한다. `environment: zadara` 로 실배포 직전 승인 게이트를 걸 수 있다. 필요한 secrets: `ZADARA_SSH_KEY`·`ZADARA_HOST`·`ZADARA_USER`·`NULLUS_DB_PASSWORD`·`NULLUS_ENCRYPTION_KEY`.

- **Zadara Cloud PoC 운영 스크립트** (`deploy/csp/zadara/`): 웹 UI 터널(`tunnel.sh`), 로컬 kubectl kubeconfig(`kubeconfig.sh`), 표준 포트 노출(`expose-web.sh`), apiserver 노출(`expose-apiserver.sh`, 기본 미사용), Keycloak realm 구성(`setup-keycloak-realm.sh`), Let's Encrypt TLS(`setup-tls.sh`).

- **차트에 SPA 런타임 설정 값 추가**: `web.auth.{mode,oidcProvider,oidcAuthority,oidcClientId}`. 비우면 이미지에 빌드된 값을 그대로 쓴다.

### Changed

- **브랜치 명명 규칙을 중첩형으로 단일화** (컨벤션 v3): `CLAUDE.md` 는 `feat/<module>/<description>`, `Nullus_PR_커밋_컨벤션.md` v2 는 `feat/stack-tools-wizard` 로 서로 다르게 규정해 실제 브랜치가 34:33 으로 갈려 있었다(릴리즈 정책 §13-6 미해결 과제). 최근 브랜치가 전부 중첩형이라 **`<type>/<module>/<desc>` + `feat`/`fix`/`chore` 3종**으로 정하고 세 문서를 맞췄다. 기존 브랜치는 그대로 두고 신규 생성만 따른다.

- **릴리즈 본문에 설치법·산출물 경로·CHANGELOG 링크 추가** (`create-release`): 기존에는 CHANGELOG 섹션만 그대로 실어, `v0.4.0-alpha` 기준 항목 55개가 아무 안내 없이 먼저 나왔다. 릴리즈 페이지만 보고도 무엇을 어떻게 받는지 알 수 있도록 `helm install` 명령, 이미지·차트 경로, CHANGELOG·릴리즈 정책 링크, 직전 태그와의 compare 링크를 앞에 붙인다. 프리릴리즈면 그 사실도 맨 위에 밝힌다.

- **PR 이벤트 트리거에 `synchronize`·`edited` 추가** (`lint-review.yml`): 지적을 고쳐 push 해도 검사가 다시 돌지 않아, 통과시키려면 PR 을 close/reopen 해야 했다. CHANGELOG 검사처럼 "고치고 다시 밀면 통과" 가 전제인 검사는 이대로면 성립하지 않는다.

- **`runbook_csp.sh`의 레지스트리·이미지 태그 기본값 교정**: `REGISTRY`를 실재 경로(`ghcr.io/cloud-nullus/nullus`)로 바로잡고, 프리릴리즈에 존재하지 않는 `IMAGE_TAG=latest` 대신 비워 두어 차트 `appVersion`으로 폴백한다. 마이그레이션 Job이 이미지 레퍼런스를 문자열로 조립하므로 배포 시작 전에 태그를 한 번 확정한다.

- **스택 설치 경로의 리소스 이름 리터럴을 도메인 상수로 통일**: 스택 설치 오케스트레이터 및 어댑터 곳곳에 흩어져 있던 리소스 이름 리터럴들을 `internal/stack/domain` 및 `internal/shared/domain` 상수로 통일했다. 설치 경로의 리소스 명칭과 상수 정의가 어긋나지 않도록 `connection_contract_test.go` 통합 검증 테스트를 추가해 정합성을 고정했다.

### Fixed

- **QEMU arm64 크로스빌드 시 npm ci Illegal instruction 크래시 방지** (`web/Dockerfile`): React/Vite 정적 자산 빌드 단계는 아키텍처에 독립적이므로 `FROM --platform=$BUILDPLATFORM node:22-alpine AS builder`를 도입해 호스트 네이티브(x86_64)에서 고속 빌드되도록 수정했다. QEMU 에뮬레이션 하에서 발생하는 Node 22 V8 JIT 크래시를 방지하고 Web 이미지 빌드 시간을 ~25분에서 59초로 단축했다.

- **같은 커밋에서 CD 가 두 번 돌던 문제**: `cd.yml` 은 `main` push 와 `v*` 태그 push 양쪽에 걸려 있다. 릴리즈를 머지하고 곧바로 태그를 밀면 같은 커밋으로 두 런이 동시에 시작해 arm64 크로스빌드가 통째로 두 번 돈다 — `ec123dc` 에서 실제로 발생했다. 태그 런이 브랜치 런의 상위집합(차트·릴리즈·배포까지)이므로 커밋 SHA 기준 `concurrency` 로 묶어 앞선 런을 접는다. 태그는 언제나 그 커밋이 브랜치에 올라간 뒤에 밀리므로 남는 쪽이 태그 런이다.

- **OpenBao 운영 모드 전환 및 ESO 시크릿 평면 구축**: dev 모드로 작동하던 OpenBao를 영속 스토리지 기반 운영 모드로 전환하고, 정적 토큰 인증을 Kubernetes Auth 기반 단기 자격으로 교체했다. OpenBao에 보관된 시크릿이 External Secrets Operator(ESO)를 거쳐 최종 애플리케이션 파드까지 안전하게 전달되도록 주입 경로를 구성했다.

- **OSS OIDC 클라이언트 자동 프로비저닝 기능 추가**: 스택 설치 시 OSS 애플리케이션의 OIDC 클라이언트를 Keycloak에 자동으로 등록하는 SSO 핸드오프 경로를 구현했다. 클라이언트 시크릿을 플랫폼이 생성하여 OpenBao에 안전하게 기록하고 Keycloak에 즉시 동기화하도록 연동했다.

- **시크릿 회전 후 소비자 워크로드 반영(Rolling Restart) 구현**: 회전된 시크릿이 실제 서비스에 즉시 반영되도록 시크릿 소비 워크로드를 재시작하는 단계를 추가했다. 시크릿 소비 방식(환경변수·파일 마운트 등)에 따라 재시작 필요 여부를 판별하여 프로바이더별 정책으로 분기 처리하도록 배선했다.

- **에어갭 무인 설치의 백엔드 API 경로 통합**: Helm CLI를 직접 호출해 백엔드를 우회하던 기존 에어갭 설치 방식을 정식 백엔드 API 경로로 통합했다. 설치 과정에서 인-클러스터 자기 자신을 클러스터 목록에 자동 등록하고, 부트스트랩 자격증명의 안전한 폐기 및 멱등한 재발급 흐름을 추가했다.

- **GitLab 저장소 자동 프로비저닝 및 Argo CD 연동**: CI/CD 파이프라인 생성 시 GitLab 앱 저장소와 빌드 스캐폴딩(.gitlab-ci.yml, Dockerfile, K8s 매니페스트)을 자동으로 생성하고, GitLab Runner 빌드 이미지와 Argo CD GitOps 배포까지 이어지는 엔드투엔드 연동 경로를 구현했다.

- **스택 접속 정보 API 도입 및 리소스 이름 단일 출처화**: Helm values와 프런트엔드 안내 문구에 파편화되어 하드코딩되어 있던 Secret·서비스 이름을 `internal/stack/domain/connection.go` 상수로 단일 출처화했다. `GET /api/v1/stacks/:stackId/connection-info` 엔드포인트를 통해 서버가 확정한 정합성 있는 접속 정보를 내려주고, 프런트엔드는 응답 데이터를 직접 구동해 표시하도록 개선했다.

- **컨테이너 레지스트리로 Harbor / Nexus 선택 가능** (스택 템플릿 `gitlab-harbor-v1`·`gitlab-nexus-v1` 추가): 기존에는 GitLab 내장 레지스트리뿐이었다. 템플릿은 Harbor 를 도구로 명시하면서도 **설치 단계가 아예 없어**, 고르면 CI 가 이미지를 올릴 곳이 존재하지 않았다. `installing_harbor`·`installing_nexus`·`provisioning_nexus` 를 설치 DAG 에 추가하고, 자격증명은 다른 도구와 같이 OpenBao → ESO 로 프로비저닝한다. Nexus 는 설치만으로는 Docker 커넥터도 저장소도 없고 관리자 비밀번호를 컨테이너가 무작위로 만들기 때문에, `provisioning_nexus` 가 비밀번호를 교체하고 `DockerToken` realm 을 켜고 docker/maven/npm 저장소와 8082 Service 를 만든다(재실행 가능). CI/CD 쪽에는 Nexus resolver 를 추가해 이미지를 UI(8081)가 아닌 Docker 커넥터(8082)로 보낸다 — UI 주소로 push 하면 이미지 대신 HTML 을 받는다. 로컬 kind 에서 두 레지스트리 모두 설치·이미지 push 를 확인했고, Harbor 는 GitLab CI 빌드 → push → Argo CD 동기화 → 파드 구동까지 digest 일치로 검증했다.

- **CI/CD 리스트와 배포 확인 모달에 사용 스택 표시**: 파이프라인은 이미 `stack_id` 를 저장하고 API 로도 내려주고 있었으나 화면에 없었다. 스택마다 레지스트리가 달라 이미지가 어디로 올라가는지가 달라지므로, 배포 직전과 목록에서 어느 스택 위에서 도는지 드러낸다.

- **OIDC 로그인이 유효한 토큰에도 401 로 거부되던 문제**: `setup-keycloak.sh` 가 `nullus-app` 클라이언트에 audience 매퍼를 만들지 않아 Keycloak 이 기본값 `aud: account` 로 토큰을 발급했고, `NULLUS_AUTH_OIDC_AUDIENCE=nullus-app` 를 검증하는 JWT 미들웨어가 이를 거부했다. audience 프로토콜 매퍼를 추가한다.

- **OIDC 로그인 후 스택 생성이 FK 위반으로 실패하던 문제**: JWT 미들웨어가 `User.OrgID` 를 채우지 않고 토큰에도 `org_id` 클레임이 없어, `resolveOrgID` 가 어떤 마이그레이션에도 존재하지 않는 `00000000-0000-0000-0000-000000000001` 로 폴백해 `stacks_org_id_fkey` 위반이 났다. 미들웨어가 `org_id`(및 `organization_id`/`org` 별칭) 클레임을 principal 로 전달하고, `setup-keycloak.sh` 가 사용자 속성과 클레임 매퍼를 등록한다. 폴백 조직도 시드 마이그레이션이 실제로 만드는 조직(`internal/shared/domain.SeededDefaultOrgID`)으로 바로잡고 `NULLUS_DEFAULT_ORG_ID` 로 덮어쓸 수 있게 했다.

- **로컬 Keycloak 이 기동 15분 뒤 realm 을 통째로 잃던 문제**: `KC_DB: dev-mem`(H2 인메모리)이 마지막 커넥션과 함께 사라져 `Table "REALM" not found (this database is empty)` 로그와 함께 토큰 엔드포인트가 500 을 반환했다. `dev-file` + 볼륨으로 바꾸고, KC 26 에서 deprecated 된 `KEYCLOAK_ADMIN*` 을 `KC_BOOTSTRAP_ADMIN_*` 로 교체했다.

- **`setup-keycloak.sh` 가 수동 개입 없이는 완주하지 못하던 문제**: master realm 의 `sslRequired` 기본값 때문에 평문 HTTP 관리 API 호출이 `HTTPS required` 로 막혔다. 실패 시 컨테이너 안에서 `kcadm` 으로 한 번 완화하고 재시도하도록 자동화하고, `nullus` realm 도 `sslRequired=none` 으로 생성한다. 응답이 비었을 때 JSON 파서가 traceback 으로 죽던 문제, Keycloak 24+ User Profile 이 선언되지 않은 `org_id` 속성을 조용히 버리던 문제도 함께 정리했다.

- **`external-secrets`·`metrics-server` 릴리스가 스택 삭제 대상에서 누락되던 문제**: `DeleteStack` 의 `stackHelmReleaseNames` 에 두 릴리스가 없어 `helm uninstall` 이 아예 호출되지 않았고, ESO 의 CRD 24개·ClusterRole 5개·webhook 2개가 그대로 남아 다음 설치를 Helm ownership 충돌로 막았다. 시크릿 평면이 상시 설치로 바뀌면서 스택을 지울 때마다 재현되는 상태가 됐다. 설치/삭제가 각자 목록을 들고 있던 것이 근본 원인이므로 `internal/stack/domain` 에 단일 출처(`InstalledHelmReleaseNames`)를 두고 양쪽이 참조하게 했다. 차트를 새로 추가하고 삭제 목록에 등록하지 않으면 설치 측 테스트가 먼저 실패한다.

- **재설치 시 cluster-scoped 리소스 소유권 충돌**: CRD·ClusterRole·webhook 은 네임스페이스 리소스와 달리 스택 삭제 후에도 남기 쉬운데(Helm 이 CRD 를 의도적으로 남기고, uninstall 이 부분 실패하거나 네임스페이스가 강제 삭제되면 RBAC 도 남는다), 옛 `meta.helm.sh/release-namespace` 가 박혀 있으면 다른 네임스페이스로의 재설치가 `invalid ownership metadata` 로 거부된다. ESO 만 갖고 있던 소유권 인수 로직을 모든 차트 설치 경로로 일반화했다. **삭제가 아니라 인수인 이유**는 CRD 가 클러스터 전역이라 지우면 다른 스택의 커스텀 리소스가 함께 사라지기 때문이다. 같은 이유로 **옛 릴리스가 아직 살아 있으면 인수하지 않는다** — 살아 있는 다른 스택의 리소스를 탈취하면 그 스택 삭제 시 함께 삭제된다. ESO 전용 인수 경로에도 같은 가드를 적용했다.

- **스택 설치 DAG에서 시크릿 평면 단계 누락 — GitLab+Argo CD 배포가 MinIO에서 멈추던 문제**: 오케스트레이터의 정식 순서(`orderedStep`)에는 `installing_external_secrets`·`provisioning_secrets`가 있었으나, 실제 실행을 구동하는 `internal/stack/usecase/install_stack.go`의 `installDAG`에는 두 단계가 없었다. PostgreSQL/MinIO 차트는 비밀번호를 values 로 받지 않고 `nullus-postgresql-credentials`·`nullus-minio-credentials`를 `existingSecret`으로 참조하는데, 이 Secret 을 만드는 경로가 `provisioning_secrets` 하나뿐이라 파드가 `FailedMount`로 영원히 기동하지 못하고 Helm 릴리스가 `pending-install`에 고착됐다. 두 단계를 DAG 에 추가하고, 시크릿 평면(OpenBao → ESO → provisioning)을 스토리지 차트 **앞으로** 재배치했다. OpenBao 는 file 스토리지 백엔드를 쓰므로 PostgreSQL/MinIO 에 의존하지 않아, `installing_openbao`의 잘못된 선행 의존도 함께 제거했다. `installDAG` 와 `orderedStep` 의 순서 일치를 양쪽 패키지에서 테스트로 고정한다.

- **시크릿 평면이 `authentication.provider=openbao` 에서만 동작하던 문제**: 차트가 프로비저닝된 Secret 을 무조건 참조하므로 선택형일 수 없는데도 opt-in 으로 게이팅되어 있었다. 프런트엔드 기본값이 `provider: ''` 라 기본 구성에서 항상 설치가 실패했다. 게이팅을 제거해 항상 실행되도록 했다.

- **`provisioning_secrets` 단계가 `unknown step` 으로 실패하던 문제**: 차트가 없는 단계인데 처리 블록이 `chartSpecForStep()` 뒤에 있어 spec 조회 실패로 떨어졌다. OpenBao 를 켜더라도 실행될 수 없던 잠재 결함으로, 다른 무차트 단계와 같이 spec 조회 앞으로 옮겼다.

- **GitLab 이 존재하지 않는 DB Secret 을 참조하던 문제**: PostgreSQL 차트를 `auth.existingSecret=nullus-postgresql-credentials` 로 설치하면 bitnami 차트가 릴리스 이름(`nullus-postgresql`)으로 Secret 을 만들지 않는데, GitLab values 의 `global.psql.password.secret` 은 그 이름을 가리키고 있었다. 그 결과 migrations·webservice·toolbox·sidekiq 파드가 `MountVolume.SetUp failed ... secret "nullus-postgresql" not found` 로 기동하지 못했다. 두 참조가 같은 상수(`ProvisionedPostgresSecret`)를 쓰도록 맞추고, 어긋나면 실패하는 테스트를 추가했다.

- **`OnDelete` 전략 워크로드에서 준비 검사가 실패하던 문제**: `kubectl rollout status` 는 RollingUpdate 전략에서만 동작하는데, OpenBao 차트의 StatefulSet 은 `OnDelete` 를 쓴다. 그래서 모든 차트가 정상 설치된 뒤에도 health_check 가 `rollout status is only available for RollingUpdate strategy type` 으로 실패해 배포가 통째로 failed 로 떨어졌다. 전략을 먼저 확인해 `OnDelete` 면 `readyReplicas` 도달을 기다리는 경로로 전환한다(에러 문구 기반 폴백 포함).

- **배포 타임라인에서 `provisioning_secrets` 로그가 사라지던 문제**: UI 의 `DEPLOY_STAGES` 가 설치 단계를 `installing_` 접두사로만 매칭해, 접두사를 쓰지 않는 `provisioning_secrets` 가 어느 스테이지에도 잡히지 않았다. 이 단계가 실제로 실행되기 시작하면서 드러난 문제로, Install 스테이지에 명시적으로 추가했다.

- **OpenBao unseal 사이드카가 키를 보내지 않고도 "제출 완료" 를 찍던 문제**: Secret 이 만들어지기 전이나 kubelet 이 마운트를 동기화하기 전에는 glob 이 아무것도 잡지 못하는데, 그때도 성공 로그를 남겨 "키를 보냈는데 안 열린다" 로 오독되었다. 제출한 조각 수를 세어 0 이면 대기 중임을 그대로 로그에 남긴다.

- **사설 CA 시크릿이 필수 마운트로 걸려 있던 문제**: API 파드가 `nullus-wildcard-tls` 시크릿을 `optional` 없이 마운트해, 해당 시크릿이 없는 환경에서는 파드가 `FailedMount`로 Pending에 갇혔다. 실 클러스터 서버 사이드 dry-run 에서만 드러나고 로컬 `helm template`은 통과시킨다. 시크릿을 선택 사항으로 바꾸고, `merge-ca-certs` 초기화 컨테이너가 CA 파일이 없으면 시스템 번들만 쓰도록 수정했다. 시크릿 이름은 `caBundle.secretName` 값으로 분리했다.

- **keycloak 서브차트 벤더링 누락으로 차트가 렌더조차 되지 않던 문제**: `Chart.yaml` 에 `keycloak` 의존성이 추가되었으나 `Chart.lock` 과 `charts/keycloak-24.4.5.tgz` 가 함께 커밋되지 않아, 저장소를 클론한 상태에서 `found in Chart.yaml, but missing in charts/ directory: keycloak` 으로 실패했다. 같은 디렉토리의 `postgresql` 은 벤더링되어 있어 규칙이 어긋나 있었다. 잠금 파일과 차트를 함께 커밋한다.

- **`bitnami/*` 이미지 소멸로 차트 기본값이 pull 되지 않던 나머지 3건**: #106 이 `postgresql.image` 를 `bitnamilegacy/*` 로 옮겼으나, 같은 원인의 참조가 세 군데 남아 있었다 — 번들 PostgreSQL 의 `volumePermissions.image`(`bitnami/os-shell`), 그리고 keycloak 서브차트가 쓰는 `bitnami/keycloak:26.1.0-debian-12-r0` 와 그 서브차트 자신의 PostgreSQL `bitnami/postgresql:17.2.0-debian-12-r6`. 뒤의 두 건은 차트 기본값 렌더에 그대로 나타나 **기본 설치가 `ImagePullBackOff` 로 실패**했다. 세 건 모두 legacy 경로로 고정해, 이제 기본값이 참조하는 이미지 6종이 모두 익명 pull 된다.

- **게시된 web 이미지로는 어떤 환경에서도 SSO 로그인이 되지 않던 문제**: Vite 는 `import.meta.env.VITE_*` 를 빌드 시점에 문자열로 인라인한다. 그래서 `cd.yml` 이 주입한 OIDC issuer(`http://keycloak.nullus.internal/realms/nullus`)가 이미지에 박혀, 그 호스트가 존재하지 않는 모든 배포에서 로그인 화면이 `Failed to fetch` 로 끝났다. 차트의 `config.auth.oidcIssuerUrl` 은 API 에만 적용되어 프런트엔드에는 닿지 않는다. 컨테이너 기동 시 `/config.js` 를 생성해 `window.__NULLUS_CONFIG__` 로 주입하는 런타임 설정을 도입한다 — 우선순위는 `런타임 > 빌드타임 > 기본값` 이라 로컬 개발 동작은 그대로다. 이제 환경마다 이미지를 다시 빌드하지 않아도 된다.

- **`setup-keycloak.sh` 를 재실행하면 사용자 `email` 이 지워지던 문제**: 신규 생성 경로는 `email` 을 넣지만 기존 사용자 갱신 경로가 이를 빠뜨렸다. Keycloak 의 사용자 PUT 은 표현을 통째로 교체하므로 두 번째 실행부터 `email` 이 비었고, API 는 토큰의 `email` 클레임으로 사용자를 조회하므로 로그인 후 사용자 매칭이 깨졌다. 갱신 payload 에 `email`·`emailVerified`·`enabled` 를 함께 보낸다.

- **재배포하면 열려 있던 탭이 죽던 문제**: 빌드 산출물은 파일명에 내용 해시가 들어가므로 재배포하면 이전 청크가 사라진다. 배포 전에 열려 있던 탭이 lazy import 를 시도하면 `Failed to fetch dynamically imported module` 로 화면이 죽고, 「다시 시도」를 눌러도 같은 옛 모듈 그래프를 다시 써서 복구되지 않았다. 게다가 nginx 의 SPA 폴백이 없는 `.js` 요청에도 `index.html` 을 200 으로 돌려줘, 브라우저가 HTML 을 모듈로 파싱하다 실패하는 혼란스러운 오류가 됐다. `/assets/` 는 없으면 404 를 주고(내용 해시가 있으므로 장기 캐시), 앱은 청크 오류를 감지해 **한 번만** 자동 새로고침한다.

- **SSO 로그인 시 모든 사용자가 developer 로 보이던 문제**: Keycloak 은 `realm_access` 를 기본적으로 **액세스 토큰에만** 싣는데, 프런트엔드는 ID 토큰 클레임(`user.profile`)에서 롤을 찾았다. 항상 빈 배열이 나와 `admin`·`devops` 계정까지 전부 최저 권한인 `developer` 로 떨어졌고, 관리자 화면을 아무도 쓸 수 없었다. ID 토큰을 먼저 보고 없으면 액세스 토큰을 직접 열어 읽는다 — 액세스 토큰의 `realm_access` 는 Keycloak 이 항상 넣어 주므로 서버 매퍼 설정과 무관하게 동작한다. 프로비저닝 스크립트에는 ID 토큰에도 롤을 싣는 realm-roles 매퍼를 추가한다(보강).

- **인증 오류가 나면 빠져나올 방법이 없던 문제**: 브라우저에 남은 세션과 서버 상태가 어긋나면 로그인 흐름이 화면에 문구만 남긴 채 끝났다(`Session not active`, `No matching state found in storage`, `login_required`). 개발자 도구로 저장소를 비우지 않으면 복구가 불가능했다. 저장소를 비우고 다시 시도하면 풀리는 오류를 가려내 **원인당 한 번만** 자동 재시도하고, 실패하면 「다시 로그인」 버튼을 함께 보여 준다. 무한 리다이렉트를 피하려고 마커를 `sessionStorage` 에 두어 새로고침해도 반복되지 않게 했다.

- **클라이언트가 바뀐 뒤 로그아웃도 되지 않던 문제**: Keycloak 은 `id_token_hint` 가 오면 그 토큰의 발급 대상과 `client_id` 가 같은지 대조하고, 다르면 로그아웃을 통째로 거부한다(`Invalid parameter: id_token_hint`). 클라이언트 ID 를 바꾼 뒤 브라우저에 이전 세션이 남아 있으면 정확히 이 상태가 되어, 사용자가 **로그아웃도 못 하는** 막다른 화면에 갇힌다. 힌트로 쓸 토큰이 현재 클라이언트의 것인지(`azp`/`aud`) 확인하고, 어긋나면 힌트를 버리고 `client_id` 만으로 로그아웃한다. 클라이언트의 `post.logout.redirect.uris` 도 함께 설정한다.

- **SSO 계정과 DB 시드 사용자의 이메일이 어긋나 있던 문제**: 인증이 OIDC 로 넘어가면서 API 는 토큰의 `email` 클레임으로 `users` 행을 찾는데, `scripts/setup-keycloak.sh` 가 만드는 계정(`admin@nullus.io`·`devops@nullus.io`·`dev@nullus.io`)과 시드 마이그레이션의 사용자(`admin@nullus.io`·`kim@nullus.io`·`park@nullus.io`)가 달랐다. `admin` 을 뺀 두 계정은 로그인은 되지만 사용자 매칭이 되지 않는다. Keycloak 쪽을 정본으로 삼아 대응 사용자와 조직 소속을 시드한다(`000058_seed_sso_users`). `kim@`·`park@` 는 여러 시드에서 문자열로 참조되는 화면 샘플이라 지우지 않고 추가만 한다.

- **OIDC 클라이언트 ID 가 프런트엔드와 나머지에서 어긋나던 문제**: 프런트엔드 기본값과 `cd.yml` 은 `nullus-web`, `setup-keycloak.sh` 가 만드는 클라이언트와 API audience 기본값(`configs/config.yaml`, 차트 values)은 `nullus-app` 이었다. 실제로 존재하는 클라이언트인 `nullus-app` 으로 통일한다.

- **Keycloak 번들 PostgreSQL 리소스 이름 충돌로 스택 설치가 실패하던 문제**: Keycloak 서브차트가 자체 PostgreSQL을 띄울 때 상위 PostgreSQL 차트와 릴리스 리소스 이름(`<release>-postgresql`)이 동일하게 조립되어 StatefulSet·Service·Secret 등 7종 리소스가 충돌하며 설치가 거부되었다. `keycloak.postgresql.nameOverride`를 통해 서브차트 리소스 이름을 분리 지정하여 충돌을 해소했다.

- **Helm 차트 기본값 불일치로 인한 API 암호화 에러 및 DB 인증 실패 문제**: 차트의 `secrets.encryptionKey` 기본값이 26바이트라 AES-256 검증(32바이트)에서 500 에러가 났고, `secrets.dbPassword`와 `postgresql.auth.password`가 어긋나 API의 DB 접속이 거부되었다. 암호화 키를 32바이트 placeholder로 교체하고 DB 비밀번호 헬퍼를 도입해 단일 출처로 맞추었으며, 소멸된 Bitnami 이미지 태그를 `bitnamilegacy` 경로로 교정했다.

- **로컬 런북 기동 시 Keycloak TLS 요구 및 OpenBao 시드 실패로 무인 실행이 차단되던 문제**: Keycloak realm의 `sslRequired` 기본값 때문에 로컬 HTTP 환경에서 관리 API 토큰 발급이 거부되었고, `seed-token-sources.sh`에서 호스트 도메인 해석이 불가능한 OpenBao 쓰기 실패 시 스크립트 전체가 중단되었다. 컨테이너 내부 `kcadm`으로 SSL 요구를 비활성화하도록 자동화하고, OpenBao 시드 실패는 경고 로그 후 계속 진행되도록 완화했다.

- **GitLab CI/CD 스택 설치 후 러너 미등록·레지스트리 통신 오류로 파이프라인이 동작하지 않던 문제**: GitLab Runner 등록 토큰 조회 실패 에러가 상쇄되어 러너 설치 없이 완료 처리되었고, `privileged` 설정 누락으로 DinD 실행이 불가능했으며, Container Registry가 미노출 및 내부 S3 리다이렉트 문제로 이미지 push/pull에 실패했다. 러너 재시도 에러 보존 및 TOML 규격 적용, Registry S3 백엔드·Gateway 8181 포트 라우팅 및 `allowedRoutes` 전면 허용 정책을 적용해 CI/CD 파이프라인 정상 구동을 복구했다.

- **OpenBao 부트스트랩 재실행 시 인증 토큰 부재로 스택 재배포가 막히던 문제**: 첫 부트스트랩 성공 후 root token이 폐기되어 재실행 시 `BAO_TOKEN`이 비어 있었는데, 부트스트랩 여부를 검증하는 `bao list` 호출이 인증을 요구해 항상 에러로 중단되었다. 토큰 부재 시 마운트된 ServiceAccount 토큰으로 Kubernetes Auth 로그인을 시도해 상태를 파악하도록 복구하고, 대기 타임아웃 확장 및 파드 에러 로그를 덤프하도록 개선했다.

- **스택 시크릿 리졸버의 잘못된 DB 캐스팅으로 인한 OpenBao 시크릿 조회 실패 문제**: VARCHAR 타입인 `stacks.id`를 조회하는 SQL 쿼리에 `$1::uuid` 타입 캐스팅이 들어가 있어 SQL 오류(`SQLSTATE 42883`)가 발생하고 스택 범위 OpenBao 시크릿 조회가 항상 실패했다. 불필요한 UUID 캐스팅을 제거해 조회 쿼리를 정상화했다.

- **스택 상세 화면의 연결 정보 안내 명령어가 고정 네임스페이스 및 잘못된 Secret 이름을 참조하던 문제**: Stack List의 connection info가 실제 네임스페이스와 상관없이 `-n nullus`를 하드코딩하고 Argo CD 시크릿 이름을 잘못 안내하여 명령 실행 시 `NotFound` 오류가 발생했다. 연결 정보 헬퍼에 네임스페이스 파라미터를 연결하고 Argo CD·PostgreSQL·MinIO 시크릿 명칭을 실제 Helm 릴리스 규격에 맞게 정정했다.

- **CI/CD 목록 모바일 화면에서 파이프라인 상세 클릭 시 ReferenceError로 페이지가 크래시되던 문제**: 모바일 레이아웃의 `PipelineDetailPanel` 컴포넌트에 정의되지 않은 `activeDeploymentId` 변수를 넘기고 있어 런타임 `ReferenceError`가 발생하고 화면이 죽던 현상을 해결했다. 미정의 prop 전달 코드를 제거해 모바일 상세 패널 렌더링을 정상화했다.

- **개발자 배포 및 스택 템플릿 화면에서 번역 키 문자열이 그대로 노출되던 문제**: 다국어 파일(ko/en)에서 `developerDeployPage.actions.execute` 및 `stackTemplatePage.actions.viewDetail` 키가 누락되어 UI 버튼에 번역되지 않은 키 문자열이 그대로 노출되던 문제를 다국어 항목 정의 보충으로 해결했다.

- **게이트웨이 포트포워딩 스크립트가 컨텍스트 오류와 네임스페이스 누락 원인을 구분하지 못하던 문제**: 포트포워딩 스크립트(`port-forward-gateway.sh`)가 K8s 컨텍스트 연결 불능 상황과 특정 네임스페이스 미존재 상황을 구별하지 않고 동일한 컨텍스트 에러 문구로 안내하던 문제를 수정했다. 실패 원인을 분리해 탐지하고 등록된 컨텍스트 목록을 검색해 올바른 실행 명령을 제안하도록 보강했다.

- **같은 클러스터에 두 번째 스택을 설치하면 metrics-server 에서 항상 실패하던 문제**: metrics-server 는 클러스터당 하나인데 차트가 ClusterRole 등 클러스터 범위 리소스를 만든다. 기존 스택이 그것을 소유하고 있어 `invalid ownership metadata` 로 설치가 멈췄다. cert-manager 는 이미 재사용 경로를 갖고 있었으나 metrics-server 에는 없었다. `v1beta1.metrics.k8s.io` APIService 존재로 감지해 재사용하며, 설치를 건너뛴 경우 health check 도 릴리스 부재를 허용한다(설치는 끝났는데 검증에서만 실패하던 구멍).

- **스택을 지우고 다시 설치하면 Argo CD 설치가 막히던 문제**: helm 은 uninstall 시 CRD 를 지우지 않아 `applications.argoproj.io` 등이 삭제된 릴리스 소유로 남았다. 삭제 경로가 Argo CD CRD 를 정리하되, 다른 스택이 아직 Application 을 갖고 있으면 건너뛴다(지우면 그 스택의 배포 정의가 사라진다). `external-secrets` 릴리스도 삭제 대상에 포함한다.

- **`registry.<도메인>` 을 두 라우트가 주장하던 문제**: 기존 registry 라우트가 어떤 레지스트리를 골랐든 `gitlab-registry` 로 보내, Harbor/Nexus 를 고른 스택이 비어 있는 GitLab 레지스트리를 가리켰다. 선택한 레지스트리 하나만 이 호스트를 갖도록 정리했다(고정: `TestGatewayRoutes_RegistryHostHasSingleOwner`).

- **PostgreSQL 이 설치 시점마다 달라지던 문제**: 차트 버전이 고정돼 있지 않았고(다른 스텝은 모두 고정), 차트 기본 이미지도 `bitnami/postgresql:latest` 였다. bitnami 가 2025-08 에 버전 태그를 `bitnamilegacy/*` 로 옮기면서 `bitnami/postgresql` 에는 `latest` 만 남았기 때문이다. 차트 `16.7.27` 과 이미지 `bitnamilegacy/postgresql:17.6.0-debian-12-r4` 로 짝을 맞춰 고정한다. **주의**: `bitnamilegacy` 는 동결 저장소라 18.x 가 없어 PostgreSQL 이 17.6.0 으로 내려간다. `latest` 로 이미 18.x 데이터가 생긴 스택은 재설치나 dump/restore 가 필요하다. 동결 저장소는 보안 패치가 오지 않으므로 장기적으로는 다른 차트로 이전해야 한다.

- **Harbor 설치 시 `externalURL` 이 실제 주소로 채워지지 않던 문제**: Harbor 는 이 값을 토큰 발급 엔드포인트로 클라이언트에 돌려준다. 기본값이 그대로 남아 `docker login`/push 가 존재하지 않는 호스트로 토큰을 요청해 `no such host` 로 실패했다 — 레지스트리는 떠 있는데 push 만 안 되는 상태다. accessDomain(없으면 네임스페이스)으로 채운다.

- **스택 설정이 없을 때 선택형 설치 단계가 켜져 있던 문제**: `isStepEnabled` 가 설정을 모르면 모든 단계를 활성으로 봤다. 선택형 도구가 추가될수록 아무도 고르지 않은 것을 설치하게 되고, 순서 검증도 그 단계를 기다리다 멈춘다.

- **레지스트리 템플릿이 인메모리에만 있고 DB 시드에는 없던 문제**: 런타임은 PostgreSQL 을 읽으므로 실제 배포에서는 템플릿 목록에 뜨지 않았다. `000059` 마이그레이션으로 템플릿과 호환성 매트릭스를 함께 시드하고, 인메모리에만 있고 마이그레이션에 없는 항목을 계약 테스트가 잡는다.

- **로컬 런북이 helm 없이 API 를 기동하던 문제**: OCI 차트(envoy gateway)는 helm CLI 로 폴백하는데, PATH 에 helm 이 없으면 게이트웨이 설치만 `executable file not found` 로 실패하고 원인이 설치 로그 깊은 곳에만 남았다. 기동 전에 확인해 즉시 드러낸다.

- **배포 워크플로가 뒤처진 커밋을 체크아웃하던 문제**: `git fetch` 는 원격 추적 ref(`origin/main`)만 갱신하고 로컬 브랜치는 그대로 두는데, `git checkout --detach "$REF"` 가 그 로컬 브랜치를 잡았다. `workflow_dispatch` 배포가 `success` 로 끝나면서도 24 커밋 전 소스로 차트를 렌더했다. 태그 push 경로는 태그가 fetch 되므로 드러나지 않던 자리다. 브랜치는 `refs/remotes/origin/`, 태그는 `refs/tags/` 로 풀어서 넘기고, 배포 디렉토리에 남은 손수정과 미추적 파일이 렌더에 섞이지 않도록 `--force` 와 `reset --hard` 를 함께 건다.

## [0.3.0-alpha] - 2026-07-28

첫 GitHub Release·태그입니다. 이전 `0.1.0-alpha`·`0.2.0-alpha` 섹션은 태그가 발행되지 않은 기록상의 버전입니다 (릴리즈 정책 §0).

### Added

- **Helm 차트 OCI 게시** (#100): `v*` 태그 push 시 `cd.yml`의 `publish-chart` 잡이 차트를 `oci://ghcr.io/cloud-nullus/charts`에 게시. 이제 저장소를 clone하지 않고 `helm install nullus oci://ghcr.io/cloud-nullus/charts/nullus --version 0.3.0-alpha`로 설치할 수 있다.
- **로컬 kind 개발용 값 파일 분리** (#100): `deploy/helm/nullus/values-dev.yaml` 신규 — 로컬 빌드 이미지 태그(`dev`), `pullPolicy: Never`, 단일 레플리카, 개발 로그 설정. 차트 기본값에 로컬 환경 값을 커밋하던 문제를 구조적으로 차단한다.
- **에어갭(air-gap) 클린 설치 전 과정 자동화** (#75, #76): 오프라인 번들만으로 외부 접근·스택 설치·DB 마이그레이션까지 동작하도록 누락 단계를 자동화. 인-클러스터 오케스트레이터가 온라인 Helm 레포 대신 로컬 OCI 레지스트리(`kind-registry:5000/charts`)에서 차트를 pull 하도록 지원.
- **카카오클라우드 air-gap 배포 자산** (#79): OpenTofu IaC 3모듈(network/security/compute)과 provision→build→transfer→install→expose 5단계 스크립트, 운영 문서를 추가. kind 기반 트랙과 kubeadm 멀티노드 트랙을 분리.
- **에어갭 번들 SBOM 자동 생성** (#91): `scripts/pre/generate-sbom.sh`가 번들 이미지와 Helm 차트의 SBOM(SPDX/CycloneDX)을 `bundle/sbom/`에 생성. syft 미설치 시 경고 후 건너뛰어 번들 빌드를 막지 않음.
- **에어갭 설치 검증에 노드 아키텍처 확인 추가** (#90): `99-verify.sh`가 각 노드의 `architecture`를 `EXPECTED_ARCH`(기본 `amd64`)와 대조해, arm64 번들을 x86 호스트에 반입하는 오반입을 설치 검증 시점에 검출.
- **스택 설정 export/import** (#92): export API를 UI에 연결하고 파일명·포맷 선택 UX를 추가. import는 preview → apply 흐름으로 신규 스택 생성과 동일 이름 스택 업데이트를 모두 지원하며, OSS별 리소스 override와 round-trip 정합성을 함께 정리.
- **공유 Prometheus용 ServiceMonitor** (#71): Argo CD / Envoy Gateway / GitLab Prometheus Server 메트릭을 공유 Prometheus가 수집하도록 `deploy/monitoring/prometheus-servicemonitors.yaml` 추가.
- **Helm 차트 `imagePullSecrets` 지원** (#87): 차트에 top-level `imagePullSecrets`(기본 `[]`)를 추가하고 runbook이 생성한 `ghcr-pull-secret`을 실제로 전달하도록 배선 — private 이미지 pull에 시크릿이 적용되지 않던 문제 해소.
- **GitLab object storage 버킷 사전 생성 단계** (#70): 설치 DAG에 `installing_object_storage_buckets`를 추가하고 `installing_gitlab`의 선행 단계로 연결.
- **에어갭 차트 카탈로그 동기화** (#59): 스택 오케스트레이터가 사용하는 14개 Helm 차트 중 번들에 누락된 9개를 추가하고 버전이 어긋난 3개를 정렬 — 에어갭에서 일부 스택을 설치할 수 없던 문제 해소.
- **스택 배포 재개 흐름 보강** (#58): 실패 지점부터의 재개 경로와 배포 로그 kubeconfig·Argo CD 시크릿 배선을 정리하고, kind 규모에 맞는 로컬 리소스 프로파일을 추가.
- **로컬 개발 환경 자동화** (#55): `runbook_local.sh`가 kind 클러스터 등록과 템플릿 시드를 자동 수행.

- **Stack Continue 배포** (`POST /api/v1/stacks/:id/continue`): 실패한 스택 배포를 rollback 없이 재개. 이미 설치된 Helm 릴리즈를 보존하고 실패 지점부터 재시작. `InstallStackInput`에 `Continue`/`PreserveLogs` 필드 추가, 실패 시 UI에 Continue 버튼 노출.
- **Pod Watch WebSocket** (`GET /ws/deployments/:id/pods`): kubectl get pods -n <namespace> -w 출력을 WebSocket으로 실시간 스트리밍. 배포 로그 페이지에 Pod Watch 패널 추가 (네임스페이스, Ready, Status, Restarts, Age 표시).
- **Org Resource Profile 저장** (`/api/v1/admin/org-resource-profiles`): 조직 단위 리소스 프로파일 CRUD. Stack Install Wizard Sizing 탭에서 프로파일 저장·불러오기 드롭다운 지원. DB 마이그레이션 `000049_org_resource_profiles`, `000050_allow_local_resource_profile`.
- **`Orchestrator.IsStepEnabled` 공개 메서드**: `stepEnabledChecker` 인터페이스를 통해 usecase 레이어에서 각 설치 단계 활성화 여부를 조회 가능.

- OpenBao 선택형 배포 경로 구현: `authentication.provider=openbao` 선택 시 `installing_openbao` 단계에서 OpenBao(공식 이미지) Deployment/Service를 생성하고 Gateway 기본 번들에 `openbao.<access_domain>` 라우트를 자동 추가합니다.
- Secret Manager 추상화 계층 추가: `internal/shared/secrets`에 provider 라우터(`Router`)와 OpenBao 구현체(`OpenBaoStore`)를 도입해, 토큰 저장/조회를 provider별 어댑터로 분리했습니다.
- Token source OpenBao 실연동: stack token source 등록 시 `metadata.secret_manager`를 저장하고, Admin `POST /api/v1/admin/token-sources/:id/reveal`가 OpenBao 실조회 값을 우선 반환하도록 확장했습니다.
- 로컬 실행 기본값 보강: `runbook_local.sh`가 `OPENBAO_ADDR`/`OPENBAO_TOKEN` 기본값을 export 하여 로컬에서 OpenBao read/write 경로를 즉시 검증할 수 있도록 개선했습니다.

- Phase A (F8-Phase5 재개) — 매트릭스 CRUD UI + 백엔드: **backend** — `port.CompatibilityRepository` 에 `Create/Update/Delete` 3 메서드 + `ErrCompatibilityMatrixNotFound`/`ErrCompatibilityMatrixExists` sentinel. Memory + Postgres 리포지토리 구현 (멱등 Delete, Create-on-conflict-404, Update updated_at touch). `ManageCompatibility` usecase + `validateMatrixPayload` (id regex / status enum / semver / tools arr ≤32 / tier/arch 화이트리스트). `CompatibilityHandler` 가 `WithManageCompatibility` + `WithCompatibilityAuditSink` 옵션 + `RegisterAdminRoutes` 로 `POST/PUT/DELETE /admin/compatibility/matrices[/:id]` 노출. `mapCompatibilityError` 가 sentinel → HTTP 400/404/409 매핑. 성공 시 audit `compatibility_matrix_{create,update,delete}` 기록 + verdict 캐시 `Clear()`. **frontend** — `MatrixInput` / `matrixInputToPayload` 타입 + `useCreate/Update/DeleteMatrix` 훅. `MatrixEditModal` (identity + k8s 범위 + 동적 tools rows, arch 체크박스 + tier select) + `ConfirmDialog` 재사용 삭제 확인. `stack-versions-page` 헤더에 "New matrix" 버튼 + 상세 패널에 Edit/Delete 버튼. i18n `stackVersionsAdmin.{actions,modal,deleteConfirm}.*` ko/en. **tests** — repo CRUD 6 케이스, usecase 8 케이스 (validation + cache clear), handler 6 케이스 (201/409/400/400-id-mismatch/404/200/204).
- Phase B (F8-Phase3 follow-up) — Retry UI 버튼: 신규 `stack/utils/retry-policy.ts` `canRetry(status)` 순수 헬퍼 (failed/rolled_back만 허용). 신규 `stack/utils/deploy-error.ts` 로 `extractDeployCompatError` 추출 (Install Wizard 와 Retry UI 양쪽 재사용). 신규 `RetryStackButton` 컴포넌트 — `canRetry` 로 자기검열 렌더, `DEPLOY_COMPAT_WARN_UNACK` 응답 시 Modal 이 ack 체크박스 표시, ack 후 재시도 시 `acknowledgeWarnings=true` 전달. `stack-list-page` Info 탭 action row 에 Delete 앞에 배치. i18n `stackList.retry.{button,confirmWarn,toasts}.*` ko/en. `retry-policy.test.ts` 가 10 enum 전체 truthy 매트릭스 검증.
- F8 follow-up 일괄 (Phase 1~7, Phase 5 drop): **Phase 1** — `web/e2e/stack-workflow.spec.ts` / `stack-monitoring.spec.ts` `beforeEach` 의 `waitForURL('**/stack/templates')` 를 `'**/'` 로 보정 (login-page 가 Home 으로 리다이렉트). `@stack-critical` 3/3 그린(새 spec 1 pass + 기존 2 는 내부 data precondition skip). **Phase 2** — `internal/shared/audit/sink.go` + `memsink.go` 도입, `*AuditLogger` / `*MemorySink` 모두 `Sink` 인터페이스 만족. DeployHandler / StackHandler / ClusterHandler / MemberHandler / OrgHandler 5곳의 필드와 variadic 파라미터 타입을 `audit.Sink` 로 narrowing. `TestDeployHandler_Gate_AuditRecordsAckAndVerdict` / `NoAuditOnBlockedWarn` 2 신규 케이스로 `acknowledge_warnings` / `compatibility_verdict` / `issue_codes` 기록 검증. **Phase 3 (F8-Retry-API)** — `POST /api/v1/stacks/:id/retry` + DeployHandler 에 `runPreDeployGate` 공통 헬퍼 추출해 Deploy/Retry 공유. `rolled_back`/`failed` → pending rewind + InstallStack 재실행. `TestRetry_*` 5 케이스 (verified pass / rolled_back pass / completed 409 / warn unack 400 / warn ack 202). 프론트 `retryStack` + `useRetryStack` 훅 추가 (UI 버튼은 follow-up). **Phase 4 (orphan stack / updateStack)** — `UpdateStack` usecase + `PUT /stacks/:id` 엔드포인트. `{pending, failed}` state 에서만 허용(409 STACK_UPDATE_INVALID_STATE), 성공 시 prior config 를 history 에 스냅샷. `WithOptions(WithUpdateStack)` 로 StackHandler 에 주입. 4 단위 테스트. **Phase 5 (Matrix CRUD UI)** — **DROP**. 백엔드 3 엔드포인트 + modal 컴포넌트 스코프가 세션 budget 초과로 별도 작업 분리 (본 프롬프트 §0 "Phase 중간 큰 회귀 시 drop 허용" 규칙 적용). **Phase 6** — `stack/usecase/verdict_cache.go`: `VerdictCache` 인터페이스 + `MemoryVerdictCache` (sync.Map + TTL + prefix invalidation). `WithVerdictCache` 옵션으로 `ValidateCompatibility` 에 주입. `VerdictCacheKey` 가 StackID/ClusterID/NodeArchitectures/Tools 를 정렬된 SHA256 으로 절충 — map 순서 독립. 5 단위 테스트. `VERDICT_CACHE_TTL_SEC` env override. **Phase 7** — `admin/scheduler/refresh_discovery.go`: `RefreshDiscoveryScheduler` 가 interval 마다 모든 cluster 의 `RefreshDiscovery` 를 sweep. `atomic.Bool` in-flight guard 로 overlap 방지, context cancel 시 graceful stop. main.go 에서 signal handler 가 schedulerCancel() 호출. 4 fake-clock/goroutine 테스트 (first sweep / failure isolation / ctx cancel / overlap skip). `REFRESH_DISCOVERY_INTERVAL` env override (기본 24h).
- Warn-forced Retry/Rollback 통합 E2E 검증 (F8 Task 7): `e2e/warn_forced_retry_rollback_test.go` (`//go:build e2e`) 신규 — 독립 `httptest` 서버 + in-memory 리포지토리 + `fakeStepExecutor` (atomic.Bool 로 성공/실패 주입) + `warnClusterReader` (arm64-only 시뮬레이션) 로 6종 subtest 실행. **A** persisted-mode validate 가 `warn` + `TOOL_ARCH_UNSUPPORTED` 반환. **B** ack 없는 deploy 는 400 `DEPLOY_COMPAT_WARN_UNACK` 로 차단되고 스택 state 는 `pending` 유지. **C** `acknowledge_warnings=true` + 정상 executor → 202 + `completed` 도달. **D** ack=true + executor 실패 → 202 수락 후 handleFailure 경로를 타고 `rolled_back` 으로 종료. **E** 실패 후 `rolled_back → pending` 테스트 레벨 rewind + executor 복구 → 재배포 → `completed` (state-machine 계약 검증; 프로덕션 `POST /stacks/:id/retry` 엔드포인트는 plan §6 follow-up 으로 분리). **F** rollback 엔드포인트 (`POST /stacks/:id/rollback`) 가 prior 버전 config 를 복원하고 새 history row 를 append 함. Playwright `web/e2e/stack-warn-forced-retry.spec.ts` (`@stack-critical`) UI 스모크도 추가 — `/admin/stack-versions` 페이지가 warn-prone 매트릭스(`github-argocd-v1`, untested)를 노출하고, `/stack/install` 의 Golden Path Quick Start 카드가 동일 매트릭스를 렌더하는지 계약 검증. 실제 deploy→terminal-state UI 구동은 Kind 클러스터 의존성 때문에 F8 Task 6 / F8-F6-Cloud 에 위임.
- Golden Path 로컬 Kind 배포 검증 스캐폴딩 (F8 Task 6): `e2e/golden_path_kind_test.go` (`//go:build e2e`) 신규 추가. 단일 테스트 `TestF8Task6_GoldenPath_KindDeploy` 가 `discoverKindCluster` 로 `nullus-platform` 클러스터를 발견하면 Narwhal pin 기반 3 종 Golden Path 매트릭스 (`github-argocd-v1`, `gitlab-argocd-v1`, `gitlab-allinone-v1`) 를 순차 subtest 로 실행 — 각각 in-memory `StackRepository` / `TemplateRepository` + 실 `helm.Orchestrator` + `InstallStack.Execute` 경로로 배포 → 상태 폴링 → `completed` 도달 여부 검증. Kind 또는 helm CLI 미설치 시 graceful skip. 테스트별 namespace (`nullus-e2e-<template>-<ts>`), `toolOverrides` 로 monitoring/logging 등을 disabled 처리해 리소스 경합 감소. 실패 시 `dumpKindDiagnostics` 가 stack config / `kubectl get pods -o wide` / `kubectl get events --sort-by=.lastTimestamp` 덤프. 신규 `Makefile` 타겟 `test-golden-path` + `docs/20_아키텍처/F8_Task6_Kind_Runbook.md` 런북. 병렬 필수 조건이었던 `MemoryPipelineRepository.Delete` 누락 (F8 Task 7 precondition) 을 동시 해결 — e2e 빌드가 clean 하게 성공. **로컬 검증 결과**: `github-argocd-v1` 실제 통과 (4분 5초, cert-manager/metrics-server/PostgreSQL/MinIO/Argo CD 실 helm 설치), `gitlab-argocd-v1` 은 단일 노드 Kind + 15분 timeout 에서 미도달 (GitLab 풀스택 리소스 부족) — 런북에 "subtest 사이 Kind 재생성 필요 (cluster-scoped cert-manager CRD/ClusterRole leak)" 관측과 권장 운영 스크립트를 문서화. EKS/GKE 검증은 `F8-F6-Cloud` follow-up 으로 분리 유지.
- Deploy 단계 서버측 Pre-Deploy Gate (F8-F3): `POST /api/v1/stacks/:id/deploy` 가 `InstallStack.Execute` 호출 전에 `ValidateCompatibility` 를 persisted mode 로 재실행한다. `fail` → `DEPLOY_COMPAT_FAIL` 400 하드 블록, `warn` 은 body 의 `acknowledge_warnings=true` 가 없으면 `DEPLOY_COMPAT_WARN_UNACK` 400 으로 블록. 두 오류 모두 응답 body 에 `verdict.overall` / `verdict.issues` / `verdict.node_architectures` / `verdict.matrix` / `verdict.checkedAt` 를 포함해 프론트가 기존 Pre-Deploy Gate UI 로 그대로 렌더링할 수 있다. `ValidateCompatibilityInput.StackID` 필드 추가 + `WithStackRepository` 옵션: tools 가 비어 있고 StackID 만 주어지면 use case 가 stack 을 로드해 tools/clusterID 를 파생, 이를 통해 UI 를 우회한 직접 API 호출도 차단된다. `POST /stacks/:stackId/validate` 가 path 의 `:stackId` 를 body 가 생략했을 때 자동 보충해 persisted mode 로 동작. 프론트 Install Wizard 의 submit 플로우 재편 (`createStack → validateCompatibility → deployStack`): 서버 verdict 이 `fail` 이면 `DEPLOY_COMPAT_FAIL` UI, `warn` 이면 전용 ack 체크박스 (`server-warn-ack`) 를 띄우고 체크 후 재제출 시 `deployStack({ acknowledgeWarnings: true })` 로 진행. `useValidateCompatibility` 훅 재사용, `useDeployStack` 가 `{ stackId, acknowledgeWarnings }` 객체 입력 + legacy string 동시 지원. 신규 순수 util `shouldBlockOnServerVerdict` (`stack/utils/server-verdict.ts`) 가 verdict → `{ block, mode, acknowledgeWarnings? }` 결정을 내리며 3 단위 테스트로 정책 고정. 백엔드 테스트 `deploy_handler_compat_test.go` 5 케이스 (pass / fail / warn-unack / warn-ack / CLUSTER_ARCH_UNKNOWN ack 분기) + `validate_compatibility_test.go` persisted mode 4 케이스 (tools 로딩, stack 부재 에러, clusterID fallback 시 arch 체크, explicit tools override) 추가. `AcknowledgeWarnings` 는 opt-in 이므로 기존 클라이언트가 body 없이 호출하면 자동 `false` 로 해석되어 warn 조합이 기본 차단 — 의도된 보안 강화.
- Stack Install Wizard Auto Select + 노드 아키텍처 게이트 (F8 Task 5): `stack-install-page.tsx`의 Pre-Deploy Compatibility Gate 영역 위에 Golden Path 빠른 시작 카드 3종을 추가. 각 카드는 `compatibilityMatrixData`를 읽어 `isMatrixCompatibleWithCluster`로 현재 `draft.clusterId` 클러스터의 `nodeArchitectures`와 교차 검증한다 — incompatible 시 버튼 disabled + 툴팁으로 누락 아키텍처 명시, unknown 시 경고색 강조, 클러스터 미선택 시 안내 메시지. 클릭 시 `loadFromTemplate(matrix.id)`로 draft 주입. Gate의 기존 verified/untested/unsupported verdict 위에 `TOOL_ARCH_UNSUPPORTED` / `CLUSTER_ARCH_UNKNOWN` 이슈를 레이어링 (verified+arch miss → `fail`, untested+arch miss → `warn` 유지). 신규 순수 함수 `isMatrixCompatibleWithCluster` / `matrixArchMismatches` (+10 단위 테스트) 를 `stack/utils/compatibility-arch.ts`에 배치. i18n `stackInstall.compatibility.autoSelect.*` / `issue.*` 네임스페이스 신설 (ko/en).
- Admin Stack Version Management 페이지 (F8 Task 4): `/admin/stack-versions` (admin 전용) 신규 페이지 추가. 좌측 Golden Path 3종 목록(verified/untested/unsupported 배지), 우측 상세 — Kubernetes 범위 + tools 테이블 (arch/tier badges 포함, F8 Task 1 필드 노출) + clusters 섹션에 각 클러스터의 `node_architectures` 표시 + 매트릭스 교차 평가 (✓/✗/Unknown) + 행별 Refresh Discovery 버튼 (`useRefreshDiscovery` 호출 후 `useClusters` 캐시 invalidate). 공유 타입 확장: `CompatibilityTool.archSupport` / `minK8sVersion` / `tier`, `Cluster.nodeArchitectures`, `CompatibilityValidationResult.nodeArchitectures` / `matrix` / `message`. `stack-api.ts`의 `normalizeCompatibilityTool` 이 snake/Camel/Pascal 세 키 모두 수용. `admin-api.ts` 에 `refreshClusterDiscovery` + `useRefreshDiscovery`. `stack-api.ts validateCompatibility` 가 `{ clusterId, nodeArchitectures, tools }` 입력과 레거시 string stackId 모두 지원. 사이드바 `데브섹옵스 스택` 그룹에 `stackVersionsAdmin` 항목 추가 (role=admin).
- Compatibility Matrix 클러스터 노드 아키텍처 검증 (F8 Task 3): 마이그레이션 `000043_cluster_node_architectures`에서 `clusters.node_architectures TEXT[]` 컬럼을 추가하고, admin 모듈의 `kube.DiscoverCluster`가 `node.status.nodeInfo.architecture`를 수집해 sorted+deduped 셋으로 저장. `ClusterUseCase.RefreshDiscovery` 경로(신규 등록/업데이트/`/clusters/:id/refresh-discovery`)가 클러스터 실제 상태를 기록하며, 실패 시 `connection_status=connection_failed` + 빈 슬라이스로 축약. Stack 모듈의 Pre-Deploy Gate(`ValidateCompatibility`)는 신규 `port.ClusterReader`를 통해 `cluster_id` 또는 explicit `node_architectures` 입력을 받아 `ToolVersion.SupportsArch`를 교차 검증 — verified 매트릭스에서 아키 miss 시 `fail`(하드 블록), untested 매트릭스에서는 `warn` 유지, 아키 미상 시 `CLUSTER_ARCH_UNKNOWN` 경고. 신규 단위/통합 테스트 6종(kube fake clientset, usecase discovery 성공/실패, memory cluster round-trip, Pre-Deploy Gate 4종 시나리오, postgres node_architectures round-trip) 추가. Admin↔Stack 간 바운디드 컨텍스트 경계는 CI/CD의 `StackReader` 패턴과 동일하게 `ClusterReader` 인터페이스로 격리.
- Compatibility Matrix Narwhal baseline 재확정 (F8 Task 2): 마이그레이션 `000042_seed_narwhal_compat_refresh`에서 Golden Path 3종 조합(`gitlab-allinone-v1`, `gitlab-argocd-v1`, `github-argocd-v1`)의 `compatibility_matrices.tools` 및 `golden_path_templates.tools`를 Narwhal VERSIONS.md 기반 canonical baseline v1으로 재확정. GitLab CE/CI/Registry `9.5.1/18.5.1`, Harbor `1.15.0/2.11.0`, MinIO `5.2.0/2024-08-03`, Argo CD `6.8.0/v2.8.3`, Prometheus `67.0.0/v2.54.1`, Grafana `8.5.0/11.1.0`로 pin. `MemoryCompatibilityRepository`는 `narwhal*` 상수 블록으로 동일 값을 공유하고, `docs/20_아키텍처/Narwhal_호환성_Seed_Sources.md`에 각 버전의 출처와 업데이트 규칙을 문서화. `TestMemoryCompatibilityRepository_NarwhalBaselineVersions`가 세 계층의 drift를 차단.
- Compatibility Matrix 스키마 세분화 (F8 Task 1): `ToolVersion`에 `MinK8sVersion`, `ArchSupport`, `Tier` 필드 추가. 마이그레이션 `000041_compat_tool_fields`에서 기존 3종 시드(gitlab-allinone-v1, gitlab-argocd-v1, github-argocd-v1)의 tools JSONB에 idempotent 하게 값을 패치 (Harbor/GitLab 계열은 amd64-only, 그 외 amd64+arm64). `ToolVersion.SupportsArch()` / `EffectiveMinK8sVersion()` 헬퍼로 Pre-Deploy Gate ARM64 체크 및 per-tool K8s 버전 검증의 기반을 제공.
- Cluster 모니터링 실집계 API 추가: `GET /api/v1/admin/clusters/:id/monitoring-summary` (kubeconfig 기반 전체 Pod/Ready Pod 및 CPU/Memory request/limit 요약)
- CI/CD 파이프라인 삭제 API 및 UI 추가: `DELETE /api/v1/cicd/pipelines/:id`, CI/CD List 상세 패널 `Delete` 버튼
- Stack Template → Install 오버라이드 공통 유틸 추가 (`web/src/features/stack/utils/template-overrides.ts`)
- CI/CD History 샘플 데이터 보강 마이그레이션 추가: `000040_seed_ml_service_history` (ML Prediction Service 배포 이력 10건)
- `runbook_local.sh`에 `refresh` 커맨드 추가 — 마이그레이션 포함 백엔드 + 프론트엔드 재빌드/재시작
- Home CTA 권한 상태와 Roadmap 연동 개선 — 로그인 사용자의 역할/권한 기반 CTA 동적 표시 및 Roadmap 페이지 연동
- 풀 CI/CD 빌드 파이프라인: Git Clone → Docker Build → Kind Load → K8s Deploy 6단계 자동화
- `ImagePreparer` Port + `docker/builder.go` 어댑터: git clone, docker build, kind load docker-image 실행
- `ClusterTargetProvider` Port: 클러스터 이름 + kubeconfig 통합 조회 (Kind 클러스터명 자동 추출)
- Pipeline에 `dockerfile_path`, `docker_context` 필드 추가 — Dockerfile 경로와 빌드 컨텍스트 지정
- Pipeline에 `env_vars` 필드 추가 — 환경변수를 K8s Deployment 매니페스트 container spec에 반영
- CI/CD 템플릿에 빌드 설정 지원: `git_repo_url`, `dockerfile_path`, `docker_context`, `env_vars` 필드 추가
- Nullus Sample App 배포 템플릿 2종 추가 (`nullus-sample-backend-v1`, `nullus-sample-frontend-v1`)
- Deploy 위저드 Step 2에 "Build Configuration (Optional)" 섹션: Dockerfile Path, Docker Build Context 입력
- Deploy 위저드 상단에 "Quick Start — Select a Template" UI: 템플릿 클릭 시 폼 자동 채움 + Step 3으로 점프
- 환경변수(Step 5)가 파이프라인 생성 시 저장되어 배포 시 K8s 매니페스트에 반영
- 템플릿 선택 시 기본 환경변수 자동 상속 (예: 프론트엔드 템플릿의 `BACKEND_HOST=sample-backend:8080`)
- `scripts/register-kind-clusters.sh`: nullus-platform, nullus-develop Kind 클러스터 자동 등록 스크립트
- Stack ↔ CI/CD 교차 컨텍스트 검증: StackReader Port 인터페이스를 통해 CI/CD 모듈이 Stack 도메인을 직접 import하지 않고 Stack 존재/조직 일치/상태를 검증합니다 (Direction B)
- `POST /cicd/pipelines` 요청 시 `stack_id`를 지정하면 Stack 존재 여부, 조직 일치, 배포 상태를 자동 검증합니다
- `GET /stacks/:stackId/pipelines` 엔드포인트 추가 (Stack 기준 Pipeline 조회)
- `GET /cicd/pipelines?stack_id=xxx` Stack 필터 지원
- Stack 배포 로그 DB 영속화(`deployment_logs`) 및 `PostgresStreamer`를 추가해 API 재시작/재구독 이후에도 로그 replay가 가능해졌습니다.
- Stack History에 Cluster 컬럼/클러스터 이름 필터/Log 바로가기 버튼을 추가해 최근 배포 로그 접근성을 개선했습니다.
- Stack Version 검증 응답에 `overall`/`issues`/`checkedAt`를 포함해 pass/warn/fail 기반 호환성 피드백을 제공하도록 확장했습니다.
- v0.1 아키텍처 설계와 실제 구현 코드를 비교 분석한 v0.2 아키텍처 문서 추가 (`docs/20_아키텍처/Nullus 상세 기능 명세 및 시스템 아키텍처_v0.2_claude.md`)
  - 설계-구현 차이 분석표 (아키텍처 변경, 기능 상태, 미구현 항목)
  - Clean Architecture + DDD 기반 실제 코드 구조 문서화
  - 5개 Bounded Context별 도메인 모델, API, 상태 머신 상세
  - 3-Phase Helm DAG Orchestrator 구현 기준 명세
  - 전체 API 엔드포인트 목록 (v0.1 대비 경로 변경 추적)
  - 데이터 모델 ERD (구현 기준 15+ 테이블)
  - 보안 아키텍처 (AES-256-GCM, Dual Auth, RBAC) 상세
  - ADR 16건 (v0.1 10건 + 신규 6건)
  - 로드맵 (v0.2-alpha → v0.2-beta → v1.0 GA)
- Nullus 설계 대비 미구현 항목만 정리한 문서 추가 (`docs/20_아키텍처/Nullus_설계_대비_미구현_항목.md`)
- 현재 `draft` 구현 기준 As-Is 아키텍처 다이어그램 문서 추가 (`docs/20_아키텍처/Nullus_As-Is_아키텍처_다이어그램.md`)
- 기존 v0.1 설계 문서를 현재 구현 기준으로 재구성한 `Nullus 상세 기능 명세 및 시스템 아키텍처_v0.2.md` 추가
- Alert Rules edit modal now loads the latest rule payload directly from the database through `GET /observability/alert-rules/:id` before editing.
- Stack Install supports leaving Storage unselected for Empty Template flows by omitting the storage block from create requests when no storage plan is chosen.
- CI/CD List에 클러스터 필터 드롭다운 추가 (`useClusters` 훅 연동)
- Pipeline Logs 전용 페이지 신규 생성 (`/cicd/pipelines/:id/logs`, 터미널 콘솔 + 배포 이력 뷰)
- CI/CD 배포 진행 UI를 WebSocket 기반 실시간 스트리밍으로 전환 (Stack Deploy 페이지 스타일)
- WebSocket 핸들러 `/ws/cicd/deployments/:id/logs` 추가 (`StepTracker` pub/sub 패턴)
- `useCicdDeployLog` 프론트엔드 훅 (WebSocket 연결, 로그/진행률/상태 관리)
- Deploy 위저드 Step 6 매니페스트 편집 단계 추가 (textarea로 YAML 수정 가능, 기본값 초기화)
- Deploy 위저드 Step 2에 Stack Git 서비스 URL 자동 연동 (Stack 선택 시 base URL + repo 이름 분리 입력)
- Deploy 위저드 Step 3 네임스페이스를 K8s API에서 실제 조회 (`useClusterNamespaces` 훅)
- Deploy 위저드 Step 4 리소스 설정에 슬라이더 + Input 동시 지원 (커스텀 값 직접 입력 가능)
- RUN 버튼이 pipeline 정보(clusterId, namespace, appName)를 Deploy 위저드에 프리필
- 모든 CI/CD 페이지에 Breadcrumb 상위 네비게이션 추가 (뒤로가기 지원)
- CI/CD 파이프라인이 kind 클러스터에 실제 K8s 리소스(Deployment, Service, Namespace)를 생성
- 배포 진행 화면에 Deploy Output 터미널 박스 (kubectl 명령어 및 결과 실시간 표시, 색상 구분)
- 배포 완료 시 생성된 K8s 리소스 목록 표시 및 `kubectl get` 확인 명령어 복사 기능 (`--context` 포함)
- `DeployStep`에 `Logs` 필드 추가, `StepTracker.AppendLog`로 스텝별 kubectl 로그 축적
- `StepTracker`에 `Subscribe`/`Unsubscribe`/`publish` 메서드 추가 (WebSocket 실시간 이벤트 전파)
- 인메모리 `StepTracker`로 배포 단계별 진행 상태 추적 (30초 후 자동 정리)
- GET `/cicd/deployments/:id` 엔드포인트 (배포 상태 + 스텝 로그 병합)
- CI/CD List 상세 패널 4개 탭: Info (Pipeline + Target + Stages + Variables), Monitoring, History, Actions
- `DataTable`에 `renderExpanded` prop 추가 (행 아래 인라인 상세 패널)
- CI/CD History 페이지에서 특정 파이프라인 배포 이력만 필터링 (`?pipeline=<id>`)
- 배포 시 현재 로그인 사용자가 `deployed_by`로 자동 기록
- Helm 차트 ServiceAccount 템플릿 추가
- API Deployment에 wait-for-db initContainer 추가 (PostgreSQL 준비 대기)
- CI/CD kind 클러스터 배포 시연 가이드 (`docs/guides/cicd-pipeline-kind-deploy-guide.md`)
- 시행착오 및 해결 방법 레퍼런스 (`docs/agent-reference.md`)
- Pipeline 타입에 `dockerfilePath`, `dockerContext`, `envVars` 필드 추가 — 프론트엔드에서 백엔드 빌드 설정 데이터를 표시
- CI/CD List 상세 Info 탭에 "Build Configuration" 카드 추가 (Dockerfile, Build Context 표시)
- CI/CD List 상세 Info 탭에 실제 환경변수 표시 (하드코딩 3개 → 백엔드 `env_vars` 기반, 마스킹 토글)
- CI/CD List 상세 Monitoring/History 탭에 로딩 상태 표시 추가

### Changed

- **설치 단계 실행 방식을 DAG 의존성 기반으로 전환** (#61): 고정 phase 루프 대신 단계 간 의존성을 따라 실행하도록 변경. OpenBao는 PostgreSQL/MinIO 이후·핵심 서비스 직전으로 정렬.
- **예시 시드 데이터 제거** (#82): 예시용 `nullus-devsecops-stack` 기본값을 제거하고, kind 클러스터 등록이 실제 endpoint를 사용하도록 수정.
- **스택 툴 아이콘·분류 표시 정리** (#73): 툴 아이콘을 로컬 SVG로 교체하고 템플릿 툴을 카테고리별 색상으로 구분.
- **배포 로그 페이지 UI 개선**: 타임라인 스텝, 세그먼트 프로그레스 바, Raw Logs 콘솔, Attention 패널(warn/error 필터), Pod Watch 패널로 구성한 새 레이아웃. WS 연결 전 "Connecting..." / 연결 후 파드 없음 "No pods in namespace yet." 으로 상태 구분.
- **Status API `namespace` 필드**: `omitempty` 제거 — 스택 네임스페이스가 빈 값이어도 항상 필드 포함해 반환.
- **`podNamespace` 폴백 처리**: `??` → `||` 변경으로 빈 문자열까지 폴백 처리.
- **Stack Install 페이지**: Quick Start 카드 및 Kubernetes Preview 섹션 제거.
- Monitoring Dashboard Cluster 뷰를 선택 클러스터 기준으로 재구성: Stack 모니터링 합산 + Stack 매핑이 없을 때 클러스터 실집계 자동 fallback
- Monitoring Dashboard의 CI/CD 탭 표시 정책 변경: 탭은 항상 노출하고, 클러스터 타입이 `target`이 아닐 때 비활성화
- CI/CD List 레이아웃을 Stack List 패턴으로 통일: 하단 확장 행 제거, 좌측 목록 + 우측 상세 패널(모바일은 하단)
- CI/CD List 상세 탭 구조 단순화: `Actions` 탭 제거, 주요 액션은 상세 헤더 버튼으로 통합
- CI/CD Pipeline Setup/Developer Deploy의 클러스터 선택을 Target Cluster 타입으로 제한
- Developer Deploy 네임스페이스 UX 개선: `default` 기본 제공 + `New Namespace` 직접 입력 지원
- Pipeline Logs 화면 상태 문구 개선: 배포 종료 후 로그 없음 상태를 명확히 안내
- Kind 로컬 클러스터 구성 조정: `scripts/kind-cluster.yaml`에 worker 노드 1대 추가
- 로그인 후 기본 진입 경로를 Home으로 통일 (모든 역할에서 로그인 완료 시 Home 페이지로 리다이렉트)
- 매니페스트 생성기: `ImageRef` 필드 추가 — 설정 시 템플릿 하드코딩 이미지 대신 빌드된 이미지 사용
- `ManifestApplier.ApplyWithTracking`에 `stepOffset` variadic 파라미터 추가 (빌드 단계 이후 인덱스 보정)
- `NewDeployPipeline`에 옵셔널 DI 패턴 도입 (`WithImagePreparer`, `WithClusterTargetProvider`)
- StepTracker 클린업 타이머 30초 → 5분 (빌드 시간 고려)
- Playwright E2E 테스트: Korean 셀렉터 → English 셀렉터 전환 (기본 언어 en)
- Rollback 테스트 제거 (CI/CD History에서 Rollback 기능 제거됨)
- Stack 템플릿 카운트 3 → 4 (github-argocd-v1 추가 반영)
- `POST /cicd/pipelines` 응답 포맷을 `{"pipeline": {...}, "warning": "..."}` 구조로 변경 (warning은 Stack 미완료 시 optional 포함)
- `PipelineRepository.List` 시그니처에 `stackID ...string` variadic 파라미터 추가
- Stack History 라우팅 동작을 조정해 설치 직후 목록 캐시가 늦게 갱신되더라도 URL의 `stackId`를 우선 유지하도록 변경했습니다.
- Stack Deploy 화면의 상태 계산 로직을 개선해 WS 연결 여부만으로 `running`으로 오인하지 않고 API의 최종 상태를 우선 반영하도록 변경했습니다.
- 템플릿 생성/수정 화면의 OSS 분류를 스택 설치 분류 체계와 일치시키고 모달 폭/ID 입력 UX를 개선했습니다.
- Stack create request mapping now translates UI storage modes (`existing-all`, `existing`) to the backend storage contract (`existing-connect`) before submission.
- Deploy 위저드를 5단계에서 6단계로 재구성 (앱 이름 → Git → 클러스터 → 리소스 → 환경변수 → 매니페스트 확인)
- 앱 템플릿 그리드 제거, CI/CD Template의 `app_type`으로 앱 타입 자동 결정
- CI/CD 배포 진행 UI를 polling 방식에서 WebSocket 실시간 스트리밍으로 전환
- CI/CD List Logs 버튼이 Pipeline Logs 전용 페이지(`/cicd/pipelines/:id/logs`)로 이동
- `DeployPipeline` usecase를 `Start`(동기, DB 저장) + `ApplyAsync`(비동기, K8s 배포)로 분리, HTTP 202 즉시 반환
- `ManifestApplier.ApplyWithTracking`이 각 매니페스트 적용 결과와 로그를 `StepTracker`에 기록
- CI/CD List/History 페이지가 각 항목 아래 인라인 상세 패널로 변경 (하단 패널 → 행 아래 인라인)
- CI/CD History에서 Rollback 기능 전체 제거 (백엔드 미구현)
- CI/CD List/History 페이지가 백엔드 API 응답을 정확히 매핑 (앱 타입, 클러스터명, 상태, 배포일)
- CI/CD List 테이블에서 Deploy 버튼 제거 (상세 패널의 Run으로 통합)
- `go-web-api` 템플릿 이미지를 빌드 이미지에서 런타임 서버로 변경 (`nginx:alpine`)
- Migration Job을 pre-install Hook에서 외부 마이그레이션 패턴으로 전환
- Cluster/CI/CD 모니터링 뷰의 목업 데이터를 실제 API 데이터로 교체 (`useDashboard()` 폴링 축적, `usePipelines()`+`useDeployments()` 연동)
- `useScopedClusters()` 훅을 `admin-api.ts`로 통합하고 `stack-api.ts` 중복 정의 제거
- CI/CD List 상세 History 탭 배포 소요 시간 포맷 개선 (`42s` → `1m 42s`)

### Fixed

- **차트 기본 이미지가 존재하지 않는 경로를 가리키던 문제** (#100): 저장소 리네임 이후 `values.yaml`의 기본값이 `ghcr.io/cloud-nullus/nullus-api`(세그먼트 부족) + ghcr에 없는 태그 `0.1.0-alpha`를 가리켜, 차트만으로는 설치가 불가능했다. 실재 경로 `ghcr.io/cloud-nullus/nullus/nullus-*`로 교정하고 `tag`를 비워 `Chart.appVersion` 폴백이 동작하도록 변경 — 릴리즈 시 동기화할 지점이 `Chart.yaml` 한 곳으로 줄었다.
- **스택 설치 기본 클러스터 선택** (#97): 클러스터 이름 하드코딩(`kind-nullus-platform`) 폴백에 의존해, name/type 데이터가 어긋나면 의도와 다른 클러스터가 선택될 수 있었다. `type`/`types` 필드만으로 판단하는 `findPlatformCluster`로 통일.
- **로그인 후 로그인 화면으로 튕기던 문제** (#78): 저장소가 `cloud-nullus/draft` → `cloud-nullus/nullus`로 리네임된 뒤에도 에어갭 설정이 옛 ghcr 경로를 참조해, 리네임 직전에 고정된 구버전 `nullus-web` 이미지가 설치되고 있었다. 해당 번들에는 세션 인증 헤더 전송 수정이 빠져 있어 API가 401을 반환하고 프론트가 로그아웃 처리했다. 이미지 경로를 새 경로로 교정.
- **GitHub 선택 시 GitLab이 함께 설치되던 문제** (#56): `installing_gitlab`·`installing_runner` 실행 조건을 도구 이름 기준으로 제한.
- **GitLab CE 표기 시 GitLab 설치 단계를 건너뛰던 문제** (#85): 템플릿 상세 모달의 버전 표기도 매트릭스 스냅샷 기준으로 정정.
- **OpenBao 설치 순서·헬스 게이트 정합화** (#57, #61, #88): OpenBao 단계를 Phase A 초반으로 배치하고 미선택 시 자동 비활성화. 토큰 소스는 설치 시점과 별도 동기화 경로에서 공통 생성하도록 정리.
- **kind 클러스터 재생성 후 설치 실패** (#62): kubeconfig 조회 시 endpoint drift를 자동 동기화해 cert-manager 단계에서 실패하던 문제를 완화.
- **otel-collector 이미지 pull 실패** (#63): 실재하지 않는 image:tag 조합을 만들던 override를 제거하고 차트 기본값을 사용 (#59 회귀).
- **클러스터 모니터링 Pod 목록 모달 복구** (#67): Pod Status 카드에서 파드 런타임 상태를 바로 확인하는 흐름을 원복.
- **카카오클라우드 배포 정합성·이식성·스크립트 버그** (#86): #79에 대한 리뷰 지적 19건을 트리아지해 검증된 항목을 반영.
- **Stack List 회귀** (#60): 머지 후 이전 버전 목록이 표시되던 문제 수정.
- **Sizing Profile 드롭다운 즉시 반영**: 프로파일 저장 후 드롭다운에 즉시 반영되지 않는 버그 수정 (캐시 invalidation 누락).
- **`usePodWatch` 재연결 시 에러 초기화**: WS 재연결 성공 시 이전 연결의 stale 에러 메시지가 남는 문제 수정.
- 스택 재시도 안정화: cert-manager 네임스페이스/CRD ownership 감지, startupapicheck job optional 처리, GitLab rollout timeout 확장, rollback 잔존 리소스 정리를 반영해 rolled_back 재배포 경로를 안정화했습니다.
- 설치 단계 정합성 수정: `installing_openbao` 단계가 Orchestrator/UseCase 순서와 일치하도록 정렬해 `integration_check out-of-order` 실패를 수정했습니다.
- Stack 상세의 Gateway PF Copy 개선: 스택별 `GATEWAY_NAME`을 포함한 명령으로 잘못된 gateway service 선택을 방지하고, 접속정보 복사본에 Primary URLs + Gateway Port-Forward 섹션을 추가했습니다.
- 포트포워딩 스크립트 안전성 보강: 컨텍스트 폴백 메시지 정정 및 선택된 gateway service에 443 포트가 없을 때 HTTP-only로 자동 폴백하도록 수정했습니다.

- Stack 모니터링 OSS 상태 계산에서 one-shot 완료 Job Pod(`*-migrations*`, `*-job*`)를 제외해 GitLab migration 완료 Pod로 인한 `warning` 오탐을 방지
- Developer Deploy 진행 화면의 Created Resources 중복 노출 이슈를 리소스 dedupe 로직으로 보정
- CI/CD Deployment 상세 파싱 보강: step 로그가 없는 응답 형식도 fallback 로그로 표시
- Use Base Template 진입 시 선택하지 않은 리소스가 자동 선택되던 문제를 수정해 실제 템플릿 선택 상태가 그대로 반영되도록 했습니다.
- Stack List 상태/클러스터 표시 정합성을 수정해 `connected + completed` 케이스가 `Running`으로 노출되고 모니터링 탭 조건이 일관되게 동작하도록 보정했습니다.
- Stack Compatibility 검증 시 스택이 실제 배포된 클러스터의 Kubernetes 버전을 기준으로 평가되도록 수정했습니다.
- Alert Rules edits now reflect immediately after Save by awaiting the update mutation, refetching the DB-backed list, and reopening the modal with fresh server data.
- Empty Template에서 Observability만 선택해도 `storage.plan_mode` 검증 오류가 나지 않도록 storage payload 생성 조건을 수정했습니다.
- Organization 화면의 `Add User` 버튼이 존재하지 않는 `/admin/user-management` 대신 실제 라우트인 `/admin/users`로 이동하도록 수정했습니다.
- Stack gateway deploy now skips `BackendTLSPolicy` manifests when the cluster does not provide the `BackendTLSPolicy` CRD, so Gateway and HTTPRoute resources can still be applied.
- Breadcrumb에서 동일한 key `/cicd/list`가 2회 사용되어 React 경고 발생하던 문제
- Dockerfile Go 버전이 `go.mod`와 불일치 (`1.24` → `1.26`)
- `web/Dockerfile` 빌드 컨텍스트 경로 오류 (`web/nginx.conf` → `nginx.conf`)
- `web/Dockerfile` npm ci peer dependency 충돌 (`--legacy-peer-deps` 추가)
- API Deployment에 ConfigMap 볼륨 마운트 누락으로 config 파일을 찾지 못하던 문제
- `getPipelineStatusLabel`의 커스텀 `Translate` 타입을 i18next `TFunction`으로 변경하여 CI/CD 3개 페이지 9건의 TypeScript 에러 해소
- Stack 페이지에서 클러스터 `connection_status` 필드 접근 오류 (`Cluster` 타입 통합 후 `status`로 수정)
- 클러스터 API 매핑과 스택 템플릿/설치 동작 정합성 수정 (클러스터 필드 매핑, 스택 템플릿 카운트, 설치 페이지 선택 동작 보정)
- 관리 화면 유효성 검증 흐름과 다국어 표시 개선 (Cluster/CI-CD/Stack/OSS 리소스 페이지 ko/en 번역 보완)
- 스택 템플릿 설명 로케일 오버라이드 보강 — 템플릿별 언어 설명 오버라이드 로직 추가
- CI/CD 템플릿 설명 다국어 처리 및 우선순위 정렬 개선
- 클러스터 등록/수정 검증 흐름 통합 — 백엔드 핸들러와 프론트엔드 클러스터 페이지의 검증 로직 단일화
- Register Cluster 다이얼로그의 클러스터 타입 옵션 순서 조정
- Stack List 삭제 후 목록 즉시 반영 및 상세 패널 중복 표시 수정

### Security

- **OSS SSO 자동로그인** (#98): Keycloak을 IdP로 포털(nullus-web)과 OSS 스택 앱(Argo CD·Grafana·Harbor·MinIO·GitLab·Prometheus·OpenSearch)을 단일 SSO로 묶었다. 포털 OIDC 로그인·로그아웃(end-session), OSS 앱 confidential client 자동 프로비저닝, OIDC 미지원 앱(Prometheus·OpenSearch)의 oauth2-proxy 우회를 포함. 브라우저 `crypto.subtle`이 secure context를 요구해 PKCE의 전제가 되므로 게이트웨이 HTTP 80 → HTTPS 443 강제 리다이렉트를 함께 배선했다. #93·#95의 provider 추상화와는 층위가 다르다 — 그쪽은 IdP 기동·주입, 이쪽은 기동된 IdP에 앱을 물리는 부분이다.
- **OIDC를 설치 옵션으로 분리** (#93): IdP를 플랫폼 상시 기능이 아니라 runbook 선택 옵션(`--auth=<keycloak|authentik|none>`, 기본 `keycloak`)으로 정리하고, provider별 OIDC 환경변수를 API·웹에 주입하도록 배선. 기본값이 `keycloak`이므로 기존 동작은 보존된다. provider 선정 근거는 `docs/20_개발가이드/OIDC_Provider_선정기준.md` 참조.

### Known Issues

alpha 단계이므로 아래 결함을 안고 릴리즈합니다 (릴리즈 정책 §9.0-3). `ci.yml`이 `disabled_manually` 상태라 자동 게이트가 없고, 2026-07-28 릴리즈 담당 로컬 검증에서 확인한 값입니다.

- **Go e2e 테스트 2건 실패**: `TestScenario4_CICDPipelineFlow`, `TestUAT2_Jieun_Developer`. 나머지 32개 패키지와 `make build`는 통과.
- **프론트엔드 단위 테스트 38건 실패** (9파일 / 490건 중). 타입체크(`tsc --noEmit`)는 통과.
- 위 실패는 이번 릴리즈에서 새로 생긴 것이 아니라 기존 결함이며, `ci.yml` 재활성화와 함께 수정합니다 (정책 §13-2).
- **`0.1.0-alpha`·`0.2.0-alpha`에는 태그·Release가 없습니다.** 해당 섹션은 기록상의 버전이므로 compare 링크도 걸리지 않습니다 (정책 §3).

## [0.2.0-alpha] - 2026-03-28

### Added

- Stack Install 5단계 Wizard 완성 (Resource Planning, Storage Plan, YAML View, Deploy Script, Dry Run)
- Stack List 상세 패널 (커넥션 정보, 상태, 인라인 디테일)
- Helm Orchestrator 다중 Phase DAG 실행 및 실제 Helm install/upgrade/rollback
- Helm Values Generator (Stack config → values.yaml 자동 생성)
- Stack Monitoring API 엔드포인트
- OSS Resource Defaults 도메인 전체 구현 (Entity, Port, Repository, UseCase)
- Stack Create/Delete UseCase 분리 및 확장
- OSS Resource Default 관리 페이지 (Admin 전용)
- Go 테스트 커버리지 검증 스크립트 (`scripts/check-coverage.sh`)
- DB 마이그레이션 000022-000028 (리소스 기본값, 템플릿 버전 정렬, 상태 enum 확장, 목업 스택)

### Changed

- Stack Install 최종 배포 검토 흐름을 YAML View → Deploy Script → Dry Run 단계로 확장
- Stack Name 하단에 Access Domain 입력란 추가 (`{StackName}.internal`)
- OSS 버전 메타데이터가 템플릿 편집 시에도 보존되도록 API 정규화
- GitLab 계열 OSS를 단일 Helm 번들로 통합
- Mock 데이터 fallback을 전 페이지에서 제거 (API 실패 시 빈 배열 반환)
- Stack Template 카드의 접근성 개선 (중첩 button → `div[role="button"]`)
- 마이그레이션 파일 번호 정리 및 중복 방지

### Fixed

- Organization 생성 시 DB 미저장 (API 경로 `/admin/organizations` → `/admin/orgs`)
- Mock auth ORG_ID/User ID가 DB 시드와 불일치하여 org 기반 API 실패
- CORS `AllowHeaders`에 `X-Org-ID` 누락
- 스택 시드 config JSON이 `StackConfig` 구조체와 불일치하여 unmarshal 실패
- 스택 히스토리 diff 초기값 하드코딩 → 최신 2개 버전 자동 선택
- 스택 히스토리 스냅샷 null 크래시 → null-safe 처리
- 클러스터 `unreachable`/`auth_failed` 상태 매핑 누락
- 개발 모드에서 rate limiter가 프론트엔드 요청 차단 → 프로덕션 전용으로 변경

## [0.1.0-alpha] - 2026-03-15

### Added

- Organization 설정 등록 — PostgreSQL Repository, CRUD API
- K8s Cluster 등록/검증 — client-go 검증 어댑터, Kubeconfig AES-256-GCM 암호화
- DevSecOps Stack 설정 5단계 Wizard — React Hook Form + Zod 검증
- Golden Path 템플릿 3종 — PostgreSQL Repository, Seed 데이터
- Stack 자동 설치/배포/이력 — Helm SDK 3-Phase DAG, WebSocket 로그 스트리밍, Rollback
- CI/CD Pipeline 템플릿 — Pipeline 템플릿, K8s Manifest Generator
- CI/CD Pipeline 배포/이력 — client-go Dynamic Applier, 배포 추적
- 모니터링/알림 — Prometheus HTTP Client, Dashboard, Alert CRUD
- 버전 호환성 관리 — 호환성 매트릭스, 검증 API, JSONB Diff
- UI 권한 체계 — Keycloak OIDC JWT, dual-mode 인증, 라우트별 RBAC
- 리소스 예상량 계산 — 리소스 계산기, 비용 추정
- React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui 프론트엔드 (15개 페이지)
- Go 1.26 + Echo v4 + PostgreSQL 백엔드 (Clean Architecture + DDD, 5 Bounded Context)
- GitHub Actions CI (Go test + Vite build + Vitest + Playwright E2E)
- Docker 멀티스테이지 빌드 + Helm 차트
- testcontainers-go 통합 테스트 인프라
- 로컬 개발 환경 스크립트 (`runbook_local.sh`)

### Fixed

- 프론트엔드-백엔드 호환성: 템플릿 tools 매핑, cluster status 필드 통일
- PostgresOrgRepository NULL default_admin_id 스캔 오류
- estimated_install_time 나노초→분 변환 오버플로우

<!--
compare 링크는 실제로 발행된 태그만 가리킨다 (릴리즈 정책 §3).
0.1.0-alpha / 0.2.0-alpha 는 CHANGELOG 기록만 존재하고 git 태그·GitHub Release 가 발행된 적이 없어
링크를 두지 않는다. v0.3.0-alpha 태그 push 이후 아래 링크가 유효해진다.
-->
[unreleased]: https://github.com/cloud-nullus/nullus/compare/v0.4.0-alpha...HEAD
[0.4.0-alpha]: https://github.com/cloud-nullus/nullus/compare/v0.3.0-alpha...v0.4.0-alpha
[0.3.0-alpha]: https://github.com/cloud-nullus/nullus/releases/tag/v0.3.0-alpha
