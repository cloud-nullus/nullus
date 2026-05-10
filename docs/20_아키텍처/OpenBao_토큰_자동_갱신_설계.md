# OpenBao 기반 OSS 토큰 자동 갱신(자동 업데이트) 설계

**작성일**: 2026-05-02  
**버전**: 1.1  
**대상**: DevOps Engineer, Backend Engineer, Platform Engineer  
**연관 문서**: `nullus_PRD_1.3.md`, `Nullus_API_설계.md`, `Nullus_DB_스키마.md`, `Nullus_인프라_배포_설계.md`

---

## 0. 구현 정합성 상태 (2026-05-10)

이 문서는 설계 문서이며, 현재 브랜치 구현 상태를 아래와 같이 반영한다.

### 0.1 구현 완료

- `authentication.provider=openbao` 선택 시에만 OpenBao 배포/라우팅(`installing_openbao`, `openbao.<access_domain>`) 수행
- Secret Manager 추상화 계층 도입: `internal/shared/secrets` (`Router`, `Store`, `OpenBaoStore`)
- Stack token source 등록 시 `metadata.secret_manager` 저장 및 OpenBao path write 연동
- Admin `reveal`이 placeholder 대신 OpenBao 실조회 값을 우선 반환
- OpenBao preflight gate: 배포 시작 시 provider 구성/헬스/token lookup-self 검증
- Rotation scheduler 기본 동작 추가: due 대상 조회, 상태/이벤트 업데이트, 실패 백오프(15m) 처리
- Reissue adapter 추상화 도입: `internal/admin/rotation/Reissuer`
- GitLab reissue 구현: PAT self rotate API 연동
- GitHub reissue 구현: GitHub App installation access token 발급 연동

### 0.2 부분 구현/주의사항

- 현재 OpenBao 배포는 개발 편의(dev mode) 구성이며, 운영 HA/TLS/스토리지 구성은 별도 운영 스펙 필요
- Provider별 metadata 요구사항(예: GitHub app_id/installation_id/private_key_pem)이 충족되어야 실제 reissue 수행
- 미구현 provider(예: Harbor/Slack 등)는 `ErrReissueUnsupported` 경로로 fallback 처리

### 0.3 미구현 항목

- 승인 워크플로우(`requires_approval`)와 회전 스케줄러의 강결합 상태전이(FAILED_MANUAL -> AWAITING_APPROVAL -> RENEWING) 완성
- 앱 무중단 반영(ESO/CSI/rolling restart 자동화)과 회전 성공 후 반영 검증 자동화
- provider별 세분화 백오프 정책 및 rate-limit/jitter 튜닝
- rotation 메트릭/알림(`token_rotation_total`, `token_expiry_seconds`)의 운영 대시보드 연동

---

## 1. 목적

Nullus가 설치/연계하는 OSS(Git provider, Registry, OIDC client, Webhook 대상 등)의 토큰·시크릿이 만료되더라도
서비스 중단 없이 자동 갱신(renew/reissue)하고 안전하게 반영하는 표준 아키텍처를 정의한다.

핵심 원칙:

- **OpenBao-first**: 원문 시크릿의 Source of Truth는 OpenBao
- **최소 권한**: Kubernetes auth + role/path 기반 접근 제어
- **무중단 지향**: 가능하면 재배포 없이 반영, 필요 시 rolling restart
- **감사 추적**: 모든 갱신/실패/재시도 이벤트를 Audit 로그로 기록

---

## 2. 범위

### 2.1 대상 시크릿

- OIDC client secret (Keycloak/Authentik 연동)
- Git provider token / webhook secret
- Container registry credential
- 외부 알림 채널(Slack/Email webhook/API key)
- 내부 연동용 machine token

### 2.2 비대상(초기)

- 사람 사용자 비밀번호 직접 회전 자동화
- 외부 IdP의 정책상 수동 승인이 필요한 credential의 완전 자동화

---

## 3. 아키텍처 개요

