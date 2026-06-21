# Nullus Air-Gap — Kakao Cloud 테스트 배포

OpenTofu 로 Kakao Cloud 에 VM 2대를 프로비저닝하고, 기존 `airgap/` 스크립트를 재사용하여 Nullus 플랫폼을 설치하는 테스트 배포 환경.

---

## 아키텍처

```
[인터넷]
    │
    ▼
┌─────────────┐   SCP 번들 전송     ┌──────────────────┐
│ builder VM  │ ─────────────── ▶ │   airgap VM      │
│ (온라인)      │                   │ (오프라인 타겟)     │
│ t1i.xlarge  │                   │ t1i.2xlarge      │
│             │                   │                  │
│ · git clone │                   │ · registry:2     │
│ · pre-build │                   │ · kind 클러스터    │
│   amd64 빌드 │                   │  (cp1 + worker1) │
└─────────────┘                   │ · Nullus 설치     │
                                  └──────────────────┘
```

**중요: amd64 전용**
- Kakao Cloud `t1i.*` 플레이버는 **x86_64/amd64** 전용.
- 로컬 개발 머신(Apple Silicon 등)에서 생성된 `airgap/dist/` 번들은 **arm64** 이므로 사용 불가.
- builder VM 이 온라인에서 `TARGET_PLATFORM=linux/amd64` 로 번들을 직접 빌드함.

---

## 사전 요건

| 항목 | 버전/비고 |
|------|----------|
| OpenTofu | >= 1.13.5 (`tofu` 커맨드) |
| Kakao Cloud 계정 | Application Credential (ID + Secret) |
| Kakao Cloud 키페어 | 콘솔 생성 불필요 — IaC 가 `key_name` 으로 생성하고 private key 를 `opentofu/<key_name>.pem` 에 저장 |
| 로컬 SSH 클라이언트 | `ssh`, `scp` 필요 |

---

## 단계별 실행

### 1. tfvars 설정

```bash
cd deploy/csp/kakaocloud/opentofu
cp terraform.tfvars.example terraform.tfvars
# terraform.tfvars 편집 — credential, key_name, ssh_key_path 필수 입력
```

> **VPC 이름/CIDR 변경 시**: `terraform.tfvars` 의 `vpc_name`/`vpc_cidr`/`vpc_default_subnet_cidr` 값을 수정하면 됨.

### 2. VM 프로비저닝

```bash
cd deploy/csp/kakaocloud
./scripts/00-provision.sh
# 자동 승인: ./scripts/00-provision.sh -auto-approve
```

`tofu apply` 완료 후 출력되는 IP 주소를 환경 변수로 내보냄:

```bash
export BUILDER_IP=<builder_public_ip>
export AIRGAP_IP=<airgap_public_ip>
export SSH_KEY=~/.ssh/my-keypair.pem
```

### 3. Builder VM 에서 amd64 번들 빌드

```bash
./scripts/10-build-on-builder.sh
```

- builder VM 에 저장소 클론 후 `airgap/pre-build.sh` 실행
- 결과물: `~/draft/airgap/dist/nullus-airgap-bundle-<date>.tar.gz`
- cloud-init 완료(최대 5분) 자동 대기

### 4. 번들 Airgap VM 으로 전송

```bash
./scripts/20-transfer-bundle.sh
```

- 경로: Builder VM → 로컬 임시 디렉토리 → Airgap VM
- sha256 체크섬 자동 검증

### 5. Airgap VM 에 설치

```bash
./scripts/30-install-on-airgap.sh
```

- `airgap/install.sh` 를 재사용하여 설치 수행
- 선택적 환경 변수: `CLUSTER_NAME`, `SKIP_VERIFY`, `PLATFORM_OVR`

---

## 필요 포트 (보안 그룹)

| 포트 | 프로토콜 | 용도 |
|------|---------|------|
| 22 | TCP | SSH 접속 |
| 80 | TCP | HTTP |
| 443 | TCP | HTTPS |
| VPC 전체 | ALL | VM 간 내부 통신 |
| 전체 egress | ALL | 인터넷 접근 (builder 빌드용) |

> registry(5001)·kind API Server(16443)는 airgap VM 내부에서 `127.0.0.1`로만 바인딩되므로 공개 ingress를 두지 않는다. 외부에서 접근이 필요하면 SSH 터널을 쓴다:
> ```bash
> ssh -i <key> -L 16443:127.0.0.1:16443 -L 5001:127.0.0.1:5001 ubuntu@<airgap_ip>
> ```

---

## 정리

```bash
./scripts/00-provision.sh -destroy
```

---

## 참고

- 설치 로직의 단일 진실 공급원: [`airgap/install.sh`](../../../../airgap/install.sh)
- 번들 빌드 로직: [`airgap/pre-build.sh`](../../../../airgap/pre-build.sh)
- 10단계 설치 흐름: [`airgap/INSTALL.md`](../../../../airgap/INSTALL.md)
