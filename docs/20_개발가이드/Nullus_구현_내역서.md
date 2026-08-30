# Nullus 구현 내역서

**작성일**: 2026-03-22
**최종 갱신**: 2026-08-31
**범위**: Phase 1 (갭 해결) + Phase 2 (OIDC 추상화, D11, 컨벤션)

> 2장 이하는 3월 시점의 Phase 1·2 작업 기록이다. 그 뒤의 누적 구현 현황은 아래 0장에
> 요약했고, 변경 이력은 `CHANGELOG.md` 가 단일 출처다.

---

## 0. 구현 현황 요약 (2026-08-31)

### 0.1 백엔드 모듈

Clean Architecture 4계층(`domain` / `usecase` / `port` / `adapter`)을 모듈마다 유지한다.

| 모듈 | 구현 파일 | 테스트 파일 | 계층 |
|------|-----------|-------------|------|
| `internal/stack` | 116 | 130 | adapter, domain, port, usecase |
| `internal/cicd` | 69 | 56 | adapter, domain, port, usecase |
| `internal/admin` | 43 | 23 | adapter, domain, port, usecase, rotation, scheduler |
| `internal/shared` | 24 | 11 | audit, config, domain, externalsecret, middleware, notification, secrets |
| `internal/observability` | 23 | 12 | adapter, domain, port, usecase |
| `internal/auth` | 21 | 21 | adapter(keycloak/authentik/middleware/token/repository), domain, port, usecase |
| `internal/cli` | 2 | 1 | 통합 `nullus` CLI 명령 트리 |

`internal/` 밖의 공유 패키지 — CLI·MCP 가 import 해야 해서 `pkg/` 에 둔다.

| 패키지 | 구현 | 테스트 | 역할 |
|--------|------|--------|------|
| `pkg/nullusclient` | 6 | 5 | API 클라이언트, 설정·토큰 해석, OIDC 획득·갱신, 서버 버전 스큐 판정 |
| `pkg/crypto` | 1 | 1 | AES-256-GCM (kubeconfig) |

프로덕션 Go 코드 약 **52,900 라인**(2026-08-11 약 38,700). 바이너리는 4개다 —
`cmd/api`, `cmd/nullus`, `cmd/nullus-bootstrap`, `cmd/token-source-sync`.

프론트엔드 feature 는 `admin`, `auth`, `cicd`, `common`, `home`, `observability`,
`stack`, **`tour`** 8개다.

### 0.2 외부 연동 현황

| 영역 | 지원 |
|------|------|
| 소스 저장소 | GitLab(스택 내 설치), **Gitea(스택 내 설치)**, GitHub(외부 SaaS) |
| CI | GitLab CI, **Jenkins(스택 내 설치)**, GitHub Actions |
| 컨테이너 레지스트리 | Harbor, Nexus, GitLab Container Registry, GHCR |
| CD | Argo CD |
| 오브젝트 스토리지 | MinIO |
| 시크릿 | OpenBao(스택마다 배포, Kubernetes Auth) + External Secrets Operator |
| 인증(플랫폼) | Keycloak / Authentik OIDC + **ID/PW 로그인**(IdP 장애 대비) |
| 인증(스택 OSS) | **Argo CD·Harbor·GitLab·Gitea·Jenkins·Grafana·MinIO → Keycloak SSO**, 도구마다 비밀번호 우회로 유지 |
| 모니터링 | Prometheus, Grafana, Loki / OpenSearch, Tempo / Jaeger, **OpenTelemetry Collector·Agent** |
| 게이트웨이 | Envoy Gateway (Gateway API), 스택 네임스페이스 + 브리지 Ingress |
| TLS | cert-manager. **와일드카드 인증서를 DNS-01 로 발급**해 기본 인증서로 지정 |

### 0.3 API·스키마 규모

