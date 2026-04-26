# Task 6 작업지시 프롬프트 — Golden Path 3종 로컬 Kind 클러스터 배포 검증

> **목적:** F8 v1 GA 스코프의 DevOps/QA 마지막 아이템. Narwhal baseline 으로 고정된 Golden Path 3종(`gitlab-allinone-v1`, `gitlab-argocd-v1`, `github-argocd-v1`) 을 실제 로컬 Kind 클러스터(`nullus-platform`) 에서 `InstallStack.Execute` 경로로 배포하고, 스택이 `completed` 상태까지 도달하는지를 integration E2E 로 보장한다.
> `docs/plans/compatibility_matrix_plan.md` Task 6 의 이행 작업. EKS/GKE 실환경 검증은 §6 Follow-up (`F8-F6-Cloud`) 으로 분리되어 있으며 본 Task 범위 밖.

---

## 0. 전제 / 선결 조건

- **로컬 Kind 클러스터 `nullus-platform` 가 존재**해야 한다. 없으면 테스트는 `t.Skip("kind cluster 'nullus-platform' not available")` 로 graceful skip. 기존 `e2e/helm_deploy_test.go` 의 `discoverKindCluster(t)` 함수가 동일한 graceful skip 패턴을 구현하고 있으니 **그대로 재사용**한다.
- 클러스터 권장 리소스: Docker Desktop 12GB memory / 4 CPU 이상. Kind node 이미지는 `kindest/node:v1.30.x` 급 권장. 실제 검증은 리소스 부족 시 타임아웃으로 자연스럽게 드러난다 — 테스트가 환경 튜닝까지 하지는 않는다.
- Helm CLI 가 PATH 에 있어야 한다. 없으면 `t.Skip`.
- 이 테스트는 CI 에서 **opt-in** 으로만 실행. 기본 `go test ./...` 는 영향받지 않는다 (`//go:build e2e` 태그).
- F8 Task 7 의 `cicd/e2e MemoryPipelineRepository.Delete` precondition 이 해결되었다는 가정 하에 진행. 아직이면 본 Task 시작 전에 먼저 해결.

---

## 1. 작업 범위

### 1.1 새 테스트 파일
- `e2e/golden_path_kind_test.go` (`//go:build e2e`)

최상위 테스트:
```go
func TestF8Task6_GoldenPath_KindDeploy(t *testing.T) {
    // 공통 kind 클러스터 발견 + helm CLI 체크 + timeout
    // t.Run("gitlab-allinone-v1", ...) ×3
}
```

### 1.2 테스트 헬퍼
같은 파일에 두거나 `helpers_test.go` 에 추가. 다른 테스트에서 재사용될 가능성을 고려해 public(함수명 대문자) 하지는 않는다 (패키지 내부 헬퍼).

- `type goldenPathCase struct { templateID, namespace string; toolOverrides map[string]map[string]any }`
  - `toolOverrides` 는 리소스 트리밍용 (예: GitLab runner replica 1, MinIO 1 replica, persistence 비활성화).
