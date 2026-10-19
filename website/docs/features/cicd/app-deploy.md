# 앱 배포

<span class="route">/cicd/developer-deploy</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>
<span class="role role-developer">developer</span>

앱 하나를 파이프라인으로 올리는 화면입니다. **developer 가 로그인하면 바로 여기로
옵니다.** 사이드바에는 어느 역할에도 항목이 없고, [CI/CD 목록](list.md)·
[CI/CD 템플릿](templates.md)의 버튼이나 홈 화면에서 들어옵니다.

!!! note "단계 띠는 이동 버튼이 아니라 스크롤입니다"
    위쪽에 `1 기본 정보 · 2 코드 체크아웃 · 3 빌드 · 4 테스트 · 5 보안 · 6 생성`
    여섯 칸이 있지만 **다음/이전 버튼이 없습니다.** 누르면 그 자리로 스크롤만
    합니다. 2·3 은 같은 영역을 공유하고, **4(테스트)와 5(보안)는 아예 렌더되지
    않습니다** — 해당 기능이 코드에서 영구 비활성이기 때문입니다.

![developer 로 로그인했을 때의 앱 배포 화면 — 사이드바 항목이 4개다](../../assets/screens/app-deploy-light.png#only-light)
![developer 로 로그인했을 때의 앱 배포 화면 — 사이드바 항목이 4개다](../../assets/screens/app-deploy-dark.png#only-dark)

## 채우는 것

### 기본 정보

| 항목 | 규칙 |
|------|------|
| **App Name** | 필수. 2~50자 |
| **Capabilities** | `CI` · `CD` · `Test` · `Security` 체크박스. 기본값은 CI·CD, **Test·Security 는 비활성** |
| **Phase** | `production` · `qa` · `development`. 기본 production |

**Phase** 는 고를 수는 있지만 생성 요청에 실리지 않습니다 — 서버로 가지 않습니다.

### 코드 체크아웃·빌드 (CI 를 골랐을 때)

먼저 **Stack 선택** 으로 어느 스택의 도구를 쓸지 정합니다(스택이 없으면 수동 입력
항목 하나만 나옵니다). 스택에 묶인 파이프라인은 저장소를 입력받지 않고, 플랫폼이 실제로
만들 자리를 보여 줍니다 — 저장소는 언제나 `<그룹>/<앱 이름>` 으로 만들어집니다
(`GET /api/v1/cicd/repository-location?stack_id=…` 가 그룹과 웹 주소를 알려 줍니다).

1. **Source Repository** — `<스택 SCM 주소>/<그룹>/<앱 이름>`(읽기 전용). 같은 이름의 저장소가
   이미 있으면 그 저장소를 그대로 씁니다
2. **Dockerfile Repository** — 저장소 루트의 `Dockerfile`(읽기 전용)
3. **Image Registry** — 플랫폼이 스택 레지스트리에 정합니다(읽기 전용)

그 자리를 알 수 없는 서버(조회 API 가 없거나 실패)에서는 예전처럼 세 칸을 경로로 입력받습니다.

### 생성 (CD 를 골랐을 때)

| 묶음 | 항목 |
|------|------|
| Deploy YAML Repository | 주소 + 디렉터리, 그리고 다시 읽기 버튼 |
| Cluster & Namespace | **Cluster**(스택을 고르면 그 스택의 클러스터로 고정) · **Namespace**(`New Namespace` 로 직접 입력 가능) · **Service URL** |
| Resource Configuration | **Replicas**(1~10, 기본 2) · CPU Request `100m` · CPU Limit `500m` · Memory Request `128Mi` · Memory Limit `512Mi` — 모두 필수 |
| Environment Variables | KEY/값 행을 **Add Variable** 로 추가, 휴지통으로 삭제 |

오른쪽 **Review Manifest** 패널에 생성될 `Deployment` · `Service` · `Ingress`
YAML 미리보기가 나오고, 그 아래가 **Execute** 입니다.

!!! warning "다시 읽기 버튼은 파일 하나를 못 찾습니다"
    `deployment.yaml` · `service.yaml` · `ingress.yaml` 셋을 가져오려 하지만,
    Nullus 가 저장소에 써 주는 파일에 `deploy/ingress.yaml` 은 **없습니다.**
    세 번째는 항상 404 입니다.

## Execute 를 누르면

1. Capability 조합으로 실행 모드를 정합니다 — `ci_cd` · `ci` · `cd`
2. 미리보기의 세 매니페스트를 환경변수
   `NULLUS_MANIFEST_DEPLOYMENT` · `NULLUS_MANIFEST_SERVICE` ·
   `NULLUS_MANIFEST_INGRESS` 에 담습니다
3. `POST /api/v1/cicd/pipelines` 로 파이프라인을 만듭니다
4. 응답에 경고가 있으면 **여기서 멈추고** "파이프라인은 만들어졌지만 설정이
   덜 됐습니다" 상자를 띄웁니다
5. 경고가 없으면 `POST /api/v1/cicd/pipelines/{id}/deploy` 를 부르고
   `/cicd/pipelines/{id}/logs` 로 이동합니다

!!! warning "리소스 설정은 매니페스트 문자열로만 전달됩니다"
    서버는 `replicas` · `port` · `provision_repository` ·
    `registry_credentials` 를 받을 수 있지만 **화면이 그 넷을 보내지
    않습니다.** 그래서 Replicas·CPU·Memory 값은 위 환경변수에 들어간 YAML 안에만
    담겨 갑니다.

실제로 가는 항목은 `name` · `app_type` · `cluster_id` · `namespace` ·
`template_id` · `stack_id` · `git_repo_url` · `dockerfile_path` ·
`docker_context` · `execution_mode` · `env_vars` 입니다.

## 플랫폼이 직접 배포할 때의 단계

스택에 연결되지 않은 파이프라인은 Nullus 가 직접 빌드·적용합니다. 그때 로그에
찍히는 단계 이름은 CI 정의의 단계와 **다릅니다**.

```
Git Clone → Docker Build → Image Load(또는 Image Push)
→ Create Namespace → Create Deployment → Create Service → Create Ingress
```

스택에 연결된 파이프라인은 단계가 **`Trigger CI` 하나**입니다 — 실제 작업은
GitLab·GitHub·Jenkins 쪽에서 돌기 때문입니다. 그쪽 단계 구성은
[CI/CD 개요](index.md#파이프라인이-실제로-돌리는-단계)를 보세요.

만든 직후의 실행은 새 실행을 만들지 않고 스캐폴딩 커밋의 push 실행에 붙습니다. push 실행은 커밋보다
몇 초 늦게 나타나므로, 브랜치 머리 커밋이 방금 생겼으면 그 실행을 최대 20초 기다립니다(GitLab CI ·
GitHub Actions). 같은 커밋의 실행이 둘 생기면 두 deploy 가 같은 브랜치에 되커밋해 하나가 실패합니다.

CI 의 deploy 단계가 배포 태그를 되커밋하면 Argo CD 가 그 커밋을 동기화해 앱이 바뀝니다. Argo CD 는
기본으로 3분마다 저장소를 확인하므로, 그대로 두면 되커밋 뒤 최대 3분 늦게 배포됩니다. 그래서 파이프라인을
만들 때 다음을 합니다.

- **GitLab 스택은 저장소에 push webhook 을 겁니다.** 대상은 스택의 Argo CD 서버
  (`http://argo-cd-argocd-server.<스택 네임스페이스>.svc:80/api/webhook`)입니다. GitLab 은 기본 설정으로
  클러스터 안 주소로 가는 webhook 을 막으므로, 그 호스트의 80 포트 하나만 관리자 설정의 허용 목록
  (Admin → Settings → Network → Outbound requests 의 *Local IP addresses and domain names that hooks and
  integrations can access*)에 더합니다. 사설망 전체 허용은 켜지 않고, 관리자가 넣어 둔 항목은 그대로
  둡니다. GitLab 은 설정을 최대 1분 캐시해서, 스택의 첫 파이프라인에서는 허용이 퍼진 뒤 webhook 을 뒤에서
  다시 겁니다(최대 2분). 로컬 실측에서 되커밋 1~2초 뒤에 동기화됐습니다(webhook 이 없을 때는 85초).
  - webhook 을 걸지 못해도(권한 · 설정) 생성 경고로 알리지 않고 서버 로그에만 남깁니다 — 배포는 늦을 뿐
    되고, 경고가 있으면 화면이 실행을 멈추기 때문입니다.
  - [정의 갱신](list.md)도 webhook 을 다시 겁니다. 이 기능 전에 만든 파이프라인, 뒤에서 하던 재시도가
    API 재시작으로 끊긴 파이프라인은 정의를 갱신하면 webhook 을 얻습니다.
- **GitHub · Gitea 스택은 webhook 을 걸지 않습니다.** github.com 은 사설 클러스터의 Argo CD 에 닿지 못하고,
  Gitea 는 기본 설정(`webhook.ALLOWED_HOST_LIST=external`)이 사설망 주소를 막는데 그 설정은 설치 때만 바꿀
  수 있습니다. 새 커밋은 Argo CD 의 정기 확인(3분)으로 반영됩니다.
- **모든 스택에서 첫 비교를 확인합니다.** 저장소 자격증명과 Application 을 한 번에 넣으면 Argo CD 가
  자격증명을 보기 전에 첫 비교를 돌려 `authentication required` 로 끝날 수 있고, 그대로 두면 다음 정기
  확인까지 동기화되지 않습니다. 만든 직후 최대 20초 동안 지켜보다가 비교 오류면 다시 읽게 합니다
  (`argocd.argoproj.io/refresh` 주석). 두 번 다시 읽게 해도 실패하면 자격증명 · 경로 문제이므로 생성 응답에
  경고로 알립니다 — 화면은 경고가 있으면 실행하지 않고 멈춥니다(위 4단계). Argo CD 가 20초 안에 비교를
  마치지 못하면 결과를 모르는 채 경고하지 않고 넘깁니다.

!!! warning "developer 는 이 화면을 끝까지 채울 수 없습니다"
    Stack · Cluster · Namespace 선택이 admin·devops 전용 조회 API 에서 옵니다.
    인증을 켠 환경에서 developer 로 들어가면 **이 선택들이 403 으로 비어** 생성까지
    가지 못합니다 — developer 의 첫 화면인데도 그렇습니다. 파이프라인 생성·배포
    자체는 서버가 허용하니, 막히는 것은 화면을 채우는 조회뿐입니다
    ([권한 표](../../reference/rbac.md#화면은-열리는데-데이터가-비는-경우)).
