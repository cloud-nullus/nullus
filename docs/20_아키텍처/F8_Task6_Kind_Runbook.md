# F8 Task 6 — Golden Path Kind Deploy Runbook

**Scope**: Narwhal baseline 으로 고정된 Golden Path 3종 (`gitlab-allinone-v1`,
`gitlab-argocd-v1`, `github-argocd-v1`) 을 로컬 Kind 클러스터에서 end-to-end
로 배포 검증한다. `InstallStack.Execute` 경로가 실제 helm chart 설치까지
이어지고, 각 매트릭스의 stack 이 `completed` 상태까지 도달하는지를 확인하는
pre-merge 스모크 절차.

EKS / GKE 등 실 클라우드 검증은 `F8-F6-Cloud` follow-up 범위이며 본 문서는
다루지 않는다.

---

## 1. 선결 조건

| 항목 | 권장 값 | 비고 |
|---|---|---|
| Docker Desktop (또는 OrbStack) | 4.x 이상 | 메모리 ≥ 12 GB, CPU ≥ 4 core 할당 |
| `kind` CLI | 0.20+ | `kind version` 로 확인 |
| `kubectl` CLI | 1.29+ | Kind 와 버전 스큐 ±1 이내 권장 |
| `helm` CLI | 3.14+ | `helm version --short` |
| Go | 1.24+ | 프로젝트 go.mod 와 일치 |

선결 실패 시 테스트는 `t.Skip(...)` 로 graceful skip 한다 — CI 에서는 기본
실행되지 않으므로 로컬 오퍼레이터가 의도적으로 실행하는 수동 검증 절차.

---

## 2. Kind 클러스터 기동

```bash
# 1) 기존 클러스터 유무 확인
kind get clusters

# 2) 새로 생성 (GitLab 풀스택을 위해 worker 1 + control-plane 1 권장)
#    이미 nullus-platform 이 있으면 스킵.
cat <<EOF > /tmp/kind-nullus.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: nullus-platform
nodes:
  - role: control-plane
  - role: worker
EOF
kind create cluster --config /tmp/kind-nullus.yaml --image kindest/node:v1.30.0

# 3) kubeconfig 내보내기 (이 테스트는 내부적으로 `kind get kubeconfig` 사용)
kubectl cluster-info --context kind-nullus-platform
```

### 리소스 힌트

| Golden Path | 예상 RAM | 예상 CPU | 예상 install time |
|---|---|---|---|
| `github-argocd-v1`  | ~4 GB  | 2 vCPU | 4-6 분 (Argo CD + Harbor + MinIO) |
| `gitlab-argocd-v1`  | ~8 GB  | 3 vCPU | 8-10 분 (GitLab CE + Argo CD + Harbor) |
| `gitlab-allinone-v1`| ~10 GB | 4 vCPU | 10-15 분 (GitLab 풀스택) |

Docker Desktop 에 12 GB 가 할당되지 않은 상태에서 `gitlab-allinone-v1` 를
돌리면 OOM 또는 image pull back-off 로 타임아웃이 발생할 수 있다. 반드시
리소스를 확보한 뒤 실행할 것.

---

## 3. 테스트 실행

```bash
cd /path/to/draft

# 선택: 스모크로 helm adapter 가 kind 에 붙는지 먼저 확인
go test -tags e2e -run "^TestHelmDeployOnKindCluster$" -timeout 10m -v ./e2e/

# 본 테스트 — 3 subtest 순차 실행, 총 60분 budget
make test-golden-path
```

`test-golden-path` 는 내부적으로:

```
go test -tags e2e -run "^TestF8Task6_GoldenPath" -timeout 60m -v ./e2e/...
```

로 실행되며, `e2e` 빌드 태그 덕에 기본 `go test ./...` 파이프라인에는
영향이 없다.

### subtest 만 선택 실행

```bash
# 가장 가벼운 github-argocd-v1 만 먼저 확인
go test -tags e2e -run "^TestF8Task6_GoldenPath_KindDeploy/github-argocd-v1$" \
  -timeout 15m -v ./e2e/
```

### 💡 실제 검증에서 확인된 사항 (2026-04-20 로컬 검증)

| subtest | 결과 | 소요시간 | 비고 |
|---|---|---|---|
| `github-argocd-v1` | ✅ PASS | 4분 5초 | Argo CD + MinIO + cert-manager 실 helm 설치 성공 |
| `gitlab-argocd-v1` | ❌ FAIL | 8~15분 | GitLab 풀 스택 — 단일 노드 Kind 에서 pod Ready 까지 도달하지 못함 |
| `gitlab-allinone-v1` | ⏭️ 미실행 | — | 리소스 부담으로 미시도; 권장 사양(10 vCPU / 20 GB RAM) 확보 후 재검증 |

**Cluster-scoped 리소스 leak 이슈**: `cert-manager` 의 CRD / ClusterRole /
ClusterRoleBinding 은 cluster-scoped 이므로 첫 subtest 가 남긴 상태가
후속 subtest 의 `helm install cert-manager` 시점에 "ownership 충돌" 을 일으킨다.

