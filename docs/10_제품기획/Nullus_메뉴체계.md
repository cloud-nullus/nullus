# Nullus 메뉴 체계

**작성일**: 2026-03-08  
**최종 갱신**: 2026-08-31 (구현 재대조)  
**용도**: proto3, 기능목록, **기능분해도(Nullus_기능분해도.csv)** 간 메뉴 명칭 통일

---

## 0. 현행 사이드바 (2026-08-31)

`web/src/components/layout/nav-model.tsx` 에서 추출했다(2026-08-11 판에는 `sidebar.tsx` 라고
적혀 있었으나, 그 뒤 메뉴 정의가 `nav-model.tsx` 로 분리됐다 — 화면 상단 `PageHeader` 도
같은 정의에서 그룹 이름을 읽는다). 아래가 실제로 화면에 뜨는 메뉴이며, 1장의 표는 초안이라
일부 다르다.

| 대메뉴 | 하위 메뉴 | 경로 | 노출 역할 |
|--------|-----------|------|-----------|
| **데브섹옵스 스택** | 스택 템플릿 | `/stack/templates` | admin, devops |
| | 스택 목록 | `/stack/list` | admin, devops |
| | 스택 이력 | `/stack/history` | admin, devops |
| | 스택 버전 | `/stack/version` | admin, devops |
| | 스택 버전 관리 | `/admin/stack-versions` | **admin** |
| | OSS 기본 리소스 | `/stack/oss-resource-default` | admin, devops |
| **CI/CD** | CI/CD 템플릿 | `/cicd/templates` | admin, devops |
| | **CI/CD 골든패스** | `/cicd/golden-paths` | admin, devops |
| | CI/CD 목록 | `/cicd/list` | admin, devops, developer |
| | CI/CD 이력 | `/cicd/history` | admin, devops, developer |
| **관측성** | 모니터링 대시보드 | `/observability/monitoring` | admin, devops, developer |
| | 알림 규칙 | `/observability/alerts` | admin, devops |
| | 알림 이력 | `/observability/alert-history` | admin, devops, developer |
| **관리** | 조직 | `/admin/organization` | **admin** |
| | 사용자 관리 | `/admin/users` | **admin** |
| | 클러스터 관리 | `/admin/clusters` | **admin** |
| | 알려진 이슈 | `/admin/known-issues` | **admin** |
| | **토큰 관리** | `/admin/token-management` | **admin** |

> 항목 **18개**. 2026-08-11 대비 2개가 늘었다 — **CI/CD 골든패스**, **토큰 관리**.
> 둘 다 그때 "도달할 수 없는 화면" 으로 기록됐던 것이 배선된 결과다(0.3 참고).

### 0.1 1장 초안과 달라진 점

- **스택 설치는 사이드바에 없다.** 스택 템플릿에서 템플릿을 고르면 설치 위자드
  (`/stack/install`)로 들어가는 흐름이라 독립 항목을 두지 않았다.
- 초안에 없던 메뉴 5개가 늘었다 — **OSS 기본 리소스**, **스택 버전 관리**(admin 전용),
  **알려진 이슈**(admin 전용), **CI/CD 골든패스**, **토큰 관리**(admin 전용).
- **초안에는 역할 제한이 없었다.** 실제로는 메뉴마다 노출 역할이 걸려 있고, 라우트에도
  `allowedRoles` 로 한 번 더 막는다.
- developer 가 보는 것은 **4개**다 — 모니터링 대시보드, 알림 이력, CI/CD 목록, CI/CD 이력.
  (2026-08-11 판은 "관측성 3개" 라고 적었으나 **알림 규칙은 admin·devops 전용**이라 3개가
  아니다. 대메뉴가 보여도 그 안의 항목은 역할마다 다르다.)

### 0.2 메뉴 없이 경로로만 접근하는 화면

| 화면 | 경로 | 진입 경로 |
|------|------|-----------|
| 홈(역할별 요약) | `/` | 로고 클릭 |
| 스택 설치 위자드 | `/stack/install` | 스택 템플릿에서 템플릿 선택 |
| 스택 도구 추가 | `/stack/:id/add-tools` | 스택 상세 |
| 스택 배포 진행 | `/stack/deploy/:id` | 설치 시작 직후 |
| 스택 설치 로그 | `/stack/logs/:deploymentId` | 배포 진행 화면 |
| 스택 재시도 이력 | `/stack/deployments/:deploymentId/retry-history` | 배포 진행 화면 |
| 스택 버전 목록 | `/stack/versions` | `/stack/version` 의 별칭 경로 |
| 알림 규칙(별칭) | `/observability/alert-rules` | 사이드바의 `/observability/alerts` 와 같은 화면 |
| 앱 배포(Developer) | `/cicd/developer-deploy` | **developer 로그인 시 기본 랜딩** |
| 파이프라인 생성 | `/cicd/create` | CI/CD 목록의 생성 버튼 |
| 파이프라인 로그 | `/cicd/pipelines/:id/logs` | CI/CD 목록·이력 |
| 조직 목록 | `/admin/organizations` | `/admin/organization` 의 별칭 경로 |
| 로그인 | `/login` | 미인증 접근 시 리다이렉트 |
| 404 | `*` | — |