- REST 엔드포인트 **116개** (모듈별 내역은 `Nullus_API_설계.md` 6.0)
- DB 테이블 **23개**, 마이그레이션 **000074** 까지 (`Nullus_DB_스키마.md` 0장)
- Golden Path 템플릿 **9개**, 호환성 매트릭스 **8개**, CI/CD 파이프라인 템플릿 **4개**
- 프론트 화면 **28개**(전부 라우팅됨), 사이드바 항목 18개

### 0.4 알아 둘 제약

- **알림은 발송되지 않는다.** 규칙·채널 CRUD 와 notifier 구현체는 있으나 잇는 코드가 없다.
- **호환성 매트릭스 관리 API 가 등록되지 않았다.** 관리자 화면의 생성·수정·삭제 버튼이
  404 로 끝나고, 매트릭스 추가는 마이그레이션 시드로만 가능하다.
- **한 클러스터에 스택은 하나만 선다.** OpenBao ClusterRoleBinding 이 클러스터 범위라
  두 번째 설치는 소유권 충돌로 막힌다(의도된 차단).

상세는 `docs/20_아키텍처/Nullus_설계_대비_미구현_항목.md`.

---

## 1. 프로젝트 개요

**Nullus Platform**: Kubernetes 기반 DevSecOps 자동화 오픈소스 플랫폼

| Phase | 목표 | 브랜치 | PR |
|-------|------|--------|-----|
| Phase 1 | 역할별 시나리오 vs 프론트엔드 갭 해결 (13건) | `feat/gap-resolution` | #3 |
| Phase 2 | OIDC 추상화 + D11 도구 추가 + 컨벤션 문서화 | `feat/phase-2` | #4 |

---

## 2. Phase 1 — 갭 해결 (feat/gap-resolution)

### 배경

역할별 사용 시나리오 문서(`Nullus_역할별_사용시나리오.md`)와 실제 프론트엔드 구현 간 갭 분석 결과, 13건의 불일치를 발견하여 해결.

### Wave 1 — 6개 태스크 (병렬)

| ID | 심각도 | 기능 | 변경 파일 | 설명 |
|----|--------|------|----------|------|
| T1 | 📝 | 시나리오 문서 수정 | `Nullus_역할별_사용시나리오.md` | 메뉴 가시성(Admin 전체 접근), DevOps 초기 화면(스택 템플릿), Developer 위자드(5단계), 탭 명칭(Observability), D11 Phase 2 이관 |
| T2 | 🟡 | 리소스 탭 강화 | `stack-install-page.tsx`, `stack-config-store.ts` | 통화 선택 드롭다운(USD/KRW/CNY), Auto/Manual 리소스 모드 토글 |
| T3 | 🟡 | Kubeconfig 파일 업로드 | `cluster-page.tsx` | `<input type="file">` 추가 (기존 textarea 병행), 1MB 크기 제한, 확장자 검증 |
| T4 | 🔴 | CI/CD 롤백 UI | `cicd-history-page.tsx`, `cicd-api.ts` | Rollback 버튼(success/failed만), diff 모달, "ROLLBACK" 텍스트 확인, `useRollbackDeployment` 훅 |
| T5 | 🟡 | 초대 링크 플로우 | `user-management-page.tsx`, `admin-api.ts` | "Generate Invite Link" 모달, 역할+만료 설정, Copy Link, 대기 목록 테이블, Revoke |
| T6 | 🟡 | 버전 Diff 강화 | `version-diff.tsx` | git-diff 스타일 unified diff, 추가/삭제/변경 색상 코딩, 라인 번호, 접기/펼치기 |

### Wave 2 — 2개 태스크 (병렬)

| ID | 심각도 | 기능 | 변경 파일 | 설명 |
|----|--------|------|----------|------|
| T7 | 🟡 | PVC 보존 옵션 | `stack-history-page.tsx`, `cicd-history-page.tsx` | Safe Mode(기본)/Clean Mode 라디오, Clean 시 "DELETE" 입력 필수, `preservePVC` API 파라미터 |
| T8 | 🟢 | Monaco YAML 에디터 | `stack-install-page.tsx`, `vite.config.ts` | `@monaco-editor/react` + `monaco-yaml`, 양방향 동기화(300ms 디바운스), 다크/라이트 테마, Copy/Format 버튼 |

