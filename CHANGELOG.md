# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **같은 커밋에서 CD 가 두 번 돌던 문제**: `cd.yml` 은 `main` push 와 `v*` 태그 push 양쪽에 걸려 있다. 릴리즈를 머지하고 곧바로 태그를 밀면 같은 커밋으로 두 런이 동시에 시작해 arm64 크로스빌드가 통째로 두 번 돈다 — `ec123dc` 에서 실제로 발생했다. 태그 런이 브랜치 런의 상위집합(차트·릴리즈·배포까지)이므로 커밋 SHA 기준 `concurrency` 로 묶어 앞선 런을 접는다. 태그는 언제나 그 커밋이 브랜치에 올라간 뒤에 밀리므로 남는 쪽이 태그 런이다.

### Added

- **CHANGELOG 누락과 릴리즈 중복을 CI 가 차단** (`scripts/check_changelog.py`, `Lint Review` 의 `📝 CHANGELOG Check`): 릴리즈 정책 §4.2 는 CHANGELOG 갱신을 PR 작성자의 의무로 규정하고 PR 템플릿 체크박스를 두기로 했으나, 체크박스는 차단 장치가 아니라 실제로 #114 가 CHANGELOG 없이 머지됐다 — GitLab 저장소 프로비저닝과 Argo CD 연동이라는 기능 본체가 릴리즈 노트에서 통째로 빠졌고, 릴리즈를 자를 때 커밋 36건을 손으로 대조해 17건을 백필해야 했다. 이제 동작이 바뀌는 파일을 고치고 `CHANGELOG.md` 를 건드리지 않으면 CI 가 실패한다(`no-changelog` 라벨로 면제, 문서·테스트 전용은 자동 면제). 같은 검사가 릴리즈를 자른 뒤 `main` 을 되머지할 때 생기는 `[Unreleased]`↔릴리즈 섹션 중복도 잡는다 — 이 역시 실제로 발생해 8건이 양쪽에 남아 있었다. 검사기는 로컬에서도 그대로 돈다.

### Changed

- **브랜치 명명 규칙을 중첩형으로 단일화** (컨벤션 v3): `CLAUDE.md` 는 `feat/<module>/<description>`, `Nullus_PR_커밋_컨벤션.md` v2 는 `feat/stack-tools-wizard` 로 서로 다르게 규정해 실제 브랜치가 34:33 으로 갈려 있었다(릴리즈 정책 §13-6 미해결 과제). 최근 브랜치가 전부 중첩형이라 **`<type>/<module>/<desc>` + `feat`/`fix`/`chore` 3종**으로 정하고 세 문서를 맞췄다. 기존 브랜치는 그대로 두고 신규 생성만 따른다.
- **릴리즈 본문에 설치법·산출물 경로·CHANGELOG 링크 추가** (`create-release`): 기존에는 CHANGELOG 섹션만 그대로 실어, `v0.4.0-alpha` 기준 항목 55개가 아무 안내 없이 먼저 나왔다. 릴리즈 페이지만 보고도 무엇을 어떻게 받는지 알 수 있도록 `helm install` 명령, 이미지·차트 경로, CHANGELOG·릴리즈 정책 링크, 직전 태그와의 compare 링크를 앞에 붙인다. 프리릴리즈면 그 사실도 맨 위에 밝힌다.
- **PR 이벤트 트리거에 `synchronize`·`edited` 추가** (`lint-review.yml`): 지적을 고쳐 push 해도 검사가 다시 돌지 않아, 통과시키려면 PR 을 close/reopen 해야 했다. CHANGELOG 검사처럼 "고치고 다시 밀면 통과" 가 전제인 검사는 이대로면 성립하지 않는다.

## [0.4.0-alpha] - 2026-08-09

### Added

