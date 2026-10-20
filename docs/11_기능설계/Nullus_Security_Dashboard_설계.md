# Nullus Security Dashboard 설계

> 대상 이슈: [cloud-nullus/nullus-plan#65](https://github.com/cloud-nullus/nullus-plan/issues/65) — [Level 3] EPIC: Security Dashboard 설계 (원본 #45)
> 선행 카드: #64 SAST · #76 이미지 스캔 · #77 공급망 증적 · #78 설치 OSS CVE · #79 SCA · #80 Secret Detection
> 작성: 2026-10-20 · 저장소 확인 기준 `58a15e0`(#358 머지 뒤 main)

---

## 0. 결정 요약

| # | 결정 | 한 줄 근거 |
|---|---|---|
| 1 | **메뉴: 관측성 › 보안 › 보안 대시보드.** 경로 `/observability/security/dashboard`, `/observability/security` 는 대시보드로 보낸다. 사이드바를 **3단**(그룹 › 하위 그룹 › 항목)으로 넓힌다 | 보안 하위 그룹은 대시보드 하나로 시작하지만 스캔 정책 화면(API 는 있고 화면이 없다, §10) 같은 후속이 들어갈 자리다 |
| 2 | **두 축을 섞지 않는다** — 화면을 **앱**(파이프라인 보안)과 **설치 OSS**(스택 이미지) 탭으로 가르고 숫자를 합치지 않는다 | 앱 쪽은 배포를 막는 **게이트**고 OSS 쪽은 막지 않는 **보고**다. 고칠 사람도 다르다. GitLab 이미지 하나가 CRITICAL 61 건이라(#78 실측) 합치면 앱 숫자가 묻힌다 |
| 3 | **앱의 현재 상태 = `main` 의 단계별 최신 기록.** 실행을 누적해 세지 않는다 | 실행 기록 동기화는 `main` 만 읽는다. 같은 취약점이 실행마다 다시 세지면 숫자가 실행 횟수에 비례한다 |
| 4 | **KPI 는 취약점 건수가 아니라 조치할 대상 수(앱 · 이미지)로 센다.** 건수는 행에서 보인다 | 같은 베이스 이미지의 CVE 가 앱 수만큼 겹쳐 세지고, 이미지 스캔과 SCA 는 앱 의존성 구간이 겹친다(#79 §2). 대상 수로 세면 겹침과 규모 차이가 같이 풀린다 |
| 5 | **지표는 다섯 개다**(§3) — 차단된 앱 · 시크릿 탐지 앱 · CRITICAL 보유 앱 · 판정 불가 앱 · 설치 OSS 조치 대상 이미지(A·B 등급) | 이슈의 "5개 내외". 각 지표가 서로 다른 조치로 이어진다. 설치 OSS 는 CVE 대응 정책의 등급(A·B)을 그대로 쓴다 |
| 6 | **셀 상태는 여섯 가지** — 통과 · 경고 · 차단 · 판정 불가 · 기록 없음 · 미적용. 뒤의 셋은 **초록으로 보이지 않는다**. 의존성 스캔이 검사 대상 0 개로 통과한 것도 초록이 아니라 "대상 없음" 이다 | 000070 의 교훈: 돌지 않은 것을 성공으로 보여주지 않는다. 스캐폴딩은 락파일을 만들지 않아(#79 §1) 대부분의 앱이 "0 개 검사 · 통과" 다 |
| 7 | **다중 스택·다중 앱:** 기본 범위는 조직 전체다. 앱 표를 **스택별로 묶고** 묶음 머리에 집계를 단다. 행은 **위험순**으로 정렬하고 "문제 있는 것만"을 기본으로 켠다. 스택 하나를 고르면 묶음을 푼다 | 앱은 스택 하나에만 속한다(`pipelines.stack_id`). 스택이 여럿이어도 어느 스택이 문제인지가 먼저 보여야 하고, 통과한 수백 행은 가려야 문제가 보인다 |
| 8 | **상세 화면은 새로 만들지 않는다.** 앱 행은 오른쪽 패널에 단계별 요약만 보이고 CI/CD 실행 기록으로, 스택 행은 스택 상세의 "이미지 취약점" 탭으로 **딥링크**한다 | 실행 단위 상세(`ImageScanDetail` · `SASTResultDetail` · 취약점 목록)는 이미 있다. 상세가 두 곳에 있으면 한쪽이 낡는다 |
| 9 | **선행 구현: SCA·시크릿 결과 저장.** 두 스캔은 CI 산출물만 있고 DB 에 아무것도 없다. 같은 변경에서 먼저 넣는다(§8) | 이 둘이 없으면 매트릭스 네 열 중 두 열이 비고, 경고를 통과와 가르지 못한다 |
| 10 | **백엔드는 observability 모듈의 읽기 모델이다.** 결과 테이블을 새로 만드는 것은 cicd(§8)뿐이고, 대시보드는 cicd·stack 의 공개 경로를 `adapter/bridge` 로 읽는다 | 결과 테이블은 cicd(`image_scan_results` · `sast_results`)와 stack(`stack_image_scans`)이 소유한다. #76 §8 이 "#65 는 cicd 의 공개 유스케이스로 읽는다" 고 정해 두었다 |
| 11 | **API 는 권한별로 둘로 나눈다** — 앱 축은 모든 역할, 설치 OSS 축은 admin · devops 만 | 지금 `/stacks/*` 는 admin · devops 만 쓸 수 있다. 대시보드가 이 경계를 우회하면 안 된다 |
| 12 | **조직은 토큰(principal)에서 정한다.** 대시보드가 링크하는 기존 단건 스캔 조회의 조직 검사 누락도 같이 막는다(§8 G4) | 보안 화면이 다른 조직의 스캔 결과로 가는 길이 되면 안 된다 |
| 13 | **추이 차트 · 앱 쪽 알림 · 정책 편집은 이번 범위가 아니다**(§10) | 설치 OSS 결과는 이력이 없고(통째 교체) 앱 결과는 `main` 만 있다. 추이는 일별 스냅샷을 쌓은 뒤 붙인다. 설치 OSS 쪽 알림은 이미 있다(`stack_cve_found`, #354) |

---

## 1. 저장소 확인 결과 (2026-10-20)

### 1.1 엔터티 — "앱" 은 파이프라인이다

| 확인 항목 | 결과 | 근거 |
|---|---|---|
| 앱 테이블 | **없다.** 파이프라인 한 행이 앱 하나다(CI 잡 이름 = 앱 이름) | `internal/cicd/adapter/handler/pipeline_handler.go:1195`, `internal/cicd/port/ci_server.go:11` |
| 앱 → 스택 | `pipelines.stack_id` 하나, NULL 가능(스택 없는 직접 배포 · 스택 삭제 시 `SET NULL`) | `000022_pipeline_stack_relation` |
| 조직 → 스택 | 여럿. 클러스터마다 하나(OpenBao ClusterRoleBinding 때문, DB 제약은 아님) | `000001_init`, `docs/20_아키텍처/Nullus_시스템_아키텍처.md:231` |
| 환경(dev/stg/prod) | **없다** | — |
| 브랜치 · 커밋 SHA | **저장하지 않는다.** 동기화는 `main` 의 최근 30 개 빌드만 읽는다 | `internal/cicd/usecase/sync_pipeline_runs.go:43` |
| 파이프라인의 실제 단계 | `pipelines.stages` (예: `SecretScan · DependencyScan · Build · SAST · ImageScan · Deploy`). 비어 있으면 "모름"(이 칸 이전 파이프라인) | `internal/cicd/domain/pipeline.go:65-71`, `000077` |
| 역할 | admin · devops · developer, **조직 단위뿐**(팀·프로젝트 권한 없음) | `internal/admin/domain/user.go:8-12` |

> `GET /observability/deployed-apps` 는 이름과 달리 **스택마다 한 줄**을 돌려준다(`dashboard_handler.go:146-187`). 이 문서의 "앱" 은 파이프라인이다.

### 1.2 이미 저장되는 보안 결과

| 출처 | 테이블 · 소유 | 판정 | 심각도 건수 | 보존 | 조회 API · 화면 |
|---|---|---|---|---|---|
| SAST (SonarQube) | `sast_results` · cicd | pass · warn · block · error | 없음 — SonarQube 지표(bugs · vulnerabilities · security_hotspots · coverage …) | 실행마다 한 행. **리포트를 읽은 실행만** | `GET /cicd/pipelines/:id/sast-results` · CI/CD 실행 기록 |
| 이미지 스캔 (Trivy) | `image_scan_results` · cicd | pass · warn · block · error | critical · high · medium · low · unknown (정책의 `ignore_unfixed` 를 따름) | 실행마다 한 행 | `GET /cicd/pipelines/:id/image-scans` · CI/CD 실행 기록 |
| 의존성 스캔 (Trivy fs) | **없음** — `dependency-scan-report.json` 은 CI 산출물로만 | — | — | — | 단계 상태만 |
| 시크릿 탐지 (gitleaks) | **없음** — `gitleaks-report.json`(값 가림)은 CI 산출물로만 | — | — | — | 단계 상태만 |
| 설치 OSS 이미지 | `stack_image_scans` · `stack_image_vulnerabilities` · stack | **없음(보고용)**. 스택 상태 scanned · pending · not_scanned(`scanner_not_installed`) | `counts` · `fixable_counts` JSONB, 이미지마다 `exposed`(외부 노출, 000103) · `shared`(공용 구성요소, 000104) | **최신만**(통째 교체, 이력 없음) | `GET /stacks/:stackId/image-scans` · 스택 상세 "이미지 취약점" 탭 |
| 스캔 정책 | `image_scan_policies` · cicd, PK `stack_id` | — | — | 스택마다 한 행 | `GET·PUT /stacks/:stackId/image-scan-policy` · **화면 없음** |

- 설치 OSS 스캔은 #354~#356 으로 넓어졌다 — 클러스터 공용 구성요소(cert-manager · metrics-server · External Secrets)도 보고, **에어갭 설치도 스캔한다**(보고서 `airgap = true`, DB 는 반입한 날짜 기준). 옛 사유 `airgap` 은 읽기만 남았다.
- 설치 OSS 의 CVE 대응 등급(`Nullus_설치_OSS_CVE_대응_정책.md` §3.1): **A** = CRITICAL · 수정판 있음 · 외부 노출, **B** = CRITICAL · 수정판 있음 · 내부, **C** = HIGH · 수정판 있음, **D** = 수정판 없음.

공통 어휘는 이미 있다 — 판정 `domain.GateResult`(pass · warn · block · error), 단계 키 `port.StageKey`, 심각도 건수와 DB 신선도(`IsDBStale`, 30일)는 `internal/shared/domain/image_scan_report.go`.

### 1.3 다중 스택 화면에서 걸리는 것

- 결과 테이블 어디에도 `org_id` · `stack_id` 가 없다. cicd 안에서는 `pipelines` 와 조인하면 되고(같은 모듈), stack 쪽은 `stacks.org_id` 로 거른다.
- 조직 전체·스택 전체를 모아 보는 보안 API 는 **없다** — 모두 파이프라인 하나 · 스택 하나 단위다.
- 단건 조회(`/cicd/pipelines/:id/image-scans|sast-results`, `/stacks/:id/image-scans`)는 **조직 소유를 확인하지 않는다**(§8 G4).
- `/observability/*` 라우트 그룹은 역할 검사가 없다(로그인만). `/stacks` 는 admin · devops 만이다.

### 1.4 화면 쪽

- 사이드바는 **2단만** 된다 — `NavGroup.items: NavItem[]`, `NavItem` 에 `children` 이 없다(`web/src/components/layout/nav-model.tsx:13-27`).
- `/cicd/list` · `/stack/list` 는 선택된 파이프라인 · 스택을 컴포넌트 상태로만 들고 있어 **URL 로 열 수 없다**(딥링크 없음).
- 재사용할 수 있는 부품은 충분하다(§6).

---

## 2. 화면에 무엇을 놓나 — 축과 단위

```
조직
 ├─ 스택 A (클러스터 a)  ──  설치 OSS 이미지 36개      ← 설치 OSS 탭 (보고)
 │    ├─ 앱 order-api   ──  시크릿 · 의존성 · SAST · 이미지  ← 앱 탭 (게이트)
 │    └─ 앱 web-front   ──  …
 ├─ 스택 B (클러스터 b)
 │    └─ …
 └─ 스택 없음            ──  직접 배포 · 스택이 지워진 앱
```

**앱 한 행 = 파이프라인 하나**, 열은 파이프라인 단계 순서(`scaffold.PipelineStageNames`)를 따른 보안 단계 넷이다 — 시크릿 → 의존성 → SAST → 이미지. 사용자가 파이프라인 화면에서 보는 순서와 같다.

### 2.1 셀 하나의 값 — 단계별 최신 기록

셀은 그 앱 `main` 의 **그 단계 최신 기록**이다. 앱의 최신 실행과 다를 수 있다 — 앞 단계가 차단하면 뒤 단계는 건너뛰기 때문이다(예: 시크릿이 막으면 이미지 스캔은 돌지 않는다).

| 대안 | 문제 |
|---|---|
| 최신 실행 하나의 결과만 | 앞 단계가 막힌 실행에서는 이미지 셀이 "건너뜀" 이 된다. 지금 돌고 있는 이미지에 CRITICAL 이 있다는 사실이 사라진다 |
| **단계별 최신 (채택)** | 셀마다 실행 번호가 다를 수 있다 → 최신 실행이 아니면 셀에 `#38` 을 흐리게 달고 툴팁으로 "최근 실행 #42 에서는 이 단계의 결과가 없다" 를 보인다 |

### 2.2 셀 상태

| 상태 (`state`) | 조건 | 표시 |
|---|---|---|
| 통과 `pass` | `gate_result = pass` | 초록. 단 의존성 스캔이 검사 대상 0 개면 회색 "대상 없음" |
| 경고 `warn` | `warn` | 노랑 |
| 차단 `block` | `block` | 빨강 |
| 판정 불가 `error` | 스캐너 장애 · 리포트 없음 | 회색 + 경고 아이콘. "통과 아님" |
| 기록 없음 `unrecorded` | 파이프라인에 단계는 있는데 `main` 결과가 아직 없다 | 점선 테두리 |
| 미적용 `not_applied` | 파이프라인 `stages` 에 단계가 없다. 시크릿은 스택 정책 `secret_detection = off` 도 | `—` |

- `stages` 가 비어 있는 옛 파이프라인은 결과 행이 있으면 그것을 보이고, 없으면 "미적용" 으로 둔다.
- 셀 안 숫자: 시크릿 = 탐지 건수, 의존성·이미지 = `C2 H5`(CRITICAL · HIGH), SAST = 취약점 · 핫스팟 수. 이미지·의존성은 DB 가 30일을 넘으면 `DB 34일` 배지를 단다(`IsDBStale`).

### 2.3 행 정렬 — 위험순 (`rank`)

| rank | 조건 | 이유 |
|---|---|---|
| 1 | 시크릿 셀이 `block` · `warn` | 커밋을 지워도 이미 유출이다(#80). 배포 여부와 무관하게 사고다 |
| 2 | 셀 하나라도 `block` | 배포가 멈춰 있다 |
| 3 | 셀 하나라도 `error` · `unrecorded` | 상태를 모른다 |
| 4 | 셀 하나라도 `warn` | |
| 5 | 적용된 셀이 모두 `pass` | |
| 6 | 모든 셀이 `not_applied` | 보안 단계가 없다 |

같은 rank 안에서는 CRITICAL 합(의존성 + 이미지) 많은 순 → 최근 실행 시각 최근 순 → 이름 순. **rank 와 정렬은 백엔드 도메인이 계산해 내려준다** — 화면은 받은 순서를 그대로 쓰고 묶기만 한다. "문제 있는 것만" 은 rank 1~4 다.

---

## 3. 지표 다섯 개

| # | 지표 | 정의 | 출처 | 이어지는 조치 | 역할 |
|---|---|---|---|---|---|
| M1 | **차단된 앱** | 셀 하나라도 `block` 인 앱 수 / 보안 단계가 하나라도 있는 앱 수 | 네 결과의 `gate_result` | 막힌 단계를 고친다 — 배포가 멈춰 있다 | 모두 |
| M2 | **시크릿 탐지 앱** | 시크릿 셀이 `block` 또는 `warn` 인 앱 수. **경고 정책이어도 센다** | `secret_scan_results`(신설, §8) | 즉시 폐기·회전 | 모두 |
| M3 | **CRITICAL 보유 앱** | 이미지 셀 또는 의존성 셀의 `critical > 0` 인 앱 수 | `image_scan_results` · `dependency_scan_results`(신설) | 의존성·베이스 이미지 갱신 | 모두 |
| M4 | **판정 불가 앱** | 셀 하나라도 `error` 또는 `unrecorded` 인 앱 수 | 위 넷 + `pipelines.stages` | 스캐너 · 러너 · 리포트 경로 점검 | 모두 |
| M5 | **설치 OSS 조치 대상 이미지** | 수정판 있는 CRITICAL 을 가진 설치 이미지 수 = **A + B**. 보조 줄에 "외부 노출(A) N · 스캔 안 함 M 개 스택" | `stack_image_scans.fixable_counts` · `exposed` | OSS 업그레이드(#70) · CVE 대응(#78, A 는 고지 24h · 검증된 업그레이드 7일) | admin · devops |

- **M1–M4 는 범위 필터(클러스터 · 스택 · 검색)를 따르고 "문제 있는 것만" 은 따르지 않는다.** 표를 줄여도 KPI 는 그대로다.
- **M3 의 건수는 스택 정책을 따른다.** `ignore_unfixed` 가 켜져 있으면(기본) 수정판 있는 것만 센다. 카드 설명에 "정책 기준" 을 적는다.
- **M5 의 등급은 이미지 단위로 매긴다.** 정책의 등급은 CVE 단위지만 KPI 는 대상 수이므로, 수정판 있는 CRITICAL 이 하나라도 있으면 그 이미지는 A(외부 노출) 또는 B(내부)다.
- **적용률은 KPI 가 아니라 열 머리에 둔다** — `SAST 12/24` 처럼. 미적용은 스택에서 스캐너를 고르지 않은 **선택**이라 KPI 로 세면 경고가 상시로 켜진다. 대신 몇 개 앱에 적용됐는지는 항상 보인다.
- **이미지 스캔 설계 §8 이 넘긴 지표와 대응** — "앱별 최신 이미지 취약점 건수" → 행 · M3, "게이트 차단 건수/통과율" → M1, "취약점 DB 신선도" → 셀 배지, "스캔 실패 파이프라인 수" → M4.
- SAST 에는 심각도 건수가 없다(SonarQube 지표만). SAST 는 M1 · M4 에만 들어가고 M3 에는 넣지 않는다.

---

## 4. 화면

### 4.1 메뉴

```
관측성                         (ChartColumn)
 ├ 모니터링 대시보드            (LayoutDashboard)
 ├ 알림 이력                    (History)
 └ 보안                         (Shield)        ← 하위 그룹, 신설
    └ 보안 대시보드              (ShieldCheck)   ← /observability/security/dashboard
```

i18n: `sidebar.security`(Security · 보안), `sidebar.securityDashboard`(Security Dashboard · 보안 대시보드). 세 역할 모두에게 보인다 — 개발자는 앱 탭만 본다(§4.4).

### 4.2 앱 탭 (기본)

```
관측성 › 보안 › 보안 대시보드                                      main 기준 · 실행 기록 10분 주기 동기화
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ [클러스터: 전체 ▾] [스택: 전체 (3) ▾]  [🔍 앱 이름]   [☑ 문제 있는 것만]               │
├────────────────┬────────────────┬────────────────┬────────────────┬──────────────────┤
│ 차단된 앱       │ 시크릿 탐지 앱  │ CRITICAL 보유 앱 │ 판정 불가 앱     │ 설치 OSS 조치 대상 │
│ 3 / 22         │ 1              │ 5              │ 2              │ 12 이미지          │
│                │ 즉시 폐기·회전   │ 정책 기준        │ 스캐너 점검      │ 외부 노출 3 · 미스캔 1 │
└────────────────┴────────────────┴────────────────┴────────────────┴──────────────────┘

[ 앱 24 ] [ 설치 OSS 3 ]
                      시크릿 20/24   의존성 14/24   SAST 12/24    이미지 20/24   최근 실행
▼ stack-prod · 클러스터 prod-a — 앱 12 · 차단 2 · 시크릿 1 · 판정 불가 1
  order-api           ■ 차단 2건      ■ 차단 C3 H5   ▲ 경고 취약 4   ■ 차단 C2 H11  #42 · 2시간 전
  payment-api         ● 통과          ○ 대상 없음     ■ 차단 취약 7   ● 통과 C0 H3   #17 · 어제
  billing-batch       ● 통과          ◌ 기록 없음     —             ◐ 판정 불가     #9 · 3일 전
▼ stack-stg · 클러스터 stg-a — 앱 8 · 차단 1
  web-front           ● 통과          ▲ 경고 C0 H2   —             ■ 차단 C1  #38  #40 · 1시간 전
▶ stack-dev · 클러스터 dev-a — 앱 2 · 문제 없음 (접힘)
▶ 스택 없음 — 앱 2 · 보안 단계 없음 (접힘)
                                                          통과한 앱 16 개를 숨겼습니다 [모두 보기]
```

- **범위 필터** — 클러스터 · 스택은 기존 `ClusterStackFilter` 에 "전체" 값을 더해 쓴다. 지금 컴포넌트는 첫 클러스터를 골라 열지만, 이 화면의 기본은 조직 전체다.
- **묶음 머리** — 스택 이름 · 클러스터 · 앱 수 · 문제 수. 문제가 없는 묶음은 접어서 연다.
- **스택 하나를 고르면** 묶음을 풀고 평평한 표로 보인다.
- **"문제 있는 것만"(기본 켬)** — rank 5 · 6 행을 숨기고 숨긴 수를 아래에 적는다.
- **"스택 없음" 묶음** — `stack_id` 가 NULL 인 앱. 스택 정책이 없으므로 시크릿 셀은 단계 유무로만 판단한다.

### 4.3 앱 상세 패널 (행 선택)

`ListDetailPanel` 의 오른쪽 패널. **요약만** 보이고 상세는 기존 화면으로 보낸다.

```
order-api                       stack-prod · 최근 실행 #42 (실패) · 2시간 전
─────────────────────────────────────────────
시크릿    ■ 차단   2건 — github-pat · aws-access-key       #42   [리포트 ↗]
          커밋을 지워도 이미 유출입니다. 즉시 폐기·회전하세요.
의존성    ■ 차단   C3 H5 M12 · 대상 2                       #42   [리포트 ↗]
SAST      ▲ 경고   걸린 조건 new_violations · 취약 4 · 핫스팟 2  #41   [SonarQube ↗]
이미지    ■ 차단   C2 H11 · registry/order-api@sha256:ab…  #41   DB 2일
─────────────────────────────────────────────
[실행 기록 열기 →]   (/cicd/list?pipeline=<id>&tab=history&run=<deploymentId>)
```

### 4.4 설치 OSS 탭 (admin · devops)

```
스택          클러스터   상태                    이미지  A   B   C   수정판 CRITICAL  HIGH   가장 심각한 이미지          마지막 스캔   DB
stack-prod   prod-a    스캔됨                   36    3   5   9   8               120    gitlab-toolbox  C61 H1569   3시간 전    2일
stack-stg    stg-a     스캔됨 · 실패 1 · 에어갭    31    0   4   6   4                97    harbor-core     C3          5시간 전    41일 (반입 기준)
stack-dev    dev-a     스캔 안 함 · 스캐너 미설치   —     —   —   —   —                —     —                           —           —
                                                                    [행 선택 → 스택 상세 › 이미지 취약점 탭]
```

- A · B · C 는 **이미지 수**다 — A: 수정판 있는 CRITICAL + 외부 노출, B: 수정판 있는 CRITICAL + 내부, C: 수정판 있는 CRITICAL 은 없고 HIGH 만 있는 이미지.
- 숫자는 `fixable_counts` 다(수정판이 있는 것). 업스트림 이미지는 사용자가 고칠 수 없는 경우가 많아 #76 §12.4 가 보고용으로 둔 것과 같은 이유다.
- "스캔 안 함" 은 0 으로 보이지 않는다 — 사유(`scanner_not_installed`)를 그대로 적는다. 에어갭 스택은 "반입 뒤 공개된 CVE 는 다음 번들을 반입해야 보인다" 를 툴팁으로 밝힌다.
- **개발자 역할**: 이 탭과 M5 카드를 감춘다(KPI 넷). 지금 `/stacks/*` 를 쓸 수 없는 역할이다. 화면은 개발자에게 이 API 를 부르지도 않는다.

### 4.5 빈 상태

| 경우 | 문구 방향 |
|---|---|
| 앱이 하나도 없다 | "아직 파이프라인이 없습니다" + 파이프라인 목록 링크 |
| 앱은 있는데 보안 단계가 하나도 없다 | "스택에서 보안 스캐너를 고르지 않았습니다" + 스택 Security 탭 안내. **0건 통과처럼 보이지 않게** |
| 필터 결과가 없다 | "조건에 맞는 앱이 없습니다" |
| 설치 OSS — 모든 스택이 스캔 안 함 | "설치 이미지를 스캔하는 스택이 없습니다" + 스택에 Trivy 를 더하면 스캔한다는 안내 |

---

## 5. 백엔드

### 5.1 모듈 배치

```
internal/observability/
  domain/security_posture.go          셀 상태 · rank · 정렬 · KPI · 스택 등급 계산 (순수 함수)
  port/security.go                    AppSecuritySource · StackSecuritySource · ScopeResolver
  usecase/get_app_security.go         범위 필터 · 묶음 · 정렬 · problem_only
  usecase/get_stack_security.go
  adapter/bridge/security_sources.go  cicd · stack 공개 경로를 관측 도메인 타입으로 옮긴다
  adapter/handler/security_handler.go
internal/cicd/
  (공개) 조직 앱의 단계별 최신 결과 — 파이프라인 · 최신 실행 · 네 결과의 최신 행 · 스택 시크릿 정책
  adapter/repository/…                DISTINCT ON (pipeline_id) … ORDER BY scanned_at DESC, pipelines 와 조인
internal/stack/
  (공개) 조직 스택의 스캔 보고 — 기존 ListByStack 위에 조직 단위 목록
```

- 패턴은 `internal/observability/adapter/bridge/monitored_stacks.go` 와 같다. 다른 모듈의 타입은 bridge 안에서만 만진다.
- 시크릿 셀의 "미적용" 판정에 쓰는 스택 정책(`image_scan_policies.secret_detection`)도 cicd 소유라 같은 cicd 경로가 함께 돌려준다.
- 대시보드 자체의 테이블은 없다. 단계별 최신은 기존 인덱스 `(pipeline_id, scanned_at DESC)` 로 계산한다. 추이가 필요해지면 스냅샷 테이블을 둔다(§10).

### 5.2 API 계약

| 메서드 · 경로 | 역할 |
|---|---|
| `GET /api/v1/observability/security/apps?cluster_id=&stack_id=&q=&problem_only=true` | 모두 |
| `GET /api/v1/observability/security/stacks?cluster_id=&stack_id=` | admin · devops (그 밖은 403) |

`problem_only` 기본값은 `false` 다(화면이 `true` 로 부른다). `stack_id=none` 은 스택 없는 앱이다.

```jsonc
// GET /observability/security/apps
{
  "generated_at": "2026-10-20T09:12:00Z",
  "truncated": false,                       // 앱 상한(1000)에 닿았다
  "summary": { "apps": 24, "with_security_stage": 22,
               "blocked": 3, "secrets": 1, "critical": 5, "undetermined": 2 },
  "coverage": { "secret_scan": 20, "dependency_scan": 14, "sast": 12, "image_scan": 20 },
  "groups": [ { "stack_id": "stk_…", "stack_name": "stack-prod",      // 스택 없음: "" · ""
                "cluster_id": "…", "cluster_name": "prod-a",
                "apps": 12, "blocked": 2, "secrets": 1, "critical": 1, "undetermined": 1, "problems": 3 } ],
  "apps": [ {                                // rank · 정렬 순서 그대로
    "pipeline_id": "pip_app_order-api_…", "name": "order-api", "stack_id": "stk_…",
    "rank": 1,
    "latest_run": { "deployment_id": "dep_ci_…_42", "version": "#42",
                    "status": "failed", "started_at": "…" },            // 실행이 없으면 null
    "cells": {
      "secret_scan":     { "state": "block", "deployment_id": "…_42", "version": "#42", "at": "…",
                           "stale_run": false, "findings": 2,
                           "rules": [ { "rule_id": "github-pat", "count": 1 } ], "link": "…" },
      "dependency_scan": { "state": "block", "deployment_id": "…_42", "version": "#42", "at": "…",
                           "stale_run": false, "targets": 2,
                           "counts": { "critical": 3, "high": 5, "medium": 12, "low": 0, "unknown": 0 },
                           "db_stale": false, "link": "…" },
      "sast":            { "state": "warn", "deployment_id": "…_41", "version": "#41", "at": "…",
                           "stale_run": true, "vulnerabilities": 4, "security_hotspots": 2,
                           "failed_conditions": [ "new_violations" ], "link": "…" },
      "image_scan":      { "state": "block", "deployment_id": "…_41", "version": "#41", "at": "…",
                           "stale_run": true, "image": "registry/order-api@sha256:ab…",
                           "counts": { "critical": 2, "high": 11, "medium": 0, "low": 0, "unknown": 0 },
                           "db_stale": false, "link": "…" }
    }
  } ]
}
```

- 셀의 `state` 가 `unrecorded` · `not_applied` 면 `state` 만 있다. 숫자 칸은 모르면 **빠진다** — 0 으로 채우지 않는다. 리포트를 못 읽은 셀(`error`)은 `counts` · `db_stale` · `db_updated_at` 이 함께 빠진다.
- `dependency_scan` · `image_scan` 셀은 `db_updated_at` 도 싣는다(화면의 "DB 34일"). `rules` · `failed_conditions` 는 결과가 있으면 늘 배열이다.
- `summary` · `coverage` · `groups` 는 범위 필터(cluster · stack · q)만 따르고 `problem_only` 는 따르지 않는다.
- `groups` 는 범위 안에 앱이 있는 스택만, 가장 위험한 앱의 rank → 문제 수 → 이름 순이고 "스택 없음" 은 늘 마지막이다. 스택이 지워진 앱은 `stack_id: ""` 로 "스택 없음" 에 든다.

```jsonc
// GET /observability/security/stacks
{
  "generated_at": "…",
  "summary": { "stacks": 3, "scanned_stacks": 2, "not_scanned_stacks": 1,
               "action_images": 12, "grade_a_images": 3, "grade_b_images": 9 },
  "stacks": [ {
    "stack_id": "stk_…", "stack_name": "stack-prod", "cluster_id": "…", "cluster_name": "prod-a",
    "state": "scanned", "reason": "", "airgap": false,
    "images": 36, "failed_images": 0, "shared_images": 5,
    "grade_a_images": 3, "grade_b_images": 5, "grade_c_images": 9,
    "fixable": { "critical": 8, "high": 120, "medium": 0, "low": 0, "unknown": 0 },
    "worst_image": { "image": "…/gitlab-toolbox:…", "critical": 61, "high": 1569, "exposed": false },
    "scanned_at": "…", "db_updated_at": "…", "db_stale": false
  } ]
}
```

- `/security/stacks` 행의 모르는 값은 빠지지 않고 `null` 이다(스캔하지 않는 스택의 이미지 수 · 등급 · 건수 등). `state` 가 `pending`(첫 스캔 대기)인 스택은 `scanned_stacks` · `not_scanned_stacks` 어디에도 세지 않는다. `worst_image` 는 전체 건수로 고르고, `db_updated_at` 은 스캔한 이미지 중 가장 오래된 DB 시각이다.
- **권한별로 나눈 이유** — `/observability` 그룹은 역할 검사가 없다. 한 응답에 섞으면 핸들러 안에서 필드를 지워야 하고, 빠뜨리면 그대로 샌다. 두 번째 경로에만 admin · devops 검사를 단다.
- **조직** — `sharedmw.OrgIDFromEchoContext`(principal 먼저). `X-Org-ID` 헤더만 믿는 기존 대시보드 핸들러(`dashboard_handler.go:191`)를 따라 하지 않는다.
- **규모** — 조직의 앱을 한 번에 돌려준다. 동기화 대상과 같은 상한(1000)을 두고, 넘으면 `truncated: true` 로 화면에 알린다. 행 하나가 작고(셀 넷) "문제 있는 것만" 이 서버에서 먼저 거른다. 상한에 닿는 조직이 생기면 페이지를 나눈다.
- **신선도** — 응답은 저장된 결과만 읽고 CI 를 부르지 않는다. 결과는 10분 주기 동기화(`CICD_RUN_SYNC_INTERVAL`)와 실행 기록 화면 조회 때 들어온다.

---

## 6. 기존 UI 재사용 판단 (이슈 태스크 ②)

| 부품 | 위치 | 판단 |
|---|---|---|
| `KpiCard` | `features/observability/components/monitoring-chart-widgets.tsx` | **그대로** — KPI 다섯 칸 |
| `DataTable` · `ListDetailPanel` · `SearchInput` | `components/shared/` | **그대로** — 알림 이력 화면(`alert-history-page.tsx`)과 같은 목록 + 오른쪽 패널 구성 |
| `ClusterStackFilter` | `features/observability/components/cluster-stack-filter.tsx` | **확장** — "전체" 값. 지금은 첫 클러스터를 골라 연다 |
| `SeverityBadge` · `SeverityCounts` | `components/shared/severity-counts.tsx` | **그대로** — 셀 안 `C2 H5`, 설치 OSS 표 |
| `ImageScanGateBadge` · `SASTGateBadge` | `features/cicd/components/` | **공유 위치로 옮긴다** — 판정 어휘가 같은 배지가 둘이다. 대시보드는 네 종류를 쓰므로 `components/shared/gate-badge.tsx` 로 합친다(관측 기능이 cicd 기능 폴더를 import 하지 않게) |
| `ImageScanDetail` · `SASTResultDetail` · `VulnerabilityList` | cicd · stack · shared | **쓰지 않고 링크한다** — 상세는 한 곳(결정 8) |
| `Tabs` · `PageHeader` · `SkeletonTable` | `components/ui` · `layout` | 그대로 |
| 빈 상태 | — | 공용 컴포넌트가 없다. 이 화면에서는 기존처럼 인라인으로 두고, 공용화는 별도로 |
| 기간 선택 | — | **쓰지 않는다** — 이번 화면은 현재 상태만 보인다 |

모니터링 대시보드 안의 탭으로 넣는 안은 택하지 않았다 — 그 화면은 클러스터·스택 하나를 골라 보는 구조(첫 클러스터 자동 선택, 범위별 임베드 탭)이고, 이 화면은 조직 전체를 한 번에 본다.

Test/Coverage 설계 §4 는 "데이터가 없는 시점의 전용 대시보드는 빈 화면" 이라며 Security Dashboard 도 같은 함정이라 적었다. 그때와 달리 지금은 SAST · 이미지 스캔 · 설치 OSS 결과가 실제로 쌓이고 있고, 비어 있는 두 출처(SCA · 시크릿)는 같은 변경에서 먼저 채운다(§8). 추이처럼 아직 데이터가 없는 것은 그리지 않는다(§10).

---

## 7. 사이드바 3단화

| 바꿀 곳 | 내용 |
|---|---|
| `components/layout/nav-model.tsx` | `NavItem` 에 `children?: NavItem[]`. 자식이 있는 항목은 펼침 단추이고 `path` 는 첫 자식으로 보내는 용도 |
| `components/layout/sidebar.tsx` | 하위 그룹 펼침 상태, 들여쓰기 한 단, 현재 경로가 자식이면 열린 채로. **접힌 아이콘 레일에서는 자식을 평평하게** 보인다 |
| `resolveNavGroupLabel` · `PageHeader` | 자식까지 찾는다. 브레드크럼은 `관측성 › 보안 › 보안 대시보드` |
| `nav-icon-consistency.test.ts` | 정규식 분할이 중첩을 못 읽는다 — 파서를 고친다. 그룹 안 아이콘 중복 금지는 하위 그룹까지 적용 |
| `sidebar.test.tsx` · `e2e/uat/rbac-menu-visibility.spec.ts` | 역할별 메뉴 수 |
| `features/admin/pages/user-management-page.tsx` | 정적 메뉴 권한 목록 |
| `app/routes.tsx` | `observability/security/dashboard`(로그인만), `observability/security` → 리다이렉트 |

---

## 8. 선행 구현 — 데이터 공백

| # | 공백 | 할 일 | 소유 |
|---|---|---|---|
| G1 | **SCA 결과가 저장되지 않는다** | `dependency_scan_results`(000106) — 이미지 스캔과 같은 틀(파이프라인 · 실행 · 심각도 건수 · 검사 대상 수 · 판정 · 스캐너 버전 · DB 시각 · 리포트 위치), 이미지 칸 없음. 동기화가 `dependency-scan-report.json` 을 읽는다 — `ParseTrivyReport` · `GateFromStageAndReport` · `GateWhenReportMissing` 재사용. 검사 대상 수는 리포트의 `Results` 수 | cicd |
| G2 | **시크릿 결과가 저장되지 않는다** | `secret_scan_results`(000107) — 탐지 건수 · 규칙별 건수 · 탐지 위치(규칙 · 파일 · 줄 · 커밋 · Fingerprint) · 판정 · 리포트 위치. **비밀값(`Secret` · `Match`)은 파싱하지도 저장하지도 않는다.** 판정은 리포트에 종료 코드가 없어 추론한다(아래) | cicd |
| G3 | **SAST 가 단계 실패 + 리포트 없음이면 흔적이 없다** | 그 경우에만 `error` 행을 남긴다. 성공 + 리포트 없음은 지금처럼 건너뛴다 — 리포트 기능 이전 파이프라인일 수 있다(`sync_pipeline_runs_sast.go` 주석) | cicd |
| G4 | **단건 스캔 조회가 조직을 확인하지 않는다** | `/cicd/pipelines/:id/image-scans · sast-results · …/vulnerabilities` 와 새 두 경로, `/stacks/:stackId/image-scans` 에 조직 소유 검사(다른 조직이면 404) | cicd · stack |
| G5 | **딥링크가 없다** | `/cicd/list?pipeline=&tab=history&run=`, `/stack/list?stack=&tab=image-scans` 를 읽어 해당 행을 펼친다 | ui |

**시크릿 판정 추론(G2)** — 스크립트(`scaffold/secret_scan.go`)는 탐지 시 정책에 따라 차단(exit 1) · 경고(exit 0)하고, 스캔 실패는 장애 정책을 따른다.

| 리포트 | 단계 | 판정 |
|---|---|---|
| 없음 | 끝남(성공·실패) | `error` (`GateWhenReportMissing`) |
| 탐지 > 0 | 실패 | `block` |
| 탐지 > 0 | 성공 | `warn` (경고 정책이 통과시켰다) |
| 탐지 0 | 성공 | `pass` |
| 탐지 0 | 실패 | `error` (탐지 없이 실패 — 스캔이 실패했다) |
| — | 스택 정책 `off` | 기록하지 않는다(미적용) |

**실행 기록 화면** — G1 · G2 를 넣으면 CI/CD 실행 기록의 실행 행과 상세에 두 요약을 이미지 스캔 · SAST 옆에 붙인다. 실행 단위 상세는 그 화면이 갖는다(결정 8). 조회 경로는 기존과 같은 모양이다 — `GET /api/v1/cicd/pipelines/:id/dependency-scans`, `GET /api/v1/cicd/pipelines/:id/secret-scans`.

> SCA · 시크릿 설계 문서는 저장 틀로 `pipeline_image_scans` 를 적었지만 실제 테이블 이름은 `image_scan_results` 다. 두 문서의 표기를 이 변경에서 고쳤다.

---

## 9. 구현 범위와 검증

한 변경으로 넣는다 — 선행(§8)이 없으면 대시보드 네 열 중 둘이 비기 때문이다.

| 묶음 | 내용 |
|---|---|
| cicd | G1 · G2 · G3 + 조회 경로 둘 + 대시보드용 공개 경로(조직 앱의 단계별 최신) |
| stack | 조직 스택의 스캔 보고 공개 경로 |
| 권한 | G4 |
| observability | §5 — 도메인 · bridge · 유스케이스 · API 둘 |
| 화면 | §4 · §6 · §7 · G5 + 실행 기록 두 요약 + i18n(en/ko) |
| 문서 사이트 | `features/observability/security-dashboard.md` + `.en.md`(영어판은 `en.json` 문구 인용), `index.md` 의 "화면은 둘입니다" 갱신, `mkdocs.yml` nav · `nav_translations`, `reference/rbac.md` 의 라우트 표와 역할별 메뉴 수, 캡처(`npm run shots:docs`) |

**테스트(TDD 순서)**

- Domain: 셀 상태 도출(6 상태 × 단계 유무 × 정책 off), rank · 정렬, KPI, 스택 등급(A·B·C), 시크릿 판정 추론(G2)
- UseCase: 범위 필터 · 묶음 · `problem_only` · `truncated` (포트 모킹)
- Handler: 역할 가드(개발자 `/security/stacks` 403) · 조직 범위(httptest)
- Repository: 단계별 최신 `DISTINCT ON` · 조직 조인 · 새 두 테이블 왕복(testcontainers)
- FE: 묶음 · 접힘 · "문제 있는 것만"(vitest), 개발자 역할에서 설치 OSS 탭 · M5 감춤(testing-library), 메뉴 3단(사이드바 · 브레드크럼)

---

## 10. 이 카드에서 닫지 않는 것

| 항목 | 이유 · 시점 |
|---|---|
| **추이 차트** | 설치 OSS 는 이력이 없고(통째 교체) 앱은 `main` 만 있다. 일별 스냅샷(`security_posture_daily`)을 쌓은 뒤 붙인다 — 빈 추이 차트는 만들지 않는다(Test/Coverage 설계 §4 와 같은 원칙) |
| **앱 쪽 알림** | #63 채널에 이벤트(`security_secret_detected` · `security_gate_blocked`)를 더하면 된다. 시크릿은 대시보드를 열기 전에 알려야 하는 종류라 다음 순서로 권한다. 설치 OSS 쪽은 `stack_cve_found`(#354)로 이미 알린다 |
| **스캔 정책 편집 화면** | `GET·PUT /stacks/:stackId/image-scan-policy` 는 있고 화면이 없다. **보안 하위 그룹의 두 번째 항목** 후보 |
| **브랜치 · 커밋** | 동기화가 `main` 만 읽고 SHA 를 저장하지 않는다. 브랜치를 넓히면 "현재 상태" 의 기준은 배포 브랜치로 둔다 |
| **검증된 업그레이드 연결** | 업그레이드 후보의 `fixes_found`(#357)를 설치 OSS 행에 "업그레이드로 N 건 해소" 로 잇는 것은 후속 |
| **Harbor 레지스트리 스캔 · 조직 공통 allowlist · 라이선스 검사** | 각 설계(#76 §11 · #79 §6)의 후속 그대로 |
| **DAST** | #64 의 명시적 보류 그대로 |

---

## 11. 결정 사항 (2026-10-20 확정)

1. **개발자에게 설치 OSS 숫자를 보이지 않는다** — 현재 `/stacks` 권한과 같게. 요약 숫자만 보이자는 요구가 생기면 두 번째 API 에 요약 전용 경로를 더한다.
2. **M2 에 경고(warn)도 센다** — 정책이 경고여도 유출은 이미 일어났다.
3. **보안 하위 그룹은 대시보드 하나로 시작한다** — 다음 후보는 스캔 정책 편집(§10).

---

## 참고 (코드 위치)

- 결과 저장: `db/migrations/000078` · `000082`(image_scan_results) · `000090`(sast_results) · `000081` · `000083` · `000103` · `000104`(stack_image_scans · vulnerabilities · 노출 · 공용) · `000080` · `000088` · `000095`(image_scan_policies)
- 동기화: `internal/cicd/usecase/sync_pipeline_runs.go`(`Execute` · `SyncAll`), `sync_pipeline_runs_sast.go`
- 판정 어휘: `internal/cicd/domain/image_scan.go`(GateResult) · `image_scan_gate.go` · `internal/cicd/port/ci_stage.go`(StageKey) · `internal/shared/domain/image_scan_report.go`
- 시크릿 · 의존성 단계: `internal/cicd/adapter/scaffold/secret_scan.go` · `dependency_scan.go`, 산출물 이름 `internal/cicd/port/ci_server.go`
- 설치 OSS 스캔: `internal/stack/usecase/scan_stack_images.go` · `internal/stack/port/image_scan.go` · `internal/stack/domain/image_scan.go`
- 관측 bridge 선례: `internal/observability/adapter/bridge/monitored_stacks.go`
- 화면: `web/src/components/layout/nav-model.tsx` · `features/observability/pages/alert-history-page.tsx` · `features/cicd/components/image-scan-summary.tsx` · `sast-result-summary.tsx` · `features/stack/components/stack-image-scans-tab.tsx`
- 관련 설계: `Nullus_컨테이너_이미지_스캔_설계.md`(§8 · §12.4) · `Nullus_의존성_취약점_스캔_설계.md`(§6) · `Nullus_시크릿_탐지_설계.md`(§5 · §7) · `Nullus_설치_OSS_CVE_대응_정책.md`(§3) · `Nullus_Test_Coverage_Dashboard_설계.md` · `Nullus_API_권한_설계.md`
