# Kakao Cloud Air-Gap 테스트 배포 — 구축 및 트러블슈팅 기록

> 작성일: 2026-06-09 · 대상: `deploy/csp/kakaocloud/`
> Nullus Platform 을 카카오 클라우드 VM 2대(builder + airgap)에 OpenTofu 로 프로비저닝하고,
> 기존 `airgap/` kind 기반 설치 자산을 재사용해 배포하는 구성의 구축 전 과정 기록.

---

## 1. 개요

### 목표
- 카카오 클라우드에 **VM 2대**를 OpenTofu(OpenTofu/Terraform 호환)로 생성
- 기존 `airgap/` 의 **kind 기반 에어갭 설치 흐름**을 재사용 (k8s 설치 로직을 새로 짜지 않음)
- OpenTofu 는 **인프라 프로비저닝까지만**, 이후는 **별도 스크립트가 SSH 로 호출**

### 2-VM 토폴로지 (builder + airgap 분리)
| VM | 역할 | 스펙 | 비고 |
|----|------|------|------|
| **builder** | 온라인 — amd64 번들 빌드 | t1i.xlarge (4c/8GB), 500GB | 인터넷에서 이미지 pull → `pre-build.sh` 로 번들 생성 |
| **airgap** | 오프라인 — kind 클러스터 설치 대상 | t1i.2xlarge (8c/16GB), 500GB | 번들 수신 → `install.sh` → registry + kind + Nullus |

> kind 는 단일 호스트의 Docker 안에 클러스터를 띄우는 도구다. airgap 설치(`install.sh`)는
> **registry:2 컨테이너 + kind 클러스터(control-plane1 + worker1) + Nullus** 를 한 VM 안에서 모두 구성한다.
> builder/airgap 분리는 airgap 의 online(빌드)/offline(설치) 구조를 그대로 2-VM 에 매핑한 것으로,
> 실제 에어갭 전송 경로(SCP)까지 검증한다.

### 핵심 ARCH 결정
- 카카오 `t1i.*` flavor 는 **x86_64/amd64**.
- 로컬 Mac 은 arm64 → 기존 `airgap/dist/` 번들(arm64)은 **카카오 VM 에서 사용 불가**.
- → **builder VM(네이티브 amd64)에서 번들을 새로 빌드**. 로컬 arm 번들 미사용.

---

## 2. 결과물 (현재 상태)

### 인프라 (OpenTofu, `apply` 완료)
| 리소스 | 값 |
|--------|-----|
| VPC | `nullus-airgap` `172.16.0.0/16` (id `78eac5ed-…`) |
| Subnet | `nullus-subnet` `172.16.0.0/24` AZ `kr-central-2-a` (id `66a5fff5-…`) |
| Security Group | `nullus-airgap-sg` (id `5c289a80-…`) — ingress 22/80/443 + VPC 내부 ALL + ICMP, egress ALL |
| Keypair | `nullus-airgap-key` (TF 생성, private key → `opentofu/nullus-airgap-key.pem`, 0600) |
| builder VM | public `<BUILDER_PUBLIC_IP>` / private `172.16.0.254` |
| airgap VM | public `<AIRGAP_PUBLIC_IP>` / private `172.16.0.133` |
| Provider | `kakaoenterprise/kakaocloud 0.3.5` (registry, lock 3-platform) |

접속: `ssh -i ./nullus-airgap-key.pem ubuntu@<public-ip>` (opentofu/ 디렉토리 기준 상대경로)

### amd64 번들 (builder 에서 빌드 완료)
- `~/draft/airgap/dist/nullus-airgap-bundle-2026-06-09.tar.gz` (**6.2G**, sha256 포함)
- **71개 이미지 전부 amd64/linux** 검증 완료 (nullus-api/web 포함)
- 번들 bin/ = **linux-amd64 단독**

### 배포 완료 ✅ (airgap VM)
- **번들 전송**: builder → airgap **내부망 직접 SCP** (172.16.0.133, 6.2G ~20초, sha256 OK)
  — 키페어를 builder `~/.ssh/airgap.pem` 으로 복사해 VPC 내부 전송 (20-transfer-bundle.sh 의 Mac 경유 방식보다 빠름)
