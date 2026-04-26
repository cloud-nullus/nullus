# Nullus Platform — Air-Gap 배포 가이드
> 인터넷 없는 폐쇄망 환경에서 kind 클러스터로 Nullus Platform을 구동하는 절차

## 개요

Nullus Platform의 에어갭(air-gap) 배포 도구 모음이다. 인터넷이 연결된 온라인 머신에서 이미지와 Helm 차트를 번들로 묶은 뒤, 오프라인 머신으로 반입하여 kind 클러스터에 설치한다. 컨테이너 이미지는 `registry:2` 기반 로컬 레지스트리에 보관되며, containerd 미러 설정을 통해 클러스터 노드가 외부 레지스트리 없이 이미지를 Pull한다. 개발 및 기능 검증 목적의 단일 호스트 kind 환경을 대상으로 한다.

---

## 빠른 시작

### 온라인 머신 (번들 생성)

```bash
# 1. 이미지 Pull
bash airgap/scripts/01-pull-images.sh

# 2. 이미지 번들 저장 (bundle/images.tar.gz + MANIFEST.txt + .sha256)
bash airgap/scripts/02-save-bundle.sh
```

생성된 `airgap/bundle/` 디렉토리와 Helm 차트(`deploy/helm/nullus/`)를 저장 매체에 복사한 뒤 오프라인 머신으로 반입한다.

### 오프라인 머신 (설치)

```bash
# 3. 번들 로드 (docker load)
# Agent 2가 작성하는 스크립트: airgap/scripts/03-load-bundle.sh

# 4. 로컬 레지스트리 기동 (registry:2 @ localhost:5001)
bash airgap/scripts/10-setup-registry.sh

# 5. kind 클러스터 생성 + containerd 미러 설정
bash airgap/scripts/11-create-cluster.sh

# 6. 이미지 리태그 & 레지스트리 푸시
bash airgap/scripts/12-push-to-registry.sh

# 7. Helm으로 Nullus 설치
# Agent 3이 작성하는 스크립트: airgap/scripts/21-install-nullus.sh

# 8. 환경 검증
bash airgap/scripts/99-verify.sh
```

---

## 디렉토리 구조

```
airgap/
├── README.md                   # 이 문서 — 랜딩 페이지
├── bundle/                     # 번들 산출물 (생성 후 반입)
│   ├── images.tar.gz           # docker save 이미지 아카이브
│   ├── images.tar.gz.sha256    # SHA-256 체크섬
│   └── MANIFEST.txt            # 이미지@digest 목록
├── docs/                       # 상세 문서
│   ├── architecture.md         # 데이터 흐름 및 컴포넌트 설계
│   ├── prerequisites.md        # 머신별 사전 요구사항 체크리스트
│   ├── runbook.md              # 단계별 운영 런북
│   └── troubleshooting.md      # 증상별 문제 해결 가이드
├── helm/                       # Helm 차트 번들 (Agent 3 소유)
│   └── README.md               # Helm 번들 사용법
├── images/
│   ├── images.txt              # Pull/번들 대상 이미지 목록
│   └── README.md               # 이미지 목록 갱신 방법 (Agent 1 소유)
├── kind/
│   ├── kind-airgap.yaml        # kind 클러스터 설정 (containerd 미러 포함)
│   └── registry.yaml           # kube-public/local-registry-hosting ConfigMap
└── scripts/
    ├── 01-pull-images.sh       # [온라인] 이미지 Pull
    ├── 02-save-bundle.sh       # [온라인] 번들 저장 + SHA-256
    ├── 03-load-bundle.sh       # [오프라인] 번들 로드 (Agent 2 작성)
    ├── 10-setup-registry.sh    # [오프라인] registry:2 기동
    ├── 11-create-cluster.sh    # [오프라인] kind 클러스터 생성
    ├── 12-push-to-registry.sh  # [오프라인] 이미지 리태그 & 푸시
    ├── 21-install-nullus.sh    # [오프라인] Helm 설치 (Agent 3 작성)
    └── 99-verify.sh            # [오프라인] 환경 검증
```

---

## 요구 사항 요약

- **⚠️ 공통 전제**: 스크립트 실행 전 **Docker 데몬(server)이 기동되어 있어야 한다**. `docker info` 가 0으로 종료되는지 반드시 확인. 꺼진 상태에서는 모든 단계가 `dial unix docker.sock: no such file or directory`로 실패한다.
  - macOS: Docker Desktop.app 실행 또는 `colima start`
  - Linux(systemd): `sudo systemctl start docker`
  - Podman 전용: `podman machine start` + 스크립트에 `PODMAN=1`
- **온라인 머신**: docker ≥ 24 (또는 podman), helm ≥ 3.14, ghcr.io/docker.io 접근 가능, 여유 디스크 ≥ 10 GiB.
- **오프라인 머신**: docker, kubectl ≥ 1.30, kind ≥ 0.23, helm ≥ 3.14, 메모리 ≥ 4 GiB, 디스크 ≥ 15 GiB, 포트 80/443/5001 미사용.
- 자세한 사항은 [docs/prerequisites.md](docs/prerequisites.md) 참고.

---

## 주요 환경 변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `CLUSTER_NAME` | `nullus-airgap` | kind 클러스터 이름 |
| `RELEASE` | `nullus` | Helm 릴리스 이름 |
| `NAMESPACE` | `nullus` | Kubernetes 네임스페이스 |
| `DRY_RUN` | `0` | `1`로 설정 시 명령만 출력하고 실행하지 않음 |
| `SKIP_LOAD` | `0` | `1`로 설정 시 `docker load` 단계를 건너뜀 (이미 로드된 경우) |
| `SKIP_VERIFY` | `0` | `1`로 설정 시 `99-verify.sh` 자동 실행을 건너뜀 |
| `PODMAN` | `0` | `1`로 설정 시 docker 대신 podman 사용 (01-pull-images.sh) |

---

## 문서 목차

| 문서 | 내용 |
|------|------|
| [docs/architecture.md](docs/architecture.md) | 데이터 흐름 다이어그램, 컴포넌트 표, 이미지 리태그 규칙, 실패 지점 |
| [docs/prerequisites.md](docs/prerequisites.md) | 온라인/오프라인 머신 사전 요구사항, 반입 체크리스트, 권한 설정 |
| [docs/runbook.md](docs/runbook.md) | 단계별 절차, 재설치/업그레이드/클린업 시나리오 |
| [docs/troubleshooting.md](docs/troubleshooting.md) | 증상별 원인 및 해결, 로그 수집 명령 |
| [images/README.md](images/README.md) | 이미지 목록(`images.txt`) 갱신 방법 |
| [helm/README.md](helm/README.md) | Helm 차트 번들 사용법 및 values-airgap.yaml 설명 |

---

## 범위 주의사항

이 도구 모음은 **개발 및 기능 검증 목적의 단일 호스트 kind 클러스터** 전용이다. Ingress Controller, TLS 인증서(cert-manager), 모니터링 스택(Prometheus/Grafana), 고가용성 구성은 본 번들에 포함되지 않으며 프로덕션 환경에 그대로 적용해서는 안 된다. 프로덕션 에어갭 배포는 별도의 엔터프라이즈 가이드를 따를 것.
