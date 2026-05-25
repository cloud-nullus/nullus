# Stack <-> CI/CD 통합모드 설계

> 작성일: 2026-05-25
> 상태: 신규 일반 운영 경로 설계
> 실행 모드: `stack_integrated`
> 긴급 배포 설계: [Stack_CICD_모듈간_관계_설계.md](./Stack_CICD_모듈간_관계_설계.md)

---

## 1. 목적

Nullus의 일반 CI/CD 모드는 Stack과 Pipeline을 단순히 `stack_id`로 연결하는 데 그치지 않는다. **Stack List에 배포 완료된 Stack이 제공하는 컴포넌트를 CI/CD의 실제 실행 자원으로 사용**한다.

```text
Stack = Code Repository / Package Registry / Image Registry / CI Platform / CD Tool 공급자
CI/CD = 선택된 Stack 컴포넌트를 연결하고 프로비저닝하며 실행 상태를 추적하는 영역
```

신규 CI/CD 개발의 기본 경로는 `stack_integrated`이다. Nullus가 직접 앱 이미지를 빌드하여 Kind에 적재하고 적용하는 기존 흐름은 일반모드가 아니라 장애 대응용 `emergency_direct` 모드로 보존한다.

## 2. 설계 원칙

### 2.1 컴포넌트 소비형 연계

일반모드 Pipeline은 배포 완료 Stack에서 제공되는 다음 컴포넌트를 선택하여 사용한다.

| 컴포넌트 | 일반모드에서의 책임 |
|---|---|
| `code_repository` | 앱 소스와 pipeline definition의 저장 및 trigger source |
| `package_registry` | 빌드 artifact, library/package, SBOM, test report 저장 |
| `image_registry` | container image 저장 및 배포 image source |
| `ci_platform` | checkout, build, test, publish, image push 실행 |
| `cd_tool` | 환경 저장소 변경 또는 image reference를 target cluster에 동기화 |

### 2.2 모듈 경계 유지

CI/CD 모듈은 Stack 도메인 타입을 직접 import하지 않는다. CI/CD가 필요한 컴포넌트 정보는 CI/CD 소유 Port를 통해 읽는다.

```text
internal/cicd/port/stack_integration_reader.go
  -> StackIntegrationReader interface

internal/cicd/adapter/repository/
  -> 현재 Modular Monolith의 조회 adapter

향후 서비스 분리 시
  -> HTTP/gRPC adapter로 대체
```

### 2.3 Credential 비노출

Pipeline 설정과 API 응답에는 credential 평문을 저장하거나 반환하지 않는다.

- DB에는 `credential_ref`만 저장한다.
- 실행 시 Secret Manager(OpenBao)에서 token/password를 읽는다.
- 사용자 화면에는 연결 준비 상태와 provider만 표시한다.

## 3. 정상 실행 흐름

```text
Stack List에서 completed Stack 선택
  -> Code Repository / Package Registry / Image Registry / CI Platform / CD Tool 조회
  -> endpoint 및 credential_ref 확인
  -> Pipeline 구성 생성
  -> 선택된 CI provider에 workflow/job 프로비저닝
  -> Source push/merge
  -> Build/Test
  -> Package Registry에 artifact/SBOM/test report publish
  -> Image Registry에 container image push
  -> 환경 저장소 manifest/image tag 또는 digest 갱신
  -> CD Tool sync/deploy
  -> Nullus가 실행 상태/로그/이력 수집
```

### 3.1 Pipeline 구성 시점

1. 사용자가 CI/CD 생성 화면에서 배포 완료 Stack을 선택한다.
2. Nullus는 해당 Stack의 사용 가능한 integration inventory를 조회한다.
3. 사용자는 소스 저장소를 기존 프로젝트에서 선택하거나 신규 생성한다.
4. Package Registry 및 Image Registry의 namespace/project/repository 경로를 결정한다.
5. CI Platform과 CD Tool을 선택하고 필요한 workflow/application 구성을 검토한다.
6. Pipeline을 저장하고 `provision`을 실행한다.

### 3.2 Runtime 시점

