# OSS 기본 리소스

<span class="route">/stack/oss-resource-default</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

도구마다 CPU·메모리 기본값을 정해 두는 화면입니다. 설치 위자드의 Resources 탭이
여기 값을 **추천값**으로 씁니다.

!!! note "화면 안 제목은 영문입니다"
    사이드바 라벨은 "OSS 기본 리소스" 인데, 화면 제목·브레드크럼·컬럼 머리글·버튼은
    `OSS Default Resource` 처럼 영문이 박혀 있습니다.

## 쓰는 법

1. `Search by category / tool / name...` 으로 도구를 찾습니다.
2. 행의 값을 바로 고치고 그 행의 **Save** 를 누릅니다.
3. 새 도구를 넣으려면 맨 아래 빈 행을 채우고 **Register** 를 누릅니다.
   `Tool Key` 와 `Display Name` 은 필수입니다.

저장은 `tool_key` 기준 **멱등 upsert** 입니다 — 같은 키로 다시 보내면 덮어씁니다.
화면 위 안내 배너가 그 계약을 그대로 적어 두고 있습니다.

## 표 컬럼

Category · Tool Key · Display Name · CPU Request · CPU Limit ·
Memory Req (Gi) · Memory Limit (Gi) · Action

!!! warning "스토리지는 화면에서 고칠 수 없습니다"
    저장 요청에 `storage_request_gi` · `storage_limit_gi` 가 함께 실려 가지만
    **표에 스토리지 컬럼이 없습니다.** 그래서 기존 행은 서버에서 받아 온 값이
    그대로 되돌아가고, 새로 등록하는 행은 항상 0 으로 들어갑니다.

## 호출하는 API

| 동작 | 엔드포인트 |
|------|-----------|
| 조회 | `GET /api/v1/stacks/resource-defaults` |
| 저장(upsert) | `POST /api/v1/stacks/resource-defaults` |
