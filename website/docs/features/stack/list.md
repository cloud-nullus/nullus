# 스택 목록

<span class="route">/stack/list</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

설치된 스택을 관리하는 화면입니다. 왼쪽이 목록, 오른쪽이 상세 패널입니다.

![왼쪽 목록에서 고르면 오른쪽에 상세가 열리는 스택 목록](../../assets/screens/stack-list-light.png#only-light)
![왼쪽 목록에서 고르면 오른쪽에 상세가 열리는 스택 목록](../../assets/screens/stack-list-dark.png#only-dark)

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
| **이미지 취약점** | 스택이 설치한 이미지의 스캔 결과와 취약점 목록. 클러스터 밖에 열린 이미지(게이트웨이 HTTPRoute, LoadBalancer · NodePort Service)에는 **외부 노출** 표시가 붙습니다(마우스를 올리면 호스트). 스택이 쓰는 클러스터 공용 구성요소(cert-manager · metrics-server · External Secrets)도 함께 스캔하고 **공용** 표시를 붙입니다 — 클러스터에 원래 있던 것을 재사용했어도 스택이 쓰므로 넣습니다. 폐쇄망(에어갭) 설치도 스캔합니다 — 이미지는 내부 레지스트리에서 받고 취약점 DB 는 번들과 함께 반입한 것을 쓰므로, 반입 뒤에 공개된 CVE 는 다음 반입 전까지 보이지 않는다는 안내가 붙습니다. 새로 발견된 "수정판 있는 CRITICAL" 은 [알림](../admin/notifications.md) `stack_cve_found` 로도 갑니다. 등급과 대응 기한은 [설치 OSS CVE 대응](cve-response.md) |
| **이력** | 설치 로그 · 변경 이력 · 재배포 기록으로 가는 버튼 |
| **버전 업그레이드** | 아래 참고 |

!!! note "버전 업그레이드는 지금 Argo CD 만 됩니다"
    클러스터에 실제로 설치된 Argo CD 릴리스와, 실클러스터 리허설을 거쳐 `verified` 로
    등록된 경로(출발 차트 → 대상 차트)만 보여 줍니다. 경로가 없으면 **검증된 업그레이드
    경로 없음** 이라고 나옵니다 — 최신이라는 뜻이 아닙니다. 경로는 마이그레이션에 들어
    있지 않아, 관리자가 리허설한 것만 [스택 버전 관리](versions.md#업그레이드-번들)의
    **업그레이드 번들** 탭에서 등록합니다.

    탭 아래 **실행 이력** 은 이 스택에서 시도한 업그레이드(시각 · 경로 · 상태 · 요청자 · 사유)
    입니다. 사전 점검에서 막힌 시도도 남습니다(`GET /api/v1/stacks/{stackId}/upgrade-runs`).

    **업그레이드** 를 누르면 대화 상자에서 사전 점검(스택 상태 · 다른 업그레이드 · Helm
    릴리스 · 버전 경로 · 24시간 이내 백업 · Helm dry-run) 결과를 보고, 사유를 적어
    **업그레이드 시작** 을 누릅니다. 적용은 서버 백그라운드에서 돌아 화면을 떠나도
    계속되고, 실패하면 직전 리비전으로 자동으로 되돌립니다. 그것도 안 되면 **수동 개입
    필요** 로 남습니다.

    번들에 그 업그레이드가 고치는 CVE 가 적혀 있고 그중 이 스택의 **이미지 취약점** 스캔에서 지금 발견된
    것이 있으면, 행에 **이 스택에서 발견된 CVE N건을 고칩니다** 가 붙고 대화 상자의 **이 업그레이드가
    고치는 이 스택의 CVE** 에 그 목록이 나옵니다. 스캔 결과에 고치는 CVE 가 없으면 그렇다고 알려 주고,
    이 스택의 그 릴리스를 아직 스캔하지 않았으면(스캐너 없음 등) 고치는 CVE 수만 보입니다.

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
