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
항목 하나만 나옵니다). 그러면 아래 세 칸의 기본 주소가 그 스택의 저장소·레지스트리
주소로 채워집니다.

1. **Source Repository** — 스택 주소(수정 불가) + 경로
2. **Dockerfile Repository**
3. **Image Registry**

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

!!! warning "developer 는 이 화면을 끝까지 채울 수 없습니다"
    Stack · Cluster · Namespace 선택이 admin·devops 전용 조회 API 에서 옵니다.
    인증을 켠 환경에서 developer 로 들어가면 **이 선택들이 403 으로 비어** 생성까지
    가지 못합니다 — developer 의 첫 화면인데도 그렇습니다. 파이프라인 생성·배포
    자체는 서버가 허용하니, 막히는 것은 화면을 채우는 조회뿐입니다
    ([권한 표](../../reference/rbac.md#화면은-열리는데-데이터가-비는-경우)).
