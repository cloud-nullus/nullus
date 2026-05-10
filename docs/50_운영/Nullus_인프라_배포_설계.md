# Nullus Platform 인프라 및 배포 설계

**프로젝트**: Nullus - Kubernetes DevSecOps 플랫폼 빌더
**작성일**: 2026-03-14
**버전**: 1.0
**기반 문서**: PRD v1.3, 상세 기능 명세 및 시스템 아키텍처 v1.0, 12주 개발 마스터 플랜, 로컬 개발환경 세팅 가이드, Day 0 착수 체크리스트
**대상 독자**: DevOps Engineer, 백엔드/풀스택 엔지니어

---

## 목차

1. [개요](#1-개요)
2. [로컬 개발 환경 Quick Start](#2-로컬-개발-환경-quick-start)
3. [개발 환경 구성 상세](#3-개발-환경-구성-상세)
4. [Docker 이미지 빌드 전략](#4-docker-이미지-빌드-전략)
5. [CI/CD 파이프라인 설계](#5-cicd-파이프라인-설계)
6. [릴리스 전략](#6-릴리스-전략)
7. [Helm Chart 구조](#7-helm-chart-구조)
8. [환경별 설정 관리](#8-환경별-설정-관리)
9. [모니터링/로깅 인프라](#9-모니터링로깅-인프라)
10. [보안 설정](#10-보안-설정)
11. [백업/복구 전략](#11-백업복구-전략)
12. [운영 런북](#12-운영-런북)

---

## 1. 개요

### 1.1 Nullus 배포 모델

Nullus는 사용자의 Kubernetes 클러스터에 설치되는 오픈소스 플랫폼 빌더입니다. 자체 SaaS 운영 환경이 없으며, 아래 두 가지 배포 모드를 지원합니다.

| 배포 모드 | 대상 | 릴리스 단계 |
|-----------|------|-------------|
| **Docker Compose** | 단일 노드, 개발/소규모 테스트 | Alpha, Beta |
| **Helm Chart** | 프로덕션 K8s 클러스터 | v1.0 GA 이상 |

### 1.2 전체 환경 구성 개요

```
┌─ Local ──────────────────┐  ┌─ Dev (GKE) ──────────────┐  ┌─ Staging (GKE) ──────────┐
│  docker-compose          │  │  GKE Autopilot           │  │  GKE Autopilot           │
│  + Kind K8s              │  │  통합 테스트 / E2E        │  │  릴리스 후보 검증         │
│  비용: $0                │  │  비용: ~$100/월           │  │  비용: ~$150/월           │
└──────────────────────────┘  └──────────────────────────┘  └──────────────────────────┘

사용자의 K8s 클러스터 (Nullus가 DevSecOps Stack을 설치하는 대상)
- Kind (로컬 테스트)
- GKE, EKS, 온프레미스 K8s 1.26+
```

### 1.3 최소 요구사항

| 항목 | 최소 | 권장 |
|------|------|------|
| Kubernetes | 1.26+ | 1.28+ |
| PostgreSQL | 18+ | 18+ |
| CPU | 2 vCPU | 4 vCPU |
| Memory | 4 GB | 8 GB |
| Storage | 20 GB | 50 GB |

---

## 2. 로컬 개발 환경 Quick Start

### 2.1 사전 준비 (macOS)

```bash
# 1. OrbStack 설치 (Docker Desktop 대체, 성능 우수)
#    https://orbstack.dev 에서 다운로드

# 2. 개발 도구 설치
brew install go@1.24 node@20
brew install kubectl helm kind
brew install golangci-lint air jq yq

# 환경 변수 반영
echo 'export PATH="/opt/homebrew/opt/go@1.24/bin:$PATH"' >> ~/.zshrc
echo 'export PATH=$(go env GOPATH)/bin:$PATH' >> ~/.zshrc
source ~/.zshrc
```

### 2.2 저장소 클론 및 초기 설정

```bash
git clone git@github.com:cloud-nullus/nullus.git
cd nullus

# Go 모듈 의존성 설치
go mod download

# 프론트엔드 의존성 설치
cd web && npm ci && cd ..

# 환경 변수 설정
cp .env.example .env
# .env 파일에서 필요한 값 확인/수정
```

### 2.3 인프라 기동 및 개발 서버 실행

```bash
# 1. 로컬 인프라 기동 (PostgreSQL + Keycloak + MinIO)
make infra-up
# 또는
docker compose up -d postgres keycloak minio

# 2. DB 마이그레이션 적용
make migrate-up

# 3. 시드 데이터 삽입 (선택)
make seed

# 4. 개발 서버 실행 (백엔드 + 프론트엔드 동시 실행)
make dev
```

브라우저에서 `http://localhost:3000` 접속하여 Nullus Web UI 확인

### 2.4 Kind K8s 클러스터 생성 (설치 엔진 E2E 테스트용)

```bash
# Kind 클러스터 생성
make kind-up
# 또는
kind create cluster --name nullus-dev --config kind-config.yaml

# 클러스터 확인
kubectl config use-context kind-nullus-dev
kubectl get nodes

# Kind 클러스터 삭제
make kind-down
```

### 2.5 전체 Makefile 명령어

| 명령어 | 설명 |
|--------|------|
| `make dev` | 백엔드(Air HMR) + 프론트엔드(Vite HMR) 동시 실행 |
| `make infra-up` | PostgreSQL + Keycloak + MinIO 기동 |
| `make infra-down` | 로컬 인프라 중지 |
| `make build` | 전체 프로덕션 빌드 (Docker 이미지) |
| `make docker-build` | API + Web Docker 이미지 빌드 |
| `make test` | 전체 테스트 실행 |
| `make test-backend` | `go test ./internal/...` 실행 |
| `make test-frontend` | `npm test` 실행 |
| `make test-e2e` | Kind 클러스터 E2E 테스트 |
| `make lint` | golangci-lint + eslint 실행 |
| `make fmt` | 코드 자동 포맷 |
| `make migrate-up` | DB 마이그레이션 적용 |
| `make migrate-down` | 마지막 마이그레이션 롤백 |
| `make migrate-create NAME=xxx` | 새 마이그레이션 파일 생성 |
| `make seed` | 시드 데이터 삽입 |
| `make swagger` | OpenAPI 3.0 문서 자동 생성 |
| `make kind-up` | Kind K8s 클러스터 생성 |
| `make kind-down` | Kind K8s 클러스터 삭제 |
| `make helm-lint` | Helm 차트 문법 검증 |
| `make help` | 모든 명령어 목록 출력 |

---

## 3. 개발 환경 구성 상세

### 3.1 Docker Compose 구성

로컬 개발에 필요한 모든 인프라를 Docker Compose로 제공합니다.

```yaml
# docker-compose.yaml
version: '3.8'

services:
  # ── PostgreSQL ─────────────────────────────────────────────
  postgres:
    image: postgres:18-alpine
    container_name: nullus-postgres
    environment:
      POSTGRES_DB: nullus
      POSTGRES_USER: nullus
      POSTGRES_PASSWORD: nullus_dev
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./db/init:/docker-entrypoint-initdb.d   # 초기 DB 생성 스크립트
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U nullus"]
      interval: 10s
      timeout: 5s
      retries: 5

  # ── Keycloak (v1.0 OIDC 인증 사전 준비) ────────────────────
  keycloak:
    image: quay.io/keycloak/keycloak:26.x
    container_name: nullus-keycloak
    command: start-dev --import-realm
    environment:
      KEYCLOAK_ADMIN: admin
      KEYCLOAK_ADMIN_PASSWORD: admin
      KC_DB: postgres
      KC_DB_URL: jdbc:postgresql://postgres:5432/keycloak
      KC_DB_USERNAME: keycloak
      KC_DB_PASSWORD: keycloak_dev
      KC_HOSTNAME: localhost
      KC_HOSTNAME_PORT: 8180
      KC_HTTP_ENABLED: "true"
    ports:
      - "8180:8080"
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - ./templates/keycloak:/opt/keycloak/data/import   # Realm 자동 임포트
      - keycloak_data:/opt/keycloak/data

  # ── MinIO (오브젝트 스토리지) ───────────────────────────────
  minio:
    image: quay.io/minio/minio:latest
    container_name: nullus-minio
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin_dev
    ports:
      - "9000:9000"   # S3 API
      - "9001:9001"   # Web Console
    volumes:
      - minio_data:/data
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 10s
      timeout: 5s
      retries: 5

  # ── Nullus API Server (개발 모드: Air HMR) ──────────────────
  api:
    build:
      context: .
      dockerfile: Dockerfile.dev
    container_name: nullus-api
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://nullus:nullus_dev@postgres:5432/nullus?sslmode=disable
      KUBECONFIG_ENCRYPTION_KEY: dev-only-32-byte-key-change-me!
      API_PORT: "8080"
      LOG_LEVEL: debug
      ENVIRONMENT: local
      KEYCLOAK_URL: http://keycloak:8080
      KEYCLOAK_REALM: nullus-dev
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - .:/app
      - /app/bin   # Air 빌드 산출물

  # ── Nullus Web UI (개발 모드: Vite HMR) ────────────────────
  web:
    build:
      context: ./web
      dockerfile: Dockerfile.dev
    container_name: nullus-web
    ports:
      - "3000:3000"
    environment:
      VITE_API_URL: http://localhost:8090
      VITE_WS_URL: ws://localhost:8090
    volumes:
      - ./web:/app
      - /app/node_modules   # node_modules 마운트 제외 (성능)

volumes:
  pgdata:
  keycloak_data:
  minio_data:
```

### 3.2 Kind K8s 클러스터 설정

```yaml
# kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: nullus-dev
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 8888    # Ingress HTTP
        protocol: TCP
      - containerPort: 443
        hostPort: 8443    # Ingress HTTPS
        protocol: TCP
  - role: worker
    extraPortMappings:
      - containerPort: 30080   # GitLab HTTP
        hostPort: 30080
      - containerPort: 30443   # GitLab HTTPS
        hostPort: 30443
      - containerPort: 30090   # Prometheus
        hostPort: 30090
      - containerPort: 30300   # Grafana
        hostPort: 30300
      - containerPort: 30900   # MinIO API
        hostPort: 30900
  - role: worker
```

### 3.3 포트 매핑 요약

| 서비스 | 포트 | 용도 |
|--------|------|------|
| Nullus Web UI (Vite) | 3000 | 프론트엔드 개발 서버 |
| Nullus API (Go) | 8080 | REST API + WebSocket |
| PostgreSQL | 5432 | 데이터베이스 |
| Keycloak | 8180 | OIDC 인증 서버 |
| MinIO API | 9000 | S3 호환 오브젝트 스토리지 |
| MinIO Console | 9001 | MinIO 웹 관리 콘솔 |
| Kind K8s API | 6443 | 로컬 K8s 클러스터 API |
| Kind Ingress HTTP | 8888 | Kind 내 서비스 HTTP 접근 |

### 3.4 환경 변수 (.env.example)

```bash
# ── 데이터베이스 ────────────────────────────────────────────
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=nullus
POSTGRES_USER=nullus
POSTGRES_PASSWORD=nullus_dev
DATABASE_URL=postgres://nullus:nullus_dev@localhost:5432/nullus?sslmode=disable

# ── API 서버 ─────────────────────────────────────────────────
API_PORT=8080
LOG_LEVEL=debug          # debug | info | warn | error
ENVIRONMENT=local        # local | dev | staging | production

# ── 보안 ─────────────────────────────────────────────────────
# 32바이트 (256-bit) AES 키: kubeconfig 암호화에 사용
# 운영 환경에서는 반드시 안전한 랜덤 값으로 교체
KUBECONFIG_ENCRYPTION_KEY=dev-only-32-byte-key-change-me!

# ── 인증 (Alpha/Beta: 세션, v1: Keycloak) ─────────────────────
SESSION_SECRET=dev-session-secret-change-me
KEYCLOAK_URL=http://localhost:8180
KEYCLOAK_REALM=nullus-dev
KEYCLOAK_CLIENT_ID=nullus-web
KEYCLOAK_CLIENT_SECRET=

# ── 프론트엔드 ────────────────────────────────────────────────
VITE_API_URL=http://localhost:8090
VITE_WS_URL=ws://localhost:8090

# ── MinIO ────────────────────────────────────────────────────
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin_dev

# ── Kubernetes ────────────────────────────────────────────────
KUBECONFIG=~/.kube/config
```

---

## 4. Docker 이미지 빌드 전략

### 4.1 멀티스테이지 빌드 — API Server (Go)

```dockerfile
# Dockerfile (API Server)

# ── Stage 1: 빌드 환경 ──────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /build

# 의존성 캐시 레이어 (소스 변경 시 재사용)
COPY go.mod go.sum ./
RUN go mod download

# 소스 빌드
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s -X main.version=${VERSION}" \
    -o /app/nullus-server ./cmd/api

# ── Stage 2: 프로덕션 이미지 ────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

# 바이너리만 복사 (OS 없음 → 공격 면적 최소화)
COPY --from=builder /app/nullus-server /nullus-server

# DB 마이그레이션 파일 포함
COPY --from=builder /build/db/migrations /migrations

# 비root 사용자 실행 (distroless nonroot 기본값: uid=65532)
USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/nullus-server"]
```

### 4.2 멀티스테이지 빌드 — Web UI (React)

```dockerfile
# Dockerfile.web

# ── Stage 1: Node.js 빌드 ───────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

# 의존성 캐시
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts

# 소스 빌드
COPY web/ .
ARG VITE_API_URL
ARG VITE_WS_URL
RUN npm run build

# ── Stage 2: Nginx 서빙 ─────────────────────────────────────
FROM nginx:1.27-alpine

# 빌드 산출물 복사
COPY --from=builder /app/dist /usr/share/nginx/html

# SPA 라우팅 지원 + 보안 헤더 적용
COPY deployments/nginx/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

```nginx
# deployments/nginx/nginx.conf
server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    # SPA 라우팅: 모든 경로를 index.html로 폴백
    location / {
        try_files $uri $uri/ /index.html;
    }

    # 보안 헤더
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline';" always;

    # 정적 자산 캐시
    location ~* \.(js|css|png|jpg|svg|ico|woff2)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # API 프록시 (프론트엔드와 백엔드 동일 도메인 서빙 시)
    location /api/ {
        proxy_pass http://nullus-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # WebSocket
    location /ws/ {
        proxy_pass http://nullus-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_read_timeout 3600s;
    }
}
```

### 4.3 개발용 Dockerfile

```dockerfile
# Dockerfile.dev (API Server — Air HMR)
FROM golang:1.24-alpine

RUN apk add --no-cache git make
RUN go install github.com/air-verse/air@latest

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

CMD ["air", "-c", ".air.toml"]
```

```dockerfile
# web/Dockerfile.dev (Web — Vite HMR)
FROM node:20-alpine

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
```

### 4.4 이미지 태깅 전략

| 태그 | 용도 | 예시 |
|------|------|------|
| `vX.Y.Z` | 정식 릴리스 | `cloudnullus/nullus-api:v0.1.0` |
| `vX.Y.Z-alpha` | Alpha 릴리스 | `cloudnullus/nullus-api:v0.1.0-alpha` |
| `vX.Y.Z-beta` | Beta 릴리스 | `cloudnullus/nullus-api:v0.1.0-beta` |
| `latest` | GA 최신 안정 버전 | `cloudnullus/nullus-api:latest` |
| `sha-{git_sha7}` | 커밋별 추적 | `cloudnullus/nullus-api:sha-a1b2c3d` |
| `develop` | develop 브랜치 최신 | `cloudnullus/nullus-api:develop` |

---

## 5. CI/CD 파이프라인 설계

### 5.1 파이프라인 전체 흐름

```
PR 생성 / develop 푸시
        │
        ▼
  ┌─────────────────────────────────────┐
  │  CI (ci.yaml)                       │
  │                                     │
  │  lint ──→ test ──→ docker-build     │
  │   │         │           │           │
  │  Go lint  Go test    빌드 검증      │
  │  TS lint  FE test    (push 없음)    │
  └─────────────────────────────────────┘
        │ (develop 머지 시)
        ▼
  ┌─────────────────────────────────────┐
  │  CD to Dev (cd-dev.yaml)            │
  │                                     │
  │  build → push (sha 태그) → deploy   │
  │         → GKE Dev 클러스터 적용     │
  └─────────────────────────────────────┘
        │ (태그 푸시 v*.* 시)
        ▼
  ┌─────────────────────────────────────┐
  │  Release (release.yaml)             │
  │                                     │
  │  build → trivy scan → push          │
  │  → GitHub Release 생성              │
  │  → Docker Hub 최신 태그 갱신        │
  │  → Helm 차트 업데이트               │
  └─────────────────────────────────────┘
```

### 5.2 CI 워크플로우 (.github/workflows/ci.yaml)

```yaml
name: CI

on:
  pull_request:
    branches: [develop, main]
  push:
    branches: [develop]

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

jobs:
  # ── Go 백엔드 린트 ──────────────────────────────────────────
  backend-lint:
    name: Backend Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache: true
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest
          args: --timeout=5m

  # ── Go 백엔드 테스트 ─────────────────────────────────────────
  backend-test:
    name: Backend Test
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
          cache: true
      - name: Run migrations
        run: |
          go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
          migrate -path db/migrations -database "postgres://nullus:test@localhost:5432/nullus_test?sslmode=disable" up
      - name: Run tests
        run: go test ./... -v -race -coverprofile=coverage.out
        env:
          DATABASE_URL: postgres://nullus:test@localhost:5432/nullus_test?sslmode=disable
      - name: Coverage report
        run: go tool cover -func=coverage.out
      - name: Coverage gate (>30% Alpha, >50% Beta, >70% GA)
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
          echo "Coverage: ${COVERAGE}%"

  # ── React 프론트엔드 린트 ────────────────────────────────────
  frontend-lint:
    name: Frontend Lint
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: npm
          cache-dependency-path: web/package-lock.json
      - run: npm ci
      - run: npm run lint
      - run: npx tsc --noEmit

  # ── React 프론트엔드 테스트 ──────────────────────────────────
  frontend-test:
    name: Frontend Test
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: npm
          cache-dependency-path: web/package-lock.json
      - run: npm ci
      - run: npm run test -- --run

  # ── Docker 빌드 검증 (push 없음) ─────────────────────────────
  docker-build:
    name: Docker Build Check
    runs-on: ubuntu-latest
    needs: [backend-test, frontend-test]
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - name: Build API image
        uses: docker/build-push-action@v6
        with:
          context: .
          file: Dockerfile
          push: false
          tags: nullus-api:ci
          cache-from: type=gha
          cache-to: type=gha,mode=max
      - name: Build Web image
        uses: docker/build-push-action@v6
        with:
          context: ./web
          file: Dockerfile.web
          push: false
          tags: nullus-web:ci
          cache-from: type=gha
          cache-to: type=gha,mode=max

  # ── Helm 차트 검증 ────────────────────────────────────────────
  helm-lint:
    name: Helm Chart Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: azure/setup-helm@v4
        with:
          version: latest
      - run: helm lint charts/nullus
      - run: helm template nullus charts/nullus --debug > /dev/null
```

### 5.3 CD Dev 워크플로우 (.github/workflows/cd-dev.yaml)

```yaml
name: CD to Dev

on:
  push:
    branches: [develop]

jobs:
  deploy-dev:
    name: Deploy to Dev (GKE)
    runs-on: ubuntu-latest
    environment: dev
    steps:
      - uses: actions/checkout@v4

      - name: Set image tag
        id: tag
        run: echo "TAG=sha-$(git rev-parse --short HEAD)" >> $GITHUB_OUTPUT

      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Build and push API
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: cloudnullus/nullus-api:${{ steps.tag.outputs.TAG }}
          build-args: VERSION=${{ steps.tag.outputs.TAG }}

      - name: Build and push Web
        uses: docker/build-push-action@v6
        with:
          context: ./web
          file: Dockerfile.web
          push: true
          tags: cloudnullus/nullus-web:${{ steps.tag.outputs.TAG }}

      - uses: google-github-actions/auth@v2
        with:
          credentials_json: ${{ secrets.GCP_SA_KEY_DEV }}

      - uses: google-github-actions/get-gke-credentials@v2
        with:
          cluster_name: nullus-dev
          location: asia-northeast3

      - name: Deploy with Helm
        run: |
          helm upgrade --install nullus charts/nullus \
            --namespace nullus-control \
            --create-namespace \
            --set api.image.tag=${{ steps.tag.outputs.TAG }} \
            --set web.image.tag=${{ steps.tag.outputs.TAG }} \
            --values charts/nullus/values-dev.yaml \
            --wait --timeout=5m
```

### 5.4 릴리스 워크플로우 (.github/workflows/release.yaml)

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    name: Build, Scan, and Release
    runs-on: ubuntu-latest
    permissions:
      contents: write
      packages: write
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Extract version
        id: version
        run: echo "VERSION=${GITHUB_REF_NAME}" >> $GITHUB_OUTPUT

      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      # ── API 이미지 빌드 ─────────────────────────────────────
      - name: Build API image
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: |
            cloudnullus/nullus-api:${{ steps.version.outputs.VERSION }}
            cloudnullus/nullus-api:latest
          build-args: VERSION=${{ steps.version.outputs.VERSION }}
          cache-from: type=registry,ref=cloudnullus/nullus-api:latest
          cache-to: type=inline

      # ── Web 이미지 빌드 ─────────────────────────────────────
      - name: Build Web image
        uses: docker/build-push-action@v6
        with:
          context: ./web
          file: Dockerfile.web
          push: true
          tags: |
            cloudnullus/nullus-web:${{ steps.version.outputs.VERSION }}
            cloudnullus/nullus-web:latest

      # ── Trivy 보안 스캔 ─────────────────────────────────────
      - name: Trivy scan - API image
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: cloudnullus/nullus-api:${{ steps.version.outputs.VERSION }}
          format: sarif
          output: trivy-api.sarif
          severity: CRITICAL,HIGH
          exit-code: '1'   # CRITICAL/HIGH CVE 발견 시 릴리스 중단

      - name: Trivy scan - Web image
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: cloudnullus/nullus-web:${{ steps.version.outputs.VERSION }}
          format: sarif
          output: trivy-web.sarif
          severity: CRITICAL,HIGH
          exit-code: '1'

      - name: Upload scan results to GitHub Security
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: |
            trivy-api.sarif
            trivy-web.sarif

      # ── Helm 차트 패키징 ────────────────────────────────────
      - name: Package Helm chart
        run: |
          helm package charts/nullus --version ${{ steps.version.outputs.VERSION }} \
            --app-version ${{ steps.version.outputs.VERSION }}
          mv nullus-*.tgz helm-chart.tgz

      # ── GitHub Release 생성 ─────────────────────────────────
      - uses: softprops/action-gh-release@v2
        with:
          generate_release_notes: true
          files: |
            helm-chart.tgz
          prerelease: ${{ contains(steps.version.outputs.VERSION, 'alpha') || contains(steps.version.outputs.VERSION, 'beta') }}
```

### 5.5 GitHub Secrets 목록

| Secret 이름 | 용도 | 등록 시점 |
|-------------|------|-----------|
| `DOCKERHUB_USERNAME` | Docker Hub 푸시 계정 | Day 0 |
| `DOCKERHUB_TOKEN` | Docker Hub 접근 토큰 | Day 0 |
| `GCP_SA_KEY_DEV` | GKE Dev 클러스터 접근 | Day 0 |
| `GCP_SA_KEY_STAGING` | GKE Staging 클러스터 접근 | Week 6 |
| `KUBECONFIG_DEV` | Dev kubeconfig (백업) | Week 2 |
| `KUBECONFIG_STAGING` | Staging kubeconfig | Week 6 |

---

## 6. 릴리스 전략

### 6.1 릴리스 단계

```
Alpha (v0.1.0-alpha)          Beta (v0.1.0-beta)            GA (v0.1.0)
2026-03-30                    2026-04-27                    2026-05-25
     │                              │                             │
     │ 대상: 클라우드브로           │ 대상: 공개 베타             │ 대상: 전체 사용자
     │       코어 멤버 5~10명       │       (누구나)              │       (프로덕션 Ready)
     │                              │                             │
     │ 배포 채널:                   │ 배포 채널:                  │ 배포 채널:
     │  GitHub Release (pre)        │  GitHub Release (pre)       │  GitHub Release (latest)
     │  Docker Hub (alpha 태그)     │  Docker Hub (beta 태그)     │  Docker Hub (latest)
     │  Discord 비공개 공지         │  공식 블로그 (한국어)        │  Homebrew Tap
     │                              │                             │  공식 블로그 (한·영)
     │                              │                             │  KCD Korea 2026
```

### 6.2 태그 및 버전 관리

**브랜치 전략 (GitHub Flow 변형)**:

```
main ──────────────────────────────────── 프로덕션 릴리스 (latest 태그)
  │
  ├── develop ──────────────────────────── 통합 브랜치 (sha 태그)
  │     │
  │     ├── feat/F0-org-setup
  │     ├── feat/F4-install-engine
  │     └── fix/F4-helm-timeout
  │
  └── release/v0.1-alpha ────────────── 릴리스 브랜치 (alpha/beta 태그)
```

**시맨틱 버전 관리**:

| 태그 | 브랜치 | Docker Hub 태그 | 비고 |
|------|--------|----------------|------|
| `v0.1.0-alpha` | `release/v0.1-alpha` | `v0.1.0-alpha` | Alpha 릴리스 |
| `v0.1.0-beta` | `release/v0.1-beta` | `v0.1.0-beta` | Beta 릴리스 |
| `v0.1.0` | `main` | `v0.1.0`, `latest` | GA 릴리스 |
| `v0.1.1` | `main` | `v0.1.1`, `latest` | 패치 릴리스 |
| `v0.2.0` | `main` | `v0.2.0`, `latest` | 마이너 릴리스 |

### 6.3 릴리스 프로세스

```bash
# 1. 릴리스 브랜치 생성
git checkout develop
git pull
git checkout -b release/v0.1.0-beta

# 2. 버전 파일 업데이트
echo "v0.1.0-beta" > VERSION
# charts/nullus/Chart.yaml의 version, appVersion 업데이트

# 3. CHANGELOG 업데이트
# CHANGELOG.md에 릴리스 내용 기록

# 4. PR → main 머지 후 태그 생성
git tag -a v0.1.0-beta -m "Beta release v0.1.0-beta"
git push origin v0.1.0-beta
# → release.yaml 워크플로우 자동 실행
```

### 6.4 품질 게이트

| 단계 | 필수 조건 |
|------|-----------|
| **Alpha** | CI 통과, 설치 Happy Path 동작, Happy path E2E 통과 |
| **Beta** | 설치 성공률 ≥85% (3개 K8s 환경), P0 버그 0건, E2E 100% |
| **v1 GA** | 설치 성공률 ≥90%, P0 버그 0건, P1 버그 ≤3건, 테스트 커버리지 ≥70%, 3개 이상 프로덕션 배포 검증 |

---

## 7. Helm Chart 구조

### 7.1 차트 디렉토리 구조

```
charts/
└── nullus/                        ← Nullus Platform 설치용 차트
    ├── Chart.yaml                  ← 차트 메타데이터
    ├── values.yaml                 ← 기본값 (프로덕션 권장 설정)
    ├── values-dev.yaml             ← Dev 환경 오버라이드
    ├── values-staging.yaml         ← Staging 환경 오버라이드
    ├── templates/
    │   ├── _helpers.tpl            ← 공통 템플릿 헬퍼
    │   ├── namespace.yaml
    │   ├── serviceaccount.yaml
    │   ├── rbac.yaml
    │   │
    │   ├── api/
    │   │   ├── deployment.yaml
    │   │   ├── service.yaml
    │   │   ├── configmap.yaml
    │   │   └── hpa.yaml
    │   │
    │   ├── web/
    │   │   ├── deployment.yaml
    │   │   └── service.yaml
    │   │
    │   ├── postgresql/
    │   │   ├── statefulset.yaml
    │   │   ├── service.yaml
    │   │   └── pvc.yaml
    │   │
    │   ├── ingress.yaml
    │   ├── secret.yaml             ← DB 비밀번호, 암호화 키 (Sealed Secret 권장)
    │   ├── networkpolicy.yaml
    │   └── NOTES.txt               ← 설치 완료 안내 메시지
    │
    └── charts/                     ← 차트 의존성 (서브차트)
        └── postgresql-18.x.tgz    ← Bitnami 또는 CNPG 대체
```

### 7.2 Chart.yaml

```yaml
apiVersion: v2
name: nullus
description: Nullus - Kubernetes DevSecOps 플랫폼 빌더
type: application
version: 0.1.0          # Helm 차트 버전
appVersion: "0.1.0"     # Nullus 앱 버전

keywords:
  - kubernetes
  - devops
  - devsecops
  - platform-engineering
  - ci-cd

home: https://nullus.io
sources:
  - https://github.com/cloud-nullus/nullus
maintainers:
  - name: Nullus Team
    email: dev@nullus.io

dependencies:
  # PostgreSQL: Bitnami 대신 레지스트리 우선순위 정책 적용
  # ghcr.io > registry.k8s.io > quay.io > docker.io
  - name: postgresql
    version: "~16.x"
    repository: "https://charts.bitnami.com/bitnami"
    condition: postgresql.enabled
```

### 7.3 values.yaml (핵심 구조)

```yaml
# charts/nullus/values.yaml

# ── 공통 설정 ────────────────────────────────────────────────
global:
  imageRegistry: ""          # 레지스트리 오버라이드 (air-gap 환경용)
  imagePullSecrets: []
  storageClass: ""

# ── Nullus API Server ─────────────────────────────────────────
api:
  replicaCount: 2
  image:
    repository: cloudnullus/nullus-api
    pullPolicy: IfNotPresent
    tag: ""                  # Chart.yaml appVersion을 기본값으로 사용
  service:
    type: ClusterIP
    port: 8080
  resources:
    requests:
      cpu: 250m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 512Mi
  autoscaling:
    enabled: false
    minReplicas: 2
    maxReplicas: 5
    targetCPUUtilizationPercentage: 70
  config:
    logLevel: info
    environment: production
  # 보안: kubeconfig 암호화 키는 반드시 외부 Secret으로 주입
  existingSecret: ""         # 기존 Secret 이름 (비어 있으면 자동 생성)

# ── Nullus Web UI ────────────────────────────────────────────
web:
  replicaCount: 2
  image:
    repository: cloudnullus/nullus-web
    pullPolicy: IfNotPresent
    tag: ""
  service:
    type: ClusterIP
    port: 80
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 200m
      memory: 128Mi

# ── PostgreSQL ────────────────────────────────────────────────
postgresql:
  enabled: true              # false: 외부 PostgreSQL 사용
  auth:
    database: nullus
    username: nullus
    existingSecret: ""       # Secret에서 비밀번호 로드
  primary:
    persistence:
      enabled: true
      size: 20Gi
    resources:
      requests:
        cpu: 250m
        memory: 256Mi

# 외부 PostgreSQL 사용 시 (postgresql.enabled: false)
externalDatabase:
  host: ""
  port: 5432
  database: nullus
  username: nullus
  existingSecret: ""
  existingSecretPasswordKey: postgres-password

# ── Ingress ───────────────────────────────────────────────────
ingress:
  enabled: true
  className: nginx            # 또는 traefik, alb 등
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: nullus.example.com
      paths:
        - path: /api
          pathType: Prefix
          backend: api
        - path: /ws
          pathType: Prefix
          backend: api
        - path: /
          pathType: Prefix
          backend: web
  tls:
    - secretName: nullus-tls
      hosts:
        - nullus.example.com

# ── Keycloak OIDC (v1.0 GA) ──────────────────────────────────
oidc:
  enabled: false             # Alpha/Beta: false, v1 GA: true
  issuerURL: ""
  clientID: nullus-web
  existingSecret: ""         # client-secret 포함

# ── 모니터링 (Nullus 자체) ────────────────────────────────────
monitoring:
  serviceMonitor:
    enabled: false           # Prometheus Operator 사용 시 true
    namespace: ""
    labels: {}

# ── 네트워크 정책 ─────────────────────────────────────────────
networkPolicy:
  enabled: false
```

### 7.4 Helm 설치 명령어

```bash
# 1. Helm 리포지토리 추가
helm repo add nullus https://charts.nullus.io
helm repo update

# 2. 기본 설치 (최소 설정)
helm install nullus nullus/nullus \
  --namespace nullus-system \
  --create-namespace \
  --set ingress.hosts[0].host=nullus.your-domain.com

# 3. 커스텀 values 파일로 설치
helm install nullus nullus/nullus \
  --namespace nullus-system \
  --create-namespace \
  --values my-values.yaml

# 4. 업그레이드
helm upgrade nullus nullus/nullus \
  --namespace nullus-system \
  --values my-values.yaml \
  --atomic --timeout=10m

# 5. 설치 상태 확인
helm status nullus -n nullus-system
kubectl get all -n nullus-system
```

---

## 8. 환경별 설정 관리

### 8.1 환경 구성 전략

```
Local           Dev (GKE)            Staging (GKE)       Production (User K8s)
  │                 │                     │                      │
docker-compose  helm install         helm install          helm install
+ Kind K8s      values-dev.yaml      values-staging.yaml   values.yaml (user)
  │
.env 파일
```

### 8.2 환경별 Helm values 오버라이드

```yaml
# charts/nullus/values-dev.yaml
# Dev 환경 오버라이드: 리소스 절감, 디버그 활성화

api:
  replicaCount: 1
  image:
    pullPolicy: Always        # 항상 최신 이미지 pull
  config:
    logLevel: debug
    environment: dev
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 256Mi
  autoscaling:
    enabled: false

web:
  replicaCount: 1
  image:
    pullPolicy: Always

postgresql:
  primary:
    persistence:
      size: 5Gi              # Dev는 소용량
    resources:
      requests:
        cpu: 100m
        memory: 128Mi

ingress:
  hosts:
    - host: nullus.dev.nullus.io
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: nullus-dev-tls
      hosts:
        - nullus.dev.nullus.io

monitoring:
  serviceMonitor:
    enabled: true
```

```yaml
# charts/nullus/values-staging.yaml
# Staging 환경 오버라이드: 프로덕션 미러, 설치 성공률 측정용

api:
  replicaCount: 2
  config:
    logLevel: info
    environment: staging

web:
  replicaCount: 2

postgresql:
  primary:
    persistence:
      size: 10Gi

ingress:
  hosts:
    - host: nullus.staging.nullus.io

monitoring:
  serviceMonitor:
    enabled: true
```

### 8.3 Secret 관리

**원칙**: 민감 정보는 코드에 절대 포함하지 않습니다.

**OpenBao-first 원칙**: 운영/스테이징 환경에서 민감 정보의 Source of Truth는 OpenBao입니다.

| 시크릿 | 저장 방식 | 환경 |
|--------|-----------|------|
| DB 비밀번호 | `.env` (로컬), **OpenBao (클라우드)** | 전체 |
| Kubeconfig 암호화 키 (AES-256) | `.env` (로컬), **OpenBao (클라우드)** | 전체 |
| Docker Hub 토큰 | GitHub Secrets | CI/CD |
| GCP 서비스 계정 키 | GitHub Secrets | CI/CD |
| Keycloak Client Secret | **OpenBao** | Dev/Staging/Prod |

> 참고: K8s Secret은 앱 주입을 위한 파생 리소스로만 사용하고, 원문 비밀값의 직접 저장소로 사용하지 않습니다.

**K8s Secret 생성 예시**:

```bash
# DB 비밀번호 및 암호화 키 Secret 생성
kubectl create secret generic nullus-secrets \
  --namespace=nullus-system \
  --from-literal=postgres-password="$(openssl rand -base64 32)" \
  --from-literal=kubeconfig-encryption-key="$(openssl rand -hex 16)" \
  --from-literal=session-secret="$(openssl rand -base64 32)"

# Sealed Secrets (GitOps 환경) 사용 권장
kubeseal --format=yaml < secret.yaml > sealed-secret.yaml
```

### 8.4 ConfigMap vs Secret 분류 기준

| 데이터 | 저장 위치 | 예시 |
|--------|-----------|------|
| 비민감 설정 | ConfigMap | LOG_LEVEL, API_PORT, ENVIRONMENT |
| 민감 정보 | OpenBao + (필요 시) Secret 파생 주입 | 비밀번호, API 키, 암호화 키 |
| Kubeconfig | OpenBao 키로 AES-256 암호화 후 DB | 사용자 등록 클러스터 정보 |

### 8.5 OpenBao 연계 배포 순서 (신규)

스택 배포 시 비밀관리 평면을 먼저 준비합니다.

1. Phase A-0: OpenBao 배포 및 health check
2. Phase A: Storage/DB/cert-manager
3. Phase B: 플랫폼 앱 배포
4. Phase C: OIDC/Webhook/ServiceMonitor 연동

연동 규칙:

- OIDC client secret, webhook token, registry credential은 OpenBao 경유로만 주입
- values 파일/로그/에러 메시지에 원문 시크릿 노출 금지

---

## 9. 모니터링/로깅 인프라

Nullus는 오픈소스 플랫폼이므로 **Nullus 자체 운영**을 위한 모니터링과, **사용자 클러스터에 설치하는 모니터링 스택** 두 가지로 구분합니다.

### 9.1 Nullus 플랫폼 자체 모니터링

#### 9.1.1 메트릭 수집 (Prometheus)

**Nullus API Server 메트릭 노출 (Go)**:

```go
// internal/middleware/metrics.go
// Prometheus 미들웨어: 요청별 지연 시간, 상태 코드 수집

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "nullus_http_request_duration_seconds",
            Help:    "HTTP 요청 처리 시간",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path", "status"},
    )

    installationTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "nullus_installation_total",
            Help: "스택 설치 시도 횟수",
        },
        []string{"status", "golden_path"},  // status: success | failure | rollback
    )

    installationDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "nullus_installation_duration_seconds",
            Help:    "스택 설치 소요 시간",
            Buckets: []float64{60, 300, 600, 1200, 3600, 7200},
        },
        []string{"golden_path"},
    )

    activeDeployments = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "nullus_active_deployments",
            Help: "현재 진행 중인 설치/배포 수",
        },
    )
)
```

**ServiceMonitor (Prometheus Operator)**:

```yaml
# templates/monitoring/servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: nullus-api
  namespace: {{ .Values.monitoring.serviceMonitor.namespace | default .Release.Namespace }}
  labels:
    {{- include "nullus.labels" . | nindent 4 }}
    {{- with .Values.monitoring.serviceMonitor.labels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: nullus-api
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
      scrapeTimeout: 10s
```

#### 9.1.2 핵심 알림 규칙

```yaml
# deployments/monitoring/nullus-alerts.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: nullus-alerts
spec:
  groups:
    - name: nullus.api
      interval: 1m
      rules:
        # API 응답 P95 > 500ms 초과
        - alert: NullusAPIHighLatency
          expr: |
            histogram_quantile(0.95,
              rate(nullus_http_request_duration_seconds_bucket[5m])
            ) > 0.5
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "Nullus API 응답 지연 경고"
            description: "P95 응답 시간이 {{ $value | humanizeDuration }} 입니다."

        # 설치 실패율 > 20% 초과
        - alert: NullusHighInstallFailureRate
          expr: |
            rate(nullus_installation_total{status="failure"}[30m]) /
            rate(nullus_installation_total[30m]) > 0.2
          for: 10m
          labels:
            severity: critical
          annotations:
            summary: "설치 실패율 임계치 초과"
            description: "최근 30분 설치 실패율: {{ $value | humanizePercentage }}"

        # API 파드 다운
        - alert: NullusAPIDown
          expr: up{job="nullus-api"} == 0
          for: 1m
          labels:
            severity: critical
          annotations:
            summary: "Nullus API 서버 다운"
```

#### 9.1.3 Grafana 대시보드

개발팀 내부 운영용 대시보드 3종을 `deployments/grafana/dashboards/`에 관리합니다.

| 대시보드 | 주요 패널 |
|----------|-----------|
| **Nullus Overview** | API 요청률, P95 응답 시간, 오류율, 활성 배포 수 |
| **Installation Metrics** | 설치 성공률 추이, 설치 시간 분포, Golden Path별 통계 |
| **Infrastructure** | Pod CPU/Memory, PostgreSQL 커넥션, 디스크 사용량 |

```bash
# Grafana 대시보드 프로비저닝 (ConfigMap으로 관리)
kubectl create configmap nullus-grafana-dashboards \
  --from-file=deployments/grafana/dashboards/ \
  --namespace=monitoring \
  --dry-run=client -o yaml | kubectl apply -f -
```

### 9.2 로깅

**구조화 로깅 (JSON 형식, Uber Zap)**:

```go
// internal/config/logger.go
import "go.uber.org/zap"

func NewLogger(level string) (*zap.Logger, error) {
    cfg := zap.NewProductionConfig()
    cfg.Level = zap.NewAtomicLevelAt(parseLevel(level))
    // 프로덕션: JSON, 개발: 컬러 콘솔
    return cfg.Build(
        zap.Fields(
            zap.String("service", "nullus-api"),
            zap.String("version", Version),
        ),
    )
}
```

**로그 레벨 및 대상**:

| 레벨 | 대상 | 예시 |
|------|------|------|
| `debug` | 개발 환경만 | SQL 쿼리, Helm 명령어 상세 |
| `info` | 모든 환경 | API 요청/응답, 설치 단계 전환 |
| `warn` | 모든 환경 | 재시도 발생, 비권장 설정 사용 |
| `error` | 모든 환경 | 설치 실패, DB 연결 오류 |

**Loki 연동 (선택, Staging/Production)**:

```yaml
# Loki + Promtail 설정 (배포팀 내부 GKE용)
# deployments/monitoring/loki-values.yaml
loki:
  persistence:
    enabled: true
    size: 20Gi
  config:
    limits_config:
      retention_period: 720h   # 30일

promtail:
  config:
    snippets:
      pipelineStages:
        - json:
            expressions:
              level: level
              msg: msg
              trace_id: trace_id
        - labels:
            level:
            trace_id:
```

---

## 10. 보안 설정

### 10.1 TLS 인증서 관리

**cert-manager + Let's Encrypt 사용 (v1.0 GA)**:

```yaml
# deployments/cert-manager/cluster-issuer.yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: dev@nullus.io
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
      - http01:
          ingress:
            class: nginx
---
# Staging 환경용 (rate limit 없음)
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    email: dev@nullus.io
    privateKeySecretRef:
      name: letsencrypt-staging
    solvers:
      - http01:
          ingress:
            class: nginx
```

**TLS 정책**:
- 외부 통신: TLS 1.3 필수 (`ssl_protocols TLSv1.3;`)
- 내부 서비스 간: mTLS (v1.1 이후 검토, 현재는 NetworkPolicy로 격리)
- 자체 서명 인증서: 개발 환경에서만 허용

### 10.2 컨테이너 보안

**보안 컨텍스트 (SecurityContext)**:

```yaml
# charts/nullus/templates/api/deployment.yaml (보안 설정 발췌)
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532      # distroless nonroot
        runAsGroup: 65532
        fsGroup: 65532
        seccompProfile:
          type: RuntimeDefault

      containers:
        - name: nullus-api
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true   # 루트 파일시스템 읽기 전용
            capabilities:
              drop: ["ALL"]               # 모든 Linux capabilities 제거
          volumeMounts:
            - name: tmp
              mountPath: /tmp            # 쓰기 필요 시 tmpfs 마운트

      volumes:
        - name: tmp
          emptyDir:
            medium: Memory
            sizeLimit: 100Mi
```

### 10.3 이미지 보안 스캔

**Trivy 스캔 전략**:

```bash
# 개발 중 로컬 스캔 (Makefile 타겟)
make docker-scan
# 내부적으로:
# trivy image --severity CRITICAL,HIGH cloudnullus/nullus-api:latest
# trivy image --severity CRITICAL,HIGH cloudnullus/nullus-web:latest
```

**스캔 정책**:
- CI/CD 릴리스 파이프라인: CRITICAL/HIGH CVE 발견 시 릴리스 중단
- 기존 이미지: 주 1회 스케줄 스캔 (GitHub Actions scheduled)
- CVE 발견 후 패치 목표: 24시간 이내 (PRD 5.2 보안 요구사항)

```yaml
# .github/workflows/scheduled-scan.yaml
name: Scheduled Security Scan
on:
  schedule:
    - cron: '0 2 * * 1'   # 매주 월요일 02:00 UTC

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: aquasecurity/trivy-action@master
        with:
          image-ref: cloudnullus/nullus-api:latest
          severity: CRITICAL,HIGH
          format: table
      - name: Notify on findings
        if: failure()
        uses: slackapi/slack-github-action@v1
        with:
          payload: '{"text": "⚠️ Nullus 이미지 취약점 스캔 결과: CRITICAL/HIGH CVE 발견됨"}'
        env:
          SLACK_WEBHOOK_URL: ${{ secrets.SLACK_SECURITY_WEBHOOK }}
```

### 10.4 RBAC 및 최소 권한

**Nullus ServiceAccount 권한 (대상 K8s 클러스터)**:

```yaml
# Nullus 설치 엔진이 사용하는 ClusterRole
# 설치 엔진은 Helm 차트 설치를 위해 광범위한 권한 필요
# 단, 직접 시크릿 읽기는 제한

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: nullus-installer
rules:
  # Helm 차트 배포 필수 권한
  - apiGroups: ["*"]
    resources: ["namespaces", "deployments", "services", "configmaps",
                "serviceaccounts", "clusterroles", "clusterrolebindings",
                "customresourcedefinitions", "persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # Helm secret (릴리스 정보 저장)
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "create", "update", "delete"]
    resourceNames: ["sh.helm.release.*"]   # Helm 릴리스 Secret만 접근
```

**Nullus API Server ServiceAccount (컨트롤 플레인)**:

```yaml
# 최소 권한: 자체 네임스페이스 내 Pod 상태 조회만 허용
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: nullus-api
  namespace: nullus-system
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["get", "list", "watch"]
```

### 10.5 네트워크 정책

```yaml
# Nullus API Server: PostgreSQL 접근만 허용
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: nullus-api-netpol
  namespace: nullus-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: nullus-api
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: nullus-web
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: ingress-nginx
      ports:
        - port: 8080
  egress:
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: postgresql
      ports:
        - port: 5432
    - to:                         # K8s API 접근 (설치 엔진)
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - port: 6443
        - port: 443
    - to: []                      # DNS
      ports:
        - port: 53
          protocol: UDP
```

---

## 11. 백업/복구 전략

### 11.1 PostgreSQL 백업

Nullus PostgreSQL에는 다음 데이터가 저장됩니다:
- Organization, 사용자, 클러스터 정보
- 스택 설정 및 버전 이력 (스냅샷)
- 배포 이력 및 로그
- 암호화된 Kubeconfig

**백업 전략**:

```
┌────────────────────────────────────────────────────────────┐
│                    백업 계층                                 │
│                                                            │
│  1. pg_dump 일별 스냅샷 (CronJob)                           │
│     └── MinIO / GCS 버킷 저장 (30일 보관)                   │
│                                                            │
│  2. WAL 아카이빙 (CNPG 사용 시, v1 GA 이후 검토)             │
│     └── Point-In-Time Recovery (PITR) 지원                  │
│                                                            │
│  3. 이력 관리 (Nullus 자체 기능)                             │
│     └── 스택 설정 버전 스냅샷 → JSONB로 DB 저장              │
└────────────────────────────────────────────────────────────┘
```

**pg_dump CronJob (K8s 배포 시)**:

```yaml
# charts/nullus/templates/backup/cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nullus-db-backup
  namespace: {{ .Release.Namespace }}
spec:
  schedule: "0 2 * * *"        # 매일 02:00 UTC
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: 7
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: pg-backup
              image: postgres:18-alpine
              command:
                - /bin/sh
                - -c
                - |
                  BACKUP_FILE="nullus_backup_$(date +%Y%m%d_%H%M%S).sql.gz"
                  pg_dump $DATABASE_URL | gzip > /tmp/$BACKUP_FILE
                  # MinIO 또는 GCS로 업로드
                  # mc cp /tmp/$BACKUP_FILE minio/nullus-backups/$BACKUP_FILE
                  echo "백업 완료: $BACKUP_FILE"
              env:
                - name: DATABASE_URL
                  valueFrom:
                    secretKeyRef:
                      name: nullus-secrets
                      key: database-url
              resources:
                requests:
                  cpu: 100m
                  memory: 128Mi
                limits:
                  cpu: 500m
                  memory: 256Mi
```

**로컬 개발 환경 수동 백업**:

```bash
# 백업
make db-backup
# 내부적으로:
# docker compose exec postgres pg_dump -U nullus nullus | gzip > backup_$(date +%Y%m%d).sql.gz

# 복구
make db-restore FILE=backup_20260314.sql.gz
# 내부적으로:
# gunzip -c $FILE | docker compose exec -T postgres psql -U nullus nullus
```

### 11.2 설정 파일 백업

**Nullus 스택 설정 Git 백업 (사용자 권장 사항)**:

```bash
# Nullus Web UI에서 설정 내보내기 (Phase 2 기능)
# 현재(Phase 1)는 사용자가 직접 백업 권장

# DB에서 현재 스택 설정 추출
pg_dump --table=stack_configs --table=stack_config_versions \
  $DATABASE_URL > nullus-config-backup.sql
```

### 11.3 복구 절차

#### 시나리오 1: PostgreSQL 데이터 복구

```bash
# 1. 서비스 중지
kubectl scale deployment nullus-api --replicas=0 -n nullus-system

# 2. 데이터 복구
kubectl exec -it nullus-postgresql-0 -n nullus-system -- \
  psql -U nullus -d nullus -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

kubectl exec -i nullus-postgresql-0 -n nullus-system -- \
  psql -U nullus -d nullus < backup_20260314.sql

# 3. 서비스 재시작
kubectl scale deployment nullus-api --replicas=2 -n nullus-system

# 4. 마이그레이션 상태 확인
kubectl exec -it deploy/nullus-api -n nullus-system -- \
  /nullus-server migrate status
```

#### 시나리오 2: Helm 릴리스 롤백

```bash
# Helm 이력 확인
helm history nullus -n nullus-system

# 이전 버전으로 롤백
helm rollback nullus [REVISION] -n nullus-system --wait

# 상태 확인
helm status nullus -n nullus-system
kubectl rollout status deployment/nullus-api -n nullus-system
```

---

## 12. 운영 런북

### 12.1 일반 장애 대응 절차

#### 설치 실패 시

```
1. 배포 로그 확인 (Nullus Web UI 또는 CLI)
   → 어느 Phase/Step에서 실패했는지 확인

2. 원인 분류:
   a. 네트워크/방화벽: Helm 차트 다운로드 실패
      → helm repo update, 프록시 설정 확인
   b. 리소스 부족: Pod Pending 상태
      → kubectl describe pod <pod-name> -n <ns>
   c. 권한 부족: RBAC 오류
      → kubectl auth can-i ... --as=system:serviceaccount:...
   d. Helm 엣지 케이스: known-issues.yaml 패턴 참조

3. 자동 롤백 확인 (Alpha: 전체 롤백만 지원)
   → Nullus Web UI에서 롤백 상태 확인

4. 수동 롤백 (필요 시)
   helm uninstall <release-name> -n <namespace>
   kubectl delete namespace <namespace>  # 데이터 삭제 주의
```

#### DB 연결 장애 시

```bash
# PostgreSQL Pod 상태 확인
kubectl get pod -n nullus-system -l app.kubernetes.io/name=postgresql

# 로그 확인
kubectl logs -n nullus-system nullus-postgresql-0 --tail=100

# 커넥션 수 확인
kubectl exec -it nullus-postgresql-0 -n nullus-system -- \
  psql -U nullus -c "SELECT count(*) FROM pg_stat_activity;"

# API 서버 재시작 (커넥션 풀 리셋)
kubectl rollout restart deployment/nullus-api -n nullus-system
```

#### WebSocket 연결 끊김 시

```
1. Ingress 설정 확인: proxy_read_timeout, proxy_send_timeout 값
2. 클라이언트에서 자동 재연결 로직 동작 확인
   (프론트엔드: WebSocket 재연결 로직 내장됨)
3. 로드밸런서 idle timeout 설정 확인 (기본 60초 → 3600초로 증가 필요)
```

#### Keycloak 인증 토큰 만료 시 (v1.0 GA)

```bash
# Keycloak Pod 상태 확인
kubectl get pod -n nullus-system -l app.kubernetes.io/name=keycloak

# 세션 설정 확인 (realm 설정)
# Keycloak Admin Console → Realm Settings → Sessions

# Refresh Token 강제 갱신
# 프론트엔드에서 /auth/refresh 엔드포인트 호출
```

#### OpenBao 토큰 자동 갱신 실패 시

```text
1. 상태 확인
   - Token Source 상태: failed_retryable | failed_manual | expired
   - 최근 이벤트에서 provider 오류 코드/권한 오류 확인

2. 분기 처리
   a. failed_retryable
      - 백오프 재시도 상태인지 확인
      - provider rate limit/네트워크 장애 해소 후 수동 rotate 트리거

   b. failed_manual
      - 승인 필요 토큰인지 확인
      - Admin이 approve 액션 수행 후 rotate 재시작

   c. expired
      - P0 알림 발행
      - 임시 운영토큰(브레이크글래스) 적용 후 정상 회전 절차 복구

3. 반영 검증
   - OpenBao path 최신 버전 확인
   - ESO/CSI 동기화 시각 확인
   - 대상 앱 reload/rolling restart 후 인증 성공 확인

4. 사후 조치
   - 원인 코드 기록 (rate_limited, policy_denied, provider_unavailable 등)
   - token_rotation_events + audit_logs에 조치 이력 남김
```

#### OpenBao 승인/롤백 운영 절차

```text
1. 승인(approve)
   - 조건: failed_manual 상태, 변경 이력 검토 완료
   - 실행: Admin 승인 -> rotate 재개

2. 롤백(rotate rollback)
   - 새 토큰 반영 후 앱 인증 실패 시
   - OpenBao 이전 버전으로 되돌림 -> 주입 동기화 -> 앱 재검증

3. 재개(resume)
   - pause 상태에서 운영 창구 승인 후 자동 갱신 재개
```

#### 관리자 토큰 조회(step-up) 운영 절차

```text
1. 기본 원칙
   - 토큰 원문 조회는 기본 비활성(마스킹 표시)
   - 원문 조회(reveal)는 관리자 재인증(step-up) 이후만 허용

2. 재인증
   - 비밀번호 재입력 또는 OIDC step-up 인증 수행
   - 성공 시 짧은 세션 토큰 발급(권장 TTL: 5분)

3. 조회 제한
   - TTL 만료 후 재조회 시 재인증 필수
   - 조회/복사 횟수 및 속도 제한 적용

4. 감사/보안
   - 누가/언제/어떤 path를 조회했는지 audit_logs 기록
   - 실패 반복 시 보안 알림 발행
```

### 12.2 온콜 체계 (v1 GA 이후)

| 레벨 | 담당 | 연락 방법 |
|------|------|-----------|
| **1차 대응** | BE/DevOps 주간 로테이션 | Discord #alerts |
| **2차 대응** | FE/풀스택 | Discord DM |
| **P0 긴급** | 전체 팀 | Discord @everyone |

### 12.3 정기 유지보수

| 주기 | 작업 |
|------|------|
| 매일 | DB 백업 자동 실행 확인, 알림 점검 |
| 매주 | 이미지 취약점 스캔 결과 검토 |
| 매월 | 호환성 매트릭스 업데이트 검토, 의존성 업그레이드 |
| 분기 | 재해 복구 훈련 (DB 복구 절차 실습) |

---

## 부록

### A. 레지스트리 우선순위 정책

Docker Hub Rate Limit 및 Bitnami 상용화 대응:

```
ghcr.io > registry.k8s.io > quay.io > docker.io
```

Helm 차트에서 이미지 소스를 결정할 때 위 우선순위를 따릅니다.

### B. known-issues.yaml 패턴 요약

Narwhal 레퍼런스에서 도출한 70+ Helm 엣지 케이스 패턴 중 핵심 항목:

| 패턴 | 해결 방법 |
|------|-----------|
| CRD 크기 262KB 초과 | `--server-side --force-conflicts` 플래그 자동 적용 |
| 비핵심 앱 배포 대기 | `--wait` 제거, `--timeout`만 사용 |
| ARM64 노드 미지원 이미지 | 노드 아키텍처 감지 후 대체 이미지 자동 선택 |
| Helm 릴리스 상태 불일치 | `helm history`로 확인 후 강제 삭제/재설치 |
| PostgreSQL 초기화 대기 | `pg_isready` 헬스체크 통과 후 다음 단계 진행 |

### C. 비용 요약 (팀 인프라)

| 항목 | 월 비용 | 비고 |
|------|---------|------|
| GitHub Team | ~$24 | 6명 × $4/인 |
| GKE Dev (Autopilot) | ~$80~120 | 통합 테스트용 |
| GKE Staging (Autopilot) | ~$100~150 | Week 6부터 |
| Docker Hub | $0 | Public 이미지 무료 |
| 합계 | **~$200~300/월** | 초기 12주 기준 |
