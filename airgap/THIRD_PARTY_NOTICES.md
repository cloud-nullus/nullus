# 서드파티 라이선스 고지 (Third-Party Notices)

> 이 에어갭 번들(`airgap/`)은 다수의 오픈소스/서드파티 컨테이너 이미지·Helm 차트를 **재배포 목적으로 포함**한다.
> 본 문서는 번들 구성요소의 **라이선스 인벤토리**다.
>
> ⚠️ **면책**: 본 문서는 법적 조언이 아니며, 라이선스 표기는 상위 프로젝트의 공개 정보를 바탕으로 한 **참고용**이다.
> 번들을 **외부에 재배포·상용 제공하기 전 반드시 법무 검토**를 거치고, 각 구성요소의 원본 LICENSE 전문을 확인할 것.
> 정확한 구성요소·버전 목록은 SBOM 생성으로 자동화하길 권장한다 (예: `syft packages dir:airgap/bundle -o spdx-json`).

---

## 1. 재배포 시 특별 주의가 필요한 구성요소 ⚠️

아래 항목은 **copyleft / source-available / 상용** 성격이라 번들 재배포·SaaS 제공 시 의무사항이 따를 수 있다. 우선 검토 대상.

| 구성요소 | 라이선스(참고) | 주의점 |
|---|---|---|
| **SILO** (`pgsty/silo`, `pgsty/mc`, minio 차트) | AGPL-3.0 | MinIO 서버의 커뮤니티 포크(MinIO Inc. 와 무관). 업스트림 MinIO 저작권 고지를 함께 담는다. **포크로 바꾼 이유는 라이선스가 아니라 배포다** — MinIO 가 Docker Hub 에 이어 quay.io 도 익명 pull 을 막아 받을 수 있는 출처가 없어졌다(2026-10-17 실측). AGPL 의무는 업스트림과 동일하게 남는다: 네트워크 제공 시 소스 공개 의무 가능, 상용은 별도 검토. |
| **Grafana** (grafana 차트/이미지) | AGPL-3.0 | 〃 (Grafana 8.x 이후 AGPL) |
| **Loki** (loki 차트) | AGPL-3.0 | 〃 |
| **GitLab** (`registry.gitlab.com/*` 15종, gitlab 차트) | GitLab 라이선스(MIT/EE 혼재) | CNG/EE 컴포넌트는 소스가용·상용 조건 혼재 → **재배포 가부 별도 확인 필수**. |
| **OpenBao** (`openbao/*`) | MPL-2.0 | 파일 단위 copyleft. 수정 시 해당 파일 공개. |
| **Sonatype Nexus Repository** (`sonatype/nexus3:3.64.0`) | EPL-1.0(코어: maven · raw · apt) + **Sonatype 독점 EULA**(Docker · npm · PyPI · Helm 등 포맷 플러그인 — 이미지 안 Nexus 설치 디렉터리의 `OSS-LICENSE.txt`) | **순수 오픈소스가 아니다.** 포맷 플러그인 EULA 는 내부 업무 용도로만 허락하고, 재배포는 Sonatype 저작권 고지와 EULA 사본을 함께 둘 때만 허용한다 — 둘 다 이미지 안에 있으므로 이미지를 고치지 않고 그대로 싣는다(`NOTICE.txt` 의 Sencha Ext JS GPLv3 FLOSS 예외 문구 포함). 판매 · 재판매 · 임대(iv)와 제3자를 위한 사용 — 서비스 뷰로 · ASP · 호스팅(vii) — 을 금지하고, Sonatype 이 서면 통지로 언제든 철회할 수 있다. EPL 코어를 재배포할 때는 보증 부인 · 소스 위치(`github.com/sonatype/nexus-public` 태그 `release-3.64.0-04`) 안내 · 상업 배포자 면책(EPL §4)이 따른다. **3.77.0 부터는 Community Edition EULA** 라 제품 내장(xiv) · 재배포(v) · 제3자 사용(vii) 자체가 금지되고 사용량 한도가 강제된다 — `NexusAppVersion` 은 그 아래로 고정한다(`TestNexusAppVersion_StaysBeforeCommunityEditionEULA`). 3.64.0 은 Sonatype 지원이 끝났고(2025-07-09 일몰) 미패치 취약점이 있다: CVE-2024-4956 비인증 경로 탐색(7.5, 3.68.1 에서 수정), CVE-2026-3199 인증 사용자 RCE(9.4, CE 시대인 3.91.0 에서만 수정). |
| **busybox** (base) | GPL-2.0 | 표준 베이스 이미지(보편 재배포). 수정 배포 시 소스 의무. |
| **Redis**(Bitnami 포함 시) | 버전별(BSD/RSALv2/SSPL) | 7.4+ 는 비-OSI. 포함 여부·버전 확인 필요. |

