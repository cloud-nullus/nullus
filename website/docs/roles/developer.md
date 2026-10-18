# Developer

로그인하면 **앱 배포**(`/cicd/developer-deploy`) 화면으로 들어갑니다.
사이드바에 보이는 것은 **4개**입니다.

| 그룹 | 항목 |
|------|------|
| CI/CD | CI/CD 목록 · CI/CD 이력 |
| 관측성 | 모니터링 대시보드 · 알림 이력 |

## 하는 일

1. **[앱 배포](../features/cicd/app-deploy.md)** 에서 앱 이름·저장소·클러스터·
   네임스페이스·리소스를 채우고 **Execute**
2. **[파이프라인 로그](../features/cicd/list.md#파이프라인-로그)** 에서 진행을
   봅니다
3. **[CI/CD 목록](../features/cicd/list.md)** 에서 상태와 취약점·SAST 결과를
   확인합니다
4. **[CI/CD 이력](../features/cicd/history.md)** 에서 과거 배포를 봅니다

## 알아 둘 것

!!! danger "첫 화면을 끝까지 채우지 못할 수 있습니다"
    앱 배포 화면의 **Stack 선택**과 **Cluster · Namespace 선택**이
    admin·devops 전용 조회 API 에서 옵니다. 인증을 켠 환경에서 developer 로
    들어가면 이 선택들이 **403 으로 비어** 생성까지 가지 못합니다.

    파이프라인을 만들고 배포하는 **권한 자체는** 서버가 developer 에게
    허용합니다 — 막히는 것은 화면을 채우는 데 필요한 조회뿐입니다. 당장은
    devops 나 admin 에게 파이프라인 생성을 부탁하고, 만들어진 파이프라인을
    CI/CD 목록에서 **Execute** 로 실행하는 쪽이 확실합니다.

!!! warning "모니터링 화면도 같은 이유로 비어 보입니다"
    모니터링 대시보드와 알림 이력의 클러스터·스택 필터가 같은 API 를
    씁니다. 경보 목록 자체는 역할 제한 없이 받을 수 있습니다.

!!! note "메뉴에 없는 화면들을 쓰게 됩니다"
    앱 배포 · 파이프라인 로그 는 사이드바 항목이 없습니다. 홈 화면의 시작
    버튼이나 CI/CD 목록의 버튼으로 들어가세요.

    CI/CD 템플릿과 골든패스는 라우트가 developer 를 허용하지만 사이드바에
    없습니다. 홈에서 한 번 들어가면 메뉴로 다시 찾아갈 수 없습니다.

## 주소로는 열리지만 메뉴에 없는 화면

사이드바에서 가려졌을 뿐 라우트에 역할 제한이 없어서, 주소를 직접 치면 열리는
화면들입니다. 다만 데이터 API 가 막혀 있어 대체로 비어 보입니다.

`/stack/templates` · `/stack/list` · `/stack/history` · `/stack/version` ·
`/cicd/templates` · `/cicd/golden-paths`

자세한 대조는 [권한 표](../reference/rbac.md)에 있습니다.
