# Nullus 설치 OSS 업그레이드 요구사항 및 설계

> 상태: Draft
> 우선순위: P0 — 검토 항목 중 실사용 임팩트 최대
> 기준 시나리오: Nullus가 설치한 Argo CD의 안전한 업그레이드
> 연계: `docs/70_전략/ROADMAP.md` Cycle 2 “Stack 업그레이드 전략”

## 1. 배경과 문제

DevOps 조직은 초기 도입 단계의 5명에서 3명, 이후 1명으로 줄며 운영 모드로 전환될 수 있다. 이때 도구를 설치했던 지식과 인력은 사라지지만, 보안 패치·Kubernetes 호환성·CRD 변경·설정 마이그레이션은 계속 필요하다. 결국 힘들게 도입한 Argo CD도 업그레이드하지 못해 취약점과 기술 부채가 쌓인다.

Nullus는 단순히 `helm upgrade`를 실행하는 UI가 아니라, **업그레이드 가능 여부를 판단하고 실패 시 복구할 수 있는 운영 절차**를 제공해야 한다.

## 2. 목표와 비목표

### 2.1 목표

1. 1명의 운영자가 별도의 Helm/Argo CD 전문 지식 없이 검증된 버전으로 업그레이드한다.
2. 적용 전 호환성, 변경 내용, 중단 위험, 복구 가능성을 확인한다.
3. 실패를 감지하면 기존 Helm revision과 설정으로 자동 rollback한다.
4. 누가, 언제, 어떤 버전을, 왜 적용했고 결과가 어떠했는지 감사 기록을 남긴다.
5. 인터넷이 없는 air-gap 환경에서도 Nullus가 배포한 검증 번들로 동일한 절차를 수행한다.

### 2.2 비목표

- MVP에서 임의의 Helm repository/chart version을 허용하지 않는다.
- 외부에서 설치한 레거시 스택의 최초 인수는 별도의 “레거시 스택 등록” 범위로 둔다.
- Kubernetes 클러스터 자체의 버전 업그레이드는 포함하지 않는다.
- 모든 OSS에 보편적 canary/blue-green을 보장하지 않는다. CRD나 PVC를 공유하는 상태 저장 도구에는 단순 복제가 안전하지 않다.
- 도구 자체의 모든 상위 버전 마이그레이션을 Nullus가 자동 작성하지 않는다. 검증된 업그레이드 경로만 제공한다.

## 3. 핵심 사용자 시나리오

### S1. 안전한 Argo CD 패치/마이너 업그레이드

1. 운영자는 스택의 **Version Upgrade** 탭에서 현재 chart/app 버전과 검증된 대상 버전을 본다.
2. Nullus는 release 상태를 직접 조회해 DB의 기대 버전과 drift가 없는지 확인한다.
3. 운영자는 변경 요약, 중단 예상, 호환성, 백업 상태와 rollback 제약을 확인한다.
4. Nullus는 dry-run/렌더링, Kubernetes 버전·아키텍처·자원 검사, deprecated API/CRD 위험, 백업 최신성을 검사한다.
5. 통과하면 업그레이드를 시작하고 진행 단계와 로그를 보여준다.
6. rollout과 Argo CD 기능 health check가 모두 통과하면 성공으로 기록한다.
7. 실패하면 자동 rollback하고, rollback도 실패하면 `manual_intervention_required`로 전환해 복구 런북과 직전 상태를 제공한다.

### S2. 차단되는 업그레이드

다음 중 하나라도 해당하면 기본적으로 실행을 차단한다.

- 현재 조합에서 대상 버전으로 가는 검증 경로가 없다.
- 클러스터 Kubernetes 버전/아키텍처가 대상 chart와 호환되지 않는다.
- 다른 install/upgrade/restore가 실행 중이다.
- release가 `pending-*` 상태이거나 Nullus 기대 설정과 drift된다.
- 필수 백업이 없거나 복구 가능성을 확인하지 못했다.
- dry-run, chart 검증, 이미지 가용성 또는 용량 사전 점검이 실패했다.

