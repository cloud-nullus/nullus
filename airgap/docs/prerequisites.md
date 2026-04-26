# 사전 요구사항
> 온라인/오프라인 머신별 환경 조건, 반입 산출물 체크리스트, 권한 및 네트워크 설정

## ⚠️ 스크립트 실행 전 공통 확인: Docker 데몬 기동

온라인/오프라인 양쪽 모두 `docker` CLI만 설치되어 있고 **데몬(docker server)이 기동되지 않은** 상태에서 `01-pull-images.sh` / `03-load-bundle.sh` / `10-setup-registry.sh` 등이 실행되면 다음과 같은 에러로 즉시 실패한다.

```
failed to connect to the docker API at unix:///.../docker.sock:
  dial unix .../docker.sock: connect: no such file or directory
```

**실행 직전 반드시 `docker info` 가 0으로 종료되는지 확인**한다.

```bash
docker info >/dev/null 2>&1 && echo "docker OK" || echo "docker DOWN"
```

### 플랫폼별 데몬 기동 방법

| 환경 | 기동 명령 | 확인 |
|------|-----------|------|
| macOS (Docker Desktop) | Docker Desktop.app 실행 또는 `open -a Docker` | 메뉴바 고래 아이콘 Running |
| macOS (Colima) | `colima start` | `colima status` |
| Linux (systemd) | `sudo systemctl start docker` | `systemctl is-active docker` |
| Linux (rootless) | `systemctl --user start docker` | `systemctl --user is-active docker` |
| Podman 전용 | `podman machine start` + 스크립트에 `PODMAN=1` | `podman info` |

Docker Desktop은 최초 기동 후 WSL2/HyperKit 초기화에 수 초~수십 초 걸리므로 `docker info` 응답을 기다린 뒤 스크립트를 실행한다.

---

## 온라인 머신 (번들 생성)

이미지 Pull과 번들 생성을 담당하는 머신이다. 인터넷 연결이 필요하다.

### 필수 도구

| 도구 | 최소 버전 | 확인 명령 |
|------|-----------|-----------|
| docker | 24.0+ | `docker version` |
| (대안) podman | 4.0+ | `podman version` |
| helm | 3.14+ | `helm version` |
| kubectl | 1.30+ | `kubectl version --client` |
| kind | 0.23+ | `kind version` |
| gzip | — | `gzip --version` |
| sha256sum / shasum | — | `sha256sum --version` (Linux) / `shasum -a 256 --version` (macOS) |

### 네트워크 접근

다음 호스트에 HTTPS(443) 접근이 가능해야 한다.

- `ghcr.io` — Nullus 앱 이미지
- `docker.io` / `registry-1.docker.io` — Bitnami 이미지, registry:2, kindest/node
- `charts.bitnami.com` — Helm 차트 의존성 업데이트 시 (번들에 이미 포함된 경우 불필요)

### 디스크 여유 공간

- 이미지 Pull 및 번들 생성: **≥ 10 GiB** 권장

---

## 오프라인 머신 (설치)

kind 클러스터를 실행하고 Nullus를 배포하는 머신이다. 외부 네트워크 없이 동작해야 한다.

### 필수 도구

| 도구 | 최소 버전 | 확인 명령 |
|------|-----------|-----------|
| docker | 24.0+ | `docker version` |
| kubectl | 1.30+ | `kubectl version --client` |
| kind | 0.23+ | `kind version` |
| helm | 3.14+ | `helm version` |
| curl | — | `curl --version` |

### 시스템 리소스

| 항목 | 최소값 | 권장값 |
|------|--------|--------|
| RAM | 4 GiB 여유 | 8 GiB 이상 |
| 디스크 | 15 GiB 여유 | 30 GiB 이상 |
| CPU | 2 코어 | 4 코어 이상 |

### 포트 가용성

kind 클러스터 생성 전에 아래 포트가 비어 있어야 한다.