- **설치**: airgap VM 에서 `install.sh` 실행 (총 **21분 17초**, 8단계 PASS)
- **kind 클러스터** `nullus-airgap` (runtime arch **x86_64/amd64**):
  ```
  nullus-airgap-control-plane   Ready   control-plane   v1.30.0
  nullus-airgap-worker          Ready   <none>          v1.30.0
  ```
- **Nullus 파드 전부 Running**:
  - `nullus` ns: nullus-api ×2, nullus-web ×2, nullus-postgresql
  - `nullus-auth` ns: keycloak, keycloak-postgresql
- **외부 접근 (영구)**: `http://<AIRGAP_PUBLIC_IP>/` — airgap VM 공인 IP, **HTTP 200 확인**
  - gateway/ingress 미설치 → `nullus-web`(ClusterIP) 을 `kubectl port-forward --address 0.0.0.0:80` 으로 노출
  - **systemd 서비스 `nullus-expose`** (enabled+active, Restart=always) 로 영구화 — 재부팅/프로세스 종료 후 자동 복구
  - 접근 제한: 보안그룹 80 을 **운영자 IP `218.235.200.120/32`** 로 제한 (`web_allowed_cidr`)

---

## 3. 디렉토리 구조

```
deploy/csp/kakaocloud/
├── README.md
├── docs/
│   └── BUILD-LOG.md                  # (이 문서)
├── opentofu/
│   ├── provider.tf                   # kakaocloud 0.3.5 + local + null
│   ├── main.tf                       # network → security → keypair → compute
│   ├── variables.tf
│   ├── terraform.tfvars(.example)
│   ├── outputs.tf                    # IP, ssh 명령, next_steps (상대경로)
│   ├── cloud-init.yaml               # docker + git + make (평문비번 없음, 키페어 기반)
│   ├── .gitignore                    # tfstate/tfvars/pem 제외, lock.hcl 은 커밋
│   ├── .terraform.lock.hcl           # 3-platform 해시 고정 (커밋 대상)
│   └── modules/{network,security,compute}/{provider,main,variables,outputs}.tf
└── scripts/
    ├── 00-provision.sh               # tofu init/apply 래퍼
    ├── 10-build-on-builder.sh        # builder SSH: clone → pre-build.sh (amd64)
    ├── 20-transfer-bundle.sh         # builder → airgap SCP
    ├── 30-install-on-airgap.sh       # airgap SSH: install.sh
    └── 40-expose-service.sh          # airgap SSH: systemd port-forward 영구 노출
```

설계 원칙:
- network 모듈은 tfvars 의 `vpc_*` / `subnet_*` 변수로 VPC·서브넷을 구성한다.
- registry(5001)·kind API(16443)는 VM 내부 127.0.0.1 바인딩 → **공개 ingress 미설정** (접근은 SSH 터널).
- 키페어는 TF 가 생성하고 private key 를 로컬에 저장 (계정 미등록 키페어 대응).

---

## 4. 구축 절차 (실제 수행 순서)

1. **참고 코드 분석** — `k-paas/csp/kakao-cloud/terraform` 의 provider/모듈 패턴 차용 (compute data-source lookup, instance/public_ip 리소스 형태)
2. **OpenTofu 스캐폴딩** — network/security/compute 3모듈 + 스크립트 (loadbalancer/heavy-provisioner 제외)
3. **검증** — `terraform validate` (kakaocloud 실제 스키마 대비) Success
4. **provider 0.2.0 → 0.3.5** 갱신, dev_overrides 제거, registry 기반 init + 3-platform lock
5. **apply** — 1차: VPC/subnet/SG 생성됐으나 인스턴스 실패(키페어 미등록) → keypair 리소스 추가 후 2차 apply 성공
6. **SSH/cloud-init 검증** — 두 VM 접속 + Docker 29.5.3 확인
7. **amd64 번들 빌드** (builder VM) — 8건의 블로커 해결(§5) 후 6.2G amd64 번들 생성
8. **arch 검증** — 71 이미지 amd64 확인

---

## 5. 트러블슈팅 전체 기록 (8건)

