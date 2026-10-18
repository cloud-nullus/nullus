# 알림 이력

<span class="route">/observability/alert-history</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>
<span class="role role-developer">developer</span>

스택에서 발생한 **심각 경보**를 시간순으로 봅니다. 화면 설명에도 적혀 있듯이,
스택마다 Prometheus 가 기본 규칙으로 평가한 결과이고 발생·해제는 알림 채널로도
갑니다.

## 쓰는 법

1. 위쪽 필터에서 클러스터·스택을 좁힙니다(선택).
2. **상태** 필로 `전체` 와 `진행 중만` 을 바꿉니다. 이건 서버에 다시 물어봅니다.
3. **기간** 필로 `최근 24시간`·`최근 7일`·`최근 30일`·`전체 기간` 을 고릅니다.
   기본값은 7일입니다.
4. 검색창에 경보 이름·스택 이름·요약을 넣어 좁힐 수 있습니다.
5. 행을 누르면 오른쪽에 상세 패널이 열립니다.

기간과 검색은 **받아 온 목록 안에서** 거릅니다. 목록은 60초마다 자동으로
새로 읽습니다.

## 표 컬럼

| 컬럼 | 내용 |
|------|------|
| (펼치기) | 행을 펼치는 버튼 |
| 경보 | 경보 이름(`alertname`) |
| 스택 | 경보가 난 스택 |
| 대상 | 네임스페이스/파드, PVC, 디플로이먼트, 노드 등 |
| 내용 | 요약 한 줄 |
| 발생 | 처음 발생한 시각 |
| 해제 | 해제 시각. 아직이면 `진행 중` 배지 |

!!! note "표에는 심각도 컬럼이 없습니다"
    거르는 기준이 이미 `critical` 하나뿐이라 컬럼으로 두지 않았습니다. 심각도는
    상세 패널에서 볼 수 있습니다.

## 상세 패널

오른쪽 패널에 경보 · 스택 · 대상 · 심각도 · 발생 · 해제 · **마지막 알림** ·
내용 · 설명이 나오고, 런북 링크와 라벨 목록이 함께 붙습니다.

`마지막 알림` 은 이 경보로 알림 채널에 **마지막으로 보낸 시각**입니다. 계속 발생
중인 경보는 4시간마다 다시 알리므로, 발생 시각과 이 값이 다를 수 있습니다.

## 호출하는 API

```
GET /api/v1/observability/stack-alerts?state=open|all&limit=200
```

응답은 `{items, total}` 이고, 각 항목에 `id`, `stack_id`, `stack_name`,
`alertname`, `severity`, `summary`, `description`, `runbook_url`, `target`,
`labels`, `started_at`, `resolved_at`, `last_notified_at`, `active` 가 들어
있습니다. 서버 기본 `limit` 은 100, 최대 500 입니다.

조직은 로그인 주체에서, 없으면 `X-Org-ID` 헤더, 그다음 기본 조직 순으로
결정합니다.

경보 목록 자체(`/observability/stack-alerts`)는 **인증만 하면** 역할 제한 없이
받을 수 있습니다. 다만 클러스터·스택 필터를 채우는 조회가 admin·devops 전용이라
developer 에게는 **필터가 비어 보입니다**
([권한 표](../../reference/rbac.md#화면은-열리는데-데이터가-비는-경우)).
