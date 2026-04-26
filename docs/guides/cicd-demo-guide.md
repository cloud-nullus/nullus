# Nullus CI/CD 파이프라인 데모 시연 가이드

## 개요
이 가이드는 Nullus CI/CD 파이프라인을 사용해 샘플 앱을 K8s에 배포하는 전체 과정을 설명합니다.
시연 시간은 약 10분 정도 소요됩니다.
발표 청중이나 내부 리뷰어를 대상으로 작성했습니다.

## 사전 준비

### 필수 환경
- Kind 클러스터 2개 실행 중 (nullus-platform, nullus-develop)
- Nullus 플랫폼 실행 중 (localhost:8090 API, localhost:5173 UI)
- Docker Desktop 실행 중
- nullus-sample-app GitHub 레포 접근 가능

### 환경 확인 명령어
```bash
# Kind 클러스터 확인
kind get clusters

# API 헬스체크
curl http://localhost:8090/health

# Docker 상태 확인
docker info
```

### 클러스터 등록 (최초 1회)
```bash
./scripts/register-kind-clusters.sh
```

### 기존 배포 정리 (깨끗한 상태에서 시작)
```bash
kubectl --context kind-nullus-develop delete ns nullus-sample --ignore-not-found
kubectl --context kind-nullus-develop delete deploy,svc -l app -n default
```

## 데모 시나리오

### Step 1: Nullus 플랫폼 로그인
- http://localhost:5173 접속합니다.
- developer@nullus.dev / developer123 계정으로 로그인합니다.
- Pipeline Setup & Deploy 페이지로 자동 이동합니다.

### Step 2: Backend 배포 (Quick Start 템플릿)
- "Quick Start, Select a Template" 섹션에서 "Nullus Sample App, Backend" 클릭합니다.
- 자동으로 채워지는 필드들을 확인합니다:
  - App Name: sample-backend
  - Git URL: https://github.com/cloud-nullus/nullus-sample-app
  - Dockerfile Path: backend/Dockerfile
  - Docker Context: backend/
- Step 3 (Cluster / Namespace) 단계로 자동 이동합니다.
  - Cluster: kind-nullus-develop (기본 선택)
  - Namespace: default
- Next 버튼을 계속 눌러 Step 6 Manifest Review까지 이동합니다.
- Deploy 버튼을 클릭합니다.
- 6단계 실시간 진행 표시를 확인합니다:
  ✅ Git Clone, ✅ Docker Build, ✅ Image Load, ✅ Namespace, ✅ Deployment, ✅ Service
- 배포 완료 후 생성된 리소스 목록을 확인합니다.

### Step 3: Frontend 배포 (Quick Start 템플릿)
- 페이지를 새로고침하거나 CI/CD List 메뉴에서 Pipeline Setup & Deploy로 이동합니다.
- "Nullus Sample App, Frontend" 클릭합니다.
- 자동으로 채워지는 필드들을 확인합니다:
  - App Name: sample-frontend
  - Git URL: https://github.com/cloud-nullus/nullus-sample-app
  - Dockerfile Path: frontend/Dockerfile
  - Docker Context: frontend/
  - Environment Variables: BACKEND_HOST=sample-backend:8080 (템플릿 자동 상속)
- Cluster는 kind-nullus-develop, Namespace는 default로 설정합니다.
- Deploy 버튼을 클릭합니다.
- 6단계 배포 과정을 거쳐 완료합니다.

### Step 4: 배포 결과 확인
#### K8s 리소스 확인
```bash
kubectl --context kind-nullus-develop get pods,svc -n default | grep sample
```

#### API 통신 검증 (port-forward)
```bash
kubectl --context kind-nullus-develop port-forward -n default svc/sample-frontend 3000:80
```

#### 웹 브라우저 접속
- http://localhost:3000 접속합니다.
- 다음 6개 섹션을 확인합니다:
  1. Hero: "Nullus, Enterprise DevSecOps Platform"
  2. Features: 백엔드 API에서 6개 기능 카드 로드 확인
  3. Architecture: 5개 Bounded Context 다이어그램
  4. Tech Stack: Go, React, K8s 등 기술 배지
  5. API Status: 녹색 "Online" 표시 (실시간 헬스체크)
  6. Deployment Info: Pod 이름, Go 버전 등 실시간 메타데이터

### Step 5: CI/CD History 확인
- 사이드바에서 CI/CD History 페이지로 이동합니다.
- sample-backend와 sample-frontend 두 건의 배포 이력을 확인합니다.
- 각 배포의 상태, 버전, 배포자, 소요 시간을 체크합니다.

## 트러블슈팅

### Docker build 실패
- Docker Desktop이 실행 중인지 확인합니다.
- `docker info` 명령어로 Docker 데몬 상태를 점검합니다.

### Kind load 실패
- `kind get clusters` 명령어로 클러스터가 있는지 확인합니다.
- `kind load docker-image` 명령은 로컬 Docker 이미지를 Kind 클러스터로 보냅니다.

### Frontend가 Backend API를 못 찾음
- Backend가 먼저 배포되어야 합니다. Service DNS 등록이 필요하기 때문입니다.
- BACKEND_HOST 환경변수가 sample-backend:8080 인지 확인합니다.
- 같은 namespace에 배포해야 DNS 해석이 가능합니다.

### port-forward 연결 끊김
- Pod가 Running 상태인지 `kubectl get pods` 명령어로 확인합니다.
- 기존 port-forward 프로세스를 `pkill -f port-forward` 명령어로 종료합니다.

## 시연 후 정리
```bash
kubectl --context kind-nullus-develop delete deploy,svc sample-backend sample-frontend -n default
```

## 아키텍처 요약

### 파이프라인 플로우
Git Repository, Nullus CI/CD, Git Clone, Docker Build, Kind Load, K8s Deploy 순서로 진행합니다.
K8s Deploy 단계에서 Namespace, Deployment, Service가 생성됩니다.

### 네트워크 플로우
사용자가 localhost:3000 접속 시 port-forward를 통해 sample-frontend(nginx:80)로 연결됩니다.
프론트엔드는 /api/* 경로 요청을 sample-backend:8080(Go API)으로 보냅니다.

### 기술 스택
- Backend: Go 1.26 (net/http), 포트 8080
- Frontend: React 19 + Vite + Tailwind CSS 4, Nginx 서빙
- nginx: /api/* 경로를 sample-backend:8080으로 리버스 프록시
- K8s: Kind 클러스터, Deployment + ClusterIP Service 사용
