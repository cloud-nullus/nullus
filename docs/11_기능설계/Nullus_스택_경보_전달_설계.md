# Nullus 스택 경보 전달 설계 (Alertmanager 수집)

> 대상 이슈: [cloud-nullus/nullus-plan#87](https://github.com/cloud-nullus/nullus-plan/issues/87) — 스택 심각 경보를 알림 채널로 전달
> 출처: #63(알림 채널)에서 분리. 범위 결정(2026-10-13): **사용자 정의 규칙 편집기는 만들지 않는다**(2안).
> 작성: 2026-10-13 · 저장소 확인 기준 `a219b87`

---

## 0. 결정 요약

| # | 결정 | 한 줄 근거 |
|---|---|---|
| 1 | **규칙을 만들지 않고 스택이 이미 평가하는 기본 규칙의 경보를 전달한다** | 스택마다 kube-prometheus-stack 이 기본 규칙 145개를 평가하고 있다(디스크 · 파드 재시작 · 노드 · PV · 대상 무응답 등). 사용자가 더할 것이 적고, 편집기 · 스택 동기화가 #87 에서 가장 비싼 부분이었다 |
| 2 | **플랫폼이 스택 Alertmanager 의 활성 경보를 주기적으로 가져온다**(API 서버 프록시, 기본 1분) | OpenBao 접근과 같은 길이라 스택에 인바운드 경로 · 웹훅 인증이 필요 없다(에어갭 · 다중 클러스터). 평가 · 지속 시간 · 침묵 · 억제는 Prometheus · Alertmanager 에 맡긴다. 대가는 최대 1분 지연 |
| 3 | **심각(critical)만, 어디서나 뜨는 잡음은 뺀다** | Watchdog · InfoInhibitor 와 **컨트롤 플레인 수집 실패**(KubeControllerManagerDown · KubeSchedulerDown · KubeProxyDown · etcd*) — kind · kubeadm 기본 · 관리형 클러스터에서 늘 뜬다. 로컬 kind 의 심각 4건이 전부 이것이었다 |
| 4 | **발생 · 해제를 알리고, 계속 발생 중이면 4시간마다 다시 알린다** | Alertmanager 기본 repeat_interval 과 같다. 발생을 알리지 않은 경보는 해제도 알리지 않는다(짝이 맞지 않는다) |
| 5 | **잠깐 안 보인 것은 해제하지 않는다** — 닿지 못하면 건너뛰고, **3분 동안 안 보여야** 해제 | Alertmanager 는 경보를 메모리에만 들고 있어 재시작하면 Prometheus 가 다시 보낼 때까지 빈 목록을 준다(실측 1분 20초). 그것을 해제로 읽으면 "해제" 와 "발생" 이 한꺼번에 나간다. 침묵 · 억제된 경보도 가져와 "보인 것" 으로 친다(알리지는 않는다). 지워진 스택의 열린 경보는 알리지 않고 닫는다 |
| 6 | **수집은 레플리카 하나만 돈다**(Postgres advisory lock) — 그 위에 DB 가 한 번 더 가린다 | 운영 기본 API 2 레플리카가 Alertmanager 를 서로 다른 시각에 읽으면 한쪽은 보고 다른 쪽은 못 봐 발생 · 해제가 엇갈린다. 잠금이 없거나 풀린 틈에도 열린 경보 부분 유니크 · 행 잠금 알림 차례 · 조건부 해제(마지막으로 본 시각이 유예보다 오래된 것만)가 한 번만 알리게 한다 |
| 6a | **발송은 수집이 끝난 뒤 동시에, 수집 ctx 와 따로** | 응답 없는 채널 하나(SMTP 재시도 약 1분)가 수집 시간을 다 써서 다른 스택을 건너뛰거나, 차례를 잡은 알림이 수집 ctx 와 함께 취소되지 않게(독립 리뷰 지적) |
| 7 | **채널은 이벤트 구독으로** — `alert_fired` · `alert_resolved` | 백업 알림과 같은 방식(#63). 새 채널은 기본으로 켜고, 기존 채널은 편집에서 켠다 |
| 8 | **알림 이력 화면 = 실제 스택 경보**, **알림 규칙 화면은 숨긴다**, **데모 이력은 지운다** | 규칙은 평가되지 않아 화면이 거짓말을 했다. 데모 이력(alerts 6행 · 발송 이력 4행)은 000021 시드에서 빼고 000097 이 이미 들어간 DB 에서 지운다 |

---

## 1. 확인 결과 (2026-10-13, 로컬 kind GitLab 스택)

| 항목 | 내용 |
|---|---|
| 스택 Prometheus | `kube-prometheus-stack` — PrometheusRule 35개, 경보 규칙 145개, 평가 30초, 규칙 선택 `release=kube-prometheus-stack` |
| Alertmanager | `kube-prometheus-stack-alertmanager:9093`, 기본 설정(수신자 `null` — 아무 데도 안 보냄) |
| 활성 경보 | 13건 — 심각 4(KubeSchedulerDown · KubeControllerManagerDown · KubeProxyDown · etcdInsufficientMembers), 나머지 경고 · Watchdog |
| 플랫폼 → 스택 | API 서버 프록시 `/api/v1/namespaces/{ns}/services/http:{svc}:{port}/proxy/...` 로 닿는다 |
| 기존 알림 규칙 | CRUD 만 있고 평가 루프 없음, `alerts` 이력은 데모 시드 |

## 2. 구조

```
[스케줄러 1분] ─▶ usecase.CollectStackAlerts
                   ├─ bridge.MonitoredStacks ── 스택 저장소 ListCompleted (모니터링 수집을 고른 완료 스택)
                   ├─ alertmanager.Source ──── API 서버 프록시 → /api/v2/alerts?active&!silenced&!inhibited
                   ├─ domain.DefaultAlertFilter (심각만 · 잡음 제외)
                   ├─ repository.PostgresStackAlertRepository (stack_alerts, 000097)
                   └─ sharednotify.Publisher ─▶ 알림 모듈(#63) ─▶ Telegram · 이메일 · Slack
[화면] 알림 이력 ─▶ GET /api/v1/observability/stack-alerts?state=open|all
```

| 레이어 | 파일 |
|---|---|
| 도메인 | `internal/observability/domain/stack_alert.go` — `MonitoredStack` · `SourceAlert` · `AlertFilter` · `StackAlert`(대상 · 발생/해제 이벤트) |
| 포트 | `internal/observability/port/stack_alerts.go` |
| 유스케이스 | `collect_stack_alerts.go`(수집 · 알림) · `list_stack_alerts.go`(화면) |
| 어댑터 | `adapter/alertmanager`(API 서버 프록시) · `adapter/bridge`(스택 모듈 → 관측 도메인) · `adapter/repository/postgres_stack_alert.go` · `adapter/handler/stack_alert_handler.go` |
| 스케줄러 | `internal/observability/scheduler` — 겹치지 않게(inFlight), 한 번에 5분 |
| 이벤트 | `internal/shared/notification` 에 `alert_fired` · `alert_resolved`, 알림 모듈 카탈로그에 추가 |

알림 본문: 제목 `{경보} — {스택}`(다시 알림은 `계속 발생 중:` · 해제는 `해제:`), 요약 · 설명, 필드 스택 · 경보 · 대상(파드 · PVC · 노드 · 인스턴스) · 런북.

## 3. 설정

| 환경 변수 | 기본 | 내용 |
|---|---|---|
| `NULLUS_STACK_ALERT_INTERVAL` | `1m` | 수집 주기. `off`·`0` 이면 끈다. 10초보다 짧게는 안 된다(스택 수 × 주기만큼 API 서버를 부른다) |

## 4. 실측 (2026-10-13, 로컬 API + kind GitLab 스택 + Mailpit · 가짜 Telegram/Slack)

| 확인 | 결과 |
|---|---|
| 000097 | 로컬 DB 의 데모 `alerts` 6행 · 데모 발송 이력 4행이 0 이 됐다 |
| 잡음 거르기 | 수집기가 돌아도 기록 0 — 활성 심각 4건이 모두 컨트롤 플레인 수집 실패였다 |
| 리허설 경보 | 스택에 PrometheusRule(심각 1 · 경고 1, `vector(1)`)을 넣었다 — 적용 43초 뒤 Prometheus 가 발생시켰고, **2초 뒤** 플랫폼이 기록하고 세 채널로 보냈다. 경고는 기록 · 전달되지 않았다 |
| 알림 내용 | 대상 `nullus-gitlab-sca/data-nullus-postgresql-0`, 런북 링크, 메일 제목 `[Nullus][심각] NullusRehearsalDiskFull — nullus-gitlab-sca` |
| 중복 | 이후 수집 주기마다 다시 보내지 않았다 |
| 해제 | 규칙을 지우자 Alertmanager 가 약 4분 뒤 경보를 내렸고(Prometheus 가 보낸 endsAt 만료), 같은 주기에 해제 기록 · 세 채널로 `해제: … 4m1s 동안 이어진 경보가 해제됐습니다`(유예 도입 전 측정) |
| **Alertmanager 재시작**(리뷰 수정 뒤) | 경보가 발생 중일 때 Alertmanager 파드를 지웠다 — 조회 실패(그 주기 건너뜀) → 빈 목록 → 리허설 경보 없는 목록이 약 1분 20초 이어진 뒤 Prometheus 가 다시 보냈다. **해제 알림 · 중복 발생 알림 0건**, 경보는 열린 채 마지막으로 본 시각만 갱신됐다 |
| 실제 해제(유예 적용) | 규칙을 지운 뒤 Alertmanager 에서 마지막으로 본 때가 12:43:24, 플랫폼 해제가 12:46:24 — 유예 3분 그대로. 세 채널로 해제 알림(이어진 시간은 마지막으로 본 때까지로 센다) |
| 발송 이력 | 알림 채널 화면의 최근 발송에 발생 3 · 해제 3 |
| 화면 | 알림 이력에 실제 경보(스택 · 대상 · 해제 시각), 상세에 설명 · 런북 · 라벨. 메뉴에 알림 규칙 없음 |

규칙의 조건이 풀려 경보가 끝나면 Prometheus 가 곧바로 해제를 보낸다. 규칙을 지운 경우는 위처럼 endsAt 이 만료될 때(약 4분)까지 활성으로 남는다. 플랫폼의 해제 알림은 그 뒤 유예 3분이 더해진다.

## 5. 이 카드에서 닫지 않는 것

| 항목 | 이유 | 재검토 |
|---|---|---|
| 사용자 정의 규칙 편집기(프리셋 · PromQL · 스택 동기화) · 규칙의 채널 연결 | 기본 규칙으로 대부분 덮인다. 숨긴 규칙 화면 · API 는 남겨 두었다 | Backlog |
| 경고(warning) 등급 · 조직별 끄기 목록 | 1차는 심각만. 잡음이 문제가 되면 조직 설정으로 | 피드백 후 |
| 컨트롤 플레인 수집을 스택 values 에서 끄기 | 경보를 거르는 대신 근본적으로 없애는 길 — kube-system Service 잔여물(#317)도 줄어든다. 설치 변경이라 따로 | 스택 설치 정리 때 |
| **모니터링 자체가 죽었을 때 알림** | Alertmanager 에 계속 닿지 못하면 지금은 로그만 남긴다 — 그 스택의 경보는 오지 않는다 | 다음 카드 후보 |
| 발송 실패 시 차례 되돌리기 | 알림 차례(마지막 알림 시각)를 먼저 적고 보낸다. 모든 채널이 실패하면 다음 다시 알림(4시간)까지 오지 않는다 — 채널별 결과는 발송 이력에 남고, 알림 모듈이 일시적 실패를 3회 재시도한다. 일부 채널만 실패했을 때 되돌리면 성공한 채널에 중복이 간다 | 놓친 알림이 문제가 되면(재시도 큐와 함께) |
| 기존 채널의 경보 구독 | #63 에서 만든 채널은 `alert_fired` · `alert_resolved` 를 구독하지 않는다 — 자동으로 켜지 않는다(원치 않는 알림 폭주). 채널 편집에서 켠다 | 출시 안내 |
| 플랫폼 화면에서 침묵(silence) 걸기 | Alertmanager 의 침묵은 이미 존중한다(가져올 때 뺀다) | 요구가 생기면 |

---

## 참고

- 형제 설계: [알림 채널 설계](./Nullus_알림_채널_설계.md)(채널 · 발송 · 이력)
- 코드: `internal/observability/**`(stack_alert · collect_stack_alerts · alertmanager · bridge · scheduler), `db/migrations/000097_stack_alerts.*`, 웹 `features/observability/pages/alert-history-page.tsx`
