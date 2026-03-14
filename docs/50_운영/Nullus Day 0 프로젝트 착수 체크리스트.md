# Nullus Day 0: 프로젝트 착수 체크리스트

**작성일**: 2026-03-03  
**목적**: 개발 시작(Week 1) 전에 완료해야 할 모든 인프라, 환경, 프로세스 준비 작업  
**담당**: 전체 팀 (역할별 분담)  
**예상 소요**: 3~5일 (팀 병렬 진행 시)

---

## 1. GitHub Organization & Repository

### 1.1 Organization 설정

GitHub Organization `cloud-nullus`는 이미 생성 완료 상태입니다. 아래 설정을 추가합니다.

```
cloud-nullus (Organization)
├── Settings
│   ├── Member privileges
│   │   ├── Base permission: Read (기본)
│   │   ├── Repository creation: Members (멤버도 생성 가능)
│   │   └── Fork: Private only
│   ├── Security
│   │   ├── 2FA: Required for all members
│   │   ├── Dependabot alerts: Enabled
│   │   └── Secret scanning: Enabled
│   └── Billing
│       └── GitHub Team Plan 확인 (Private repo 무제한, Actions 3,000분/월)
│
├── Teams
│   ├── @cloud-nullus/core       (BE-1, BE-2, FE-1, FE-2, DevOps, QA)
│   ├── @cloud-nullus/frontend   (FE-1, FE-2)
│   ├── @cloud-nullus/backend    (BE-1, BE-2)
│   └── @cloud-nullus/reviewers  (BE-1, FE-1, DevOps) ← CODEOWNERS 용
│
└── Repositories (아래 1.2 참조)
```

**담당**: DevOps  
**체크리스트**:
- [ ] Organization 2FA 필수 설정
- [ ] Team 생성 및 멤버 할당
- [ ] Dependabot / Secret scanning 활성화
- [ ] GitHub Actions 사용량 확인 (Team Plan: 3,000분/월)

### 1.2 Repository 구성

모노레포 전략을 사용합니다 (ADR-008).

```
cloud-nullus/nullus              ← 메인 모노레포 (BE + FE + Helm)
cloud-nullus/nullus-helm-charts  ← 사용자 설치용 Helm 차트 (배포 전용)
cloud-nullus/nullus-docs         ← 사용자 문서 사이트 (GitHub Pages)
cloud-nullus/.github             ← Org 레벨 프로필, 기본 템플릿
```

#### 메인 리포지토리 `nullus` 초기 설정

**Branch 전략 (GitHub Flow 변형)**:

```
main ──────────────────────────────────────── 프로덕션 릴리스
  │
  ├── develop ─────────────────────────────── 통합 브랜치 (Beta까지의 기본 브랜치)
  │     │
  │     ├── feat/F0-org-setup              ── 기능 브랜치
  │     ├── feat/F1-cluster-registration
  │     ├── feat/F4-install-engine
  │     ├── fix/F4-helm-timeout
  │     └── chore/ci-pipeline-setup
  │
  └── release/v0.1-alpha ──────────────────── 릴리스 브랜치 (태그 후 삭제)
```

**Branch Protection Rules**:

| 브랜치 | 규칙 |
|---|---|
| `main` | PR 필수, 리뷰 2명, CI 통과 필수, Force push 금지, Admin도 예외 없음 |
| `develop` | PR 필수, 리뷰 1명, CI 통과 필수, Force push 금지 |
| `feat/*` | 제한 없음 (개인 작업 브랜치) |

**CODEOWNERS** (`.github/CODEOWNERS`):

```
# 전체
*                           @cloud-nullus/reviewers

# 백엔드
/cmd/                       @cloud-nullus/backend
/internal/                  @cloud-nullus/backend

# 설치 엔진 (크리티컬 — 2명 리뷰 필수)
/internal/engine/           @BE-1 @DevOps

# 프론트엔드
/web/                       @cloud-nullus/frontend

# Helm 차트
/templates/                 @DevOps
/charts/                    @DevOps

# CI/CD
/.github/workflows/         @DevOps

# DB 스키마 (변경 시 BE 리드 필수 리뷰)
/db/migrations/             @BE-1
```

**담당**: DevOps + 풀스택/QA  
**체크리스트**:
- [ ] `nullus` 리포지토리 생성 (Private → Alpha 이후 Public 전환)
- [ ] `nullus-helm-charts` 리포지토리 생성 (Public)
- [ ] `nullus-docs` 리포지토리 생성 (Public, GitHub Pages 활성화)
- [ ] `.github` 리포지토리 생성 (Org 프로필)
- [ ] Branch protection rules 설정 (main, develop)
- [ ] CODEOWNERS 파일 생성
- [ ] 기본 라벨 설정 (아래 참조)
- [ ] Issue 템플릿 생성 (Bug Report, Feature Request)
- [ ] PR 템플릿 생성

### 1.3 GitHub Labels

