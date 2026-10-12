# Zadara Cloud PoC 배포 재계획

작성일: 2026-09-09 · 갱신: 2026-09-10 (G0 실측 반영) · 검토 기준: 저장소 `2649693` · 상태: **G1/G2/G3 2026-09-13 실행 완료** — 결과는 [설치 로그](../../deploy/csp/zadara/INSTALL_LOG_2026-09-13.md), 현재 아키텍처는 [README](../../deploy/csp/zadara/README.md), 절차는 [INSTALL.md](../../deploy/csp/zadara/INSTALL.md)

**온라인 PoC, m1(master) + w1(worker)의 단일 클러스터**를 기준으로 KakaoCloud OpenTofu 구성을 Zadara에 맞게 전환한다. airgap·builder·번들 전송은 제외한다.

2026-09-10 기존 4대를 실측했다. 합계는 **20 vCPU / RAM 38.8GiB(공칭 40GB) / disk 400GiB**로 기존 문서 합산값과 일치한다. m1은 4 vCPU/8GB/100GB(`z2.xlarge`), w1은 16 vCPU/32GB/300GB(`z2.4xlarge`)로 배분한다. **다만 이 값은 현재 사용량이지 테넌트 quota 한도가 아니다** — 한도는 API로도 DryRun으로도 읽히지 않아 테넌트 관리자 확인이 남아 있다. 실측 내역과 검증 방법은 [OpenTofu 설계 §1](zadara_opentofu_design.md).

인프라 생성부터의 상세 설계·구성 비교·전환 파일 목록은 [Zadara OpenTofu 설계](zadara_opentofu_design.md)를 따른다. 아래는 그 이후 기능 배포·복구 검증 계획이다. 이번 검토에서는 원격 접속·배포·DNS/보안 그룹 변경을 수행하지 않았다. 기존 4대 환경의 기록은 참고 자료이며 현재 자원 확인을 대신하지 않는다.

## 1. 기존 문서 검토 결과

| 우선순위 | 근거 | 문제와 반영 방안 |
|---|---|---|
| P0 | `deploy/csp/zadara/README.md` §4.1, `deploy/helm/nullus/templates/migration-job.yaml` | README는 수동 마이그레이션을 안내하지만 현재 차트는 `post-install,pre-upgrade` Job을 실행한다. 첫 설치와 업그레이드를 나눠 검증하고 Job 완료·스키마 버전·실제 API 응답을 함께 확인한다. |
| P0 | README §4, `deploy/helm/nullus/values.yaml` secrets/postgresql | 기존 명령의 `secrets.dbPassword`만 변경하면 번들 DB 비밀번호는 바뀌지 않는다. `postgresql.auth.password` 및 Keycloak/Keycloak DB의 실제 Secret 출처를 확인하고 기본 비밀번호를 제거한다. 기존 PVC의 DB 비밀번호 변경은 별도 회전 절차가 필요하다. |
| P0 | README §7, `values-zadara.yaml`, `setup-tls.sh` | 문서는 TLS 미구성이라지만 값 파일은 `nullus.io`·`auth.nullus.io` HTTPS를 전제한다. DNS→443→Ingress→Keycloak→JWT 검증 경로를 배포 선행 조건으로 통합한다. |
| P0 | `.github/workflows/cd.yml` deploy-zadara | main·태그·수동 실행이 같은 `platform/nullus`에 배포된다. 검증 중 덮어쓰기를 막을 환경 잠금과 고정 리비전이 필요하다. |
| P1 | 구축 가이드 §1·§5, README §1 | ~~OS 가정은 22.04, 과거 기록은 24.04.3이다.~~ **2026-09-10 실측: 4대 모두 Ubuntu 24.04.3 LTS / kernel 6.8.0-90 / containerd 2.2.1 / Kubernetes v1.34.3.** 남은 것은 CNI 모드와 Pod/Service CIDR 수집, 재구축/유지 결정이다. |
| P1 | README §2.3, 공식 Zadara 자료 | 이 환경의 LB 미연동을 Zadara 제품의 미지원으로 단정하면 안 된다. 테넌트의 NLB/API/IAM 제공 여부를 먼저 확인한다. |
| P1 | README §2.3, 공식 Kubernetes 공지 | ingress-nginx는 2026년 3월 유지보수 종료 대상이다. 신규 배포의 기본 선택에서 제외하고, 기존 설치는 교체 전 제한된 검증 용도로만 다룬다. |
| P1 | README §7, `templates/deployment.yaml` | 워커 1대라고 replica 2개가 반드시 Pending인 것은 아니다. API/Web 템플릿에 강제 anti-affinity는 없다. replica 1은 용량 절약이며 HA 보장이 없다는 의미로 정정한다. |
| P1 | 구축 가이드 §6·§10 | debug의 `var=kubeconfig_localhost,helm_enabled`는 두 변수를 개별 검증하지 않는다. 각각 조회한다. 실패 후 reset을 일반 복구법으로 쓰지 않고 데이터·로그 보존부터 수행한다. |

