# Zadara 구현/설치 준비 검증 — 2026-09-10

## 실행 결과

| 명령/검증 | 관측 결과 |
|---|---|
| `tofu init -backend=false -input=false` | 성공: aws 3.33.0 / local 2.9.0 / external 2.3.5 |
| `tofu fmt -check -recursive` | exit 0 |
| `tofu validate` | `Success! The configuration is valid.` |
| `tofu test` | `Success! 15 passed, 0 failed.` |
| `python3 -m unittest discover -s scripts -p 'test_*.py'` | 3 tests, OK |
| `tofu plan -input=false -out=generated/zadara.tfplan` | 31 create / 0 change / 0 destroy |
| `.sh` ↔ `.sh.bak` byte 비교 | 6개 모두 일치 |
| `git diff --check` | exit 0 |

실제 plan은 ignored `generated/zadara.tfplan`, 로그는 `generated/plan.log`에 있다.
plan SHA256: `7fd8e9e3a140b54d2dcf26ee3bc322b5cdd4e6d604f42ec16e41dcc3447327d8`.
설정/클라우드 상태가 바뀌면 재plan 후 검토하며, 이 해시의 plan을 무조건 재사용하지 않는다.

## 실제 읽기 접근

`aws --profile zadara ec2`의 describe-instances / describe-instance-types /
describe-images / describe-vpcs / describe-nat-gateways / describe-key-pairs /
describe-availability-zones / describe-volumes 호출 모두 성공했다.

- Nullus VM 4대 running, 관리용 CoreDNS VM 2대도 별도로 존재.
- `z2.xlarge`: 4 vCPU / 8192MiB, `z2.4xlarge`: 16 vCPU / 32768MiB.
- Ubuntu Server 24.04 이미지 available, `nullus-key` 키페어, `symphony` AZ available.
- 기존 VPC CIDR: 172.31.0.0/16, 10.0.0.0/16. 새 계획: 10.20.0.0/16.
- NAT 현재 0개. 전체 볼륨은 6개/408GiB이며 Nullus VM 예산 400과 관리용 자원을 구분해야 한다.
- 기존 node-10에 SSH 성공. platform PVC 11개 Bound, develop PVC 0개.

CPU/RAM 조회 필터가 무시되는 실제 API 문제를 확인해 `scripts/instance_types.py`로
전체 목록 중 정확한 이름 1건을 선택한다. 이를 통해 plan에서 실제 사양을 확인했다.

## 설치 대기 조건 (2026-09-10 시점, 이후 실행 완료 — 아래 §실제 apply 참고)

기존 node-10/11/20/21이 잠정 지원량을 사용 중이었다. 사용자는 백업 없이 기존 4대
terminate 를 결정했다 — platform/develop 클러스터의 PVC 11개, Helm release 12개는
폐기됐다. 2026-09-13 apply/Kubespray 실행으로 이 대기 상태는 종료됐다.

## 실제 apply — 2026-09-13

| 단계 | 결과 |
|---|---|
| 기존 자원 반납 | node-10/11/20/21 terminate, 4x100GiB root volume 자동 삭제(delete_on_termination=true). CoreDNS 서비스 볼륨 4GiB x2만 잔존. EIP `eipalloc-bf54f437`(121.78.39.184)는 detach 상태로 남아 수동 release 필요 |
| terminate 후 type 표기 변경 | describe-instances 보고 type이 `c4.large`→`z2.large`, `c5.2xlarge`→`z2.2xlarge`로 바뀜(카탈로그 실제 type 노출) |
| apply 결함 2건(.tf에 D7/D8 주석 반영) | D7: `aws_security_group`에 tags 지정 시 `InvalidParameterValue: Invalid format for tags`로 400 실패, 별도 create-tags는 exit 0이나 describe-vpcs Tags는 `[]`로 회귀 → 이 플랫폼은 태그 저장/조회 불가, node SG tags 제거. D8: `aws_instance`에 `associate_public_ip_address`(false 포함) 존재 시 `Network details contain unsupported params AssociatePublicIpAddress`로 400 실패, 속성 제거하고 `map_public_ip_on_launch=false` + `aws_eip.node`로 대체 |
| apply 결과 | 3회 apply로 31개 전부 생성(1차 18개 후 D7 실패, 2차 SG/rule 9개 후 D8 실패, 3차 4개) |
| NAT 실증 | NAT gateway 생성 34초 성공. w1(private)에서 `https://archive.ubuntu.com` 200 확인 → egress 실증(2026-09-10 기록의 "생성 검증은 아니다"는 갱신됨) |
| EIP/사설 IP | EIP 2개(NAT, m1) 할당 성공. `private_ip` 지정(10.20.0.10 / 10.20.1.10) 정상 반영. m1 공인 IP 121.78.39.241 |
| cloud-init | 두 노드 모두 done, Python 3.12.3 |
| Kubespray v2.30.0 cluster.yml | m1 ok=667 failed=0 ignored=4, w1 ok=417 failed=0. 노드 2대 Ready v1.34.3, containerd 2.2.1, m1 taint 없음(schedule_on_control_plane 동작 확인). ignored 4건은 최초 설치의 etcd --version / calicoctl get felixconfig·ippool·bgpconfig 조회 |
| Ansible 운영 메모 | non-TTY 파이프 실행 시 "Ansible requires blocking IO" 실패 → `</dev/null >log 2>&1`로 우회 |
| 후속(범위 밖) | local-path-provisioner v0.0.31 default SC, ingress-nginx NodePort 30080/30443 설치, Nullus Helm 배포 진행 중 — 결과는 배포 런북에 기록 |
| DNS 갱신 필요 | `nullus.io`/`auth.nullus.io`/`www.nullus.io`가 여전히 구 IP 121.78.39.184를 가리킴 → 121.78.39.241로 갱신 필요(TLS/SSO 선행조건) |

**미확인 잔여**: vCPU/RAM/디스크 quota 상한 자체(현재 사용량 20 vCPU/40GB/400GiB로 생성 성공했으므로 최소 그만큼은 보유). `root_block_device.delete_on_termination=false`인 신규 VM은 반납 시 볼륨이 남는다.

## 준비된 실행 환경 (로컬, git 제외)

- Kubespray: `generated/.kubespray`, v2.30.0, commit `f4ccdb5e72395eaf9f3444056ebd1a6625ddb89a`.
- Python 3.12 venv: `generated/ansible-venv`; Kubespray requirements 설치 완료.
- 로컬 `terraform.tfvars`에 실제 SSH 키 경로와 운영자 /32를 반영. 파일 내용/개인키는 커밋하지 않는다.