1. 개발자가 연결된 Code Repository에 push 또는 merge한다.
2. CI Platform이 소스를 checkout하고 build/test를 수행한다.
3. 빌드 산출물과 provenance 자료를 Package Registry에 publish한다.
4. container image를 만들어 Image Registry에 immutable tag 또는 digest로 push한다.
5. CI가 별도 환경 저장소의 deployment manifest 또는 values를 갱신한다.
6. CD Tool이 환경 저장소 변경을 감지하고 target cluster에 동기화한다.
7. Nullus는 provider run 및 CD sync 상태를 조회하여 이력과 로그를 제공한다.

## 4. `nullus-devsecops-stack` 적용 예시

`nullus-devsecops-stack`은 고정된 provider 이름이 아니라, 실제 배포된 Stack 레코드의 `config`와 connection 상태를 기준으로 해석한다. 예를 들어 해당 Stack이 다음 컴포넌트를 제공한다고 가정한다.

| 용도 | 배포된 컴포넌트 |
|---|---|
| Code Repository | GitLab CE |
| Package Registry | GitLab Package Registry |
| Image Registry | GitLab Registry |
| CI Platform | GitLab CI / GitLab Runner |
| CD Tool | Argo CD |

이 Stack을 선택하여 `orders-api` Pipeline을 만들면 다음과 같이 동작한다.

```text
[생성/프로비저닝]
nullus-devsecops-stack 선택
  -> GitLab endpoint 및 token reference 조회
  -> orders-api GitLab project 기존 선택 또는 생성
  -> GitLab Package Registry 대상 경로 구성
  -> GitLab Container Registry image repository 구성
  -> .gitlab-ci.yml commit 또는 project pipeline 설정
  -> 환경 저장소 선택 또는 신규 생성
  -> Argo CD Application 등록

[실행]
Developer push/merge to GitLab project
  -> GitLab Runner: build/test
  -> GitLab Package Registry: artifact + SBOM + test report 저장
  -> GitLab Registry: orders-api:<commit-sha> image push
  -> Environment Repository: image digest/tag 갱신 commit
  -> Argo CD: sync 및 target namespace 배포
  -> Nullus: provisioning/run/sync 상태 표시
```

이 경로에서는 Nullus API 서버가 정상 실행 과정에서 직접 `git clone`, `docker build`, `kind load`, `kubectl apply`를 수행하지 않는다. Nullus는 배포된 컴포넌트에 설정을 프로비저닝하고 그 실행 상태를 관리한다.

## 5. Integration Inventory

### 5.1 모델

Stack 모듈은 배포된 컴포넌트를 CI/CD가 사용할 수 있는 integration inventory 형태로 노출한다.

```go
type StackIntegration struct {
    ID                       string
    StackID                  string
    ComponentType            string // code_repository, package_registry, image_registry, ci_platform, cd_tool
    Provider                 string
    Endpoint                 string
    APIEndpoint              string
    CredentialRef            string
    HealthStatus             string // ready, degraded, unavailable, credential_required
    ProvisioningCapabilities []string
    Metadata                 map[string]any
}
```

### 5.2 조회 Port

CI/CD Context가 소유할 최소 포트는 다음 역할을 제공한다.

```go
type StackIntegrationReader interface {
    GetStackIntegrationProfile(ctx context.Context, stackID string) (*StackIntegrationProfile, error)
}

type StackIntegrationProfile struct {
    StackID      string
    OrgID        string
    ClusterID    string
    State        string
    Integrations []StackIntegration
}
```

Pipeline 생성 초안에는 Stack 상태 경고를 표시할 수 있지만, 실제 provisioning 및 runtime 실행에는 필요한 integration이 준비된 Stack이어야 한다.

## 6. Pipeline 모델

일반모드 Pipeline은 사용 중인 Stack과 각 컴포넌트 binding을 명시한다.

```text
execution_mode                     = stack_integrated
stack_id                           = selected Stack ID
code_repository_integration_id     = source repository binding
package_registry_integration_id    = artifact publish binding
image_registry_integration_id      = container image push binding
ci_platform_integration_id         = CI workflow/job binding
cd_tool_integration_id             = deployment sync binding
source_repo_mode                   = existing | create
source_repo_ref                    = provider repository/project identifier
environment_repo_mode              = existing | create
environment_repo_ref               = GitOps environment repository identifier
environment_path                   = manifest/values location
target_branch                      = deployment update branch
```

