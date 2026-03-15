# Nullus 로컬 개발환경 세팅 가이드

**프로젝트**: Nullus - Kubernetes DevSecOps 플랫폼 빌더
**작성일**: 2026-03-08
**대상**: 개발팀 전원 (주니어 포함)
**기반 문서**: Nullus PRD v1.2, 상세 기능 명세 및 시스템 아키텍처

---

## Section 1: 개요

이 문서는 Nullus 프로젝트를 로컬에서 개발하고 테스트하기 위한 환경 구성 가이드입니다. Nullus는 복잡한 Kubernetes 생태계의 도구들을 자동화하여 설치하고 관리하는 플랫폼이므로, 개발 환경 역시 로컬 Kubernetes 클러스터와 컨테이너 환경을 포함합니다.

### 1.1 로컬 개발 환경의 목적

Nullus 개발 환경은 단순히 코드를 작성하는 공간을 넘어, 실제 Kubernetes 클러스터에 도구를 배포하고 연동하는 과정을 시뮬레이션할 수 있어야 합니다. 이를 위해 다음과 같은 목표를 가집니다.

- **격리성**: 로컬 환경의 설정이 다른 프로젝트나 시스템에 영향을 주지 않아야 합니다.
- **재현성**: 모든 팀원이 동일한 버전의 도구와 설정을 사용하여 "내 컴퓨터에서는 되는데" 문제를 방지합니다.
- **신속성**: Hot Reload(Air, Vite)를 통해 코드 변경 사항을 즉시 확인하고 피드백 루프를 단축합니다.
- **검증 가능성**: Kind 클러스터를 통해 실제 K8s API와의 연동을 로컬에서 완벽히 테스트합니다.

### 1.2 아키텍처 다이어그램 (로컬 개발 환경)

```
┌─ 로컬 개발환경 ──────────────────────────────────────────────────┐
│                                                                    │
│  ┌──────────┐      ┌──────────┐      ┌──────────────────┐         │
│  │ Frontend │      │ Backend  │      │ PostgreSQL       │         │
│  │ React    │◄────►│ Go API   │◄────►│ (docker-compose) │         │
│  │ :5173    │      │ :8080    │      │ :5433            │         │
│  └──────────┘      └────┬─────┘      └──────────────────┘         │
│                         │                                          │
│                         │ client-go / Helm SDK                     │
│                         ▼                                          │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │ Kind K8s Cluster (설치 엔진 E2E 테스트용)                     │ │
│  │                                                                │ │
│  │  - Context: kind-nullus-dev                                    │ │
│  │  - API Server: :6443                                           │ │
│  │  - Ingress Controller (Optional)                               │ │
│  └────────────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────────┘
```

### 1.3 포트 매핑 및 서비스 상세

| 서비스 | 포트 | 프로토콜 | 설명 |
|--------|------|----------|------|
| React (Vite) | 5173 | HTTP | 프론트엔드 HMR 개발 서버. 브라우저 접속 주소. |
| Go API | 8080 | HTTP | 백엔드 REST API 서버. 프론트엔드와 통신. |
| PostgreSQL | 5433 | TCP | 메타데이터 저장용 DB. |
| Keycloak | 8180 | HTTP | 로컬 OIDC 서버 (`http://localhost:8180`, `admin`/`admin`). |
| Kind K8s API | 6443 | HTTPS | 로컬 K8s 클러스터 API 엔드포인트. |
| WebSocket | 8080 | WS | 설치 로그 실시간 스트리밍용 (API 서버 내장). |

### 1.4 기술 스택 상세 (Target Versions)

| 계층 | 기술 | 버전 | 비고 |
|------|------|------|------|
| **Frontend** | React | 19 | Functional Components + Hooks |
| | TypeScript | 5.4+ | Strict Mode 활성화 |
| | Vite | 5.x | 빠른 빌드 및 HMR |
| | Zustand | 5.x | 가벼운 상태 관리 |
| | Tailwind CSS | 4.x | 유틸리티 퍼스트 스타일링 |
| | shadcn/ui | latest | Radix UI 기반 컴포넌트 |
| **Backend** | Go | 1.24+ | 최신 제네릭 및 성능 개선 적용 |
| | Echo | v4 | 고성능 HTTP 웹 프레임워크 |
| | PostgreSQL | 18+ | JSONB 및 고급 쿼리 기능 활용 |
| | golang-migrate| latest | SQL 기반 마이그레이션 관리 |
| | client-go | latest | K8s API 통신 표준 라이브러리 |
| | Helm Go SDK | latest | Helm 차트 프로그래밍 제어 |
| **Infra** | Docker | 24+ | 컨테이너 런타임 |
| | Kind | latest | Docker 기반 로컬 K8s 클러스터 |
| | Air | latest | Go 백엔드 Hot Reload 도구 |

