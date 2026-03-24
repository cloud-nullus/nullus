# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased] - 2026-03-24

### Fixed

- Organization 생성 시 DB 미저장 — 프론트엔드 API 경로 `/admin/organizations` → `/admin/orgs` 수정
- Mock auth ORG_ID/User ID가 DB 시드와 불일치하여 멤버 조회 등 org 기반 API 실패하던 문제 수정

### Added

- Mock auth 사용자(`@nullus.dev`) 3명 DB 시드 등록 (login-page.tsx TEST_ACCOUNTS와 ID 동기화)
- 데모 조직 2개 추가 (Acme Corp, Startup Labs), 사용자 5명, 클러스터 3개 시드
- 클러스터 `connection_status` 전체 enum 커버 (connected, pending, unreachable, auth_failed)

### Merged

- Phase1 (#10) — Mock fallback 제거, 접근성 개선, 마이그레이션 정리, 포트 설정 통일

### Changed

#### Mock 데이터 제거 (전 페이지)

**변경 이유:**
프론트엔드 페이지들이 API 응답 실패 시 하드코딩된 MOCK 데이터를 fallback으로 사용하고 있었습니다.
이 패턴은 두 가지 문제를 야기합니다:
1. **오류 은폐**: API가 실패해도 화면에 가짜 데이터가 표시되어 문제를 인지하지 못함
2. **개발 혼란**: 실제 API 연동 여부를 UI만으로 판단할 수 없음

권장 전략: API 실패 시 빈 배열(`[]`) 또는 `null` 반환 → 에러 상태를 명시적으로 처리

**변경된 파일:**
- `web/src/features/cicd/pages/cicd-list-page.tsx` — `MOCK_PIPELINES` 제거, fallback `[]`
- `web/src/features/cicd/pages/cicd-template-page.tsx` — `MOCK_CICD_TEMPLATES` 제거, fallback `[]`
- `web/src/features/cicd/pages/cicd-history-page.tsx` — `MOCK_DEPLOYMENTS` 제거, fallback `[]`
- `web/src/features/stack/pages/stack-template-page.tsx` — `MOCK_TEMPLATES` 제거, fallback `[]`
- `web/src/features/stack/pages/stack-history-page.tsx` — `MOCK_STACKS_FOR_HISTORY` 제거, fallback `[]`
- `web/src/features/stack/pages/stack-list-page.tsx` — MOCK 제거, fallback `[]`
- `web/src/features/stack/pages/stack-version-page.tsx` — MOCK 제거, fallback `[]`
- `web/src/features/stack/pages/stack-add-tools-page.tsx` — `MOCK_STACKS` 제거, fallback `null`
- `web/src/features/admin/pages/user-management-page.tsx` — `MOCK_INVITES` 유지 (초대 링크 API 미연동 시 샘플 표시)
- `web/src/features/observability/pages/monitoring-page.tsx` — `MOCK_APPS` 유지 (실제 API 연동 전 시각화 데이터로 사용)

#### 접근성 개선 (stack-template-page.tsx)

**변경 이유:**
HTML 명세상 `<button>` 내부에 `<button>`을 중첩할 수 없음 (interactive content model 위반).
브라우저 콘솔 경고 및 스크린리더 동작 불일치 발생.

**변경 내용:**
- 카드 외곽 `<button>` → `<div role="button" tabIndex={0}>` + `onKeyDown` 핸들러 추가
- 내부 `<Button>` (Use Template)은 유지

#### 마이그레이션 파일 정리

**변경 이유:**
- `000016_sync_org_members` → `000021_sync_org_members`: `phase1`이 이미 `000019_seed_demo_data`, `000020_pipeline_stack_relation`을 사용하므로 번호 충돌 방지
- `000017_fix_healthcheck_enum.up.down.sql` (잘못된 이중 확장자) → `000017_fix_healthcheck_enum.down.sql`로 수정
- `000007_seed_templates.up.sql`: `golden_path_templates` 테이블 CREATE TABLE 구문 추가 (INSERT 전 테이블이 없어 마이그레이션 실패하던 문제 수정)

### Fixed

- `web/src/__tests__/uat-devops-scenario.test.tsx`: `useTemplates` mock을 `data: MOCK_TEMPLATES`로 변경하여 MOCK 제거 후에도 UAT 시나리오 테스트 통과

---

## [0.1.0-alpha] - 2026-03-15

### Core Features (F0-F10)
- Organization 설정 등록 (F0) — Postgres Repository, CRUD API
- K8S Cluster 등록/검증 (F1) — client-go 검증 어댑터, Kubeconfig AES-256 암호화
- DevSecOps Stack 설정 UI (F2) — React 5단계 Wizard, RHF + Zod 검증
- Golden Path 템플릿 (F3) — 3개 템플릿, Postgres Repository, Seed 데이터
- Stack 자동 설치/배포/이력 (F4) — Helm SDK 3-Phase DAG, WebSocket 로그 스트리밍, Rollback
- CI/CD Pipeline 템플릿 (F5) — Pipeline 템플릿, K8s Manifest Generator
- CI/CD Pipeline 배포/이력 (F6) — client-go Dynamic Applier, 배포 추적
- 모니터링/알림 (F7) — Prometheus HTTP Client, Dashboard, Alert CRUD
- 버전 호환성 관리 (F8) — 호환성 매트릭스, 검증 API, JSONB Diff
- UI 권한 체계 (F9) — Keycloak OIDC JWT, dual-mode 인증, 라우트별 RBAC
- 리소스 예상량 계산 (F10) — 리소스 계산기, 비용 추정

### Backend
- Clean Architecture + DDD 모듈 구조 (5 Bounded Context)
- Helm SDK (helm.sh/helm/v3) 기반 설치 엔진, StepExecutor 인터페이스
- PostgreSQL 6종 레포지토리 마이그레이션 + 시드 데이터
- Keycloak SSO 자동 프로비저닝 (GitLab, Grafana, ArgoCD, MinIO)
- Rate Limiting 미들웨어 (sliding window, 4 categories)
- Audit Logging (DB 기반) + 알림 시스템 (Slack/Email)

### Frontend
- React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui
- 15개 페이지 (Admin, Stack, CI/CD, Observability, Developer Self-Service)
- TanStack Query API 연동, React Hook Form + Zod 검증
- Recharts 4종 차트 (CPU, Memory, Pipeline, Pod Status)
- 다국어 지원 (i18n), 역할별 UI 분기, Toast 알림 (sonner)
- Auth 세션 영속성 (sessionStorage), Route Guard, Error Boundary

### Infrastructure
- GitHub Actions CI (Go test + Vite build + Vitest + Playwright E2E)
- Docker 멀티스테이지 빌드 + Helm 차트 (9 templates)
- testcontainers-go 통합 테스트 인프라
- 로컬 개발 환경 스크립트 (runbook_local.sh)

### Fixed
- 프론트엔드-백엔드 호환성: 템플릿 tools[] 매핑, cluster status 필드 통일
- PostgresOrgRepository NULL default_admin_id 스캔 수정
- estimated_install_time 나노초→분 변환 오버플로우 수정
