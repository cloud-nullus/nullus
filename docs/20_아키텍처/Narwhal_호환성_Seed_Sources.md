# 호환성 매트릭스 버전 출처

**최초 작성**: 2026-04-19 (Narwhal baseline v1)
**개정**: 2026-08-12 — 기준선을 Nullus 설치 경로로 이관
**기능 ID**: F8 (DevSecOps Stack OSS 버전 호환성 관리)
**관련 마이그레이션**: `000042_seed_narwhal_compat_refresh`, `000062_compat_baseline_matches_install`
**관련 소스**: `internal/stack/domain/connection.go`, `internal/stack/adapter/repository/memory_compatibility.go`

> 파일명에 `Narwhal` 이 남아 있는 것은 CHANGELOG·계획 문서의 링크를 살리기 위해서다.
> 현재 기준선은 Narwhal 이 아니다 — § 2 참고.

---

## 1. 목적

호환성 매트릭스(Compatibility Matrix)는 배포 위저드의 Pre-Deploy Gate가 "검증된 조합"으로 화면에 보여주는 버전 표다. 이 문서는 그 버전이 어디서 오는지를 기록한다.

값은 세 계층이 공유한다.

1. **DB 계층** — `compatibility_matrices` / `golden_path_templates`
2. **인메모리 계층** — `MemoryCompatibilityRepository` (`memory_compatibility.go`의 `baseline*` 상수)
3. **설치 계층** — `defaultChartSpecForStep` (`helm_step_metadata.go`)

세 계층이 갈라지면 테스트/스테이징과 실제 DB 환경에서 Pre-Deploy Gate의 verdict가 달라지고, 무엇보다 **화면이 안내한 버전과 실제로 깔리는 버전이 달라진다.**

---

## 2. 기준선의 출처 (2026-08 개정)

**현재 기준선은 Nullus 가 실제로 설치하는 차트 버전이다.** 단일 출처는 `internal/stack/domain/connection.go` 의 상수다.

```
domain.ArgoCDChartVersion ──┬──> defaultChartSpecForStep("installing_argocd")   (설치)
                            └──> baselineArgoCDHelmVersion                       (매트릭스)
```

### 왜 바꿨나

2026-04 최초 구성에서는 외부 프로젝트 **Narwhal**(`dasomel/narwhal`)의 `VERSIONS.md` 를 1순위 기준으로 삼았다. Narwhal 은 Vagrant 기반 K8s IDP 프로비저닝 도구로, Nullus 의 Install Engine이 풀어야 할 문제를 셸 스크립트로 먼저 풀어 본 프로토타입이다(경쟁 분석 보고서 § Narwhal 참고). 설치 순서 DAG와 Helm edge case는 지금도 유효한 참고 자료다.

문제는 **버전 픽까지 외부 프로젝트를 따라간 것**이었다. Nullus 의 설치 경로가 독자적으로 올라가면서 두 값이 갈라졌고, 아무도 눈치채지 못했다. 2026-08-12 실측:

| 도구 | 매트릭스(Narwhal v1) | 실제 설치 |
| :--- | :--- | :--- |
| GitLab CE/CI/Registry | `9.5.1` / `18.5.1` | `8.7.2` / `v17.7.0` |
| MinIO | `5.2.0` / `2024-08-03` | `5.4.0` / `2024-12-18` |
| Argo CD | `6.8.0` / `v2.8.3` | `7.7.16` / `v2.13.3` |
| Prometheus | `67.0.0` / `v2.54.1` | `69.3.0` / `v3.1.0` |
| Grafana | `8.5.0` / `11.1.0` | `8.9.0` / `11.5.1` |
| Harbor / Nexus | (일치) | (일치) |

Harbor 와 Nexus 만 어긋나지 않았는데, 그 둘만 `domain` 상수를 참조하고 있었기 때문이다. 나머지는 매트릭스·설치·테스트가 각자 리터럴을 들고 있었다. `TestChartVersionsMatchCompatibilityMatrix` 도 Harbor/Nexus 두 개만 검사해 나머지 드리프트를 못 잡았다.

`000062_compat_baseline_matches_install` 이 DB 를 실제 설치 값으로 맞췄고, 드리프트 테스트를 클러스터 내부에 설치되는 전 도구로 넓혔다.

### 버전 선택 규칙

1. **`domain` 상수** — 클러스터에 설치되는 모든 도구. 차트 버전을 올릴 때 이 상수만 고치면 설치와 매트릭스가 함께 따라온다.
2. **외부 SaaS** — GitHub / GitHub Actions / GHCR 처럼 클러스터에 설치되지 않는 것은 `helm_version=external`, `app_version=external`.
3. **편집 값** — `MinK8sVersion`, `ArchSupport`, `Tier` 는 차트에서 끌어올 수 없다. 아래 제약을 따른다.
   - **Kubernetes min**: 플랫폼 계열(GitLab, GitHub, Harbor) `1.27`, 그 외 워크로드 계열 `1.26`. EKS / GKE 의 LTS 지원 구간과 일치한다.
   - **아키텍처**: Harbor 및 GitLab 계열 차트는 공식 arm64 이미지를 2026-Q1 기준 미발행. 그 외는 amd64/arm64 듀얼.
   - **Tier**: 매트릭스 `status` 가 `verified` 면 `stable`, `untested` 면 `beta`, `unsupported` 면 `deprecated` (`000041_compat_tool_fields` 규칙과 동일).