### 1.5 프로젝트 디렉토리 구조 상세

Nullus는 모노레포(Monorepo) 구조를 지향하며, 각 디렉토리의 역할은 다음과 같습니다.

```text
nullus/
├── cmd/                   # 애플리케이션 진입점 (Main 패키지)
│   └── api/               # API 서버 실행 파일
├── internal/              # 외부에서 임포트할 수 없는 내부 로직
│   ├── handler/           # HTTP 요청 처리 (Controller 계층)
│   ├── service/           # 비즈니스 로직 (핵심 엔진 포함)
│   ├── repository/        # 데이터베이스 접근 (DAO 계층)
│   ├── model/             # 도메인 모델 및 DB 스키마 정의
│   ├── engine/            # K8s 설치 및 호환성 검증 엔진
│   └── config/            # 환경 변수 및 설정 로드
├── pkg/                   # 외부 프로젝트에서도 사용 가능한 공용 라이브러리
├── web/                   # React 프론트엔드 프로젝트 (Vite 기반)
│   ├── src/
│   │   ├── components/    # UI 컴포넌트
│   │   ├── pages/         # 라우트별 페이지
│   │   ├── stores/        # Zustand 상태 관리
│   │   └── api/           # API 클라이언트
├── migrations/            # PostgreSQL 마이그레이션 SQL 파일
├── deployments/           # Docker, K8s 배포 관련 설정 파일
├── docs/                  # API 문서 (Swagger) 및 설계 문서
├── scripts/               # 개발 및 운영 보조 스크립트
├── Makefile               # 빌드 및 개발 자동화 명령어
├── docker-compose.yaml    # 로컬 인프라 구성
└── .env.example           # 환경 변수 템플릿
```

---

## Section 2: 공통 사전 준비

모든 운영체제에서 공통적으로 필요한 준비 사항입니다.

### 2.1 Git 설치 및 고급 설정

1. **Git 설치**: 각 OS별 패키지 매니저를 통해 설치합니다.
2. **사용자 설정**:
   ```bash
   git config --global user.name "Your Name"
   git config --global user.email "your-email@example.com"
   ```
3. **유용한 Alias 설정 (권장)**:
   ```bash
   git config --global alias.co checkout
   git config --global alias.br branch
   git config --global alias.ci commit
   git config --global alias.st status
   git config --global alias.lg "log --graph --abbrev-commit --decorate --format=format:'%C(bold blue)%h%C(reset) - %C(bold green)(%ar)%C(reset) %C(white)%s%C(reset) %C(dim white)- %an%C(reset)%C(bold yellow)%d%C(reset)' --all"
   ```
4. **Global Ignore 설정**:
   ```bash
   # ~/.gitignore_global 파일 생성
   .DS_Store
   Thumbs.db
   *.swp
   .vscode/
   .idea/
   
   git config --global core.excludesfile ~/.gitignore_global
   ```
5. **SSH 키 생성 및 등록**:
   ```bash
   ssh-keygen -t ed25519 -C "your-email@example.com"
   # 생성된 ~/.ssh/id_ed25519.pub 내용을 GitHub Settings -> SSH and GPG keys에 등록
   ```
6. **저장소 클론**:
   ```bash
   git clone git@github.com:cloud-nullus/nullus.git
   cd nullus
   ```

### 2.2 IDE 설치 및 확장 프로그램 상세

Nullus 개발팀은 **VS Code** 사용을 권장합니다.

#### VS Code 필수 확장 및 설정 이유

| 확장 | ID | 상세 용도 |
|------|----|-----------|
| **Go** | `golang.go` | 코드 탐색, 자동 완성, 테스트 실행, 디버깅 필수 도구. |
| **ESLint** | `dbaeumer.vscode-eslint` | TypeScript 코드 품질 유지 및 잠재적 버그 방지. |
| **Prettier** | `esbenp.prettier-vscode` | 팀 내 일관된 코드 포맷 유지. |
| **Tailwind CSS** | `bradlc.vscode-tailwindcss` | 클래스명 자동 완성 및 프리뷰 기능 제공. |
| **Docker** | `ms-azuretools.vscode-docker` | Dockerfile 문법 강조 및 컨테이너 상태 모니터링. |
| **Kubernetes** | `ms-kubernetes-tools.vscode-kubernetes-tools` | 클러스터 리소스(Pod, Service 등) 실시간 확인. |
| **Thunder Client** | `rangav.vscode-thunder-client` | 별도 앱 없이 VS Code 내에서 API 요청 테스트. |
| **GitLens** | `eamodio.gitlens` | 라인별 수정자 확인 및 강력한 히스토리 탐색. |
| **Error Lens** | `usernamehw.errorlens` | 에러 메시지를 코드 라인에 즉시 표시하여 가독성 향상. |

---

## Section 3: macOS 개발환경 (OrbStack)