```
# 타입
type/bug             #d73a4a    버그
type/feature         #0075ca    새 기능
type/chore           #e4e669    유지보수, 리팩토링
type/docs            #0075ca    문서

# 우선순위
priority/P0-critical #b60205    즉시 수정 (릴리스 블로커)
priority/P1-high     #d93f0b    현 스프린트 내 수정
priority/P2-medium   #fbca04    다음 스프린트
priority/P3-low      #0e8a16    백로그

# 기능 영역
area/frontend        #1d76db
area/backend         #1d76db
area/engine          #1d76db    설치 엔진
area/helm            #1d76db
area/ci-cd           #1d76db
area/docs            #1d76db

# 기능 번호
feat/F0-org          #c5def5
feat/F1-cluster      #c5def5
feat/F2-setup-ui     #c5def5
feat/F3-golden-path  #c5def5
feat/F4-installer    #c5def5
feat/F5-cicd-tmpl    #c5def5
feat/F6-cicd-deploy  #c5def5
feat/F7-monitoring   #c5def5
feat/F8-compat       #c5def5
feat/F9-rbac         #c5def5
feat/F10-resources   #c5def5

# 릴리스
release/alpha        #5319e7
release/beta         #5319e7
release/v1           #5319e7

# 커뮤니티 (Public 전환 후)
good-first-issue     #7057ff
help-wanted          #008672
```

### 1.4 GitHub Projects (프로젝트 보드)

GitHub Projects V2로 스프린트 보드를 구성합니다.

```
Project: "Nullus v0.1 Development"
├── View: Board (Sprint 단위)
│   ├── Backlog
│   ├── Sprint Ready (이번 스프린트 작업 대기)
│   ├── In Progress
│   ├── In Review (PR 생성됨)
│   └── Done
│
├── View: Roadmap (타임라인)
│   ├── Phase A (W1~W4, Alpha)
│   ├── Phase B (W5~W8, Beta)
│   └── Phase C (W9~W12, v1 GA)
│
└── Custom Fields
    ├── Sprint: Sprint 0, 1, 2A, 2B, 3A, 3B, 4A, 5A, 5B, 6
    ├── Feature: F0~F10
    ├── Priority: P0~P3
    ├── Assignee
    ├── Release Target: Alpha / Beta / v1
    └── Story Points: 1, 2, 3, 5, 8, 13
```

**담당**: 풀스택/QA  
**체크리스트**:
- [ ] GitHub Project 생성 (Board + Roadmap 뷰)
- [ ] Custom Fields 설정
- [ ] 초기 Epic 이슈 생성 (기능 0~10, 각각 1개 Epic)
- [ ] Sprint 0 작업 이슈 생성 및 할당

---

## 2. 모노레포 초기화

### 2.1 프로젝트 뼈대 생성

```bash
# 리포지토리 클론
git clone git@github.com:cloud-nullus/nullus.git
cd nullus

# Go 모듈 초기화
go mod init github.com/cloud-nullus/nullus

# 디렉토리 구조 생성
mkdir -p cmd/api
mkdir -p internal/{handler,service,engine/steps,compatibility,repository,middleware,config}
mkdir -p db/{migrations,seed}
mkdir -p web/src/{components,pages,stores,hooks,api,types}
mkdir -p api
mkdir -p templates/{golden-paths,pipelines,compatibility,known-issues}
mkdir -p charts/nullus/{templates}
mkdir -p docs/{user-guide}
mkdir -p scripts
mkdir -p .github/{workflows,ISSUE_TEMPLATE}
```

### 2.2 Go 의존성 초기 설치

```bash
# 웹 프레임워크
go get github.com/labstack/echo/v4

# Kubernetes 클라이언트
go get k8s.io/client-go@latest
go get k8s.io/apimachinery@latest

# Helm SDK
go get helm.sh/helm/v3

# 데이터베이스
go get github.com/lib/pq
go get github.com/golang-migrate/migrate/v4

# WebSocket
go get github.com/gorilla/websocket

# 세션 (Alpha/Beta 인증)
go get github.com/gorilla/sessions

# 설정
go get github.com/spf13/viper

# 로깅
go get go.uber.org/zap

# 테스트
go get github.com/stretchr/testify

# API 문서
go get github.com/swaggo/swag
go get github.com/swaggo/echo-swagger

# 린터
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### 2.3 React 프로젝트 초기화

```bash
cd web

# Vite + React + TypeScript
npm create vite@latest . -- --template react-ts

# 핵심 의존성
npm install zustand                     # 상태 관리
npm install @tanstack/react-query       # 서버 상태 관리
npm install react-router-dom            # 라우팅
npm install axios                       # HTTP 클라이언트
npm install tailwindcss @tailwindcss/vite  # 스타일링
npm install lucide-react                # 아이콘

# 개발 도구
npm install -D eslint @typescript-eslint/eslint-plugin
npm install -D prettier eslint-config-prettier
npm install -D @testing-library/react @testing-library/jest-dom
npm install -D vitest jsdom
npm install -D @storybook/react-vite    # 컴포넌트 문서화
```

### 2.4 루트 설정 파일

**Makefile**:
```makefile
# 주요 타겟만 정의 (상세 구현은 Week 1에서)
.PHONY: help dev build test lint migrate