- **Zadara Cloud PoC 배포 산출물** (`deploy/csp/zadara/`): `values-zadara.yaml`(워커 1대·NodePort ingress·local-path 스토리지 기준)과 배포 런북 `README.md`.
- **태그 push 로 릴리즈·배포까지 자동화** (`cd.yml`): `create-release` 가 CHANGELOG 의 해당 버전 섹션을 릴리즈 본문으로 쓰고(없으면 자동 생성 노트) GA 이전 버전은 프리릴리즈로 표시한다. `deploy-zadara` 가 bastion SSH 로 Zadara 클러스터에 `helm upgrade` 를 수행하고 공개 엔드포인트 2곳이 200 인지까지 확인한다. `environment: zadara` 로 실배포 직전 승인 게이트를 걸 수 있다. 필요한 secrets: `ZADARA_SSH_KEY`·`ZADARA_HOST`·`ZADARA_USER`·`NULLUS_DB_PASSWORD`·`NULLUS_ENCRYPTION_KEY`.
- **Zadara Cloud PoC 운영 스크립트** (`deploy/csp/zadara/`): 웹 UI 터널(`tunnel.sh`), 로컬 kubectl kubeconfig(`kubeconfig.sh`), 표준 포트 노출(`expose-web.sh`), apiserver 노출(`expose-apiserver.sh`, 기본 미사용), Keycloak realm 구성(`setup-keycloak-realm.sh`), Let's Encrypt TLS(`setup-tls.sh`).
- **차트에 SPA 런타임 설정 값 추가**: `web.auth.{mode,oidcProvider,oidcAuthority,oidcClientId}`. 비우면 이미지에 빌드된 값을 그대로 쓴다.
- **OpenBao 운영 모드 전환 및 ESO 시크릿 평면 구축**: dev 모드로 작동하던 OpenBao를 영속 스토리지 기반 운영 모드로 전환하고, 정적 토큰 인증을 Kubernetes Auth 기반 단기 자격으로 교체했다. OpenBao에 보관된 시크릿이 External Secrets Operator(ESO)를 거쳐 최종 애플리케이션 파드까지 안전하게 전달되도록 주입 경로를 구성했다.
- **OSS OIDC 클라이언트 자동 프로비저닝 기능 추가**: 스택 설치 시 OSS 애플리케이션의 OIDC 클라이언트를 Keycloak에 자동으로 등록하는 SSO 핸드오프 경로를 구현했다. 클라이언트 시크릿을 플랫폼이 생성하여 OpenBao에 안전하게 기록하고 Keycloak에 즉시 동기화하도록 연동했다.
- **시크릿 회전 후 소비자 워크로드 반영(Rolling Restart) 구현**: 회전된 시크릿이 실제 서비스에 즉시 반영되도록 시크릿 소비 워크로드를 재시작하는 단계를 추가했다. 시크릿 소비 방식(환경변수·파일 마운트 등)에 따라 재시작 필요 여부를 판별하여 프로바이더별 정책으로 분기 처리하도록 배선했다.
- **에어갭 무인 설치의 백엔드 API 경로 통합**: Helm CLI를 직접 호출해 백엔드를 우회하던 기존 에어갭 설치 방식을 정식 백엔드 API 경로로 통합했다. 설치 과정에서 인-클러스터 자기 자신을 클러스터 목록에 자동 등록하고, 부트스트랩 자격증명의 안전한 폐기 및 멱등한 재발급 흐름을 추가했다.
- **GitLab 저장소 자동 프로비저닝 및 Argo CD 연동**: CI/CD 파이프라인 생성 시 GitLab 앱 저장소와 빌드 스캐폴딩(.gitlab-ci.yml, Dockerfile, K8s 매니페스트)을 자동으로 생성하고, GitLab Runner 빌드 이미지와 Argo CD GitOps 배포까지 이어지는 엔드투엔드 연동 경로를 구현했다.
- **스택 접속 정보 API 도입 및 리소스 이름 단일 출처화**: Helm values와 프런트엔드 안내 문구에 파편화되어 하드코딩되어 있던 Secret·서비스 이름을 `internal/stack/domain/connection.go` 상수로 단일 출처화했다. `GET /api/v1/stacks/:stackId/connection-info` 엔드포인트를 통해 서버가 확정한 정합성 있는 접속 정보를 내려주고, 프런트엔드는 응답 데이터를 직접 구동해 표시하도록 개선했다.

