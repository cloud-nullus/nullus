# Narwhal 호환성 Seed Sources

**작성일**: 2026-04-19
**기능 ID**: F8 (DevSecOps Stack OSS 버전 호환성 관리)
**관련 마이그레이션**: `000042_seed_narwhal_compat_refresh`
**관련 소스**: `internal/stack/adapter/repository/memory_compatibility.go`
**관련 계획 문서**: `docs/plans/compatibility_matrix_plan.md` (Task 2)

---

## 1. 목적

이 문서는 Nullus Platform이 배포 위저드(Pre-Deploy Gate)에서 사용하는 **호환성 매트릭스(Compatibility Matrix)** 시드 데이터의 버전 픽(pin) 출처와 근거를 기록한다. 호환성 매트릭스는 세 계층에서 동일한 버전 고정값(canonical baseline)을 공유해야 한다.

1. **DB 계층** — `compatibility_matrices` / `golden_path_templates` (마이그레이션 `000042_seed_narwhal_compat_refresh`)
2. **인메모리 계층** — `MemoryCompatibilityRepository` (`internal/stack/adapter/repository/memory_compatibility.go`의 `narwhal*` 상수)
3. **문서 계층** — 본 문서

세 계층이 drift 되면 테스트/스테이징 환경과 실제 DB 환경에서 Pre-Deploy Gate의 verdict가 달라진다. 따라서 하나의 버전을 변경할 때는 반드시 세 계층을 **한 커밋에서 함께** 갱신한다.

---

## 2. 버전 출처 원칙 (Version Pinning Policy)

각 도구의 버전은 다음 우선순위로 결정한다.

1. **Narwhal (dasomel/narwhal) `VERSIONS.md`** — 분기별 검증 차트 세트. 대다수 도구가 이 기준을 따른다.
2. **Bitnami / 공식 Helm Chart 리포지토리** — Narwhal에서 제외되거나 최근 릴리스가 늦게 반영되는 항목(Argo CD, Grafana 등)에 한하여 Helm chart `appVersion` 기준.
3. **외부 SaaS** — GitHub / GitHub Actions 처럼 클러스터 내에 설치되지 않는 서비스는 `helm_version=external`, `app_version=external`로 고정.

버전 선택 시 고려한 공통 제약은 다음과 같다.

- **Kubernetes min 버전**: 플랫폼 계열(GitLab, GitHub, Harbor) `1.27`, 그 외 워크로드 계열 `1.26`. Narwhal이 검증한 최저 K8s 라인을 따랐으며, EKS / GKE의 LTS 지원 구간과도 일치한다.
- **아키텍처**: Harbor 및 GitLab 계열 차트는 공식 arm64 이미지를 2026-Q1 기준으로 아직 발행하지 않는다. 그 외 도구는 amd64/arm64 듀얼 아키 이미지를 제공한다.
- **Tier**: 매트릭스 `status`가 `verified`이면 tool tier는 `stable`, `untested`이면 `beta`, `unsupported`이면 `deprecated`로 매핑한다(`000041_compat_tool_fields` 규칙과 동일).

---

## 3. 도구별 버전 매핑

아래 표는 `000042_seed_narwhal_compat_refresh`가 세 매트릭스에 공통적으로 주입하는 Narwhal baseline v1 값이다.