## 2. 범위와 배치 결정

| 결정 | 이유 | 비용/한계 | 전환 조건 |
|---|---|---|---|
| D1: m1+w1에 기존 VM 합계 배분 | 큰 worker에서 다양한 기능·스택 테스트 | control plane/worker 각각 단일 장애점. **기존 클러스터 2개(`platform`+`develop`)를 함께 반납해야 한다** | type 확정(`z2.xlarge`/`z2.4xlarge`). quota 한도는 관리자 확인 대기 |
| D2: airgap/builder 제외, 온라인 설치 | 빌드·번들 전송용 VM 자원 제거 | 이미지/패키지 egress 필요 | 폐쇄망 검증은 이번 범위 밖 |
| D3: local-path 임시 사용 | CSI 도입 전 최소 기능 검증 | 노드 손실 시 데이터 소실, worker 추가로 PVC가 이동하지 않음 | 데이터 보존·노드 간 이동 요구 시 CSI/이관 검증 |
| D4: 배포 리비전 고정 | 코드·차트·SQL·이미지 일치 | 이미지 게시와 CD 통제 확인 필요 | 검증 후 명시적 업데이트 |
| D5: 지원되는 Ingress 구현 선정 | 종료된 컨트롤러 의존 해소 | nginx 전용 annotation·스택 코드 변경 가능 | 호환성 검증 전 공개 서비스 전환 보류 |

| 위치 | 기본안 배치 |
|---|---|
| m1 | 4 vCPU/8GB/100GB, control plane + etcd, 관리 SSH 진입점, **일반 워크로드도 배치(taint 없음)** |
| w1 | 16 vCPU/32GB/300GB, Nullus/Keycloak/DB/Ingress와 테스트 스택 |
| 클러스터 외부 | 백업 보관소와 암호화 키 보관. 별도 VM 신설을 전제하지 않음 |

기존 자원은 `platform`(node-10 control-plane + node-11)과 `develop`(node-20/21) 두 클러스터이며, m1+w1 전환은 둘 다 반납하는 것을 뜻한다. `develop` 클러스터에서 진행 중인 작업과 보존 대상 데이터를 G0에서 먼저 확정한다.

**m1의 control-plane taint를 제거하고 m1에도 일반 애플리케이션을 배치한다.** 노드 2대에서 m1의 4 vCPU/8GB를 놀리지 않기 위한 선택이며, Kubespray inventory에서 m1을 `kube_node`에 함께 넣어 구현한다. 대신 control plane과 워크로드가 자원을 공유하므로 m1의 requests/limits와 etcd 지연을 함께 관찰한다. 플랫폼 requests와 테스트 namespace ResourceQuota/LimitRange를 정하고 실제 메모리·OOM·디스크 및 설치 피크로 동시 테스트 범위를 조정한다. VM에 할당한 전체 자원을 Pod 사용량으로 소진하도록 설정하지 않는다. 기존 자원이 quota를 점유하므로 백업·사양 변경/import/반납·재생성 순서를 OpenTofu 설계에서 먼저 검토한다.

## 3. 네트워크·스토리지 설계

```text
운영자 ── SSH(제한된 소스) ── m1 ── worker w1
브라우저 ── HTTPS 진입점 ── Ingress ── Nullus / Keycloak / 검증 스택
플랫폼·스택 ── 온라인 이미지/패키지 저장소
백업 작업 ── TLS ── 클러스터 외부 보관소
```