- `func runGoldenPathDeploy(t *testing.T, clusterName string, kubeconfig []byte, tc goldenPathCase)`:
  1. `stackrepo.NewMemoryStackRepository()`, `stackrepo.NewMemoryTemplateRepository()`, `stackrepo.NewMemoryCompatibilityRepository()`, `logadapter.NewMemoryStreamer()` 준비.
  2. `templateRepo` 에서 `tc.templateID` 조회 → 없으면 `t.Fatalf`. `CreateStack` usecase 로 stack 생성.
  3. `tc.toolOverrides` 를 `stack.Config` 에 적용 (작은 helm values 로 각 도구를 축소). override 는 기존 `template-overrides.ts` 가 아니라 Go 쪽 stack 도메인 수준에서 적용 — `domain.StackConfig.ToolValues[category]` 에 key-value 주입.
  4. `helmadapter.NewHelmInstaller(kubeconfig)` + `helmadapter.NewOrchestrator(installer, kubeconfig, tc.namespace)` 생성. `orch.SetNamespace(tc.namespace)`, `orch.SetStackConfig(stack.Config)`.
  5. `stackuc.NewInstallStack(stackRepo, streamer, stackuc.WithExecutor(orch))` 로 real executor 주입.
  6. `ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)` → `installUC.Execute(ctx, stackuc.InstallStackInput{StackID: stack.ID})`.
  7. 동시에 `go` 루틴 또는 polling 으로 `stackRepo.GetByID` 를 반복 조회해 `state` 추적. 최대 15분 내 `completed` 도달 여부 확인. `failed` / `rolled_back` 는 즉시 `t.Errorf("...")` + 상세 로그 덤프 (실패 원인 분석용).
  8. `t.Cleanup` 으로 헬름 릴리스 uninstall + namespace delete 수행 (순서: uninstall all releases, then `kubectl delete ns` 시도, 실패하면 로그만 남기고 skip — 실제 kind 재사용성을 위해).

### 1.3 3종 subtest 정의
각 케이스마다 고유 namespace 부여 (`nullus-e2e-<template>-<timestamp>`) 해서 재실행 격리.

#### `gitlab-allinone-v1` subtest
- Namespace: `nullus-e2e-glab-allinone-<ts>`
- 리소스 오버라이드:
  - GitLab CE: `global.hosts.domain="example.internal"`, `gitlab.webservice.minReplicas=1`, `gitlab.sidekiq.minReplicas=1`, `gitlab.gitaly.persistence.enabled=false` (로컬 ephemeral), `certmanager.install=false`, `nginx-ingress.enabled=false`.
  - MinIO: `mode=standalone`, `persistence.enabled=false`, `resources.requests.cpu=100m/memory=256Mi`.
  - Harbor: `persistence.enabled=false`, `expose.type=clusterIP`, `notary.enabled=false`, `trivy.enabled=false`.
- 성공 기준: 7분 내 `stack.state == completed`, 이후 `kubectl get pods -n <ns>` 에서 Running 비율 > 70% (flapping 감안).

#### `gitlab-argocd-v1` subtest
- Namespace: `nullus-e2e-glab-argocd-<ts>`
- 오버라이드:
  - GitLab Runner: `replicas=1`, `runners.locked=false`, `runners.tags="e2e"`, `rbac.create=true`.
  - Argo CD: `configs.params.server\\.insecure=true`, `server.service.type=ClusterIP`, `dex.enabled=false`, `notifications.enabled=false`.
  - Harbor: 위와 동일 경량화.
- 성공 기준: 7분 내 completed.

#### `github-argocd-v1` subtest
- Namespace: `nullus-e2e-gh-argocd-<ts>`
- 오버라이드:
  - Argo CD: 위와 동일.
  - Harbor: 위와 동일.
  - GitHub 측은 외부 SaaS 전제이므로 helm 설치 대상 아님 (`source_repository` 카테고리가 external 로 처리되는지 확인 — 아니면 skip 마크).
- 성공 기준: 7분 내 completed (Harbor + Argo CD 2종 helm 설치).

각 케이스는 독립 subtest 로 `t.Parallel()` 은 **사용하지 않는다** — 단일 Kind 클러스터가 3종을 동시 감당하기 어렵다 (리소스 경합으로 flaky). 순차 실행.

### 1.4 리소스/네트워크 격리
- 각 subtest 끝나면 `t.Cleanup` 에서 namespace 삭제를 시도. 실패해도 테스트 자체는 fail 하지 않는다 (최선 노력).
- 동일 Kind 클러스터를 재사용하므로 PVC 누락 방지 위해 모든 helm values 에 `persistence.enabled=false` 를 기본. 필요한 경우만 override 에서 뒤집을 것.

### 1.5 Makefile 타겟
`Makefile` 에 신규 타겟:

```make
.PHONY: test-golden-path

test-golden-path:
	go test -tags e2e -run "^TestF8Task6_GoldenPath" -timeout 60m -v ./e2e/...
```

기존 `.PHONY` 목록(line 1)에도 `test-golden-path` 를 추가해 `make help` 에 노출. `test-integration` 과는 별도 타겟(해당 타겟은 `-tags integration` 이며 e2e 태그와 충돌하지 않음).

### 1.6 CI 훅 (문서화만)
- CI 는 본 테스트를 기본 실행하지 않는다. 본 Task 에서는 **수동 runbook 문서** 만 추가 (코드/워크플로우 변경 금지):
  - `docs/20_아키텍처/F8_Task6_Kind_Runbook.md` (신규) — Kind 클러스터 준비 → `make test-golden-path` 실행 → 결과 해석 가이드.
  - 향후 F8-F6-Cloud follow-up 에서 GitHub Actions 통합은 별도 Task 로 다룬다.

---

## 2. 검증 항목

### 2.1 기본 (반드시 통과)
- 3종 subtest 모두 `completed` 도달 (각 15분 타임아웃).
- `t.Cleanup` 종료 후 kind 클러스터 자체는 살아있고 (re-usable), 임시 namespace 는 삭제됨 (best-effort).
- `go vet ./...` 클린 (태그 없이도 파일 자체는 컴파일 OK 여야 함 — `//go:build e2e` 로 빌드 시점 분리되므로 본체 빌드에는 영향 없음).

### 2.2 실패 시나리오 기록
- 3종 중 하나라도 실패 시: `stack.state`, 마지막 `errorMessage`, phase 목록(Install/Configure/HealthCheck 중 어디서 멈췄는지), `kubectl get events --sort-by=.lastTimestamp -n <ns> | tail -50` 출력을 `t.Logf` 로 남긴다. 사후 분석 가능성 확보.
- 특정 helm 차트가 Narwhal baseline 과 현재 helm repo 상태의 drift 로 실패하는 경우: **테스트는 실패로 남기고**, 보고서에 "drift 원인" 으로 명시. 시드 수정은 Task 2 범위이지 Task 6 이 임의로 손대지 않는다.

### 2.3 회귀
- 기존 `TestHelmDeployOnKindCluster` (nginx 설치) 가 그대로 통과하는지 확인. 본 Task 가 helm installer/orchestrator 를 건드리지 않으므로 영향이 없어야 정상.
- `go test -tags integration ./e2e/` (기존 testcontainer 기반 DB 통합) 도 영향 없음을 스모크로 확인.

---

## 3. 하지 말 것 (Forbidden)

- **마이그레이션/시드 수정 금지.** Golden Path 의 버전 pin 은 Task 2 에서 확정. 본 Task 는 이를 **관찰자** 로서 실측만 한다. 배포가 실패하면 그 자체가 데이터 — Task 2 follow-up 으로 분리할 것.
- **프로덕션 helm chart / 도메인 모델 수정 금지.** override 는 test-local `StackConfig` 수준에서만 주입.
- **EKS / GKE / AWS / GCP 관련 코드/문서 추가 금지.** v1 GA 후 `F8-F6-Cloud` follow-up 범위.
- **`go:build e2e` 태그 누락 금지.** 태그 없으면 기본 `go test ./...` 가 kind 를 찾다가 실패하며 CI 전체가 깨진다.
- **`t.Parallel()` 금지** (§1.3 참고).
- **`test-integration` 타겟 의미 변경 금지.** 새 Makefile 타겟은 추가만, 기존은 그대로.
- **커밋/푸시 금지** — 작업 보고만.

---

## 4. 체크리스트

