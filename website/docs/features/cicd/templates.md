# CI/CD 템플릿

<span class="route">/cicd/templates</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

파이프라인 기본 템플릿을 고르는 화면입니다.

![파이프라인 템플릿 목록](../../assets/screens/cicd-templates-light.png#only-light)
![파이프라인 템플릿 목록](../../assets/screens/cicd-templates-dark.png#only-dark)

## 쓰는 법

1. 카드를 훑거나 검색으로 좁힙니다(이름·설명 대상).
2. 카드의 **Use Base Template** 을 누릅니다.
3. [앱 배포](app-deploy.md) 화면(`/cicd/developer-deploy?template=…`)으로 넘어가
   값이 미리 채워집니다.

!!! note "`/cicd/create` 로 가지 않습니다"
    파이프라인 생성 화면(`/cicd/create`)이 따로 있지만, 이 버튼은 거기로 가지
    않습니다. 자세한 사정은 [CI/CD 목록](list.md#파이프라인-생성-화면)에 적어
    두었습니다.

## 카드 구성

템플릿은 **CI/CD Type** 세 묶음으로 나뉘어 보입니다 — `Default`(Deployment ·
Service · Ingress) · `Helm`(Helm 차트 릴리스) · `Cronjob/Job`. 분류는 ID·이름·
stages 문자열을 보고 화면에서 정합니다.

카드에는 이름, 설명, CI/CD Type, Application Type, **Capabilities** 배지
(CI · CD · Test · Security), 만든 사람이 나옵니다. 원본을 가져오는 템플릿이면
**Source** 칸에 원본 저장소와 버전이 함께 나옵니다([아래](#원본을-가져오는-템플릿)).

Nullus 샘플 앱 템플릿 두 개와 전자정부 표준프레임워크 템플릿 두 개가 항상 맨 앞으로
정렬됩니다.

!!! warning "목록이 서버 응답과 다릅니다"
    화면이 두 가지를 손봅니다.

    - `web-frontend-v1` 은 **목록에서 제거**합니다
    - `web-backend-v1` 은 이름을 **`User Custom Pipeline`** 으로 바꿔 보여 줍니다

    설명 문구도 화면 안에 한국어·영어로 박아 둔 표에서 가져옵니다 — 서버의
    `description` 이 아닙니다.

## 원본을 가져오는 템플릿

템플릿에 **원본(source)** 이 있으면, 파이프라인을 만들 때 원본 저장소의 정해 둔 커밋을
새 앱 저장소의 첫 커밋으로 복사합니다. 그 위에 플랫폼이 CI 정의와 배포 매니페스트를
얹습니다.

기본으로 두 개가 들어 있습니다(마이그레이션 `000101`).

| 템플릿 | 원본 | 빌드 프로필 |
|--------|------|-------------|
| eGovFrame Boot Sample (Spring Boot) | `eGovFramework/egovframe-boot-sample-java-config` v5.0.0 | `spring-boot` |
| eGovFrame Web Sample (JSP) | `eGovFramework/egovframe-web-sample` v5.0.0 | `maven-war` |

둘 다 내장 HSQLDB(메모리)로 돌아 DB 를 따로 띄우지 않습니다. 앱은 8080 에서 듣고
`/` 에서 열립니다.

### 가져오는 방식

- **fork 하지 않습니다.** 그 커밋의 파일만 복사하고 이력은 가져오지 않습니다. fork 는
  같은 호스트 안에서만 되고(GitHub → 스택 GitLab 불가), 공개 저장소의 fork 는 private 로
  둘 수 없습니다.
- 원본은 **커밋 SHA** 로 고정합니다. 태그는 다시 찍힐 수 있어 받지 않습니다 — 카드에 보이는
  `v5.0.0` 은 표시용 이름입니다.
- 파이프라인을 만들 때 API 서버가 `codeload.github.com` 에서 원본을 받습니다. **인터넷
  연결이 필요하고**, 오프라인(에어갭) 설치에서는 이 템플릿을 쓸 수 없습니다. 지금은
  `https://github.com` 저장소만 가져올 수 있습니다.
- 원본은 **새로 만든 저장소에만** 받습니다. 받기 · 검사 · 커밋 중 하나라도 실패하면 방금 만든
  저장소를 지우고 실패합니다 — README 만 든 빈 저장소가 남지 않습니다.
  - 원본 호스트 쪽 문제(오프라인 · 없는 커밋 · 한도 초과 · 파일 1000개/32MB 초과):
    `502 TEMPLATE_SOURCE_UNAVAILABLE`
  - 쓸 수 없는 원본(심볼릭 링크 · 지원하지 않는 호스트): `422 TEMPLATE_SOURCE_UNSUPPORTED`
- 스택에 묶인 파이프라인으로만 만들 수 있습니다. 스택 없이 만들려 하면
  `400 TEMPLATE_SOURCE_REQUIRES_STACK` 입니다.
- 같은 이름의 저장소가 이미 있으면 원본도 스캐폴딩도 쓰지 않고, 원본을 받지도 않습니다(일반
  템플릿과 같은 규칙).
- 원본 파일의 실행 권한은 GitLab · GitHub 스택에서 지켜집니다. Gitea 는 파일 모드를 받지 않습니다.

### 파일을 누가 관리하나

| 구분 | 파일 |
|------|------|
| 원본 그대로 | 소스, `README.md`, 원본 `Dockerfile`(빌드 프로필이 바꾸지 않을 때) |
| 플랫폼 소유 | CI 정의(`.gitlab-ci.yml` · `.github/workflows/nullus-ci.yml` · `Jenkinsfile`), `deploy/`, `.pre-commit-config.yaml` · `.gitleaks.toml` |
| 가져오지 않음 | 원본의 다른 CI 정의(`.github/workflows/*` 등), 원본의 `deploy/` |

원본의 다른 CI 정의는 우리 CI 와 겹쳐 돌고, `deploy/` 는 Argo CD 가 통째로 적용하므로
가져오지 않습니다. 무엇을 빼고 바꿨는지는 저장소의 `NULLUS.md` 에 적힙니다 — 원본 README 를
덮지 않는 대신 흐름·변수·출처 안내를 이 파일에 둡니다. CI 정의만 다시 쓰는
[정의 갱신](list.md)은 원본이 있는 저장소에서도 같게 동작하고, 저장소가 없어 다시 만들 때는 처음처럼
원본을 다시 가져옵니다.

### 빌드 프로필

| 프로필 | Dockerfile | 배포 매니페스트 |
|--------|-----------|----------------|
| `spring-boot` | 원본에 있으면 그대로, 없으면 Maven 으로 실행 jar 를 만들어 `eclipse-temurin:17-jre` 에 올리는 Dockerfile 을 씁니다 | readiness(TCP), 메모리 384Mi/768Mi, CPU 200m/1 |
| `maven-war` | 항상 플랫폼 것 — Maven 으로 WAR 를 만들어 `tomcat:10.1-jdk17-temurin` 의 `ROOT.war` 로 올립니다 | readiness(TCP), 메모리 512Mi/1Gi, CPU 250m/1 |

- eGov 부트 샘플 v5.0.0 에는 Dockerfile 이 없어 플랫폼이 씁니다(원본 main 에는 그 뒤에 생겼습니다).
- eGov 웹 샘플의 원본 Dockerfile 은 미리 빌드한 `target/*.war` 를 복사하는 구조라 CI 의
  `docker build .` 로는 돌지 않습니다. 그래서 `maven-war` 는 늘 바꿉니다.
- Maven 테스트는 건너뜁니다(`-Dmaven.test.skip=true`) — 원본 테스트에 브라우저가 필요한
  Selenium 테스트가 있습니다.
- readiness 는 HTTP 경로가 아니라 포트가 열렸는지를 봅니다. actuator 나 `/` 응답이 있다는
  보장이 없습니다(eGov 부트 샘플 v5.0.0 에는 actuator 가 없습니다).

### 원본이 업데이트되면

- **이미 만든 파이프라인의 저장소는 바뀌지 않습니다.** 가져온 순간부터 사용자의 코드입니다.
- **새 파이프라인은 템플릿에 고정한 커밋을 받습니다.** 원본의 새 버전은 템플릿의 원본 커밋을
  올린 뒤 새로 만드는 파이프라인부터 들어옵니다.
- 파이프라인은 처음 가져온 원본(저장소·커밋)을 기록합니다(API 응답의 `pipeline.source`).
  템플릿의 커밋이 올라가도 그 저장소가 어디서 시작했는지 남습니다.
- 프레임워크 보안 패치·버전업은 샘플 코드가 아니라 `pom.xml` 의 의존성 버전으로 받습니다.

### API 로 원본 다루기

템플릿의 `source` 는 `repo_url` · `commit`(40자 소문자 SHA) · `label` · `build_profile`
(`""` · `spring-boot` · `maven-war`)입니다. 화면의 만들기·수정 창은 원본을 다루지 않아
API 로만 넣습니다. 수정 요청에서 `source` 를 빼면 저장된 원본을 그대로 두고, `{}` 를 보내면
지우며, 일부 칸만 보내면 400 입니다. 저장할 때 가져올 수 있는 주소인지 봅니다(지금은
`https://github.com/<소유자>/<저장소>`).

## 만들기·수정·삭제 (admin)

**Create Template · Edit · Delete** 는 admin 에게만 보입니다.

입력 항목은 Template ID(수정 시 잠김) · Name · Description ·
Application Type(`web` · `backend` · `batch`) · Capabilities 체크박스
(CI · CD · Test · Security)입니다. 이름은 필수이고 Capability 하나 이상을 골라야
합니다.

!!! warning "Capabilities 가 stages 칸에 저장됩니다"
    창이 저장할 때 체크한 Capability 라벨을 그대로 서버의 `stages` 컬럼에
    넣습니다. 즉 admin 이 만든 템플릿은 `stages = ["CI","CD"]` 처럼 **실제 단계
    이름이 아닌 값**을 갖습니다. 그 값이 나중에 [CI/CD 목록](list.md)의 이력 탭에서
    단계 목록으로 그려지므로, 단계가 이상하게 보일 수 있습니다.

## 호출하는 API

| 동작 | 엔드포인트 |
|------|-----------|
| 목록 | `GET /api/v1/cicd/templates` |
| 생성 | `POST /api/v1/cicd/templates` |
| 수정 | `PUT /api/v1/cicd/templates/{id}` (보내지 않은 항목은 기존 값 유지) |
| 삭제 | `DELETE /api/v1/cicd/templates/{id}` |

!!! warning "developer 는 메뉴로 올 수 없습니다"
    라우트는 developer 를 허용하지만 사이드바에는 항목이 없습니다. 홈 화면의
    시작 버튼으로 들어올 수는 있어서, 한 번 벗어나면 메뉴로 다시 찾아갈 수
    없습니다.
