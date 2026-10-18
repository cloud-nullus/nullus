# 스택 목록

<span class="route">/stack/list</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

설치된 스택을 관리하는 화면입니다. 왼쪽이 목록, 오른쪽이 상세 패널입니다.

## 표와 필터

표 컬럼은 **스택 이름**(아래 줄에 클러스터 이름이 같이 들어갑니다)과 **상태**,
**생성일** 입니다. 클러스터는 별도 컬럼이 아닙니다.

툴바에는 상태 선택(전체·실행 중·완료·삭제 중·대기 중·실패·취소됨), 클러스터 선택,
`스택 검색...` 이 있습니다.

!!! note "검색·필터·정렬은 전부 브라우저에서 합니다"
    `GET /api/v1/stacks` 는 **질의 파라미터를 모두 무시합니다.** 프런트엔드가
    `status`·`search`·`include_deleted` 를 보내지만 서버는 조직 ID 만 읽습니다.
    응답도 `{items, total}` 형태입니다. 스택 수가 많아지면 느려질 수 있습니다.

위쪽 버튼은 **가져오기** 와 **새 스택** 입니다. `새 스택` 은 설치 화면이 아니라
[스택 템플릿](templates.md)으로 보냅니다. 키보드 `n` 은 바로 `/stack/install` 로
갑니다.

## 상세 패널 탭

| 탭 | 내용 |
|----|------|
| **정보** | Gateway 포트포워드 명령 복사, `/etc/hosts` 매핑 복사, **도구 추가**, **재시도**, **삭제**, **내보내기**, **접속정보 조회** |
| **워크로드** | 네임스페이스의 파드·워크로드 상태 |
| **모니터링** | 스택이 건강할 때만 나타납니다 |
| **설정** | 릴리스를 골라 values 를 Monaco 편집기에서 수정 → **변경 미리보기** → **적용** → **되돌리기** |
| **이미지 취약점** | 이미지 스캔 결과와 취약점 목록 |
| **이력** | 설치 로그 · 변경 이력 · 재배포 기록으로 가는 버튼 |
| **버전 업그레이드** | 아래 참고 |

!!! warning "버전 업그레이드 탭은 아직 껍데기입니다"
    표에 나오는 항목(GitLab v16.7→v16.9 등)과 "3 updates available" 배지는
    **화면에 박아 둔 고정값**입니다. 업그레이드 버튼은 "개발중인 기능입니다"
    토스트만 띄웁니다.

## 도구 추가

<span class="route">/stack/:id/add-tools</span>

상세 패널의 **도구 추가** 로 들어갑니다. 여기는 실제로 **3단계**이고 탭 이름에
번호가 붙어 있습니다.

1. **카테고리 선택** — 아티팩트 · 파이프라인 · 관측성 · 로깅. 이미 설치된
   카테고리는 `설치됨` 배지와 함께 비활성화됩니다
2. **도구 설정** — 고른 카테고리마다 도구와 버전을 지정
3. **검토 및 저장** — 표로 확인하고 **확인 및 저장**

저장하면 "도구 설정이 저장되었습니다 (배포는 별도로 진행해야 합니다)" 토스트가
뜨고 목록으로 돌아갑니다. **설정만 저장하고 배포하지는 않습니다**
(`PATCH /api/v1/stacks/{stackId}/tools`).

## 재배포 기록

<span class="route">/stack/deployments/:deploymentId/retry-history</span>

시각 · 결과 · 이슈 컬럼으로 재시도 내역을 봅니다
(`GET /api/v1/stacks/{id}/retry-history`). 감사 로그에서 `retry` 동작만 걸러
보여 주며, 기록이 없으면 빈 목록을 줍니다.

## 내보내기·가져오기

- 내보내기 — `GET /api/v1/stacks/{id}/export`, 파일 이름과 형식을 고릅니다
- 가져오기 — 파일을 고르면 `POST /api/v1/stacks/import/preview` 로 차이를 보여
  주고, 확인하면 `POST /api/v1/stacks/import` 로 반영합니다

!!! danger "가져오기는 기존 스택을 덮어쓸 수 있습니다"
    `replace_existing` 옵션이 있습니다. 미리보기에서 바뀌는 내용을 반드시
    확인하세요.

## 호출하는 API

`GET /api/v1/stacks` · `DELETE /api/v1/stacks/{stackId}` ·
`GET …/{stackId}/history` · `/monitoring` · `/workloads` · `/connection-info` ·
`/image-scans` · `/image-scans/vulnerabilities` · `/releases` ·
`/releases/{name}/values`(GET·PUT) · `/values/preview` ·
`POST …/{id}/retry` · `GET …/{id}/export` · `POST /api/v1/stacks/import[/preview]`
