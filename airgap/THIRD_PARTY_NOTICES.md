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
| **MinIO** (`minio/*`, minio 차트) | AGPL-3.0 | 네트워크 제공 시 소스 공개 의무 가능. 상용은 별도 상용 라이선스 검토. |
| **Grafana** (grafana 차트/이미지) | AGPL-3.0 | 〃 (Grafana 8.x 이후 AGPL) |
| **Loki** (loki 차트) | AGPL-3.0 | 〃 |
| **GitLab** (`registry.gitlab.com/*` 15종, gitlab 차트) | GitLab 라이선스(MIT/EE 혼재) | CNG/EE 컴포넌트는 소스가용·상용 조건 혼재 → **재배포 가부 별도 확인 필수**. |
| **OpenBao** (`openbao/*`) | MPL-2.0 | 파일 단위 copyleft. 수정 시 해당 파일 공개. |
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
| Bitnami 차트/패키징(`docker.io/bitnami/*`) | Apache-2.0(패키징) — **내장 소프트웨어는 각자 라이선스** |

---

## 3. 카탈로그 차트 목록 (`airgap/helm/charts-catalog/`)

cert-manager v1.16.3 · metrics-server 3.12.2 · minio 5.4.0 · gitlab 8.7.2 · gitlab-runner 0.72.0 · argo-cd 7.7.16 · kube-prometheus-stack 69.3.0 · grafana 8.9.0 · loki 2.10.3 · opensearch 2.22.0 · opentelemetry-collector 0.75.0 · keycloak 24.4.5 · harbor 1.15.0 · gateway-helm v1.4.3 · postgresql 16.7.21

전체 이미지 목록은 `airgap/images/images.txt`(71개), digest 는 `airgap/bundle/MANIFEST.txt` 참조.

---

## 4. 후속 (권장)
- [x] **SBOM 생성**을 빌드 파이프라인에 추가 → `airgap/scripts/pre/generate-sbom.sh`(syft), `pre-build.sh` 5/6 단계. 출력 `bundle/sbom/`(번들 포함).
- [ ] 각 구성요소의 **원본 LICENSE 전문 수집** → `airgap/licenses/<component>/LICENSE` 동봉.
- [ ] §1 항목(AGPL/GitLab/OpenBao 등) 재배포 가부 **법무 확인** 후 본 문서에 결론 반영.