### Wave 3 — 1개 태스크

| ID | 심각도 | 기능 | 변경 파일 | 설명 |
|----|--------|------|----------|------|
| T9 | 🟢 | Keycloak OIDC | `auth-store.ts`, `login-page.tsx`, `protected-route.tsx`, `sidebar.tsx`, `main.tsx` | `react-oidc-context` + `oidc-client-ts`, dual-mode(mock+OIDC), `VITE_AUTH_MODE` 환경변수, 역할 추출, silent renew |

---

## 3. Phase 2 — OIDC 추상화 + D11 + 컨벤션 (feat/phase-2)

### Workstream A: OIDC Provider 추상화

**목표**: Keycloak 하드코딩 → Keycloak/Authentik 환경변수 기반 전환

#### Frontend 변경

| 파일 | 변경 내용 |
|------|----------|
| `web/src/lib/oidc-providers.ts` (신규) | `OIDCProviderConfig` 인터페이스, `keycloakExtractRoles`(realm_access.roles), `authentikExtractRoles`(groups), `getProviderConfig()` factory, `toAuthProviderProps()` |
| `web/src/lib/oidc-config.ts` (신규) | oidc-providers.ts re-export (하위 호환) |
| `web/src/stores/auth-store.ts` | `extractRoleFromOidc()` — provider config 기반 역할 추출 |
| `web/src/main.tsx` | `isOidcMode` 조건부 `AuthProvider` 래핑 |
| `web/src/features/auth/pages/login-page.tsx` | `OidcLoginContent` / `MockLoginContent` 분리, provider 이름 동적 표시 |
| `web/src/components/layout/sidebar.tsx` | Authentik 커스텀 logout (removeUser → manual redirect) |
| `web/.env.example` (신규) | `VITE_AUTH_MODE`, `VITE_OIDC_PROVIDER`, `VITE_OIDC_AUTHORITY`, `VITE_OIDC_CLIENT_ID` |

#### Backend 변경

| 파일 | 변경 내용 |
|------|----------|
| `internal/auth/port/oidc_provider.go` (신규) | `OIDCProvider` 인터페이스 — `ExtractRoles(claims) []string`, `Name() string` |
| `internal/auth/adapter/keycloak/oidc_provider.go` (신규) | `realm_access.roles` 중첩 객체 추출 |
| `internal/auth/adapter/authentik/oidc_provider.go` (신규) | `groups` 최상위 배열 추출 |
| `internal/auth/adapter/provider_factory.go` (신규) | `NewOIDCProvider("keycloak"\|"authentik")` factory |
| `internal/auth/adapter/middleware/jwt_middleware.go` | `OIDCProvider` 주입, `m.provider.ExtractRoles(claims)` 사용 |
| `internal/auth/adapter/keycloak/oidc_provider_test.go` (신규) | Keycloak claims 추출 테스트 (5 cases) |
| `internal/auth/adapter/authentik/oidc_provider_test.go` (신규) | Authentik claims 추출 테스트 (5 cases) |
| `internal/auth/adapter/middleware/jwt_middleware_test.go` | Keycloak + Authentik provider 양쪽 테스트 |

### Workstream B: D11 기존 스택 도구 추가

**목표**: 기존 스택에 도구를 추가 설치하는 기능 (새 스택 생성 없이)

#### Backend (Clean Architecture)

