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
    API 는 열려 있습니다.

!!! danger "development 모드는 모든 검사를 끕니다"
    `server.mode` 가 `development` 면 인증 미들웨어와 역할 검사가 **등록되지
    않습니다.** 네 그룹 모두 열립니다.

## 화면 권한과 API 권한이 다릅니다

사이드바와 라우트는 프런트엔드 쪽 장치이고, **실제 권한의 기준은 서버 그룹**
입니다. 어느 화면이 어느 API 에 막히는지는 [권한 표](rbac.md)에 있습니다.

대표적으로, `/stack/install` 과 `/cicd/developer-deploy` 는 화면 권한이 더
넓은데 화면을 채우는 조회 API 가 **admin 전용**이라, 열리기는 하지만 데이터가
비어 보입니다.

## 응답 형태

목록 엔드포인트는 대체로 `{items: [...], total: N}` 입니다.

!!! warning "질의 파라미터를 무시하는 목록이 있습니다"
    `GET /api/v1/stacks` 와 `GET /api/v1/cicd/pipelines` 는 `status`·`search` 같은
    값을 보내도 **읽지 않고** 조직의 전체 목록을 돌려줍니다. 걸러내기와 정렬은
    화면이 받아서 합니다. `page`·`page_size`·`sort` 를 읽는 엔드포인트는 없고,
    `limit`·`offset` 은 취약점 목록·감사 로그·백업 이력·스택 경보·알림 발송
    이력에서만 동작합니다.

## 건강 확인

```
GET /health
```

`status` · `db` · `version` 을 돌려줍니다. `db` 가 `connected` 여야 정상입니다.

## 전체 명세

엔드포인트별 상세는 저장소의
[API 설계 문서](https://github.com/cloud-nullus/nullus/blob/main/docs/20_%EC%95%84%ED%82%A4%ED%85%8D%EC%B2%98/Nullus_API_%EC%84%A4%EA%B3%84.md)에
있습니다.

그 문서는 설계안입니다. 지금 떠 있는 경로는 이 사이트의 기능 페이지에서 확인하세요
— 화면마다 "호출하는 API" 를 적어 두었습니다.
