# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/).

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
