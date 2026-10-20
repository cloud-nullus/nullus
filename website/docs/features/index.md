# 기능 한눈에 보기

사이드바에 뜨는 화면은 **19개**이고, 메뉴 없이 경로로만 들어가는 화면이 **13개** 더
있습니다. 아래 표가 전부입니다.

출처는 `web/src/components/layout/nav-model.tsx`(사이드바),
`web/src/app/routes.tsx`(라우트와 권한), `web/src/i18n/ko.json`(화면 이름)입니다.

![스택 목록 — 왼쪽에서 고르면 오른쪽에 상세가 열린다](../assets/screens/stack-list-light.png#only-light)
![스택 목록 — 왼쪽에서 고르면 오른쪽에 상세가 열린다](../assets/screens/stack-list-dark.png#only-dark)

## 사이드바 메뉴

### 데브섹옵스 스택

| 화면 | 경로 | 권한 | 하는 일 |
|------|------|------|---------|
| [스택 템플릿](stack/templates.md) | `/stack/templates` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> | 검증된 OSS 조합을 골라 설치 위자드로 들어간다 |
| [스택 목록](stack/list.md) | `/stack/list` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> | 설치된 스택과 상태·접속 정보를 본다 |
| [스택 이력](stack/history.md) | `/stack/history` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> | 스택 변경 내역을 비교하고 되돌린다 |
| [스택 버전](stack/versions.md) | `/stack/version` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> | 도구 버전 호환성 매트릭스를 본다 |
| [스택 버전 관리](stack/versions.md#스택-버전-관리) | `/admin/stack-versions` | <span class="role role-admin">admin</span> | 호환성 매트릭스를 직접 등록·수정한다 |
| [OSS 기본 리소스](stack/oss-defaults.md) | `/stack/oss-resource-default` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> | 도구별 CPU·메모리 기본값을 조정한다 |

### CI/CD

| 화면 | 경로 | 권한 | 하는 일 |
|------|------|------|---------|
| [CI/CD 템플릿](cicd/templates.md) | `/cicd/templates` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> | 파이프라인 템플릿을 고른다 |
| [골든패스](cicd/golden-paths.md) | `/cicd/golden-paths` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> | 조직 표준 파이프라인 경로를 정의한다 |
| [CI/CD 목록](cicd/list.md) | `/cicd/list` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> <span class="role role-developer">developer</span> | 만든 파이프라인과 최근 실행 결과를 본다 |
| [CI/CD 이력](cicd/history.md) | `/cicd/history` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> <span class="role role-developer">developer</span> | 배포 이력을 추적한다 |

### 관측성

| 화면 | 경로 | 권한 | 하는 일 |
|------|------|------|---------|
| [모니터링 대시보드](observability/monitoring.md) | `/observability/monitoring` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> <span class="role role-developer">developer</span> | 클러스터·파이프라인·도구 상태를 한 화면에서 본다 |
| [알림 이력](observability/alert-history.md) | `/observability/alert-history` | <span class="role role-admin">admin</span> <span class="role role-devops">devops</span> <span class="role role-developer">developer</span> | 발생한 경보를 시간순으로 본다 |

!!! note "알림 규칙 화면은 일부러 숨겨져 있습니다"
    `alert-rules-page.tsx` 는 저장소에 있지만 **사이드바에도 라우트에도 올리지
    않았습니다.** 규칙이 저장만 되고 평가되지 않아, 화면을 열어 두면 "규칙을
    만들었는데 알림이 안 온다"가 되기 때문입니다. 규칙 편집기가 생길 때까지
    숨긴다는 결정이 `web/src/app/routes.alert-rules.test.tsx` 에 테스트로 고정돼
    있습니다. 지금 전달되는 것은 스택 기본 규칙의 심각 경보뿐입니다.

### 관리

| 화면 | 경로 | 권한 | 하는 일 |
|------|------|------|---------|
| [조직](admin/organization.md) | `/admin/organization` | <span class="role role-admin">admin</span> | 조직 정보와 리소스 프로파일을 관리한다 |
| [사용자 관리](admin/users.md) | `/admin/users` | <span class="role role-admin">admin</span> | 구성원을 초대하고 역할을 준다 |
| [클러스터 관리](admin/clusters.md) | `/admin/clusters` | <span class="role role-admin">admin</span> | 대상 Kubernetes 클러스터를 등록·검증한다 |
| [알려진 이슈](admin/known-issues.md) | `/admin/known-issues` | <span class="role role-admin">admin</span> | 알려진 제약과 회피법을 본다 |
| [토큰 관리](admin/tokens.md) | `/admin/token-management` | <span class="role role-admin">admin</span> | 외부 도구 토큰 출처와 갱신 상태를 본다 |
| [백업·복구](admin/backup.md) | `/admin/backup` | <span class="role role-admin">admin</span> | 플랫폼 상태를 백업하고 되돌린다 |
| [알림 채널](admin/notifications.md) | `/admin/notifications` | <span class="role role-admin">admin</span> | 경보를 받을 채널을 등록하고 시험 발송한다 |
| [AI 어시스턴트](assistant.md#켜기) | `/admin/assistant` | <span class="role role-admin">admin</span> | 어시스턴트가 쓸 모델 공급자 · 모델 · API 키를 등록한다 |

## 메뉴에 없는 화면

사이드바에 항목을 두지 않고, 다른 화면에서 이어 들어가는 경로입니다.

| 화면 | 경로 | 들어가는 길 |
|------|------|-------------|
| 홈(역할별 요약) | `/` | 로고 클릭 |
| [스택 설치 위자드](stack/install.md) | `/stack/install` | 스택 템플릿에서 템플릿 선택 |
| 스택 도구 추가 | `/stack/:id/add-tools` | 스택 상세 |
| 스택 배포 진행 | `/stack/deploy/:id` | 설치 시작 직후 |
| 스택 설치 로그 | `/stack/logs/:deploymentId` | 배포 진행 화면 |
| 스택 재시도 이력 | `/stack/deployments/:deploymentId/retry-history` | 배포 진행 화면 |
| 스택 버전(별칭) | `/stack/versions` | `/stack/version` 과 같은 화면 |
| 조직(별칭) | `/admin/organizations` | `/admin/organization` 과 같은 화면 |
| [앱 배포](cicd/app-deploy.md) | `/cicd/developer-deploy` | **developer 로그인 시 첫 화면** |
| 파이프라인 생성 | `/cicd/create` | CI/CD 목록의 생성 버튼 |
| 파이프라인 로그 | `/cicd/pipelines/:id/logs` | CI/CD 목록·이력 |
| 로그인 | `/login` | 인증이 없을 때 자동 이동 |
| 404 | 그 외 모든 경로 | — |
| [AI 어시스턴트](assistant.md) | 경로 없음 — 패널 | 모든 화면 오른쪽 아래 버튼(관리자가 켰을 때만) |

## 사이드바와 라우트가 다를 수 있습니다

권한은 두 군데에서 걸립니다 — **사이드바가 메뉴를 숨기는 것**과 **라우트가 접근을
막는 것**이고, 둘이 항상 같지는 않습니다.

예를 들어 `/stack/templates`·`/stack/list`·`/stack/history`·`/stack/version` 은
라우트에 역할 제한이 없는 묶음(`<ProtectedRoute />`)에 들어 있어 **로그인한
developer 도 주소로는 열 수 있습니다.** 사이드바에서만 가려져 있을 뿐입니다.
`/cicd/templates` 와 `/cicd/golden-paths` 도 라우트는 세 역할 모두 허용하는데
사이드바는 admin·devops 에게만 보여 줍니다.

전체 대조표는 [권한 표](../reference/rbac.md)에 있습니다.
