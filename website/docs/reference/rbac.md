# 권한 표

역할은 **admin · devops · developer** 셋입니다. 접근 제한은 세 군데에서 걸립니다.

1. **사이드바** — 메뉴를 보여 줄지 (`web/src/components/layout/nav-model.tsx`)
2. **라우트** — 주소로 들어오는 것을 막을지 (`web/src/app/routes.tsx` 의 `ProtectedRoute allowedRoles`)
3. **서버** — API 호출을 거부할지

세 겹이 늘 같지는 않습니다. 사이드바에서 안 보이는 화면이 주소로는 열리는 경우가
있으니, **권한의 기준은 서버**로 보시는 게 맞습니다.

## 로그인 후 첫 화면

`web/src/features/auth/role-landing.ts` 가 단일 출처입니다. 로그인 리다이렉트와
홈의 시작 버튼이 같은 값을 읽습니다.

| 역할 | 첫 화면 |
|------|---------|
| admin | `/admin/organization` |
| devops | `/stack/templates` |
| developer | `/cicd/developer-deploy` |

역할을 알 수 없으면 홈(`/`)으로 보냅니다 — 권한 밖 경로로 보내면 바로 튕기기
때문입니다.

## 라우트 권한 — 네 묶음

`routes.tsx` 는 화면을 네 묶음으로 나눠 감싸고 있습니다.

| 묶음 | 허용 역할 | 들어 있는 경로 |
|------|-----------|----------------|
| `<ProtectedRoute />`<br>(역할 제한 없음 = 로그인만 하면 됨) | admin · devops · developer | `/`<br>`/stack/templates`<br>`/stack/list`<br>`/stack/history/:stackId?`<br>`/stack/version`, `/stack/versions`<br>`/stack/logs/:deploymentId`<br>`/stack/deployments/:deploymentId/retry-history`<br>`/observability/monitoring`<br>`/observability/alert-history`<br>`/observability/security/dashboard` |
| `allowedRoles={['admin','devops']}` | admin · devops | `/stack/install`<br>`/stack/:id/add-tools`<br>`/stack/deploy/:id`<br>`/stack/oss-resource-default` |
| `allowedRoles={['admin','devops','developer']}` | admin · devops · developer | `/cicd/developer-deploy`<br>`/cicd/templates`<br>`/cicd/create`<br>`/cicd/golden-paths`<br>`/cicd/list`<br>`/cicd/history`<br>`/cicd/pipelines/:id/logs` |
| `allowedRoles={['admin']}` | admin | `/admin/organization`, `/admin/organizations`<br>`/admin/users`<br>`/admin/clusters`<br>`/admin/known-issues`<br>`/admin/token-management`<br>`/admin/stack-versions`<br>`/admin/backup`<br>`/admin/notifications`<br>`/admin/assistant` |

## 사이드바와 라우트가 어긋나는 곳

아래 화면은 **메뉴에서는 가려지지만 주소로는 열립니다.** 설계상 의도일 수도 있고
아닐 수도 있으니, 민감한 정보를 다루는 화면이라면 서버 쪽 권한도 함께 확인하세요.

| 화면 | 사이드바에 보이는 역할 | 라우트가 허용하는 역할 | 차이 |
|------|----------------------|----------------------|------|
| `/stack/templates` | admin · devops | **모든 로그인 사용자** | developer 가 주소로 열 수 있다 |
| `/stack/list` | admin · devops | **모든 로그인 사용자** | 〃 |
| `/stack/history` | admin · devops | **모든 로그인 사용자** | 〃 |
| `/stack/version` | admin · devops | **모든 로그인 사용자** | 〃 |
| `/cicd/templates` | admin · devops | admin · devops · developer | 〃 |
| `/cicd/golden-paths` | admin · devops | admin · devops · developer | 〃 |

반대로 어긋나는 경우(메뉴에는 보이는데 라우트가 막는 경우)는 없습니다.

## 서버 쪽 권한 — 실제 기준

API 는 그룹 단위로 역할을 검사합니다. **여기가 진짜 경계입니다.**

| 그룹 | 경로 | 허용 역할 |
|------|------|-----------|
| 관리 | `/api/v1/admin/**` | **admin 만** |
| 스택 | `/api/v1/stacks/**` | admin · devops |
| CI/CD | `/api/v1/cicd/**` | admin · devops · developer |
| 골든패스 쓰기 | `POST`·`PUT`·`DELETE /api/v1/cicd/golden-paths` | admin · devops |
| 관측성 | `/api/v1/observability/**` | **역할 검사 없음** (인증만) |
| 보안 대시보드 설치 OSS | `GET /api/v1/observability/security/stacks` | admin · devops |
| 로그인 | `POST /api/v1/auth/login` | 인증 전 |