| 파일 | 변경 내용 |
|------|----------|
| `internal/stack/domain/stack.go` | `AddTools()` 도메인 메서드 — 중복 검증 + tools 추가 + UpdatedAt 갱신 |
| `internal/stack/domain/stack_test.go` | AddTools 성공/중복 에러 테스트 |
| `internal/stack/usecase/add_tools.go` (신규) | `AddToolsUseCase` — FindByID → AddTools → UpdateTools |
| `internal/stack/usecase/add_tools_test.go` (신규) | 성공/중복/미발견 테스트 (3 cases) |
| `internal/stack/adapter/handler/stack_handler.go` | `PATCH /:stackId/tools` 라우트 + `AddTools` handler |
| `internal/stack/port/repository.go` | `FindByID`, `UpdateTools` 인터페이스 추가 |

#### Frontend

| 파일 | 변경 내용 |
|------|----------|
| `web/src/features/stack/pages/stack-add-tools-page.tsx` (신규) | 3단계 wizard — ① 카테고리 선택(설치됨 표시) ② 도구 설정(기존 disabled) ③ 리뷰 & 배포 |
| `web/src/features/stack/pages/stack-add-tools-page.test.tsx` (신규) | 18개 Vitest 테스트 (wizard 흐름, 선택, API payload, 설치됨 상태) |
| `web/src/features/stack/pages/stack-list-page.tsx` | Actions 컬럼에 "Add Tools" 버튼 추가 |
| `web/src/features/stack/api/stack-api.ts` | `useAddTools` mutation 훅 추가 |
| `web/src/app/routes.tsx` | `/stack/:id/add-tools` 라우트 등록 |

### Workstream C: PR/커밋 컨벤션 문서화

| 파일 | 내용 |
|------|------|
| `docs/20_개발가이드/Nullus_PR_커밋_컨벤션.md` (신규, 495줄) | 브랜치 전략, 커밋 메시지 형식, PR 작성 가이드, 코드 리뷰 기준, 머지 전략, FAQ |

---

## 4. 아키텍처 결정 기록 (ADR)

### ADR-001: OIDC Provider 추상화

- **결정**: 인터페이스 기반 provider 전환 (Keycloak/Authentik/향후 확장)
- **이유**: Keycloak `realm_access.roles` vs Authentik `groups` — claim 경로가 provider마다 다름
- **결과**: FE `getProviderConfig()` + BE `OIDCProvider.ExtractRoles()` 인터페이스로 캡슐화. 새 provider 추가 시 구현체만 작성.

### ADR-002: Dual Auth Mode (mock + OIDC)

- **결정**: `VITE_AUTH_MODE=mock|oidc` 환경변수로 전환
- **이유**: 개발 시 OIDC 서버 없이도 작업 가능해야 함. E2E 테스트도 mock으로 실행.
- **결과**: mock 모드에서 3개 테스트 계정 사용, OIDC 모드에서 실제 IdP 연동.

### ADR-003: D11 부분 설치 (Add Tools)

- **결정**: 기존 wizard UI 패턴 재사용 + 설치된 도구 disabled 표시
- **이유**: 새 wizard를 처음부터 만드는 대신 install-page의 ToolSelector 패턴 활용으로 개발 속도 향상 + UX 일관성
- **결과**: 3단계 wizard (카테고리 선택 → 도구 설정 → 배포). 기존 도구는 회색/disabled.

### ADR-004: Monaco YAML 양방향 동기화

- **결정**: 300ms 디바운스 + ref 기반 루프 방지
- **이유**: Form → YAML → Form 무한 루프 방지, 사용자 타이핑 중 커서 점프 방지
- **결과**: `isExternalUpdate` ref로 동기화 방향 추적. 잘못된 YAML은 폼 업데이트 차단.

### ADR-005: PVC 보존 옵션

- **결정**: Safe Mode(기본) / Clean Mode 분리, Clean은 "DELETE" 입력 필수
- **이유**: 데이터 보호가 기본값이어야 함. 실수로 PV 삭제 방지.
- **결과**: 롤백 모달에 라디오 그룹 + 조건부 경고 + 텍스트 확인.

### ADR-006: Page.route() Mock 전략