| 포트 | 용도 |
|------|------|
| `5001` | 로컬 레지스트리 (kind-registry) |
| `80` | kind control-plane 노드 HTTP 매핑 |
| `443` | kind control-plane 노드 HTTPS 매핑 |

포트 충돌 확인:
```bash
# macOS
lsof -i :5001 -i :80 -i :443

# Linux
ss -tlnp | grep -E ':(5001|80|443)'
```

---

## 반입 산출물 체크리스트

오프라인 머신에 다음 파일이 모두 있는지 확인한다.

```
airgap/
├── bundle/
│   ├── images.tar.gz          # 이미지 아카이브
│   ├── images.tar.gz.sha256   # SHA-256 체크섬
│   └── MANIFEST.txt           # 이미지@digest 목록
├── helm/
│   └── nullus-*.tgz           # Helm 차트 번들 (Agent 3 생성)
├── images/
│   └── images.txt             # 이미지 목록
├── kind/
│   ├── kind-airgap.yaml
│   └── registry.yaml
└── scripts/
    ├── 03-load-bundle.sh
    ├── 10-setup-registry.sh
    ├── 11-create-cluster.sh
    ├── 12-push-to-registry.sh
    ├── 21-install-nullus.sh
    └── 99-verify.sh
```

### SHA-256 무결성 검증

반입 후 아카이브 무결성을 반드시 확인한다.

```bash
# Linux
cd airgap/bundle
sha256sum -c images.tar.gz.sha256

# macOS
cd airgap/bundle
shasum -a 256 -c images.tar.gz.sha256
```

정상 출력 예시:
```
images.tar.gz: OK
```

`FAILED` 또는 `WARNING: 1 line is improperly formatted`가 출력되면 파일이 손상된 것이므로 재반입해야 한다.

---

## 권한 설정

### Docker 데몬 접근

`docker` 명령을 `sudo` 없이 실행하려면 현재 사용자가 `docker` 그룹에 속해야 한다.

```bash
# 그룹 추가 후 재로그인 필요
sudo usermod -aG docker $USER
newgrp docker
```

### 포트 80/443 바인딩

- **macOS**: Docker Desktop이 포트 바인딩을 관리하므로 추가 설정 불필요.
- **Linux (rootful docker)**: Docker daemon이 루트로 실행되므로 추가 설정 불필요.
- **Linux (rootless docker)**: 1024 미만 포트 바인딩에 `net.ipv4.ip_unprivileged_port_start` 조정이 필요할 수 있다.

```bash
# rootless 환경에서 80/443 허용
sudo sysctl -w net.ipv4.ip_unprivileged_port_start=80
```

---

## 네트워크 / 방화벽

에어갭 환경 내에서 아래 통신 경로가 차단되어서는 안 된다.

| 출발지 | 목적지 | 프로토콜/포트 | 용도 |
|--------|--------|--------------|------|
| kind 노드 컨테이너 | `kind-registry` 컨테이너 | TCP 5000 (내부) | containerd mirror Pull |
| 호스트 | `localhost:5001` | TCP 5001 | docker push, curl 검증 |
| 호스트 | kind API server | TCP 6443 (기본) | kubectl 명령 |

kind 노드와 레지스트리 컨테이너는 같은 docker 네트워크(`kind`)에 연결된다. `11-create-cluster.sh`가 자동으로 `docker network connect kind kind-registry`를 실행한다. 호스트 방화벽(ufw, firewalld)이 활성화된 경우 docker 브리지 네트워크 트래픽을 허용해야 한다.

```bash
# ufw 사용 환경 — docker 네트워크 CIDR 확인
docker network inspect kind | grep Subnet

# 해당 CIDR 허용 예시 (172.18.0.0/16인 경우)
sudo ufw allow from 172.18.0.0/16
```

관련 문서: [../README.md](../README.md) | [architecture.md](./architecture.md) | [runbook.md](./runbook.md)
