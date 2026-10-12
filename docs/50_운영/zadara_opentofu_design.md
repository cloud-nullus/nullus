# Zadara OpenTofu 설계 — 온라인 PoC, m1 + w1

작성일: 2026-09-09 · 갱신: 2026-09-10 (I0 실측 반영, I1 구현 완료) · 상태: I2 plan 검토 단계

KakaoCloud의 `deploy/csp/kakaocloud/opentofu`를 참고해 Zadara용 구성을 새로 만든다. **airgap·builder VM·kind·번들 전송은 제외한다.** `m1`(master), `w1`(worker)은 VM 이름이며 Zadara instance type 이름을 뜻하지 않는다. 기존 클라우드 자원은 이번 설계에서 변경하지 않았다.

## 1. 구성과 자원 상한

**m1(master) 1대 + w1(worker) 1대의 단일 Kubernetes 클러스터**를 기본 계획으로 삼는다. 지원 자원을 최대한 VM에 할당하고 w1에서 Nullus와 다양한 스택을 테스트한다. airgap과 별도 builder는 제외한다.

**2026-09-10 실측으로 대조를 마쳤다.** 기존 4대에 bastion(`121.78.39.184`) 경유로 직접 접속해 측정한 합계는 **20 vCPU / RAM 38.8GiB(공칭 40GB) / disk 400GiB**로, 기존 문서의 4대 합산값과 일치한다. 단위는 RAM이 공칭 GB(실측 GiB에 약 3% 펌웨어 예약), 디스크가 GiB다.

| VM | 보고 type | 실측 vCPU | 실측 RAM | 디스크 | 클러스터 |
|---|---|---:|---:|---:|---|
| nullus-node-10 | c4.large | 2 | 3.8GiB | 100GiB | platform (control-plane) |
| nullus-node-11 | c5.2xlarge | 8 | 15.6GiB | 100GiB | platform (worker) |
| nullus-node-20 | c5.large | 2 | 3.8GiB | 100GiB | develop |
| nullus-node-21 | c5.2xlarge | 8 | 15.6GiB | 100GiB | develop |
| 합계 | | **20** | **38.8GiB** | **400GiB** | |

**다만 이것은 현재 사용량이지 테넌트 한도가 아니다.** quota 상한은 여전히 미측정이다. EC2 호환 API에 quota 조회가 없고(`describe-account-attributes`는 supported-platforms/default-vpc만 반환), DryRun으로도 측정되지 않는다 — zCompute는 DryRun을 존중하지만 `zp2.28xlarge`(112 vCPU) 1대, `z2.4xlarge` 50대, 카탈로그에 없는 `c5.2xlarge`까지 모두 "would have succeeded"를 반환한다. 요청 형식과 권한만 검사한다. **테넌트 관리자 확인이 남은 유일한 수단이다.** 그때까지 20/40/400은 "현재 사용량 기준"이라는 근거로만 쓴다.

instance type은 `z2/z4/z8/z16`, `zp2/zp4/zp8/zp16` 계열 378종이 카탈로그에 있고, 아래 배분에 맞는 것은 m1 = `z2.xlarge`(4c/8GiB), w1 = `z2.4xlarge`(16c/32GiB)다. 기존 4대가 보고하는 `c4`/`c5`는 카탈로그에 없으나, DryRun이 type을 검증하지 않으므로 생성 불가라고 단정하지는 않는다.

| VM | vCPU | RAM | disk | 역할 |
|---|---:|---:|---:|---|
| m1 | 4 | 8GB | 100GB | control plane + etcd, 관리용 SSH 진입점 |
| w1 | 16 | 32GB | 300GB | Nullus API/Web·PostgreSQL·Keycloak/DB·Ingress·테스트 스택 |
| 합계 | **20** | **40GB** | **400GB** | 문서상 기존 4대의 합계와 동일 |

**m1에도 일반 워크로드를 배치한다.** 노드가 2대뿐이라 m1의 4 vCPU/8GB를 control plane 전용으로 비워 두지 않는다. inventory에서 m1을 `kube_control_plane`·`etcd`·`kube_node`에 모두 넣으면 Kubespray v2.30.0이 kubeadm `nodeRegistration`을 `taints: []`로 렌더해 NoSchedule taint가 붙지 않는다(`roles/kubernetes/control-plane/templates/kubeadm-config.v1beta4.yaml.j2:19`). `schedule_on_control_plane = false`로 되돌리면 taint가 유지되지만 워크로드는 w1 한 대에 몰린다. 기존 `platform`/`develop` 클러스터는 둘 다 taint를 유지하고 있어 이 PoC와 다르다. w1의 `nullus` namespace에 플랫폼, 별도 namespace에 테스트 스택을 배치한다. namespace 분리는 별도 클러스터 검증이나 완전한 자원 격리를 뜻하지 않는다. 플랫폼 requests와 테스트 ResourceQuota/LimitRange를 설정하고 필요하면 NetworkPolicy도 적용한다.

