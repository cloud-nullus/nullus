# OpenBao 구현 체크리스트

**목적**: Nullus에 OpenBao-first 시크릿 관리, 자동 토큰 갱신, 관리자 step-up 조회를 구현하기 위한 실행 체크리스트

**연관 문서**: `OpenBao_시크릿_평면_구축_설계.md` (P1~P3 구축 설계), `OpenBao_토큰_자동_갱신_설계.md` (회전 정책)

## 현재 구현 반영 (2026-07-28)

> P1~P3 core 는 kind 클러스터 통합 테스트로 검증 완료. 상세는
> `OpenBao_시크릿_평면_구축_설계.md` 0장을 참조한다.

- [x] `authentication.provider=openbao` 선택 시에만 OpenBao 배포 step(`installing_openbao`) 실행
- [x] Stack Gateway 기본 번들에 `openbao.<access_domain>` HTTPRoute 자동 생성
- [x] OpenBao UI 접속 경로 확보 (`/ui/`)
- [x] 토큰 소스 저장 시 `metadata.secret_manager=openbao` 저장 및 path 정규화(`kv/nullus/dev/...`)
- [x] `cmd/api/main.go`에 token source handler wiring 완료 및 `rotate/approve/reveal` smoke 검증 완료
- [x] `scripts/seed-token-sources.sh`로 token source 테스트 데이터 seed 분리
- [x] `runbook_local.sh down --volumes`로 PostgreSQL 볼륨까지 초기화하는 경로 추가
- [x] Admin `reveal` API가 placeholder 대신 OpenBao 실조회 값을 우선 반환
- [x] Secret backend 추상화 계층(`internal/shared/secrets`) 도입으로 provider 확장 포인트 확보
- [x] `token_sources` / `token_rotation_events` 마이그레이션 적용 (`000047`)
- [x] Admin token-source API 8종 라우팅 완료 (list/events/rotate/approve/pause/resume/re-auth/reveal)
- [x] Reissue adapter 추상화 및 GitLab/GitHub 어댑터 구현
- [x] dev 모드 폐기 및 공식 차트 + 영속 스토리지 전환 (HA/TLS 는 후속 과제)
- [x] OpenBao preflight gate와 배포 차단 정책 (sealed 미해소 시 설치 중단)
- [x] Rotation scheduler 기동 배선 (`main.go` 에서 기동)
- [ ] 미지원 provider의 회전 fallback 정리 (현재 랜덤 문자열 생성 후 성공 처리)
- [ ] OpenBao HA 구성 및 클러스터 내부 TLS

---

## 구축 3단계 (P1~P3)

상세 설계는 `OpenBao_시크릿_평면_구축_설계.md`를 따른다. 세 단계는 순서가 강제된다.

### P1. 운영 모드 전환

- [x] 자체 OpenBao 매니페스트 폐기, 공식 Helm 차트로 전환 (차트·앱 버전 고정)
- [x] `standalone` + file storage + `dataStorage` PVC 구성
- [x] `StackConfig.Storage.StorageClass` 필드 추가 (리소스 프로파일이 아닌 스택 설정에 저장)
- [x] `GET /api/v1/admin/clusters/:id/storage-classes` 신설 (name/provisioner/is_default/reclaim_policy/volume_binding_mode)
- [x] 설치 마법사에 StorageClass 선택 UI 추가 — 기본 SC 자동 선택, 부재 시 선택 강제, `Retain` 경고
- [x] 선택값을 OpenBao `dataStorage.storageClass`에 적용 (타 구성요소는 순차 전환)
- [x] preflight에서 선택된 StorageClass의 실존 여부 검증
- [x] init Job 구현 — `/v1/sys/init` 확인 후 분기하는 **멱등성 코드로 강제**
- [x] key shares/threshold 기본값 `1/1` 적용 및 설치 옵션 노출
- [x] unseal 사이드카 구현 (`extraContainers`, 로컬 폴링, Secret 볼륨 마운트)
- [x] `openbao-unseal-keys` Secret 생성 및 `resourceNames` 단위 RBAC 제한
- [ ] 설치 완료 화면에서 unseal key / root token 1회 표시 + 다운로드
- [x] preflight gate를 경고에서 **차단 조건으로 승격** (미응답 / sealed 미해소 / 엔진 미구성)
- [x] 스택 삭제 시 OpenBao PVC + `openbao-unseal-keys` Secret 함께 삭제 (한쪽만 잔존 시 재설치 불가)
- [ ] OpenBao 리소스를 삭제 대상에 포함 — 차트 values로 `nullus.io/stack-name` 라벨 주입 또는 명시적 목록 추가
- [ ] 삭제 확인 UI에 "시크릿 영구 삭제·복구 불가" 경고 표기
- [ ] PV `reclaimPolicy=Retain` 환경에서 볼륨이 잔존할 수 있음을 운영 문서에 명시
- [x] `airgap/images/images.txt`의 OpenBao 태그 고정 (`latest` 제거)

### P2. Kubernetes Auth

- [x] 부트스트랩 Job 구현 (engine / auth / policy / role, 멱등)
- [x] KV v2 엔진을 **`kv` 이름으로** enable
- [x] `openbao_store.go`의 mount 재작성 로직(`kv` → `secret`) 제거
- [x] `auth/kubernetes/config`에 `kubernetes_host`만 설정 — `token_reviewer_jwt`·`kubernetes_ca_cert` 미설정
- [x] `nullus-controller-write` / `nullus-eso-read` 정책 작성 (`delete`/`destroy` 미부여)
- [x] `nullus-controller` / `nullus-eso` role 작성 (TTL 1h)
- [x] 백엔드 인증 전환 — TokenRequest → k8s auth login → client_token 캐시/갱신
- [x] 토큰 획득 전략 인터페이스 분리 (`static` 로컬 / `kubernetes` 운영)
- [x] Secret Router 키를 `(provider, stackID)`로 확장, 스택별 지연 생성
- [x] API server proxy 경유 접속 경로 적용
- [x] Rotation scheduler 기동 배선 추가
- [x] root token revoke 및 복구 절차(`operator generate-root`) 런북 반영

