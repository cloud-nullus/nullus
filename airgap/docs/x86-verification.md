# x86(amd64) 검증 포인트 & 운영 정리

> 대상: `airgap/` 에어갭 번들 — 빌드(온라인)부터 설치(오프라인)까지.
> 목적: **x86_64/amd64 타겟 배포**에서 반드시 확인할 검증 포인트와 번들/문서 인벤토리를 한곳에 정리한다.
> 근거: 실제 카카오클라우드 2-VM 배포(`deploy/csp/kakaocloud/docs/BUILD-LOG.md`)에서 확인된 amd64 이슈를 반영.

---

## 1. airgap 스크립트 역할

### 1-1. 빌드 단계 (온라인 머신 — `pre-build.sh` 오케스트레이션)
| 스크립트 | 역할 |
|---|---|
| `pre-build.sh` | 마스터 오케스트레이터 — 00/01/02 + 바이너리/차트 fetch + 번들 패키징 호출 |
| `scripts/00-generate-images.sh` | helm template 로 `images/images.txt` 재생성 |
| `scripts/01-pull-images.sh` | 이미지 pull (crane 또는 docker, **platform-aware**) |
| `scripts/02-save-bundle.sh` | 이미지 → `bundle/images.tar.gz` + sha256 + MANIFEST |
| `scripts/pre/pull-binaries.sh` | `$PLATFORMS` 별 kind/kubectl/helm → `bin/<platform>/` |
| `scripts/pre/pull-charts-catalog.sh` | DevOps 스택 차트 15종 다운로드 |
| `scripts/pre/package-bundle.sh` | `dist/nullus-airgap-bundle-<date>.tar.gz` 조립 |

### 1-2. 설치 단계 (오프라인 머신 — `install.sh` 순차 실행)
| 순서 | 스크립트 | 역할 |
|---|---|---|
| 1 | `03-load-bundle.sh` | `docker load bundle/images.tar.gz` |
| 2 | `10-setup-registry.sh` | `registry:2` 를 `localhost:5001` 로 기동 |
| 3 | `11-create-cluster.sh` | kind 클러스터 `nullus-airgap`(v1.30.0) + containerd mirror |
| 4 | `12-push-to-registry.sh` | 71개 이미지 재태깅 → `localhost:5001` push |
| 5 | `28-push-charts-oci.sh` | 카탈로그 차트 OCI 레지스트리 push |
| 6 | `21-install-nullus.sh` | `helm install nullus` + PostgreSQL |
| 7 | `26-migrate-db.sh` | PostgreSQL 스키마 초기화(필수) |
| 8 | `22-install-platform-stack.sh` | Keycloak + (옵션) kube-prometheus-stack |
| 9 | `23-setup-gateway.sh` | Envoy Gateway 외부 노출(CRD 존재 시) |
| 10 | `99-verify.sh` + `13-set-config.sh` | 클러스터 준비 검증 + kubeconfig 정리 |

### 1-3. 옵션 스크립트
`27-install-stacks.sh`(DevOps 카탈로그: ArgoCD/Harbor/MinIO/GitLab/Prometheus) · `24-register-hosts.sh`(/etc/hosts) · `25-port-forward.sh`(영구 포트포워드)

---

## 2. x86(amd64) 검증 포인트 ✅

**핵심 원칙: 번들의 이미지·바이너리 아키텍처와 타겟 VM 아키텍처가 반드시 일치해야 한다.** Apple Silicon(arm64) 머신에서 만든 기본 번들은 x86 VM 에서 동작하지 않는다.

### 2-1. 빌드 시 (온라인 머신)
- [ ] **타겟 플랫폼 명시**: amd64 타겟이면 `TARGET_PLATFORM=linux/amd64 PLATFORMS=linux-amd64` 로 빌드. (`scripts/01-pull-images.sh:61-111`, `scripts/pre/pull-binaries.sh:32`)
  - ⚠️ arm Mac 에서 기본 빌드 시 `_host_arch()` 가 arm64 로 굳음 → **x86 VM 부적합**. (BUILD-LOG TS-1)
  - 권장: **네이티브 amd64 빌더 VM 에서 빌드** (크로스빌드보다 안정적).
- [ ] **바이너리 플랫폼 포함**: `bin/linux-amd64/` 에 kind/kubectl/helm 존재 확인.
- [ ] **docker 멀티아치 손상 방지**(docker 24+/29): 빌드 전 `/etc/docker/daemon.json` 에 `{"features":{"containerd-snapshotter":false}}` → overlay2 스토어. (BUILD-LOG TS-4)
- [ ] **digest 고정 이미지 재태깅**: `:tag@sha256:` 형식 이미지는 `docker pull` 후 `:tag` 로 재태깅돼야 save 누락 없음. (BUILD-LOG TS-6)

### 2-2. 번들 검증 (전송 직후)
- [ ] `sha256sum -c bundle/images.tar.gz.sha256` 무결성 확인.
- [ ] **이미지 아키텍처 일괄 검증**:
  ```sh
  while read -r img; do echo "$img"; done < images/images.txt   # 목록 확인
  # 로드 후: docker image inspect <img> --format '{{.Architecture}}'  → 전부 amd64 여야 함
  ```
- [ ] 번들 `bin/` 아키텍처: `file bin/linux-amd64/kubectl` → `x86-64`.