지원 자원을 VM에 최대한 할당하는 것과 Pod가 실제 자원을 100% 소모하도록 설정하는 것은 다르다. OS·etcd·kubelet·이미지 캐시·일시적 배포 피크의 여유를 VM 안에 둔다. 300GB는 worker 볼륨 예산이며 전부 PVC 요청량으로 배정하지 않는다. root/data 분리 여부는 mount·보존 정책과 함께 정한다. snapshot/백업이 같은 quota에 포함되면 그만큼 VM 디스크 배분도 조정한다.

### 테스트 범위

- 플랫폼: 로그인/권한/조직, DB 마이그레이션, 플랫폼 업데이트와 복원.
- 스택: 설치/삭제, CI/CD, 레지스트리, 관측 도구와 SSO 조합. 설치 피크와 실사용량으로 동시 실행 범위를 정한다.
- 네트워크: m1↔w1의 API/kubelet/CNI 통신, DNS·Service·Ingress. worker 두 대 사이의 workload 통신 검증은 제외한다.
- 장애: 동일 worker에서 Pod 재시작과 데이터 보존, w1 장애의 서비스 중단과 복구 절차.
- 스토리지: PVC 쓰기/보존, 클러스터 외부 백업·복원. local-path는 worker 장애 시 다른 노드로 자동 이동하지 않는다.

**제외:** 외부 클러스터 등록 검증, worker 간 재배치, control-plane HA. 해당 시험이 필요하면 같은 총량 내 w1을 w1/w2로 분할하거나 별도 클러스터로 재구성하는 후속 계획을 세운다. 단순 변수 변경으로 기존 PVC와 클러스터 소속을 자동 이전하지 않는다.

### 기존 4대에서 전환

**실측 결과 기존 4대는 클러스터 2개다.** `platform`(node-10 control-plane + node-11)과 `develop`(node-20/21)이며, 네 노드 모두 Ubuntu 24.04.3 LTS / kernel 6.8.0-90 / containerd 2.2.1 / Kubernetes v1.34.3이다. m1+w1로 전환한다는 것은 `platform`뿐 아니라 **`develop` 클러스터도 함께 반납한다는 뜻**이다. 아래 순서는 두 클러스터 모두를 대상으로 한다.

현재 VM이 상한을 점유한다고 보고 새 2대를 병행 생성할 수 있다는 가정을 하지 않는다.

1. 기존 VM/볼륨/IP/클러스터와 실제 사용량을 조회하고 보존할 DB·Keycloak·키·스택 데이터를 확정한다.
2. 보존 데이터는 반납할 VM 밖으로 백업하고 복원 가능성을 확인한다. 기존 환경 자동 CD의 중단/재개 시간과 DNS 전환을 계획한다.
3. API에서 지원하는 사양 변경과 import 가능성을 확인해 기존 VM 재사용 또는 백업 후 재생성을 선택한다. 스토리지 반환·볼륨 보존이 quota에 미치는 영향도 확인한다.
4. 삭제/교체 대상·중단 시간·복구 방법을 검토한 뒤 필요한 기존 자원을 반납하고, quota가 실제 반환된 후 m1/w1 생성 또는 크기 변경을 진행한다.
5. 새 클러스터에 데이터와 설정을 복원하고 기능 검증 후 CD/DNS를 연결한다. 옛 VM 반납 이후 원복은 단순 DNS 복귀가 아니라 재생성·복원일 수 있다.

이번 계획 수립은 기존 자원 삭제·초기화 실행을 포함하지 않는다. 백업/전환 절차와 실제 plan이 준비되기 전에는 자원을 반납하지 않는다.

## 2. KakaoCloud 코드 전환 범위

