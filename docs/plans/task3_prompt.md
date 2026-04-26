# F8 Task 3 — 클러스터 Node Architecture 체크 및 Pre-Deploy Gate 파라미터 전달

> Claude CLI에 그대로 붙여넣어 지시할 수 있는 단일 프롬프트입니다.
> 사용 시 `cd /path/to/cloud-nullus/draft` 먼저 이동하고 `claude` 실행 후 복사해 주세요.

---

## 컨텍스트 (먼저 읽어주세요)

이 작업은 Nullus Platform (`github.com/cloud-nullus/draft`) F8 "DevSecOps Stack OSS 버전 호환성 관리" 백로그의 **Task 3** 입니다.
배경 문서:

- `docs/plans/compatibility_matrix_plan.md` — F8 계획 문서 (Task 1, 2 완료, Task 3 대상)
- `docs/20_아키텍처/Narwhal_호환성_Seed_Sources.md` — 호환성 seed v1 버전 출처
- `docs/20_아키텍처/Nullus_DB_스키마.md` — DB 스키마 전반
- `docs/20_아키텍처/Nullus_백엔드_상세설계.md` — Clean Architecture + DDD 구조
- `CLAUDE.md` (루트) — 코드 컨벤션

선행 작업에서 이미 완료된 부분:

- 마이그레이션 `000041_compat_tool_fields` — `ToolVersion.ArchSupport []string` / `MinK8sVersion` / `Tier` JSONB 필드 도입.
- 마이그레이션 `000042_seed_narwhal_compat_refresh` — Golden Path 3종에 Narwhal v1 arch pin 주입 (Harbor / GitLab 계열 `amd64`, 그 외 `amd64,arm64`).
- `internal/stack/domain/compatibility.go` — `ToolVersion.SupportsArch(arch string) bool`, `EffectiveMinK8sVersion(matrix) string` 헬퍼.
- `internal/stack/adapter/repository/memory_compatibility.go` — `narwhal*` 상수 기반 캐노니컬 baseline.

Task 3 의 목표는 **실제 클러스터의 노드 아키텍처를 discovery 해서 Pre-Deploy Gate에 입력으로 흘려보내는 것** 입니다.
"ToolVersion이 arm64 를 지원하지 않는데 대상 클러스터에 arm64 노드가 있다" 라는 상황을 배포 위저드 단계에서 차단/경고할 수 있어야 합니다.

---

## Task 3 상세 요구사항

### 1. 도메인 (Domain Layer)

- `internal/admin/domain/cluster.go`
  - `Cluster` 구조체에 `NodeArchitectures []string` 필드 추가 (JSON tag `node_architectures`, 예: `["amd64"]`, `["amd64","arm64"]`).
  - 값의 소스는 쿠버네티스 `node.status.nodeInfo.architecture` 필드의 distinct set.
  - `NodeArchitectures`는 **unordered set 의미**이지만 결정적(sort)하게 저장되어야 한다 (ex: `amd64` 가 있으면 항상 `arm64` 앞).

- 새 값 객체: `ClusterDiscoveryInfo`
  - 필드: `ServerVersion string`, `NodeArchitectures []string`, `NodeCount int`, `DiscoveredAt time.Time`.
  - cluster discovery 결과를 담는 도메인 값 객체. `VerifyCluster`에서 반환.

### 2. 어댑터 (Kubernetes Discovery)

- `internal/admin/adapter/kube/client.go`
  - 현재 `VerifyCluster(kubeconfigBytes []byte)`는 `ServerVersion` 만 반환함. 이를 확장해서 node list 도 가져온다.
  - 새 함수 (or 기존 함수의 반환 구조체 확장) 이름 제안: `DiscoverCluster(ctx context.Context, kubeconfigBytes []byte) (*domain.ClusterDiscoveryInfo, error)`.
  - `clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})` 호출 후 각 노드의 `status.nodeInfo.architecture` 를 수집 → sort + dedupe 해서 `NodeArchitectures` 구성.
  - 기존 `VerifyCluster`는 호환성을 위해 유지하되 내부적으로 `DiscoverCluster`를 호출하도록 refactor.
  - 타임아웃은 기존 `10 * time.Second` 유지. node list 호출도 이 범위 안에서 완료.
  - 네트워크 오류/권한 오류는 기존과 동일한 포맷으로 wrap (`fmt.Errorf("list cluster nodes: %w", err)`).

### 3. 영속화 (Persistence)

- `internal/admin/port/repository.go`의 `ClusterRepository`:
  - `Update` 로 기존 cluster를 discovery 결과로 갱신할 수 있으므로 port 자체에는 추가 메서드 **없음**. 단, 호출자(usecase)가 discovery 결과를 Cluster 객체에 반영하고 `Update`만 호출하면 되도록 한다.
  - 단, `NodeArchitectures`만 touch 하는 전용 쿼리가 필요하다면 `UpdateDiscoveryInfo(ctx, id string, info *domain.ClusterDiscoveryInfo) error` 를 추가해도 좋다 (선택). 추가한다면 postgres / memory / mock 세 구현 모두 업데이트할 것.

