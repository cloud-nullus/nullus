# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- CI/CD 파이프라인이 kind 클러스터에 실제 K8s 리소스(Deployment, Service, Namespace)를 생성
- CI/CD developer-deploy 위저드 5단계 전체 플로우 연결 (앱 이름 → Git → 클러스터 → 리소스 → 환경변수 → Deploy)
- 위저드 Step 4에 Replicas 슬라이더 추가 (1~5, 기본값 2)
- 배포 진행 화면에 Deploy Output 터미널 박스 추가 (kubectl 명령어 및 결과 실시간 표시, 색상 구분)
- 배포 완료 시 생성된 K8s 리소스 목록 표시 및 `kubectl get` 확인 명령어 복사 기능 (`--context` 포함)
- 앱 템플릿 선택 시 Git Repository URL 자동 입력 (`sample-go-api`, `sample-react-app`, `sample-spring-boot`)
- `DeployStep`에 `Logs` 필드 추가, `StepTracker.AppendLog`로 스텝별 kubectl 로그 축적
- 인메모리 `StepTracker`로 배포 단계별 진행 상태 추적 (30초 후 자동 정리)
- GET `/cicd/deployments/:id` 엔드포인트 (배포 상태 + 스텝 로그 병합)
- CI/CD List 상세 패널 4개 탭: Info (Pipeline + Target + Stages + Variables), Monitoring, History, Actions
- `DataTable`에 `renderExpanded` prop 추가 (행 아래 인라인 상세 패널)
- CI/CD History 페이지에서 특정 파이프라인 배포 이력만 필터링 (`?pipeline=<id>`)
- CI/CD List 파이프라인 상세 패널의 Logs 버튼이 해당 파이프라인 이력으로 바로 이동
- 배포 시 현재 로그인 사용자가 `deployed_by`로 자동 기록
- Helm 차트 ServiceAccount 템플릿 추가
- API Deployment에 wait-for-db initContainer 추가 (PostgreSQL 준비 대기)
- CI/CD kind 클러스터 배포 시연 가이드 (`docs/guides/cicd-pipeline-kind-deploy-guide.md`)
- 시행착오 및 해결 방법 레퍼런스 (`docs/agent-reference.md`)

### Changed
- `DeployPipeline` usecase를 `Start`(동기, DB 저장) + `ApplyAsync`(비동기, K8s 배포)로 분리, HTTP 202 즉시 반환
- `ManifestApplier.ApplyWithTracking`이 각 매니페스트 적용 결과와 로그를 `StepTracker`에 기록
- CI/CD List/History 페이지가 각 항목 아래 인라인 상세 패널로 변경 (하단 패널 → 행 아래 인라인)
- CI/CD History에서 Rollback 기능 전체 제거 (백엔드 미구현)
- CI/CD List/History 페이지가 백엔드 API 응답을 정확히 매핑 (앱 타입, 클러스터명, 상태, 배포일)
- CI/CD List 테이블에서 Deploy 버튼 제거 (상세 패널의 Run으로 통합)
- `go-web-api` 템플릿 이미지를 빌드 이미지에서 런타임 서버로 변경 (`nginx:alpine`)
- Migration Job을 pre-install Hook에서 외부 마이그레이션 패턴으로 전환

### Fixed
- Breadcrumb에서 동일한 key `/cicd/list`가 2회 사용되어 React 경고 발생하던 문제
- Dockerfile Go 버전이 `go.mod`와 불일치 (`1.24` → `1.26`)
- `web/Dockerfile` 빌드 컨텍스트 경로 오류 (`web/nginx.conf` → `nginx.conf`)
- `web/Dockerfile` npm ci peer dependency 충돌 (`--legacy-peer-deps` 추가)
- API Deployment에 ConfigMap 볼륨 마운트 누락으로 config 파일을 찾지 못하던 문제

## [0.2.0-alpha] - 2026-03-28