```
Error: Unable to continue with install: ClusterRole "cert-manager-cainjector"
...cannot be imported into the current release: invalid ownership metadata;
annotation validation error: key "meta.helm.sh/release-namespace"
must equal "<new-ns>": current value is "<prev-ns>"
```

테스트가 subtest 종료 시 `helm uninstall cert-manager` 를 최선 노력 수행하지만,
`helm uninstall` 이 기본적으로 일부 cluster-scoped 리소스를 남기므로 완벽하지
않다. 따라서 **여러 subtest 를 정확히 순차 실행하려면 subtest 사이에 Kind
클러스터 자체를 재생성해야 한다.**

권장 운영 방식:

```bash
# subtest 하나씩 돌릴 때마다 Kind 를 리셋
for tc in github-argocd-v1 gitlab-argocd-v1 gitlab-allinone-v1; do
  kind delete cluster --name nullus-platform
  kind create cluster --name nullus-platform --image kindest/node:v1.30.0
  go test -tags e2e -run "^TestF8Task6_GoldenPath_KindDeploy/${tc}\$" \
    -timeout 20m -v ./e2e/
done
kind delete cluster --name nullus-platform
```

이 순서는 단일 스크립트화할 수 있고, CI 에서 runner 당 하나의 subtest 만
돌리도록 샤딩하는 편이 자연스럽다.

---

## 4. 결과 해석

성공 로그 패턴:

```
=== RUN   TestF8Task6_GoldenPath_KindDeploy
    golden_path_kind_test.go:47: using kind cluster "nullus-platform"
=== RUN   TestF8Task6_GoldenPath_KindDeploy/github-argocd-v1
    golden_path_kind_test.go:206: stack stk-e2e-... state: validating
    golden_path_kind_test.go:206: stack stk-e2e-... state: installing
    golden_path_kind_test.go:206: stack stk-e2e-... state: configuring
    golden_path_kind_test.go:206: stack stk-e2e-... state: health_check
    golden_path_kind_test.go:206: stack stk-e2e-... state: completed
    golden_path_kind_test.go:195: template "github-argocd-v1" reached completed state
--- PASS: TestF8Task6_GoldenPath_KindDeploy/github-argocd-v1 (4m53s)
```

실패 시:
- subtest 가 `failed` / `rolled_back` 에 도달하면 `dumpKindDiagnostics` 가
  pod 목록 + events 를 tail 50줄까지 덤프한다.
- 종종 발견되는 drift 유형:
  1. **Helm 차트 저장소 deprecation** — pin 된 버전이 차트 저장소에서 제거됨.
     → Task 2 follow-up (시드 bump) 으로 이슈화.
  2. **ImagePullBackOff** — Kind 노드에 특정 아키텍처 이미지 캐시 없음.
     → Docker Desktop Rosetta 설정 또는 arm64 전용 차트 사용 확인.
  3. **PVC Pending** — Kind 기본 StorageClass(`standard`) 가 준비되지 않음.
     → `kubectl get sc`, 필요 시 `local-path-provisioner` 재설치.
  4. **리소스 부족 (Pending 파드 다수)** — Docker Desktop 메모리/CPU 부족.
     → Docker Desktop 리소스 재할당.

### 관측 중심 원칙 (§3 하지 말 것)

- 본 Task 는 **관찰자 (observer)** 로서 실측만 한다. 차트 drift / 실패는
  데이터로 남기고, seed 수정은 Task 2 범위로 분리한다.
- 프로덕션 helm chart / 도메인 모델은 건드리지 않는다. 조정이 필요하면
  test-local `StackConfig` override 수준에서만 튜닝한다.

---

## 5. 정리

```bash
# 임시 namespace (테스트가 t.Cleanup 으로 최선 시도 삭제, 실패하면 수동)
kubectl --context kind-nullus-platform get ns | grep nullus-e2e
kubectl --context kind-nullus-platform delete ns nullus-e2e-...

# Kind 클러스터 자체 제거 (재사용 하지 않을 때)
kind delete cluster --name nullus-platform
```

---

## 6. 관련 자료

- `e2e/golden_path_kind_test.go` — 실행 스크립트 본체.
- `e2e/helm_deploy_test.go` `discoverKindCluster` — Kind 자동 발견 로직 재사용.
- `internal/stack/usecase/install_stack.go` — `InstallStack.Execute` / state 전이.
- `internal/stack/adapter/helm/orchestrator.go` — 실제 helm 설치 실행자.
- `docs/plans/compatibility_matrix_plan.md` Task 6 — 본 작업의 계획 문서.

follow-up 이슈:
- `F8-F6-Cloud`: EKS / GKE 환경에서 동일 검증 (GitHub Actions 연동 포함).
- `F8-F2-Drift`: 실측 중 발견되는 helm 차트 drift 를 시드 리프레시 마이그레이션
  으로 반영 (Task 2 의 후속).
