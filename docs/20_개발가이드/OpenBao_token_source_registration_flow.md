# OpenBao Token Source Registration Flow

## 1. 목적

이 문서는 Nullus 스택 설치 완료 후 token source를 등록하고, 실제 토큰 값을 OpenBao에 저장하는 현재 구현 흐름을 설명한다.

핵심 목표는 다음과 같다.

- 스택에서 선택된 도구별 token source 경로를 `token_sources` 테이블에 등록한다.
- 원문 토큰 값은 DB에 저장하지 않고 OpenBao에 저장한다.
- OpenBao 저장 성공/실패 상태는 `token_sources.metadata`에 기록한다.
- GitHub provider는 `NULLUS_GITHUB_TOKEN` 또는 `GITHUB_TOKEN` 환경변수 값을 초기 token value로 사용할 수 있다.

## 2. 관련 코드

| 파일 | 역할 |
|---|---|
| `cmd/api/main.go` | OpenBao secret router 생성, admin token source API 등록, stack install usecase에 의존성 주입 |
| `internal/shared/secrets/store.go` | secret provider router 추상화 |
| `internal/shared/secrets/openbao_store.go` | OpenBao KV read/write 구현 |
| `internal/stack/usecase/install_stack.go` | 설치 완료 후 token source 목록 생성 |
| `internal/stack/adapter/repository/postgres_token_source_registry.go` | OpenBao write 및 `token_sources` upsert |
| `internal/stack/port/token_source_registry.go` | stack usecase가 의존하는 token source registry port |

## 3. 서버 부팅 시 secret router 구성

API 서버는 부팅 시 `OPENBAO_ADDR`, `OPENBAO_TOKEN` 환경변수를 확인한다.

두 값이 모두 있으면 `newSecretRouterFromEnv()`가 `secrets.Router`를 만들고, `openbao` provider를 등록한다.

```go
router := secrets.NewRouter()
router.Register("openbao", secrets.NewOpenBaoStore(addr, token))
```

이 router는 다음 두 경로에 주입된다.

| 주입 대상 | 용도 |
|---|---|
| `TokenSourceUseCase` | admin token management API에서 reveal/read 동작 지원 |
| `InstallStack` / `PostgresTokenSourceRegistry` | 스택 설치 완료 후 token source 등록 및 OpenBao write |

환경변수가 없으면 router는 `nil`이다. 이 경우 token source DB 등록은 가능하지만 OpenBao write는 수행되지 않는다.

## 4. 설치 완료 후 token source 등록

스택 설치 usecase는 설치가 성공적으로 완료된 뒤 `registerStackTokenSources()`를 호출한다.

```go
if err := uc.registerStackTokenSources(ctx, stack); err != nil {
    slog.Warn("token source registration failed", "stack_id", stack.ID, "error", err)
}
```

이 단계는 설치 성공 자체를 되돌리지 않는다. token source 등록 실패는 warning으로 남기고 설치 완료 상태는 유지한다.

`registerStackTokenSources()`는 다음 조건을 만족할 때만 동작한다.

- `tokenRegistry`가 주입되어 있어야 한다.
- stack config를 `StackConfig`로 읽을 수 있어야 한다.
- `config.authentication.provider`가 `openbao`여야 한다.

즉 OpenBao를 선택하지 않은 스택은 이 등록 흐름을 타지 않는다.

## 5. token source path 생성 규칙

기본 경로 형식은 다음과 같다.

```text
kv/nullus/{env}/{org_id}/{module}/{provider}/token
```

예시:

```text
kv/nullus/dev/org-1/artifacts/github/token
kv/nullus/dev/org-1/pipeline/argocd/token
```

`env`는 `WithTokenSourceRegistry(registry, env)`로 주입된 값을 사용한다. 값이 비어 있으면 기본값은 `dev`다.

현재 자동 등록 대상은 stack config에서 선택된 주요 도구다.

| module | provider source |
|---|---|
| `artifacts` | source repository |
| `artifacts` | container registry |
| `pipeline` | CI platform |
| `pipeline` | CD tool |

추가로 생성형/부트스트랩 리소스가 선택된 경우 access path도 등록한다.