Pipeline 생성 이후 Stack의 설정이 변경될 수 있으므로, Pipeline은 프로비저닝 시 사용한 integration binding snapshot을 보존한다. 사용자가 명시적으로 재연결할 때에만 최신 Stack integration으로 갱신한다.

## 7. 검증 및 허용 정책

Compatibility Matrix는 일반모드 Pipeline의 **hard gate가 아니다**.

| 상태 | 처리 |
|---|---|
| 이미 검증된 조합 | 검증됨 표시 후 실행 허용 |
| 검증되지 않은 조합 | 위험 안내와 사용자 확인 후 실행 허용 |
| 알려진 경고가 있는 조합 | 경고 내용 및 확인 이력 기록 후 실행 허용 |

다음은 실행 자체가 불가능하므로 차단한다.

| Hard Block 조건 | 이유 |
|---|---|
| 필요한 component가 Stack에 없음 | 역할을 수행할 provider 부재 |
| component endpoint가 없음 | provider 호출 또는 publish 불가 |
| credential reference가 없음 | 인증이 필요한 연동 수행 불가 |
| 해당 provider adapter가 구현되지 않음 | Nullus가 프로비저닝/상태 조회 불가 |
| component health check가 실패함 | 현재 연결 또는 실행 불가 |
| provisioning 실행 시 Stack이 usable 상태가 아님 | 실제 CI/CD 도구 사용 불가 |

Stack 설치 전 수행하는 OSS/Kubernetes 호환성 검증은 이 정책과 별도이다. 여기서 warning-only로 정의하는 대상은 **배포된 Stack 컴포넌트를 CI/CD Pipeline에 조합하여 사용하는 정책**이다.

## 8. API 설계

### 8.1 Stack Integration 조회

```http
GET /api/v1/stacks/{stackId}/integrations
```

응답은 component별 provider, endpoint, health, provisioning capability, credential 준비 여부를 포함하되 credential 값은 노출하지 않는다.

### 8.2 Pipeline 생성

```http
POST /api/v1/cicd/pipelines
Content-Type: application/json

{
  "name": "orders-api",
  "execution_mode": "stack_integrated",
  "stack_id": "stk_nullus_devsecops",
  "code_repository_integration_id": "int_gitlab_source",
  "package_registry_integration_id": "int_gitlab_package",
  "image_registry_integration_id": "int_gitlab_image",
  "ci_platform_integration_id": "int_gitlab_ci",
  "cd_tool_integration_id": "int_argocd",
  "source_repo_mode": "existing",
  "source_repo_ref": "platform/orders-api",
  "environment_repo_mode": "create",
  "environment_repo_ref": "platform/orders-api-env",
  "compatibility_acknowledged": true
}
```

### 8.3 Provisioning

```http
POST /api/v1/cicd/pipelines/{pipelineId}/provision
GET  /api/v1/cicd/pipelines/{pipelineId}/provisionings
```

Provisioning은 CI workflow/job, Registry 연결 설정, 환경 저장소 구성, CD application을 provider에 생성하고 각 단계 상태를 기록한다.

### 8.4 Runtime 조회 및 Trigger

```http
GET  /api/v1/cicd/pipelines/{pipelineId}/runs
POST /api/v1/cicd/pipelines/{pipelineId}/trigger
```

일반모드 수동 실행은 CI provider pipeline을 trigger한다. 기존 직접 배포 API는 `emergency_direct` 모드에서의 하위 호환 실행 경로로 유지한다.

## 9. UI 흐름

CI/CD 생성 화면의 첫 단계는 Stack 선택이다.

```text
Selected Stack: nullus-devsecops-stack

Code Repository   GitLab CE                 Ready
Package Registry  GitLab Package Registry   Ready
Image Registry    GitLab Registry           Ready
CI Platform       GitLab CI / Runner        Ready
CD Tool           Argo CD                   Ready
Compatibility     Unverified - proceed after acknowledgement
```

화면 동작:

