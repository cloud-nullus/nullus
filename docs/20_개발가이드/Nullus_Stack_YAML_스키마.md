# Nullus `stack.yaml` 선언형 스키마 (v1alpha1)

- **상태**: 확정 (CLI+MCP 구현 백로그 0-2 산출물)
- **작성일**: 2026-08-23
- **관련 문서**: [CLI 컨셉](./Nullus_CLI_컨셉.md) · [Automation 계약](./Nullus_CLI_Automation_계약.md) · [CLI+MCP 구현 백로그](../plans/2026-08-22-cli-mcp-구현-백로그.md)
- **대상 명령**: `nullus stack deploy -f stack.yaml` (트랙 A-5) · MCP `stack_deploy` tool (트랙 B-3)

> 웹 마법사의 대화형 입력을 파일 하나로 대체하는 선언형 형식이다. 스키마의 진실 원천은 서버의 생성 API payload이며, 본 문서는 그 대응 관계를 고정한다.

---

## 1. 설계 결정

### 1.1 `config`는 서버 API payload와 1:1 (변환 없음)

`config` 블록은 `POST /api/v1/stacks` 요청의 `config`(`internal/stack/domain/config.go`의 `StackConfig`)와 **필드·이름·구조가 동일**하다(snake_case). CLI는 YAML→JSON 직렬화만 하고 자체 변환 계층을 두지 않는다.

- **이유**: CLI는 얇은 클라이언트다(컨셉 §5). CLI 소유 스키마를 따로 두면 서버 API가 변할 때마다 매핑 계층을 유지해야 한다 — ADR-0001이 감수한 이중 유지비를 더 키운다.
- **대가**: 서버 API의 breaking change가 파일 형식에 그대로 전파된다. 그래서 `apiVersion: v1alpha1`로 시작하고, F5·F6 안정화(README "구현됨" 전환) 후 `v1`로 승격하면서 호환성을 고정한다.
- CLI가 소유하는 것은 **top-level 필드의 매핑과 편의 해석뿐**이다(§1.2).

### 1.2 top-level은 CLI 소유 (마법사 상단 입력에 대응)

| stack.yaml | 생성 API payload | 편의 해석 |
|---|---|---|
| `name` | `name` | 필수 |
| `cluster` | `cluster_id` | **클러스터 이름 또는 ID** — 이름이면 CLI가 `GET /api/v1/admin/clusters`로 ID 해석. 중복 이름이면 exit 2 |
| `namespace` | `namespace` | 생략 시 서버 기본값(`nullus`) |
| `template` | `golden_path_id` | 생략 시 `""` (커스텀 구성) |

### 1.3 시크릿·런타임 결정은 파일에 넣지 않는다

배포 본문(`POST /stacks/:id/deploy`)으로만 전달되는 값은 스키마에서 **금지**한다:

| 값 | 전달 방법 | 배포 API 필드 |
|---|---|---|
| SCM PAT (GitHub 토큰) | `--scm-token` 플래그 또는 `NULLUS_SCM_TOKEN` env | `source_control.personal_access_token` |
| 호환성 경고 승인 | `--ack-warnings` 플래그 | `acknowledge_warnings` |

- PAT는 서버도 config에 저장하지 않는 정책(`deploy_handler.go` — OpenBao로 이동)이므로 파일 금지가 서버 정책과 일치한다.
- 경고 승인은 "이번 실행의 결정"이지 구성이 아니다. 파일에 넣으면 리뷰 없이 영구 승인된다.
- CLI는 파일에서 `personal_access_token`류 키를 발견하면 exit 2로 거부한다.

### 1.4 `stack deploy -f`는 생성+배포 2단계를 감싼다

웹 마법사의 Deploy 버튼과 동일하게 (1) `POST /stacks`로 생성 → id 획득, (2) `POST /stacks/:id/deploy`로 배포를 순차 호출한다. 생성만 하려면 `--create-only`. 실패 지점에 따라 exit code는 [Automation 계약](./Nullus_CLI_Automation_계약.md)을 따른다 (스키마 오류·호환성 게이트 400 → 2, 배포 시작 후 실패 → 6).

---

## 2. 파일 형식 (전체 예시)

```yaml
apiVersion: nullus.io/v1alpha1
kind: Stack
name: team-alpha
cluster: prod-01                      # 이름 또는 클러스터 ID
namespace: nullus-team-alpha
template: gitlab-argocd-v1            # 생략 가능 (커스텀 구성)

config:
  access_domain: team-alpha.internal  # 생략 시 <name>.internal
  access_domain_tls:
    enabled: false
  authentication:
    provider: openbao                 # "" | openbao
  source_control:                     # 외부 SCM(GitHub 등) 쓸 때만. PAT는 여기 금지(§1.3)
    owner: my-org
    api_base_url: https://ghe.example.com/api/v3

  artifacts:
    package_registry:   { name: gitlab,          version: v17.7.0, enabled: true }
    source_repository:  { name: gitlab,          version: v17.7.0, enabled: true }
    container_registry: { name: gitlab-registry, version: v17.7.0, enabled: true }
    storage_backend:    { name: minio,           version: latest,  enabled: true }
  pipeline:
    ci_platform: { name: gitlab-ci, version: v17.7.0, enabled: true }
    cd_tool:     { name: argocd,    version: v2.13.3, enabled: true }
  monitoring:
    collection:    { name: prometheus, version: v3.1.0, enabled: true }
    visualization: { name: grafana,    version: 11.5.1, enabled: true }
  logging:
    search:      { name: opensearch, version: 2.14.0, enabled: true }
    trace_layer: { name: tempo,      version: 2.7.0,  enabled: true }

  resources:
    developers: 10
    concurrent_runners: 5
    weekly_commits: 50
    build_frequency: medium           # low | medium | high

  storage:
    plan_mode: integrated-create      # integrated-create | existing-connect
    database:
      mode: create                    # create | existing-connect
      provider_or_engine: postgres
      version: "17"
      size: 50                        # Gi
    object_storage:
      mode: create
      provider_or_engine: minio
      size: 100

  # 고급: 도구별 매니페스트/values 오버라이드 (웹 YAML View 탭에 대응)
  yaml_overrides:
    installing_gitlab: |
      gitlab:
        webservice:
          resources:
            requests: { cpu: 400m, memory: 1Gi }
  applied_resource_overrides:         # 지정 시 서버의 리소스 자동 계획을 건너뜀
    gitlab:
      cpuRequest: 2
      cpuLimit: 4
      memoryRequestGi: 4
      memoryLimitGi: 8
      storageRequestGi: 100
      storageLimitGi: 100
```