## 4. 제품 요구사항

| ID | 요구사항 | MVP |
|---|---|---|
| UPG-01 | 클러스터에 실제 설치된 release/chart/app/revision을 발견한다. | 필수 |
| UPG-02 | 현재 버전에서 갈 수 있는 **Nullus 검증 대상**만 제안한다. | 필수 |
| UPG-03 | chart/app 버전, 변경 요약, breaking change, 중단 예상, 백업 요구를 보여준다. | 필수 |
| UPG-04 | 호환성·drift·용량·API/CRD·렌더링·백업을 사전 점검한다. | 필수 |
| UPG-05 | 동일 스택의 변경 작업을 한 건만 허용한다. | 필수 |
| UPG-06 | Helm atomic/wait/timeout 정책으로 적용하고 단계별 로그를 저장한다. | 필수 |
| UPG-07 | rollout + 도구별 synthetic health check로 성공을 판정한다. | 필수 |
| UPG-08 | 실패 시 직전 Helm revision으로 자동 rollback하고 그 결과도 검증한다. | 필수 |
| UPG-09 | 실행자·사유·전후 버전·점검·로그·rollback을 감사 기록으로 남긴다. | 필수 |
| UPG-10 | 중단 후 API 재호출이 중복 실행을 만들지 않는다. | 필수 |
| UPG-11 | 예정 실행, 유지보수 창, 알림을 지원한다. | 후속 |
| UPG-12 | 다수 스택의 순차/canary 업그레이드를 지원한다. | 후속 |

## 5. 안전 정책

### 5.1 버전 정책

- `latest`를 직접 따라가지 않고, 서명/체크섬이 고정된 **Upgrade Bundle**만 사용한다.
- 번들은 도구, source/target chart·app 버전, Kubernetes 범위, 아키텍처, 업그레이드 단계, values 변환, 검증 항목, rollback 제약, release note URL/요약을 포함한다.
- 다중 단계가 필요한 상위 버전은 `2.10 → 2.11 → 2.12`처럼 경로를 분할하며 중간 단계마다 검증한다.
- 운영자가 미검증 버전을 force하는 기능은 MVP에서 제공하지 않는다.

### 5.2 Argo CD 특화 점검

- API server, repo server, application controller, Redis의 rollout/Ready
- Argo CD API `/api/version` 응답과 대상 버전 일치
- 기존 `Application`/`AppProject` 개수와 핵심 spec 보존
- 표본 `Application`의 refresh, compare, sync/health 수행 가능
- repository credential, OIDC, RBAC 구성의 보존; Secret 값은 로그/매니페스트 diff에서 마스킹
- CRD 변경은 일반 Helm rollback만으로 원상 복구되지 않을 수 있으므로, 업그레이드 전 CRD 스키마 호환성과 대상 리소스 백업을 필수 검사

### 5.3 rollback 정책

1. 적용 전 release revision, values, 관리 리소스, 백업 참조를 snapshot으로 고정한다.
2. Helm 적용 실패는 atomic rollback을 우선하고, 적용 후 health check 실패는 직전 revision으로 명시적 rollback한다.
3. rollback 후에도 같은 health check를 수행한다.
4. 비가역 데이터/CRD 변환이 있는 번들은 “자동 rollback 가능”으로 표시하지 않고, 백업·복구 런북과 명시적 승인을 요구한다.

## 6. 설계

### 6.1 전체 흐름

```text
Discover → Resolve Path → Preflight → Approve → Snapshot/Backup
         → Helm Upgrade → Rollout Check → Tool Health Check → Complete
                                      └─ failure → Rollback → Verify
                                                           └─ failure → Manual Intervention
```

### 6.2 도메인 모델

`UpgradeBundle`

- `id`, `tool`, `source`, `target`
- `kubernetes_range`, `architectures`, `required_intermediate_versions`
- `chart_ref`, `chart_digest`, `image_manifest`
- `values_migrations`, `preflight_rules`, `health_checks`
- `backup_policy`, `rollback_policy`, `release_notes`
- `risk_level`: `low | medium | high`, `expected_disruption`
- `status`: `draft | verified | deprecated | revoked`