### Added
- Stack Install 5단계 Wizard 완성 (Resource Planning, Storage Plan, YAML View, Deploy Script, Dry Run)
- Stack List 상세 패널 (커넥션 정보, 상태, 인라인 디테일)
- Helm Orchestrator 다중 Phase DAG 실행 및 실제 Helm install/upgrade/rollback
- Helm Values Generator (Stack config → values.yaml 자동 생성)
- Stack Monitoring API 엔드포인트
- OSS Resource Defaults 도메인 전체 구현 (Entity, Port, Repository, UseCase)
- Stack Create/Delete UseCase 분리 및 확장
- OSS Resource Default 관리 페이지 (Admin 전용)
- Go 테스트 커버리지 검증 스크립트 (`scripts/check-coverage.sh`)
- DB 마이그레이션 000022-000028 (리소스 기본값, 템플릿 버전 정렬, 상태 enum 확장, 목업 스택)

### Changed
- Stack Install 최종 배포 검토 흐름을 YAML View → Deploy Script → Dry Run 단계로 확장
- Stack Name 하단에 Access Domain 입력란 추가 (`{StackName}.internal`)
- OSS 버전 메타데이터가 템플릿 편집 시에도 보존되도록 API 정규화
- GitLab 계열 OSS를 단일 Helm 번들로 통합
- Mock 데이터 fallback을 전 페이지에서 제거 (API 실패 시 빈 배열 반환)
- Stack Template 카드의 접근성 개선 (중첩 button → `div[role="button"]`)
- 마이그레이션 파일 번호 정리 및 중복 방지

### Fixed
- Organization 생성 시 DB 미저장 (API 경로 `/admin/organizations` → `/admin/orgs`)
- Mock auth ORG_ID/User ID가 DB 시드와 불일치하여 org 기반 API 실패
- CORS `AllowHeaders`에 `X-Org-ID` 누락
- 스택 시드 config JSON이 `StackConfig` 구조체와 불일치하여 unmarshal 실패
- 스택 히스토리 diff 초기값 하드코딩 → 최신 2개 버전 자동 선택
- 스택 히스토리 스냅샷 null 크래시 → null-safe 처리
- 클러스터 `unreachable`/`auth_failed` 상태 매핑 누락
- 개발 모드에서 rate limiter가 프론트엔드 요청 차단 → 프로덕션 전용으로 변경

## [0.1.0-alpha] - 2026-03-15

### Added
- Organization 설정 등록 — PostgreSQL Repository, CRUD API
- K8s Cluster 등록/검증 — client-go 검증 어댑터, Kubeconfig AES-256-GCM 암호화
- DevSecOps Stack 설정 5단계 Wizard — React Hook Form + Zod 검증
- Golden Path 템플릿 3종 — PostgreSQL Repository, Seed 데이터
- Stack 자동 설치/배포/이력 — Helm SDK 3-Phase DAG, WebSocket 로그 스트리밍, Rollback
- CI/CD Pipeline 템플릿 — Pipeline 템플릿, K8s Manifest Generator
- CI/CD Pipeline 배포/이력 — client-go Dynamic Applier, 배포 추적
- 모니터링/알림 — Prometheus HTTP Client, Dashboard, Alert CRUD
- 버전 호환성 관리 — 호환성 매트릭스, 검증 API, JSONB Diff
- UI 권한 체계 — Keycloak OIDC JWT, dual-mode 인증, 라우트별 RBAC
- 리소스 예상량 계산 — 리소스 계산기, 비용 추정
- React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui 프론트엔드 (15개 페이지)
- Go 1.26 + Echo v4 + PostgreSQL 백엔드 (Clean Architecture + DDD, 5 Bounded Context)
- GitHub Actions CI (Go test + Vite build + Vitest + Playwright E2E)
- Docker 멀티스테이지 빌드 + Helm 차트
- testcontainers-go 통합 테스트 인프라
- 로컬 개발 환경 스크립트 (`runbook_local.sh`)

### Fixed
- 프론트엔드-백엔드 호환성: 템플릿 tools 매핑, cluster status 필드 통일
- PostgresOrgRepository NULL default_admin_id 스캔 오류
- estimated_install_time 나노초→분 변환 오버플로우

[unreleased]: https://github.com/cloud-nullus/draft/compare/v0.2.0-alpha...HEAD
[0.2.0-alpha]: https://github.com/cloud-nullus/draft/compare/v0.1.0-alpha...v0.2.0-alpha
[0.1.0-alpha]: https://github.com/cloud-nullus/draft/releases/tag/v0.1.0-alpha