help:            ## 사용 가능한 명령어 목록
dev:             ## 로컬 개발 환경 실행 (API + Web + DB)
build:           ## 프로덕션 빌드 (Docker 이미지)
test:            ## 전체 테스트 실행
lint:            ## 린터 실행 (Go + TypeScript)
migrate-up:      ## DB 마이그레이션 적용
migrate-down:    ## DB 마이그레이션 롤백
seed:            ## 시드 데이터 삽입
```

**docker-compose.yaml** (로컬 개발용):
```yaml
version: '3.8'
services:
  db:
    image: postgres:18-alpine
    environment:
      POSTGRES_DB: nullus
      POSTGRES_USER: nullus
      POSTGRES_PASSWORD: nullus_dev
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  api:
    build:
      context: .
      dockerfile: Dockerfile.dev
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://nullus:nullus_dev@db:5432/nullus?sslmode=disable
      KUBECONFIG_ENCRYPTION_KEY: dev-only-key-32-bytes-long!!!!!
    depends_on:
      - db
    volumes:
      - .:/app

  web:
    build:
      context: ./web
      dockerfile: Dockerfile.dev
    ports:
      - "3000:3000"
    environment:
      VITE_API_URL: http://localhost:8080
    volumes:
      - ./web:/app
      - /app/node_modules

volumes:
  pgdata:
```

**담당**: 풀스택/QA (뼈대), BE-1 (Go 설정), FE-1 (React 설정)  
**체크리스트**:
- [ ] 디렉토리 구조 생성
- [ ] Go 모듈 초기화 + 의존성 설치
- [ ] React 프로젝트 초기화 + 의존성 설치
- [ ] Makefile 작성
- [ ] docker-compose.yaml 작성
- [ ] .gitignore 작성 (Go, Node, IDE, OS)
- [ ] .editorconfig 작성
- [ ] 초기 커밋 → develop 브랜치 생성

---

## 3. 코드 품질 & 린트 설정

### 3.1 Go 린터 설정

`.golangci.yml`:
```yaml
run:
  timeout: 5m
  go: '1.24'

linters:
  enable:
    - errcheck        # 에러 체크 누락
    - gosimple        # 코드 단순화
    - govet           # Go 표준 검사
    - ineffassign     # 무효한 할당
    - staticcheck     # 정적 분석
    - unused          # 미사용 코드
    - gofmt           # 포맷팅
    - goimports       # import 정리
    - misspell        # 오타
    - bodyclose       # HTTP body close 누락
    - gocritic        # 코드 품질
    - revive          # golint 대체

linters-settings:
  revive:
    rules:
      - name: exported
        severity: warning

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

### 3.2 TypeScript/React 린터 설정

`web/.eslintrc.cjs`:
```javascript
module.exports = {
  root: true,
  env: { browser: true, es2020: true },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:react-hooks/recommended',
    'prettier'  // prettier와 충돌 방지 (항상 마지막)
  ],
  parser: '@typescript-eslint/parser',
  plugins: ['@typescript-eslint', 'react-refresh'],
  rules: {
    'react-refresh/only-export-components': 'warn',
    '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    '@typescript-eslint/no-explicit-any': 'warn',
    'no-console': ['warn', { allow: ['warn', 'error'] }]
  }
};
```

`web/.prettierrc`:
```json
{
  "semi": true,
  "trailingComma": "all",
  "singleQuote": true,
  "printWidth": 100,
  "tabWidth": 2
}
```

### 3.3 Commit Convention

**Conventional Commits** 형식을 사용합니다.

```
<type>(<scope>): <subject>

feat(F4): 설치 오케스트레이터 상태 머신 구현
fix(F1): kubeconfig 업로드 시 파일 크기 검증 누락
chore(ci): GitHub Actions 린트 워크플로우 추가
docs: Quick Start 가이드 초안 작성
refactor(engine): Step Runner 인터페이스 추출
test(F0): Organization 생성 API 단위 테스트
```

| Type | 용도 |
|---|---|
| `feat` | 새 기능 |
| `fix` | 버그 수정 |
| `chore` | 유지보수 (CI, 의존성, 설정) |
| `docs` | 문서 |
| `refactor` | 리팩토링 (기능 변화 없음) |
| `test` | 테스트 추가/수정 |
| `style` | 코드 스타일 (포맷팅) |

**Git Hooks (Husky + commitlint)** — 프론트엔드 쪽에서 관리:

```bash
cd web
npm install -D husky @commitlint/cli @commitlint/config-conventional
npx husky init
```

**담당**: 풀스택/QA  
**체크리스트**:
- [ ] `.golangci.yml` 설정
- [ ] `web/.eslintrc.cjs` + `web/.prettierrc` 설정
- [ ] Commit convention 문서화 (CONTRIBUTING.md)
- [ ] Git hooks 설정 (commitlint)
- [ ] 전 팀원 로컬 환경에서 lint 통과 확인

---

## 4. CI/CD 파이프라인 (GitHub Actions)

### 4.1 CI 워크플로우

`.github/workflows/ci.yaml`:

```yaml
name: CI

on:
  pull_request:
    branches: [develop, main]
  push:
    branches: [develop]

jobs:
  # ─── Go 백엔드 ───
  backend-lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  backend-test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:18-alpine
        env:
          POSTGRES_DB: nullus_test
          POSTGRES_USER: nullus
          POSTGRES_PASSWORD: test
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go test ./... -v -race -coverprofile=coverage.out
      - run: go tool cover -func=coverage.out

  # ─── React 프론트엔드 ───
  frontend-lint:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json
      - run: npm ci
      - run: npm run lint
      - run: npx tsc --noEmit

  frontend-test:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json
      - run: npm ci
      - run: npm run test -- --run

  # ─── Docker 빌드 검증 ───
  docker-build:
    runs-on: ubuntu-latest
    needs: [backend-test, frontend-test]
    steps:
      - uses: actions/checkout@v4
      - run: docker build -t nullus-api:ci .
      - run: docker build -t nullus-web:ci -f Dockerfile.web ./web
```