별칭 경로가 3쌍 있다 — `stack/version`↔`stack/versions`,
`observability/alerts`↔`observability/alert-rules`,
`admin/organization`↔`admin/organizations`. 같은 컴포넌트를 가리키며 사이드바는 앞쪽만 쓴다.

### 0.3 도달할 수 없던 화면 4개 — **해소됨 (2026-08-11, PR #133)**

2026-08-11 판은 아래 4개가 라우트에도 사이드바에도 연결돼 있지 않다고 기록했다.
지금은 넷 다 연결됐다.

| 화면 | 현재 |
|------|------|
| `cicd-golden-path-page.tsx` | `/cicd/golden-paths` — **사이드바 CI/CD 그룹** |
| `token-management-page.tsx` | `/admin/token-management` — **사이드바 관리 그룹** |
| `cicd-pipeline-setup-page.tsx` | `/cicd/create` — 라우트만(CI/CD 목록에서 진입) |
| `stack-deployment-logs-page.tsx` | `/stack/deployments/:deploymentId/retry-history` — 라우트만 |

함께 기록됐던 **Developer 랜딩 불일치**도 해소됐다. 홈은 `/cicd/templates` 로 보내는데
사이드바는 그 메뉴를 developer 에게 숨겨, 한 번 벗어나면 메뉴로 다시 찾을 수 없었다.
지금은 로그인 리다이렉트와 홈의 시작 버튼이 `web/src/features/auth/role-landing.ts`
한 곳을 보고, developer 는 `/cicd/developer-deploy` 로 간다.

| 역할 | 로그인 후 랜딩 |
|------|---------------|
| admin | `/admin/organization` |
| devops | `/stack/templates` |
| developer | `/cicd/developer-deploy` |

---

## 1. 통일된 메뉴 구조

### 1.1 사이드바 메뉴 (최상위 → 하위)

| 대메뉴 | 하위 메뉴 | 메뉴 ID (data-page) | 설명 | 기능분해도 중분류 |
|--------|-----------|---------------------|------|-------------------|
| **데브섹옵스 스택** | 스택 템플릿 | templates | Golden Path 템플릿 선택 | Golden Path 템플릿 |
| | 스택 설치 | install | 5단계 설정 워크플로우 + Deploy | 노코드 설정 UI, 스택 생성/배포, 리소스 예상량 계산 |
| | 스택 목록 | list | 구성된 스택 목록 (검색/필터/정렬) | 스택 목록 관리 |
| | 스택 이력 | history | 스택 변경 이력 + diff + 롤백 | 스택 이력 관리 |
| | 스택 버전 관리 | compatibility | OSS 호환성 매트릭스 | OSS 버전 호환성 |
| **CI/CD** | CI/CD 템플릿 | cicdtemplates | 파이프라인 템플릿 목록 | 파이프라인 템플릿 |
| | CI/CD 목록 | cicdlist | 생성된 파이프라인 목록 | 파이프라인 관리 |
| | CI/CD 이력 | cicdhistory | 파이프라인 배포 이력 | 파이프라인 배포 |
| **관측성** | 모니터링 대시보드 | monitoring | Cluster/Pipeline/Tool Health | 모니터링 |
| | 알림 규칙 | alertlist | 알림 규칙 목록 | 알림 관리 |
| | 알림 이력 | alerthistory | 알림 발생 이력 | 알림 관리 |
| **관리** | 조직 | organization | 조직 정보 등록/수정 | 조직 관리 |
| | 사용자 관리 | users | 역할 부여/비활성화 | 사용자 관리 |
| | 클러스터 관리 | clusters | 클러스터 등록/수정/상태 | 클러스터 관리 |
| **사용자** | 로그아웃 | — | 로그아웃 | 인증 |

### 1.2 역할 전환 시 추가 화면

| 화면 | 설명 | 표시 조건 | 기능분해도 |
|------|------|-----------|------------|
| 앱 배포 | Developer Self-Service 배포 위자드 | Developer 역할 선택 시 | Developer Self-Service (CIC_040) |

### 1.3 공통 화면 (메뉴 외)

| 화면 | 설명 | 기능분해도 |
|------|------|------------|
| 홈 | 역할별 요약 대시보드 | USR 홈/대시보드 |
| 로그인 | 세션/Keycloak OIDC | USR 인증 |
| 다국어 | UI 언어 전환 (en/ko) | USR 다국어 (NULLUS_USR_030_010) |

---

## 2. 명칭 통일 규칙

### 2.1 대메뉴