> 위 라이선스 분류는 **확인 필요(참고용)** 다. 특히 GitLab·Redis 는 버전/에디션에 따라 조건이 갈리므로 원본 확인.

---

## 2. 주요 Apache-2.0 / 허용형(permissive) 구성요소

아래는 일반적으로 Apache-2.0(또는 동등 허용형)으로, 고지·라이선스 사본 동봉 외 재배포 제약이 낮다. (참고용 — 원본 확인 권장)

| 구성요소 | 라이선스(참고) |
|---|---|
| Kubernetes / kind / kubectl / kube-state-metrics / metrics-server | Apache-2.0 |
| Helm | Apache-2.0 |
| Distribution(`registry:2`) | Apache-2.0 |
| cert-manager (Jetstack) | Apache-2.0 |
| Argo CD | Apache-2.0 |
| Prometheus / Alertmanager / node-exporter / prometheus-operator (kube-prometheus-stack) | Apache-2.0 |
| Keycloak | Apache-2.0 |
| Harbor | Apache-2.0 |
| OpenSearch | Apache-2.0 |
| OpenTelemetry Collector | Apache-2.0 |
| Envoy Gateway (gateway-helm) | Apache-2.0 |
| GitLab Runner | MIT |
| PostgreSQL (소프트웨어) | PostgreSQL License(허용형) |
| Sonatype `nexus-repository-manager` 차트 64.2.0 | Apache-2.0 — 차트만. 2023-10-24 부터 지원 중단(deprecated). 이미지는 §1 |
| Bitnami 차트/패키징(`docker.io/bitnami/*`) | Apache-2.0(패키징) — **내장 소프트웨어는 각자 라이선스** |

---

## 3. 카탈로그 차트 목록 (`airgap/helm/charts-catalog/`)

cert-manager v1.16.3 · metrics-server 3.12.2 · minio 5.4.0 · gitlab 8.7.2 · gitlab-runner 0.72.0 · argo-cd 7.7.16 · kube-prometheus-stack 69.3.0 · grafana 8.9.0 · loki 2.10.3 · opensearch 2.22.0 · opentelemetry-collector 0.75.0 · keycloak 24.4.5 · harbor 1.15.0 · nexus-repository-manager 64.2.0 · gateway-helm v1.4.3 · postgresql 16.7.21

전체 이미지 목록은 `airgap/images/images.txt`(71개), digest 는 `airgap/bundle/MANIFEST.txt` 참조.

---

## 4. 후속 (권장)
- [x] **SBOM 생성**을 빌드 파이프라인에 추가 → `airgap/scripts/pre/generate-sbom.sh`(syft), `pre-build.sh` 5/6 단계. 출력 `bundle/sbom/`(번들 포함).
- [ ] 각 구성요소의 **원본 LICENSE 전문 수집** → `airgap/licenses/<component>/LICENSE` 동봉.
- [ ] §1 항목(AGPL/GitLab/OpenBao/Nexus 등) 재배포 가부 **법무 확인** 후 본 문서에 결론 반영.
- [ ] **Nexus 방향 결정** — 3.64.0 은 지원 종료 · 미패치 취약점이 있고, 패치된 버전(3.91.0+)은 CE EULA 라 내장할 수 없다. 선택지: CE 이전 마지막 판(3.76.x)으로 올려 비인증 취약점만 막기 / 설치 선택지를 내리고 고객이 가진 Nexus 에 연결만 하기 / Sonatype 과 별도 계약.