### 4.2 릴리스 워크플로우

`.github/workflows/release.yaml` — 태그 푸시 시 자동 빌드 + Docker Hub 푸시:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}
      - uses: docker/build-push-action@v5
        with:
          push: true
          tags: |
            cloudnullus/nullus-api:${{ github.ref_name }}
            cloudnullus/nullus-api:latest
      - uses: softprops/action-gh-release@v1
        with:
          generate_release_notes: true
```

### 4.3 필요한 GitHub Secrets

| Secret | 용도 | 설정 시점 |
|---|---|---|
| `DOCKERHUB_USERNAME` | Docker Hub 이미지 푸시 | Day 0 |
| `DOCKERHUB_TOKEN` | Docker Hub 인증 토큰 | Day 0 |
| `KUBECONFIG_DEV` | Dev 클러스터 접근 (E2E 테스트) | Week 2 |
| `KUBECONFIG_STAGING` | Staging 클러스터 접근 | Week 6 |

**담당**: DevOps  
**체크리스트**:
- [ ] `.github/workflows/ci.yaml` 작성
- [ ] `.github/workflows/release.yaml` 작성
- [ ] Docker Hub Organization `cloudnullus` 생성
- [ ] GitHub Secrets 등록 (DOCKERHUB_USERNAME, DOCKERHUB_TOKEN)
- [ ] CI 워크플로우 동작 확인 (첫 PR에서 검증)

---

## 5. 클라우드 인프라 & 환경 구성

### 5.1 환경 전략 개요

```
┌─────────────────────────────────────────────────────────────────────┐
│                          환경 구성 전략                               │
│                                                                      │
│  ┌─ Local ──────┐  ┌─ Dev ──────────┐  ┌─ Staging ────────────────┐ │
│  │ docker-compose│  │ GKE Autopilot  │  │ GKE Autopilot            │ │
│  │ + Kind (K8s)  │  │ (공유 클러스터)│  │ (프로덕션 미러)          │ │
│  │              │  │               │  │                          │ │
│  │ 용도:        │  │ 용도:         │  │ 용도:                    │ │
│  │ 개인 개발    │  │ 통합 테스트   │  │ 릴리스 후보 검증          │ │
│  │ 단위 테스트  │  │ E2E 테스트    │  │ Beta/GA 설치 테스트       │ │
│  │              │  │ PR 검증       │  │ 성능 테스트               │ │
│  │              │  │               │  │                          │ │
│  │ 비용: $0     │  │ 비용: ~$100/월│  │ 비용: ~$150/월           │ │
│  └──────────────┘  └───────────────┘  └──────────────────────────┘ │
│                                                                      │
│                         ┌─ Production ──────────────────────┐       │
│                         │ 사용자의 K8s 클러스터               │       │
│                         │ (Nullus가 설치되는 대상)            │       │
│                         │                                    │       │
│                         │ Nullus는 SaaS가 아니므로           │       │
│                         │ 자체 Production 환경 불필요         │       │
│                         │ → 테스트용 Production-like 환경만   │       │
│                         └────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────────┘
```

Nullus는 사용자 클러스터에 설치되는 오픈소스 프로젝트이므로 **자체 Production 환경은 불필요**합니다. 대신 Dev/Staging에서 "Nullus가 대상 클러스터에 도구를 설치하는 과정"을 테스트합니다.

### 5.2 Local 개발 환경

각 개발자의 로컬 머신에서 구동하는 환경입니다.

**필수 도구 설치 (macOS 기준)**:

```bash
# 패키지 매니저
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 런타임
brew install go@1.24          # Go 백엔드
brew install node@20          # React 프론트엔드
brew install postgresql@18    # 로컬 DB (docker-compose 대안)

# Kubernetes 도구
brew install kubectl           # K8s CLI
brew install helm              # Helm 3
brew install kind              # 로컬 K8s 클러스터

# 컨테이너
brew install --cask docker     # Docker Desktop (또는 colima)

# 개발 도구
brew install jq                # JSON 처리
brew install yq                # YAML 처리
brew install direnv            # 환경 변수 관리
brew install pre-commit        # Git hooks

# IDE 권장
brew install --cask visual-studio-code
# VS Code Extensions: Go, ESLint, Prettier, Kubernetes, Docker, YAML
```

**로컬 K8s 클러스터 (Kind)** — Nullus 설치 엔진 테스트용:

```yaml
# kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: nullus-dev
nodes:
  - role: control-plane
  - role: worker
    extraPortMappings:
      - containerPort: 30080    # GitLab HTTP
        hostPort: 30080
      - containerPort: 30443    # GitLab HTTPS
        hostPort: 30443
      - containerPort: 30090    # Prometheus
        hostPort: 30090
      - containerPort: 30300    # Grafana
        hostPort: 30300
  - role: worker
```

```bash
# Kind 클러스터 생성
kind create cluster --config kind-config.yaml