`UpgradeRun`

- `id`, `stack_id`, `tool`, `bundle_id`, `requested_by`, `reason`
- `source_snapshot`, `target`, `preflight_result`, `backup_ref`
- `status`, `current_step`, `started_at`, `finished_at`, `error`
- `previous_revision`, `result_revision`, `rollback_result`
- 요청 중복 방지용 `idempotency_key`

상태는 다음으로 제한한다.

```text
planned → preflighting → awaiting_approval → backing_up → upgrading
        → verifying → succeeded
                     └─→ rolling_back → rolled_back
                                          └─→ manual_intervention_required
```

### 6.3 컴포넌트 경계

- **Upgrade Catalog**: 검증된 번들과 취소/revoke 정보를 제공한다. Air-gap에서는 동일 메타데이터를 오프라인 번들에 포함한다.
- **Upgrade Planner**: 실제 release와 Nullus 설정을 비교하고 적용 가능한 경로를 계산한다.
- **Preflight Engine**: 기존 compatibility gate와 Helm dry-run을 재사용하고, drift/CRD/backup/image/resource 검사를 추가한다.
- **Upgrade Orchestrator**: 상태 전이, lock, timeout, retry, cancellation, rollback을 담당한다.
- **Tool Driver**: Helm 공통 적용 위에 Argo CD처럼 도구별 백업·검증·설정 변환을 구현한다.
- **Audit/Notification**: 기존 history/log 저장소와 알림 채널에 결과를 기록한다.

### 6.4 API 초안

| Method | Path | 목적 |
|---|---|---|
| `GET` | `/api/v1/stacks/{stackId}/upgrades` | 실제 release와 업그레이드 후보 조회 |
| `POST` | `/api/v1/stacks/{stackId}/upgrades/preflight` | 번들을 지정해 사전 점검 실행 |
| `POST` | `/api/v1/stacks/{stackId}/upgrade-runs` | 사유·승인과 함께 실행 생성 |
| `GET` | `/api/v1/stacks/{stackId}/upgrade-runs/{runId}` | 상태·단계·로그·결과 조회 |
| `POST` | `/api/v1/stacks/{stackId}/upgrade-runs/{runId}/rollback` | 성공한 작업의 수동 rollback |

`POST` 요청은 `Idempotency-Key`를 필수로 받고, 동일 스택의 설치·설정 적용·백업/복구·업그레이드 lock을 공유한다. 조회 API는 요청 시점의 실제 release 정보와 `checked_at`을 반환해 정적 mock을 제품 정보로 보여주지 않는다.

### 6.5 UI

Version Upgrade 탭의 현재 정적 `UPGRADE_ITEMS`를 API 데이터로 대체한다.

- 도구별 현재 chart/app 버전, 검증 대상, 위험도, 예상 중단, 검사 시각
- 사전 점검의 통과/경고/차단 사유와 해결 방법
- 실행 전 필수 사유 입력, 고위험 번들의 명시적 확인
- 단계별 진행률과 실시간 로그; 화면을 닫아도 서버 작업은 계속
- 성공, rollback 성공, 수동 개입 필요를 구분하고 런북/감사 로그로 연결

## 7. 구현 범위

### Phase 0 — 설계 고정과 실환경 리허설

- Argo CD 현재 기준 버전에서 다음 검증 버전으로 가는 단일 경로 선정
- Application/OIDC/repository credential/CRD가 있는 실환경 리허설
- 성공, Helm 실패, rollout 실패, health check 실패, rollback 실패 결과 기록
- 리허설로 버전 경로·timeout·health check·런북을 확정한 뒤에만 번들을 `verified`로 배포

### Phase 1 — MVP, Argo CD 단일 도구