macOS 사용자는 Docker Desktop 대신 성능과 리소스 효율이 뛰어난 **OrbStack** 사용을 강력히 권장합니다.

### 3.1 OrbStack 설치 및 최적화

1. **다운로드**: [OrbStack 공식 홈페이지](https://orbstack.dev)에서 설치 파일을 내려받아 설치합니다.
2. **왜 OrbStack인가?**:
   - **성능**: Docker Desktop 대비 컨테이너 시작 속도가 2~3배 빠릅니다.
   - **리소스**: 유휴 상태에서 CPU 사용량이 거의 0%에 가깝습니다.
   - **네트워크**: 로컬 머신과 컨테이너 간의 네트워크 통신이 매우 매끄럽습니다.
   - **K8s**: 별도의 복잡한 설정 없이 클릭 한 번으로 가벼운 K8s 클러스터를 제공합니다.
3. **설정 최적화**:
   - Settings -> Resources: CPU 4개 이상, Memory 8GB 이상 할당 권장.
   - Settings -> Kubernetes: "Enable Kubernetes" 활성화.
4. **확인**:
   ```bash
   docker version
   kubectl get nodes  # 'orbstack' 노드가 Ready 상태인지 확인
   ```

### 3.2 개발 도구 설치 (Homebrew)

Homebrew를 사용하여 필요한 런타임과 도구들을 설치합니다.

```bash
# Homebrew 설치 (없는 경우)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 1. 언어 런타임
brew install go@1.24
brew install node@20

# 2. Kubernetes 관리 도구
brew install kubectl    # K8s 클러스터 조작
brew install helm       # K8s 패키지 매니저
brew install kind       # 로컬 K8s 클러스터 생성기

# 3. 개발 생산성 도구
brew install golangci-lint # Go 정적 분석 도구
brew install air           # Go 서버 실시간 재시작
brew install jq            # JSON 처리 유틸리티
brew install make          # 빌드 자동화 도구

# 4. 환경 변수 반영
echo 'export PATH="/opt/homebrew/opt/go@1.24/bin:$PATH"' >> ~/.zshrc
echo 'export PATH=$(go env GOPATH)/bin:$PATH' >> ~/.zshrc
source ~/.zshrc
```

### 3.3 프로젝트 초기 설정

저장소를 클론한 후 초기 의존성을 설치합니다.

```bash
cd nullus

# Go 모듈 초기화
go mod download
go mod verify

# 프론트엔드 의존성 설치
cd web
npm install
cd ..

# 환경 변수 설정
cp .env.example .env
```

#### `.env` 파일 상세 가이드

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `POSTGRES_HOST` | `localhost` | 로컬 Docker로 실행 중인 DB 주소. |
| `POSTGRES_PORT` | `5432` | PostgreSQL 기본 포트. |
| `POSTGRES_DB` | `nullus` | 애플리케이션용 데이터베이스 이름. |
| `POSTGRES_USER` | `nullus` | DB 관리자 계정. |
| `POSTGRES_PASSWORD` | `nullus_dev` | 개발용 비밀번호 (보안 주의). |
| `API_PORT` | `8080` | 백엔드 서버가 리스닝할 포트. |
| `VITE_API_URL` | `http://localhost:8080` | 프론트엔드 API 호출 기본 URL. |
| `ENCRYPTION_KEY` | `nullus-dev-key-32bytes-padding!!` | kubeconfig 암호화 키. 반드시 32바이트. |
| `KUBECONFIG` | `~/.kube/config` | Kind 또는 OrbStack K8s 접속 설정. |
| `LOG_LEVEL` | `debug` | 개발 시에는 `debug`, 운영 시에는 `info`. |

백엔드 기본 설정 파일은 `configs/config.yaml`입니다.

### 3.4 인프라 실행 및 마이그레이션

```bash
# 권장: 로컬 인프라/마이그레이션/API/프론트까지 한 번에 기동
./scripts/runbook_local.sh up

# 상태 확인
./scripts/runbook_local.sh status
```

> 수동 실행이 필요하면 `make dev`, `make run`, `make web-dev`를 개별로 사용할 수 있습니다.

### 3.5 Kind 클러스터 구성 (E2E 테스트용)

설치 엔진의 동작을 검증하기 위해 로컬 K8s 클러스터가 필요합니다.

```bash
# Kind 클러스터 생성
kind create cluster --config scripts/kind-cluster.yaml

# Ingress Nginx 설치 (필요 시)
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# 클러스터 준비 대기
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

### 3.6 macOS 개발 Pro Tips

- **Raycast 사용**: Spotlight 대신 Raycast를 사용하고 Docker/Kubernetes 확장을 설치하면 생산성이 크게 향상됩니다.
- **iTerm2 + Oh My Zsh**: 터미널 환경을 개선하고 `kube-ps1` 플러그인을 사용하여 현재 K8s 컨텍스트를 프롬프트에 표시하세요.
- **Homebrew 자동 업데이트 끄기**: `export HOMEBREW_NO_AUTO_UPDATE=1`을 `.zshrc`에 추가하면 도구 설치 시 대기 시간을 줄일 수 있습니다.
- **OrbStack 도메인**: OrbStack은 컨테이너에 `.orb.local` 도메인을 자동으로 할당합니다. 이를 활용하면 IP 대신 도메인으로 서비스 간 통신 테스트가 가능합니다.

---

## Section 4: Windows 개발환경 (Docker Desktop)

Windows 사용자는 **WSL2(Windows Subsystem for Linux 2)** 환경에서 개발하는 것을 원칙으로 합니다.

### 4.1 WSL2 설치 및 최적화

1. **WSL2 설치**:
   ```powershell
   wsl --install -d Ubuntu-24.04
   ```
2. **WSL2 메모리 관리**: Windows 파일 탐색기 주소창에 `%UserProfile%` 입력 후 `.wslconfig` 파일 생성.
   ```ini
   [wsl2]
   memory=8GB      # 시스템 메모리의 50% 권장
   processors=4    # CPU 코어의 50% 권장
   swap=4GB
   guiApplications=false
   ```
3. **파일 시스템 주의사항**:
   - **절대 금지**: `/mnt/c/Users/...` (Windows 영역)에서 작업하지 마세요.
   - **권장**: `~/projects/nullus` (Linux 영역)에서 작업하세요.
   - 이유: Windows-Linux 간 파일 시스템 브릿지는 성능이 매우 느려 `npm install` 등에 수십 분이 소요될 수 있습니다.

### 4.2 Docker Desktop 설정

1. **설치**: [Docker Desktop](https://www.docker.com/products/docker-desktop) 설치.
2. **WSL2 연동**: Settings -> Resources -> WSL Integration에서 "Ubuntu-24.04" 활성화.
3. **Kubernetes**: Settings -> Kubernetes -> "Enable Kubernetes" 체크.
   - 단, 로컬 테스트 시에는 Docker Desktop 내장 K8s보다 **Kind**를 WSL2 내부에서 실행하는 것이 더 유연합니다.

### 4.3 WSL2 내부 도구 설치 (Ubuntu 24.04)

WSL2 터미널을 열고 다음 과정을 진행합니다.

```bash
# 1. 시스템 업데이트
sudo apt update && sudo apt upgrade -y

# 2. Go 1.24 설치
curl -LO https://go.dev/dl/go1.24.1.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc

# 3. Node.js 20 (nvm 사용)
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.0/install.sh | bash
source ~/.bashrc
nvm install 20

# 4. Kubernetes 도구
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

go install sigs.k8s.io/kind@latest
go install github.com/air-verse/air@latest
```

### 4.4 VS Code 연동

1. Windows에서 VS Code 실행.
2. "Remote - WSL" 확장 설치.
3. 왼쪽 하단 초록색 아이콘 클릭 -> "Connect to WSL".
4. WSL 터미널에서 `code .` 입력하여 프로젝트 열기.

### 4.5 Windows 개발 Pro Tips

- **Windows Terminal**: 기본 PowerShell 대신 Windows Terminal을 사용하고 Ubuntu 프로필을 기본값으로 설정하세요.
- **WSL2 미러드 네트워크**: 최신 Windows 11에서는 `.wslconfig`에 `networkingMode=mirrored`를 설정하여 Windows와 WSL2 간의 네트워크 경계를 없앨 수 있습니다.
- **파일 탐색기 연동**: WSL2 터미널에서 `explorer.exe .`을 입력하면 현재 디렉토리를 Windows 파일 탐색기에서 열 수 있습니다.
- **메모리 회수**: WSL2가 메모리를 너무 많이 점유할 경우, PowerShell에서 `wsl --shutdown`으로 초기화하거나 `echo 3 | sudo tee /proc/sys/vm/drop_caches`로 캐시를 비울 수 있습니다.

---

## Section 5: 개발 워크플로우

### 5.1 Makefile 상세 가이드

Nullus는 복잡한 명령어를 단순화하기 위해 `Makefile`을 적극 활용합니다.

| 명령어 | 상세 설명 |
|--------|-----------|
| `make dev` | **가장 많이 사용.** 백엔드(Air)와 프론트엔드(Vite)를 동시에 실행합니다. |
| `make build` | 전체 프로젝트를 빌드하여 `dist/` 폴더에 결과물을 생성합니다. |
| `make docker-build` | API 서버와 웹 서버를 각각 Docker 이미지로 빌드합니다. |
| `make test` | 모든 유닛 테스트를 실행합니다. |
| `make test-backend` | `go test ./internal/...` 명령을 실행합니다. |
| `make test-frontend` | `web` 폴더에서 `npm test`를 실행합니다. |
| `make test-e2e` | Kind 클러스터에 실제 배포 테스트를 수행합니다. |
| `make lint` | `golangci-lint`와 `eslint`를 실행하여 코드 품질을 검사합니다. |
| `make fmt` | 모든 소스 코드를 팀 컨벤션에 맞게 자동 정렬합니다. |
| `make migrate-up` | `migrations/` 폴더의 모든 SQL을 DB에 적용합니다. |
| `make migrate-down` | 가장 최근에 적용된 마이그레이션을 취소합니다. |
| `make swagger` | Go 주석을 파싱하여 `docs/swagger.yaml`을 생성합니다. |
| `make help` | 사용 가능한 모든 명령어 목록을 보여줍니다. |

### 5.2 백엔드 개발 프로세스 (Go)

1. **서버 실행**: `make dev`를 실행하면 `air`가 소스 코드를 감시합니다.
2. **코드 수정**: `internal/` 하위 코드를 수정하면 서버가 1~2초 내에 재시작됩니다.
3. **구조적 설계**:
   - `internal/handler/`: Gin 핸들러 정의. 요청 바인딩 및 응답 처리.
   - `internal/service/`: 비즈니스 로직의 핵심. 인터페이스 기반 설계 권장.
   - `internal/repository/`: DB 쿼리 수행. SQLC 또는 Raw SQL 사용.
4. **테스트 작성**:
   - 파일명은 반드시 `_test.go`로 끝내야 합니다.
   - `testify/assert` 라이브러리를 사용하여 가독성 높은 단언문을 작성하세요.

### 5.3 프론트엔드 개발 프로세스 (React)

1. **서버 실행**: `web/` 디렉토리에서 `npm run dev`가 실행됩니다.
2. **컴포넌트 개발**: `web/src/components/ui/`에 shadcn 컴포넌트를 추가하여 사용합니다.
3. **상태 관리 (Zustand)**:
   ```typescript
   // web/src/stores/useAuthStore.ts 예시
   import { create } from 'zustand';

   interface AuthState {
     user: User | null;
     setUser: (user: User | null) => void;
   }

   export const useAuthStore = create<AuthState>((set) => ({
     user: null,
     setUser: (user) => set({ user }),
   }));
   ```
4. **스타일링**: Tailwind CSS 클래스를 사용하며, 복잡한 조건부 클래스는 `cn()` 유틸리티를 활용합니다.

### 5.4 데이터베이스 마이그레이션 상세

새로운 테이블이나 컬럼이 필요한 경우:

1. **파일 생성**:
   ```bash
   make migrate-create NAME=add_users_table
   ```
2. **SQL 작성**:
   - `up.sql`: `CREATE TABLE users (...);`
   - `down.sql`: `DROP TABLE users;`
3. **적용**:
   ```bash
   make migrate-up
   ```
4. **주의**: 이미 머지된 마이그레이션 파일은 절대 수정하지 마세요. 수정이 필요하면 새로운 마이그레이션 파일을 생성해야 합니다.

### 5.5 Git 협업 및 PR 가이드

#### 브랜치 네이밍 규칙
- `feature/F{Issue#}-{Description}`: 기능 개발 (예: `feature/F12-login-api`)
- `fix/F{Issue#}-{Description}`: 버그 수정 (예: `fix/F45-cors-issue`)
- `docs/F{Issue#}-{Description}`: 문서 작업

#### 커밋 메시지 템플릿
```text
feat: 사용자 로그인 API 구현

- Gin 핸들러 및 JWT 발급 로직 추가
- PostgreSQL users 테이블 연동
- 관련 유닛 테스트 작성

Fixes: #12
```

#### Pull Request 체크리스트
- [ ] `make lint` 결과에 에러가 없는가?
- [ ] `make test`가 모두 통과하는가?
- [ ] 새로운 기능에 대한 테스트 코드가 포함되었는가?
- [ ] 관련 문서(Swagger 등)가 업데이트되었는가?

### 5.6 API 문서화 (Swagger)

Nullus는 `swaggo/swag`를 사용하여 Go 소스 코드의 주석으로부터 OpenAPI 3.0 문서를 자동 생성합니다.

#### 주석 작성 규칙
- **Summary**: API의 짧은 요약 (한 줄)
- **Description**: API의 상세 설명
- **Tags**: API 그룹화 (예: Auth, Cluster, Stack)
- **Accept/Produce**: 요청 및 응답 데이터 형식 (주로 `json`)
- **Param**: 요청 파라미터 (query, path, body 등)
- **Success/Failure**: 응답 코드 및 데이터 모델
- **Router**: 엔드포인트 경로 및 HTTP 메서드

#### 예시 코드
```go
// @Summary 클러스터 등록
// @Description 새로운 Kubernetes 클러스터를 kubeconfig 파일을 통해 등록합니다.
// @Tags Cluster
// @Accept json
// @Produce json
// @Param request body model.ClusterCreateRequest true "클러스터 정보"
// @Success 201 {object} model.ClusterResponse
// @Failure 400 {object} model.ErrorResponse
// @Router /api/v1/clusters [post]
func (h *ClusterHandler) Create(c echo.Context) error { ... }
```

#### 문서 갱신 및 확인
1. 코드 수정 후 `make swagger` 실행.
2. 서버 실행 (`make dev`).
3. 브라우저에서 `http://localhost:8080/swagger/index.html` 접속하여 확인.

### 5.7 테스트 전략 및 작성법

Nullus는 견고한 플랫폼 빌더를 목표로 하며, 이를 위해 다층적인 테스트 전략을 사용합니다.

#### 1. 유닛 테스트 (Unit Test)
- **대상**: 개별 함수, 메서드, 유틸리티.
- **도구**: Go `testing` 패키지, `testify/assert`.
- **규칙**: 외부 의존성(DB, K8s API)은 Mock을 사용하여 격리합니다.
- **실행**: `go test ./internal/service/...`

#### 2. 통합 테스트 (Integration Test)
- **대상**: DB 쿼리, 외부 API 연동 로직.
- **도구**: `testcontainers-go` (필요 시).
- **규칙**: 실제 PostgreSQL 컨테이너를 띄워 쿼리가 정상 작동하는지 확인합니다.
- **실행**: `make test-backend` (일부 포함).

#### 3. E2E 테스트 (End-to-End Test)
- **대상**: 설치 엔진의 전체 워크플로우.
- **도구**: `Kind`, `Helm Go SDK`.
- **규칙**: 로컬 Kind 클러스터에 실제로 도구를 설치하고 상태를 검증합니다.
- **실행**: `make test-e2e`

### 5.8 프론트엔드 상태 관리 (Zustand)

Nullus 프론트엔드는 Redux보다 가볍고 직관적인 **Zustand**를 사용합니다.

#### 스토어 정의 가이드
- **파일 위치**: `web/src/stores/`
- **네이밍**: `use{Name}Store.ts`
- **원칙**: 하나의 스토어는 하나의 도메인(Auth, UI, Config 등)만 담당합니다.

#### 예시: UI 상태 관리
```typescript
import { create } from 'zustand';

interface UIState {
  isSidebarOpen: boolean;
  toggleSidebar: () => void;
  setSidebarOpen: (open: boolean) => void;
}

export const useUIStore = create<UIState>((set) => ({
  isSidebarOpen: true,
  toggleSidebar: () => set((state) => ({ isSidebarOpen: !state.isSidebarOpen })),
  setSidebarOpen: (open) => set({ isSidebarOpen: open }),
}));
```

### 5.9 스타일링 및 UI 컴포넌트 (Tailwind & shadcn/ui)

#### Tailwind CSS 사용 원칙
- **Utility-First**: 가급적 커스텀 CSS 파일 작성을 지양하고 Tailwind 클래스만 사용합니다.
- **Arbitrary Values**: `h-[123px]`와 같은 임의 값 사용은 최소화하고 디자인 시스템의 토큰을 사용합니다.
- **Dark Mode**: `dark:` 접두사를 사용하여 다크 모드 대응을 기본으로 합니다.

#### shadcn/ui 활용
- **컴포넌트 추가**: `npx shadcn-ui@latest add [component-name]`
- **커스터마이징**: `web/src/components/ui/`에 생성된 코드를 직접 수정하여 프로젝트에 맞게 변경합니다.
- **조합**: 복잡한 UI는 여러 shadcn 컴포넌트를 조합하여 새로운 컴포넌트를 만들어 사용합니다.

---

## Section 6: IDE 설정 (VS Code)

### 6.1 프로젝트 설정 (`.vscode/settings.json`)

```json
{
  "editor.formatOnSave": true,
  "editor.codeActionsOnSave": {
    "source.organizeImports": "explicit",
    "source.fixAll.eslint": "explicit"
  },
  "editor.defaultFormatter": "esbenp.prettier-vscode",
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  },
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast", "--timeout", "5m"],
  "go.useLanguageServer": true,
  "typescript.tsdk": "node_modules/typescript/lib",
  "tailwindCSS.experimental.classRegex": [
    ["cva\\(([^)]*)\\)", "[\"'`]([^\"'`]*).*?[\"'`]"],
    ["cn\\(([^)]*)\\)", "[\"'`]([^\"'`]*).*?[\"'`]"]
  ],
  "files.insertFinalNewline": true,
  "files.trimTrailingWhitespace": true
}
```

### 6.2 팀 공유 확장 (`.vscode/extensions.json`)

```json
{
  "recommendations": [
    "golang.go",
    "dbaeumer.vscode-eslint",
    "esbenp.prettier-vscode",
    "bradlc.vscode-tailwindcss",
    "ms-azuretools.vscode-docker",
    "ms-kubernetes-tools.vscode-kubernetes-tools",
    "rangav.vscode-thunder-client",
    "eamodio.gitlens",
    "usernamehw.errorlens",
    "tamasfe.even-better-toml",
    "redhat.vscode-yaml"
  ]
}
```

---

## Section 7: 트러블슈팅 가이드

### 7.1 네트워크 및 포트 문제

**Q: `make dev` 실행 시 포트 충돌 에러가 납니다.**
- **증상**: `listen tcp :8080: bind: address already in use`
- **해결**:
  - 해당 포트를 점유 중인 프로세스를 찾아 종료합니다.
  - macOS: `sudo lsof -i :8080` -> `kill -9 <PID>`
  - Windows: `netstat -ano | findstr :8080` -> `taskkill /F /PID <PID>`

**Q: WSL2에서 로컬 호스트 접속이 안 됩니다.**
- **원인**: WSL2는 별도의 가상 IP를 가집니다.
- **해결**: `.env`에서 `POSTGRES_HOST`를 `localhost` 대신 `127.0.0.1`로 설정하거나, Docker Desktop의 WSL Integration이 켜져 있는지 확인하세요.

### 7.2 데이터베이스 문제

**Q: 마이그레이션 실행 시 `Dirty database` 에러가 발생합니다.**
- **원인**: 이전 마이그레이션이 실패하여 DB 상태가 불확실함.
- **해결**:
  ```bash
  # 마이그레이션 버전을 강제로 고정 (현재 버전이 5라면)
  migrate -path migrations/ -database "postgres://..." force 5
  ```

**Q: PostgreSQL 컨테이너가 계속 재시작됩니다.**
- **원인**: 데이터 폴더 권한 문제 또는 디스크 용량 부족.
- **해결**: `docker logs <container_id>`로 에러 메시지를 확인하세요. 필요 시 `docker volume rm`으로 데이터를 초기화할 수 있습니다 (주의: 데이터 삭제됨).

### 7.3 Kubernetes 및 Kind 문제

**Q: `kind create cluster`가 실패합니다.**
- **원인**: Docker 리소스 부족 또는 이전 클러스터 잔재.
- **해결**:
  - `docker system prune`으로 불필요한 리소스 정리.
  - `kind delete cluster --name nullus-dev` 실행 후 재시도.

**Q: `kubectl` 명령어가 엉뚱한 클러스터로 전송됩니다.**
- **해결**: 컨텍스트를 명시적으로 전환하세요.
  ```bash
  kubectl config use-context kind-nullus-dev
  ```

### 7.4 Go 및 의존성 문제

**Q: VS Code에서 Go 코드가 빨간 줄로 가득합니다.**
- **해결**:
  - `go mod tidy` 실행.
  - VS Code에서 `Command + Shift + P` -> `Go: Install/Update Tools` 실행하여 모든 도구 업데이트.

**Q: `air` 실행 시 파일 변경을 감지하지 못합니다.**
- **원인**: OS의 파일 핸들 제한(ulimit) 초과.
- **해결**:
  - macOS: `ulimit -n 2048`
  - Linux: `/etc/security/limits.conf` 수정.

### 7.5 프론트엔드 문제

**Q: `npm install` 시 `ERESOLVE could not resolve dependency` 에러가 납니다.**
- **해결**: `npm install --legacy-peer-deps`를 사용하거나, `node_modules`와 `package-lock.json`을 삭제 후 재시도하세요.

**Q: Tailwind 클래스를 추가했는데 스타일이 반영되지 않습니다.**
- **해결**: `tailwind.config.js`의 `content` 경로에 해당 파일 확장자가 포함되어 있는지 확인하세요.

### 7.6 Kubeconfig 암호화 및 복호화 문제
- **증상**: 클러스터 등록 후 연결 테스트 시 `decryption failed` 에러 발생.
- **원인**: `.env`의 `ENCRYPTION_KEY`가 변경되었거나 설정되지 않음.
- **해결**:
  - `.env` 파일에 32바이트 길이의 `ENCRYPTION_KEY`가 있는지 확인하세요.
  - 키를 변경하면 기존에 DB에 저장된 kubeconfig는 모두 다시 등록해야 합니다.

### 7.7 Helm 차트 다운로드 실패
- **증상**: `make test-e2e` 실행 중 Helm 차트 fetch 실패.
- **원인**: 네트워크 방화벽 또는 Helm 레포지토리 주소 변경.
- **해결**:
  - `helm repo list`로 레포지토리가 등록되어 있는지 확인하세요.
  - `helm repo update`를 실행하여 최신 인덱스를 가져오세요.
  - 프록시 환경이라면 `HTTPS_PROXY` 환경 변수를 설정해야 할 수 있습니다.

### 7.8 Air (Hot Reload) 미작동 (추가)
- **증상**: Go 파일을 수정해도 서버가 재시작되지 않음.
- **원인**: `.air.toml` 설정 오류 또는 파일 감시 한계 초과.
- **해결**:
  - `.air.toml`의 `include_ext`에 `.go`가 포함되어 있는지 확인하세요.
  - Linux/WSL2: `sudo sysctl fs.inotify.max_user_watches=524288` 명령으로 파일 감시 제한을 상향 조정하세요.

### 7.9 Vite HMR 미작동 (추가)
- **증상**: 프론트엔드 코드 수정 시 브라우저가 자동 갱신되지 않음.
- **원인**: 브라우저와 Vite 서버 간 WebSocket 연결 실패.
- **해결**:
  - 브라우저 개발자 도구(F12)의 Console 탭에서 에러 확인.
  - `.env`의 `VITE_API_URL`이 올바른지 확인.
  - 브라우저 캐시 삭제 후 재시도.

---

## Section 8: 온보딩 체크리스트 (첫 출근 가이드)

새로운 팀원이 합류했을 때 다음 순서대로 환경을 구축하세요.

1. [ ] **GitHub 저장소 접근 권한 확인**: 팀장에게 GitHub ID를 전달하고 `cloud-nullus` 조직에 초대받습니다.
2. [ ] **저장소 클론**: `git clone git@github.com:cloud-nullus/nullus.git`
3. [ ] **가상화 환경 구축**: OrbStack(macOS) 또는 WSL2(Windows)를 설치하고 Kubernetes를 활성화합니다.
4. [ ] **런타임 설치**: Go 1.24 및 Node.js 20을 설치합니다. 버전이 정확한지 `go version`, `node -v`로 확인합니다.
5. [ ] **IDE 설정**: VS Code를 설치하고 추천 확장 프로그램을 모두 설치합니다.
6. [ ] **인프라 실행**: `./scripts/runbook_local.sh up`으로 로컬 인프라/API/프론트를 기동합니다.
7. [ ] **DB 초기화**: `make migrate-up` 명령으로 테이블을 생성합니다.
8. [ ] **개발 서버 실행**: 브라우저에서 `http://localhost:5173` 접속을 확인합니다.
9. [ ] **테스트 실행**: `make test-backend` 및 `make test-frontend`가 모두 통과하는지 확인합니다.
10. [ ] **첫 이슈 할당**: Jira 또는 GitHub Issues에서 첫 번째 작업을 할당받고 브랜치를 생성합니다.