### TS-1. 로컬 arm64 번들이 카카오 x86 VM 에 부적합
- **증상**: 기존 `airgap/dist/` 번들 내 이미지가 arm64/linux.
- **원인**: `01-pull-images.sh` 의 `TARGET_PLATFORM=linux/$(host arch)` 기본값 → arm Mac 에서 arm64 로 굳음.
- **해결**: builder VM(네이티브 amd64)에서 `TARGET_PLATFORM=linux/amd64 PLATFORMS=linux-amd64` 로 재빌드.

### TS-2. apply 시 인스턴스 생성 실패 — `Invalid key_name`
- **증상**: VPC/subnet/SG 는 생성, 인스턴스 2개 `400 Bad Request: Invalid key_name provided`.
- **원인**: tfvars 의 `KPAAS_KEYPAIR` 가 이 카카오 계정에 미등록(다른 계정 키페어).
- **해결**: `kakaocloud_keypair` 리소스로 **TF 가 신규 키페어 생성** + `local_sensitive_file` 로 private key 저장. compute 가 이 키페어에 의존하도록 배선. 재apply 로 인스턴스 생성.

### TS-3. crane pull finalize hang
- **증상**: 로컬 크로스빌드 시 `crane pull --platform linux/amd64 ghcr.io/cloud-nullus/draft/nullus-api` 가 39.8MB 중 39MB 받고 **무한 대기** (1차 빌드 ~20시간 정체).
- **원인**: crane 0.21.6 이 이 ghcr 이미지에서 finalize 단계 hang. crane 이 exit 하지 않아 스크립트의 docker 폴백도 안 걸림.
- **해결**: 빌드를 builder VM(네이티브 amd64)으로 이전 + `USE_CRANE=0` (네이티브 `docker pull` 사용, crane 우회).

### TS-4. docker 29 containerd 이미지 스토어 — save/load 멀티아치 손상 위험
- **증상**: builder VM docker 29.5.3 의 기본 스토어가 `io.containerd.snapshotter.v1`. `USE_CRANE=0` + 단일플랫폼 save/load 시 멀티아치 blob 유실 → 설치 push 가 `does not provide any platform` 으로 깨질 수 있음.
- **원인**: docker 29 가 containerd 스냅샷터를 기본 사용.
- **해결**: `10-build-on-builder.sh` 가 빌드 전 `/etc/docker/daemon.json` 에 `{"features":{"containerd-snapshotter":false}}` 설정 + docker 재시작 → **classic overlay2 스토어** 로 전환. 네이티브 amd64 `docker pull` 이 단일아치로 저장돼 save/load 안전.

### TS-5. ghcr 인증 — 조직 패키지 권한
- **증상**: `nullus-api:main`, `nullus-web:main` pull `denied`. (`docker login` "Login Succeeded" 떠도 denied — 로그인 성공 ≠ 패키지 읽기 권한.)
- **원인**: builder 에 쓴 PAT(`dasomel`)가 본인 패키지(`dasomel/*`)는 읽지만 **cloud-nullus 조직 소유 사설 패키지는 권한 없음**. (rate limit 아님 — goharbor 는 OK, nullus 만 denied 로 격리 확인.)
- **해결**: Mac keychain(credStore=desktop)에 저장된 **권한 있는 PAT 를 추출해 builder 에 주입** (secret 은 stdin 파이프로만 전달, 미출력). 검증: `docker manifest inspect nullus-api` → OK.

### TS-6. digest-pinned 이미지 save 누락
- **증상**: 71/71 pull 성공인데 save 단계에서 `ingress-nginx/controller:v1.11.2` `not found locally. Aborting`.
- **원인**: images.txt 항목이 `...controller:v1.11.2@sha256:…` (digest 고정). `docker pull` 은 digest ref(`@sha256`)로만 저장 → save 가 `:v1.11.2` 태그로 조회해 불일치. crane 경로(USE_CRANE=1)는 digest→tag 재태깅을 하지만 **USE_CRANE=0 경로엔 그 재태깅이 없음**.
- **해결**: 빌더에서 해당 이미지 ID 를 `:v1.11.2` 로 수동 재태깅 후 재실행.