- [ ] `e2e/golden_path_kind_test.go` 신규 (`//go:build e2e`, `TestF8Task6_GoldenPath_KindDeploy` + 3 subtest).
- [ ] `runGoldenPathDeploy` / `goldenPathCase` 헬퍼 정의.
- [ ] `Makefile` 에 `test-golden-path` 타겟 + `.PHONY` 등록.
- [ ] `docs/20_아키텍처/F8_Task6_Kind_Runbook.md` 신규 — Kind 준비, 실행, 결과 해석.
- [ ] `make test-golden-path` 실행 → 3종 모두 completed 확인 (실패 시 §2.2 의 로그 덤프 보고).
- [ ] 기존 `TestHelmDeployOnKindCluster` 재실행 시 회귀 없음 확인.
- [ ] `go vet ./...` / `go build ./...` 클린.
- [ ] `CHANGELOG.md` `[Unreleased] > Added` 에 Task 6 엔트리 추가.
- [ ] `docs/plans/compatibility_matrix_plan.md` Task 6 을 `[x]` 로 + 3~5줄 요약 (실환경 실측으로 Narwhal pin 의 실제 helm 차트 가용성을 확인했다는 점 강조).
- [ ] 실패 케이스가 있다면 plan `## 6` follow-up 섹션에 각 케이스별로 후속 이슈 TODO 로 기록.

---

## 5. 추천 작업 순서

1. **Dry-run**: `TestHelmDeployOnKindCluster` 를 `make test-golden-path` 와 동일한 환경에서 실행해 Kind 가 살아있고 helm 이 동작하는지 확인.
2. `goldenPathCase` 구조체 + 공통 헬퍼 `runGoldenPathDeploy` 스켈레톤 작성 → 가장 단순한 `github-argocd-v1` (Argo CD + Harbor 2종) 부터 케이스 추가 → 동작 확인.
3. `gitlab-argocd-v1` (GitLab Runner + Argo CD + Harbor) 추가.
4. `gitlab-allinone-v1` (full GitLab 풀스택) 추가 — 가장 무거우므로 맨 마지막.
5. Makefile 타겟 + runbook 문서 작성.
6. 전체 `make test-golden-path` 실행, 60분 타임아웃 이내 통과 확인.
7. CHANGELOG / plan 문서 반영.
8. 보고.

---

## 6. 완료 보고 포맷

1. **변경된 파일 목록** — 테스트 / Makefile / 문서 구분.
2. **실행 결과 요약** — 3종 각각 상태(`completed` / `failed` / `skip`), 소요 시간, 리소스 overrides 요약. 실패가 있으면 §2.2 포맷의 로그 덤프 첨부.
3. **Narwhal baseline drift 관찰** — 실제 helm repo 에서 pin 된 버전이 아직 유효한지, deprecated 경고가 나왔는지 등 (있다면 Task 2 follow-up 후보로 보고).
4. **환경 정보** — Kind 버전, Docker 할당 리소스, helm 버전.
5. **남은 follow-up** — F8-F6-Cloud (EKS/GKE), 리소스 부족 failure 가 있었다면 CI 클러스터 사이즈 권장, helm values override 의 config 화 여부 등.

---

## 7. 참고 포인터

- Kind 발견 로직: `e2e/helm_deploy_test.go` `discoverKindCluster(t)` — 그대로 재사용.
- 실제 helm 설치: `e2e/helm_deploy_test.go` `TestHelmDeployOnKindCluster` (nginx 1종 단순 케이스).
- `InstallStack` usecase: `internal/stack/usecase/install_stack.go` `Execute(ctx, InstallStackInput{StackID})` — `WithExecutor(port.StepExecutor)` 옵션.
- Helm orchestrator: `internal/stack/adapter/helm/orchestrator.go` `NewOrchestrator(installer, kubeconfig, namespace, opts...)`. `SetStackConfig`, `SetNamespace`, `VerifyDeployment` 메서드 존재.
- Golden Path 템플릿 시드: `internal/stack/adapter/repository/memory_template.go` — 3종 ID (`gitlab-allinone-v1`, `gitlab-argocd-v1`, `github-argocd-v1`) 위치 참고.
- 도메인 state machine: `internal/stack/domain/stack.go` `validTransitions` — `StateCompleted` 도달 경로.