# 리소스 확인
kubectl get nodes
kubectl cluster-info
```

**환경 변수 (`.envrc` — direnv 사용)**:

```bash
# .envrc (gitignore에 포함)
export DATABASE_URL="postgres://nullus:nullus_dev@localhost:5432/nullus?sslmode=disable"
export KUBECONFIG_ENCRYPTION_KEY="dev-only-key-32-bytes-long!!!!!!"
export API_PORT="8080"
export WEB_PORT="3000"
export LOG_LEVEL="debug"
export ENVIRONMENT="local"
```

**담당**: 풀스택/QA (가이드 작성), 각 개발자 (자기 환경 셋업)  
**체크리스트**:
- [ ] `docs/dev-setup.md` 로컬 개발 환경 가이드 작성
- [ ] `kind-config.yaml` 작성
- [ ] `docker-compose.yaml` 동작 확인
- [ ] `.envrc.example` 작성 (.envrc는 gitignore)
- [ ] 전 팀원 로컬 환경 셋업 완료 확인

### 5.2.1 OIDC 사전 준비 (W9~W10 대비)

Keycloak 기반 OIDC 인증 구현을 위한 사전 준비 작업입니다. 실제 구현은 Week 9~10에서 진행하지만, Day 0에 기본 인프라와 검증 계획을 수립합니다.

**Keycloak 개발 인스턴스 설정**:

```bash
# docker-compose.yaml에 Keycloak 서비스 추가
# (기존 db, api, web 서비스 아래에 추가)

  keycloak:
    image: quay.io/keycloak/keycloak:latest
    environment:
      KEYCLOAK_ADMIN: admin
      KEYCLOAK_ADMIN_PASSWORD: admin
      KC_DB: postgres
      KC_DB_URL: jdbc:postgresql://db:5432/keycloak
      KC_DB_USERNAME: keycloak
      KC_DB_PASSWORD: keycloak_dev
      KC_HOSTNAME: localhost
      KC_HOSTNAME_PORT: 8180
      KC_HTTP_ENABLED: 'true'
      KC_PROXY: edge
    ports:
      - "8180:8080"
    depends_on:
      - db
    volumes:
      - keycloak_data:/opt/keycloak/data
```

**테스트 Realm 템플릿 생성**:

```bash
# templates/keycloak/nullus-dev-realm.json
# 다음 항목을 포함하는 Realm 정의:
# - Realm name: nullus-dev
# - Client: nullus-web (OIDC 클라이언트)
#   - Client ID: nullus-web
#   - Client Secret: (자동 생성)
#   - Redirect URIs: http://localhost:3000/auth/callback
#   - Web Origins: http://localhost:3000
# - Client Scopes: openid, profile, email, groups
# - Test Users: admin@nullus.dev, devops@nullus.dev, developer@nullus.dev
# - User Groups: admin, devops, developer (각 역할 매핑)
```

**담당**: BE-1 (OIDC 플로우 설계), DevOps (Keycloak 인프라)  
**체크리스트**:
- [ ] Keycloak 개발 인스턴스 Docker Compose에 추가
- [ ] 테스트 realm 템플릿 작성 (`templates/keycloak/nullus-dev-realm.json`)
- [ ] 인증서 체인 테스트 시나리오 문서화 (self-signed, Let's Encrypt)
  - [ ] Self-signed 인증서 생성 및 검증 방법
  - [ ] Let's Encrypt 통합 시나리오 (Staging/Production)
  - [ ] 인증서 갱신 자동화 계획
- [ ] groups/roles scope 검증 계획 수립
  - [ ] OIDC 토큰에 groups claim 포함 확인
  - [ ] 사용자 역할(admin/devops/developer)과 K8s RBAC 매핑 전략
- [ ] OIDC 기본 플로우 PoC 작성 (Authorization Code Flow)
  - [ ] 로그인 엔드포인트 (`/auth/login`)
  - [ ] 콜백 엔드포인트 (`/auth/callback`)
  - [ ] 토큰 검증 및 사용자 정보 추출
  - [ ] 세션 관리 (JWT 또는 세션 쿠키)

### 5.3 Dev 환경 (GKE Autopilot)

통합 테스트와 PR 검증에 사용하는 공유 클러스터입니다.

**GKE Autopilot 클러스터 생성**:

```bash
# GCP 프로젝트 설정
gcloud config set project nullus-dev

# Autopilot 클러스터 생성
gcloud container clusters create-auto nullus-dev \
  --region=asia-northeast3 \
  --release-channel=regular \
  --network=default \
  --subnetwork=default

# kubeconfig 가져오기
gcloud container clusters get-credentials nullus-dev \
  --region=asia-northeast3

# 네임스페이스 생성
kubectl create namespace nullus-control    # Nullus 컨트롤 플레인
kubectl create namespace nullus-test       # 설치 엔진 테스트 대상
```

**Dev 클러스터 용도**:
- Nullus API Server + Web UI 배포 (CI에서 자동 배포)
- 설치 엔진이 `nullus-test` 네임스페이스에 도구를 설치하는 E2E 테스트
- PR 머지 시 자동으로 develop 브랜치 배포

**비용 최적화**:
- GKE Autopilot은 사용한 Pod 리소스만 과금
- 업무 시간 외 스케일 다운: 테스트 네임스페이스의 워크로드를 0으로
- 월 예상 비용: ~$80~120 (테스트 워크로드 규모에 따라)

**접근 제어**:

```bash
# 팀원에게 GKE 접근 권한 부여
gcloud projects add-iam-policy-binding nullus-dev \
  --member="user:engineer@example.com" \
  --role="roles/container.developer"

# CI 서비스 어카운트 생성
gcloud iam service-accounts create nullus-ci \
  --display-name="Nullus CI"