- DB 마이그레이션 `db/migrations/000043_cluster_node_architectures.up.sql` + `.down.sql`
  - `clusters` 테이블에 `node_architectures TEXT[] NOT NULL DEFAULT '{}'` 컬럼 추가.
  - down 은 컬럼 drop.
  - `000042` 다음 번호를 부여. 만약 그 사이에 다른 마이그레이션이 추가됐다면 `ls db/migrations` 로 가장 큰 번호 + 1 을 사용.

- `internal/admin/adapter/repository/postgres_cluster.go`
  - `SELECT ... node_architectures ...` / `INSERT ... node_architectures ...` / `UPDATE ... node_architectures ...` 반영.
  - Postgres 의 `TEXT[]`는 `pq.Array(...)` / `pq.StringArray` 로 스캔.

- `internal/admin/adapter/repository/memory_cluster.go`
  - Cluster 복제 시 `NodeArchitectures` 슬라이스도 deep copy.

### 4. 유스케이스 (Use Case / Service)

- `internal/admin/usecase/cluster_usecase.go`
  - `RegisterCluster` (신규 등록) 와 `UpdateCluster` (갱신) 경로에서 kubeconfig 가 유효하면 자동으로 `DiscoverCluster`를 호출해 결과를 Cluster 도메인에 반영하도록 수정.
  - 별도 호출 경로 `RefreshDiscovery(ctx, clusterID)` 를 추가해 주기적 갱신/수동 재판독이 가능하도록 한다 (내부 API 로도 노출할 수 있도록).
  - discovery 실패 시: cluster 등록 자체는 실패시키지 말고 `connection_status = "connection_failed"` 로 기록하고, `node_architectures` 는 빈 슬라이스로 둔다. 로그만 남긴다. (사용자는 나중에 Refresh 가능.)

### 5. Pre-Deploy Gate 연동 (Stack Bounded Context)

- `internal/stack/` 의 Pre-Deploy Gate / Compatibility 검증 서비스가 현재 어떻게 구성되어 있는지 먼저 `rg -l "PreDeployGate\|CompatibilityService\|SupportsArch"` 로 파악할 것.
- 검증 서비스 input DTO 에 `NodeArchitectures []string` 를 추가하고, 각 `ToolVersion.SupportsArch(arch)` 를 교차 검증하는 로직을 넣는다.
  - 한 ToolVersion 이 클러스터 아키텍처 전부를 지원하면 `pass`.
  - 한 ToolVersion 이 클러스터 아키텍처 중 **하나라도** 지원하지 않으면 `fail` (현재 로직상 노드 taint 기반 스케줄링을 사용자가 추가 입력하지 않는 한 안전하게 차단).
  - `status="untested"` 매트릭스의 경우 동일 충돌을 `warn` 으로 완화할지 여부는 기존 verdict 계산 함수의 정책과 **일치**시킬 것. (기존 함수를 확인 후 보수적으로 결정.)
- API 핸들러 쪽:
  - `GET /api/v1/compatibility/validate` 또는 `POST /api/v1/compatibility/validate` (현 노선에 맞춰)가 `cluster_id` 또는 `node_architectures` 를 입력으로 받을 수 있도록 확장.
  - `cluster_id` 만 들어오면 admin 모듈에서 조회해 노드 아키텍처 주입 (모듈 간 참조는 port / reader interface 경유. `StackReader` 패턴과 동일하게 `ClusterReader` port 를 stack 쪽에 정의할 수 있음).

### 6. 테스트 (TDD)

아래를 **Red → Green** 순으로 새로 추가하세요. 기존 테스트를 깨지 않는 범위에서 작업할 것.

- `internal/admin/adapter/kube/client_test.go` (신규 또는 확장)
  - Fake clientset (`k8s.io/client-go/kubernetes/fake`) 으로 1노드 / 멀티노드 / 혼합 아키 케이스 검증.
- `internal/admin/usecase/cluster_usecase_test.go`
  - discovery 성공 시 `NodeArchitectures` 가 Cluster 에 반영되는지.
  - discovery 실패 시 `connection_status` 만 변경되고 빈 슬라이스로 남는지.
- `internal/admin/adapter/repository/memory_cluster_test.go`
  - `NodeArchitectures` round-trip (set / get / update) deep copy 검증.
- `internal/stack/domain/compatibility_test.go` 또는 Pre-Deploy Gate 서비스 테스트
  - 혼합 `["amd64","arm64"]` 클러스터 + `ArchSupport=["amd64"]` Harbor → fail/warn verdict.
  - `["amd64"]` 단일 클러스터 + 동일 Harbor → pass.
  - untested 매트릭스에서의 edge case.
- Postgres repository 통합 테스트가 존재한다면 (`postgres_integration_test.go`) `node_architectures` 컬럼 round-trip 도 추가.

### 7. 프론트엔드

**이 프롬프트의 scope 에는 포함하지 않는다.** UI 측 표시(Task 4/5)는 별도 Task 로 분리되어 있음. 다만 API 응답 스키마에 `node_architectures` 필드를 내보내야 하므로 기존 `GET /admin/clusters/:id` / `/clusters` 응답 DTO 에 포함시킬 것.

---