| 현재 파일/기능 | Zadara에서 할 일 |
|---|---|
| `provider.tf`: kakaocloud 0.3.5 | AWS provider + Zadara endpoint 및 검증된 버전 고정 |
| `modules/network` | VPC/subnet/route/IGW 및 private VM의 egress 정의 |
| `modules/security` | 운영자 SSH, 클러스터 내부 통신, 선택한 웹 접근만 허용 |
| `modules/compute`: builder/airgap 고정 2대 | `nodes` map을 `for_each`로 순회하여 m1/w1 생성 |
| VM별 공인 IP | 기본 m1만 EIP. private worker의 인터넷 egress는 별도 확보 |
| 키페어 생성 + 개인키 파일 저장 | 등록된 `nullus-key` 참조. 개인키를 state/cloud-init에 넣지 않음 |
| Docker/Compose cloud-init | Python·SSH·CA 등 Ansible 준비만. containerd/Kubernetes는 Kubespray 담당 |
| image/flavor 이름 검색 후 `[0]` | 확정: image `ami-d57f606b4d714d0cb591196e33259d53`(Ubuntu 24.04), m1 `z2.xlarge` / w1 `z2.4xlarge`, AZ `symphony` |
| image/flavor `ignore_changes` | 그대로 복사하지 않고 변경에 따른 VM 교체를 plan에 노출 |
| 번들 빌드·전송 outputs | 노드 주소/역할, SSH 경로, Kubespray inventory로 교체 |
| 사용하지 않는 null provider | 제거 |

KakaoCloud 파일·state·실제 tfvars는 수정/복사하지 않는다. 새 경로는 `deploy/csp/zadara/opentofu/`다. 기존 network/security/compute 분리는 유지하되 공통 CSP 추상화 모듈까지 만들지 않는다.

## 3. 먼저 해결할 Provider 제약

