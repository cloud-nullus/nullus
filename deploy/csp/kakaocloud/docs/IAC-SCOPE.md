# 카카오클라우드 OpenTofu IaC 범위 정의 (Phase 1 배포)

> 목표: 카카오클라우드 배포에서 OpenTofu 가 담당할 **VM / Kind / 스토리지** 구성 범위를 확정하고,
> Phase 1(첫 배포 마일스톤, 이하 "Page 1") 배포 필수 여부를 판단한다.
>
> 작성 가정:
> - **"회의록"** 별도 파일 부재 → IaC 경계 결정이 기록된 `BUILD-LOG.md` / `README.md` 를 결정 기록으로 채택.
> - **"Page 1"** 로드맵상 미정의 → `docs/70_전략/ROADMAP.md` 의 **Phase 1 Alpha(2026-03-30) 카카오클라우드 배포**로 해석.

---

## 1. 결정 기록의 OpenTofu / Terraform 언급 정리

| 출처 | 핵심 문장 |
|---|---|
| `docs/BUILD-LOG.md:12-14` | "카카오 클라우드에 **VM 2대**를 OpenTofu 로 생성", "OpenTofu 는 **인프라 프로비저닝까지만**, 이후는 **별도 스크립트가 SSH 로 호출**" |
| `docs/BUILD-LOG.md:108` | "OpenTofu 스캐폴딩 — **network/security/compute 3모듈** + 스크립트 (**loadbalancer / heavy-provisioner 제외**)" |
| `README.md:3,13-14` | "OpenTofu 로 VM 2대를 프로비저닝하고, 기존 `airgap/` 스크립트를 재사용하여 Nullus 플랫폼을 설치" |
| `ROADMAP.md:34-39` | Phase 3(InfraOps, 2027+)에 "클러스터 프로비저닝, 멀티 클러스터 관리, **IaC 통합**" 명시 → IaC 의 클러스터 영역 확장은 Phase 3 사안 |

**확정 경계**: OpenTofu = **인프라 레이어(L1)** 전담. 클러스터/플랫폼 설치(L2)는 `airgap/scripts/` + Helm 소유.

---

## 2. 최소 리소스 범위 정의 (Page 1 필수 = MUST)

OpenTofu 가 선언적으로 소유하는 최소 집합:

| 영역 | 리소스 | 모듈 | Page 1 | 비고 |
|---|---|---|---|---|
| **네트워크** | VPC `nullus-airgap` 172.16.0.0/16 + 서브넷 | `network` | MUST | tfvars `vpc_*` / `subnet_*` 로 제어 |
| **보안** | Security Group (ingress 22/80/443 + VPC 내부 ALL + ICMP, egress ALL) | `security` | MUST | registry(5001)·kind API(16443)는 미노출(127.0.0.1) |
| **인증키** | Keypair 생성 + `.pem` 0600 저장 | root | MUST | 계정 미등록 키 대응 |
| **VM(설치 타겟)** | airgap VM `t1i.2xlarge`(8c/32GB) | `compute` | MUST | kind+레지스트리+Nullus 호스트 |
| **VM(빌드)** | builder VM `t1i.xlarge`(4c/16GB) | `compute` | MUST(전환적) | amd64 번들 빌드 전용 — **빌드 후 파기 가능** |
| **스토리지** | 부트 볼륨 500GB ×2 (SSD) | `compute` | MUST | **별도 블록스토리지/PV 없음** — kind 는 부트디스크 local-path 사용 |
| **부트스트랩** | cloud-init (docker/git/make) | root | MUST | VM 내부 SW 준비 |

> **스토리지 결론**: Page 1 범위에서 별도 볼륨 리소스는 **불필요**. PV 는 클러스터 내부(local-path-storage)가 부트디스크에서 동적 프로비저닝 → IaC 비대상.
> **Kind 결론**: kind 클러스터는 OpenTofu **비대상**. `airgap/scripts/install.sh` 가 컨테이너로 생성(L2).

---

## 3. Phase 2 포함 / 제외 기준 (decision rule)

리소스를 IaC(OpenTofu)에 편입할지 판단하는 규칙:

- **기준 A (포함)** — 리소스가 **인프라 레이어**(VM·네트워크·스토리지·클러스터 엔드포인트·LB)이고, 선언적 멱등 관리가 드리프트 방지에 이득이면 → **OpenTofu 포함**.
- **기준 B (제외)** — 리소스가 **플랫폼/앱 레이어**(Helm 릴리스, 클러스터 내부 객체, 번들 빌드 절차)이면 → **스크립트 / Helm / GitOps 소유, IaC 제외**.
- **기준 C (승격)** — 카카오클라우드 **매니지드 서비스로 전환**(예: Kubernetes Engine, 매니지드 LB, Object Storage)되는 순간 해당 리소스는 IaC 포함 대상으로 **승격**.

### 기준 적용 결과

| 후보 리소스 | 현재 | 기준 | Phase 2 판정 |
|---|---|---|---|
| LoadBalancer / Ingress | systemd port-forward(40-expose) workaround | A+C | **편입 후보** — 매니지드 LB 도입 시 IaC |
| 별도 Block Storage / PV | 부트디스크 local-path | B | 제외 — 클러스터 내부 사안 |
| Kind → 매니지드 K8s 전환 | kind(스크립트) | C | **편입 후보** — 전환 시 cluster provisioning IaC |
| Helm 차트(Nullus/Keycloak/카탈로그) | scripts/Helm | B | 제외(영구) |
| 멀티클러스터 / IaC 통합 | 없음 | A | Phase 3 (ROADMAP 명시) |

---

## 4. Page 1(Phase 1 첫 배포) 필수 여부 결론

**결론: 필수(YES) — 단, 신규 IaC 개발은 불필요(현 스코프로 충족).**

근거:
1. OpenTofu 의 network/security/compute 산출물은 `airgap/scripts/` 설치 흐름의 **차단 의존성(blocking dependency)**.
   VPC/VM/SG 가 없으면 10-build → 20-transfer → 30-install 파이프라인 자체가 실행 불가.
2. 현 코드는 이미 **완성·검증**됨 — airgap/builder VM 2대 `active`, 실측 가동 확인.
3. 따라서 Page 1 = **현 3모듈 그대로 충족**. 추가 작업 없음.

### 권장 후속(차단 아님, optional)
- t1i 는 burstable 계열 → 상시 CI 부하 시 동일 스펙 비-burstable `m2a.2xlarge` 검토 (`terraform.tfvars.example` 주석 참고).
- builder VM 은 번들 빌드 후 `tofu destroy -target` 으로 파기 시 비용 절감(전환적 리소스).

---

## 요약

| 항목 | 결론 |
|---|---|
| OpenTofu 범위 | 인프라 L1(network/security/compute/keypair/cloud-init)만 |
| VM | airgap(필수) + builder(전환적 필수) |
| Kind | **비대상** (scripts/Helm 소유) |
| 스토리지 | 부트볼륨 500GB만, **별도 볼륨/PV 비대상** |
| Phase 2 기준 | A(인프라→포함)/B(플랫폼→제외)/C(매니지드 전환→승격) |
| Page 1 필수 | **YES, 그러나 현 스코프로 이미 충족 — 신규 IaC 불요** |
