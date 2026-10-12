# Zadara zCompute OpenTofu — 온라인 PoC (m1 + w1)

`deploy/csp/kakaocloud/opentofu` 를 참고해 Zadara 용으로 새로 만든 구성이다.
**airgap·builder VM·kind·번들 전송은 포함하지 않는다.** 설계 근거는
[Zadara OpenTofu 설계](../../../../docs/50_운영/zadara_opentofu_design.md),
배포 이후 단계는 [배포 재계획](../../../../docs/50_운영/zadara_cloud_deployment_plan.md).

소유 범위는 **클라우드 자원과 Kubespray inventory 까지**다. Kubernetes/containerd/CNI 는
Kubespray, Nullus 와 애드온은 Helm/스크립트가 담당한다. `remote-exec` 로 묶지 않는다.

## 구성

| 파일 | 내용 |
|---|---|
| `provider.tf` | AWS provider 3.33 고정 + zCompute EC2 endpoint. TLS 검증은 해제하지 않는다 |
| `modules/network` | VPC / public·private 서브넷 / IGW / NAT gateway |
| `modules/security` | node(클러스터 내부, 전 노드) · bastion(운영자 SSH, control plane) · web(공인 IP 보유 노드, 기본 0개 규칙) |
| `modules/compute` | `nodes` map 을 `for_each` 로 순회해 VM + 공인 노드 EIP 생성 |
| `templates/inventory.ini.tftpl` | Kubespray inventory. `generated/inventory.ini` 로 출력 |
| `cloud-init.yaml` | Python/SSH/CA 등 Ansible 준비만 |
| `capacity.tf` · `scripts/instance_types.py` | 실제 카탈로그를 읽어 선언한 CPU/RAM과 대조 |
| `tests/topology.tftest.hcl` | 토폴로지·총량·입력 검증 (mock provider, 테넌트 접근 불필요) |

## 배치 결정

- **m1(control plane)** 은 public 서브넷의 운영자 SSH 진입점이다. Kubernetes API 는 외부에
  열지 않는다. 웹은 초기에 SSH 터널로 접근하고, TLS/SSO 경로 확정 후 `web_cidrs` 를 좁혀 연다.
- **m1 에도 워크로드를 배치한다** (`schedule_on_control_plane = true`, 기본값). 노드가 2대뿐이라
  m1 의 4 vCPU/8GB 를 control plane 전용으로 놀리지 않는다. inventory 에서 m1 을
  `kube_control_plane`·`etcd`·`kube_node` 세 그룹에 모두 넣으면 Kubespray v2.30.0 이
  kubeadm `nodeRegistration` 을 `taints: []` 로 렌더해 NoSchedule taint 가 붙지 않는다
  (`roles/kubernetes/control-plane/templates/kubeadm-config.v1beta4.yaml.j2:19`).
  control plane 을 격리하려면 `schedule_on_control_plane = false` 로 둔다 — 그때는 워크로드가
  w1 한 대에만 몰린다.
  기존 `platform`/`develop` 클러스터는 둘 다 taint 를 유지하고 있어 node-10/20 에는
  kube-system 만 떠 있다. 이 PoC 는 그와 다른 선택을 한다.
- **w1(worker)** 의 인터넷 egress 는 `worker_egress` 로 고른다.
  - `nat` (기본): private 서브넷 + NAT gateway. worker 에 공인 IP 가 없다.
  - `public_ip`: NAT 를 쓸 수 없는 테넌트용 대안. worker 를 public 서브넷에 두고 공인 IP 를 붙인다.
  private 서브넷만 만들고 egress 를 확보하지 않으면 이미지/패키지 설치가 되지 않는다.
  이 값은 테넌트의 NAT 제공 여부·비용을 확인한 뒤 정한다.
- **총량 상한**은 `capacity_limit`(기본 20 vCPU / 40GB / 400GB)이며 `nodes` 합계가 넘으면
  변수 검증에서 막는다. 실제 quota 를 조회해 값을 바로잡는다.
- **역할**은 `role` 을 생략하면 이름으로 정한다: `m*` = control_plane, `w*` = worker.
  다른 이름을 쓰려면 `role` 을 명시한다.
- **사설 IP** 는 서브넷 CIDR 에서 결정적으로 배정한다(오프셋 10부터). apply 전에 plan 과
  inventory 를 함께 검토할 수 있어야 하기 때문이다. `private_ip` 로 개별 지정도 가능하다.
- `image_id` / `instance_type` 에 `ignore_changes` 를 걸지 않는다. 값이 바뀌면 VM 교체가
  plan 에 그대로 드러나야 한다.