Zadara 공식 가이드는 AWS provider 3.33, 공식 Kubernetes 모듈은 `>=3.33.0, <=3.35.0`을 안내한다. 최신 provider를 무조건 선택하지 않고 **테넌트 지원 버전 → provider 설치 → 읽기 조회 → 시험 VM 수명주기** 순서로 확인한다. [공식 가이드](https://www.zadara.com/blog/2024/03/19/terraform-infrastructure-as-code-with-zadara-zcompute/), [공식 버전 제약](https://github.com/zadarastorage/terraform-zcompute-k8s/blob/main/versions.tf)

- EC2 endpoint, 프로젝트/계정 범위, CA, image ID, instance type, AZ와 quota가 필요하다. 키는 Zadara 전용 profile/환경변수로 공급한다.
- 공식 예제의 TLS 검증 해제는 복사하지 않는다. 사용하는 AWS 서비스 endpoint는 모두 명시해 AWS 기본 endpoint로 향하지 않게 한다.
- ~~오래된 provider의 macOS arm64 패키지 제공 여부를 확인한다.~~ **확인 완료**: `hashicorp/aws` 3.33.0 darwin_arm64 패키지가 제공되어 로컬에서 init/plan이 동작한다. Linux amd64 고정은 불필요하다.
- 현재 로컬은 `OpenTofu v1.12.5 on darwin_arm64`다. KakaoCloud의 `required_version >=1.13.5`는 그대로 복사하면 안 된다. 사용하는 문법에 맞는 최소 버전과 실행 버전을 정한다.
- 검증된 버전과 `.terraform.lock.hcl`을 고정한다. `tofu validate`만으로 zCompute API 호환성을 증명하지 않는다. **2026-09-10 실제 endpoint에 `tofu plan`이 통과했다**(22 to add / 0 to change / 0 to destroy). VPC·subnet·IGW·NAT·route table·security group·EIP·instance 조회가 모두 응답했다.

## 4. 네트워크와 부트스트랩

### 관리 진입점 m1

m1에 공인 IP를 두고 운영자 CIDR에서 SSH만 허용한다. Kubernetes API는 외부 공개하지 않는다. 웹은 초기 SSH 터널, SSO 검증부터 실제 HTTPS/도메인 경로를 구성한다. 인터넷 이미지/패키지를 직접 받으며 별도 builder·registry mirror·NAT VM은 만들지 않는다.

### worker w1의 인터넷 접근

추가 private VM에는 이미지 pull과 패키지 설치를 위한 outbound 경로가 필요하다. **private subnet만 생성해서는 설치가 되지 않는다.** 테넌트 제공 NAT/기존 egress 사용을 우선 확인하고 비용도 포함한다. 불가능하면 노드별 public IP와 제한된 inbound를 대안으로 비교한다. m1의 임의 NAT 겸용은 기본안에서 제외한다. 선택한 egress 방식은 변수와 README에 명시하고, 미확정 상태에서 private worker 생성을 진행하지 않는다.

Kubespray는 로컬/별도 실행 환경에서 SSH 경유로 실행하는 안을 우선한다. Ansible controller의 지원 Python 환경을 별도로 고정한다. 생성 inventory에 m1/w1의 private IP/역할을 담고 ProxyJump 또는 검증된 bastion 경로를 적용한다. 개인키는 로컬에 유지한다.

Kubespray inventory는 단일 `platform` 클러스터용으로 생성한다. m1은 `kube_control_plane`/`etcd`/`kube_node`, w1은 `kube_node`에 배치한다. **m1의 NoSchedule taint가 실제로 없는지**와 두 노드 모두에 Pod가 스케줄되는지를 검증한다.

## 5. 파일 구조와 변경 경계

**2026-09-10 아래 구조로 구현을 마쳤다.** 상세는 [`deploy/csp/zadara/opentofu/README.md`](../../deploy/csp/zadara/opentofu/README.md).

```text
deploy/csp/zadara/opentofu/
  provider.tf / main.tf / variables.tf / outputs.tf
  cloud-init.yaml
  terraform.tfvars.example
  .gitignore / .terraform.lock.hcl
  modules/network/
  modules/security/
  modules/compute/
  templates/inventory.ini.tftpl
  tests/topology.tftest.hcl
  README.md
```

`tests/topology.tftest.hcl`은 mock provider로 토폴로지·총량·입력 검증을 확인한다(11 run, 테넌트 접근 불필요). 신규 VPC 대역은 기존 `172.31.0.0/16`·`10.0.0.0/16`과 겹치지 않게 `10.20.0.0/16`으로 잡았다.

- `nodes`: 이름을 key로 cluster, role, private IP, instance type, disk 크기를 지정한다. count index 대신 이름을 써 향후 worker 추가가 기존 VM 재생성으로 이어지지 않게 한다.
- 초기 구현은 m1/w1과 platform inventory 하나를 대상으로 한다. 잠정 상한 20 vCPU/40GB/400GB와 실제 quota/type을 대조해 nodes map에 배분한다.
- OpenTofu는 클라우드 자원과 inventory까지 소유한다. `remote-exec`로 Kubespray/Helm을 apply에 묶지 않는다.
- cloud-init은 최소 OS 준비, Kubespray는 Kubernetes/containerd/CNI, Helm/스크립트는 Nullus와 애드온을 담당한다.
- local-path는 노드 로컬 데이터다. worker 추가가 기존 PVC의 이동/복제를 뜻하지 않는다. 데이터 이관이나 CSI는 별도 단계다.
- state/plan/tfvars/개인키/generated는 git 제외, provider lock은 커밋 대상이다. 팀/CI 실행 전 잠금과 백업이 검증된 backend를 정한다.

## 6. 실행 순서와 통과 기준

| 단계 | 작업 | 통과 기준 |
|---|---|---|
| I0 ◑ | 기존 4대 합계·실제 quota/type·전환 방식·테넌트 API 확정 | 사양·type·image·AZ·API 조회 **완료**. **quota 상한과 egress 방식 미확정** |
| I1 ✅ | KakaoCloud 구조에서 Zadara 최소 코드 작성 | fmt/init/validate 통과, 입력 검증 테스트 11 run 통과 |
| I2 | m1/w1 전체 plan 검토 | 지원 총량/쿼터 내 2대 배분 및 기존 자원 반납 순서, SG/IP/egress/disk 확인, 의도하지 않은 기존 자원 삭제/교체 없음 |
| I3 | 저장 plan 적용과 inventory 인계 | cloud-init/SSH/Python/sudo/egress 정상, 재plan no-op |
| I4 | m1/w1 단일 클러스터 구축과 플랫폼 배포 | 노드 Ready, **m1에 NoSchedule taint 없음**, 두 노드에 Pod 스케줄, 로그인/DB/API 정상 |
| I5 | 작은 스택 1개 설치 | 데이터 쓰기·HTTPS/SSO·상태 조회/삭제 정책 정상 |
| I6 | 스택 조합·노드 간·장애 테스트 | 노드 통신, 설치 피크, Pod 재시작·worker 장애 영향과 용량 한계 확인 |
| I7 | 백업/복원과 결과 정리 | 클러스터 외부 백업으로 데이터·로그인 복원, 잔여 위험 기록 |

플랫폼과 스택은 같은 클러스터를 사용한다. 내부 클러스터 등록이 필요한 제품 흐름은 별도로 검증하되 원격 클러스터 검증으로 보고하지 않는다. 기존 환경의 import/재생성은 §1 전환 순서를 따른다.

**첫 산출물은 확정한 지원량을 m1/w1에 배분하는 코드와 검토 가능한 plan**이며 2026-09-10 둘 다 확보했다. plan은 22개 생성 / 0개 변경 / 0개 삭제로 기존 자원을 건드리지 않는다. apply는 실행하지 않았다 — 기존 4대가 자원을 점유하고 있고 quota 상한이 미확정이므로 §1 전환 순서(백업 → 반납 → 생성)가 선행이다.