### TS-7. helm 미설치
- **증상**: `2/5 Helm 차트 번들` 단계에서 `helm 이 설치되어 있지 않습니다`.
- **원인**: cloud-init 이 docker/git/make 만 설치, helm 누락. 차트 단계는 바이너리 다운로드 단계보다 먼저라 helm 이 PATH 에 선설치돼 있어야 함.
- **해결**: builder 에 `get-helm-3` 로 helm 설치 (v3.21.0). 이미지 save 는 이미 끝났으므로 `SKIP_IMAGES=1` 로 차트→바이너리→패키징만 재실행.

### TS-8. 카탈로그 차트 — helm repo add 일시 타임아웃
- **증상**: `3/5 카탈로그 chart 다운로드` 에서 `helm repo add argo https://argoproj.github.io/argo-helm` 이 `context deadline exceeded` 로 abort (set -e).
- **원인**: 카카오 VM → github.io 일시 네트워크 타임아웃 (재확인 시 200, 0.078s 정상). 차트 다운로드 스크립트에 재시도 없음.
- **해결**: 재실행으로 통과 (catalog 14개+ 정상 다운로드).

### 설치 단계 (airgap install.sh) — 블로커 없음
- 번들 빌드와 달리 **설치는 1회에 PASS** (8단계, 21분 17초).
- 번들이 `bin/linux-amd64`(kind v0.31 / kubectl v1.30 / helm v3.16)를 자체 포함 → airgap VM 에 helm 등 사전설치 불필요.
- airgap VM 도 docker 29 containerd 스토어지만, 번들 이미지가 **단일아치 amd64** 라 `docker load` + registry push 정상 (멀티아치 손상 이슈는 빌드 측에서 이미 해소됨).
- 참고: fresh SSH 세션에서 `kubectl` 은 PATH 에 없음 → 번들 bin 또는 `25-port-forward.sh` 사용.

---

## 6. 권장 영구 수정 (repo 패치 — 재발 방지)

> 이번에 builder/현장에서 임시 처리한 항목들을 repo 에 반영하면 다음 빌드/CI/신규 VM 에서 재발하지 않는다.

| # | 파일 | 수정 내용 |
|---|------|-----------|
| P1 | `airgap/scripts/01-pull-images.sh` | `USE_CRANE=0` 경로에도 **digest-pinned 이미지 → :tag 재태깅** 추가 (crane 경로의 `desired="${image%@*}"; docker tag` 로직 동일 적용) → TS-6 근본 해결 |
| P2 | `deploy/csp/kakaocloud/opentofu/cloud-init.yaml` | builder VM 에 **helm/kubectl/kind 설치** 추가 → TS-7 근본 해결 |
| P3 | `airgap/scripts/pre/pull-charts-catalog.sh` | `helm repo add` 에 **재시도(backoff)** 로직 → TS-8 완화 |
| P4 | (반영됨) `10-build-on-builder.sh` | `USE_CRANE=0` + containerd→classic 스토어 전환 + ghcr/git 인증 — 이미 적용됨 |

---

## 6-2. 외부 노출 & 보안그룹 강화 (추가 구성)

설치 후 외부 접근을 **영구화 + IP 제한** 하고 IaC 로 반영했다.

### 영구 노출 (systemd) — `scripts/40-expose-service.sh`
- kind 는 80/443 을 호스트에 매핑하지 않으므로(127.0.0.1 바인딩), `nullus-web`(ClusterIP)을
  `kubectl port-forward --address 0.0.0.0:80` 으로 노출.
- 일회성 port-forward 는 재부팅/종료 시 끊김 → **systemd 유닛 `nullus-expose`** 로 고정
  (`Restart=always`, `ExecStartPre` 로 kind apiserver 준비 대기).
- kubectl 을 `/usr/local/bin` 으로 설치해 유닛이 안정 경로 참조.
- 결과: `http://<AIRGAP_PUBLIC_IP>/` → HTTP 200, `systemctl is-enabled/is-active` = enabled/active.