| 카테고리 | 도구 | Helm 차트 버전 | App 버전 | Min K8s | 아키텍처 | 출처 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `source_repository` | GitLab CE | `9.5.1` | `18.5.1` | `1.27` | `amd64` | Narwhal VERSIONS.md (GitLab 18.x 라인) |
| `ci_platform` | GitLab CI | `9.5.1` | `18.5.1` | `1.27` | `amd64` | GitLab 차트에 포함 (동일 패키지) |
| `container_registry` | GitLab Registry | `9.5.1` | `18.5.1` | `1.27` | `amd64` | GitLab 차트에 포함 (동일 패키지) |
| `container_registry` | Harbor | `1.15.0` | `2.11.0` | `1.27` | `amd64` | Narwhal VERSIONS.md (Harbor 2.11 LTS) |
| `storage_backend` | MinIO | `5.2.0` | `RELEASE.2024-08-03T04-33-23Z` | `1.26` | `amd64,arm64` | MinIO 공식 차트 (Narwhal 2024-08 스냅샷 일치) |
| `cd_tool` | Argo CD | `6.8.0` | `v2.8.3` | `1.26` | `amd64,arm64` | Argo CD 공식 Helm chart (LTS 2.8 라인) |
| `monitoring_collection` | Prometheus | `67.0.0` | `v2.54.1` | `1.26` | `amd64,arm64` | kube-prometheus-stack (Prometheus 2.54.x) |
| `monitoring_visualization` | Grafana | `8.5.0` | `11.1.0` | `1.26` | `amd64,arm64` | Grafana 공식 차트 (Grafana 11.1 라인) |
| `source_repository` | GitHub | `external` | `external` | `1.27` | `amd64,arm64` | 외부 SaaS — 클러스터 미설치 |
| `ci_platform` | GitHub Actions | `external` | `external` | `1.27` | `amd64,arm64` | 외부 SaaS — Self-hosted Runner 가정 시에도 버전은 Runner가 소유 |

---

## 4. Golden Path별 구성

### 4.1 `gitlab-allinone-v1` — GitLab All-in-One

GitLab CE를 중심으로 소스 저장소 / CI / 레지스트리를 한 패키지에서 제공한다. 상태 `verified`, 모든 도구의 `Tier=stable`. amd64 전용이므로 Pre-Deploy Gate는 arm64 대상 클러스터 선택 시 `fail` 이다.

### 4.2 `gitlab-argocd-v1` — GitLab + Argo CD

GitLab은 동일하게 GitLab CE 차트를 사용하되, CD를 Argo CD로 대체한 구성이다. GitOps 패턴을 선호하는 팀 대상. 상태 `verified`.

### 4.3 `github-argocd-v1` — GitHub + Argo CD

소스/CI는 GitHub SaaS를 외부 시스템으로 가정하고, 클러스터 내부에는 Harbor + Argo CD + MinIO + Prometheus + Grafana 만 설치한다. Narwhal에서 독립 검증 트랙이 아직 부재하여 상태 `untested`, 모든 도구 `Tier=beta`. Harbor 구성요소는 `amd64` 전용이므로 arm64 노드 클러스터에서는 마찬가지로 차단된다.

---

## 5. 업데이트 규칙

1. **소스 변경**: Narwhal `VERSIONS.md` 가 새 분기를 릴리스하면, 아래 세 곳을 한 번의 PR에서 수정한다.
   - `db/migrations/000042_seed_narwhal_compat_refresh.up.sql`의 JSONB 리터럴
   - `internal/stack/adapter/repository/memory_compatibility.go`의 `narwhal*` 상수
   - 본 문서의 § 3 표
2. **신규 도구 추가**: § 3 표에 행을 추가하고, 어느 Golden Path에 편입되는지 § 4 에 명시한다. 신규 도구에 대한 `MinK8sVersion` / `ArchSupport` 는 `000041_compat_tool_fields` 가 정의한 규칙을 따른다.
3. **Tier 변경**: `verified ↔ untested ↔ unsupported` 전이가 발생하면 `000042`의 후속 마이그레이션을 신설하거나 별도 `NNNN_narwhal_baseline_bumpX` 마이그레이션으로 반영한다. `000042` 자체는 **재확정(reassert)** 의 의미이므로 기존 파일을 직접 수정하지 않는다.
4. **검증**: 수정 후 `make test` 또는 `go test ./internal/stack/...` 로 `TestMemoryCompatibilityRepository_ToolV2Fields` / `TestMemoryCompatibilityRepository_NarwhalBaselineVersions` 가 통과하는지 확인한다.

---

## 6. 참고 링크 (internal)

- Narwhal 프로젝트: https://github.com/dasomel/narwhal (외부 리포지토리, 버전 라인은 `VERSIONS.md` 참고)
- F8 기능 설계: `docs/plans/compatibility_matrix_plan.md`
- DB 스키마 정의: `docs/20_아키텍처/Nullus_DB_스키마.md` §8 (Context 3: Stack)
- 마이그레이션 히스토리: `db/migrations/000008_*`, `000024_*`, `000026_*`, `000033_*`, `000041_*`, `000042_*`
