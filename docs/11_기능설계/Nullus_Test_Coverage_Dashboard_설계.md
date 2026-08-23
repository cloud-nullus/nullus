# Test / Coverage Dashboard 설계 (nullus-plan#46)

> EPIC: [cloud-nullus/nullus-plan#46](https://github.com/cloud-nullus/nullus-plan/issues/46) — Test/Coverage Dashboard 설계.
> 본 문서는 설계 카드다 — 산출물은 이 문서뿐이고, 구현은 별도 작업으로 분리한다.

작성일: 2026-08-16

---

## 요약 (비관계자용 — 이것만 읽어도 논의 가능)

**무엇을 설계하나.**
Nullus로 배포하는 애플리케이션의 **테스트 결과(성공/실패)와 커버리지(테스트가 코드를 얼마나 덮는지 %)** 를 Nullus 화면에서 보여주는 기능의 설계다.

**핵심 발견 — 지금은 보여줄 데이터 자체가 없다.**
조사 결과, Nullus가 만들어 주는 파이프라인은 현재 **Build(빌드) → Deploy(배포) 2단계뿐이고, 테스트를 실행하는 단계가 없다.** 과거에는 화면이 Test 단계를 "성공"으로 표시했지만 실제로는 돌지 않는 가짜였고, 팀은 "실행되지 않은 일을 성공이라 말하면 안 된다"며 정직하게 2단계로 정렬했다(코드 주석에 기록된 교훈). 즉 이 EPIC은 "대시보드를 그리는 일"이기 전에 **"테스트를 돌리고 그 결과를 Nullus로 가져오는 길"부터 설계**해야 한다.

**설계 결론 — 3단계 로드맵.**
1. **테스트 단계 만들기**: 파이프라인에 테스트 실행 단계를 선택 옵션으로 추가한다(모든 앱이 테스트를 갖고 있진 않으므로 강제하지 않는다).
2. **결과 가져오기**: Nullus에는 이미 "CI 서버의 빌드 이력을 주기적으로 **당겨와(pull)** 실행 기록으로 들이는" 동기화가 있다. 테스트 결과·커버리지도 **그 동기화를 확장해 같은 길로 가져온다** — 새 수신 창구를 만들지 않는다.
3. **보여주기**: 새 화면을 만들지 않고 **기존 CI/CD 화면(History·모니터링)에 열/타일을 추가**하는 것부터 시작한다. 전용 대시보드는 데이터가 쌓인 뒤 2차로.

**왜 새 화면부터 만들지 않는가.**
데이터가 쌓이기 전의 전용 대시보드는 빈 화면이 된다. 기존 화면 확장이 먼저고, 트렌드 차트가 의미를 가질 만큼 데이터가 모이면 전용 페이지를 검토한다.

**이 설계로 당장 바뀌는 것.** 없다 — 구현은 별도 카드로 분리한다.

---

## 상세 (근거와 코드 위치)

### 1. As-Is

| 영역 | 현황 | 근거 |
|---|---|---|
| 파이프라인 단계 | **Build·Deploy 2단계뿐, Test 없음** | `internal/cicd/adapter/scaffold/renderer.go:535` `PipelineStageNames() → ["Build","Deploy"]`, gitlab 렌더러 `stages: build/deploy`(`renderer.go:257`) |
| 과거 교훈 | 템플릿이 Build/Test/ImageBuild/Deploy 4단계를 선언했으나 실제는 2단계 — **돌지 않은 Test를 성공으로 표시** → 000070에서 정렬 | `db/migrations/000070_align_pipeline_template_stages.up.sql` 주석 |
| 결과 저장 틀 | 배포 단계별 결과는 이미 저장·표시됨 — 다만 **로그 문자열 중심, 수치 지표 없음** | `deployments.steps JSONB`(000071), `DeployStep{Name,Status,Kind,Message,Logs}`(`internal/cicd/domain/pipeline.go:67`) |
| **실행 기록 동기화 (pull)** | **이미 존재** — CI 서버 빌드 이력을 당겨와 배포 기록으로 들인다. 빌드 1건 = `dep_ci_<pipeline>_<build#>` ID 의 Deployment 로 멱등 편입 → **테스트 결과를 걸 실행 축이 이미 있다** | `internal/cicd/usecase/sync_pipeline_runs.go`, `port.CIBuildReader.ListBuilds`(`port/ci_server.go:87`) — 현재 구현은 Jenkins(`adapter/jenkins/client.go`) |
| 인바운드 채널 | **없음** — 존재하는 webhook 은 Gitea→Jenkins 빌드 트리거용(`/gitea-webhook/post`)이고 Nullus 가 받는 수신 창구는 0 | `usecase/provision_app_project.go:369` |
| 자체 선행 자산 | Nullus 저장소 **자체 개발용** 커버리지 게이트는 이미 있음(제품 기능 아님, 참고 모델) | `scripts/check-coverage.sh`(go cover, 60% 게이트), `web/vite.config.ts` vitest thresholds, `ci.yml` |
| UI 자산 | CI/CD 목록·이력 페이지, 모니터링의 CI/CD 뷰, 차트 위젯, 상태 아이콘 체계 | `cicd-history-page.tsx`, `monitoring-cicd-view.tsx`, `monitoring-chart-widgets.tsx`, `status-icon.tsx` |

### 2. 표시할 테스트 결과 항목 (세부 태스크 ①)

**최소 항목** — 도구 불문 표준 포맷에서 나오는 것만 1차로 담는다:

| 항목 | 출처 포맷 | 비고 |
|---|---|---|
| 테스트 총계 (전체/성공/실패/스킵) | JUnit XML | 사실상 모든 테스트 러너가 지원 |
| 실패 테스트 목록 (이름 + 메시지) | JUnit XML | 실패 원인 진입점 |
| 실행 시간 | JUnit XML | |
| 커버리지 % (라인 기준 1개 숫자) | Cobertura XML 또는 lcov | 언어 불문 공통분모 |
| (2차) 커버리지 추이·파일별 상세 | 동일 | 데이터 축적 후 |

포맷을 JUnit/Cobertura·lcov로 못박는 이유: **언어·프레임워크마다 러너가 달라도 이 둘은 공통 출력**이라, Nullus가 도구별 파서를 늘리지 않아도 된다.

### 3. 수집 흐름 초안 (세부 태스크 ②)

흐름: **테스트 단계가 표준 리포트 생성 → 기존 빌드 이력 동기화(`SyncPipelineRuns`)가 빌드를 들일 때 테스트 결과·커버리지도 함께 당겨옴 → 파싱·저장**.

반입 방식 두 가지를 비교했다. 처음 초안은 push 를 유력하게 봤으나, **수집 영역을 먼저 검토한 결과 기존 pull 자산이 이미 있어 결론이 뒤집혔다**:

| 방식 | 내용 | 판정 |
|---|---|---|
| **당겨오기 (pull) — 기존 동기화 확장** | `CIBuildReader` 에 테스트 리포트 조회를 추가하고 `SyncPipelineRuns` 가 빌드와 함께 들인다. Jenkins 는 JUnit 결과를 주는 **내장 `/testReport` API** 가 있어 같은 클라이언트(`adapter/jenkins/client.go`)에 메서드 하나를 얹는 확장이다 | ✅ **채택** — 검증된 기존 경로(멱등 동기화·`dep_ci_*` 실행 축·CI 접속 자격)를 그대로 재사용. **새 인바운드 표면이 생기지 않는다** |
| 파이프라인이 밀어넣기 (push) | test job 끝에 Nullus 수집 API 로 리포트 업로드 | ⚪ 보류 — Nullus 에는 현재 인바운드 수신 창구가 0 이라 **API·인증·토큰 배포를 전부 새로 구축**해야 한다. 인바운드를 열게 되면 **#44(Webhook API 범위 정의)와 묶어서** 설계할 사안 — 그때 재검토 |

- 도구별 커버리지 소스: Jenkins 는 Coverage 플러그인 API 또는 아카이브된 아티팩트(cobertura/lcov) 조회. **1차는 내장 API 로 되는 테스트 결과(JUnit)부터** 들이고, 커버리지는 플러그인/아티팩트 접근을 확인 후 잇는다.
- 현재 pull 지원 CI 는 **Jenkins 뿐**이다(GitLab CI·GitHub Actions 는 `CIBuildReader` 구현 자체가 아직 없음). 테스트 수집도 같은 순서를 따른다 — 어댑터가 생길 때 그 어댑터에 얹는다.
- **저장은 새 테이블**(예: `test_runs` — deployment FK, 총계·coverage 수치 컬럼, 원본 리포트 JSONB)로 한다. 실행 축은 기존 동기화가 만드는 `dep_ci_*` Deployment 를 그대로 쓴다. `deployments.steps` JSONB 에 넣지 않는 이유: 그건 실행 로그의 자리고, 이건 **파이프라인 축으로 추이를 조회**해야 하는 구조화 지표라 조회 축이 다르다.

### 4. 기존 페이지 재사용 판단 (세부 태스크 ③)

**결론: 재사용 우선, 전용 페이지는 2차.**

| 단계 | 노출 위치 | 방식 |
|---|---|---|
| 1차 | CI/CD History (`cicd-history-page`) | 행에 테스트/커버리지 열 추가 + 상세 패널에 실패 목록 |
| 1차 | 모니터링 대시보드 CI/CD 뷰 (`monitoring-cicd-view`) | 요약 타일(최근 성공률·평균 커버리지) |
| 2차 | 전용 Test/Coverage 페이지 | 추이 차트(`monitoring-chart-widgets` 재사용) — 데이터 축적 후 |

이유: 데이터가 없는 시점의 전용 대시보드는 빈 화면이다(Security Dashboard #45도 같은 함정 — 그쪽은 SAST 선행 결과에 맞춰 별도 진행 중). 기존 화면의 열·타일 추가가 첫 가치 전달이 가장 빠르다.

### 5. 화면 노출 모델 (세부 태스크 ④)

노출 위계는 데이터의 축을 따른다:

- **배포(1회 실행) 상세** → 그 실행의 테스트 결과·커버리지
- **파이프라인** → 최근 결과 + 추이
- **모니터링 대시보드** → 조직 단위 요약 타일

원칙 하나를 000070의 교훈에서 그대로 가져온다: **"실행되지 않은 것은 보여주지 않는다."** 테스트 단계가 없는 파이프라인은 "테스트 미구성"으로 명시하고, 0건 성공처럼 보이게 하지 않는다.

### 6. 범위와 선행 조건

- 본 카드의 산출물은 **이 설계 문서뿐**이다. 구현(테스트 단계 옵션 → 수집 API·저장 → 화면 확장)은 순서대로 별도 카드로 분리한다.
- 테스트 단계 추가는 스캐폴드 수정(cicd 모듈)이라 CI/CD 안정화(F5/F6) 흐름과 조율이 필요하다.
- #45(Security Dashboard)와 수집·노출 구조를 공유할 수 있다 — 그쪽 SAST 선행(타 팀원 진행 중) 결과와 정합 확인 후 통합 여부를 판단한다.

## 참고 (코드 위치)

- 파이프라인 단계: `internal/cicd/adapter/scaffold/renderer.go:535`(PipelineStageNames), `:257`(gitlab stages)
- 교훈 기록: `db/migrations/000070_align_pipeline_template_stages.up.sql`
- 결과 저장 틀: `db/migrations/000071_deployment_steps.up.sql`, `internal/cicd/domain/pipeline.go:67`(DeployStep)
- 실행 기록 동기화(pull, 확장 대상): `internal/cicd/usecase/sync_pipeline_runs.go`, `internal/cicd/port/ci_server.go:87`(CIBuildReader·CIBuild), `internal/cicd/adapter/jenkins/client.go`
- 인바운드 부재 근거: `internal/cicd/usecase/provision_app_project.go:369`(webhook 은 Gitea→Jenkins 트리거용)
- 자체 커버리지 게이트(참고 모델): `scripts/check-coverage.sh`, `web/vite.config.ts`(coverage.thresholds), `.github/workflows/ci.yml`
- UI 재사용 후보: `web/src/features/cicd/pages/cicd-history-page.tsx`, `web/src/features/observability/components/monitoring-cicd-view.tsx`·`monitoring-chart-widgets.tsx`