### P3. 주입 평면 (ESO)

- [x] ESO 공식 차트 설치 스텝 추가 (버전 고정)
- [x] `SecretStore` (`nullus-openbao`) 생성
- [x] 시크릿 지도 확정 — 경로 / 생성 방식 / 대상 Secret / 소비자 / 재시작 필요 여부
- [x] `provisioning_secrets` 스텝 구현 (생성 → write → ExternalSecret → **K8s Secret 생성 대기**)
- [x] Helm values의 하드코딩 비밀번호를 `existingSecret` 참조로 교체 (5개 지점 중복 제거)
- [x] `token_source_inputs.go`의 bootstrap 하드코딩 문자열 및 placeholder 값 제거
- [x] 설치 스텝 의존성 그래프 재배선
- [x] 구성요소별 `restart_required` 메타데이터 정의 및 회전 컨트롤러 연동 (rolling restart)
- [x] ESO 이미지·차트·values를 에어갭 번들에 추가
- [x] `argocd-secret`을 ESO 단독 소유로 전환 (`configs.secret.createSecret: false`, admin 해시 + OIDC secret 통합)
- [ ] breaking change 영향 범위를 CHANGELOG에 명시

#### P3-SSO. OIDC 연계 (상세: `Nullus_OSS_SSO_자동로그인_설계.md` 7장)

- [x] **Keycloak을 `deploy/helm/nullus` 차트 조건부 의존성으로 추가** (`condition: keycloak.enabled`)
- [ ] realm 부트스트랩을 `setup-keycloak.sh`에서 백엔드 기동 경로로 이관 (멱등)
- [x] `authentication`에 OIDC issuer 필드 추가 (플랫폼 Keycloak / 외부 IdP / 미사용)
- [x] client ID를 `{stack-slug}-{tool}`로 네임스페이싱 (공용 realm 내 충돌 방지)
- [x] `RegisterOIDCClient`에 client secret 파라미터 추가
- [x] **`RegisterOIDCClient`를 upsert로 전환** — 현재 `409 Conflict`를 성공 처리해 회전 시 Keycloak에 반영되지 않음
- [x] `ToolSSOSpec`에 PKCE/webOrigins 추가 (Grafana·Harbor·GitLab은 `S256`, MinIO·ArgoCD 미설정)
- [x] `provisioning_sso` 스텝 신설 + `internal/stack/port` 인터페이스 정의 + `main.go` 주입
- [x] OIDC client secret을 시크릿 지도에 포함 (`kv/.../auth/{client_id}/client-secret`)
- [x] 코드 생성 values에 OSS별 OIDC 블록 추가 — issuer를 accessDomain 기반으로 생성
- [x] values의 client secret을 값이 아닌 참조(`secretKeyRef` 등)로 전환
- [ ] 에어갭 스크립트의 하드코딩 secret 5종(`*-dev-secret`) 제거
- [ ] 에어갭 번들 전체(82개 이미지) 오프라인 종단 검증 — 구성요소별 검증은 완료, 번들 빌드 미수행

#### P4. 에어갭 설치 경로 통합 (P3 완료 후)

- [x] 자기 클러스터 등록(self-registration) 구현 — in-cluster SA 기반, `ClusterType` 확장
- [x] 부트스트랩 인증 경로 구현 (Keycloak service account, 폐기 + 멱등 재발급, CLI `cmd/nullus-bootstrap`)
- [x] `27-install-stacks.sh`를 helm 직접 호출 → 백엔드 API 호출로 재작성
- [x] `30-provision-sso.sh` 폐기 예정 표시 (백엔드 `provisioning_sso`가 대체) — 실제 제거는 에어갭 검증 후
- [x] `22-install-platform-stack.sh`에 Keycloak 차트 이관 안내 추가 — 실제 블록 제거는 에어갭 검증 후

---

> 아래 1)~10)은 전체 범위 체크리스트다. 위의 P1~P3은 그중 즉시 착수할 구축 항목을 순서대로 뽑아낸 것이며, 겹치는 항목은 P1~P3의 서술을 우선한다.

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

- [x] `token_sources` 테이블 생성 마이그레이션 (`000047`)
- [x] `token_rotation_events` 테이블 생성 마이그레이션 (`000047`)
- [ ] 상태머신 enum/상수 구현(healthy/renew_due/renewing/rotated/failed/expired)
- [x] 인덱스/유니크 제약 반영 (`uk_token_sources_org_provider_path` 외 4종)
- [ ] 원문 토큰 DB 저장 금지 검증 테스트

## 4) 백엔드 API 구현

- [x] `GET /api/v1/admin/token-sources` 구현
- [x] `GET /api/v1/admin/token-sources/:id/events` 구현
- [x] `POST /api/v1/admin/token-sources/:id/rotate` 구현
- [x] `POST /api/v1/admin/token-sources/:id/approve` 구현
- [x] `POST /api/v1/admin/token-sources/:id/pause`, `/resume` 구현
- [x] `POST /api/v1/admin/token-sources/:id/re-auth` 구현(step-up)
- [x] `POST /api/v1/admin/token-sources/:id/reveal` 구현(step_up_token 필수)
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
- [x] step_up_token TTL(권장 5분) 구현
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
