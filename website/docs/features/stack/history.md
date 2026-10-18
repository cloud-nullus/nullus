# 스택 이력

<span class="route">/stack/history/:stackId?</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

스택 설정이 언제 누구에 의해 어떻게 바뀌었는지 보고, 필요하면 되돌리는
화면입니다.

`/stack/history` 로 들어오면 보이는 첫 스택으로 자동 이동합니다. 즉 실제로
머무는 주소에는 스택 ID 가 붙습니다.

![스택의 버전별 변경 이력](../../assets/screens/stack-history-light.png#only-light)
![스택의 버전별 변경 이력](../../assets/screens/stack-history-dark.png#only-dark)

## 표 컬럼

| 컬럼 | 내용 |
|------|------|
| (펼치기) | 행을 펼치는 버튼 |
| **버전** | 현재 버전에는 `현재` 배지가 붙습니다 |
| **변경자** | 바꾼 사람 |
| **변경 시간** | |
| **변경 사유** | |
| **작업** | **로그**(설치 로그로 이동) · **롤백**(현재 버전에는 안 나옴) |

툴바에 클러스터 선택과 `변경자 / 변경 사유 검색...` 이 있습니다. 위쪽 **스택**
선택으로 다른 스택의 이력으로 넘어갑니다.

행을 고르면 오른쪽에 그 시점의 **설정 스냅샷**이 경로→값 형태로 펼쳐집니다.

## 버전 비교

위쪽 **버전 비교** 를 누르면 버전 A·B 를 골라 차이를 봅니다
(`GET /api/v1/stacks/{id}/history/diff?versionA=&versionB=`).

## 롤백

!!! warning "롤백 버튼은 지금 동작하지 않습니다"
    프런트엔드가 `{version, preservePVC}` 를 보내는데 서버는 **`versionId`** 를
    요구합니다. 그래서 요청이 `ROLLBACK_REQUEST_INVALID / "versionId is required"`
    로 400 을 받습니다.

    덤으로, 롤백 창의 **데이터 보존 옵션**(Safe Mode — 데이터 보존 / Clean Mode —
    볼륨 삭제)과 Clean Mode 의 `DELETE` 확인 입력은 **서버에 전달되지
    않습니다.** `preservePVC` 는 요청 구조체에 아예 없습니다. 서버의 롤백 구현은
    대상 버전의 설정을 스택에 복사하고 이력 한 줄을 남기는 것까지입니다.

    고쳐지기 전까지는 [스택 목록](list.md)의 **설정** 탭에서 values 를 직접
    되돌리는 쪽이 확실합니다.

## 호출하는 API

| 동작 | 엔드포인트 |
|------|-----------|
| 이력 | `GET /api/v1/stacks/{stackId}/history` |
| 비교 | `GET /api/v1/stacks/{id}/history/diff?versionA=&versionB=` |
| 롤백 | `POST /api/v1/stacks/{stackId}/rollback` (본문에 `versionId` 필수) |