### 보안그룹 IP 제한 — `web_allowed_cidr` 변수 (Terraform)
- security 모듈에 `web_allowed_cidr` 변수 추가 → 80 포트 ingress 의 `remote_ip_prefix` 에 적용.
- tfvars 에서 운영자 IP 로 제한: `web_allowed_cidr = "218.235.200.120/32"`.
- 기본값 `0.0.0.0/0` (미설정 시 전체 공개). 22/443 은 변경 없음.

### ⚠️ 적용 시 주의 — 인스턴스 강제 교체 방지 (`lifecycle ignore_changes`)
- **증상**: SG 변경만 의도했는데 `terraform plan` 이 **인스턴스 2개를 must be replaced** 로 표시.
- **원인**: compute 모듈이 `kakaocloud_images`/`kakaocloud_instance_flavors` data-source 로 image_id/flavor_id 를
  이름조회 → 재apply 시 `(known after apply)` 로 churn → `image_id forces replacement` → 기존 VM(+클러스터) 파괴 위험.
- **해결**: 두 인스턴스에 `lifecycle { ignore_changes = [image_id, flavor_id] }` 추가.
  → 재plan 결과 `0 to add, 1 to change(SG in-place), 0 to destroy` 로 안전. **운영 중 재apply 필수 안전장치.**
- (이미지/플레이버를 의도적으로 교체하려면 해당 ignore_changes 를 일시 제거)

---

## 6-3. 클러스터 등록 (Cluster Management)

신규 설치 시 등록된 클러스터는 **0개가 정상**이다 — 마이그레이션 `000044_remove_seeded_sample_stack_and_cluster_data` 가 샘플 stack/cluster 를 의도적으로 제거("fresh local environments start clean"). 직접 등록해야 스택/파이프라인 배포 대상이 생긴다.

### 클러스터 모델 (역할 = type)
- `pipeline` — CI/CD 파이프라인 실행 클러스터
- `target` — 워크로드/스택 배포 대상 클러스터 (DataPlane)
- `types: []` 배열로 **한 클러스터가 두 역할 겸용 가능** (`["pipeline","target"]`).
- 이 airgap 환경은 kind 클러스터 **1개**(`nullus-airgap`, control-plane+worker 2노드)뿐이므로 **1개를 pipeline+target 겸용**으로 등록한다. (docker ps 의 control-plane/worker 컨테이너 2개는 1클러스터의 2노드 — 클러스터 2개가 아님.)

### ⚠️ 핵심 — in-cluster endpoint
nullus-api 는 kind 클러스터 **내부 파드**로 돌고, serviceAccount 토큰 automount=false → **등록된 kubeconfig 로** 대상 클러스터에 접속한다. `kind get kubeconfig` 의 server 는 `https://127.0.0.1:16443` 이라 **파드 내부에선 자기 자신**이라 도달 불가.
→ server 를 **`https://kubernetes.default.svc`** 로 바꿔야 한다 (apiserver 인증서 SAN 에 `kubernetes.default.svc` / `10.96.0.1` 포함 확인됨, client-cert 인증은 그대로 유효).

### 등록 kubeconfig 생성
```bash
# airgap VM 에서 (server 를 in-cluster 주소로 재작성)
kind get kubeconfig --name nullus-airgap \
  | sed 's#https://127.0.0.1:[0-9]*#https://kubernetes.default.svc#'
# → deploy/csp/kakaocloud/kind-incluster.kubeconfig (민감정보, *.kubeconfig gitignore)
```

### 등록 절차 (UI — Cluster Management 권장)
1. admin 로그인 → 좌측 **관리(Admin) → 클러스터 관리** (`/admin/clusters`)
2. **클러스터 등록** 클릭
3. 입력:
   - 이름: `nullus-airgap`
   - 타입: **pipeline + target 둘 다 선택**
   - Cloud Provider: `on_premise`
   - Endpoint: `https://kubernetes.default.svc`
   - Kubeconfig: 위 `kind-incluster.kubeconfig` 내용 붙여넣기 (server 가 `kubernetes.default.svc` 인 것)
4. 등록 후 **Verify Connection** → `connected` + K8s 버전(v1.30.0) 표시 확인