```text
                ┌───────────────────────────┐
                │   Nullus Rotation Ctrl    │
                │  (scheduler + state machine)
                └──────────────┬────────────┘
                               │
                               │ read/renew/reissue
                               ▼
                      ┌──────────────────┐
                      │ OpenBao (KV/Auth)│
                      └───────┬──────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
            ┌───────▼────────┐  ┌──────▼─────────┐
            │ ESO / CSI Driver │  │ Direct Fetch   │
            │ (권장)            │  │ (예외)         │
            └───────┬─────────┘  └──────┬─────────┘
                    │                    │
                    ▼                    ▼
             K8s Secret(파생)         App Runtime
             (원문 저장소 아님)       (reload/restart)
```

---

## 4. 토큰 유형별 전략

| 유형 | 특징 | 전략 | 기본 동작 |
|---|---|---|---|
| Lease 기반 토큰 | TTL/lease_id 존재 | **Renew 우선** | 만료 전 임계 시점에 renew |
| 고정 수명 토큰 | 재발급 API 필요 | **Reissue 후 교체** | 새 토큰 발급 -> OpenBao 저장 -> 주입 |
| 수동 승인 토큰 | 승인 절차 필요 | **반자동** | 사전 경고 + 승인 후 재발급 |

권장 임계값:

- `T_expire - now <= 20% TTL` 또는 `<= 24h`면 갱신 시도
- 실패 시 지수 백오프 재시도: `1m -> 5m -> 15m -> 1h`

---

## 5. 상태 머신

```text
REGISTERED
  -> HEALTHY
  -> RENEW_DUE
  -> RENEWING
      -> ROTATED (success)
      -> FAILED_RETRYABLE
          -> RENEWING (backoff)
      -> FAILED_MANUAL
          -> AWAITING_APPROVAL
              -> RENEWING
  -> EXPIRED (critical)
```

상태 정의:

- `HEALTHY`: 만료 여유 충분
- `RENEW_DUE`: 임계 구간 진입
- `ROTATED`: 새 버전 반영 완료
- `FAILED_RETRYABLE`: 일시 실패(네트워크/RateLimit)
- `FAILED_MANUAL`: 수동 승인 또는 운영자 개입 필요
- `EXPIRED`: 만료로 인한 위험 상태

---

## 6. 데이터 모델 (제안)

### 6.1 `token_sources`

| 컬럼 | 타입 | 설명 |
|---|---|---|
| `id` | UUID | 식별자 |
| `org_id` | UUID | 조직 |
| `module` | TEXT | auth/stack/cicd/observability |
| `provider` | TEXT | keycloak/github/gitlab/harbor/slack 등 |
| `path` | TEXT | OpenBao path |
| `token_type` | TEXT | lease/reissue/manual |
| `status` | TEXT | 상태 머신 상태 |
| `expires_at` | TIMESTAMPTZ | 만료 시각 |
| `last_rotated_at` | TIMESTAMPTZ | 마지막 갱신 |
| `next_check_at` | TIMESTAMPTZ | 다음 점검 시각 |
| `requires_approval` | BOOLEAN | 수동 승인 필요 여부 |
| `created_at`/`updated_at` | TIMESTAMPTZ | 공통 |

### 6.2 `token_rotation_events`

| 컬럼 | 타입 | 설명 |
|---|---|---|
| `id` | UUID | 식별자 |
| `token_source_id` | UUID | 대상 |
| `event_type` | TEXT | renew/reissue/fail/approve/apply |
| `result` | TEXT | success/failure |
| `reason` | TEXT | 실패 이유 코드 |
| `detail_json` | JSONB | provider 응답 요약(민감값 제외) |
| `trace_id` | TEXT | 추적 ID |
| `created_at` | TIMESTAMPTZ | 발생 시각 |

---

## 7. API 계약 (제안)

### 7.1 조회

- `GET /api/v1/admin/token-sources`
  - 필터: `status`, `module`, `provider`, `org_id`
