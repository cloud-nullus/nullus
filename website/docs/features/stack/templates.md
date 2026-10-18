# 스택 템플릿

<span class="route">/stack/templates</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

검증된 OSS 조합을 카드로 늘어놓고, 하나를 골라 설치로 들어가는 화면입니다.

![검증된 조합을 카드로 고르는 스택 템플릿 화면](../../assets/screens/stack-templates-light.png#only-light)
![검증된 조합을 카드로 고르는 스택 템플릿 화면](../../assets/screens/stack-templates-dark.png#only-dark)

## 쓰는 법

1. 카드를 훑어보거나 `템플릿 검색...` 으로 좁힙니다.
2. **상세 보기** 로 포함된 도구와 버전을 확인합니다.
3. **이 템플릿 사용** 을 누르면 설치 화면(`/stack/install?template=<id>`)으로
   넘어가면서 템플릿 내용이 미리 채워집니다.

빈 템플릿(`empty-template-v1`)을 고르면 버튼 이름이 **빈 기반에서 시작** 으로
바뀝니다.

## 카드에 적히는 것

- 설치 시간
- 권장 사용
- 최소 리소스
- 포함된 도구 — 앞의 4개와 `+N`

## 버튼과 권한

| 버튼 | 누가 볼 수 있나 |
|------|----------------|
| 상세 보기 | 모두 |
| 이 템플릿 사용 / 빈 기반에서 시작 | 모두 |
| 복제 | 모두 |
| **템플릿 생성** | admin |
| **수정** | admin |
| **삭제** | admin |

템플릿을 만들거나 고칠 때 입력하는 것은 템플릿 ID · 이름 · 설명 · 규모
프로파일(Local · Startup · Standard · Enterprise) · 섹션별 도구 구성입니다.

## 호출하는 API

| 동작 | 엔드포인트 |
|------|-----------|
| 목록 | `GET /api/v1/stacks/templates` |
| 생성 | `POST /api/v1/stacks/templates` |
| 수정 | `PUT /api/v1/stacks/templates/{id}` |
| 삭제 | `DELETE /api/v1/stacks/templates/{id}` |

!!! note "developer 도 주소로는 열립니다"
    이 화면은 사이드바에서 developer 에게 가려지지만 라우트에는 역할 제한이
    없습니다. 다만 API 가 admin·devops 전용이라 목록이 비어 보입니다.