### 등록 절차 (API 대안)
```bash
KCFG_B64=$(base64 < deploy/csp/kakaocloud/kind-incluster.kubeconfig | tr -d '\n')
ORG_ID=$(curl -s -H 'X-User-ID: 1' -H 'X-User-Email: admin@nullus.dev' -H 'X-User-Role: admin' \
  -H 'X-User-OrgID: <org>' http://<AIRGAP_PUBLIC_IP>/api/v1/admin/organization | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')
curl -s -X POST http://<AIRGAP_PUBLIC_IP>/api/v1/admin/clusters \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 1' -H 'X-User-Email: admin@nullus.dev' -H 'X-User-Name: Admin User' \
  -H 'X-User-Role: admin' -H "X-User-OrgID: $ORG_ID" \
  -d "{\"name\":\"nullus-airgap\",\"type\":\"target\",\"types\":[\"pipeline\",\"target\"],\"cloud_provider\":\"on_premise\",\"endpoint\":\"https://kubernetes.default.svc\",\"org_id\":\"$ORG_ID\",\"kubeconfig\":\"$KCFG_B64\"}"
# 이후: POST /api/v1/admin/clusters/<id>/verify → {"status":"connected",...}
```

> 유효 enum: ClusterType=`pipeline|target`, CloudProvider=`aws|azure|gcp|oci|ibm_cloud|alibaba_cloud|tencent_cloud|naver_cloud|kt_cloud|nhn_cloud|on_premise`.

---

## 6-4. 운영 중 발견 — 로그인 후 튕김(401) & ghcr 이미지 경로 (repo 리네임)

설치·로그인 후 **로그인 화면으로 즉시 튕기는** 증상 발생. 근본 원인은 빌드 단계가 아니라 **repo 리네임에 따른 ghcr 이미지 경로 분리**였다.

### 증상 → 원인 추적
- mock 로그인(client-side)은 성공하나, 대시보드 `/api/v1/*` 호출이 **401** → axios 인터셉터 `logout()` → 로그인으로 튕김.
- 배포된 `nullus-web` 번들에 **`X-User-*` 세션 헤더 전송 코드가 없음** (구버전).
- 소스 `web/src/lib/api.ts` 에는 수정(`661e458`, 2026-05-31)이 있는데 배포 이미지엔 없음.
- **진짜 원인**: repo 가 `cloud-nullus/draft` → `cloud-nullus/nullus` 로 **리네임**되며 CD(`cd.yml`, `${{ github.repository }}`)는 새 패키지 경로 `ghcr.io/cloud-nullus/nullus/*` 로 푸시. 그러나 airgap `images.txt` 는 **옛 경로 `cloud-nullus/draft/*`** 참조 → 리네임 직전(2026-05-25, `07cecae`)에 동결된 구버전 이미지를 받아 설치.
  - 옛 경로 `draft/nullus-web:main` = 5/25(07cecae, 수정 前)
  - 새 경로 `nullus/nullus-web:main` = 6/07(5976e39, 수정 後) ← CI 정상 게시 중
- **CI 자체는 정상.** airgap 설정이 옛 경로를 가리킨 게 문제.

### 핫픽스 (running 클러스터 — 적용 완료 ✅)
airgap VM containerd 스토어에서 `docker pull/push` 는 멀티아치 "does not provide any platform" 으로 실패 → **crane** 으로 단일 플랫폼 복사:
```bash
# airgap VM (ghcr 로그인 필요, read:packages + org 패키지 권한 PAT)
crane copy --insecure --platform linux/amd64 \
  ghcr.io/cloud-nullus/nullus/nullus-web:main \
  localhost:5001/cloud-nullus/nullus/nullus-web:main
kubectl set image deploy/nullus-web web=localhost:5001/cloud-nullus/nullus/nullus-web:main -n nullus
kubectl rollout status deploy/nullus-web -n nullus
```
→ 서빙 번들에 `X-User-*` 코드 확인 → **로그인 정상화** (`http://<AIRGAP_PUBLIC_IP>/` admin@nullus.dev/admin123).
> nullus-api 도 옛 경로(07cecae)지만 백엔드는 X-User 를 *받는* 쪽이라 로그인엔 영향 없음(미교체).

