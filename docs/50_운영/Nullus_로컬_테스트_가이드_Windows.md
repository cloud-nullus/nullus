# Nullus 로컬 테스트 가이드 (Windows)

Windows 환경에서 Nullus를 로컬 실행하려면 **WSL2 (Windows Subsystem for Linux 2)**가 필요합니다. Nullus의 빌드 스크립트(`runbook_local.sh`, `Makefile`)는 bash 기반이며, Go/Node 툴체인도 Linux 환경에서 가장 안정적으로 동작합니다.

> macOS/Linux 환경은 [Nullus_로컬_테스트_가이드.md](./Nullus_로컬_테스트_가이드.md)를 참고하세요.

---

## 1. 사전 요구사항

### 1.1 WSL2 설치

PowerShell (관리자 권한)에서 실행:

```powershell
wsl --install -d Ubuntu-24.04
```

설치 후 재부팅하고, Ubuntu 터미널에서 사용자 이름/비밀번호를 설정합니다.

WSL2 버전 확인:

```powershell
wsl --version
# WSL 버전: 2.x.x 이상 필요
```

### 1.2 Docker Desktop for Windows

1. [Docker Desktop](https://www.docker.com/products/docker-desktop/) 설치
2. Settings > General > **Use the WSL 2 based engine** 체크
3. Settings > Resources > WSL Integration > Ubuntu-24.04 활성화
4. Docker Desktop 재시작

WSL2 터미널에서 확인:

```bash
docker --version
docker compose version
```

### 1.3 개발 도구 설치 (WSL2 Ubuntu 내부)

WSL2 Ubuntu 터미널을 열고 다음을 실행합니다:

```bash
# Go 1.24+
wget https://go.dev/dl/go1.24.1.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
go version

# Node.js 22+ (nvm 사용)
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.0/install.sh | bash
source ~/.bashrc
nvm install 22
node --version

# 기타 도구
sudo apt update && sudo apt install -y make curl lsof git
```

선택 도구 (K8s 테스트 시):

```bash
# kind
go install sigs.k8s.io/kind@latest

# kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl && sudo mv kubectl /usr/local/bin/

# helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

---

## 2. 프로젝트 클론 및 실행

### 2.1 리포지토리 클론

WSL2 터미널에서 실행합니다. Windows 파일시스템(`/mnt/c/`)이 아닌 **Linux 파일시스템**(`~/`)에 클론해야 성능이 좋습니다.

```bash
cd ~
git clone https://github.com/cloud-nullus/draft.git
cd draft
```

### 2.2 인프라 기동

```bash
./scripts/runbook_local.sh up
```

모든 서비스가 시작됩니다:

| 서비스 | 주소 | 접근 방법 |
|--------|------|-----------|
| API 서버 | `http://localhost:8090` | WSL2 + Windows 브라우저 모두 가능 |
| 프론트엔드 | `http://localhost:5173` | Windows 브라우저에서 접근 |
| PostgreSQL | `localhost:5433` | WSL2 터미널에서 psql |
| Keycloak | `http://localhost:8180` | Windows 브라우저 |
| MinIO 콘솔 | `http://localhost:9001` | Windows 브라우저 |

WSL2의 포트는 Windows 호스트에 자동 포워딩되므로, Windows 브라우저에서 `http://localhost:5173`으로 직접 접근할 수 있습니다.

### 2.3 상태 확인

```bash
./scripts/runbook_local.sh status
```

### 2.4 종료

```bash
./scripts/runbook_local.sh down
```

---

## 3. 테스트 계정

macOS/Linux 가이드와 동일합니다.

| 이메일 | 비밀번호 | 역할 |
|--------|----------|------|
| `admin@nullus.dev` | `admin123` | Admin |
| `devops@nullus.dev` | `devops123` | DevOps |
| `developer@nullus.dev` | `developer123` | Developer |

---

## 4. 테스트 실행

### 4.1 Go 테스트

```bash
go test ./... -count=1
```

### 4.2 React 테스트 (Vitest)

```bash
cd web && npx vitest run
```

### 4.3 Playwright E2E

```bash
# Chromium 설치 (WSL2에서 headless 실행)
cd web && npx playwright install --with-deps chromium

# E2E 실행
npx playwright test --reporter=list
```

Playwright는 WSL2에서 headless 모드로 실행됩니다. headed 모드(`--headed`)는 WSLg가 설치된 Windows 11에서만 동작합니다.

### 4.4 API Smoke Test

```bash
./scripts/runbook_local.sh smoke
```

---

## 5. Windows 고유 이슈 및 해결

| 문제 | 원인 | 해결 |
|------|------|------|
| `./scripts/runbook_local.sh: Permission denied` | 실행 권한 없음 | `chmod +x scripts/runbook_local.sh` |
| `make: command not found` | make 미설치 | `sudo apt install make` |
| Docker 명령어 실패 | Docker Desktop WSL2 연동 비활성 | Docker Desktop > Settings > WSL Integration에서 Ubuntu 활성화 |
| 파일 변경 감지 안 됨 (HMR) | `/mnt/c/` 경로 사용 | 프로젝트를 `~/` (Linux 파일시스템)로 이동 |
| `localhost` 접근 불가 | WSL2 네트워크 모드 | `wsl --version`에서 WSL 2.x 확인. 구버전은 IP 주소 직접 사용: `ip addr show eth0` |
| npm install 느림 | Windows 파일시스템 I/O | 프로젝트를 `~/` 경로에 두고 실행 |
| Playwright headed 모드 안 됨 | WSLg 미지원 (Windows 10) | `--headed` 제거하고 headless로 실행 |
| `lsof: command not found` | lsof 미설치 | `sudo apt install lsof` |
| kind 클러스터 생성 실패 | Docker 리소스 부족 | Docker Desktop > Settings > Resources에서 메모리 4GB 이상 할당 |

---

## 6. VS Code 연동

VS Code에서 WSL2 내부의 프로젝트를 편집하려면:

1. VS Code에 [WSL 확장](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-wsl) 설치
2. WSL2 터미널에서 프로젝트 디렉토리로 이동 후:

```bash
code .
```

VS Code가 WSL2 모드로 열리며, 터미널도 WSL2 bash를 사용합니다.

---

## 7. 참고 문서

- [로컬 테스트 가이드 (macOS/Linux)](./Nullus_로컬_테스트_가이드.md)
- [개발자 온보딩 가이드](./Nullus_개발자_온보딩_가이드.md)
- [kind E2E 테스트 가이드](../guides/kind-e2e-testing-guide.md)
