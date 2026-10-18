# 스택 설치

<span class="route">/stack/install</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

템플릿에서 가져온 구성을 다듬고 클러스터에 배포하는 화면입니다. 사이드바에는
없고, [스택 템플릿](templates.md)에서 **이 템플릿 사용** 을 누르면 들어옵니다.

!!! note "단계가 아니라 탭입니다"
    기획 문서 몇 곳이 "5단계 워크플로우" 로 적고 있지만 구현에는 단계가
    없습니다. 순서 없이 눌러 다니는 **탭 10개**이고 진행 표시(스테퍼)도
    없습니다. 아무 탭에서나 **배포**를 누를 수 있습니다.

![탭으로 구성을 채우는 설치 화면](../../assets/screens/stack-install-light.png#only-light)
![탭으로 구성을 채우는 설치 화면](../../assets/screens/stack-install-dark.png#only-dark)

## 탭이 아니라 먼저 채우는 것 — 머리 카드

탭 위에 항상 붙어 있는 카드입니다. 여기가 실제 필수 입력입니다.

| 항목 | 규칙 |
|------|------|
| **스택 이름** | 필수. 2~50자, 영문·숫자·하이픈(`^[a-zA-Z0-9-]+$`). 같은 클러스터에 같은 이름이 있으면 거부합니다 |
| **접근 도메인** | 기본값 `{스택이름}.internal`. 각 도구는 `{도구}.{스택}.internal` 로 접근합니다 |
| Access Domain TLS 적용 (cert-manager) | 체크하면 **TLS Secret 이름**·**네임스페이스**·**Issuer 이름** 세 개가 필수가 됩니다 |
| **대상 클러스터** | 필수. 연결 상태가 함께 표시됩니다 |
| **네임스페이스** | `기본 (nullus)` · 기존 네임스페이스 · `새 네임스페이스 생성...` |

오른쪽에는 **Configuration Summary** 패널이 고정돼 선택한 값을 계속 보여 줍니다.
위쪽 버튼은 **임시 저장** 과 **배포** 둘입니다.

## 탭 10개

| # | 탭 | 고르는 것 |
|---|-----|----------|
| 1 | **Authentication** | 없음 또는 OpenBao |
| 2 | **Artifacts** | 소스 저장소 · 컨테이너 레지스트리 · 패키지 레지스트리. GitHub 을 고르면 Organization·API 주소·Personal Access Token 입력란이 열립니다 |
| 3 | **CI/CD** | CI/CD 플랫폼 · CD 도구 |
| 4 | **Observability** | 시각화(복수) · 메트릭 · 로그 · 트레이스 · Exporter/Agent |
| 5 | **Security** | 이미지 스캐너 · 정적 분석(SAST) · 의존성 취약점(SCA) |
| 6 | **Storage** | Storage Plan(미선택 / 기존 연결 / 통합 생성) · StorageClass · Database · Object Storage |
| 7 | **Resources** | Sizing Profile(Local·Startup·Standard·Enterprise + 조직 프로파일) · 도구별 CPU·메모리·스토리지 적용값 |
| 8 | **YAML View** | Gateway 와 도구별 매니페스트를 Monaco 편집기에서 직접 수정 |
| 9 | **Preview Deploy Script** | 생성될 `<스택>-deploy.sh` 를 읽기 전용으로 확인 |
| 10 | **Dry Run** | 검사 10개를 돌려 PASS/WARN/FAIL 을 봅니다 |

**로그 설정은 Observability 탭 안에 있습니다** — 별도 Logging 탭은 없습니다.

### 탭 7개는 아무 때나, 3개는 조건부

**YAML View · Preview Deploy Script · Dry Run** 으로 넘어가려면 스택 이름(중복
아님) · 대상 클러스터 · (새 네임스페이스 모드일 때) 네임스페이스가 채워져 있어야
합니다. 아니면 Artifacts 탭으로 되돌립니다.

### Dry Run 이 보는 10가지

스택 이름 형식 · 클러스터 선택 · 네임스페이스 유효성 · 접근 도메인 규칙 ·
Gateway TLS · 설치 파일 생성 상태 · YAML/values 검증 · 리소스 하한 ·
번들 OSS 버전 충돌 · 스토리지 플랜.

`Run Dry Run` 버튼은 "마지막 실행 시각"만 찍습니다 — 검사 자체는 값이 바뀔 때마다
다시 계산됩니다.

## 배포

**배포** 버튼은 아래 중 하나라도 걸리면 눌리지 않습니다.

- 스택 이름이 비었거나 2자 미만, 또는 중복
- 클러스터 미선택
- 새 네임스페이스 모드인데 네임스페이스가 빔
- 매니페스트 검증 오류
- 클라이언트 호환성 검사가 `fail`, 또는 `warn` 인데 확인하지 않음

누르면 `POST /api/v1/stacks` 로 스택을 만들고(같은 클러스터·이름의 진행 중이거나
`pending` 행이 있으면 재사용), `POST /api/v1/stacks/{id}/deploy` 로 배포를 시작한
뒤 `/stack/deploy/{id}` 로 이동합니다.

**임시 저장** 은 `POST /api/v1/stacks/draft` 입니다.

## 배포 진행 화면

<span class="route">/stack/deploy/:id</span> · <span class="route">/stack/logs/:deploymentId</span>

두 경로가 같은 화면입니다. 단계 타임라인과 진행 바, 실패 시 마지막 오류,
경고·오류 목록, **Raw Logs** 접이식 패널, **Continue** 버튼, 그리고 네임스페이스의
파드를 실시간으로 보여 주는 패널이 있습니다.

로그와 파드는 WebSocket(`/ws/deployments/{id}/logs`, `/ws/deployments/{id}/pods`)으로
들어옵니다.

!!! note "WebSocket 은 oidc 모드에서만 인증됩니다"
    `auth.mode` 가 `oidc` 가 아니면 소켓에 인증 미들웨어가 붙지 않습니다.
    브라우저 WebSocket 에는 `X-User-*` 헤더를 붙일 수 없어서입니다.

## 알아 두면 좋은 것

!!! warning "서버 Pre-Deploy Gate 는 동작하지 않습니다"
    화면에 "서버 Pre-Deploy Gate" 패널과 서버 경고 확인 체크박스가 있지만,
    배포 핸들러가 호환성 검증 없이 조립돼 있어 서버 검사가 바로 통과(빠져나감)
    합니다. 실제로 도는 것은 **클라이언트 쪽 매트릭스·아키텍처 검사**뿐입니다.

!!! warning "리소스 예상량은 서버에서 계산하지 않습니다"
    `POST /api/v1/stacks/estimate` 엔드포인트가 있지만 **아무도 부르지
    않습니다.** Resources 탭의 숫자는 `resource-defaults` 를 받아 브라우저에서
    계산한 값입니다.

내보내기·가져오기 관련 주의는 [스택 목록](list.md#내보내기가져오기)에 적어
두었습니다.