### 영구 수정 (durable — 미머지 상태)
airgap 이미지 경로 `draft/` → `nullus/` 교정을 `fix/airgap-ghcr-image-path` 브랜치로 작성(커밋 `9487d2f`, 소스 6파일: images.txt / 00-generate-images.sh / values-airgap.yaml / docs 3종).
- PR #78 생성 후 **닫음** (airgap 정리 미완 — bundle/MANIFEST 재생성·nullus-api 등 추가 필요, main ruleset 이 셀프-승인 불가로 머지도 차단).
- **상태**: 브랜치 로컬 보존, **미머지**. 추후 완전 정리 후 재PR 예정.
- 잔여 정리 항목: `airgap/bundle/MANIFEST.txt`·`airgap/dist/*.tar.gz`(빌드 산출물) 재생성, nullus-api 새 경로 반영, 번들 재빌드.

---

## 6-5. 클러스터 등록 — 완료 기록

§6-3 절차로 **등록 완료 (connected)**:
```
name: nullus-airgap | id: 1a0a55d1-a2d3-4811-9ab4-0fe0a0f454f7
types: [pipeline, target]  (올인원 — 단일 kind 가 CI+배포+플랫폼 겸용)
cloud_provider: on_premise | endpoint: https://kubernetes.default.svc
connection_status: connected | node_architectures: [amd64] | org: 22222222… (Acme Corp)
```
- 등록 방식: API(POST /api/v1/admin/clusters) — UI 붙여넣기 실패(127.0.0.1 kubeconfig) 후 in-cluster kubeconfig 로 등록.
- **pipeline/target 의미**: pipeline=CI/CD 도구 실행 클러스터, target=앱(스택) 배포 런타임 클러스터. 포털(web/api/keycloak)이 도는 클러스터는 "플랫폼 클러스터"로 별개 개념 — 단일 환경이라 한 kind 가 3역할 겸함.

---

## 7. 운영 메모

- **과금 주의**: VM 2대(t1i.xlarge + t1i.2xlarge, 500GB×2)가 가동 중. 테스트 종료 시 `cd opentofu && terraform destroy`.
- **SSH 터널** (registry/kind API 는 미공개):
  ```bash
  ssh -i ./nullus-airgap-key.pem -L 16443:127.0.0.1:16443 -L 5001:127.0.0.1:5001 ubuntu@<AIRGAP_PUBLIC_IP>
  ```
- **ghcr PAT**: 대화 중 노출된 PAT 는 폐기(revoke) 권장. builder 의 ghcr 로그인은 `~/.docker/config.json`(평문 base64)에 저장됨 — 테스트 종료 시 `docker logout ghcr.io` 고려.
- **dev_overrides**: 로컬 `~/.terraformrc` 의 kakaocloud dev_override 는 비활성화함(백업 `~/.terraformrc.bak.*`). 로컬 빌드로 되돌리려면 주석 해제.

---

## 8. 명령 레퍼런스 (다음 단계)

```bash
cd deploy/csp/kakaocloud/opentofu

# 출력 확인
terraform output

# (다음) 번들 전송: builder → airgap
BUILDER_IP=<BUILDER_PUBLIC_IP> AIRGAP_IP=<AIRGAP_PUBLIC_IP> \
  SSH_KEY=./nullus-airgap-key.pem ../scripts/20-transfer-bundle.sh

# (다음) airgap VM 에서 설치 (registry + kind + Nullus)
AIRGAP_IP=<AIRGAP_PUBLIC_IP> SSH_KEY=./nullus-airgap-key.pem \
  ../scripts/30-install-on-airgap.sh

# 영구 외부 노출 (systemd port-forward) — SG 는 web_allowed_cidr 로 제한
AIRGAP_IP=<AIRGAP_PUBLIC_IP> SSH_KEY=./nullus-airgap-key.pem \
  ../scripts/40-expose-service.sh

# 노출 중지 / 상태
ssh -i ./nullus-airgap-key.pem ubuntu@<AIRGAP_PUBLIC_IP> 'sudo systemctl disable --now nullus-expose'
ssh -i ./nullus-airgap-key.pem ubuntu@<AIRGAP_PUBLIC_IP> 'systemctl status nullus-expose'

# 정리 (과금 중지)
terraform destroy
```
