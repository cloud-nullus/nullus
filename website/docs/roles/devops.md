# DevOps

로그인하면 **스택 템플릿**(`/stack/templates`) 화면으로 들어갑니다. 사이드바에
11개가 보입니다 — 관리 그룹과 스택 버전 관리만 가려집니다.

![devops 로 로그인하면 열리는 스택 템플릿 화면](../assets/screens/stack-templates-light.png#only-light)
![devops 로 로그인하면 열리는 스택 템플릿 화면](../assets/screens/stack-templates-dark.png#only-dark)

## 하는 일

```mermaid
graph LR
  T[스택 템플릿] --> I[스택 설치] --> D[배포 진행] --> L[스택 목록]
  L --> C[CI/CD 템플릿] --> P[앱 배포]
```

1. **[스택 템플릿](../features/stack/templates.md)** 에서 조합을 고르고
   `이 템플릿 사용`
2. **[스택 설치](../features/stack/install.md)** 에서 스택 이름·클러스터·
   네임스페이스를 채우고 탭을 다듬습니다. **Dry Run** 으로 검사 10개를 돌려
   보세요
3. `배포` → 배포 진행 화면에서 로그를 봅니다
4. **[스택 목록](../features/stack/list.md)** 에서 상태·워크로드·취약점을
   확인하고, 필요하면 **설정** 탭에서 values 를 고칩니다
5. **[CI/CD 템플릿](../features/cicd/templates.md)** → **[앱 배포](../features/cicd/app-deploy.md)**
   로 파이프라인을 만듭니다
6. **[모니터링 대시보드](../features/observability/monitoring.md)** 와
   **[알림 이력](../features/observability/alert-history.md)** 로 지켜봅니다

## 볼 수 있는 메뉴

| 그룹 | 항목 |
|------|------|
| 데브섹옵스 스택 | 스택 템플릿 · 스택 목록 · 스택 이력 · 스택 버전 · OSS 기본 리소스 (5개) |
| CI/CD | CI/CD 템플릿 · 골든패스 · CI/CD 목록 · CI/CD 이력 (4개) |
| 관측성 | 모니터링 대시보드 · 알림 이력 (2개) |

스택 템플릿의 **템플릿 생성·수정·삭제** 버튼은 admin 에게만 보입니다.
호환성 매트릭스를 고치는 **스택 버전 관리** 화면도 admin 전용입니다.

## 막히는 지점

!!! warning "설치 위자드의 드롭다운이 비어 있을 수 있습니다"
    `/stack/install` 은 devops 에게 열려 있지만, 화면이 채워야 할 네 가지가
    **admin 전용 API** 에서 옵니다 — 대상 클러스터 · 네임스페이스 ·
    StorageClass · 조직 리소스 프로파일.

    인증을 켠 환경에서는 이 선택들이 403 으로 비어 보일 것으로 보입니다.
    그러면 admin 에게 클러스터 등록과 리소스 프로파일 저장을 부탁해야 합니다.

!!! warning "지금 동작하지 않는 것"
    | 기능 | 상태 | 대안 |
    |------|------|------|
    | 스택 이력의 **롤백** | 요청 형식이 어긋나 400 | [스택 목록](../features/stack/list.md) 설정 탭에서 values 직접 수정 |
    | 스택 버전의 **현재 스택 검증** | 서버 조립 누락으로 400 | 설치 위자드의 Dry Run |
    | 스택 목록의 **버전 업그레이드** 탭 | 고정값 표시, 버튼은 토스트만 | — |
    | CI/CD 목록의 **상태 필터** | 값이 맞지 않아 표가 비워짐 | 검색창 사용 |
    | Execute 창의 빌드·배포 설정 | 전송되지 않음 | — |
    | 서버 Pre-Deploy Gate | 비활성 | 클라이언트 검사와 Dry Run |
