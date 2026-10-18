# 골든패스

<span class="route">/cicd/golden-paths</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

도구 조합이 미리 정해진 추천 경로를 보는 화면입니다. 고르면 **스택 설치**로
이어집니다.

!!! note "화면 제목이 영문입니다"
    사이드바 라벨은 "골든패스" 인데 화면 제목은 `CI/CD Golden Path` 로 박혀
    있습니다.

## 쓰는 법

1. 카드를 훑거나 검색으로 좁힙니다.
2. **View Detail** 로 포함 도구와 버전을 확인합니다.
3. **Use this Golden Path** 를 누르면 도구 선택이 스택 설치 설정으로 옮겨지면서
   [`/stack/install`](../stack/install.md) 로 넘어갑니다.

카드에는 이름, 설명, 예상 설치 시간(분), 권장 사용, 최소 리소스, 앞의 도구 4개
배지와 `+N` 이 나옵니다. 상세 창에서는 도구마다 `category` · `name` ·
`Helm: <버전>` · `App: <버전>` 을 봅니다.

## 들어 있는 골든패스 3개

| ID | 이름 | 예상 | 최소 리소스 | 도구 |
|----|------|------|------------|------|
| `gitlab-allinone-v1` | GitLab All-in-One | 90분 | 8 vCPU / 16Gi / 100Gi | GitLab CE · GitLab CI · GitLab Registry · 오브젝트 스토리지 · Argo CD · Prometheus · Grafana · Loki |
| `gitlab-argocd-v1` | GitLab + Argo CD | 120분 | 10 vCPU / 20Gi / 130Gi | 위와 같고 레지스트리가 **Harbor** |
| `github-argocd-v1` | GitHub + Argo CD | 45분 | 4 vCPU / 8Gi / 50Gi | GitHub · GitHub Actions · GHCR (셋 다 외부) · 오브젝트 스토리지 · Argo CD · Prometheus · Grafana · Loki |

세 항목은 `cicd_golden_paths` 테이블에 마이그레이션이 넣어 둔 기본값입니다.
카드 순서는 `sort_order` 로 고정돼 있어 요청마다 바뀌지 않습니다.

!!! note "화면에는 편집기가 없습니다"
    추가·수정·삭제는 테이블에 남지만, 그 일을 하는 화면이 없습니다. 지금은
    API 를 직접 부르거나 SQL 로 넣어야 합니다. 바꾼 내용은 서버를 다시 띄워도
    남습니다.

!!! note "스택 템플릿의 골든패스와 다릅니다"
    [스택 템플릿](../stack/templates.md)에 나오는 `gitea-jenkins-argocd-v1`·
    `gitlab-harbor-v1`·`empty-template-v1` 같은 11개 항목은 **별개**입니다.
    그쪽은 데이터베이스 테이블(`golden_path_templates`)이고
    `/api/v1/stacks/templates` 가 서비스합니다. 여기 세 개와 섞지 마세요.
    도구 버전도 두 쪽이 어긋나 있습니다(예: Argo CD Helm 7.7.2 vs 7.7.16).

!!! warning "GitHub 골든패스는 레지스트리가 바뀝니다"
    `github-argocd-v1` 을 고르면 GHCR 이 스택 설정으로 옮겨지지 않고 기본
    GitLab 레지스트리로 되돌아갑니다. 도구 이름을 스택 쪽 ID 로 바꿔 주는 표에
    GHCR 항목이 빠져 있어서입니다.

!!! warning "developer 는 눌러도 못 들어갑니다"
    라우트는 developer 를 허용하지만, **Use this Golden Path** 가 보내는
    `/stack/install` 은 admin·devops 전용이라 바로 되돌려집니다.

## 호출하는 API

화면이 부르는 것은 목록 하나입니다.

```
GET /api/v1/cicd/golden-paths
```

응답은 배열이고 각 항목에 `id`, `name`, `description`,
`tools[{category, name, helm_version, app_version}]`,
`estimated_install_time`, `recommended_use_case`, `min_resources`, `created_at`
가 들어 있습니다. 순서는 `sort_order` → `id` 입니다.

나머지는 화면이 쓰지 않지만 열려 있습니다. 읽기는 `/cicd` 의 세 역할 모두,
**쓰기는 admin·devops 만** 할 수 있습니다.

| 메서드 | 경로 | 돌려주는 것 |
|--------|------|-------------|
| `GET` | `/api/v1/cicd/golden-paths/{id}` | 없으면 404 `CICD_GOLDEN_PATH_NOT_FOUND` |
| `POST` | `/api/v1/cicd/golden-paths` | 201. `id`·`name` 이 없으면 400, 같은 `id` 가 있으면 409 |
| `PUT` | `/api/v1/cicd/golden-paths/{id}` | 200. 없으면 404 — 새로 만들지 않습니다 |
| `DELETE` | `/api/v1/cicd/golden-paths/{id}` | 204. 없으면 404 |

`POST` 로 만든 것은 기본 세 건 뒤에 붙습니다. `PUT` 은 내용만 바꾸고 순서는
건드리지 않습니다.