### AppVersion 예외 하나

`kube-prometheus-stack` 차트의 `appVersion` 은 **prometheus-operator** 버전(`v0.80.0`)이다. 그대로 옮기면 화면에 오퍼레이터 버전이 Prometheus 버전인 양 뜬다. 사용자가 알아야 하는 것은 실제로 서는 Prometheus 서버 버전이므로 `v3.1.0` 을 적는다. 나머지 도구는 차트 `appVersion` 과 같다.

---

## 3. 현재 버전 (2026-08-12)

| 카테고리 | 도구 | Helm 차트 | App | Min K8s | 아키텍처 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `source_repository` | GitLab CE | `8.7.2` | `v17.7.0` | `1.27` | `amd64` |
| `ci_platform` | GitLab CI | `8.7.2` | `v17.7.0` | `1.27` | `amd64` |
| `container_registry` | GitLab Registry | `8.7.2` | `v17.7.0` | `1.27` | `amd64` |
| `container_registry` | Harbor | `1.15.0` | `2.11.0` | `1.27` | `amd64` |
| `package_registry` | Nexus | `64.2.0` | `3.64.0` | `1.27` | `amd64` |
| `storage_backend` | MinIO | `5.4.0` | `RELEASE.2024-12-18T13-15-44Z` | `1.26` | `amd64,arm64` |
| `cd_tool` | Argo CD | `7.7.16` | `v2.13.3` | `1.26` | `amd64,arm64` |
| `monitoring_collection` | Prometheus | `69.3.0` | `v3.1.0` | `1.26` | `amd64,arm64` |
| `monitoring_visualization` | Grafana | `8.9.0` | `11.5.1` | `1.26` | `amd64,arm64` |
| `source_repository` | GitHub | `external` | `external` | `1.27` | `amd64,arm64` |
| `ci_platform` | GitHub Actions | `external` | `external` | `1.27` | `amd64,arm64` |
| `container_registry` | GHCR | `external` | `external` | `1.27` | `amd64,arm64` |

GitLab 차트 하나가 소스 저장소·CI·레지스트리를 겸하므로 세 항목의 버전은 항상 같다.

---

## 4. Golden Path별 구성

### 4.1 `gitlab-allinone-v1` — GitLab All-in-One

GitLab CE를 중심으로 소스 저장소 / CI / 레지스트리를 한 패키지에서 제공한다. 상태 `verified`, 모든 도구 `Tier=stable`. amd64 전용이므로 Pre-Deploy Gate는 arm64 대상 클러스터 선택 시 `fail` 이다.

### 4.2 `gitlab-argocd-v1` — GitLab + Argo CD

GitLab CE 차트는 동일하고 CD를 Argo CD로 대체한 구성이다. GitOps 패턴을 선호하는 팀 대상. 상태 `verified`.

### 4.3 `github-argocd-v1` — GitHub + Argo CD

소스/CI는 GitHub SaaS를 외부 시스템으로 가정하고, 클러스터 내부에는 Argo CD + MinIO + Prometheus + Grafana 만 설치한다. 레지스트리는 GHCR 이다 — GitHub 호스티드 러너는 클러스터 내부 Harbor 에 닿을 수 없기 때문이다(`000060_github_stack_uses_ghcr`). 독립 검증 트랙이 아직 없어 상태 `untested`, 모든 도구 `Tier=beta`.

---

## 5. 업데이트 규칙

1. **버전 올리기**: 아래 세 곳을 한 커밋에서 함께 수정한다.
   - `internal/stack/domain/connection.go` 의 상수 (설치·매트릭스가 함께 따라온다)
   - 새 마이그레이션 (`000062` 를 본떠 도구 이름으로 찾아 버전 필드만 갱신한다 — 블롭을 통째로 덮으면 이후 마이그레이션이 바꾼 값이 되돌아간다)
   - 본 문서 § 3 표
2. **신규 도구 추가**: § 3 표에 행을 추가하고 어느 Golden Path에 편입되는지 § 4 에 명시한다. `defaultChartSpecForStep` 에 설치 단계를 만들었다면 `TestChartVersionsMatchCompatibilityMatrix` 의 케이스 목록에도 추가한다.
3. **Tier 변경**: `verified ↔ untested ↔ unsupported` 전이는 새 마이그레이션으로 반영한다. `000042` 는 재확정(reassert)의 의미이므로 직접 수정하지 않는다.
4. **검증**: `go test ./internal/stack/...` 로 `TestChartVersionsMatchCompatibilityMatrix` / `TestMemoryCompatibilityRepository_BaselineVersionsAreShared` / `TestMemoryCompatibilityRepository_ToolV2Fields` 통과 확인.

---

## 6. 참고 링크

- Narwhal 프로젝트: https://github.com/dasomel/narwhal — 설치 순서 DAG와 Helm edge case의 참고 자료. **버전 기준선으로는 더 이상 쓰지 않는다.**
- F8 기능 설계: `docs/plans/compatibility_matrix_plan.md`
- DB 스키마 정의: `docs/20_아키텍처/Nullus_DB_스키마.md` §8 (Context 3: Stack)
- 마이그레이션 히스토리: `db/migrations/000008_*`, `000024_*`, `000026_*`, `000033_*`, `000041_*`, `000042_*`, `000060_*`, `000062_*`