## 제약사항 (중요)

1. **Clean Architecture + DDD**: `stack` 모듈이 `admin` 모듈의 내부 타입을 직접 import 하지 않는다. 필요 시 stack 쪽에 `port.ClusterReader` interface 를 정의하고 wire 에서 admin usecase 를 어댑터로 주입한다 (CI/CD 모듈의 `StackReader` 패턴 참고).
2. **마이그레이션 idempotency**: `000043` 의 up/down 은 재실행 안전해야 한다. 컬럼 존재 여부 `IF NOT EXISTS` / `IF EXISTS` 사용.
3. **Nil-safe**: `NodeArchitectures == nil` 과 `len == 0` 을 동치로 처리. 검증 로직에서 빈 슬라이스는 "아키 불명" 으로 간주 → 보수적으로 `warn` 반환하고 사용자에게 Refresh 를 유도.
4. **결정론**: `NodeArchitectures` 는 반드시 정렬된 형태로 저장/반환 (unit test 에서 `assert.Equal` 가 깨지지 않도록).
5. **CHANGELOG 업데이트**: `CHANGELOG.md` 의 `## [Unreleased] > ### Added` 맨 위에 Task 3 항목을 추가. Task 1/2 스타일을 유지.
6. **plan 문서 업데이트**: `docs/plans/compatibility_matrix_plan.md` 의 Task 3 체크박스를 `[x]` 로 바꾸고 구현 요약을 Task 2 형식대로 하위 불릿으로 기록.
7. **테스트 실행**: 마무리 전 반드시 로컬에서 아래를 실행해서 모두 통과시킬 것:
   ```
   go test ./internal/admin/...
   go test ./internal/stack/...
   make lint || true   # 프로젝트에 lint target 있으면
   ```
   실패 시 커밋하지 말고 원인 수정 후 재실행.

---

## 산출물 체크리스트 (작업 종료 전 확인)

- [ ] 마이그레이션 `000043_cluster_node_architectures.{up,down}.sql` (idempotent)
- [ ] `internal/admin/domain/cluster.go` 에 `NodeArchitectures` 필드 + `ClusterDiscoveryInfo` 값 객체
- [ ] `internal/admin/adapter/kube/client.go` 의 `DiscoverCluster` (또는 확장된 `VerifyCluster`)
- [ ] `postgres_cluster.go` / `memory_cluster.go` / (있다면) mock cluster repository — `NodeArchitectures` round-trip
- [ ] `cluster_usecase.go` — Register/Update 시 자동 discovery, `RefreshDiscovery` 추가
- [ ] `stack` 모듈의 Pre-Deploy Gate 서비스 — 노드 아키 입력 수용 + `SupportsArch` 교차 검증
- [ ] `port.ClusterReader` interface (stack 쪽) + wire 주입
- [ ] API 핸들러: `cluster_id` 또는 `node_architectures` 입력 수용, 응답 DTO 에 `node_architectures` 포함
- [ ] 단위/통합 테스트 6종 (위 § 6 기준)
- [ ] `CHANGELOG.md` Unreleased Added 맨 위에 Task 3 항목
- [ ] `docs/plans/compatibility_matrix_plan.md` Task 3 체크박스 `[x]` + 구현 요약
- [ ] 로컬 `go test ./internal/admin/... ./internal/stack/...` 통과 로그 확인

---

## 작업 순서 제안

1. 먼저 현재 저장소의 Pre-Deploy Gate / validate 핸들러 구조를 `rg` 로 파악 (`rg -l "compatibility" internal/stack`).
2. 도메인 변경 → 테스트부터 (Red) → kube adapter → repository → usecase → stack gate → handler 순서로 layer outward.
3. 마이그레이션은 코드 변경 마무리 후 작성해도 되지만, repository 테스트 돌리기 전에 먼저 적용.
4. 마지막에 CHANGELOG / plan / (필요 시) `Narwhal_호환성_Seed_Sources.md` 의 § 6 참고 링크에 이 마이그레이션 번호를 추가.

---

## 하지 말아야 할 것

- DB schema 의 `status`/`k8s_min` 같은 **compat matrix 쪽 필드 추가 변경** — Task 3 범위 밖. 필요하면 별도 follow-up 이슈로 기록.
- 프론트엔드 수정 — Task 4/5 범위.
- `000041` / `000042` 의 up/down 파일 재편집 — 이미 배포 내려간 것으로 간주. 반드시 새 번호의 마이그레이션을 추가.
- `ToolVersion` 구조체 수정 — 이미 Task 1 에서 확정. 동일 로직은 `SupportsArch` 호출로 해결.
- `@anthropic.com` co-author tag 없이 임의로 git 커밋 생성하지 말 것. 커밋은 사용자 요청이 있을 때만.

---

## 완료 시 보고 형식

작업이 끝나면 한국어로, 아래 구조로 짧게 보고하세요.

1. 변경된 파일 목록 (카테고리별)
2. 새 테스트와 그 의도 (1~2 문장씩)
3. `go test ./internal/admin/... ./internal/stack/...` 실행 결과 요약
4. 후속 작업 제안 (Task 4/5 와의 연결 지점)
