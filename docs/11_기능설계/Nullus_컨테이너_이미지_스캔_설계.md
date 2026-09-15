# Nullus 컨테이너 이미지 취약점 스캔 설계 (Trivy)

> 대상 이슈: [cloud-nullus/nullus-plan#76](https://github.com/cloud-nullus/nullus-plan/issues/76) — [Level 3] EPIC: 컨테이너 이미지 취약점 스캔 — Trivy 파이프라인 단계화
> 축: **A축 — 파이프라인 보안** (대상: 사용자가 Nullus 로 만든 앱의 컨테이너 이미지)
> 작성: 2026-09-08 · **개정 2026-09-10** (중앙 스캐너 구조 → **Stack 영역 선택 설치**로 재조정, §2.3·§3) · 저장소 확인 기준 `ca04854`

---

## 0. 결정 요약

| # | 결정 | 한 줄 근거 |
|---|---|---|
| 1 | **스캐너를 Stack 구성의 선택 항목으로 등록한다** — 새 슬롯 `security.imageScanner` | Harbor · GitLab · Argo CD · Grafana 가 전부 스택 단위 설치다. 스캐너만 플랫폼 전역이면 예외가 된다 |
| 2 | **Trivy 를 server 모드로 설치한다.** 취약점 DB 는 스택의 서버에만 둔다 | client/server 모드에서 **DB 는 서버에만 필요하다** — 파이프라인마다 DB 를 감당하는 비용이 사라진다 |
| 3 | **차단 게이트는 그대로 파이프라인 단계다.** 중앙화하는 것은 *일하는 곳*이지 *막는 곳*이 아니다 | 지원 레지스트리 5종 중 스캔 기능이 있는 것은 Harbor 1종뿐. 게이트를 레지스트리에 두면 4종이 게이트 없이 초록불이 된다 |
| 4 | **스캔 소스를 스택 구성까지 보고 고른다** — 스캐너 있으면 스캐너, 없고 Harbor 면 Harbor, 둘 다 없으면 **스캔 단계를 켤 수 없다** | 선택 설치로 만들면 "아무것도 없는 스택"이 생긴다. 조용히 스캔 없이 만들면 사용자는 스캔된 줄 안다 |
| 5 | **CI 잡은 얇은 client 호출**로 만든다 (`trivy image --server ...`) | 방금 빌드한 이미지가 러너에 이미 있다. 서버가 다시 pull 하면 같은 이미지를 네트워크로 한 번 더 옮긴다 |
| 6 | **렌더러와 템플릿 `stages` 를 같은 PR 에서 바꾼다.** 파이프라인별 실효 stages 를 저장한다 | 마이그레이션 `000070` 이 정확히 이 실패를 되돌린 이력이다 |
| 7 | 기본값: CRITICAL 차단 · HIGH 경고 · `--ignore-unfixed` · **스캐너 도달 불가는 차단** | 수정본 없는 CVE 로 막으면 사용자가 할 수 있는 일이 없다. 반면 스캔이 못 돈 것을 통과로 넘기면 거짓말이 된다 |
| 8 | **취약점 DB 나이(`db_updated_at`)를 결과와 함께 저장하고 화면에 노출한다** | 에어갭에서 낡은 DB 의 "0건" 은 거짓말에 가깝다 |
| 9 | **정책은 스택 단위로 저장하고 CI 에 푸시한다.** 판정은 CI 가 스스로 한다 (§5.4) | 플랫폼 인증은 사용자 JWT 뿐이라 CI 가 플랫폼에 되물을 기계 인증 경로가 없다. 인바운드 게이트 API 는 토큰 발급·회전과 CI→플랫폼 네트워크 경로를 새로 요구한다 |

> **개정 이력**
> - **초안(09-08)** — CI 잡이 Trivy 를 통째로 실행. 이슈 본문이 이미 지적한 *"DB 갱신·캐시를 파이프라인마다 감당"* 을 그대로 떠안는다.
> - **1차 개정(09-10)** — client/server 모드로 DB 를 한 곳에 모음. 다만 스캐너를 **플랫폼 고정 컴포넌트**로 뒀다.
> - **2차 개정(09-10)** — 스캐너를 **Stack 구성의 선택 항목**으로 내림. §3 이 새로 생기고 §2.3·§4.1·§9 가 함께 바뀌었다.
> - **3차 개정(09-13, 현재)** — §5.4 의 게이트 API(`nullus-ci scan-gate`)를 버리고 **정책 푸시**로 바꿈. 결정 9 가 새로 생기고 §5.4·§6 이 함께 바뀌었다.

---

## 1. 저장소 확인 결과

| 확인 항목 | 결과 | 근거 |
|---|---|---|
| 저장소 내 Trivy 스캔 설정 | **없음** | `grep -ril trivy` — 히트는 전부 Harbor 컴포넌트 이미지·문서 |
| 스캐폴딩 파이프라인 단계 | `stages: build, deploy` 2단계 | [renderer.go:267](internal/cicd/adapter/scaffold/renderer.go#L267) |
| 단계 이름의 단일 출처 | `PipelineStageNames() → ["Build","Deploy"]` | [renderer.go:556](internal/cicd/adapter/scaffold/renderer.go#L556) |
| 템플릿 선언 stages | `["Build","Deploy"]` (000070 에서 축소됨) | [000070_align_pipeline_template_stages.up.sql](db/migrations/000070_align_pipeline_template_stages.up.sql) |
| **Harbor 내장 스캐너 활성 여부** | ✅ **활성** — 차트 기본값 `trivy.enabled: true`, Nullus values 는 이미지만 재정의하고 끄지 않음 | harbor 차트 `values.yaml:790-792`, [stack-values/harbor.yaml:67](airgap/helm/stack-values/harbor.yaml#L67) |
| **설치 계획 슬롯** | **11개** — artifacts 4 · pipeline 2 · monitoring 2 · logging 3. **`security` 계열 없음** | [planning.go:28-38](internal/stack/domain/planning.go#L28) |
| **UI 템플릿 편집기** | ⚠️ **`Security` / `Scanner` / `Trivy, SonarQube` 를 placeholder 로 안내한다** | [stack-template-page.tsx:1012](web/src/features/stack/pages/stack-template-page.tsx#L1012) |
| 위 템플릿 섹션이 설치로 이어지나 | ❌ **아니다.** `golden_path_templates.tools` 는 표시용 JSONB 고, 실제 설치는 `StackConfig` 의 11개 슬롯으로만 일어난다 | [000006_golden_path_templates.up.sql](db/migrations/000006_golden_path_templates.up.sql) |
| Trivy 서버·CLI 이미지, 취약점 DB 반입 | ❌ **없음** — 스캔을 넣으면 에어갭에서 그대로 실패한다 | `images.txt` 94줄에 `aquasecurity/trivy` 없음 |
| CI 플랫폼 | 3종 — GitLab CI · Jenkins · GitHub Actions | [renderer.go:158-165](internal/cicd/adapter/scaffold/renderer.go#L158) |
| 이미지 레지스트리 | **5종** — `scm_project` · `harbor` · `nexus` · `ghcr` · `external` | [port/image_registry.go:32-46](internal/cicd/port/image_registry.go#L32) |
| 설치 도구 목록의 단일 관문 | `domain.InstalledToolWorkloads()` — 여기 없으면 어느 화면에도 안 뜬다 | [tool_workload.go:64](internal/stack/domain/tool_workload.go#L64) |
| 취약점 결과 저장 위치 | **없음** | 전체 마이그레이션 확인 |

> **이미 있는 간극 하나** — 템플릿 편집기는 사용자에게 *"Security 섹션에 Scanner 로 Trivy 를 넣으세요"* 라고 안내하는데, **그렇게 적어도 설치되지 않는다.** 대응 슬롯이 없기 때문이다. `000070`("템플릿은 선언했지만 스캐폴딩은 만들지 않는다")과 같은 종류의 간극이고, 이 카드가 그것을 닫는다.

---

## 2. 결정 1 — 스캔 구조

### 2.1 지원 레지스트리별 스캔 기능 지원 현황

Nullus 는 레지스트리를 Harbor 로 고정하지 않는다. 그래서 먼저 레지스트리별로 스캔 기능이 실제로 있는지 확인했다.

| `RegistryKind` | 대표 OSS | 레지스트리 자체 스캔 | 결과 조회 API | 취약 이미지 pull 차단 |
|---|---|---|---|---|
| `harbor` | Harbor | ✅ **내장 Trivy** · scan-on-push · 예약 스캔 | ✅ `.../artifacts/{ref}/additions/vulnerabilities` | ✅ Deployment security + CVE allowlist |
| `scm_project` | GitLab Container Registry | ❌ GitLab 의 Container Scanning 은 *CI 잡*이지 레지스트리 기능이 아니다 | ❌ (Vulnerability Report 는 **Ultimate**) | ❌ |
| `nexus` | Nexus Repository **OSS** | ❌ 컨테이너 취약점 분석은 Sonatype IQ / Container Security(**상용**) 별도 제품 | ❌ | ❌ |
| `ghcr` | GitHub Container Registry | ❌ 네이티브 이미지 스캔 없음 | ❌ | ❌ |
| `external` | ECR 등 | ❓ 벤더별 상이 — 제품이 보증할 수 없음 | ❓ | ❓ |

**5종 중 1종만 스캔 기능이 있다.**

### 2.2 없는 쪽을 플러그인·별도 설치로 채울 수 있나

| 레지스트리 | 스캐너를 꽂는 확장점 | 결과 |
|---|---|---|
| Harbor | ✅ Pluggable Scanner 프레임워크 (Clair · Anchore 등록 가능) | **이미 Trivy 가 기본으로 붙어 있어 새로 얻는 것이 없다** |
| Nexus OSS | ❌ 확장점 없음 | 통합되는 스캐너가 전부 **상용**(Sonatype IQ, Prisma Cloud, Snyk) |
| GitLab CE 레지스트리 | ❌ 확장점 없음 | 스캐너를 꽂는 인터페이스 자체가 없다 |
| GHCR | ❌ 확장점 없음 | SaaS 라 붙일 자리가 없다 |

> **확장점이 있는 곳은 이미 스캐너가 있고, 없는 곳은 확장점이 없다.** §2.1 의 1/5 는 플러그인으로 바뀌지 않는다.

레지스트리에 **붙이는** 길이 막혔으므로, 레지스트리 **바깥에** 세운다 — 그리고 그 자리는 **스택**이다(§2.3).

### 2.3 채택 구조 — 스택에 선택 설치하는 스캐너 + 파이프라인 게이트

Trivy 를 **스택 구성에서 고르면** 그 스택에 server 모드로 설치되고, Nullus 가 스캔을 추상화한다.

```
   [스택 네임스페이스]                        [Nullus 플랫폼]
   ┌────────────────────────────┐
   │ trivy server (선택 설치)    │◀──── 매칭 요청 ──── Nullus API
   │ └ 취약점 DB                │                      (레지스트리 연계 스캔)
   └────────────────────────────┘                            │
            ▲                                                │
   매칭 요청 │                                                 │
            │                                                 ▼
      [CI 러너] trivy client ──── 리포트 ──────────▶  image_scan_results
            │                                                 │
            └──────── 게이트 판정 (exit code) ◀───────────────┘
                                                     (정책은 플랫폼이 소유)
```

**두 경로가 같은 서버를 쓴다.**

| | 경로 1 — 게이트 (**이 카드 필수**) | 경로 2 — 레지스트리 연계 (**후속**) |
|---|---|---|
| 실행 주체 | CI 잡의 `trivy client` | Nullus API |
| 대상 | 방금 빌드해 push 한 이미지 | 레지스트리에 이미 있는 이미지 |
| 목적 | **배포 전 차단** | 재스캔 · 기존 이미지 · Harbor 결과 통합 |
| 필요한 것 | CI 가 이미 가진 자격증명 | Nullus 의 레지스트리 **read** 자격증명 |

#### 왜 플랫폼 고정이 아니라 스택 선택인가

| | 플랫폼 고정 (1차 개정안) | **스택 선택 (채택)** |
|---|---|---|
| 다른 OSS 와의 일관성 | ❌ 스캐너만 예외 | ✅ Harbor · GitLab · Argo CD · Grafana 와 같은 취급 |
| 스택 격리 | ❌ 스캔 결과가 스택 경계를 넘는다 | ✅ 스택이 곧 조직·환경 경계다 |
| 안 쓰는 조직의 비용 | ❌ 항상 뜬다 | ✅ 안 고르면 안 깔린다 |
| 리소스 총량 | 1벌 | ⚠️ **스택 N개면 N벌** (§3.4) |
| 에어갭 DB 반입 | 1회 | ✅ **여전히 1회** — 번들에 한 번 넣고 각 스택 서버가 같은 내부 미러를 읽는다 |

리소스 총량이 유일한 비용이고, 두 가지로 완화된다: **선택이라 안 쓰면 안 깔리고**, Harbor 가 이미 있는 스택은 Harbor 스캐너를 쓰면 되므로 중복 설치를 피할 수 있다 — §4.1 의 전략이 그 선택을 자동으로 한다.

#### 초안 대비 유지되는 이득

| 항목 | 초안 (CI 가 Trivy 통째 실행) | 채택안 |
|---|---|---|
| **취약점 DB** | 파이프라인마다 내려받고 캐시 | **서버에만** — client 는 DB 를 받지 않는다 |
| 정책 변경 | 렌더된 스크립트에 박혀 있어 **재스캐폴딩 필요** | 플랫폼에서 바꾸면 즉시 적용 |
| 결과 수집 | CI 아티팩트를 3종 API 로 각각 회수 | 잡이 Nullus 에 **직접 POST** |

> 이슈 본문은 파이프라인 스캔의 단점으로 *"DB 갱신·캐시를 파이프라인마다 감당"* 을 들었다. **client/server 모드가 정확히 그 단점을 없앤다** — 공식 문서상 취약점 DB 는 **서버에만 필요**하고 클라이언트는 내려받지 않는다.

### 2.4 왜 CI 러너가 client 인가

경로 1 에서 서버가 이미지를 직접 pull 해 스캔할 수도 있다. 그렇게 하지 않는다.

1. **방금 빌드한 이미지가 러너에 이미 있다.** 서버가 pull 하면 같은 이미지를 네트워크로 한 번 더 옮긴다.
2. **서버가 모든 레지스트리의 read 자격증명을 가질 필요가 없다.** 경로 1 은 CI 가 push 에 쓰는 자격증명을 그대로 쓴다.
3. **게이트가 파이프라인 단계로 남는다.** 화면에 단계로 보이고, 실패가 파이프라인 실패로 이어진다.

client 가 하는 일은 이미지에서 패키지 목록을 뽑아 보내는 것까지다. **CVE 매칭과 DB 는 서버 몫이다.**

### 2.5 레지스트리 스캔의 자리 — 2차 방어, 게이트 아님

| | 파이프라인 게이트 | 레지스트리 스캔 (Harbor 한정) |
|---|---|---|
| 역할 | **차단 게이트** | 2차 방어 · 사후 탐지 |
| 시점 | 배포 전 | 푸시 후 · 주기적 재스캔 |
| 잡아내는 것 | 이 빌드가 들여오는 취약점 | **이미 배포된 이미지에 나중에 공개된 CVE** |

---

## 3. 결정 2 — Stack 영역 등록

### 3.1 새 슬롯 — `security.imageScanner`

계획 슬롯은 **도구 이름이 아니라 자리로 잇는다**. 기존 주석이 이유를 적어 뒀다 — *"이름은 gitlab / gitlab-ce 처럼 출처마다 흔들리지만 자리는 흔들리지 않는다"* ([planning.go:26](internal/stack/domain/planning.go#L26)). 스캐너도 같은 규칙을 따른다.

```go
// StackConfig 에 추가
type StackConfig struct {
    // ...기존 필드
    Security SecurityConfig `json:"security"`
}

// SecurityConfig 는 보안 도구 선택이다.
//
// 이미지 스캐너는 선택이다 — 고르지 않으면 설치되지 않고, 그 스택의
// 파이프라인은 이미지 스캔 단계를 켤 수 없다(§3.3). 레지스트리가 Harbor 면
// Harbor 내장 스캐너로 대신할 수 있다.
type SecurityConfig struct {
    ImageScanner ToolSelection `json:"image_scanner,omitempty"`
}
```

```go
// planning.go
const SlotImageScanner = "security.imageScanner"
```

### 3.2 건드릴 곳 — 7군데

새 슬롯 하나가 도는 데 필요한 전부다. **마지막 두 개가 빠지면 설치는 되는데 화면에 안 뜨거나, 크기 계획이 안 선다.**

| # | 곳 | 내용 |
|---|---|---|
| 1 | `domain.StackConfig` | `Security SecurityConfig` 필드 (위) |
| 2 | `domain.SlotImageScanner` + `PlanningSlots()` | 슬롯 상수 · 마법사 화면과 **같은 순서**로 등록 |
| 3 | `domain.plannedSelections()` | `{SlotImageScanner, cfg.Security.ImageScanner}` — [planning_plan.go:50](internal/stack/domain/planning_plan.go#L50) |
| 4 | `airgap/helm/stack-values/trivy.yaml` + `images.txt` | server 모드 values · 오프라인 DB 경로 · PVC · replica |
| 5 | 호환성 매트릭스 | 도구·버전 등록, 지원 아키텍처 |
| 6 | **`domain.InstalledToolWorkloads()`** | 파드 접두사 등록 — [tool_workload.go:64](internal/stack/domain/tool_workload.go#L64) 의 `candidates` 에 추가. **여기 없으면 어느 화면에도 안 뜬다** |
| 7 | 프론트 `PlanningSlot` union + 마법사 단계 | [install-planning-utils.ts:13](web/src/features/stack/utils/install-planning-utils.ts#L13). **Go 와 값이 갈리면 UI 설치와 API 설치가 다른 크기로 깔린다** (`TestPlanResourceVector_MatchesInstallWizard` 가 고정) |

> 6번과 7번은 이 저장소가 이미 값을 치른 실패다. 6번의 규칙은 *"무엇이 설치되는지 판단하는 규칙은 `domain.InstalledToolWorkloads` 한 곳에만 둔다"* ([monitoring_handler.go:868](internal/stack/adapter/handler/monitoring_handler.go#L868)), 7번은 *"프론트의 값이 기준이다 — 숫자가 갈리면 두 경로가 다른 크기로 깔린다"* ([planning.go](internal/stack/domain/planning.go)) 로 각각 주석에 남아 있다.

### 3.3 스캐너가 없는 스택은 어떻게 되나 — 이 결정의 핵심

선택으로 만들면 **아무 스캐너도 없는 스택**이 생긴다. 그 스택의 파이프라인에서 이미지 스캔 단계를 켜려 하면?

**조용히 스캔 없이 만들면 안 된다.** 사용자는 켰다고 생각하고, 화면은 단계를 보여주지 않으며, 아무도 그 사실을 모른다 — `000070` 이 되돌린 실패와 정확히 같은 모양이다.

```
스택에 security.imageScanner 가 설치돼 있나?
  ├─ 예 ──────────────────────▶ ScanSourceCentral (그 스택의 Trivy 서버)
  └─ 아니오
       └─ 이미지 레지스트리가 Harbor 인가?
            ├─ 예 ────────────▶ ScanSourceRegistry (Harbor 내장 스캐너)
            └─ 아니오 ─────────▶ ScanSourceNone
                                  → 스캔 단계를 켤 수 없다.
                                    파이프라인 생성 시점에 막고 이유를 말한다.
```

`ScanSourceNone` 일 때의 동작:

- 파이프라인 생성·수정에서 스캔 단계를 켜면 **거부한다** (`ErrNoScannerAvailable`).
- 설치 마법사·파이프라인 화면에서 **왜 못 켜는지와 무엇을 하면 되는지**를 함께 보여준다 — *"스택에 이미지 스캐너를 추가하거나 컨테이너 레지스트리를 Harbor 로 고르세요."*
- 스택에서 스캐너를 **나중에 제거**하면 이미 스캔 단계를 쓰는 파이프라인이 깨진다. 제거를 막지는 않되(사용자의 결정이다), **영향받는 파이프라인 목록을 먼저 보여준다.**

### 3.4 리소스 계획

슬롯마다 "얼마나 크게 깔지"를 묻는 옵션이 있다([planning.go:110-145](internal/stack/domain/planning.go#L110)). 스캐너의 부하는 **매칭 요청 수**에 비례하고 저장은 DB 크기라 거의 고정이므로, 다른 슬롯과 성격이 다르다.

| 옵션 | 기준값 | 성격 |
|---|---|---|
| `scansPerDay` | 파이프라인 실행 수를 따라간다 (`deploymentsPerDay` 기준값 40 과 같은 급) | CPU 지배적 |
| `concurrentScans` | 2 | CPU·메모리 |
| 저장 | — | **거의 고정** (DB 크기). 참고점: Harbor 의 trivy-adapter PVC 2Gi. server 모드는 DB 를 상주시키므로 그 이상 |

> 정확한 벡터는 실측이 필요하다 — **이 카드의 신규 태스크**다. 기준 벡터가 없는 도구는 계획에서 건너뛰어 차트 기본값에 맡겨진다([planning_plan.go:67](internal/stack/domain/planning_plan.go#L67) 주석). 실측 전에는 그 편이 낫다.

---

## 4. 결정 3 — 추상화 설계

레지스트리를 Harbor 로 고정하지 않는다는 제약을 코드 구조로 못박는다. 저장소의 세 선례를 따른다.

- `port.RegistryKind` — 레지스트리 종류가 이미 포트로 올라와 있다.
- `port.ErrImageDeletionUnsupported` — *"조용히 건너뛰지 않는 이유는, 사용자가 고른 동작이 아무 일도 일어나지 않으면 된 줄 알고 넘어가기 때문"* ([image_registry.go:8](internal/cicd/port/image_registry.go#L8)).
- `port.NormalizeStageStatus` — CI 별 어휘를 어댑터 경계에서 끝낸다. 스캐너별 심각도 어휘도 똑같이 처리한다.

### 4.1 스캔 소스 전략 — 스택 구성과 레지스트리를 함께 본다

§3.3 의 판단을 **한 곳**에 둔다. 레지스트리만 보던 초안과 달리, **스택에 무엇이 설치됐는지**를 함께 받는다.

```go
// ScanSource 는 이 이미지의 스캔 결과를 어디서 얻을지다.
type ScanSource string

const (
    // ScanSourceCentral 은 스택에 설치된 Trivy 서버로 스캔하는 것이다.
    ScanSourceCentral  ScanSource = "central"
    // ScanSourceRegistry 는 레지스트리가 이미 가진 결과를 읽는 것이다.
    // 스캔을 한 번 더 돌리지 않는다.
    ScanSourceRegistry ScanSource = "registry"
    // ScanSourceNone 은 이 스택에서 이미지를 스캔할 수단이 없다는 뜻이다.
    //
    // 스캔을 건너뛰라는 뜻이 아니다 — 스캔 단계를 켤 수 없다는 뜻이고,
    // 호출부는 이것을 침묵이 아니라 거부로 다뤄야 한다.
    ScanSourceNone     ScanSource = "none"
)

// ScanSourceFor 는 스택 구성과 레지스트리 종류로 스캔 소스를 고른다.
//
// 스택에 스캐너가 설치돼 있으면 그것을 쓴다. 없으면 레지스트리 자체 기능으로
// 떨어지는데, 2026-09 기준 그것이 있는 것은 Harbor 뿐이다 — GitLab Container
// Registry 의 Container Scanning 은 CI 잡이지 레지스트리 기능이 아니고,
// Nexus 는 OSS 에 스캐너가 없으며(Sonatype IQ 는 상용), GHCR 은 네이티브
// 스캔이 없다.
//
// Harbor 가 있는 스택에 스캐너를 또 깔 필요는 없다. 그 판단을 여기서 한다.
func ScanSourceFor(cfg StackConfig, kind RegistryKind) ScanSource {
    if cfg.Security.ImageScanner.Enabled {
        return ScanSourceCentral
    }
    if kind == RegistryKindHarbor {
        return ScanSourceRegistry
    }
    return ScanSourceNone
}

// ErrNoScannerAvailable 은 이 스택에 이미지를 스캔할 수단이 없다는 뜻이다.
//
// 스캔 단계를 조용히 빼지 않는 이유는 ErrImageDeletionUnsupported 와 같다 —
// 사용자가 켠 단계가 아무 일도 하지 않으면 스캔된 줄 알고 넘어간다.
var ErrNoScannerAvailable = errors.New(
    "이 스택에는 이미지 스캐너가 없습니다: 스택에 스캐너를 추가하거나 컨테이너 레지스트리를 Harbor 로 고르세요")
```

**고정 테스트** — 상수가 늘 때 판단을 강제한다.

```go
// 새 RegistryKind 를 추가하면 이 테스트가 먼저 깨진다.
// 스캔 소스를 정하지 않은 채 레지스트리를 늘리지 못하게 막는 장치다.
func TestScanSourceFor_CoversAllKinds(t *testing.T)
```

### 4.2 신규 포트 — `internal/cicd/port/image_scan.go`

```go
// ScanSeverity 는 심각도의 정규화된 어휘다.
//
// 스캐너마다 표기가 다르다 — Trivy 는 CRITICAL/HIGH/…, Harbor API 는
// Critical/High/… 을 쓴다. 이 변환을 어댑터 경계에서 끝낸다.
type ScanSeverity string

const (
    SeverityCritical ScanSeverity = "critical"
    SeverityHigh     ScanSeverity = "high"
    SeverityMedium   ScanSeverity = "medium"
    SeverityLow      ScanSeverity = "low"
    // SeverityUnknown 은 스캐너가 등급을 매기지 못한 것이다. 낮음으로 넘겨짚지 않는다.
    SeverityUnknown  ScanSeverity = "unknown"
)

func NormalizeSeverity(raw string) ScanSeverity  // NormalizeStageStatus 와 같은 꼴

// ScanReport 는 이미지 하나에 대한 스캔 결과다.
// 어느 소스에서 왔든 이 모양으로 정규화된다.
type ScanReport struct {
    Source      ScanSource
    Repository  string
    Digest      string          // 태그가 아니라 digest 로 고정한다 — 태그는 움직인다
    Tag         string
    Scanner     string          // "trivy" | "harbor-trivy"
    ScannerVer  string
    DBUpdatedAt time.Time       // 취약점 DB 가 언제 것인가 (§7.3)
    ScannedAt   time.Time
    Summary     map[ScanSeverity]int
    Findings    []ScanFinding   // 원본 전량이 아니라 게이트에 걸린 것만 (§8)
}

// ImageScanGate 는 스캔 결과를 차단/경고/통과로 판정한다.
//
// 판정 규칙은 도메인이 소유한다. CI 스크립트에 흩어지면 기준이 CI 마다 갈리고,
// 정책을 바꿀 때 모든 파이프라인을 다시 스캐폴딩해야 한다.
type ImageScanGate interface {
    Evaluate(ctx context.Context, r ScanReport, policy ScanPolicy) (GateResult, error)
}

// ErrScannerUnreachable 은 스캐너에 닿지 못했다는 뜻이다.
//
// 취약점이 없다는 뜻이 아니다. 이 둘을 같은 값으로 뭉치면 스캐너가 죽은 동안
// 모든 배포가 초록불로 통과한다 — 가장 나쁜 형태의 실패다 (§6.1).
var ErrScannerUnreachable = errors.New("취약점 스캐너에 닿을 수 없습니다")
```

### 4.3 어댑터 배치

| 소스 | 구현 | 위치 |
|---|---|---|
| `ScanSourceCentral` | 스택의 Trivy 서버 클라이언트 | **신규** `adapter/trivy/` |
| `ScanSourceRegistry` (Harbor) | `additions/vulnerabilities` 호출 | 기존 `adapter/harbor/` 에 추가 |
| `ScanSourceNone` | 어댑터 없음 | 호출부가 `ErrNoScannerAvailable` 로 거부 |

---

## 5. 결정 4 — 파이프라인 단계 삽입

### 5.1 어디에 끼우나 — push 후, deploy 앞

```
build (이미지 빌드 + push)  →  image-scan (게이트)  →  deploy (매니페스트 태그 갱신)
```

**push 후 스캔이 실제로 배포를 막는 근거**: `deploy` 단계는 배포하지 않는다. 매니페스트의 이미지 태그를 갱신해 커밋할 뿐이고 실제 배포는 Argo CD 가 그 커밋을 보고 한다(GitOps, [renderer.go:256](internal/cicd/adapter/scaffold/renderer.go#L256) 주석). **태그 갱신을 막으면 Argo CD 가 새 이미지를 집어가지 않는다.**

### 5.2 `000070` 을 반복하지 않는다

마이그레이션 `000070` 은 *"템플릿은 Build/Test/ImageBuild/Deploy 를 선언했지만 스캐폴딩은 Build/Deploy 만 만든다. 그래서 화면이 돌지도 않은 Test 를 성공으로 보여줬다"* 며 선언을 걷어낸 이력이다. 두 테스트가 이를 고정한다:

- `TestPipelineStageNames_MatchRenderedJenkinsfile` — 선언 단계 수와 렌더된 `stage('` 개수가 **정확히** 같아야 한다
- `TestPipelineStageNames_MatchRenderedGitLabCI`

**변경은 반드시 한 PR 에서 같이 간다:** 렌더러 3종 + `PipelineStageNames` + 템플릿 `stages` 마이그레이션 + 위 테스트 확장(GitHub Actions 판 추가).

### 5.3 옵션이 켜지고 꺼지면 stages 는 어떻게 되나

`pipeline_templates.stages` 는 **템플릿 단위** 컬럼인데 스캔 on/off 는 **파이프라인 단위** 선택이다.

| 안 | 방식 | 판정 |
|---|---|---|
| A | 템플릿에 항상 선언하고, 꺼진 파이프라인은 `CIStageSkipped` 로 표시 | ❌ GitLab 은 `rules` 로 걸러낸 잡을 파이프라인에서 **아예 빼버린다** — CI 가 skipped 를 알려주지 않으므로 Nullus 가 상태를 지어내야 한다 |
| B | 꺼진 파이프라인은 잡을 렌더링하지 않고, **파이프라인별 실효 stages 를 저장**한다 | ✅ **채택** |

```sql
ALTER TABLE pipelines ADD COLUMN IF NOT EXISTS stages JSONB NOT NULL DEFAULT '[]'::jsonb;
```

> **#76 단독 요구가 아니다.** #64(SAST) · #79(SCA) · #80(Secret Detection) 이 전부 선택적 단계를 하나씩 더한다 (§10).

### 5.4 잡 모양 — 얇은 client 호출, 정책은 푸시받은 변수로

DB 를 내려받지 않는다. 정책도 파이프라인 파일에 박지 않는다 — **CI 가 푸시받은 변수로 스스로 판정한다.**

```yaml
image-scan:
  stage: image-scan
  image: $NULLUS_TRIVY_IMAGE           # CLI 만. DB 없음
  needs: [build]
  script:
    # 1) 리포트. 심각도를 거르지 않는다 — 거르면 HIGH 가 리포트에 없어 경고가 영원히 안 나온다.
    #    스캔 자체를 못 하면 정책(NULLUS_SCAN_ON_UNREACHABLE)에 따라 멈추거나 통과시킨다.
    - 'if ! trivy image --server "$NULLUS_TRIVY_SERVER" --scanners vuln <unfixed> --format json
         --output trivy-report.json "$IMAGE_REPOSITORY:$IMAGE_TAG"; then
         if [ "${NULLUS_SCAN_ON_UNREACHABLE:-block}" = "allow" ]; then exit 0; fi; exit 1; fi'
    # 2) 게이트. 변수가 없으면(한 번도 푸시하지 않은 파이프라인) 기본 정책으로 판정한다.
    - 'trivy image --server "$NULLUS_TRIVY_SERVER" --scanners vuln
         --severity "${NULLUS_SCAN_SEVERITY:-CRITICAL}" <unfixed> --exit-code 1 --quiet "$IMAGE_REPOSITORY:$IMAGE_TAG"'
  artifacts:
    when: always
    paths: [trivy-report.json]
```

Jenkins · GitHub Actions 판도 **같은 두 명령**이다. `<unfixed>` 는 `NULLUS_SCAN_IGNORE_UNFIXED`(기본 true)를 읽는다.

#### 정책을 CI 에 싣는 자리

| CI | 자리 | 파일에서 읽는 방식 |
|---|---|---|
| GitLab CI | 앱 프로젝트 CI/CD 변수 | `.gitlab-ci.yml` 의 variables 보다 **앞선다** — 이미 스캐폴딩한 파이프라인도 재커밋 없이 바뀐다 |
| GitHub Actions | 리포 Actions **변수**(vars) | `env: NULLUS_SCAN_SEVERITY: ${{ vars.NULLUS_SCAN_SEVERITY }}` — 시크릿이 아니다(시크릿이면 `vars` 로 읽히지 않고 로그에서 가려진다) |
| Jenkins | 스택 네임스페이스의 ConfigMap `nullus-scan-policy` | 스캐너 컨테이너 `envFrom: configMapRef (optional)` — 스택 하나에 하나라 한 번 적용하면 그 스택의 Jenkins 파이프라인 전체가 따른다 |

푸시 시점은 둘이다. **정책을 저장할 때**(`PUT /stacks/:stackId/image-scan-policy`, 그 스택의 스캔 파이프라인 전체)와 **스캔 파이프라인을 새로 만들 때**(그 파이프라인 하나). 푸시 실패로 저장을 되돌리지 않고 파이프라인별 결과를 돌려준다.

#### 게이트 API 를 버린 이유

초안은 판정을 플랫폼이 하는 `nullus-ci scan-gate` 였다. 정책이 플랫폼에만 있어 실시간으로 바뀐다는 장점이 있지만,

- 플랫폼 인증은 Keycloak 사용자 JWT 뿐이다. CI 잡이 부를 **기계 인증**(파이프라인별 토큰 발급·회전·폐기)을 새로 만들어야 한다.
- CI 러너 → 플랫폼 API 의 **인바운드 네트워크 경로**가 필요하다. 에어갭·사설망 설치에서 가장 먼저 막히는 곳이다.
- 플랫폼이 죽으면 모든 스택의 배포가 멈춘다 — 스택 단위로 장애를 가둔 §6.1 의 이득이 사라진다.

정책 푸시는 반영이 **다음 실행부터**라는 비용만 진다. 결과 수집은 실행 기록 동기화(§8)가 이미 한다.

---

## 6. 결정 5 — 차단 기준

| 항목 | 기본값 | 근거 |
|---|---|---|
| 차단 심각도 | **CRITICAL** | HIGH 까지 막으면 흔한 베이스 이미지로 첫 배포가 안 된다 |
| 경고 심각도 | **HIGH** | 게이트는 통과하되 결과에 남기고 화면에 노출 |
| 수정본 없는 CVE | **제외 (`--ignore-unfixed`)** | 업스트림에 패치가 없는 CVE 로 막으면 사용자가 할 수 있는 일이 없다 |
| 예외 처리 | 리포지토리 루트의 **`.trivyignore`** | Trivy 기본 기능이라 새로 만들 게 없다. 리뷰 대상이 되도록 코드와 같은 곳에 둔다 |
| 조직 공통 allowlist | **이번 범위 밖** | 소유·승인 절차가 필요하다 (§11) |
| **스캐너 도달 불가** | **차단** (`OnScannerUnreachable: block`) | 아래 |

### 6.1 스캐너 장애의 영향 범위

스캐너가 죽으면 **그 스택의** 파이프라인 스캔 단계가 실패한다. 플랫폼 고정안과 달리 **영향이 스택 하나로 막힌다** — 스택 선택 설치의 부수 효과다.

- 스캐너에 못 닿은 것은 `ErrScannerUnreachable` 이고 결과는 `gate_result = 'error'` 다. **`pass` 로 뭉치지 않는다** — 스캐너가 죽은 동안 모든 배포가 초록불로 통과하는 것이 가장 나쁜 실패다.
- 기본 동작은 **차단**이되, 정책의 `OnScannerUnreachable` 로 운영자가 명시적으로 완화할 수 있다. 긴급 배포용 우회로가 없으면 사람들은 파이프라인 자체를 우회한다.
- 서버는 replica 2 이상을 권장한다. DB 는 읽기 전용이라 수평 확장이 쉽다.

---

## 7. 결정 6 — 에어갭 취약점 DB 반입

### 7.1 반입 대상

**스택이 여러 개여도 반입은 한 번이다.** 번들에 한 번 넣고 내부 레지스트리에 미러하면, 각 스택의 서버가 같은 곳을 읽는다.

| 산출물 | 형태 | 반입 방법 | 넣을 곳 |
|---|---|---|---|
| `aquasecurity/trivy:<ver>` | 일반 컨테이너 이미지 | `docker pull/save` (기존 경로) | `airgap/images/images.txt` |
| `ghcr.io/aquasecurity/trivy-db:2` | **OCI 아티팩트** | ⚠️ `docker pull` 로 안 받아진다 — `oras` 또는 `crane` 필요 | **신규** `airgap/images/oci-artifacts.txt` |
| `ghcr.io/aquasecurity/trivy-java-db:1` | OCI 아티팩트 | 〃 | 〃 |

> **`images.txt` 에 그냥 추가하면 안 된다.** trivy-db 는 컨테이너 이미지가 아니라 OCI 아티팩트라 이미지 미디어 타입이 아니다. 기존 pull 스크립트가 조용히 실패하거나 이상한 오류를 낸다.

### 7.2 서버가 DB 를 읽는 경로

서버는 내부 레지스트리에 미러된 DB 를 `TRIVY_DB_REPOSITORY` 로 가리킨다. `--skip-db-update` 는 쓰지 않는다 — 내부 미러에서 정상 갱신 경로를 타게 두면, DB 를 갱신했을 때 서버가 저절로 새 DB 를 쓴다.

**누가 넣나** — API 로 설치하는 에어갭 스택(`29-install-stacks-via-api.sh`)은 `stack-values/trivy.yaml` 을 읽지 않는다. API 가 에어갭 모드(`NULLUS_HELM_OCI_REGISTRY`, 예: `kind-registry:5000/charts`)에서 **같은 레지스트리의 루트**로 `trivy.dbRepository=kind-registry:5000/aquasecurity/trivy-db` 와 `TRIVY_INSECURE=true`(내부 레지스트리가 plain HTTP)를 넣는다(`trivyAirgapDBValues`). 처음에는 이 값이 values 파일에만 있어 API 설치의 서버가 ghcr.io 를 찾았고, 파일 값도 호스트 주소(`localhost:5001`)라 파드 안에서는 닿지 않았다. helm 으로 직접 설치하는 경로는 `stack-values/trivy.yaml` 이 같은 값을 가진다.

**제약**: DB 미러는 클러스터 안에서 접근 가능한 레지스트리 경로여야 한다. 스택마다 다른 곳을 보게 만들지 않는다 — 반입 절차가 갈라지면 그중 하나는 반드시 낡는다. Trivy Operator 를 나중에 넣더라도(§11) 같은 미러를 그대로 쓴다.

### 7.3 갱신 주기 — 그리고 지켜지지 않을 때

| 사실 | 값 |
|---|---|
| 업스트림 trivy-db 빌드 주기 | 6시간 |
| Trivy 클라이언트 기본 갱신 확인 주기 | 24시간 |
| **에어갭에서 실제 신선도** | **번들 재반입 주기에 종속** |

1. 에어갭 번들 릴리스마다 DB 를 최신으로 재반입한다.
2. **모든 스캔 결과에 `db_updated_at` 을 함께 저장한다**(§8).
3. DB 나이가 임계(제안: **30일**)를 넘으면 결과를 초록불로 표시하지 않고 **"DB 오래됨"** 을 함께 표시한다.

> 3번이 이 절의 핵심이다. 6개월 된 DB 로 나온 "취약점 0건" 은 정보가 아니라 오해다. 낡은 DB 를 강제로 막을 수는 없지만(에어갭에서는 정당한 상황일 수 있다), **언제 것인지 숨기지는 않는다.** 스택마다 서버가 하나씩이므로 이 값은 스택 단위로 신뢰할 수 있다 — 러너마다 캐시 나이가 다를 여지가 없다.

### 7.4 Harbor 내장 스캐너의 DB

경로가 다르다. Harbor 는 자체 오프라인 취약점 데이터 임포트 절차를 따로 제공한다. **`ScanSourceRegistry` 를 쓰는 스택(스캐너를 안 깔고 Harbor 로 가는 구성)은 이 절차가 필수**가 된다 — §11 로 미루지 않고 §4.3 구현과 함께 다룬다.

---

## 8. 결정 7 — 결과 저장 범위 (#65 입력)

원본 리포트는 DB 에 넣지 않는다(이미지 하나에 수 MB). **요약 + 게이트 판정 + 원본 위치**만 둔다.

```sql
CREATE TABLE image_scan_results (
    id            VARCHAR(100) PRIMARY KEY,
    pipeline_id   VARCHAR(100) NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    deployment_id VARCHAR(100) REFERENCES pipeline_deployments(id) ON DELETE SET NULL,

    image_repository VARCHAR(500) NOT NULL,
    image_tag        VARCHAR(255) NOT NULL,
    image_digest     VARCHAR(255) NOT NULL,   -- 태그는 움직인다. 결과는 digest 에 붙인다

    scan_source      VARCHAR(20)  NOT NULL,   -- central | registry  (§4.1)
    scanner          VARCHAR(50)  NOT NULL,   -- "trivy" | "harbor-trivy"
    scanner_version  VARCHAR(50)  NOT NULL,
    db_updated_at    TIMESTAMPTZ,             -- §7.3 — NULL 이면 "알 수 없음", 초록불 금지

    critical_count   INTEGER NOT NULL DEFAULT 0,
    high_count       INTEGER NOT NULL DEFAULT 0,
    medium_count     INTEGER NOT NULL DEFAULT 0,
    low_count        INTEGER NOT NULL DEFAULT 0,
    unknown_count    INTEGER NOT NULL DEFAULT 0,

    gate_result      VARCHAR(20)  NOT NULL,   -- pass | warn | block | error
    report_uri       TEXT,                    -- CI 아티팩트 링크
    scanned_at       TIMESTAMPTZ  NOT NULL
);
```

`gate_result` 에 `error` 를 둔다. **스캔이 못 돈 것과 취약점이 없는 것은 다르다**(§6.1).

**어떻게 들어오나** — 실행 기록 동기화가 CI 산출물 `trivy-report.json` 을 읽어 채운다. §5.4 에서 게이트 API(`nullus-ci scan-gate`)를 버렸으므로 CI 가 플랫폼으로 보내는 인바운드 경로는 없다. 동기화는 화면 조회 때와 주기적으로 돈다(§12.3). `report_uri` 에는 브라우저가 여는 CI 리포트 주소를 채운다(§12.2).

**단계 상태와의 관계** — 스캔 단계의 성공/실패는 기존 경로(`pipeline_deployments.steps`, `kind: "ci_stage"`)로 들어온다. 이 테이블은 **거기에 없는 것**(심각도별 건수 · DB 나이 · 게이트 판정 · 스캔 소스)만 담는다.

**#65 대시보드가 읽는 방법** — `image_scan_results` 는 **cicd 모듈이 소유**한다. #65 는 이 테이블을 직접 조회하지 않고 cicd 의 공개 유스케이스를 통해 읽는다 (CLAUDE.md — *"다른 모듈의 테이블을 직접 조회하지 않는다"*).

| 지표 | 출처 |
|---|---|
| 앱별 최신 이미지 취약점 건수 (심각도별) | `*_count` |
| 게이트 차단 건수 / 통과율 | `gate_result` |
| 취약점 DB 신선도 | `db_updated_at` |
| 스캔 실패(미실행) 파이프라인 수 | `gate_result = 'error'` |

---

## 9. 구현 순서

| # | 작업 | 결과물 | 이것만으로 배포 가능? |
|---|---|---|---|
| 1 | **Stack 슬롯 등록** | §3.2 의 7군데 + 리소스 실측 | ✅ 고르지 않으면 아무 일도 안 일어남 |
| 2 | **에어갭 DB 반입** | `oci-artifacts.txt` + oras/crane 스크립트 | ✅ |
| 3 | 포트 정의 | `port/image_scan.go` · `ScanSourceFor` + 커버리지 테스트 | ✅ 호출부 없음 |
| 4 | 저장소 | `image_scan_results` 마이그레이션 + repository | ✅ 쓰는 곳 없음 |
| 5 | 게이트 API | 판정 엔드포인트 + `ImageScanGate` + 정책 | ✅ 호출부 없음 |
| 6 | **렌더러 + 템플릿 stages** | 렌더러 3종 · `PipelineStageNames` · `pipelines.stages` · 마이그레이션 · 고정 테스트 3종 | ⚠️ **한 PR 이어야 한다** (§5.2) |
| 7 | Harbor `ScanSourceRegistry` | Harbor 결과 읽기 + 오프라인 DB 임포트 절차(§7.4) | 선택 |
| 8 | 경로 2 — 레지스트리 연계 스캔 | 재스캔 · 기존 이미지 | 후속 |

1·2 가 6 보다 앞선다. **스캐너를 고를 수 없는 상태로 스캔 단계를 렌더링하면 `ScanSourceNone` 밖에 나올 수 없다.**

TDD 순서(CLAUDE.md): 각 단계마다 실패하는 테스트를 먼저 쓴다. 6번은 `TestPipelineStageNames_*`, 1번은 `TestPlanResourceVector_MatchesInstallWizard` 가 **먼저 빨갛게** 되는 것을 확인하고 들어간다.

---

## 10. #64 에 넘기는 요구사항

이슈는 "#64 의 옵션화 스키마 준수" 를 요구한다. #64 도 아직 설계 전이므로, **#76 이 그 스키마에 요구하는 것**을 명시해 둔다.

1. 보안 단계 on/off 는 **파이프라인 단위** 선택이다 (템플릿 단위가 아니다).
2. 따라서 **파이프라인별 실효 stages 저장**이 필요하다 (§5.3 B안). #64 · #79 · #80 도 같은 것을 필요로 한다.
3. **단계별 정책은 렌더된 스크립트가 아니라 플랫폼에 둔다** (§5.4).
4. **보안 단계는 스택에 그 도구가 설치돼 있어야 켤 수 있다** (§3.3). SAST(SonarQube) 도 같은 구조가 필요하다 — `security.sast` 슬롯 + `ErrNoScannerAvailable` 과 같은 거부 경로. **`security` 계열 슬롯 그룹을 이 카드가 처음 만든다.**

> #64 가 다른 결론을 내면 §5.3 과 §6 을 그에 맞춰 고친다. 그 전까지 §9 의 1~5 단계는 영향 없이 진행할 수 있다.

---

## 11. 이 카드에서 닫지 않는 것

| 항목 | 이유 | 재검토 시점 |
|---|---|---|
| **경로 2 — 레지스트리 연계 스캔** | 게이트가 먼저 선다. 서버가 서면 증분 작업이다 | §9 8번 |
| Trivy Operator (실행 중 워크로드 스캔) | 게이트가 아니라 탐지. **§7.2 의 DB 미러를 그대로 쓴다**. 스택 네임스페이스의 설치 이미지는 §12.4 가 주기 스캔으로 덮는다 — 사용자 앱·다른 네임스페이스 워크로드는 여전히 범위 밖 | 경로 2 이후 |
| Dependency-Track (SBOM 지속 모니터링) | NVD 피드라는 **별개 반입 경로**가 하나 더 늘어난다 | **#77 소관** |
| 조직 공통 CVE allowlist | 소유·승인 절차 설계 필요 | 운영 규칙 정리 시 |
| Harbor Deployment security 활성화 방침 | 2차 방어. 게이트가 먼저 선다 | §9 7번 이후 |
| 템플릿 `tools` JSONB 와 슬롯의 일반적 정합 | §1 의 간극은 스캐너 슬롯을 만들며 **스캐너에 한해** 닫힌다. 다른 자유 텍스트 도구는 그대로다 | 템플릿 개편 시 |
| Nullus **제품 이미지** 스캔 | **#77 소관** (B축) | — |

---

## 12. 후속 — 결과 노출 · 리포트 링크 · 주기 동기화 · 설치 OSS 이미지 스캔 (2026-09-14)

### 12.1 파이프라인 화면

실행 이력의 실행마다 게이트 판정 배지와 심각도 건수를, 선택한 실행에는 이미지·digest·스캐너 버전·취약점 DB 날짜·리포트 링크를 보인다. 결과는 `deployment_id` 로 실행에 붙는다.

- `counts` 가 없으면 **모름** 이다. 0 으로 그리지 않는다.
- `error` 판정은 초록불이 아니다 — 스캔이 못 돈 것이다(§6.1).
- DB 가 30일을 넘었거나 날짜를 모르면 통과여도 **"DB 오래됨"** 을 함께 보인다(§7.3).

### 12.2 리포트 링크 — `report_uri`

CI API 클라이언트는 클러스터 내부 주소로 붙는다. 그 주소는 브라우저가 열 수 없으므로 링크는 **스택 접속 도메인(https)** 으로 만든다(스택 모듈 `domain.ToolAccessURL` 과 같은 규칙). 산출물 조회기가 선택적으로 `port.CIArtifactLinker` 를 구현한다.

| CI | 링크 |
|---|---|
| GitLab | `https://gitlab.<도메인>/<그룹>/<앱>/-/jobs/<잡 id>/artifacts/file/trivy-report.json` |
| GitHub Actions | `<웹 주소>/<owner>/<repo>/actions/runs/<실행 id>` — 산출물 파일 주소가 없어 실행 페이지로 간다 |
| Jenkins | `https://jenkins.<도메인>/job/<앱>/job/<브랜치>/<빌드>/artifact/trivy-report.json` |

- 리포트를 읽은 실행에만 건다. 리포트가 없으면 열어도 없는 파일이다.
- 접속 도메인을 모르면 링크를 만들지 않는다 — 죽은 링크보다 없는 편이 낫다.
- 이 기능 전에 기록한 실행은 리포트를 다시 받지 않고 링크만 채운다.

### 12.3 주기 동기화

화면을 열 때만 들이면 아무도 보지 않는 파이프라인의 스캔 결과가 쌓이지 않는다. 조회 시 동기화에 더해 **`CICD_RUN_SYNC_INTERVAL`(기본 10m)** 마다 스택에 묶인 모든 파이프라인을 들인다.

- 번들(CI 클라이언트)은 스택마다 한 번만 만든다 — 조립마다 SCM 인증 확인이 따른다.
- 설치 중이거나 사라진 스택은 경고 없이 건너뛴다.
- 레플리카가 여럿이면 각자 돈다. 기록은 upsert 라 결과는 같고 CI 조회만 는다.

### 12.4 설치 OSS 이미지 스캔 — 보고용

파이프라인 게이트는 사용자 앱 이미지만 본다. 스택이 설치한 GitLab · Harbor · Argo CD 등의 이미지는 아무도 보지 않았다.

| 항목 | 결정 | 이유 |
|---|---|---|
| 판정 | **보고만. 설치를 막지 않는다** | 업스트림 이미지의 CVE 는 사용자가 고칠 수 없는 경우가 많다. 막으면 스택을 세울 방법이 사라진다 |
| 스캐너를 고르지 않은 스택 | `not_scanned` · `scanner_not_installed` | "0건" 으로 보이면 안 된다 |
| 에어갭 | **스캔하지 않는다** (`not_scanned` · `airgap`) | 이미지를 받을 외부 레지스트리에 닿지 못하고, DB 는 사람이 넣은 만큼만 새것이다. 판정 근거는 `NULLUS_HELM_OCI_REGISTRY`(에어갭 차트 저장소) |
| 시점 | 설치 완료 직후 1회 + **`STACK_IMAGE_RESCAN_INTERVAL`(기본 24h)** 주기 재스캔 | 설치 뒤에 공개된 CVE 는 다시 스캔해야 보인다 |

**대상** — 스택 네임스페이스에서 `Running` 인 파드의 `imageID`(저장소@digest). digest 단위로 한 번만 스캔하고, 파드가 도는 노드의 아키텍처로 `--platform` 을 준다(Trivy 기본값은 amd64 라 arm64 노드의 이미지와 다른 것을 본다). 끝난 Job 파드, digest 가 없는 로컬 이미지, 스캔 Job 자신은 뺀다. 스택 네임스페이스 밖(cert-manager 등 클러스터 공용 구성요소)은 대상이 아니다.

**실행** — 스택 네임스페이스에 Job `nullus-image-scan` 을 세운다. `aquasec/trivy` client 가 스택 Trivy 서버(§2.3)에 붙는다.

- 결과를 JSON 리포트가 아니라 **취약점 하나당 탭으로 나눈 한 줄**(심각도 · ID · 패키지 · 설치/수정 버전 · Trivy Class · 대상 · 링크)로 만들고, 이미지마다 **gzip+base64 한 줄**로 싸서 로그에 찍는다. GitLab 이미지 하나에 수천 건이라 그대로 찍으면 kubelet 컨테이너 로그 상한(기본 10Mi)을 넘어 앞부분이 잘린다. kind 실측에서 alpine 48건이 7.2KB → 784B 로 줄었고, 건수는 같은 이미지의 JSON 리포트와 일치했다(§12.5).
- 서버 모드 client 는 템플릿 출력에 DB 시각을 싣지 않는다. DB 날짜는 서버의 `metadata.json` 에서 읽는다.
- 이미지 참조는 셸 스크립트에 들어가므로 `저장소@sha256:` 모양이 아니면 넣지 않는다.
- 이미지마다 **최대 3회** 시도한다(사이 10초, `NULLUS_SCAN_ATTEMPTS` · `NULLUS_SCAN_RETRY_DELAY`). 설치 직후 스캔에서 레이어를 받다 끊기는 일시적 실패(argocd 의 `failed to extract the archive: unexpected EOF`, gitlab-runner 의 파일 열기 실패)가 났고, 같은 이미지를 다시 스캔하면 통과했다. 한 번만 시도하면 다음 주기 재스캔(기본 24h)까지 `failed` 로 남는다. 끝내 실패하면 마지막 시도의 오류를 `N회 시도 후 실패:` 와 함께 남긴다.

**저장** — `stack_image_scans`(000081, **stack 모듈 소유**). 파이프라인 결과(`image_scan_results`)와 나눈다 — 그쪽은 cicd 가 소유한 CI 실행 기록이다.

- 다시 스캔하면 스택의 행을 **통째로 바꾼다**. 더는 돌지 않는 이미지의 결과가 남지 않는다.
- 스캔 자체가 실패하면 **이전 결과를 지우지 않는다**. 일시 장애로 보고서가 비면 알던 취약점이 사라진 것처럼 보인다.
- 이미지 하나를 못 스캔하면 `failed` 와 오류를 남기고 건수는 NULL 이다.
- 한 스택의 스캔은 한 번에 하나다. 설치 직후 스캔과 주기 재스캔이 겹치면 같은 이름의 Job 을 서로 지운다.

**노출** — `GET /api/v1/stacks/:stackId/image-scans`(상태 · 사유 · 요약 · 이미지별 결과), 스택 상세 화면.

**공용 어휘** — Trivy 리포트 요약 · 심각도 건수 · DB 신선도는 `internal/shared/domain` 으로 옮겼다. 모듈끼리 import 할 수 없는데 각자 세면 같은 이미지가 화면마다 다른 건수로 보인다.


### 12.5 취약점 목록 — 베이스 이미지와 앱 의존성

건수와 리포트 링크만으로는 무엇을 고쳐야 하는지 화면에서 알 수 없다. 파이프라인 실행과 스택 설치 이미지 모두 **취약점 목록**(CVE · 패키지 · 설치/수정 버전 · 심각도 · 링크)을 보인다.

**공용 모델** — `internal/shared/domain` 의 `ImageVulnerability`. Trivy 리포트의 결과 묶음은 `Class` 로 갈린다 — `os-pkgs` 는 **베이스 이미지의 OS 패키지**(`os`), `lang-pkgs` 는 **앱이 넣은 의존성**(`library`), 나머지는 `other`. 이 구분으로 "베이스 이미지 취약점" 을 따로 보인다.

**조회 조건** — `severity`(쉼표 구분) · `class` · `fixable=true` · `q`(ID·패키지 검색) · `limit`(기본 50, 최대 200) · `offset`. 심각한 순으로 정렬한다. 응답의 `targets`(대상별 전체 건수)는 필터와 무관하다.

**못 보일 때는 이유를 담는다** — 200 과 `status: unavailable`, `reason`. 빈 목록은 취약점 0건으로 읽히기 때문이다.

| reason | 뜻 |
|---|---|
| `report_expired` | CI 리포트가 보관 기간이 지나 사라졌다. 건수는 기록에 남아 있다 |
| `report_missing` | 이 스캔에 리포트 위치가 없다(리포트를 쓰기 전에 스캔이 실패했거나 위치 기록 전의 기록) |
| `ci_unreachable` | CI 서버에 닿지 못했다 |
| `not_recorded` | (스택) 목록 기능 전에 스캔했다. 다음 스캔 뒤에 보인다 |
| `scanner_not_installed` · `airgap` | (스택) 설치 이미지를 스캔하지 않는 스택이다(§12.4) |

**파이프라인 — 저장하지 않는다** — `GET /api/v1/cicd/pipelines/:id/image-scans/:scanId/vulnerabilities`. §8 의 결정(원본 리포트는 DB 에 넣지 않는다)을 지킨다. 동기화가 리포트를 읽을 때 **리포트 위치**(`image_scan_results.report_ref`, 000082 — GitLab 잡 id · GitHub 실행 id · Jenkins 빌드 번호)를 남기고, 볼 때 그 위치로 CI 리포트를 다시 읽어 목록을 만든다. 목록이 리포트와 어긋날 일이 없는 대신, CI 가 보관 기간(GitLab 인스턴스 기본값 등)이 지나 산출물을 지우면 목록은 볼 수 없고 건수만 남는다. 다른 파이프라인의 스캔 id 로는 읽을 수 없다. 위치 기록 전의 결과는 다음 동기화 때 리포트를 다시 받지 않고 위치만 채운다.

**스택 설치 이미지 — 스캔할 때 저장한다** — `GET /api/v1/stacks/:stackId/image-scans/vulnerabilities?digest=`. 설치 이미지는 다시 읽을 CI 리포트가 없다. 스캔 Job 의 결과 줄(§12.4)을 풀어 `stack_image_vulnerabilities`(000083, stack 모듈 소유)에 COPY 로 넣는다. 스캔 결과 행과 함께 교체되고(ON DELETE CASCADE), 이미지별로만 읽는다. `stack_image_scans.vulnerabilities_recorded` 로 "목록 기능 전에 스캔한 이미지" 와 "취약점 0건인 이미지" 를 가른다 — 둘 다 목록 행이 없다.

---

## 참고

- Trivy — [Client/Server 모드](https://trivy.dev/latest/docs/references/modes/client-server/) · [Air-Gapped Environment](https://trivy.dev/latest/docs/advanced/air-gap/) · [Self-Hosting Trivy's Databases](https://trivy.dev/latest/docs/advanced/self-hosting/)
- Harbor — [Vulnerability Scanning](https://goharbor.io/docs/2.13.0/administration/vulnerability-scanning/) · [Pluggable Scanners](https://goharbor.io/docs/2.13.0/administration/vulnerability-scanning/pluggable-scanners/)
- GitLab — [Container Scanning](https://docs.gitlab.com/user/application_security/container_scanning/)
- Sonatype — [Docker Image Analysis](https://help.sonatype.com/en/docker-image-analysis.html) · [Container Security](https://help.sonatype.com/en/sonatype-container-security.html)
- Aqua — [Trivy Operator](https://github.com/aquasecurity/trivy-operator) · OWASP — [Dependency-Track](https://dependencytrack.org/)
- 저장소 — [`internal/stack/domain/planning.go`](internal/stack/domain/planning.go), [`internal/stack/domain/planning_plan.go`](internal/stack/domain/planning_plan.go), [`internal/stack/domain/tool_workload.go`](internal/stack/domain/tool_workload.go), [`internal/cicd/port/image_registry.go`](internal/cicd/port/image_registry.go), [`internal/cicd/adapter/scaffold/renderer.go`](internal/cicd/adapter/scaffold/renderer.go), [`web/src/features/stack/utils/install-planning-utils.ts`](web/src/features/stack/utils/install-planning-utils.ts)