gcloud projects add-iam-policy-binding nullus-dev \
  --member="serviceAccount:nullus-ci@nullus-dev.iam.gserviceaccount.com" \
  --role="roles/container.developer"
```

### 5.4 Staging 환경 (GKE Autopilot)

Beta 릴리스 후보 검증과 설치 성공률 측정에 사용합니다.

```bash
# Staging 클러스터 생성
gcloud container clusters create-auto nullus-staging \
  --region=asia-northeast3 \
  --release-channel=stable \
  --network=default \
  --subnetwork=default

# 네임스페이스 생성
kubectl create namespace nullus-control
kubectl create namespace nullus-target    # 실제 설치 테스트 (프로덕션 미러)
```

**Staging 특징**:
- `stable` 릴리스 채널 사용 (Dev는 `regular`)
- 프로덕션과 유사한 리소스 제한 적용
- Beta/GA 릴리스 전 반드시 Staging에서 전체 설치 시나리오 검증
- 설치 성공률 측정 자동화

**Day 0에는 Staging 클러스터를 생성만 해두고, 실제 활용은 Week 6(Beta 준비)부터 시작합니다.**

### 5.5 추가 테스트 환경 (선택)

설치 성공률 측정을 위해 GKE 외 환경에서도 테스트합니다. 이 환경들은 Day 0에 구축하지 않고 Week 7~8에 필요 시 생성합니다.

| 환경 | 용도 | 생성 시점 |
|---|---|---|
| EKS (AWS) | 크로스 플랫폼 호환성 검증 | Week 7 |
| Kind (CI) | GitHub Actions에서 E2E 테스트 | Week 4 |

### 5.6 GCP 프로젝트 구조

```
nullus-dev (GCP 프로젝트)
├── GKE
│   ├── nullus-dev (Autopilot, asia-northeast3)
│   └── nullus-staging (Autopilot, asia-northeast3)
│
├── Artifact Registry
│   └── nullus-images (Docker 이미지, 내부용)
│
├── IAM
│   ├── nullus-ci@nullus-dev.iam.gserviceaccount.com (CI 전용)
│   └── 팀원 개별 계정 (container.developer 역할)
│
├── Cloud DNS (선택)
│   └── dev.nullus.io, staging.nullus.io
│
└── Budget Alert
    └── 월 $300 초과 시 알림
```

**담당**: DevOps  
**체크리스트**:
- [ ] GCP 프로젝트 `nullus-dev` 생성 (또는 기존 프로젝트 사용)
- [ ] 결제 계정 연결 + Budget Alert ($300/월) 설정
- [ ] GKE Autopilot `nullus-dev` 클러스터 생성
- [ ] GKE Autopilot `nullus-staging` 클러스터 생성
- [ ] CI 서비스 어카운트 생성 + 키 발급
- [ ] Artifact Registry 리포지토리 생성
- [ ] 팀원 IAM 권한 부여
- [ ] kubeconfig를 GitHub Secrets에 등록

---

## 6. 데이터베이스 초기화

### 6.1 초기 마이그레이션

> **DB 스키마 정본**: `docs/20_아키텍처/Nullus_DB_스키마.md` 참조
> Day 0에서는 마이그레이션 도구로 스키마를 적용합니다:
> ```bash
> migrate -path db/migrations -database "$DATABASE_URL" up
> ```

**담당**: BE-2  
**체크리스트**:
- [ ] 마이그레이션 파일 작성
- [ ] 로컬 PostgreSQL에서 마이그레이션 실행 확인
- [ ] `db/seed/` 초기 데이터 스크립트 작성 (테스트용 Admin 계정, Golden Path 메타데이터)
- [ ] 마이그레이션 롤백 동작 확인

---

## 7. 핵심 문서 작성

### 7.1 Day 0에 준비할 문서 목록

| 문서 | 위치 | 담당 | 내용 |
|---|---|---|---|
| **README.md** | `/README.md` | 풀스택/QA | 프로젝트 소개, 빌드/실행 방법, 기여 가이드 링크 |
| **CONTRIBUTING.md** | `/CONTRIBUTING.md` | 풀스택/QA | 개발 환경 셋업, 브랜치 전략, 커밋 컨벤션, PR 규칙, 코드 리뷰 가이드 |
| **개발 환경 가이드** | `/docs/dev-setup.md` | 풀스택/QA | 로컬 환경 셋업 단계별 가이드 (macOS/Linux) |
| **아키텍처 문서** | `/docs/architecture.md` | BE-1 | 시스템 아키텍처 개요 (기존 작성 문서 기반) |
| **API 설계 초안** | `/api/openapi.yaml` | BE-1 | OpenAPI 3.0 스펙 초안 (엔드포인트 목록 + 모델) |
| **DB 스키마 문서** | `/docs/database.md` | BE-2 | ERD, 테이블 설명, JSONB 구조 |
| **ADR 디렉토리** | `/docs/adr/` | 전체 | 기술 의사결정 기록 (ADR-001 ~ ADR-010) |

> **참고**: PRD v1.2에서 Narwhal 레퍼런스 분석이 추가되었습니다. `known-issues.yaml` 시드 데이터와 호환성 매트릭스의 chart/app 버전 분리 구조는 Week 2~3에서 DevOps가 작성합니다. 상세 내용은 `기획단계/아키텍처/개발계획/Narwhal 분석 기반 Nullus 적용 항목.md`를 참고하세요.

### 7.2 README.md 초안 구조

```markdown
# Nullus