역할 검사는 **계층이 아니라 명단**입니다 — admin 도 명단에 적혀 있어야
통과합니다.

골든패스와 보안 대시보드의 설치 OSS 경로는 그룹보다 좁습니다. 골든패스는 조직
전체가 고르는 설치 조합이라 읽기는 CI/CD 그룹 그대로 열어 두고, 바꾸는 쪽은
admin·devops 로 좁혀 두었습니다. 설치 OSS 경로는 스택 그룹(`/stacks/**`)과 같은
데이터를 모아 보여 주므로 스택 그룹과 같은 역할만 받습니다.

!!! danger "관측성 그룹은 역할을 보지 않습니다"
    알림 규칙 생성·수정·삭제까지 **인증한 모든 역할**이 호출할 수 있습니다.
    화면은 내렸지만 API 는 열려 있는 상태입니다. 예외는 보안 대시보드의
    `/observability/security/stacks` 하나로, 경로에 직접 역할 검사를 달았습니다.

!!! danger "development 모드는 모든 검사를 끕니다"
    `server.mode` 가 `development` 면 인증 미들웨어와 역할 검사가 아예
    등록되지 않습니다. 네 그룹 전부 열립니다.

!!! note "쓰이지 않는 권한 매트릭스가 코드에 남아 있습니다"
    `roleAPIMatrix` 와 경로별 역할 표를 담은 미들웨어가 저장소에 있지만
    **어디에도 연결돼 있지 않습니다.** 그 표에 적힌 `/api/v1/clusters` 같은
    경로는 존재하지도 않습니다. 권한을 확인할 때 그 코드를 근거로 삼지 마세요.

## 화면은 열리는데 데이터가 비는 경우

프런트엔드의 여러 화면이 자기 권한보다 좁은 API 를 부릅니다. 그래서 화면은
열리지만 목록이 비어 보입니다.

| 화면 | 화면 권한 | 부르는 API | API 권한 |
|------|-----------|-----------|----------|
| [스택 설치](../features/stack/install.md) | admin · devops | `/api/v1/admin/clusters`<br>`…/clusters/{id}/namespaces`<br>`…/clusters/{id}/storage-classes`<br>`…/organization/resource-profiles` | **admin** |
| [앱 배포](../features/cicd/app-deploy.md) | 세 역할 | `/api/v1/admin/clusters`<br>`…/clusters/{id}/namespaces`<br>`/api/v1/stacks`<br>`/api/v1/stacks/{id}/integrations` | admin / admin·devops |
| [CI/CD 목록](../features/cicd/list.md) | 세 역할 | `/api/v1/admin/clusters`<br>`/api/v1/stacks` | admin / admin·devops |
| [모니터링 대시보드](../features/observability/monitoring.md) | 세 역할 | `/api/v1/admin/clusters`<br>`/api/v1/stacks/{id}/monitoring` | admin / admin·devops |
| [알림 이력](../features/observability/alert-history.md) | 세 역할 | `/api/v1/admin/clusters`<br>`/api/v1/stacks` | admin / admin·devops |
| 홈 | 세 역할 | `/api/v1/admin/clusters` | admin |

비교적 영향이 큰 쪽은 **developer 의 첫 화면인 앱 배포**와 **devops 의 설치
위자드**입니다.

## 역할별로 메뉴에 보이는 것

| 그룹 | admin | devops | developer |
|------|:-----:|:------:|:---------:|
| 데브섹옵스 스택 | 6개 | 5개 | — |
| CI/CD | 4개 | 4개 | 2개 |
| 관측성 | 3개 | 3개 | 3개 |
| 관리 | 7개 | — | — |
| **합계** | **20** | **12** | **5** |

- devops 에게 안 보이는 스택 메뉴 1개는 **스택 버전 관리**(`/admin/stack-versions`)로, admin 전용입니다.
- developer 에게 보이는 5개는 **CI/CD 목록 · CI/CD 이력 · 모니터링 대시보드 · 알림 이력 · 보안 대시보드**입니다.
- 관측성의 **보안 대시보드**는 하위 그룹 **보안** 아래에 있습니다(관측성 › 보안 › 보안 대시보드). 하위 그룹은 메뉴 수에 세지 않습니다.