1. `completed` 또는 integration 사용 가능 상태의 Stack을 선택한다.
2. component별 endpoint 준비 여부와 상태를 확인한다.
3. 미검증 조합이면 경고를 표시하되 사용자가 승인하면 진행한다.
4. component 누락 또는 접속 불가이면 일반모드 실행 버튼을 비활성화한다.
5. 필요한 경우 별도 액션으로 `Emergency Direct Deploy`를 제공한다.
6. 생성 후 Pipeline 상세에서 Stack, component bindings, provisioning 상태, run/sync 이력을 표시한다.

## 10. 데이터 및 구현 범위

### 10.1 신규 저장 정보

| 저장 대상 | 내용 |
|---|---|
| Stack integration inventory | 컴포넌트 provider/endpoint/credential reference/health/capability |
| Pipeline binding | 선택 Stack 및 component integration 식별자, execution mode |
| Provisioning history | workflow/job/application 구성 단계, 결과, 실패 원인 |
| Provider run history | CI run, package publish, image push, CD sync 상태 |
| Compatibility acknowledgement | 미검증/경고 조합에 대한 사용자 승인 이력 |

### 10.2 기존 직접 배포와의 공존

| 경로 | 목적 | 기존 코드 처리 |
|---|---|---|
| `stack_integrated` | 일반 운영 CI/CD | 신규 구현 |
| `emergency_direct` | 장애 대응/Kind 검증/임시 복구 | 기존 코드 유지 |

## 11. 개발 단계

### Phase 1. Integration 조회 및 화면 연결

- Stack integration inventory 모델과 조회 API를 추가한다.
- CI/CD 생성 화면에서 Stack 선택 및 component 상태를 표시한다.
- Pipeline에 `execution_mode`와 integration binding을 저장한다.
- Compatibility 결과를 warning/acknowledgement 정책으로 표시한다.

### Phase 2. 현재 배포 Stack 기준 E2E

- 실제 배포된 `nullus-devsecops-stack`의 `config`에서 provider 구성을 읽는다.
- 해당 Stack이 GitLab 기반이면 GitLab Repository, Package Registry, Registry, GitLab CI/Runner, Argo CD를 잇는 기준 경로를 구현한다.
- provisioning 단계와 run/sync 상태를 이력 및 로그 화면에 제공한다.

### Phase 3. Provider Adapter 확장

- Code Repository, Package Registry, Image Registry, CI Platform, CD Tool별 provider adapter를 추가한다.
- 미검증 조합도 필요한 adapter와 연결 정보가 준비되어 있으면 실행할 수 있도록 한다.

### Phase 4. 운영 안정화

- Token rotation 후 integration credential 재주입 및 재검증을 지원한다.
- provider run, artifact/image publish, CD sync 오류를 통합 이력에 저장한다.
- 통합모드 실패 시 긴급모드 진입 이력과 운영자 승인 정보를 기록한다.

## 12. 테스트 및 완료 조건

| 시나리오 | 기대 결과 |
|---|---|
| 배포 완료 Stack 선택 | component inventory와 상태 표시 |
| GitLab 기반 `nullus-devsecops-stack` 연결 | Repository/Package/Image/CI/CD binding 저장 및 provisioning 가능 |
| 미검증 컴포넌트 조합 선택 | 승인 전 경고, 승인 후 실행 허용 |
| endpoint/credential/adapter 누락 | 일반모드 실행 차단 및 원인 표시 |
| 일반모드 실행 | Package Registry publish, Image Registry push, CD sync 결과 추적 |
| 일반모드 실행 중 | Nullus direct `kind load` 경로 사용 안 함 |
| 긴급모드 선택 | 기존 직접 배포 경로가 변경 없이 실행 가능 |

## 13. 전제

- `stack_integrated`가 신규 CI/CD 기능의 기본 운영 모드다.
- Stack은 단순 참조 대상이 아니라 CI/CD 실행 자원을 제공하는 integration source다.
- Compatibility Matrix는 CI/CD 컴포넌트 사용을 강제로 차단하지 않는다.
- 정상모드에서는 Image Registry의 immutable image tag 또는 digest를 배포 기준으로 사용한다.
- `emergency_direct` 코드는 삭제하지 않고 별도 문서의 긴급 경로로 유지한다.