> 검증된 Golden Path로 1시간 안에 프로덕션 레디 DevOps 파이프라인을 구축하세요.

## 빠른 시작
## 주요 기능
## 아키텍처
## 개발 환경 셋업
## 기여 방법
## 라이선스
```

**담당**: 각 문서별 담당자 (위 표 참조)  
**체크리스트**:
- [ ] README.md 초안 작성
- [ ] CONTRIBUTING.md 작성
- [ ] docs/dev-setup.md 작성
- [ ] docs/adr/ 디렉토리에 ADR 10건 기록
- [ ] api/openapi.yaml 초안 (엔드포인트 목록만)

---

## 8. 커뮤니케이션 & 프로젝트 관리 도구

### 8.1 필요 도구 목록

| 도구 | 용도 | 비용 |
|---|---|---|
| **Discord** | 팀 내부 + 커뮤니티 소통 | 무료 |
| **GitHub Projects** | 스프린트 관리, 이슈 트래킹 | GitHub Team 포함 |
| **GitHub Wiki** 또는 **Notion** | 내부 문서 (회의록, 기술 메모) | 무료/팀 플랜 |
| **Figma** | UI 디자인, 프로토타입 공유 | 무료 (3 프로젝트) |
| **Google Meet** / **Discord 음성** | 스탠드업, 싱크 미팅 | 무료 |

### 8.2 Discord 채널 구조

```
cloud-nullus Discord
├── 📢 공지
│   └── #announcements          릴리스 공지, 중요 변경
│
├── 💻 개발
│   ├── #general                일반 개발 논의
│   ├── #frontend               React, UI 관련
│   ├── #backend                Go, API 관련
│   ├── #engine                 설치 엔진, Helm
│   ├── #ci-cd                  GitHub Actions, 빌드
│   └── #code-review            PR 알림 (GitHub Webhook)
│
├── 🔧 운영
│   ├── #infra                  GKE, 클라우드 인프라
│   ├── #alerts                 모니터링 알림 (선택)
│   └── #daily-standup          비동기 스탠드업 (텍스트)
│
├── 📖 기획
│   ├── #prd-discussion         PRD 논의
│   ├── #design                 UI/UX 디자인 공유
│   └── #architecture           아키텍처 논의
│
└── 🌍 커뮤니티 (Public 전환 후)
    ├── #welcome
    ├── #beta-feedback
    ├── #help
    └── 🔊 office-hour          주간 음성 채널
```

### 8.3 GitHub ↔ Discord 연동

```
GitHub Webhook → Discord #code-review
  - PR 생성 알림
  - PR 리뷰 요청 알림
  - CI 실패 알림

GitHub Webhook → Discord #alerts
  - Release 생성 알림
  - Issue 생성 알림 (P0 라벨)
```

**담당**: DevOps (Discord 생성), 풀스택/QA (Webhook 설정)  
**체크리스트**:
- [ ] Discord 서버 생성 + 채널 구조 셋업
- [ ] GitHub → Discord Webhook 연동 (#code-review)
- [ ] 팀원 전원 Discord 초대
- [ ] 비동기 스탠드업 템플릿 공유 (#daily-standup)

---

## 9. 보안 기본 설정

### 9.1 시크릿 관리

| 시크릿 | 저장 위치 | 용도 |
|---|---|---|
| DB 비밀번호 (dev) | `.envrc` (gitignore) | 로컬 개발 |
| DB 비밀번호 (dev/staging) | K8s Secret (GKE) | 클라우드 환경 |
| Kubeconfig 암호화 키 | `.envrc` / K8s Secret | kubeconfig AES-256-GCM 암호화 |
| Docker Hub 토큰 | GitHub Secrets | CI/CD 이미지 푸시 |
| GCP 서비스 어카운트 키 | GitHub Secrets | CI/CD에서 GKE 접근 |

**절대 커밋하면 안 되는 것들**:
- `.envrc` (환경 변수)
- `*.pem`, `*.key` (인증서, 키)
- `kubeconfig` 파일
- `.env` 파일

**`.gitignore`에 반드시 포함**:

```
# 환경 변수
.envrc
.env
.env.*

# 시크릿
*.pem
*.key
kubeconfig
kubeconfig.*

# IDE
.idea/
.vscode/settings.json

# OS
.DS_Store
Thumbs.db

# 빌드 산출물
/bin/
/dist/
/web/dist/
/web/node_modules/

# 테스트
coverage.out
coverage/
```

### 9.2 Git Secrets (사전 차단)

실수로 시크릿이 커밋되는 것을 방지합니다.

```bash
# git-secrets 설치
brew install git-secrets

