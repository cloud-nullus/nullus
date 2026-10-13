# Nullus 알림 채널 설계 (Telegram · 이메일(SMTP) · Slack)

> 대상 이슈: [cloud-nullus/nullus-plan#63](https://github.com/cloud-nullus/nullus-plan/issues/63) — [Level 2] EPIC: Alert 채널 1차 확장
> 연계: 백업복구 설계 §9 F10 · B3-3(백업 실패 알림, #75 → #63 의존)
> 작성: 2026-10-13 · 저장소 확인 기준 `449c492`

---

## 0. 결정 요약

| # | 결정 | 한 줄 근거 |
|---|---|---|
| 1 | **1차 채널은 세 가지 — Telegram · 이메일(SMTP) · Slack** | 카드가 고르려던 Telegram/SMTP 둘에 Slack 을 더했다(사용자 결정). 셋 다 표준 라이브러리 + HTTP 로 되고 외부 SDK 가 필요 없다 |
| 2 | **알림 모듈을 따로 둔다**(`internal/notification`) | 관리자 핸들러가 SQL 을 직접 쓰던 설정 CRUD 와, 아무도 쓰지 않던 `shared/notification` 의 Slack/Email notifier 를 대체한다. Clean Architecture(도메인 · 유스케이스 · 포트 · 어댑터) |
| 3 | **생산자는 채널을 모른다** — 공개 계약 `sharednotify.Event` + `Publisher` | 백업 같은 모듈은 이벤트만 내고, 어느 채널로 갈지는 조직의 구독이 정한다. 채널이 늘어도 생산자를 고치지 않는다 |
| 4 | **첫 생산자는 백업 결과(B3-3)** — `backup_failed`(심각) · `backup_partial`(경고) · `backup_succeeded`(정보, 기본 끔) | "백업 실패보다 백업 실패를 모르는 것이 더 나쁘다"(F10). 생산자가 있는 이벤트만 구독 목록에 둔다 — 아무도 내지 않는 이벤트를 보여 주면 설정은 저장되는데 알림은 영영 오지 않는다 |
| 5 | **비밀은 암호화해 저장하고 돌려주지 않는다** | 봇 토큰 · 웹훅 URL · SMTP 비밀번호를 `ENCRYPTION_KEY`(AES-256-GCM, `pkg/crypto`)로 `secret_encrypted` 에 둔다. API 는 `secret_set` 과 힌트(봇 ID · 웹훅 호스트+끝 4자)만 준다. 오류 문구에서도 비밀을 지운다 |
| 6 | **전송 보호를 낮추지 않는다** | 웹훅 URL 은 https 만(같은 호스트 예외). SMTP `starttls` 는 서버가 STARTTLS 를 광고하지 않으면 **보내지 않는다**(평문 강등 없음). 인증은 TLS 위에서만 — 평문(`none`)은 인증 없는 사내 릴레이 전용 |
| 7 | **재시도는 일시적 실패만** — 3번, 1초 · 2초(서버의 `retry_after` 는 최대 10초까지 따름) | 네트워크 · 5xx · 429 · SMTP 4xx 는 다시 보내고, 설정이 틀린 실패(401 · 404 · `chat not found` · SMTP 5xx · 인증서)는 바로 실패로 적는다 |
| 7a | **채널은 서로 기다리지 않고, 생산자는 발송을 기다리지 않는다** | 채널마다 동시에 보내고, 백업은 결과를 기록한 뒤 따로(자기 시간 2분) 보낸다. 이력은 생산자의 ctx 와 무관하게 쓴다 — 순서대로 보내던 첫 구현은 응답 없는 SMTP 하나가 백업 마무리 30초를 다 써서 다른 채널과 이력까지 지웠다(리뷰 지적) |
| 8 | **발송마다 이력을 남긴다**(`notification_history`) — 채널 · 이벤트 · 결과 · 시도 횟수 · 오류 | 화면에서 "설정만 되고 알림이 안 가는" 상태를 바로 알아본다. 테스트 전송도 같은 길로 남는다 |
| 9 | **알림 규칙(AlertRule) 평가는 이 카드에서 하지 않는다** | 규칙을 평가하는 루프가 아직 없다(§6). 평가가 생기면 `alert_fired` 이벤트로 같은 `Publisher` 를 쓴다 |

---

## 1. 저장소 확인 결과 (2026-10-13)

| 확인 항목 | 결과 | 근거 |
|---|---|---|
| 알림 저장 | `notification_configs`(org · channel · config JSONB · events) · `notification_history` 테이블은 있다(000012) | 관리자 핸들러가 SQL 로 직접 CRUD, **조직 구분 없이 전부 반환**, 웹훅 URL 을 평문으로 저장·반환 |
| 알림 전송 | `shared/notification` 에 Slack/Email notifier 가 있으나 **프로덕션 코드에서 아무도 만들지 않는다**. Slack 본문이 `text` 없이 `subject`/`body` 라 실제 웹훅은 거절했을 것이다 | `git grep shared/notification` 0건 |
| 알림 규칙 | CRUD 만 있다. 평가 루프 · `alerts` 기록 생산자 없음(화면의 이력은 데모 시드) | 기능목록 F7 |
| 백업 알림 포트 | `port.Notifier` 에 `LogNotifier`(구조화 로그)만 | `internal/backup/adapter/notify` |
| 화면 | 채널 설정 화면 없음(관리자 API 만) | — |

## 2. 구조

```
[백업 RunBackup.finish] ──NotifyBackupResult──▶ backup/adapter/notify.ChannelNotifier
                                                   │ 구조화 로그는 그대로
                                                   ▼ sharednotify.Event (backup_failed …)
                                  notification/usecase.Dispatcher (sharednotify.Publisher)
                                                   │ 조직의 켜진 채널 중 그 이벤트를 구독한 것
                                                   ▼
                                  usecase.Deliverer ── 재시도 ──▶ adapter/sender.{Telegram,Email,Slack}
                                                   │
                                                   └──▶ notification_history (DeliveryRepository)
```

| 레이어 | 파일 |
|---|---|
| 공개 계약 | `internal/shared/notification/event.go` — `EventType` · `Severity` · `Field` · `Event` · `Publisher` |
| 도메인 | `internal/notification/domain` — `Channel`(종류별 검증 · 정규화 · `SecretHint` · `Subscribes` · `ApplyUpdate`), `Message`, `Delivery`, `EventCatalog` |
| 유스케이스 | `ChannelService`(CRUD, 조직 경계) · `Deliverer`(재시도 + 이력) · `Dispatcher`(Publisher) · `SendTest` · `ListDeliveries` |
| 포트 | `ChannelRepository` · `DeliveryRepository` · `Sender` · `SendError{Retryable, RetryAfter}` |
| 어댑터 | `repository`(Postgres · 메모리), `sender`(Slack · Telegram · SMTP), `handler`(관리자 API) |
| 조립 | `internal/notification/wiring.go`, `cmd/api/main.go` — 백업 모듈에 `Notifications: notificationModule.Publisher` |

## 3. 채널별 규칙

| | Telegram | 이메일(SMTP) | Slack |
|---|---|---|---|
| 비밀(Secret) | 봇 토큰(`<숫자>:<문자열>`) | SMTP 비밀번호(사용자 이름이 있을 때만). **서버 · 포트 · 보안 · 계정을 바꾸면 다시 입력**해야 한다 — 비밀번호를 볼 수 없는 사람이 서버만 자기 것으로 돌려 테스트 전송으로 빼내지 못하게 | Incoming Webhook URL |
| 설정 | `chat_id` — 숫자(그룹은 음수) 또는 `@공개채널` | `smtp_host` · `smtp_port`(비면 587/465/25) · `smtp_security`(starttls · tls · none) · `username` · `from` · `to`(≤20) | 없음 |
| 보내는 방식 | Bot API `sendMessage`, `parse_mode: HTML`, 링크 미리보기 끔, 4096자 넘으면 요약 → 필드 순으로 줄인다 | `net/smtp` — STARTTLS(강등 없음) 또는 암묵 TLS, AUTH PLAIN → 없으면 LOGIN(Office 365). UTF-8 · RFC 2047 제목 · quoted-printable · `Auto-Submitted: auto-generated` | `{"text": mrkdwn}` — Slack 호환 웹훅(Mattermost 등)도 받는다 |
| 이스케이프 | HTML(`& < > " '`) | 제목의 줄바꿈 제거, 주소의 CR/LF 거절(`mail.ParseAddress` 는 주석 안의 CR/LF 를 받는다), 헤더는 파싱한 주소로 | `& < >` |
| 일시적 실패 | 5xx · 429(`parameters.retry_after`) · 연결 | 연결 · 시간 초과 · 4xx 응답 | 5xx · 429(`Retry-After`) · 연결 |
| 영구 실패 | 400 `chat not found` · 401 · 403(봇 차단) | 5xx(인증 실패 535 · 수신자 550) · 인증서 · STARTTLS 미지원 | 400 · 403 · 404 `no_service` · 410 |
| 주소 바꾸기 | `NULLUS_TELEGRAM_API_URL`(사내 프록시 · Local Bot API Server) | 채널 설정 | 웹훅 URL |

원격 응답 본문은 알려진 오류 코드 모양(`no_service` 같은 소문자 식별자)이나 Bot API 의 `description` 일 때만 이력에 싣는다 — 웹훅 URL 을 내부 주소로 돌려 그 서비스의 응답을 읽어 내는 길을 좁힌다. SMTP 응답은 160자까지.

복호화하지 못한 행(키 교체)은 그 행만 "비밀을 읽을 수 없음" 으로 표시하고 보내지 않는다 — 목록 · 발송 전체를 멈추지 않는다.

HTTP 채널은 기본 Transport 를 쓰므로 `HTTPS_PROXY` 를 따른다. 에어갭 설치는 외부로 나가지 못하므로 **사내 SMTP 릴레이**가 현실적인 채널이다.

## 4. API (관리자 전용 — `/api/v1/admin`)

| 메서드 · 경로 | 내용 |
|---|---|
| `GET /notifications/events` | 구독할 수 있는 이벤트(`default_on` 포함)와 채널 종류 |
| `GET · POST /notifications/channels` | 조직의 채널 목록 · 생성(검증 실패는 400 `NOTIFICATION_CHANNEL_INVALID`, `detail` 에 필드 이름) |
| `GET · PATCH · DELETE /notifications/channels/:id` | 조회 · 부분 수정(비밀을 비워 보내면 유지, 종류는 못 바꾼다) · 삭제. 다른 조직의 채널은 404 |
| `POST /notifications/channels/:id/test` | 테스트 전송 — **실패해도 200**, 결과(`status` · `error` · `attempts`)가 응답이다. 꺼진 채널에도 보낸다 |
| `GET /notifications/history?limit=` | 최근 발송(기본 50, 최대 200). 채널을 지워도 보낼 때의 이름으로 남는다 |

옛 `/admin/notifications/configs` 는 없앴다(쓰는 화면·CLI 가 없었다. 런북 스모크만 바꿨다).

## 5. 저장과 이행 (000096)

- `notification_configs` 에 `name` · `secret_encrypted` · `updated_at`, `notification_history` 에 `channel_id`(채널 삭제 시 NULL) · `attempts` 를 더했다.
- 000096 전에 `config` 에 평문으로 들어간 비밀(`webhook_url` · `bot_token` · `password` …)은 **API 가 시작할 때** 암호화해 옮기고 `config` 에서 지운다(로컬 실측: 데모 시드의 Slack 웹훅 1건).
- `ENCRYPTION_KEY` 가 없으면 비밀이 필요한 채널을 만들 수 없다(시작 로그 경고). 키를 바꾸면 복호화가 실패한다 — 백업 키 에스크로 런북의 `ENCRYPTION_KEY` 와 같은 키다.

## 6. 실측 (2026-10-13, 로컬 API + Mailpit + 가짜 Telegram/Slack 수신기)

| 확인 | 결과 |
|---|---|
| 채널 3종 생성 | 201. DB 의 `secret_encrypted`·`config` 어디에도 토큰·웹훅 원문이 없다. 응답은 `secret_hint`(`bot 123456789`, `127.0.0.1:18082 …OCAL`)만 |
| 잘못된 웹훅(`http://` 외부 호스트) | 400, `detail: secret`, "URL 자체가 자격증명이라 평문으로 보내지 않습니다" |
| 테스트 전송 3종 | 모두 `sent`. Telegram 은 `/bot<토큰>/sendMessage` 에 `chat_id` · `HTML` · 미리보기 끔, Slack 은 mrkdwn `text`, 메일은 Mailpit 에 `[Nullus][정보] Nullus 알림 테스트`(제목 · 보낸 사람 한글 디코드 정상, `Auto-Submitted`) |
| **백업 성공**(platform_only) | 구독한 메일 · Slack 만 받았다(Telegram 은 구독 안 함) — `백업 성공 — 플랫폼`, 실행 ID · 모드 · 트리거 · 크기(796.6 KiB) |
| **백업 실패**(MinIO 를 멈춰 `BACKUP_DESTINATION_UNAVAILABLE`) | Telegram · 메일 · Slack **세 곳 모두** `🔴 백업 실패 — 플랫폼` + 원인 문구 |
| 영구 실패 | 없는 chat → 1회 시도 후 `HTTP 400: Bad Request: chat not found`, 사라진 웹훅 → `HTTP 404: no_service` |
| 일시적 실패 | 닫힌 SMTP 포트 → 3회 시도(1초 · 2초 대기, 총 3.0초) 후 실패 |
| 느린 채널(리뷰 수정 뒤) | 메일 채널을 닿지 않는 주소(10.255.255.1)로 두고 백업을 실패시켰다 — 백업 API 는 5초에 돌아왔고(목적지 확인 시간), Telegram · Slack 은 그 즉시 받았다. 메일은 33초 뒤 `i/o timeout` 3회 실패로 이력에 남았다 |
| 비밀 노출 | API 로그에 토큰 · 웹훅 원문 0건 |
| 화면 | 관리 > 알림 채널 — 채널 표(보내는 곳 · 받는 이벤트 · 상태 · 테스트/편집/삭제) · 최근 발송(결과 · 시도 · 오류). 실제 API 에 붙여 확인 |

실제 Telegram · Slack 서버로의 전송은 사용자 토큰·웹훅이 있어야 한다 — 화면의 "테스트 전송" 으로 확인한다. STARTTLS · 암묵 TLS · AUTH PLAIN/LOGIN 은 단위 테스트의 가짜 SMTP 서버(자체 서명 인증서)로 확인했다.

## 7. 이 카드에서 닫지 않는 것

| 항목 | 이유 | 재검토 |
|---|---|---|
| 알림 규칙(AlertRule) 평가 루프와 `alert_fired` 이벤트 | 규칙의 대상(어느 스택의 Prometheus) · 평가 주기 · 해제 조건부터 정해야 한다. 규칙의 `channel`(slack/email 종류)은 그때 **채널 ID 연결**로 바꾼다 | 후속 카드 |
| 파이프라인 실패 · 설치 실패 · 토큰 회전 실패 이벤트 | 생산자마다 이벤트를 하나씩 더한다(`EventCatalog` 와 생산자 어댑터) | 요구가 생기는 순서대로 |
| 세트 백업을 한 알림으로 묶기 | 지금은 멤버 실행마다 하나씩 간다(세트 ID 필드로 묶어 볼 수 있다) | 세트 알림이 시끄럽다는 피드백이 오면 |
| 사설 CA 를 쓰는 SMTP 서버 | 시스템 CA 로만 검증한다(검증 끄기 옵션은 두지 않는다) | 에어갭 고객 요구 시 CA 번들 설정 |
| 발송 이력 보존 기간 | 테이블 수명주기는 백업복구 설계의 로그 테이블 보존 과제와 함께 | 그 과제 |
| 재시도 큐(API 재시작을 견디는 발송) | 지금은 최대 3회, 백업 알림은 API 프로세스 안의 고루틴이다 — 그 사이 API 가 재시작하면 그 알림은 사라진다 | 놓친 알림이 문제가 되면 |
| 사설 · 클러스터 대역 주소 차단 | 관리자만 채널을 만든다. 사설 대역을 막으면 에어갭의 사내 SMTP 릴레이 · 사내 Mattermost 를 쓸 수 없다. 대신 원격 본문을 싣지 않는다(§3). http 는 같은 호스트(loopback)만 허용한다 | 다중 테넌트 · 조직 관리자 위임이 생기면 |
| 조직 존재 확인 | 백업 API 처럼 `X-Org-ID` 의 조직이 있는지 보지 않는다(로그인한 사용자의 조직이 먼저다) | 관리자 API 공통 정리 때 |

---

## 참고

- 코드: `internal/notification/**`, `internal/shared/notification/event.go`, `internal/backup/adapter/notify/channel_notifier.go`, `db/migrations/000096_notification_channels.*`, 웹 `features/admin/{api/notification-api.ts,components/notification-channel-dialog.tsx,pages/notification-channels-page.tsx}`
- 연계: [백업복구 설계](./Nullus_백업복구_설계.md) §9 F10 · B3-3, [백업 키 자재 에스크로 런북](../50_운영/Nullus_백업_키_자재_에스크로_런북.md)(`ENCRYPTION_KEY`)