- **m1 공인 IP 는 VM 보다 오래 산다(D10, 2026-09-13).** `aws_eip.public` 을 루트에서
  `prevent_destroy` 로 보호하고 VM 과는 `aws_eip_association` 으로만 묶는다. VM 교체·
  `nodes` 변경은 association 만 다시 만들고 121.78.39.241 은 유지된다. 스택을 완전히
  걷어낼 때만 `prevent_destroy` 를 먼저 푼다. 기존 EIP 는 `tofu state mv` 와
  association `tofu import` 로 무중단 전환했고 zCompute 에서 association import 가 동작함을 확인했다.
- 개인키는 만들지도 저장하지도 않는다. 이미 등록된 `key_name` 만 참조하고, 로컬 경로는
  inventory 에만 기록한다.

## 자격증명

`~/.aws/config`:

```ini
[profile zadara]
region = symphony
```

`~/.aws/credentials`:

```ini
[zadara]
aws_access_key_id     = <zCompute access key>
aws_secret_access_key = <zCompute secret key>
```

`ec2_endpoint` 는 tfvars 로 넣는다. 코드에 자격증명을 두지 않는다.

## 사용

```bash
cp terraform.tfvars.example terraform.tfvars   # 조회로 확정한 값을 채운다
tofu init
tofu fmt -check -recursive
tofu validate
tofu test                                       # mock 테스트, 테넌트 접근 없음
python3 -m unittest discover -s scripts -p 'test_*.py'
tofu plan -out=tfplan                           # 여기서부터 테넌트 접근 필요
tofu apply tfplan                               # 기존 환경 반납/백업 결정 후에만 실행
```

apply 후 `generated/inventory.ini` 를 Kubespray 에 인계한다. private worker 는 m1 을
경유하는 `ProxyCommand` 가 inventory 에 들어간다.

```bash
tofu output -raw kubespray_inventory
ansible -i generated/inventory.ini all -m ping
```

## 조회로 확인한 테넌트 사실 (2026-09-10, `--profile zadara` 읽기 조회)

| 항목 | 값 | 비고 |
|---|---|---|
| EC2 endpoint | `https://compute-iaas-kr1-01.zadara.com/api/v2/aws/ec2` | `~/.aws/config` 의 `[profile zadara]` |
| region / AZ | `symphony` | 두 값이 같다 |
| 기존 VPC | `172.31.0.0/16`(default, subnet `172.31.0.0/20`), `10.0.0.0/16` | 신규 대역을 `10.20.0.0/16` 으로 잡은 이유 |
| 기존 VM | `nullus-node-10/11/20/21` 4대 running | **클러스터 2개**: `platform`(node-10 control-plane + node-11), `develop`(node-20/21) |
| 기존 볼륨 | 4 x 100GiB gp2 = **400GiB** | `lsblk` 107GB = 100GiB. 문서의 disk 400GB 와 일치 |
| 카탈로그 instance type | `z2/z4/z8/z16`, `zp2/zp4/zp8/zp16` 계열 378종 | **`c4`/`c5` 는 목록에 없다**. 생성 불가 증명은 아니다(아래 DryRun 참고) |
| 4c/8G · 16c/32G | `z2.xlarge` / `z2.4xlarge` (또는 `zp2.*`) | m1 / w1 후보 |
| 이미지 | Ubuntu Server 24.04 `ami-d57f606b4d714d0cb591196e33259d53`, 22.04 `ami-8fffbd9d1f9d4eb789e39d6c81119ce3` | 32개 available |
| keypair | `nullus-key` 1개 | 개인키는 로컬에서 확보해야 한다 |
| EIP | 1개 할당됨(2026-09-10) | 2026-09-13 apply로 2개 추가 할당(NAT, m1) 성공. 반납된 구 EIP `eipalloc-bf54f437`(121.78.39.184)는 detach 상태로 수동 release 필요 |
| NAT gateway | 현재 0개(2026-09-10) | 2026-09-13 apply로 생성 34초 성공, w1(private)에서 `https://archive.ubuntu.com` 200 확인 — egress 실증 완료(생성 검증됨) |

### 기존 4대 실측 (bastion `121.78.39.184` 경유 SSH, 2026-09-10)

`c4`/`c5` 는 생성 가능 목록에 없어 API 로 사양을 읽을 수 없다. 노드에 직접 붙어 측정했다.