| 조건 | 등록 예시 |
|---|---|
| PostgreSQL create mode | `kv/nullus/dev/{org}/storage/postgresql/access` |
| MinIO storage backend enabled | `kv/nullus/dev/{org}/artifacts/minio/access` |
| Argo CD enabled | `kv/nullus/dev/{org}/pipeline/argocd/access` |
| GitLab source repository enabled | `kv/nullus/dev/{org}/artifacts/gitlab/access` |

## 6. GitHub token value 선택

GitHub 또는 GitHub Actions provider는 실제 token value를 환경변수에서 찾는다.

우선순위는 다음과 같다.

1. `NULLUS_GITHUB_TOKEN`
2. `GITHUB_TOKEN`
3. `managed-by-nullus`

`managed-by-nullus`는 placeholder다. 실제 외부 인증에 사용할 값이 아니라, Nullus가 관리 대상으로 등록했다는 표시다.

## 7. OpenBao write 및 DB upsert

`PostgresTokenSourceRegistry.Upsert()`는 다음 순서로 동작한다.

1. secret manager 이름을 결정한다.
2. `TokenValue`가 비어 있지 않고 secret router가 있으면 OpenBao write를 시도한다.
3. write 결과를 `metadata`에 기록한다.
4. `token_sources` 테이블에 upsert한다.

OpenBao write 성공 시 metadata:

```json
{
  "secret_manager": "openbao",
  "secret_write_status": "stored"
}
```

OpenBao write 실패 시 metadata:

```json
{
  "secret_manager": "openbao",
  "secret_write_status": "failed",
  "secret_write_error": "openbao write failed: ..."
}
```

중요한 점은 OpenBao write 실패가 곧바로 설치 실패로 전파되지 않는다는 것이다. 실패 정보는 DB metadata에 남기고, 운영자가 admin token management 화면/API에서 상태를 확인할 수 있게 한다.

## 8. DB 저장 정책

`token_sources`에는 원문 토큰을 저장하지 않는다.

저장되는 값은 다음과 같은 메타데이터다.

- 조직 ID
- module
- provider
- OpenBao path
- token type
- status
- next check time
- metadata

원문 token value는 OpenBao write에만 사용된다. DB는 token source의 상태와 경로를 추적하는 용도다.

## 9. 확인 방법

스택 설치 후 DB에서 다음 쿼리로 등록 상태를 확인할 수 있다.

```sql
select provider, path, token_type, status, metadata
from token_sources
where org_id = '<org_id>'
order by updated_at desc;
```

GitHub token이 OpenBao에 정상 저장되었으면 `metadata.secret_write_status`가 `stored`다.

```json
{
  "secret_manager": "openbao",
  "secret_write_status": "stored"
}
```

실패한 경우에는 `failed`와 함께 `secret_write_error`가 기록된다.

## 10. 현재 검증 상태

단위/패키지 테스트:

```powershell
go test ./internal/admin/usecase ./internal/admin/adapter/handler ./internal/stack/usecase ./internal/stack/adapter/repository ./cmd/api -count=1
```

Kind 기반 Golden Path smoke:

```powershell
go test -tags e2e -run "^TestF8Task6_GoldenPath_KindDeploy/github-argocd-v1$" -timeout 20m -v ./e2e/...
```

주의: 현재 kind smoke의 `github-argocd-v1` 시나리오는 리소스 절감을 위해 GitHub/GitHub Actions 설치 step을 skip한다. 따라서 설치 orchestration 회귀는 확인하지만, OpenBao에 GitHub token이 실제 저장되는 통합 검증까지 포함하지는 않는다.

OpenBao write까지 확인하려면 API 실행 환경에 다음 값이 필요하다.

```env
OPENBAO_ADDR=http://...
OPENBAO_TOKEN=...
NULLUS_GITHUB_TOKEN=...
NULLUS_ENV=dev
```

## 11. 운영상 주의사항

- `OPENBAO_ADDR`, `OPENBAO_TOKEN`이 없으면 secret router가 생성되지 않는다.
- `authentication.provider=openbao`가 아닌 스택은 token source 자동 등록 대상이 아니다.
- OpenBao write 실패는 설치 실패가 아니라 metadata 상태로 남는다.
- `managed-by-nullus` placeholder는 실제 credential이 아니다.
- provider별 실제 reissue/rotation에 필요한 추가 metadata는 별도 구현이 필요하다.