`config` 하위 필드의 전체 목록·타입은 `internal/stack/domain/config.go`가 진실 원천이다. 위 예시에 없는 필드(`option_overrides`, `row_units`, `storage_class`, TLS 세부 등)도 API와 동일 이름으로 그대로 쓸 수 있다.

---

## 3. 마법사 → stack.yaml → API 대응표

웹 마법사(실제 9탭: Authentication → Artifacts → CI/CD → Observability → Storage → Resources → YAML View → Preview Deploy Script → Dry Run)의 입력이 어디로 가는지:

| 마법사 탭/입력 | stack.yaml | 생성 API payload | 백엔드 Go 필드 |
|---|---|---|---|
| 상단: Stack Name | `name` | `name` | `createStackRequest.Name` |
| 상단: Target Cluster | `cluster` | `cluster_id` | `.ClusterID` |
| 상단: Namespace | `namespace` | `namespace` | `.Namespace` |
| 템플릿 선택 | `template` | `golden_path_id` | `.TemplateID` |
| Authentication: 도메인/TLS/provider/SCM | `config.access_domain`·`access_domain_tls`·`authentication`·`source_control` | 동일 | `StackConfig.*` |
| Artifacts (4개 카테고리) | `config.artifacts.*` | 동일 | `.Artifacts` |
| CI/CD (CI 플랫폼·CD 도구) | `config.pipeline.{ci_platform,cd_tool}` | 동일 | `.Pipeline` |
| Observability (메트릭·시각화·로그·트레이스) | `config.monitoring.*`·`config.logging.*` | 동일 | `.Monitoring`·`.Logging` |
| Storage | `config.storage.*` | 동일 | `.Storage` |
| Resources (개발자 수 등 4입력 + 행별 오버라이드) | `config.resources.*`·`applied_resource_overrides`·`option_overrides`·`row_units` | 동일 | `.Resources` 외 |
| YAML View (렌더/편집 매니페스트) | `config.yaml_overrides` | 동일 | `.YAMLOverrides` |
| Preview Deploy Script / Dry Run | — (CLI 범위 밖, 조회성) | — | — |
| 배포 시: SCM PAT | **파일 금지** → `--scm-token`/env | 배포 본문 `source_control.personal_access_token` | `deployRequest.SourceControl` |
| 배포 시: 경고 승인 | **파일 금지** → `--ack-warnings` | 배포 본문 `acknowledge_warnings` | `.AcknowledgeWarnings` |

참고 구현 세부 (프론트 대응 코드 — 스키마와 무관하게 참고용):

- 마법사 camelCase→API snake_case 변환의 단일 출처: `web/src/features/stack/api/stack-normalizers.ts` `toCreateStackBody()`
- 마법사 도구 선택은 `{tool, version}` 형태지만 API는 `{name, version, enabled}` — stack.yaml은 **API 형태를 그대로 쓴다**
- 프론트가 보내는 `monitoring.visualizations`(복수)는 백엔드가 받지 않으므로 스키마에서 제외

## 4. CLI 검증 규칙 (exit 2 대상)

1. `apiVersion`은 `nullus.io/v1alpha1`, `kind`는 `Stack`이어야 한다 (미지의 버전 → 명확한 안내와 함께 거부)
2. `name`, `cluster` 필수. `cluster`가 이름일 때 해석 불가·중복이면 거부
3. 시크릿 키(`personal_access_token` 등) 발견 시 거부 (§1.3)
4. 알 수 없는 top-level 필드는 거부, `config` 내부는 **서버 검증에 위임** (CLI는 구조만 확인 — 도구명·버전 유효성은 서버 400/호환성 게이트가 판정)

## 5. 웹 마법사 "yaml 내보내기" — 결정

**추가를 권고하되 CLI 착수를 막지 않는 별도 태스크로 분리한다.**

- 마법사에는 이미 YAML View 탭과 `toCreateStackBody()` 조립 로직이 있어, 그 결과를 본 스키마로 직렬화하는 작업은 소규모다 (top-level 4필드 + config 그대로)
- 가치: 사용자가 마법사로 탐색·확정한 구성을 내보내 CI에서 `nullus stack deploy -f`로 재사용 — 스키마 검증을 겸하는 왕복 경로
- 처리: 백로그 트랙 A에 **A-9 (S, 선택): 웹 마법사 Export stack.yaml** 후보로 등록. 웹(ui 모듈) 작업이므로 CLI 트랙과 독립

## 6. 버저닝 정책

- `v1alpha1` (현재): 필드 추가·변경 가능. CLI는 파일의 `apiVersion`을 보고 지원 여부를 판정한다
- `v1` 승격 조건: F5·F6 README "구현됨" 전환 + automation 계약 v1.0 고정과 동시
- `v1` 이후: 필드 추가는 하위 호환, 제거·의미 변경은 새 apiVersion으로만