---

## Section 9: 기여 가이드 (Contribution Guide)

### 9.1 코드 리뷰 원칙
- **존중**: 모든 리뷰는 건설적이고 존중하는 언어로 작성합니다. "이 코드는 틀렸습니다" 대신 "이 부분은 ~한 이유로 ~게 개선하면 어떨까요?"라고 제안합니다.
- **객관성**: 개인적인 취향보다는 프로젝트 컨벤션, 성능, 가독성, 유지보수성을 우선합니다.
- **신속성**: PR이 올라오면 24시간 이내에 첫 리뷰를 남기는 것을 지향합니다.
- **승인 조건**: 최소 1명 이상의 Approve가 있어야 머지 가능합니다.

### 9.2 문서화 의무
- **API**: 새로운 API를 만들거나 기존 API를 수정하면 반드시 Swagger 주석을 업데이트하고 `make swagger`를 실행합니다.
- **설계**: 복잡한 비즈니스 로직이나 아키텍처 변경이 포함된 경우 `docs/` 폴더에 ADR(Architecture Decision Record) 또는 설계 문서를 추가합니다.
- **사용자 가이드**: UI 변경이나 새로운 기능이 추가되어 사용자 가이드가 필요한 경우 관련 문서를 업데이트합니다.

### 9.3 보안 수칙
- **비밀번호 및 키**: API Key, DB 비밀번호, 인증 토큰 등 민감 정보를 코드에 하드코딩하는 것은 절대 금지입니다.
- **환경 변수**: 반드시 `.env` 파일을 사용하고, 새로운 환경 변수가 추가되면 `.env.example`에도 반영하여 다른 팀원들이 알 수 있게 합니다.
- **의존성**: 새로운 패키지를 추가할 때는 보안 취약점이 없는지 확인하고, 가급적 널리 사용되는 라이브러리를 선택합니다.

### 9.4 테스트 의무
- 모든 버그 수정에는 해당 버그를 재현하는 테스트 코드가 포함되어야 합니다.
- 새로운 기능 추가 시 유닛 테스트는 필수이며, 가능한 경우 통합 테스트까지 작성합니다.
- PR 제출 전 `make lint`를 실행하여 정적 분석 에러가 없는지 확인합니다.
