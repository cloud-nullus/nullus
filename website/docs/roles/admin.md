# Admin

로그인하면 **조직**(`/admin/organization`) 화면으로 들어갑니다. 사이드바 19개가
모두 보이고, `/api/v1/admin/**` 전체를 호출할 수 있는 유일한 역할입니다.

![admin 으로 로그인하면 열리는 조직 화면](../assets/screens/admin-organization-light.png#only-light)
![admin 으로 로그인하면 열리는 조직 화면](../assets/screens/admin-organization-dark.png#only-dark)

## 처음 세팅하는 순서

1. **[클러스터 관리](../features/admin/clusters.md)** 에서 대상 클러스터를
   등록합니다. kubeconfig 를 붙여넣고 **Verify Connection** 이 `connected` 가
   돼야 등록 버튼이 열립니다. — 가장 먼저 할 일입니다
2. **[조직](../features/admin/organization.md)** 에서 이름·도메인과
   **Cluster Access Scope** 를 맞춥니다
3. **[사용자 관리](../features/admin/users.md)** 에서 구성원을 추가하고 역할을
   줍니다
4. **[알림 채널](../features/admin/notifications.md)** 을 등록하고 **테스트
   전송**으로 확인합니다
5. **[백업](../features/admin/backup.md)** 을 한 번 돌려 목적지 설정이 맞는지
   봅니다

그다음은 devops 와 같은 길입니다 — [DevOps](devops.md) 를 보세요.

## Admin 만 할 수 있는 것

| 일 | 화면 |
|----|------|
| 조직 정보·클러스터 접근 범위 | [조직](../features/admin/organization.md) |
| 구성원 추가·역할 부여·비활성화 | [사용자 관리](../features/admin/users.md) |
| 클러스터 등록·검증 | [클러스터 관리](../features/admin/clusters.md) |
| 호환성 매트릭스 관리 | [스택 버전 관리](../features/stack/versions.md#스택-버전-관리) |
| 토큰 출처 확인 | [토큰 관리](../features/admin/tokens.md) |
| 백업·복구 | [백업·복구](../features/admin/backup.md) |
| 알림 채널 | [알림 채널](../features/admin/notifications.md) |
| 스택 템플릿 만들기·수정·삭제 | [스택 템플릿](../features/stack/templates.md) |
| CI/CD 템플릿 만들기·수정·삭제 | [CI/CD 템플릿](../features/cicd/templates.md) |
| 모니터링 임베드 탭 관리 | [모니터링 대시보드](../features/observability/monitoring.md#임베드-탭-manage-tabs) |

## 운영 전에 확인할 것

!!! danger "인증 모드"
    기본값 `session` 은 **서버가 요청 헤더의 역할 주장을 그대로 믿습니다.**
    운영에서는 `NULLUS_AUTH_MODE=oidc` 와 프런트엔드의 `authMode=oidc` 를 함께
    맞추세요. 둘이 달라도 안 됩니다.

!!! danger "서버 모드"
    `server.mode=development` 면 인증과 역할 검사가 **전부 꺼집니다.**
    차트 기본값은 `production` 입니다.

!!! warning "지금 동작하지 않는 관리 기능"
    각 문서에 자세히 적어 두었지만, 운영 계획을 세울 때 먼저 알아 두시면
    좋은 것들입니다.

    | 기능 | 상태 |
    |------|------|
    | 초대 링크 | 서버가 빈 응답을 주는 자리막음. 수락 경로도 없음 |
    | Role Permissions 탭 | 브라우저 localStorage 에만 저장. 효력 없음 |
    | 조직 상태(Status) | 저장되지 않고, 읽어서 막는 코드도 없음 |
    | 호환성 매트릭스 CRUD | 라우트 미등록 — 404 |
    | 토큰 회전·승인·일시정지 버튼 | 상태 글자만 바꿈. 실제 재발급은 서버 스케줄러가 함 |
    | 비밀번호 로그인 | 어떤 계정으로도 성공할 수 없음 |
    | 복구 | 화면 없음. API 전용 |
    | 감사 로그 조회 | API 는 있으나 보는 화면이 없음 |