- `GET /api/v1/admin/token-sources/:id/events`

### 7.2 수동 제어

- `POST /api/v1/admin/token-sources/:id/rotate`
  - 즉시 갱신 트리거
- `POST /api/v1/admin/token-sources/:id/approve`
  - 수동 승인 상태에서 진행
- `POST /api/v1/admin/token-sources/:id/pause`
  - 자동 갱신 일시 정지
- `POST /api/v1/admin/token-sources/:id/resume`

### 7.3 에러 코드 (제안)

- `TOKEN_ROTATE_PROVIDER_UNAVAILABLE`
- `TOKEN_ROTATE_RATE_LIMITED`
- `TOKEN_ROTATE_APPROVAL_REQUIRED`
- `TOKEN_ROTATE_POLICY_DENIED`
- `TOKEN_ROTATE_EXPIRED`

---

## 8. 배포/주입 방식

권장 순서:

1. OpenBao 배포 + auth/kubernetes 설정
2. token source 등록(`token_sources`)
3. ESO/CSI 리소스 생성
4. 앱 배포(시크릿 참조)
5. Rotation Controller 활성화

주입 원칙:

- K8s Secret은 파생 리소스
- 앱 로그에 시크릿 원문 출력 금지
- reload 가능 앱은 SIGHUP/동적 재로딩, 불가하면 rolling restart

---

## 9. 관측성/알림

필수 메트릭:

- `token_rotation_total{result,provider,module}`
- `token_rotation_duration_seconds`
- `token_expiry_seconds{provider,module}`
- `token_rotation_retries_total`

필수 알림:

- `EXPIRED` 진입 즉시 P0 알림
- `FAILED_RETRYABLE` N회 초과 시 P1 알림
- `FAILED_MANUAL` 진입 시 승인 요청 알림

---

## 10. 보안/감사

- OpenBao role은 모듈/조직 단위 최소 권한(path 제한)
- Controller는 write 권한 최소화(필요 path만)
- 회전 이벤트는 `audit_logs` + `token_rotation_events` 이중 기록
- 로그/이벤트에 원문 토큰 금지, 해시/메타데이터만 저장

---

## 11. 실패 시나리오와 대응

1. Provider API 429
   - 백오프 + jitter 재시도
2. OpenBao 접근 권한 오류
   - policy 점검 알림 + 자동 중단
3. 앱 반영 실패
   - 이전 버전 유지 + 롤백 이벤트 기록 + 운영자 알림
4. 만료 임박 대량 발생
   - priority queue로 provider별 처리량 제한

---

## 12. 구현 단계 제안

### Phase 1 (MVP)

- token source 등록/조회
- lease 기반 renew + 이벤트 기록
- 실패 재시도/알림

### Phase 2

- reissue 플러그인(provider adapter)
- 승인 워크플로우
- 앱별 무중단 반영 최적화

### Phase 3

- 정책 자동 검증
- 대규모 회전 부하 제어
- 회전 SLO 대시보드

---

## 13. 운영 체크리스트

- [ ] OpenBao health check/backup 검증
- [ ] Kubernetes auth role/path 최소권한 검증
- [ ] 만료 7일 이내 토큰 주간 점검
- [ ] EXPIRED 0건 목표 유지
- [ ] 분기별 회전 복구 리허설

---

## 14. 예시 경로 표준

```text
kv/nullus/prod/org-a/auth/keycloak/client-secret
kv/nullus/prod/org-a/cicd/github/webhook-token
kv/nullus/prod/org-a/shared/db/postgres-password
kv/nullus/staging/org-a/observability/grafana/admin-password
```

---

## 15. 결정 사항 요약

1. 원문 시크릿 저장소는 OpenBao로 통일
2. 토큰 만료 대응은 renew/reissue 이원 전략
3. 앱 반영은 무중단 우선, 불가 시 롤링 재기동
4. 감사/알림/재시도는 기본 내장 기능으로 제공