- **웹 진입점:** 테넌트 NLB 사용 가능 여부·비용·health check·보안 그룹부터 확인한다. 가능하면 NLB→NodePort 연결을 먼저 검증한다. Kubernetes `Service: LoadBalancer` 자동화는 CCM/controller 설치·권한 검증을 별도로 거쳐야 한다.
- **대체 경로:** NLB를 쓸 수 없으면 m1 443→NodePort 포워딩을 한시적으로 활용한다. 재부팅 후 규칙 복원, 소스 제한, 내부 Pod에서 공개 FQDN으로 되돌아오는 경로를 검증한다. 동일 클러스터의 w1에 있는 Ingress로 전달되는 경로를 검증한다.
- **방화벽:** SSH는 운영자→bastion 및 bastion→노드, API는 관리 경로와 필요한 플랫폼 내부 접근에 한정한다. etcd는 클러스터 내부 구성원, kubelet/CNI는 실제 모드에 필요한 노드 간 트래픽만 허용한다. 전체 VPC·전체 NodePort 대역을 일괄 개방하지 않는다. 외부 DNS/NTP/패키지·이미지 저장소/ACME egress도 확인한다.
- **주소:** 노드/VPC CIDR와 선택한 클러스터의 Pod/Service CIDR를 기록한다. 신규 구축은 상호 비중복 대역을 택한다. 기존 오버레이가 같더라도 즉시 재구축하지 않고 실제 API·웹 경로 충돌 여부부터 검증한다.
- **Ingress 교체:** 지원 중인 구현 후보를 비교하되 선정 조건은 기존 Ingress API, TLS Secret, WebSocket, 장시간 요청, www redirect, 스택별 SSO annotation의 동작이다. chart의 className만 바꾸는 것으로 완료 처리하지 않는다. Gateway API 전환은 별도 범위로 산정한다.
- **CSI:** Zadara 공식 EKS-D 예제는 AWS CCM/EBS CSI/LB Controller 활용 경로를 제공한다. 이를 Kubespray 환경에 바로 적용할 수 있다고 가정하지 않는다. API endpoint, IAM, providerID, 드라이버/Kubernetes 호환성과 지원 범위를 확인하고 PVC 생성→쓰기→Pod 재시작→확장→삭제 정책을 검증한다. worker 간 detach/attach는 추가 worker가 있어야 검증 가능하다.
- **TLS:** platform은 현재 값 파일의 `nullus.io`/`auth.nullus.io`를 기준으로 삼되 DNS 소유·현재 사용 여부부터 확인한다. 테스트 도구의 호스트 매핑표를 만든다. `*.nullus.io`는 `grafana.develop.nullus.io`를 덮지 않으므로 인증서 SAN을 명시한다. TLS Secret은 네임스페이스/클러스터마다 필요하고 DNS-01 발급·갱신 책임자를 정한다.

## 4. 실행 단계와 통과 기준

예상 공수는 담당자 작업일 기준이며 클라우드 권한·DNS 변경·증설 대기 시간은 제외한다. G0 뒤 G1과 G2의 조사 작업은 병행할 수 있다. 실제 변경은 대상별로 직렬화한다.

| 단계 | 담당 역할 / 예상 공수 | 작업 | 다음 단계 진입 조건과 증거 |
|---|---|---|---|
| G0 현황 확정 ◑ | 인프라 + 배포 담당 / 0.5~1일 | **완료**: 노드 사양·OS·k8s 버전·클러스터 구성(`platform`/`develop`)·VPC/subnet/SG/EIP·볼륨. **남음**: kube-system UID, CNI 모드, Pod/Service CIDR, Helm/PVC/TLS/DNS/배포 이미지와 자동 CD 상태, quota 한도, 기존 데이터 보존 여부 | 비밀값을 뺀 현황표, 신규/import 결정, OpenTofu I0~I3 결과, 고정할 리비전 및 변경 시간대 기록 |
| G1 인프라 기반 | 인프라 / 1~2일 | 노드·DNS·egress·스토리지 검증. OpenTofu로 생성/편입한 m1/w1에 Kubespray 구성. OS/Python/Ansible/Kubernetes 버전 조합 고정 | 대상 API readyz 정상, 노드 Ready, **m1에 NoSchedule taint 없고 두 노드에 Pod 스케줄**, DNS 및 Pod 통신 성공, PVC 쓰기/재시작 보존, 용량 기준 통과 |
| G2 진입점·인증 기반 | 네트워크 + 배포 담당 / 1~3일 | LB/대체 경로 확정, 지원 Ingress 후보 호환성 검증, DNS와 cert-manager 구성, DB/Keycloak 비밀번호 및 키 준비 | 브라우저와 API Pod/테스트 Pod에서 인증서 검증 성공, 각 FQDN의 목적지 확인, 발급·갱신 시험 기록 |
| G3 Nullus 배포·SSO | 앱 + 배포 담당 / 1~2일 | 차트/이미지 고정, 의존 차트 잠금, Helm 배포와 마이그레이션 확인, realm/client/audience/redirect 구성 | Job Complete, 스키마 dirty=false 및 대상 버전 일치, 템플릿 목록 API 정상, 브라우저 로그인/로그아웃·새로고침·권한 거부 정상 |
| G4 스택 검증 | 앱 + 인프라 / 1~2일 | 같은 클러스터의 테스트 namespace에 작은 스택부터 설치, RBAC·StorageClass·도구 FQDN·자원 한도 확인 | 플랫폼 namespace 보존, PVC·HTTPS·도구 SSO·상태 조회·삭제 정책 정상 |
| G5 복구·인수 | 운영 + 검증 담당 / 1일 | 외부 백업, 별도 네임스페이스/검증 환경 복원, 배포 업데이트·실패 대응, 알림·정리 절차 확인 | 복원 후 로그인·등록 kubeconfig 복호화·데이터 조회 성공, RPO/RTO 실측, 결과표 및 잔여 위험 인수 |

