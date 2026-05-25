# Nullus Air-Gap Bundle 설치 가이드

`nullus-airgap-bundle-<version>.tar.gz` 단일 파일로 폐쇄망(인터넷 차단 환경)에 Nullus 데모 클러스터를 부팅하는 절차입니다. 외부 레지스트리/저장소 접근 없이 동작합니다.

---

## 1. 사전 요구사항 (오프라인 머신)

| 항목 | 최소 사양 |
|------|-----------|
| OS | Linux x86_64 / arm64, macOS arm64 (번들 빌드 시 PLATFORMS 에 포함된 경우) |
| Docker | 24.0 이상, **데몬 기동 중** |
| 메모리 | 4 GiB 여유 |
| 디스크 | 15 GiB 여유 (번들 압축 해제 + kind 노드 + 이미지) |
| 포트 | 80, 443, 5001 비어 있음 |
| CLI | 번들이 `bin/<platform>/{kind,kubectl,helm}` 을 자체 포함 — 추가 설치 불필요 |

`docker info` 가 종료 코드 0 으로 응답하는지 먼저 확인하세요.

```bash
docker info >/dev/null 2>&1 && echo "docker OK" || echo "docker DOWN — 데몬을 먼저 기동하세요"
```

---

## 2. 번들 반입

온라인 머신에서 빌드된 `nullus-airgap-bundle-<version>.tar.gz` 와 `.sha256` 파일을 USB / SCP / 내부망 전송 등으로 오프라인 머신으로 옮깁니다.

```bash
# 무결성 검증 (Linux)
sha256sum -c nullus-airgap-bundle-*.tar.gz.sha256

# 무결성 검증 (macOS)
shasum -a 256 -c nullus-airgap-bundle-*.tar.gz.sha256
```

`OK` 가 출력되면 손상 없음.

---

## 3. 원샷 설치

번들이 알아서 압축 해제 + 모든 단계 수행합니다.

```bash
./install.sh nullus-airgap-bundle-<version>.tar.gz
```

또는 직접 압축 해제 후 실행:

```bash
tar -xzf nullus-airgap-bundle-<version>.tar.gz
cd nullus-airgap-bundle-<version>/
./install.sh
```

내부적으로 다음 7단계가 순차 실행됩니다 (총 약 5 ~ 15 분).

| # | 단계 | 스크립트 |
|---|------|----------|
| 1 | 이미지 docker load (~2 GB tar 풀기, 30개 image) | `scripts/03-load-bundle.sh` |
| 2 | 로컬 레지스트리(registry:2) 컨테이너 기동 | `scripts/10-setup-registry.sh` |
| 3 | kind 클러스터 생성 + 레지스트리 연결 | `scripts/11-create-cluster.sh` |
| 4 | 이미지 → 로컬 레지스트리 retag/push | `scripts/12-push-to-registry.sh` |
| 5 | Helm 차트 설치 (Nullus + PostgreSQL) | `scripts/21-install-nullus.sh` |
| 6 | Platform stack 설치 (Keycloak; INSTALL_FULL=1 시 Prometheus stack 추가) | `scripts/22-install-platform-stack.sh` |
| 7 | 검증 + kubeconfig 컨텍스트 정리 | `scripts/99-verify.sh` + `13-set-config.sh` |

---

## 4. 설치 결과 확인

```bash
kubectl get pods -n nullus
```

성공 시 출력 (`-n nullus`):

```
NAME                          READY   STATUS    RESTARTS   AGE
nullus-api-xxxxx              1/1     Running   0          2m
nullus-api-yyyyy              1/1     Running   0          2m
nullus-postgresql-0           1/1     Running   0          2m
nullus-web-aaaaa              1/1     Running   0          2m
nullus-web-bbbbb              1/1     Running   0          2m
```

Keycloak (`-n nullus-auth`):

```
NAME                    READY   STATUS    RESTARTS   AGE
keycloak-0              1/1     Running   0          2m
keycloak-postgresql-0   1/1     Running   0          2m
```

Keycloak 접근:

```bash
kubectl port-forward -n nullus-auth svc/keycloak 8180:80
# http://localhost:8180  로그인: admin / admin
# Realm 'nullus' 생성: scripts/setup-keycloak.sh 또는 admin UI
```

웹 UI 접근:

```bash
kubectl port-forward -n nullus svc/nullus-web 8080:80
# http://localhost:8080 접속
```

---

## 5. 환경 변수 (선택)

`install.sh` 직전에 export 하면 동작이 바뀝니다.

| 변수 | 기본값 | 용도 |
|------|--------|------|
| `CLUSTER_NAME` | `nullus-airgap` | kind 클러스터 이름 |
| `SKIP_VERIFY` | `0` | `1` 이면 마지막 verify 단계 건너뜀 |
| `SKIP_PLATFORM` | `0` | `1` 이면 STEP 6 (Keycloak) 건너뜀 |
| `INSTALL_FULL` | `0` | `1` 이면 STEP 6 에서 kube-prometheus-stack 도 설치 |
| `SKIP_KEYCLOAK` | `0` | `1` 이면 STEP 6 의 Keycloak 만 건너뜀 (다른 platform 컴포넌트는 유지) |
| `PLATFORM_OVR` | (자동 탐지) | bin 디렉토리 강제 (예: `linux-amd64`) |
| `RELEASE` | `nullus` | helm 릴리스 이름 |
| `NAMESPACE` | `nullus` | Nullus 네임스페이스 |
| `NAMESPACE_AUTH` | `nullus-auth` | Keycloak 네임스페이스 |
| `NAMESPACE_OBSERV` | `nullus-monitoring` | Prometheus stack 네임스페이스 (INSTALL_FULL=1 시) |
| `KEYCLOAK_ADMIN` | `admin` | Keycloak 관리자 계정 |
| `KEYCLOAK_PASSWORD` | `admin` | Keycloak 관리자 비밀번호 (운영에선 반드시 교체) |

예시 — 이름과 시크릿을 바꾸면서 설치:

```bash
NAMESPACE=prod \
EXTRA_ARGS='--set secrets.dbPassword=<강력비밀번호> --set secrets.encryptionKey=<32자이상키>' \
./install.sh
```

---

## 6. 정리 (재설치 / 클린업)

```bash
# 클러스터 + 로컬 레지스트리 컨테이너 제거
cd nullus-airgap-bundle-<version>/
make clean

# 또는 직접
kind delete cluster --name nullus-airgap
docker rm -f kind-registry
```

이후 `./install.sh` 를 다시 실행하면 새 클러스터로 재설치됩니다.

---

## 7. 트러블슈팅

| 증상 | 원인 / 해결 |
|------|-------------|
| `dial unix docker.sock: no such file or directory` | docker 데몬 미기동 — Docker Desktop / `colima start` / `sudo systemctl start docker` |
| `이 번들에는 <platform> 바이너리가 없습니다` | 빌드 시 PLATFORMS 에 해당 OS/arch 가 포함되지 않음 — 온라인에서 `PLATFORMS="linux-amd64,linux-arm64,darwin-arm64" make pre-build` 로 재생성 |
| `ImagePullBackOff: connection refused localhost:5001` | kind 노드가 미러 endpoint 에 도달 못함 — `make clean` 후 재설치 |
| `bind() to 0.0.0.0:80 failed: Permission denied` (web pod) | 차트가 비특권 포트(8080) 미반영 — 번들 빌드 직전 `helm dep update && helm template` 으로 차트 갱신 후 재빌드 |
| `helm install` `unrecognized images` | Bitnami security 가드 — `helm/values-airgap.yaml` 에 `global.security.allowInsecureImages: true` 가 있는지 확인 |
| 디스크 부족 (`no space left on device`) | `/var/lib/docker` 용량 확보 또는 다른 디스크로 docker root 변경 |

자세한 로그 수집 명령:

```bash
docker logs kind-registry                                # 레지스트리
kubectl describe pod -n nullus <pod>                     # 파드 상태
docker exec nullus-airgap-control-plane crictl images    # 노드 캐시 이미지
```