- Upgrade Bundle 저장/조회와 air-gap artifact 포함
- release discovery, 검증 경로 계산, drift 탐지
- preflight API/UI, Helm dry-run 매니페스트 diff의 Secret 마스킹
- 즉시 실행, 상태 머신, 스택 lock, 진행 로그
- 사전 백업 연결, Helm upgrade, Argo CD health check, 자동/수동 rollback
- 감사 이력과 Version Upgrade 탭의 정적 mock 제거

### Phase 2 — 운영 자동화

- 예정 실행, maintenance window, 사전/사후 알림
- 보안 업데이트 중요도와 지연 시간 표시
- CLI/MCP의 동일 API 지원, 승인 정책/RBAC 세분화
- Prometheus/Grafana/Harbor 순으로 Tool Driver 확장

### Phase 3 — Fleet 업그레이드

- 동일 템플릿 스택의 canary 그룹, 중지 임계치, 순차 rollout
- 독립 인스턴스로 분리 가능한 도구에 한해 blue-green 지원
- 조직 단위 버전 정책과 유지보수 캠페인

## 8. MVP 제외 및 의존성

MVP에서 제외한다.

- Argo CD 외 도구, 다중 도구를 한 번에 올리는 스택 업그레이드
- 임의 버전, downgrade, 자동 최신 버전 추종
- 예약, 다단계 승인, fleet canary/blue-green
- 파괴적 CRD/데이터 마이그레이션의 자동 역변환

핵심 의존성은 백업/복구 기능, compatibility matrix, Helm release manager, 클러스터 discovery, 인증/RBAC, air-gap 이미지 번들이다. 의존 기능이 꺼져 있을 때는 안전 조건을 완화하지 말고 업그레이드를 차단한다.

## 9. 수용 기준

1. 실제 버전과 DB가 다르면 업그레이드 버튼을 비활성화하고 drift 해소 방법을 표시한다.
2. 검증되지 않은 대상이나 지원 범위 밖 Kubernetes에서는 API와 UI 모두 실행을 차단한다.
3. 사전 점검 결과는 성공/경고/차단과 함께 기계가 아닌 사람이 실행할 수 있는 해결 방법을 포함한다.
4. 같은 스택의 두 업그레이드 요청이 동시에 도착해도 하나만 실행된다.
5. API 타임아웃이나 UI 이탈 후에도 작업은 서버에서 완결되며, 재접속하면 현재 단계를 복원한다.
6. 성공 후 Argo CD API 버전·핵심 workload·표본 Application·OIDC/저장소 연결이 모두 검증된다.
7. 강제된 health check 실패에서 자동 rollback하고, 직전 버전과 기능 검증이 복구된다.
8. Secret은 API, 로그, manifest diff, 감사 기록 어디에도 평문으로 노출되지 않는다.
9. online과 air-gap 환경에서 같은 번들의 판단과 실행 결과가 동일하다.

## 10. 성과 지표와 운영 기준

- 검증된 Argo CD 업그레이드 성공률 ≥ 95%
- 실패 시 자동 rollback 성공률 ≥ 99%(복구 가능 번들 기준)
- 일반 패치/마이너 경로의 운영자 실작업 시간 ≤ 15분(대기 시간 제외)
- 수동 명령어 입력 없이 UI 또는 CLI에서 완결
- `manual_intervention_required` 비율과 원인, 업그레이드 지연 일수, 버전별 실패율을 운영 지표로 집계

## 11. 오픈 결정

| 결정 | 선택지 | 제안 |
|---|---|---|
| 검증 번들 배포 위치 | DB migration / 서명된 원격 catalog / release artifact | MVP는 release artifact + DB import; 후속에 서명된 catalog |
| 강제 예외 | 완전 금지 / 전문가 권한으로 허용 | MVP 금지, 운영 데이터 축적 후 재검토 |
| 고위험 승인 | 단일 운영자 확인 / 2인 승인 | 1명 조직을 위해 단일 확인 + 강한 감사 기록 |
| 백업 필수 범위 | 모든 번들 / 고위험만 | 초기에는 모든 번들; 리허설 근거로 완화 가능 |