위 공수는 기존 배포 경로의 초기 추정이다. m1+w1 전환과 OpenTofu provider 검증/구현 공수는 별도이며 I0 결과로 재산정한다. CSI 본격 도입, Ingress 관련 다수 스택 코드 수정, HA 구축이 필요하면 G0/G2 결과로 별도 산정한다.

### G3 배포 시 지켜야 할 순서

1. 현재 Helm revision, DB 스키마 버전, 이미지 식별자와 백업을 기록한다. `main` 자동 배포와 수동 배포가 겹치지 않게 운영 통제를 확정한다. 현재 CD는 realm 전체 생성 대신 테마 적용만 실행하므로 초기 구성 경로를 대신하지 못한다.
2. 검증된 commit/tag, 차트 버전, API/Web 이미지 태그와 실제 digest를 기록한다. 로컬 차트는 `0.5.0`이지만 레지스트리 게시·현재 배포 여부는 미확인이다. chart lock을 이용한 dependency build와 모든 보조 이미지의 pull 가능성도 확인한다.
3. 번들 PostgreSQL·Keycloak·Keycloak DB 비밀번호, 정확히 32바이트인 `secrets.encryptionKey`를 안전하게 주입한다. 기존 암호화 키는 재배포마다 생성하지 않는다. Secret을 로그나 검토용 렌더 결과에 남기지 않는다.
4. 릴리스/네임스페이스는 기존 프록시 구성을 고려해 `nullus`, context는 `platform`을 명시한다. 첫 설치는 PostgreSQL 생성 후 `post-install` 마이그레이션, 업그레이드는 기존 DB에 `pre-upgrade` 마이그레이션이 실행됨을 확인한다. Ready만으로 SQL 적용을 판단하지 않는다.
5. Keycloak Ready 후 `setup-keycloak-realm.sh`의 전체 구성 경로를 사용한다. 실제 사용자·기본 계정·비밀번호 변경 동작을 사전 검토하고, `nullus-app`, issuer, audience, redirect URI/web origin, 조직/역할 매핑을 맞춘다.
6. `/config.js`, OIDC discovery/JWKS, 브라우저 로그인, 인증된 API와 권한 없는 요청을 검증한다. 브라우저 성공과 API Pod의 issuer 도달성은 별도 검증한다.

### G4 테스트 범위의 한계

플랫폼과 테스트 스택은 같은 worker에서 동작한다. namespace 분리와 권한/자원 정책을 검증하되, 외부 클러스터 등록·worker 간 재배치·HA를 검증한 것으로 보고하지 않는다. 추가 노드가 필요한 시험은 동일 지원량 내 자원 재분배와 데이터 이관을 별도 계획한다.

## 5. 백업·실패 대응·운영 전환

