# CI/CD 템플릿

<span class="route">/cicd/templates</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

파이프라인 기본 템플릿을 고르는 화면입니다.

![파이프라인 템플릿 목록](../../assets/screens/cicd-templates-light.png#only-light)
![파이프라인 템플릿 목록](../../assets/screens/cicd-templates-dark.png#only-dark)

## 쓰는 법

1. 카드를 훑거나 검색으로 좁힙니다(이름·설명 대상).
2. 카드의 **Use Base Template** 을 누릅니다.
3. [앱 배포](app-deploy.md) 화면(`/cicd/developer-deploy?template=…`)으로 넘어가
   값이 미리 채워집니다.

!!! note "`/cicd/create` 로 가지 않습니다"
    파이프라인 생성 화면(`/cicd/create`)이 따로 있지만, 이 버튼은 거기로 가지
    않습니다. 자세한 사정은 [CI/CD 목록](list.md#파이프라인-생성-화면)에 적어
    두었습니다.

## 카드 구성

템플릿은 **CI/CD Type** 세 묶음으로 나뉘어 보입니다 — `Default`(Deployment ·
Service · Ingress) · `Helm`(Helm 차트 릴리스) · `Cronjob/Job`. 분류는 ID·이름·
stages 문자열을 보고 화면에서 정합니다.

카드에는 이름, 설명, CI/CD Type, Application Type, **Capabilities** 배지
(CI · CD · Test · Security), 만든 사람이 나옵니다.

Nullus 샘플 앱 템플릿 두 개가 항상 맨 앞으로 정렬됩니다.

!!! warning "목록이 서버 응답과 다릅니다"
    화면이 두 가지를 손봅니다.

    - `web-frontend-v1` 은 **목록에서 제거**합니다
    - `web-backend-v1` 은 이름을 **`User Custom Pipeline`** 으로 바꿔 보여 줍니다

    설명 문구도 화면 안에 한국어·영어로 박아 둔 표에서 가져옵니다 — 서버의
    `description` 이 아닙니다.

## 만들기·수정·삭제 (admin)

**Create Template · Edit · Delete** 는 admin 에게만 보입니다.

입력 항목은 Template ID(수정 시 잠김) · Name · Description ·
Application Type(`web` · `backend` · `batch`) · Capabilities 체크박스
(CI · CD · Test · Security)입니다. 이름은 필수이고 Capability 하나 이상을 골라야
합니다.

!!! warning "Capabilities 가 stages 칸에 저장됩니다"
    창이 저장할 때 체크한 Capability 라벨을 그대로 서버의 `stages` 컬럼에
    넣습니다. 즉 admin 이 만든 템플릿은 `stages = ["CI","CD"]` 처럼 **실제 단계
    이름이 아닌 값**을 갖습니다. 그 값이 나중에 [CI/CD 목록](list.md)의 이력 탭에서
    단계 목록으로 그려지므로, 단계가 이상하게 보일 수 있습니다.

## 호출하는 API

| 동작 | 엔드포인트 |
|------|-----------|
| 목록 | `GET /api/v1/cicd/templates` |
| 생성 | `POST /api/v1/cicd/templates` |
| 수정 | `PUT /api/v1/cicd/templates/{id}` (보내지 않은 항목은 기존 값 유지) |
| 삭제 | `DELETE /api/v1/cicd/templates/{id}` |

!!! warning "developer 는 메뉴로 올 수 없습니다"
    라우트는 developer 를 허용하지만 사이드바에는 항목이 없습니다. 홈 화면의
    시작 버튼으로 들어올 수는 있어서, 한 번 벗어나면 메뉴로 다시 찾아갈 수
    없습니다.