- **컨테이너 레지스트리로 Harbor / Nexus 선택 가능** (스택 템플릿 `gitlab-harbor-v1`·`gitlab-nexus-v1` 추가): 기존에는 GitLab 내장 레지스트리뿐이었다. 템플릿은 Harbor 를 도구로 명시하면서도 **설치 단계가 아예 없어**, 고르면 CI 가 이미지를 올릴 곳이 존재하지 않았다. `installing_harbor`·`installing_nexus`·`provisioning_nexus` 를 설치 DAG 에 추가하고, 자격증명은 다른 도구와 같이 OpenBao → ESO 로 프로비저닝한다. Nexus 는 설치만으로는 Docker 커넥터도 저장소도 없고 관리자 비밀번호를 컨테이너가 무작위로 만들기 때문에, `provisioning_nexus` 가 비밀번호를 교체하고 `DockerToken` realm 을 켜고 docker/maven/npm 저장소와 8082 Service 를 만든다(재실행 가능). CI/CD 쪽에는 Nexus resolver 를 추가해 이미지를 UI(8081)가 아닌 Docker 커넥터(8082)로 보낸다 — UI 주소로 push 하면 이미지 대신 HTML 을 받는다. 로컬 kind 에서 두 레지스트리 모두 설치·이미지 push 를 확인했고, Harbor 는 GitLab CI 빌드 → push → Argo CD 동기화 → 파드 구동까지 digest 일치로 검증했다.
- **CI/CD 리스트와 배포 확인 모달에 사용 스택 표시**: 파이프라인은 이미 `stack_id` 를 저장하고 API 로도 내려주고 있었으나 화면에 없었다. 스택마다 레지스트리가 달라 이미지가 어디로 올라가는지가 달라지므로, 배포 직전과 목록에서 어느 스택 위에서 도는지 드러낸다.
### Changed

- **`runbook_csp.sh`의 레지스트리·이미지 태그 기본값 교정**: `REGISTRY`를 실재 경로(`ghcr.io/cloud-nullus/nullus`)로 바로잡고, 프리릴리즈에 존재하지 않는 `IMAGE_TAG=latest` 대신 비워 두어 차트 `appVersion`으로 폴백한다. 마이그레이션 Job이 이미지 레퍼런스를 문자열로 조립하므로 배포 시작 전에 태그를 한 번 확정한다.
- **스택 설치 경로의 리소스 이름 리터럴을 도메인 상수로 통일**: 스택 설치 오케스트레이터 및 어댑터 곳곳에 흩어져 있던 리소스 이름 리터럴들을 `internal/stack/domain` 및 `internal/shared/domain` 상수로 통일했다. 설치 경로의 리소스 명칭과 상수 정의가 어긋나지 않도록 `connection_contract_test.go` 통합 검증 테스트를 추가해 정합성을 고정했다.

### Fixed

- **OIDC 로그인이 유효한 토큰에도 401 로 거부되던 문제**: `setup-keycloak.sh` 가 `nullus-app` 클라이언트에 audience 매퍼를 만들지 않아 Keycloak 이 기본값 `aud: account` 로 토큰을 발급했고, `NULLUS_AUTH_OIDC_AUDIENCE=nullus-app` 를 검증하는 JWT 미들웨어가 이를 거부했다. audience 프로토콜 매퍼를 추가한다.
- **OIDC 로그인 후 스택 생성이 FK 위반으로 실패하던 문제**: JWT 미들웨어가 `User.OrgID` 를 채우지 않고 토큰에도 `org_id` 클레임이 없어, `resolveOrgID` 가 어떤 마이그레이션에도 존재하지 않는 `00000000-0000-0000-0000-000000000001` 로 폴백해 `stacks_org_id_fkey` 위반이 났다. 미들웨어가 `org_id`(및 `organization_id`/`org` 별칭) 클레임을 principal 로 전달하고, `setup-keycloak.sh` 가 사용자 속성과 클레임 매퍼를 등록한다. 폴백 조직도 시드 마이그레이션이 실제로 만드는 조직(`internal/shared/domain.SeededDefaultOrgID`)으로 바로잡고 `NULLUS_DEFAULT_ORG_ID` 로 덮어쓸 수 있게 했다.
- **로컬 Keycloak 이 기동 15분 뒤 realm 을 통째로 잃던 문제**: `KC_DB: dev-mem`(H2 인메모리)이 마지막 커넥션과 함께 사라져 `Table "REALM" not found (this database is empty)` 로그와 함께 토큰 엔드포인트가 500 을 반환했다. `dev-file` + 볼륨으로 바꾸고, KC 26 에서 deprecated 된 `KEYCLOAK_ADMIN*` 을 `KC_BOOTSTRAP_ADMIN_*` 로 교체했다.
- **`setup-keycloak.sh` 가 수동 개입 없이는 완주하지 못하던 문제**: master realm 의 `sslRequired` 기본값 때문에 평문 HTTP 관리 API 호출이 `HTTPS required` 로 막혔다. 실패 시 컨테이너 안에서 `kcadm` 으로 한 번 완화하고 재시도하도록 자동화하고, `nullus` realm 도 `sslRequired=none` 으로 생성한다. 응답이 비었을 때 JSON 파서가 traceback 으로 죽던 문제, Keycloak 24+ User Profile 이 선언되지 않은 `org_id` 속성을 조용히 버리던 문제도 함께 정리했다.
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