- 백업 대상은 Nullus DB, Keycloak DB, 암호화 키, 필요한 Kubernetes/Helm 설정, 스택 데이터다. etcd snapshot은 클러스터 상태 복구용이며 DB/PV 애플리케이션 백업을 대체하지 않는다.
- 차트의 `config.backup.enabled` 기본값은 false다. 외부 S3 호환 저장소, Keycloak DB 연결, encryptionKey와 다른 봉인 키를 구성하고 실제 산출물·복원을 검증한 뒤 활성화 완료로 기록한다.
- 초기 제안 목표는 RPO 24시간, RTO 4시간이다. 별도 환경에서 복원 시간을 측정하기 전에는 보장값이 아니다. 백업 성공 알림뿐 아니라 실패·만료 인증서·디스크 부족도 확인한다.
- 마이그레이션 실패는 새 배포 진행을 중단하고 Job 로그/DB 상태를 보존한다. dirty 상태를 무조건 force 해제하지 않는다. Helm rollback은 DB 스키마를 되돌리지 않으므로 이전 코드와 호환성을 확인하거나 DB 복구 계획을 실행한다.
- CNI/노드 실패는 이벤트·로그·인벤토리·백업부터 수집한다. 데이터가 있는 클러스터에서 `reset.yml`을 재시도 수단으로 사용하지 않는다. 초기화는 별도 폐기 결정 후 진행한다.
- 종료 시 검증 namespace/PVC, LB/공인 IP/볼륨, DNS, SG, 임시 계정·키를 소유자별로 정리한다. 보존할 백업과 삭제할 자원을 구분한다.
- 운영 전환은 별도 승인 기준이다: 클러스터별 control plane/etcd 3대, 워크로드 분산 가능한 worker 수, bastion 분리, 검증된 CSI/백업 복원, 지원 버전, 고정 릴리스 배포 및 복구 절차를 설계한다. 단일 worker에서 replica만 늘려 HA로 취급하지 않는다.

## 6. 착수 시 확정할 입력값과 후속 변경

| 입력값 | 미확정 시 기본 처리 |
|---|---|
| PoC 목적·참여자·기한 | 내부 기능 검증, 운영자 제한 접근 |
| 기존 4대의 실제 합계/quota/type과 데이터 | 기존 자원 보존, 신규 생성/import 결정 후 진행 |
| Zadara 테넌트 버전/API/IAM/NLB/CSI 지원·요금 | 제품 지원과 테넌트 제공 여부를 구분해 운영자/사업자 확인 |
| DNS 권한과 공개 호스트/백업 목적지 | 기존 값은 후보로만 사용, 소유·사용 확인 후 변경 |
| 첫 스택과 자원 요구량 | 가장 작은 사용 가능 템플릿 1개 선정 후 렌더 자원 합산 |
| 배포 리비전·CD 통제 방법 | SHA/이미지 식별자를 고정하고 동시 배포 차단 방법 확정 |

후속 구현은 우선 Zadara OpenTofu 설계의 I0~I2를 진행한다. 플랫폼 배포 단계의 변경 범위는 `deploy/csp/zadara/README.md`의 현황/배포 순서 정정, `values-zadara.yaml`의 비밀번호·Ingress·자원 설정 검증, Zadara CD의 환경 분리/동시성 통제, TLS/realm/복구 런북 통합이다. 이번 변경에서는 실행 스크립트·차트·CI를 수정하지 않았다.

## 7. 근거 자료

- [기존 클러스터 구축 가이드](zadara_cloud_poc.md), [기존 배포 런북](../../deploy/csp/zadara/README.md), [Zadara 값 파일](../../deploy/csp/zadara/values-zadara.yaml)
- [Helm 차트](../../deploy/helm/nullus/Chart.yaml), [마이그레이션 Job](../../deploy/helm/nullus/templates/migration-job.yaml), [CD 워크플로](../../.github/workflows/cd.yml)
- [Kubespray v2.30.0 릴리스](https://github.com/kubernetes-sigs/kubespray/releases/tag/v2.30.0): 기존 설치 기준. 재구축 버전은 지원 조합 확인 후 고정한다.
- [Kubernetes ingress-nginx 종료 공지](https://kubernetes.io/blog/2026/01/29/ingress-nginx-statement/): 2026년 3월 유지보수 종료 및 이전 필요성.
- [Zadara 공식 Kubernetes 예제](https://github.com/zadarastorage/zadara-examples/blob/main/k8s/eksd/docs/README.md): AWS 계열 CCM/CSI/LB 도구 활용 및 Zadara endpoint 조정 안내. Kubespray 호환성 증거는 아님.
- [zCompute Load Balancer 가이드](https://guides.zadarastorage.com/cs-lb-guide/latest/setup-lb.html): LB/target group 기능. 현 테넌트 제공 여부는 별도 확인.