| 통일 명칭 | 이전/혼용 표현 | 기능분해도 대분류 코드 |
|-----------|----------------|------------------------|
| 데브섹옵스 스택 | DevSecOps Stack | DSS |
| CI/CD | — | CIC |
| 관측성 | Observability | OBS |
| 관리 | Admin | ADM |
| 사용자 | User | USR |

### 2.2 하위 메뉴

| 통일 명칭 | 이전/혼용 표현 |
|-----------|----------------|
| 스택 템플릿 | DevSecOps Stack Template |
| 스택 설치 | DevSecOps Stack Install, Install DevSecOps |
| 스택 목록 | DevSecOps Stack List |
| 스택 이력 | DevSecOps Stack History |
| 스택 버전 관리 | DevSecOps Stack Version Management |
| CI/CD 템플릿 | CI/CD Template |
| CI/CD 목록 | CI/CD List |
| CI/CD 이력 | CI/CD History |
| 모니터링 대시보드 | Monitoring Dashboard |
| 알림 규칙 | Alert Rule List |
| 알림 이력 | Alert History |
| 조직 | Organization |
| 사용자 관리 | User Management |
| 클러스터 관리 | Cluster Management |
| 로그아웃 | Log out |

---

## 3. 기능분해도 매핑

기능분해도(Nullus_기능분해도.csv)의 대분류/중분류와 메뉴/화면 매핑

| 대분류 | 대분류 코드 | 중분류 | 연결 메뉴/화면 | 대표 기능 ID |
|--------|-------------|--------|----------------|--------------|
| 조직 | ORG | 시스템 최초 설정 | — (백엔드) | NULLUS_ORG_000_xxx |
| | | Organization 관리 | 관리 > 조직 | NULLUS_ORG_010_xxx |
| | | 클러스터 접근 범위 | 관리 > 조직 (하위) | NULLUS_ORG_020_xxx |
| | | 멤버 관리 | 관리 > 조직/사용자 관리 | NULLUS_ORG_030_xxx |
| 클러스터 | CLU | 클러스터 관리 | 관리 > 클러스터 관리 | NULLUS_CLU_010_xxx |
| | | Kubeconfig 관리 | 관리 > 클러스터 관리 (등록/수정 시) | NULLUS_CLU_020_xxx |
| | | 클러스터 메타정보 | 관리 > 클러스터 관리 | NULLUS_CLU_030_xxx |
| | | 클러스터 선택 | 데브섹옵스 스택 > 스택 설치 | NULLUS_CLU_040_010 |
| | | Organization 접근 | 관리 > 클러스터 관리 (하위) | NULLUS_CLU_040_020/030 |
| DevSecOps 스택 | DSS | Golden Path 템플릿 | 데브섹옵스 스택 > 스택 템플릿 | NULLUS_DSS_010_xxx |
| | | 노코드 설정 UI | 데브섹옵스 스택 > 스택 설치 | NULLUS_DSS_020_xxx |
| | | 스택 생성/배포 | 데브섹옵스 스택 > 스택 설치 | NULLUS_DSS_030_xxx |
| | | 스택 목록 관리 | 데브섹옵스 스택 > 스택 목록 | NULLUS_DSS_040_xxx |
| | | 스택 이력 관리 | 데브섹옵스 스택 > 스택 이력 | NULLUS_DSS_050_xxx |
| | | OSS 버전 호환성 | 데브섹옵스 스택 > 스택 버전 관리 | NULLUS_DSS_060_xxx |
| | | 리소스 예상량 계산 | 데브섹옵스 스택 > 스택 설치 (Resources 탭) | NULLUS_DSS_070_xxx |
| CI/CD | CIC | 파이프라인 템플릿 | CI/CD > CI/CD 템플릿 | NULLUS_CIC_010_xxx |
| | | 파이프라인 관리 | CI/CD > CI/CD 목록 | NULLUS_CIC_020_xxx |
| | | 파이프라인 배포 | CI/CD > CI/CD 이력 | NULLUS_CIC_030_xxx |
| | | Developer Self-Service | 앱 배포 (역할 전환 시) | NULLUS_CIC_040_xxx |
| 관측성 | OBS | 모니터링 | 관측성 > 모니터링 대시보드 | NULLUS_OBS_010_xxx |
| | | 알림 관리 | 관측성 > 알림 규칙, 알림 이력 | NULLUS_OBS_020_xxx |
| 관리 | ADM | 조직 관리 | 관리 > 조직 | NULLUS_ADM_010_xxx |
| | | 사용자 관리 | 관리 > 사용자 관리 | NULLUS_ADM_020_xxx |
| | | 클러스터 관리 | 관리 > 클러스터 관리 | NULLUS_ADM_030_xxx |
| 사용자 | USR | 홈/대시보드 | 홈 | NULLUS_USR_010_xxx |
| | | 인증 | 로그인, 로그아웃 | NULLUS_USR_020_xxx |
| | | 다국어 | UI 언어 전환 (en/ko) | NULLUS_USR_030_010 |
