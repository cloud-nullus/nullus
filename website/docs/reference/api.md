# API

기준 경로는 **`/api/v1`** 입니다. 프런트엔드는 `/api/v1` 을 기본 주소로 두고
상대 경로로 호출합니다.

## 그룹과 권한

엔드포인트는 네 그룹으로 나뉘고, 그룹 단위로 역할 검사가 걸립니다.

| 그룹 | 경로 | 허용 역할 |
|------|------|-----------|
| 관리 | `/api/v1/admin/**` | **admin** |
| 스택 | `/api/v1/stacks/**` | admin · devops |
| CI/CD | `/api/v1/cicd/**` | admin · devops · developer |
| 관측성 | `/api/v1/observability/**` | **역할 검사 없음** — 인증만 |
| 로그인 | `POST /api/v1/auth/login` | 인증 전 |

모든 그룹에 사용자별 요청 제한이 함께 걸립니다.

!!! danger "관측성 그룹에는 역할 검사가 없습니다"
    `/api/v1/observability/**` 는 인증만 통과하면 **누구나** 호출할 수 있습니다.
    여기에는 알림 규칙 생성·수정·삭제도 포함됩니다 — 그 화면은 UI 에서 내렸지만
    API 는 열려 있습니다. 설계 문서가 "역할 검사를 추가한다" 고 적어 둔 항목이
    아직 적용되지 않은 상태입니다.

!!! danger "development 모드는 모든 검사를 끕니다"
    `server.mode` 가 `development` 면 인증 미들웨어와 역할 검사가 **등록되지
    않습니다.** 네 그룹 모두 열립니다.

## 화면 권한과 API 권한이 다릅니다

사이드바와 라우트는 프런트엔드 쪽 장치이고, **실제 권한의 기준은 서버 그룹**
입니다. 둘이 어긋나는 지점이 여러 곳 있어 따로 정리했습니다 —
[권한 표](rbac.md)를 보세요.

대표적으로, `/stack/install` 과 `/cicd/developer-deploy` 는 화면 권한이 더
넓은데 화면을 채우는 조회 API 가 **admin 전용**이라, 열리기는 하지만 데이터가
비어 보입니다.

## 응답 형태

목록 엔드포인트는 대체로 `{items: [...], total: N}` 입니다.

!!! warning "질의 파라미터를 무시하는 목록이 있습니다"
    `GET /api/v1/stacks` 는 `status`·`search`·`include_deleted` 를 보내도
    **읽지 않습니다.** 페이지네이션·필터·정렬을 지원한다고 적은 API 설계 문서는
    이 점에서 낡았습니다. `GET /api/v1/cicd/pipelines` 도 `status`·`search` 를
    보지 않습니다.

## 건강 확인

```
GET /health
```

`status` · `db` · `version` 을 돌려줍니다. `db` 가 `connected` 여야 정상입니다.

## 전체 명세

엔드포인트별 상세는 저장소의
[API 설계 문서](https://github.com/cloud-nullus/nullus/blob/main/docs/20_%EC%95%84%ED%82%A4%ED%85%8D%EC%B2%98/Nullus_API_%EC%84%A4%EA%B3%84.md)에
있습니다.

!!! warning "그 문서에는 코드와 다른 부분이 있습니다"
    확인된 것만 적어 둡니다.

    | 문서가 적은 것 | 실제 |
    |---------------|------|
    | `PUT /api/v1/stacks/:id` | 그런 라우트 없음. `PATCH …/{id}/tools` 와 `POST …/{id}/config` |
    | `GET …/history/:versionId/diff` | `GET …/history/diff?versionA=&versionB=` (질의 파라미터) |
    | `POST …/rollback/:versionId` | `POST …/rollback`, 본문에 `versionId` 필수 |
    | `/api/v1/deployments/stacks/…` | 그런 접두사 없음. `/api/v1/stacks/{id}/deploy` 등 |
    | `/api/v1/pipelines/…` | `/api/v1/cicd/…` |
    | `/api/v1/monitoring/…` | `/api/v1/observability/…` |
    | `/api/v1/clusters/…`, `PUT /clusters/:id` | `/api/v1/admin/clusters/…`, 수정은 **PATCH** |
    | `PUT /orgs/:orgId/status` | 없음 — 조직 상태는 저장되지 않습니다 |
    | 스택 라우트 40개 | 이미지 스캔 관련 5개가 목록에서 빠져 있습니다 |
