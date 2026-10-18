# 조직

<span class="route">/admin/organization</span>
<span class="role role-admin">admin</span>

조직 정보와 그 조직이 쓸 수 있는 클러스터 범위를 관리합니다.
`/admin/organizations` 도 같은 화면입니다.

## 화면 구성

왼쪽에 조직 목록, 오른쪽에 상세 양식이 있습니다.

| 항목 | 저장되나 |
|------|---------|
| Organization Name | ✅ |
| Slug | ❌ — 아래 참고 |
| Domain | ✅ |
| Status (`active`·`inactive`·`suspended`) | ❌ |
| **Cluster Access Scope** (클러스터 체크박스 격자) | ✅ — 클러스터 **이름**으로 저장합니다 |

아래에 **Member Management** 표(이름·이메일·역할·상태·작업)와 **Add User**
(→ [사용자 관리](users.md)), **Invite Member** 창이 있습니다.
**New Organization** 창으로 조직을 새로 만들 수 있습니다.

!!! warning "Slug 와 Status 는 고쳐도 저장되지 않습니다"
    양식에서는 편집할 수 있게 보이지만, 수정 API 가 받는 항목은
    **`name` · `domain` · `clusterAccessScope` 셋뿐**입니다. Slug 와 Status 는
    전달되지 않습니다.

    Status 의 `suspended` 는 서버 쪽 값으로 아예 존재하지 않습니다
    (`active`·`inactive` 둘뿐). 그리고 **조직 상태를 읽어 접근을 막는 코드도
    없습니다** — 비활성으로 바꿔도 아무 일도 일어나지 않습니다.

!!! danger "저장이 실패해도 성공처럼 보입니다"
    수정 요청이 실패하면 화면이 조용히 **브라우저 안의 상태만** 바꿉니다.
    새로 고치면 값이 되돌아가 있습니다. 저장 후에는 한 번 새로 고쳐 확인하세요.

!!! note "기본 관리자 지정은 화면에 없습니다"
    데이터 모델에는 기본 관리자 항목이 있지만, 그것을 설정하는 UI 도 API 도
    없습니다.

## 조직은 어떻게 고르나

조회 API 는 조직을 이 순서로 결정합니다.

1. `X-Org-ID` 헤더
2. `?orgId` 질의 파라미터
3. 로그인한 사용자의 조직
4. **테이블의 첫 조직**

프런트엔드는 `X-Org-ID` 를 고정값으로 보냅니다
([관리 개요](index.md) 참고).

## 호출하는 API

| 동작 | 엔드포인트 |
|------|-----------|
| 조회 | `GET /api/v1/admin/organization` |
| 수정 | `PATCH /api/v1/admin/organization` |
| 생성 | `POST /api/v1/admin/orgs` — slug 형식·중복을 검사하고 감사 로그를 남깁니다 |
| 구성원 | `/api/v1/admin/organizations/{orgId}/members…` |
| 클러스터(접근 범위용) | `GET /api/v1/admin/clusters` |

## 리소스 프로파일

조직에는 스택 설치용 **리소스 프로파일**도 딸려 있습니다
(`/api/v1/admin/organization/resource-profiles`). 다만 이 화면이 아니라
[스택 설치](../stack/install.md) 위자드의 Resources 탭에서 저장·삭제합니다.
그래서 devops 가 쓰는 화면이 admin 전용 API 를 부르는 구조가 됩니다.