| VM | 보고 type | 실측 vCPU | 실측 RAM | 디스크 | 클러스터 |
|---|---|---:|---:|---:|---|
| nullus-node-10 | c4.large | 2 | 3.8 GiB | 100 GiB | platform (control-plane) |
| nullus-node-11 | c5.2xlarge | 8 | 15.6 GiB | 100 GiB | platform (worker) |
| nullus-node-20 | c5.large | 2 | 3.8 GiB | 100 GiB | develop |
| nullus-node-21 | c5.2xlarge | 8 | 15.6 GiB | 100 GiB | develop |
| **합계** | | **20** | **38.8 GiB** (공칭 40 GB) | **400 GiB** | |

- 문서의 잠정 상한 **20 vCPU / 40 GB / 400 GB 는 현재 사용량과 일치**한다. 단위는 RAM 이
  공칭 GB(=실측 GiB 에 약 3% 펌웨어 예약), 디스크는 GiB 다.
- **기존 자원은 클러스터 2개다.** m1+w1 로 전환하려면 `platform` 뿐 아니라 `develop`
  클러스터도 함께 반납해야 한다. 설계 §1 전환 순서는 이 전제로 다시 봐야 한다.
- OS 는 4대 모두 Ubuntu 24.04.3 LTS / kernel 6.8.0-90 / containerd 2.2.1 / k8s v1.34.3.
  설계가 남겨 둔 "22.04 가정 vs 24.04.3 기록" 불일치는 **24.04.3 이 실제**다.
- **2026-09-13 terminate 이후**: 4대 모두 삭제, 4x100GiB root volume은 인스턴스와 함께 자동 삭제됨(`delete_on_termination=true`). 반납 직후 describe-instances가 보고하던 type이 `c4.large`→`z2.large`, `c5.2xlarge`→`z2.2xlarge`로 바뀌었다(카탈로그 실제 type 노출). 남은 볼륨은 CoreDNS 서비스 VM용 4GiB x2뿐. platform 클러스터엔 PVC 11개, Helm release 12개가 있었고 백업 없이 폐기됐다(사용자 결정).

**아직 확인되지 않은 것**

- **quota 상한 자체는 여전히 미측정이다.** 위 20/38.8/400 은 *현재 사용량*이지 테넌트
  한도가 아니다. `describe-account-attributes` 는 supported-platforms/default-vpc 만
  반환하고 EC2 호환 API 에 quota 조회가 없다. zCompute 네이티브 API 자격증명도 없다.
  **DryRun 으로도 측정되지 않는다** — 아래 프로브 결과 참고. 테넌트 관리자 확인이 남은
  유일한 수단이다. (2026-09-13 추가) `describe-account-attributes`에 `max-instances = 20` 항목이 있으나
  이는 인스턴스 개수 한도이며 vCPU/RAM 한도가 아니다. `service-quotas` API는 400을 반환한다.
- NAT gateway / EIP 실제 생성과 요금, 기존 자원 반납 시 quota 반환 시점.
- 기존 4대의 데이터·역할과 반납/전환 순서.

### DryRun 프로브 결과 (2026-09-10, 자원 생성 0건)

zCompute 는 DryRun 플래그를 **존중한다**. `allocate-address --dry-run` 이
`DryRunOperation` 을 반환했고 EIP 목록은 1개 그대로였다. 이어서 `run-instances --dry-run`
을 돌렸고 인스턴스 수는 6개 그대로였다.

| 프로브 | 결과 | 해석 |
|---|---|---|
| `z2.xlarge` / `z2.4xlarge` 1대 | would have succeeded | — |
| `zp2.28xlarge`(112 vCPU/224GiB) 1대 | would have succeeded | 현 사용량 위에 얹힐 수 없는 크기 |
| `z2.4xlarge` **50대** | would have succeeded | 명백히 quota 초과 |
| 카탈로그에 없는 `c5.2xlarge` | would have succeeded | type 유효성조차 검사하지 않는다 |

**따라서 DryRun 은 quota 도 instance type 도 검증하지 않는다.** 요청 형식과 권한만 본다.
"would have succeeded" 를 생성 가능의 근거로 쓰면 안 된다. 부수적으로 얻은 사실은
두 가지뿐이다: 이 자격증명에 EIP 할당과 인스턴스 생성 **권한이 있다**는 것,
그리고 `c4`/`c5` 가 생성 불가라고 **단정할 수 없다**는 것(카탈로그에 없을 뿐이다).

이 값들이 확정되기 전에는 `capacity_limit` 을 근거 있는 상한으로 취급하지 않는다.

## 실행 전 확정할 값 (설계 I0)

`tofu plan` 을 실행하려면 아래가 먼저 필요하다. 미확정 상태로 apply 하지 않는다.