---

## 8. 번들 구조 참조

```
nullus-airgap-bundle-<version>/
├── install.sh                      ← 원샷 진입점 (이 가이드의 3절)
├── INSTALL.md                      ← 본 문서
├── VERSION                         ← 빌드 메타정보
├── Makefile                        ← 단계별 수동 실행용 (make help)
├── bin/<platform>/                 ← kind, kubectl, helm 바이너리
├── images/
│   ├── images.tar.gz               ← 모든 컨테이너 이미지 (docker save 산출물)
│   ├── images.tar.gz.sha256
│   └── MANIFEST.txt
├── helm/
│   ├── nullus-<ver>.tgz            ← Nullus Helm 차트
│   ├── values-airgap.yaml          ← 에어갭 환경 values 오버라이드
│   ├── charts-catalog/             ← Stack 카탈로그 (stack orchestrator 가 사용자 클러스터에 배포)
│   │   ├── cert-manager-v1.16.3.tgz         ← TLS 인증서 (orchestrator 1순위)
│   │   ├── metrics-server-3.12.2.tgz        ← HPA/리소스 메트릭
│   │   ├── keycloak-24.4.5.tgz              ← OIDC (STEP 6 자동 설치)
│   │   ├── minio-5.4.0.tgz                  ← 오브젝트 스토리지 (charts.min.io)
│   │   ├── gitlab-8.7.2.tgz                 ← 소스 저장소 (embedded GitLab)
│   │   ├── gitlab-runner-0.72.0.tgz         ← GitLab CI runner
│   │   ├── argo-cd-7.7.16.tgz               ← GitOps CD
│   │   ├── kube-prometheus-stack-69.3.0.tgz ← Prometheus + AlertManager + operator
│   │   ├── grafana-8.9.0.tgz                ← 대시보드
│   │   ├── loki-2.10.3.tgz                  ← 로그 수집
│   │   ├── opensearch-2.22.0.tgz            ← 로그 검색
│   │   ├── opentelemetry-collector-0.75.0.tgz ← Tracing
│   │   ├── gateway-helm-v1.4.3.tgz          ← Envoy Gateway (oci://docker.io)
│   │   └── harbor-1.15.0.tgz                ← (선택) 컨테이너 레지스트리
│   └── charts-catalog-values/      ← 일부 카탈로그 chart 의 helm-template 렌더용 values
│       ├── gitlab.yaml                       ← global.hosts.domain 등 필수 값
│       └── opentelemetry-collector.yaml      ← mode 등 필수 값
├── kind/
│   ├── kind-airgap.yaml            ← 클러스터 설정 + containerd mirror 패치
│   └── registry.yaml               ← local-registry-hosting ConfigMap
├── scripts/
│   ├── 03-load-bundle.sh           ← 이미지 docker load
│   ├── 10-setup-registry.sh        ← registry:2 기동
│   ├── 11-create-cluster.sh        ← kind 생성
│   ├── 12-push-to-registry.sh      ← retag + push
│   ├── 13-set-config.sh            ← kubeconfig 정리
│   ├── 21-install-nullus.sh        ← helm install
│   ├── 99-verify.sh                ← 5항목 PASS/FAIL
│   └── bootstrap.sh                ← (대안) 단계별 수동 실행 시 entrypoint
└── images.txt                      ← 이미지 목록 (12-push 가 참조)
```

---

## 9. 개발자용: 번들 재생성 (온라인)

```bash
cd <repo-root>/airgap
PLATFORMS="linux-amd64,linux-arm64,darwin-arm64" \
KIND_VERSION=v0.31.0 KUBECTL_VERSION=v1.30.0 HELM_VERSION=v3.16.0 \
make pre-build
# → dist/nullus-airgap-bundle-<date>.tar.gz 생성
```

산출물(`bundle/`, `bin/`, `dist/`, `helm/charts/`, `helm/nullus-*.tgz`)은 `.gitignore` 로 제외되어 커밋되지 않습니다.