# 리포지토리에 설정
cd nullus
git secrets --install
git secrets --register-aws    # AWS 키 패턴
git secrets --add 'PRIVATE KEY'
git secrets --add 'password\s*=\s*.+'
```

**담당**: DevOps  
**체크리스트**:
- [ ] `.gitignore` 작성
- [ ] `.envrc.example` 작성 (실제 값 없는 템플릿)
- [ ] git-secrets 설치 가이드를 dev-setup.md에 추가
- [ ] GitHub Secrets에 필요한 시크릿 등록

---

## 10. Day 0 종합 체크리스트

### 역할별 담당 요약

#### DevOps 담당

- [ ] GitHub Organization 보안 설정 (2FA, Secret scanning)
- [ ] GitHub Teams 생성 및 멤버 할당
- [ ] CI 워크플로우 (`ci.yaml`) 작성
- [ ] 릴리스 워크플로우 (`release.yaml`) 작성
- [ ] Docker Hub Organization `cloudnullus` 생성
- [ ] GitHub Secrets 등록 (DOCKERHUB_USERNAME, DOCKERHUB_TOKEN)
- [ ] GCP 프로젝트 생성 + 결제 + Budget Alert
- [ ] GKE Autopilot Dev 클러스터 생성
- [ ] GKE Autopilot Staging 클러스터 생성
- [ ] CI 서비스 어카운트 생성 + kubeconfig를 GitHub Secrets에 등록
- [ ] Artifact Registry 생성
- [ ] 팀원 GCP IAM 권한 부여
- [ ] Discord 서버 생성 + 채널 구조 셋업
- [ ] GitHub → Discord Webhook 연동
- [ ] `.gitignore` 작성
- [ ] git-secrets 가이드 작성
- [ ] `templates/known-issues/known-issues.yaml` 초기 시드 데이터 작성 (Narwhal 70+ 패턴)

#### BE-1 (백엔드 리드) 담당

- [ ] Go 모듈 초기화 (`go mod init`)
- [ ] Go 핵심 의존성 설치
- [ ] `.golangci.yml` 작성
- [ ] `api/openapi.yaml` 초안 (엔드포인트 목록)
- [ ] 아키텍처 문서 (`docs/architecture.md`)
- [ ] ADR 문서 작성 (ADR-001 ~ ADR-010)

#### BE-2 (백엔드) 담당

- [ ] DB 마이그레이션 파일 작성 (`001_init.up.sql`, `001_init.down.sql`)
  - [ ] `pipeline_deployments` 테이블에 `change_reason TEXT` 컬럼 추가 (배포 변경 사유 기록)
- [ ] 로컬 PostgreSQL 마이그레이션 실행 확인
- [ ] 시드 데이터 스크립트 작성 (`db/seed/`)
- [ ] DB 스키마 문서 (`docs/database.md`)

#### FE-1 (프론트엔드 리드) 담당

- [ ] React + Vite + TypeScript 프로젝트 초기화
- [ ] Tailwind CSS 설정
- [ ] ESLint + Prettier 설정
- [ ] Zustand 스토어 뼈대
- [ ] API 클라이언트 (axios) 뼈대
- [ ] 라우팅 구조 설정 (react-router-dom)

#### FE-2 (프론트엔드) 담당

- [ ] UI 레이아웃 셸 (사이드바, 헤더, 다크 테마)
- [ ] 디자인 토큰 정의 (색상, 타이포그래피, 간격)
- [ ] proto2 프로토타입 → React 컴포넌트 매핑 목록 작성

#### 풀스택/QA 담당

- [ ] 리포지토리 생성 (nullus, nullus-helm-charts, nullus-docs, .github)
- [ ] Branch protection rules 설정
- [ ] CODEOWNERS 작성
- [ ] GitHub Labels 생성
- [ ] Issue/PR 템플릿 생성
- [ ] GitHub Project 보드 생성 + Custom Fields
- [ ] 초기 Epic 이슈 생성 (기능 0~10)
- [ ] Sprint 0 작업 이슈 생성 및 할당
- [ ] README.md 초안
- [ ] CONTRIBUTING.md 작성
- [ ] `docs/dev-setup.md` 로컬 개발 가이드
- [ ] `Makefile` 작성
- [ ] `docker-compose.yaml` 작성
- [ ] Commit convention (commitlint + Husky) 설정
- [ ] `.editorconfig` 작성

### 완료 확인 기준

Day 0 작업이 모두 완료되었음을 확인하는 최종 검증 항목입니다.

```
[ ] 모든 팀원이 로컬에서 `docker-compose up` 으로 API + Web + DB 실행 가능
[ ] 모든 팀원이 로컬에서 Kind 클러스터 생성 + kubectl 접근 가능
[ ] 모든 팀원이 GitHub에 PR 생성 → CI 통과 → 리뷰 → 머지 가능
[ ] develop 브랜치에 첫 커밋 존재 (프로젝트 뼈대)
[ ] GKE Dev 클러스터에 kubectl 접근 가능 (전 팀원)
[ ] Discord에서 GitHub PR 알림 수신 확인
[ ] DB 마이그레이션이 로컬 PostgreSQL에서 정상 실행
[ ] Sprint 0 이슈가 GitHub Project 보드에 할당 완료
```

---

## 11. 예상 비용 요약

| 항목 | 월 비용 | 비고 |
|---|---|---|
| **GitHub Team** | ~$4/인 × 6 = $24 | Private repo, Actions 3,000분 |
| **GKE Dev** | ~$80~120 | Autopilot, 테스트 워크로드 규모에 따라 |
| **GKE Staging** | ~$100~150 | Week 6부터 본격 사용 |
| **Docker Hub** | $0 (무료 플랜) | Public 이미지만 |
| **Discord** | $0 | 무료 |
| **도메인 (선택)** | ~$12/년 | nullus.io 등 |
| **합계** | **~$200~300/월** | 초기 3개월 기준 |