- zCompute EC2 endpoint, 프로젝트/계정 범위, CA
- 확정된 image ID, instance type, AZ, 실제 quota (문서 합계가 아니라 조회 결과)
- worker egress 방식(테넌트 NAT 제공 여부와 비용)
- 기존 4대 자원의 반납/전환 순서 — quota 가 실제로 반환된 뒤에 생성한다

## 제외 범위

state backend(팀/CI 실행 전 잠금·백업 검증 필요), CSI, LB/CCM, Ingress 구현 선정,
control-plane HA, worker 간 재배치. `local-path` 는 노드 로컬 데이터이며 worker 추가가
기존 PVC 이동을 뜻하지 않는다.

## 이번 구현의 검증과 Zadara API 차이 (2026-09-10)

- `tofu validate`: 성공. `tofu test`: 15 passed / 0 failed. 카탈로그 선택 Python 단위 테스트: 3 passed.
- `zadara` profile로 인스턴스·type·이미지·VPC·NAT·키페어·AZ·볼륨 읽기 접근을 재확인했다. 이 날짜의 직접 조회에서 전체 볼륨은 관리용 자원을 포함해 6개/408GiB였으므로 위 4개/400GiB 기록은 Nullus VM용 예산과 구분한다.
- AWS provider 3.33의 `aws_ec2_instance_type`은 zCompute에서 `multiple instance types found`로 실패했다. `describe-instance-types` 필터에 의존하지 않고 전체 결과에서 이름이 정확히 일치하는 1건을 선택하는 읽기 전용 Python 프로그램을 external data source로 사용한다. AWS CLI v2와 Python 3가 plan 실행 환경에 필요하다.
- 실제 plan은 31개 추가, 변경/삭제 0개였다. 이는 신규 VPC/NAT/EIP/VM 계획이며 기존 4대의 state를 관리하거나 quota를 반환하지 않는다. 생성 성공·quota 여유를 증명하는 결과가 아니다.
- 기존 `.sh` 6개는 원본과 동일한 `.sh.bak`으로 보존했다. CD가 원본 경로를 호출하므로 원본은 유지한다. 이 IaC는 기존 스크립트를 자동 실행하지 않는다.

`web_ports`는 보안 그룹만 설정한다. 80/443 listener/포워딩은 별도 설치가 필요하다. NodePort를 직접 시험하려면 30080/30443과 Ingress의 실제 포트를 맞춘다. `root_block_device.delete_on_termination=false`이므로 VM을 반납해도 root volume이 남아 저장 quota를 계속 사용한다. 백업 후 별도 볼륨 반납을 검토한다.

**2026-09-13 실제 apply 결과** (`VALIDATION.md` §실제 apply 참고): plan 단계에서 미확인이던 "apply 미실행" 상태는 종료됐다. 실측 결함 2건(D7 태그 400, D8 associate_public_ip_address 400)을 `.tf`에 반영한 뒤 3회 apply로 31개 전부 생성. NAT gateway·EIP 생성과 egress를 실증했고, Kubespray v2.30.0으로 m1/w1 2노드 클러스터가 Ready 상태다. 위 "생성 성공·quota 여유를 증명하는 결과가 아니다"라는 2026-09-10 시점 서술은 plan 단계에 한정되며, apply 자체는 이후 성공했다.

### Kubespray 인계

Kubespray v2.30.0(`f4ccdb5e72395eaf9f3444056ebd1a6625ddb89a`)과 해당 `requirements.txt`를 사용한다. inventory 하나만으로 실행하지 말고 sample group_vars에 생성 override를 합친다.

```bash
# Kubespray clone 디렉토리에서. TF_ROOT는 OpenTofu 디렉토리의 절대 경로.
cp -a inventory/sample inventory/platform
cp "$TF_ROOT/generated/inventory.ini" inventory/platform/inventory.ini
cp "$TF_ROOT/generated/zz-nullus-overrides.yml" inventory/platform/group_vars/k8s_cluster/
ansible-inventory -i inventory/platform/inventory.ini --graph
ansible -i inventory/platform/inventory.ini all -b -m command -a 'cloud-init status --wait'
ansible -i inventory/platform/inventory.ini all -m ping
ansible-playbook -i inventory/platform/inventory.ini cluster.yml -b
```

override는 Calico VXLAN(`4789/udp`, IPIP/BGP 미사용)에 맞춰 SG와 일치시킨다. 최초 SSH host key는 신뢰할 수 있는 경로로 확인하고 known_hosts에 등록한다. 새 VPC CIDR와 Kubespray Pod/Service CIDR가 겹치지 않는지 실행 전 확인한다. inventory의 IP/키 경로를 기존 4대 환경의 스크립트 기본값으로 덮어쓰지 않는다.
