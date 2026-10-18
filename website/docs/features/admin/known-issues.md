# 알려진 이슈

<span class="route">/admin/known-issues</span>
<span class="role role-admin">admin</span>

알려진 제약과 그 우회 방법을 보는 **읽기 전용** 표입니다.

![알려진 이슈 목록](../../assets/screens/admin-known-issues-light.png#only-light)
![알려진 이슈 목록](../../assets/screens/admin-known-issues-dark.png#only-dark)

## 표 컬럼

| 컬럼 | 내용 |
|------|------|
| ID | 이슈 식별자 |
| 심각도 | severity |
| 제목 | 제목과 설명 |
| 상태 | status |
| 우회 방법 | workaround |

만들기·수정·삭제 버튼은 없습니다.

## 내용은 어디서 오나

데이터베이스의 `known_issues` 테이블입니다
(`GET /api/v1/admin/known-issues` → `{items: […]}`).

테이블과 **최초 3건**은 마이그레이션
`db/migrations/000013_known_issues.up.sql` 이 넣습니다 — Helm 설치에 cluster-admin
필요, 대시보드 지표 지연, 인증서 자동 갱신 없음.

!!! warning "제품에서 목록을 고칠 수 없습니다"
    쓰기 경로가 **전혀 없습니다.** API 에 생성·수정·삭제가 없고 UI 에도 없습니다.
    항목을 추가하려면 새 마이그레이션을 쓰거나 DB 에 직접 넣어야 합니다.