### 2-3. 설치 시 (오프라인 머신)
- [ ] `install.sh` 의 `detect_platform()` 가 `x86_64→amd64` 로 인식하는지. (`install.sh:80-99`)
  - 미지원 아키텍처면 즉시 종료. `$BIN_DIR/$PLATFORM/` 부재 시 "이 번들에는 $PLATFORM 바이너리가 없습니다".
- [ ] kind 노드 런타임 아키텍처: `kubectl get nodes -o wide` → `kindest/node:v1.30.0` 가 x86_64.
- [ ] 파드 ImagePullBackOff/`exec format error` 부재(아키텍처 불일치 시그널).

### 2-4. 알려진 x86 실패 모드 (BUILD-LOG §5 요약)
| ID | 증상 | 해결 |
|---|---|---|
| TS-1 | arm64 번들이 x86 VM 에서 부적합 | amd64 빌더에서 재빌드 |
| TS-3 | crane pull finalize 무한 대기 | `USE_CRANE=0`(네이티브 docker pull) |
| TS-4 | docker 29 containerd 스냅샷터 멀티아치 손상 | overlay2 스토어 전환 |
| TS-6 | digest 고정 이미지 save 누락 | `:tag` 재태깅 |

---

## 3. 번들 / 이미지 / 차트 인벤토리

### 3-1. 이미지
- **목록**: `airgap/images/images.txt` — **71개**(주석 제외). 형식 `<registry>/<repo>:<tag>` 또는 digest 고정 `:tag@sha256:…`.
- **레지스트리 분포**: quay.io 17 · registry.gitlab.com 15 · ghcr.io 12 · docker.io 12 · registry.k8s.io 3 · minio 2 · busybox 2 · 기타(opensearch/otel/openbao/grafana/kindest/registry/ecr/jimmidyson) 각 1.
- **MANIFEST**: `airgap/bundle/MANIFEST.txt` — 75행(`image@sha256:digest`). **이미지 71 vs MANIFEST 75 차이**는 `localhost:5001` 재태깅 참조가 추가로 기록된 것. 점검 시 71(원본)/75(재태깅 포함) 구분.
- 목록 재생성: `airgap/images/README.md` 참조(helm template + 인프라 이미지).

### 3-2. 차트 (카탈로그 15종, `airgap/helm/charts-catalog/`)
cert-manager v1.16.3 · metrics-server 3.12.2 · minio 5.4.0 · gitlab 8.7.2 · gitlab-runner 0.72.0 · argo-cd 7.7.16 · kube-prometheus-stack 69.3.0 · grafana 8.9.0 · loki 2.10.3 · opensearch 2.22.0 · opentelemetry-collector 0.75.0 · keycloak 24.4.5 · harbor 1.15.0 · gateway-helm v1.4.3 · postgresql 16.7.21.
- 코어 차트: `airgap/helm/nullus-0.1.0.tgz` + `values-airgap.yaml`.

### 3-3. 번들 구성 (`dist/nullus-airgap-bundle-<date>.tar.gz`, ~6.2GB)
`bin/<platform>/` · `bundle/images.tar.gz`(+sha256, MANIFEST) · `helm/`(nullus + charts-catalog + values) · `kind/`(kind-airgap.yaml, registry.yaml) · `scripts/`(설치용) · `images/images.txt` · `INSTALL.md` · `VERSION` · `Makefile`.

---

## 4. 운영 문서 맵 & 보강 포인트

### 기존 문서
| 문서 | 내용 |
|---|---|
| `airgap/README.md` | 개요·퀵스타트·디렉토리·환경변수 |
| `airgap/INSTALL.md` | 설치 가이드(사전요건·번들 반입·10단계·트러블슈팅) |
| `airgap/docs/architecture.md` | 데이터 흐름·이미지 재태깅 규칙·멀티아치 플래트닝 |
| `airgap/docs/prerequisites.md` | 온/오프라인 머신 요구사항·네트워크 체크리스트 |
| `airgap/docs/runbook.md` | 9단계 절차 + 재설치/업그레이드/정리 시나리오 |
| `airgap/docs/troubleshooting.md` | ImagePullBackOff·helm timeout·파드 crash·레지스트리 |
| `deploy/csp/kakaocloud/docs/BUILD-LOG.md` | 실 배포 로그 + amd64 블로커 8건 |

### 본 문서가 채우는 공백
- ✅ **x86/amd64 명시적 검증 체크리스트**(기존엔 암묵적) — §2.
- ✅ **빌드↔설치 스크립트 역할 일람** — §1.
- ✅ **이미지/차트/번들 인벤토리 단일 출처** + 71 vs 75 불일치 해설 — §3.
- ➡️ **라이선스 고지**: 별도 [`airgap/THIRD_PARTY_NOTICES.md`](../THIRD_PARTY_NOTICES.md) 로 분리(번들 재배포 시 필수 검토).

### 보강 완료 / 후속
- ✅ **SBOM 생성 자동화**: `scripts/pre/generate-sbom.sh`(syft) — `pre-build.sh` 5/6 단계, 출력 `bundle/sbom/`(번들 포함).
- ✅ **`99-verify.sh` 아키텍처 검증**: 노드 `.status.nodeInfo.architecture` 를 `EXPECTED_ARCH`(기본 amd64)와 대조.
- ➡️ 후속: 각 구성요소 원본 LICENSE 전문 동봉, §1 재배포 민감 항목 법무 검토.
