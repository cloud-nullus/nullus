# Webhook API 범위 정의 (nullus-plan#44)

> EPIC: [cloud-nullus/nullus-plan#44](https://github.com/cloud-nullus/nullus-plan/issues/44) — Webhook API 범위 정의.
> 본 문서는 범위 정의 카드다 — 산출물은 이 문서뿐이고, 구현은 별도 작업으로 분리한다.

작성일: 2026-08-16

---

## 요약 (비관계자용 — 이것만 읽어도 논의 가능)

**무엇을 정하나.**
외부 시스템(CI 서버, 모니터링 등)이 "일이 끝났어요/문제가 생겼어요" 같은 이벤트를 **Nullus에 먼저 알려오는 수신 창구(inbound webhook)** 를 만들 것인지, 만든다면 무엇을·어떻게 받을지의 범위다.

**현재 상태.**
Nullus에는 **외부에서 걸어오는 수신 창구가 하나도 없다.** 지금은 반대 방향만 있다 — 사용자가 화면을 열 때 Nullus가 CI 서버에 "새 빌드 있어요?"라고 **물어보러 가는(pull)** 방식이고, 화면을 열기 전까지는 결과가 반영되지 않는다.

**핵심 설계 원칙 — "웹훅은 초인종, 데이터는 직접 가서 확인".**
웹훅으로 받은 내용(페이로드)을 그대로 믿고 기록하면, 위조된 요청이 가짜 기록을 만들 수 있고 외부 시스템마다 다른 형식의 해석기를 유지해야 한다. 그래서 **웹훅은 "확인하러 와라"는 신호로만 쓰고, 실제 데이터는 이미 검증된 기존 조회 경로(pull)로 Nullus가 직접 가서 가져온다.** 신호가 위조돼도 결과적으로 진짜 데이터만 반영된다.

**받을 이벤트(1차 범위) — 딱 하나.**
- **CI 빌드 완료 신호**: 빌드가 끝나면 CI 서버가 Nullus를 호출 → Nullus가 해당 파이프라인만 즉시 재동기화. 화면을 열지 않아도 실행 기록·(향후) 테스트 결과가 실시간 반영된다. Test/Coverage 수집 설계(#46)가 남긴 "push 수집" 요구의 답이기도 하다.

**나중으로 미루는 것**: 모니터링 경보 수신(Alertmanager — 지금은 경보를 만들어내는 체계 자체가 미구현), 레지스트리 이미지·스캔 이벤트(보안 기능 #41/#45 확정 후), 소스 push 이벤트(이미 Gitea→Jenkins 직결로 해결돼 Nullus가 받을 이유 없음).

**우선순위 재판단(이 카드의 결론).**
**당장 구현하지 않는다 — 후순위 유지가 맞다.** 현재의 "화면 열 때 동기화"로 실사용이 충족되고 있고, 수신 창구를 여는 순간 서명 검증·시크릿 배포·재전송 처리 같은 보안·운영 비용이 따라온다. 아래 승격 조건 중 하나가 오면 이 문서의 범위로 착수한다.

---

## 상세 (근거와 코드 위치)

### 1. As-Is

| 영역 | 현황 | 근거 |
|---|---|---|
| 인바운드 수신 창구 | **없음 (0개)** | `/api/v1/*` 전체가 사용자/세션 인증 전제, 무인 수신 경로 부재 |
| 기존 webhook | Nullus가 **등록만** 해준다 — Gitea 저장소에 push 훅을 걸되 **수신자는 Jenkins**(`/gitea-webhook/post`). "커밋 → 빌드 시작" 자동화용 | `internal/cicd/usecase/provision_app_project.go:369`, `adapter/gitea/client.go:267` |
| CI 실행 기록 반영 | **화면 조회 시점 pull** — 배포 이력 조회(`GET /cicd/deployments?pipeline=`)가 오면 그때 `SyncPipelineRuns.ForPipeline` 이 CI 서버에서 당겨온다. 백그라운드 주기 실행 없음 → **실시간 아님** | `adapter/handler/pipeline_handler.go:405`, `usecase/sync_pipeline_runs.go` |
| 배포 상태 추적 | k8s watch 로 직접 관측 — webhook 불필요 영역 | `adapter/kube/step_tracker.go` |
| 알림(경보) | **발신만 있음**(notifier, #35 에서 채널 확장 예정). 수신·평가·`alert-history` 기록을 만드는 생산자는 미구현 | `internal/shared/notification/notifier.go`, observability 에 history insert 주체 없음 |

### 2. 외부 이벤트 유형 정의 (세부 태스크 ①)

후보 전수와 판정:

| 이벤트 | 발신자 | 판정 | 근거 |
|---|---|---|---|
| **CI 빌드 완료** | Jenkins(·향후 GitLab/GitHub) | ✅ **1차 채택** | pull 동기화의 실시간화. #46(테스트/커버리지 수집)의 push 요구를 "신호 수신 + pull 재사용"으로 충족 |
| 모니터링 경보 | Alertmanager | ⚪ 2차 | 받을 곳(alert-history 생산 체계)이 아직 없다 — 경보 평가/기록 기능이 생길 때 함께. #35(알림 채널)는 발신이라 별개 |
| 이미지 push·스캔 완료 | Harbor | ⚪ 2차 | 보안 단계(#41)·Security Dashboard(#45) 확정 후 그 수요에 맞춰 |
| 소스 push | Gitea/GitLab | ❌ 제외 | 이미 Gitea→Jenkins 직결로 해결. Nullus 가 받을 이유 없음 |
| Argo CD sync 상태 | Argo CD | ❌ 제외 | k8s watch 로 이미 관측 가능 |

### 3. 인증 방식 후보 (세부 태스크 ②)

무인 경로라 기존 RBAC(사용자 토큰) 밖이다. 후보:

| 방식 | 내용 | 판정 |
|---|---|---|
| **HMAC 서명** | 소스별 공유 시크릿으로 본문 서명(`X-*-Signature` 류). Gitea·GitHub·Jenkins 플러그인 표준 | ✅ **1차** — 시크릿은 OpenBao 에 보관(기존 시크릿 평면 재사용) |
| Bearer 토큰 | 서명 미지원 발신자(Alertmanager 등)용 고정 토큰 | ⚪ 2차 이벤트 도입 시 |
| mTLS | 상호 TLS | ❌ 제외 — 운영 비용 과잉 |

공통 방침: 소스별 시크릿 분리(하나 유출돼도 나머지 무사), 재전송 대비 **멱등 처리**(기존 `dep_ci_*` 멱등 ID 체계가 이미 이를 보장), 기존 rate limiter 적용, 페이로드 크기 제한. **서명이 뚫려도 신호일 뿐이라는 것이 마지막 방어선이다** — 데이터는 pull 이 진실을 확인한다.

#### HMAC 서명의 동작

GitHub·Gitea 웹훅이 쓰는 표준 방식 그대로다. 발신자와 Nullus 가 시크릿을 공유하고, 발신자는 **시크릿이 아니라 "시크릿으로 본문을 서명한 해시"** 를 헤더에 실어 보낸다. Nullus 는 같은 시크릿으로 본문을 다시 서명해 헤더 값과 비교한다.

```
[발신: CI]   서명 = HMAC-SHA256(시크릿, 요청 본문)
             POST /api/v1/webhooks/ci/pipe-42
             X-Signature: sha256=7f83b165...        ← 서명만 실림
[수신: Nullus] 같은 시크릿으로 재계산 → 헤더와 비교 → 불일치면 401
```

Bearer 토큰 대신 서명을 1차로 두는 이유: (1) **시크릿이 회선에 실리지 않는다** — 도청당해도 새는 것은 서명뿐이다. (2) **본문 변조가 탐지된다** — body 한 글자만 바뀌어도 서명이 어긋난다. (3) CI 도구들이 표준 지원한다 — 기존 `EnsureWebhook` 도 이미 secret 인자를 받는 구조다(`adapter/gitea/client.go:267`).

#### 재전송 멱등 — `dep_ci_*` 가 이미 풀어 둔 문제

웹훅의 고질 문제는 재전송이다 — 타임아웃이 나면 발신자는 같은 이벤트를 다시 보내고(정상 동작), 순진하게 구현하면 빌드 1건이 기록 2건이 된다. Nullus 는 기록 ID 를 난수가 아니라 **"파이프라인+빌드 번호"에서 결정적으로 생성**하므로 이 문제가 이미 풀려 있다(`sync_pipeline_runs.go` `runDeploymentID`):

| 상황 | 생성 ID | 결과 |
|---|---|---|
| pipe-42 빌드 #17 첫 동기화 | `dep_ci_pipe-42_17` | 새 기록 (실행 중) |
| 재전송으로 같은 빌드 재동기화 | `dep_ci_pipe-42_17` **(동일)** | 같은 ID 갱신 — **개수 불변**, 상태만 전이(실행 중→성공) |

따라서 웹훅 쪽에 이벤트 중복제거 저장소를 따로 만들 필요가 없다 — 신호가 10번 와도 pull 이 10번 돌 뿐, 기록은 빌드 수만큼만 남는다.

#### 정리 — 3중 방어

| 층 | 막는 것 |
|---|---|
| HMAC 서명 | 위조 발신자 (1차 차단) |
| 신호-only 원칙 | 서명이 뚫려도 페이로드를 믿지 않으므로 **가짜 데이터 주입 불가** — pull 이 CI 서버에서 진실만 가져온다 |
| `dep_ci_*` 멱등 | 재전송·신호 폭주 — 몇 번 와도 기록 무결, 최악 피해는 "불필요한 pull 몇 번" |

이 조합 덕에 신규 구현은 사실상 "서명 검증 미들웨어 + `ForPipeline` 호출"로 수렴한다 — 어려운 부분(중복·위조 데이터)은 기존 구조가 흡수한다.

### 4. 최소 엔드포인트 초안 (세부 태스크 ③)

```
POST /api/v1/webhooks/ci/:pipelineId     # CI 빌드 완료 신호
  - 인증: HMAC 서명 (파이프라인 프로비저닝 시 발급한 시크릿)
  - 동작: 페이로드는 파싱하지 않는다(신호로만 취급) →
          SyncPipelineRuns.ForPipeline(pipelineId) 호출 (기존 코드 재사용)
  - 응답: 202 Accepted (수신 즉시 반환, 동기화는 비동기)
```

- 라우팅 그룹은 인증 미들웨어가 다른 **`/api/v1/webhooks/*` 별도 그룹**으로 둔다(사용자 인증 그룹과 격리).
- 등록 자동화: 파이프라인 프로비저닝(`provision_app_project`)이 Gitea→Jenkins 훅을 걸듯, **Jenkins job post-build 훅으로 이 URL 을 함께 걸어 준다** — 사용자 수작업 없음.
- 2차 이벤트(alerts, registry)는 같은 그룹 아래 `POST /webhooks/alerts`, `POST /webhooks/registry` 로 확장한다 — 이 문서에서는 자리만 예약.

### 5. 우선순위 재판단 (세부 태스크 ④ — 이 카드의 결론)

**후순위 유지.** 근거:

1. 현행 on-demand pull 이 화면 요구를 충족하고 있다 — 웹훅의 증분은 "화면을 안 열어도 반영"뿐.
2. 인바운드 표면 신설은 서명 검증·시크릿 배포·멱등·rate limit 등 보안·운영 비용을 동반한다 — 수요가 확정되기 전에 열 이유가 없다.

**승격 트리거** (하나라도 오면 이 문서 범위로 착수):
- #46 테스트/커버리지 수집 구현이 실시간 반영을 요구할 때
- 파이프라인 수 증가로 조회 시점 동기화의 지연·부하가 체감될 때
- 경보 수신(Alertmanager) 기능이 로드맵에 확정될 때

## 참고 (코드 위치)

- 기존 webhook 등록(아웃바운드): `internal/cicd/usecase/provision_app_project.go:369`, `internal/cicd/adapter/gitea/client.go:267`
- pull 동기화(재사용 대상): `internal/cicd/usecase/sync_pipeline_runs.go`, `internal/cicd/adapter/handler/pipeline_handler.go:405`
- 멱등 실행 축: `dep_ci_<pipeline>_<build#>` (sync_pipeline_runs.go)
- 발신 알림(수신과 별개): `internal/shared/notification/notifier.go` — 채널 확장은 nullus-plan#35
- 연계 설계: [Nullus_Test_Coverage_Dashboard_설계.md](./Nullus_Test_Coverage_Dashboard_설계.md) (#46 — push 수집을 본 카드와 묶어 재검토하기로 한 출처)
