# Nullus Platform

Kubernetes 기반 DevSecOps 자동화 오픈소스 플랫폼

## 개요

Nullus는 DevOps 엔지니어가 검증된 CI/CD 베스트 프랙티스 조합(Golden Path)을 선택하고, 웹 UI에서 노코드로 설정한 후 한 번의 버튼 클릭으로 Kubernetes 클러스터에 전체 DevSecOps 스택을 자동 설치할 수 있도록 하는 오픈소스 플랫폼입니다.

### 핵심 가치

- **Golden Path 템플릿**: 검증된 CI/CD 도구 조합으로 선택의 어려움 제거
- **노코드 설정**: 웹 UI의 체크박스/드롭다운으로 5단계 설정 워크플로우
- **자동 설치**: 한 번의 Deploy로 전체 스택 자동 설치 및 연동
- **버전 호합성 보장**: 테스트 완료된 도구 버전 조합만 제공
- **빠른 출시**: 플랫폼 구축 시간 90% 단축 (6-18개월 → 며칠 설정 + 1-2시간 설치)

## 기술 스택

- **Frontend**: React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui
- **Backend**: Go 1.24+ (Echo v4) + PostgreSQL 18+
- **Infrastructure**: Docker, Docker Compose, Helm, Kubernetes 1.26+

## Quick Start

### 요구사항

- Docker + Docker Compose 또는 Kubernetes 1.26+
- Go 1.24+ (백엔드 개발 시)
- Node.js 22+ (프론트엔드 개발 시)

### 1. 개발 환경 시작

인프라 기동 및 데이터베이스 마이그레이션:

```bash
make dev
```

이 명령어는 다음을 자동으로 수행합니다:
- Docker Compose로 PostgreSQL, MinIO, Redis 기동
- 데이터베이스 마이그레이션 실행

완료 후 다음 서비스에 접근할 수 있습니다:
- PostgreSQL: `localhost:5433`
- MinIO 콘솔: `localhost:9001`
- Redis: `localhost:6380`

### 2. 백엔드 실행

```bash
make run
```

API 서버가 실행됩니다. 환경 변수는 Makefile에서 자동으로 설정됩니다.

### 3. 프론트엔드 개발 서버

```bash
make web-dev
```

React 개발 서버가 실행됩니다 (Vite 사용).

## 테스트

### Go 단위 및 통합 테스트

```bash
make test
```

### 커버리지 리포트 생성

```bash
make test-cover
```

생성된 `coverage.html`을 브라우저에서 열어 커버리지를 확인할 수 있습니다.

### React 단위 테스트

```bash
make web-test
```

### E2E 테스트 (Playwright)

```bash
cd e2e
npm run test
```

## 코드 품질

### Go 린트

```bash
make lint
```

golangci-lint 규칙에 따라 코드 검사를 수행합니다.

## 데이터베이스

### 마이그레이션 실행

```bash
make migrate-up
```

### 마이그레이션 상태 확인

```bash
make migrate-status
```

### 마이그레이션 되돌리기

```bash
make migrate-down
```

### 데이터베이스 쉘 접속

```bash
make db-shell
```

psql로 직접 데이터베이스에 접속합니다.

## 빌드

### Go 애플리케이션 빌드

```bash
make build
```

바이너리는 `bin/api`에 생성됩니다.

### 프론트엔드 빌드

```bash
make web-build
```

프로덕션 빌드는 `web/dist`에 생성됩니다.

### 전체 빌드

```bash
make all
```

## 정리

### 개발 환경 종료

```bash
make dev-down
```

### 개발 환경 초기화 (볼륨 제거)

```bash
make dev-clean
```

### 빌드 산출물 정리

```bash
make clean
```

## 프로젝트 구조

```
nullus/
├── cmd/                    # 진입점 (main 함수)
│   └── api/               # API 서버
├── internal/              # 내부 모듈 (Clean Architecture 기반)
│   ├── stack/             # 스택 설치 모듈
│   ├── cicd/              # CI/CD 파이프라인 모듈
│   ├── admin/             # 조직/클러스터 관리 모듈
│   ├── auth/              # 인증/SSO 모듈
│   ├── observability/     # 모니터링/로깅 모듈
│   └── shared/            # 공유 타입 및 유틸
├── pkg/                   # 외부에서 import 가능한 패키지
├── db/                    # 데이터베이스 마이그레이션
├── templates/             # Helm 차트 및 설정 템플릿
├── web/                   # React 프론트엔드
├── e2e/                   # Playwright E2E 테스트
└── Makefile              # 개발 명령어
```

## 아키텍처

### 설계 원칙

**Modular Monolith**: 모놀리스로 시작하되, 모듈 경계를 명확히 하여 향후 마이크로서비스로 전환 가능하도록 설계합니다.

**Clean Architecture**: 의존성은 항상 안쪽(도메인)을 향합니다.

```
[Handler/Controller] → [UseCase/Service] → [Domain/Entity]
        ↓                      ↓
  [Repository Interface]  [Domain Logic]
        ↓
  [Repository Impl (DB/API)]
```

**Domain-Driven Design (DDD)**: 5개 Bounded Context로 구성됩니다.

| Context | 모듈 | 핵심 기능 |
|---------|------|----------|
| Stack Management | `internal/stack/` | DevSecOps 스택 설치/관리 |
| CI/CD Pipeline | `internal/cicd/` | 파이프라인 템플릿/배포 |
| Observability | `internal/observability/` | 모니터링/로깅 |
| Organization | `internal/admin/` | 조직/사용자/클러스터 관리 |
| Auth | `internal/auth/` | OIDC/세션 관리 |

## 역할 체계

Nullus는 3가지 역할을 지원합니다:

| 역할 | 권한 | 주요 기능 |
|------|------|----------|
| Admin | 최고 권한 | 조직 설정, 사용자 관리, 클러스터 관리 |
| DevOps Engineer | 플랫폼 구축 | 스택 설치, 클러스터 등록, 파이프라인 템플릿 관리 |
| Developer | 파이프라인 사용 | CI/CD 파이프라인 배포, 모니터링 대시보드 조회 |

## 라이선스

Apache License 2.0

## 커뮤니티

- **GitHub**: [cloud-nullus/draft](https://github.com/cloud-nullus/draft)
- **Issues**: 기능 요청 및 버그 리포트는 GitHub Issues를 통해 진행합니다
- **Discussions**: 아이디어 및 일반 질문은 GitHub Discussions에서 논의합니다

기여 방법은 [CONTRIBUTING.md](./CONTRIBUTING.md)를 참조하세요.

## 다음 단계

- [개발 가이드](./CONTRIBUTING.md) 읽기
- [CLAUDE.md](./CLAUDE.md)에서 아키텍처 원칙 확인
- 문서 읽기: `docs/` 디렉토리 참조
