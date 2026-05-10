# OpenBao 구현 체크리스트

**목적**: Nullus에 OpenBao-first 시크릿 관리, 자동 토큰 갱신, 관리자 step-up 조회를 구현하기 위한 실행 체크리스트

## 현재 구현 반영 (2026-05-10)

- [x] `authentication.provider=openbao` 선택 시에만 OpenBao 배포 step(`installing_openbao`) 실행
- [x] Stack Gateway 기본 번들에 `openbao.<access_domain>` HTTPRoute 자동 생성
- [x] OpenBao UI 접속 경로 확보 (`/ui/`)
- [x] 토큰 소스 저장 시 `metadata.secret_manager=openbao` 저장 및 path 정규화(`kv/nullus/dev/...`)
- [x] Admin `reveal` API가 placeholder 대신 OpenBao 실조회 값을 우선 반환
- [x] Secret backend 추상화 계층(`internal/shared/secrets`) 도입으로 provider 확장 포인트 확보
- [ ] OpenBao HA/스토리지/TLS 운영 구성을 공식 배포 스펙으로 전환 (현재 dev 모드)
- [ ] reissue/lease/manual provider별 실제 회전 어댑터 구현
- [ ] OpenBao preflight gate와 배포 차단 정책 완성

---

## 1) 아키텍처/보안 기준 확정

- [ ] OpenBao를 시크릿 원천(Source of Truth)으로 공식 확정
- [ ] Kubernetes Secret은 파생 주입용으로만 사용 정책 확정
- [ ] 로컬(`.env.dev`) fallback 허용 범위(dev only) 문서화
- [ ] 토큰 유형 분류(lease/reissue/manual) 완료
- [ ] 토큰 조회/회전 권한 매트릭스(Admin 전용 액션) 확정

## 2) OpenBao 인프라 준비

- [ ] OpenBao HA/스토리지/TLS 구성
- [ ] `auth/kubernetes` 설정
- [ ] 환경별 path 설계(`kv/nullus/{env}/{org}/{module}/{app}/{secret}`)
- [ ] Role/Policy 최소권한 적용
- [ ] 백업/복구(snapshot/replication) 검증

## 3) DB/도메인 모델 구현

- [ ] `token_sources` 테이블 생성 마이그레이션
- [ ] `token_rotation_events` 테이블 생성 마이그레이션
- [ ] 상태머신 enum/상수 구현(healthy/renew_due/renewing/rotated/failed/expired)
- [ ] 인덱스/유니크 제약 반영
- [ ] 원문 토큰 DB 저장 금지 검증 테스트

## 4) 백엔드 API 구현

- [ ] `GET /api/v1/admin/token-sources` 구현
- [ ] `GET /api/v1/admin/token-sources/:id/events` 구현
- [ ] `POST /api/v1/admin/token-sources/:id/rotate` 구현
- [ ] `POST /api/v1/admin/token-sources/:id/approve` 구현
- [ ] `POST /api/v1/admin/token-sources/:id/pause`, `/resume` 구현
- [ ] `POST /api/v1/admin/token-sources/:id/re-auth` 구현(step-up)
- [ ] `POST /api/v1/admin/token-sources/:id/reveal` 구현(step_up_token 필수)
- [ ] 신규 에러코드 반영 및 표준화

## 5) Rotation Controller 구현

- [ ] 스케줄러(next_check_at 기반) 구현
- [ ] lease 토큰 renew 로직 구현
- [ ] reissue provider adapter 인터페이스 구현
- [ ] 실패 백오프(1m -> 5m -> 15m -> 1h) 구현
- [ ] 임계치 초과 알림 트리거 구현
- [ ] 만료(EXPIRED) 긴급 처리 분기 구현

## 6) 주입/반영 파이프라인

- [ ] ESO 또는 CSI 방식 선택/확정
- [ ] OpenBao -> K8s 주입 경로 구성
- [ ] 앱별 reload/rolling restart 전략 구현
- [ ] 배포 전 OpenBao preflight gate 구현
- [ ] 반영 성공 검증(토큰 사용 API 헬스체크) 자동화

## 7) 관리자 step-up 조회 보안

- [ ] step-up 인증 방식(비밀번호 재입력/OIDC step-up) 확정
- [ ] step_up_token TTL(권장 5분) 구현
- [ ] reveal 기본 masked, full은 정책 허용 시만
- [ ] 조회/복사 rate limit 적용
- [ ] 조회 이벤트(audit) 강제 기록

## 8) 관측성/감사/운영

- [ ] 메트릭: `token_rotation_total`, `duration`, `expiry_seconds` 추가
- [ ] 알림: FAILED_MANUAL/EXPIRED/P0 경고 연결
- [ ] `audit_logs`에 re-auth/reveal/rotate 이벤트 기록
- [ ] 운영 런북에 실패 대응/승인/롤백 절차 반영
- [ ] 분기별 회전 리허설 절차 수립

## 9) 테스트

- [ ] 단위 테스트: 상태전이/백오프/권한검증
- [ ] 통합 테스트: OpenBao mock + DB + API
- [ ] E2E: 만료 시나리오 -> 자동 갱신 -> 앱 반영 성공
- [ ] E2E: 실패 시나리오(429/권한오류/네트워크단절)
- [ ] 보안 테스트: 원문 노출 방지(로그/응답/이벤트)

## 10) 문서/릴리스

- [ ] API 문서(OpenAPI) 갱신
- [ ] DB 스키마 문서 반영 확인
- [ ] 기능목록/운영가이드/온보딩 문서 정합성 확인
- [ ] 릴리스 노트에 breaking/non-breaking 영향 명시
- [ ] 운영팀 핸드오프 완료