- **결정**: 백엔드 미구현 API는 Playwright `page.route()`로 mock하여 프론트엔드 E2E 테스트 실행
- **이유**: FE/BE 개발 속도 차이로 API가 아직 없는 상태에서도 프론트엔드 테스트가 가능해야 함
- **결과**: CI/CD rollback, invite link 등의 E2E 테스트에서 mock 응답 사용.

---

## 5. 테스트 현황

| 영역 | 프레임워크 | 수량 | 범위 |
|------|----------|------|------|
| Go 전체 | `testing` + `testify` | 276 | domain, usecase, handler, middleware (전 모듈) |
| React 단위 | Vitest + Testing Library | 174 | 18개 테스트 파일 (admin, stack, cicd, observability, home, auth) |
| E2E | Playwright | 18 파일 | 전체 사용자 시나리오 |

---

## 6. 신규 의존성

### Frontend (npm)

| 패키지 | 용도 | 추가 시점 |
|--------|------|----------|
| `@monaco-editor/react` | Monaco YAML 에디터 | Phase 1 T8 |
| `monaco-yaml` | YAML 문법 지원 | Phase 1 T8 |
| `yaml` | YAML 파싱/직렬화 | Phase 1 T8 |
| `react-oidc-context` | OIDC 인증 | Phase 1 T9 |
| `oidc-client-ts` | OIDC 클라이언트 라이브러리 | Phase 1 T9 |

### Backend (Go)

신규 Go 의존성 추가 없음. 기존 `golang-jwt/jwt/v5` 활용.

---

## 7. 환경변수 전체 목록

### Frontend (`web/.env.example`)

```bash
VITE_AUTH_MODE=mock              # mock | oidc
VITE_OIDC_PROVIDER=keycloak     # keycloak | authentik
VITE_OIDC_AUTHORITY=http://localhost:8180/realms/nullus
VITE_OIDC_CLIENT_ID=nullus-web
```

### Backend (`configs/config.yaml`)

```yaml
auth:
  mode: session                  # session | oidc
oidc:
  provider: keycloak             # keycloak | authentik
  issuer_url: http://localhost:8180/realms/nullus
  audience: nullus-app
```

---

## 8. 커밋 이력

### Phase 1 (feat/gap-resolution)

| 커밋 | 메시지 |
|------|--------|
| `cc5b6ac` | feat: Wave 1 갭 해결 — 문서 수정, 리소스 모드, kubeconfig 업로드, CI/CD 롤백, 초대 링크, diff 강화 |
| `534cf62` | feat: Wave 2 갭 해결 — PVC 보존 옵션, Monaco YAML 에디터 |
| `d621224` | feat(auth): Keycloak OIDC 인증 연동 (dual-mode: mock + OIDC) |

### Phase 2 (feat/phase-2)

| 커밋 | 메시지 |
|------|--------|
| `f78f2f5` | feat: Wave 1 — OIDC provider 추상화(FE) + PR/커밋 컨벤션 문서화 |
| `e996652` | feat: Wave 2 — OIDC backend 추상화 + D11 도구 추가 페이지 |

---

## 9. 참고 문서

| 문서 | 경로 |
|------|------|
| 역할별 사용 시나리오 | `docs/10_제품기획/Nullus_역할별_사용시나리오.md` |
| OIDC Provider 가이드 | `docs/20_개발가이드/Nullus_OIDC_Provider_가이드.md` |
| PR/커밋 컨벤션 | `docs/20_개발가이드/Nullus_PR_커밋_컨벤션.md` |
| 프론트엔드 아키텍처 | `docs/20_개발가이드/Nullus_프론트엔드_아키텍처_가이드.md` |
| 백엔드 모듈 가이드 | `docs/20_개발가이드/Nullus_백엔드_모듈_개발_가이드.md` |
| API 설계 | `docs/20_아키텍처/Nullus_API_설계.md` |
| 백엔드 상세설계 | `docs/20_아키텍처/Nullus_백엔드_상세설계.md` |
| CLAUDE.md | 프로젝트 루트 — 아키텍처 원칙, TDD, 워크플로우 |
