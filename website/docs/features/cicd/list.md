# CI/CD 목록

<span class="route">/cicd/list</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>
<span class="role role-developer">developer</span>

만든 파이프라인을 관리하는 화면입니다. 왼쪽 표, 오른쪽 상세 패널입니다.

![만들어진 파이프라인 목록](../../assets/screens/cicd-list-light.png#only-light)
![만들어진 파이프라인 목록](../../assets/screens/cicd-list-dark.png#only-dark)

## 표

컬럼은 **Name · Mode · App Type · Access**(외부 링크) **· Cluster ·
Stack**(없거나 `삭제됨`) **· Status · Last Deployed** 입니다.

툴바에 상태 선택, 클러스터 선택, 검색(이름·클러스터 이름)이 있습니다.
위쪽 버튼 둘은 [앱 배포](app-deploy.md)와 [CI/CD 템플릿](templates.md)으로
갑니다.

!!! warning "상태 필터는 아무것도 못 고릅니다"
    선택 항목이 `success` · `running` · `pending` · `failed` · `cancelled` 인데,
    파이프라인의 `status` 값은 **`active` / `inactive` 둘뿐**입니다. 그래서 어떤
    상태를 골라도 표가 비어 버립니다. 필터는 브라우저에서 돌고, 서버는
    `status`·`search` 를 보지 않습니다.

## 상세 패널 탭

### Info

서비스 URL 링크 버튼들과 카드 여섯 묶음입니다.

| 카드 | 내용 |
|------|------|
| Pipeline Info | 이름 · 앱 종류 · 실행 모드 배지(`CI Only`/`CD Only`/`CI/CD`) · 템플릿 · 상태 · 생성 · 마지막 배포 · 접근 URL |
| Code Checkout | Git 저장소 · 스택 |
| Build | Dockerfile · 빌드 컨텍스트 · 이미지 레지스트리 |
| Deployment Target | 클러스터 · 네임스페이스, 그리고 Ingress→Service→Deployment/StatefulSet→Pod 자원 사슬(Job·CronJob 포함) |
| Environment Variables | 가려져 있고 누르면 보입니다 |
| Manifest Overrides | |

### Monitoring

성공률, 평균 소요 시간, 7일 성공/실패 추이, 앱 자원 사용량 차트와 로그 패널,
새로 고침 버튼이 있는 `Live Resources` 목록.

### History

배포 한 건이 한 줄입니다 — 상태 배지 · 버전 · 이미지 스캔 게이트 배지와 심각도
개수 · SAST 게이트 배지 · 의존성 스캔 게이트 배지와 심각도 개수 · 시크릿 탐지 게이트
배지와 탐지 건수 · 실행자 · 소요 시간 · 시작 시각.

행을 펼치면 배포 ID, 스캔 상세(취약점 목록·필터), SAST 실패 조건과 지표, 의존성 스캔
상세, 시크릿 탐지 상세(규칙 · 파일:줄 · 커밋 · Fingerprint — 비밀값은 없다),
**Pipeline Stages** 목록, **Detailed Logs** 터미널이 나옵니다. 시크릿이 탐지된
실행은 폐기 · 회전 경고가 먼저 보입니다.

[보안 대시보드](../observability/security-dashboard.md)의 **실행 기록 열기** 는
`/cicd/list?pipeline=<ID>&tab=history&run=<배포 ID>` 로 이 탭과 그 실행을 바로 엽니다.

!!! note "단계 결과를 보고하지 않는 CI 도 있습니다"
    단계별 데이터가 없으면 목록에 `템플릿 정의 · 이 CI 는 단계별 결과를 보고하지
    않습니다` 라고 적히고, 단계는 `unknown` 으로 남아 초록색이 되지 않습니다.

## 버튼

| 버튼 | 하는 일 |
|------|---------|
| **Execute** | 파이프라인을 실행합니다 (`POST /api/v1/cicd/pipelines/{id}/deploy` → 202) |
| **Logs** | `/cicd/pipelines/{id}/logs` 로 이동 |
| **Refresh definition** | 저장소의 CI 정의를 다시 써 줍니다. 스택에 연결된 파이프라인에만 보입니다 |
| **Delete** | 아래 참고 |

!!! warning "Execute 창의 설정은 전송되지 않습니다"
    실행 창에 `Cluster / Build / Deploy` 탭과 Dockerfile 프리셋, 배포 YAML 선택이
    있지만 **아무것도 서버로 가지 않습니다.** 실제로 보내는 것은 파이프라인 ID
    하나뿐입니다. 창의 선택은 장식입니다.

### Refresh definition 이 덮어쓰는 것

창에 그대로 적혀 있습니다 — `.gitlab-ci.yml` · `.github/workflows/nullus-ci.yml` ·
`Jenkinsfile` 중 하나와 `SONAR_TOKEN` 같은 CI 변수입니다.
**앱 코드 · Dockerfile · `deploy/` 매니페스트 · Argo CD Application 은 건드리지
않습니다.**
GitLab 스택에서는 저장소가 새 커밋을 Argo CD 에 바로 알리는 push webhook 이 없으면 다시 겁니다
([앱 배포](app-deploy.md)).

### Delete 의 선택 항목

체크박스 세 개가 모두 **꺼진 상태**로 시작합니다 — 클러스터 리소스 · 저장소 ·
이미지. 저장소를 고르면 `owner/repo` 경로를 정확히 입력해야 확인 버튼이
열립니다. 지우지 못한 것은 배너로 알려 줍니다.

**클러스터 리소스**를 고르면 Argo CD Application 과 배포된 워크로드에 더해, 파이프라인을
만들 때 플랫폼이 넣은 Secret 두 개도 지웁니다 — 앱 네임스페이스의 이미지 pull
자격증명(`nullus-registry-<앱>`)과 Argo CD 네임스페이스의 저장소 자격증명
(`nullus-repo-<앱>`). 플랫폼 표시(`app.kubernetes.io/managed-by=nullus-cicd`)가 없는
Secret 은 이름이 같아도 남깁니다. 앱 네임스페이스 자체는 지우지 않습니다.

Gitea · Jenkins 스택은 파이프라인 자격증명을 따로 둡니다 — 스택 네임스페이스의 ExternalSecret
`nullus-ci-<앱>`, ESO 가 그것으로 만든 Secret, OpenBao 의
`kv/nullus/<env>/<org>/cicd/pipelines/<앱>/` 아래 값. 이 자격증명은 CI job 만 쓰므로 **체크박스와
상관없이** CI job 과 함께 지웁니다. ExternalSecret 을 먼저 지우고(ESO 가 만든 Secret 은 소유자 참조로
함께 사라집니다) 금고 값을 지웁니다. 플랫폼 표시가 없는 ExternalSecret 은 원격 키가 모두 그 파이프라인의
금고 경로 아래일 때만 지웁니다. 지우지 못해도 나머지 삭제는 이어 가고 배너로 알려 줍니다.

자격증명은 파이프라인 이름으로 찾습니다. 같은 스택에 같은 이름의 다른 파이프라인이 있으면 그것도 이
자격증명을 쓰므로 지우지 않고 그 이유를 알려 줍니다. 이름이 금고 경로의 한 세그먼트로 쓸 수 없는 값
(`/` 가 든 이름, `.` · `..` 그 자체)이어도 지우지 않습니다 — 다른 파이프라인의 값을 가리키게 됩니다.

!!! warning "이미 설치된 스택은 금고 값이 남습니다"
    금고 값을 지우려면 금고 정책에 삭제 권한이 있어야 하는데, 그 권한(파이프라인 자격증명 경로에만)은
    스택을 설치할 때 정책으로 들어갑니다. 설치 뒤에는 root token 이 폐기돼 정책을 고칠 수 없으므로,
    이 기능 전에 설치한 스택에서는 금고 값이 남고 "삭제 권한이 없습니다" 배너가 뜹니다. ExternalSecret 과
    Secret 은 지워집니다.

## 파이프라인 생성 화면

<span class="route">/cicd/create</span>

Pipeline Name 과 템플릿을 고르고 `Cluster / Build / Deploy / Pipeline YAML` 탭을
채우는 화면이 따로 있습니다.

!!! warning "들어가는 길이 없습니다"
    앱 안에서 `/cicd/create` 로 보내는 링크가 **이 화면 자신의 템플릿 선택
    말고는 없습니다.** CI/CD 템플릿의 `Use Base Template` 은 [앱 배포](app-deploy.md)로
    갑니다.

    게다가 Build 탭의 Dockerfile 프리셋 3개와 Deploy 탭의 YAML 프리셋 3개는 읽기
    전용 고정값이고, `Pipeline YAML` 탭의 내용은 화면에서 만들어 보여 주기만 하고
    **전송되지 않습니다.** 생성 시 실제로 가는 값은 이름·앱 종류·클러스터·템플릿
    넷뿐입니다.

    템플릿 목록이 비면 `web-frontend`·`web-backend`·`batch-job` 이라는 가짜
    템플릿으로 떨어지는데, 그 앱 종류 값은 DB 가 받지 않는 값(`web`·`backend`·
    `batch` 만 허용)이라 생성이 거부됩니다.

    실제로 파이프라인을 만드는 길은 [앱 배포](app-deploy.md)입니다.

## 파이프라인 로그

<span class="route">/cicd/pipelines/:id/logs</span>

왼쪽에 최근 배포 목록(버전·상태·시작 시각, 최신 우선), 오른쪽에 단계별 블록이
있는 터미널 창입니다. 화면 제목은 영문 `Pipeline Logs` 입니다.

!!! note "`Streaming...` 표시는 실제 스트리밍이 아닙니다"
    2초마다 `GET /api/v1/cicd/deployments/{id}` 를 **다시 요청**하다가 상태가
    `success` 또는 `failed` 가 되면 멈춥니다. WebSocket 으로 로그를 받는 코드가
    저장소에 있지만 쓰는 곳도, 서버 쪽 라우트도 없습니다.

## 호출하는 API

`GET /api/v1/cicd/pipelines` · `/templates` · `/deployments` ·
`/deployments/{id}` · `/pipelines/{id}/resources` · `/image-scans` ·
`/image-scans/{scanId}/vulnerabilities` · `/sast-results` ·
`POST /pipelines/{id}/deploy` · `/pipelines/{id}/refresh-definition` ·
`DELETE /pipelines/{id}?cluster_resources=&repository=&images=`

여기에 `GET /api/v1/admin/clusters`(admin 전용)와 `GET /api/v1/stacks`
(admin·devops)도 함께 부릅니다 — developer 에게는 403 이라 클러스터·스택 선택이
비어 보입니다([권한 표](../../reference/rbac.md#화면은-열리는데-데이터가-비는-경우)).
