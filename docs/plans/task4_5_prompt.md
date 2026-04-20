# F8 Task 4 + Task 5 — 프론트엔드 어드민 뷰 & Install Wizard Auto Select

> Claude CLI 에 그대로 붙여넣을 수 있는 단일 프롬프트입니다.
> 사용 시 `cd /path/to/cloud-nullus/draft` 이동 후 `claude` 실행, 프롬프트 전체 복사.

---

## 컨텍스트 (먼저 읽어주세요)

Nullus Platform F8 "DevSecOps Stack OSS 버전 호환성 관리" 백로그의 **Task 4 와 Task 5 를 한 브랜치에서 묶어** 처리합니다. 두 Task 모두 프론트엔드 작업이고 공유하는 API 타입 / 훅이 많아서 쪼개면 중복 작업이 발생합니다.

배경 문서 (작업 시작 전 반드시 확인):

- `docs/plans/compatibility_matrix_plan.md` — F8 계획 문서. Task 1, 2, 3 완료. Task 4, 5 본 작업 대상.
- `docs/20_아키텍처/Narwhal_호환성_Seed_Sources.md` — Golden Path 3종 canonical 버전.
- `CHANGELOG.md` `## [Unreleased]` 의 Task 1/2/3 항목 — 사용할 API 와 error code 참고.
- `internal/stack/adapter/handler/compatibility_handler.go` — `POST /stacks/:stackId/validate` 의 **신규** 요청 body (`cluster_id`, `node_architectures`) 및 응답 (`node_architectures`, `overall`, `issues`, `checkedAt`). 핸들러 주석을 꼭 읽을 것.
- `internal/stack/usecase/validate_compatibility.go` `applyArchCheck` — verdict 정책 (Verified+arch miss=fail, Untested+arch miss=warn, nil/empty=CLUSTER_ARCH_UNKNOWN warn).
- `internal/admin/adapter/handler/cluster_handler.go` — Task 3 가 추가한 `POST /admin/clusters/:id/refresh-discovery` 및 응답 DTO 의 `node_architectures` 필드.

선행 결과 중 프론트가 소비할 핵심:

- `GET /api/v1/stacks/compatibility` — `CompatibilityMatrix[]` 반환. 각 tool 에 `ArchSupport` / `MinK8sVersion` / `Tier` 포함 (Task 1).
- `POST /api/v1/stacks/:stackId/validate` body: `{ tools?, cluster_id?, node_architectures? }`. 응답에 `overall.state` (`pass|warn|fail`), `issues[]` (`{tool, message, severity, code}`), `node_architectures[]`, `checkedAt`.
- `GET /api/v1/admin/clusters` / `/admin/clusters/:id` 응답에 `node_architectures: string[]`.
- `POST /api/v1/admin/clusters/:id/refresh-discovery` — kubeconfig 재판독 (성공 시 갱신된 Cluster JSON, 실패 시 `KUBECONFIG_NOT_REGISTERED` 등 AppError).
- Error codes 매핑: `TOOL_ARCH_UNSUPPORTED`, `CLUSTER_ARCH_UNKNOWN`, `KUBECONFIG_NOT_REGISTERED`.

---

## 1. Task 4 — 어드민 뷰 "스택 버전 관리" 페이지

### 1.1 라우트 & 권한

- 라우트: `/admin/stack-versions`. Role `admin` 만 접근 가능 (routes.tsx 의 admin 그룹에 추가).
- 파일: `web/src/features/admin/pages/stack-versions-page.tsx` (신규). 기존 `cluster-page.tsx` / `user-management-page.tsx` 의 `ListDetailPanel` 레이아웃 / `Breadcrumb` / `STATUS_CONFIG` 패턴을 그대로 재사용.
- 사이드바 네비게이션: `AppLayout` (`web/src/app/layout.tsx`) 또는 nav 관련 설정 파일에 `데브섹옵스 스택 > 스택 버전 관리` 항목 추가. Role gate `admin` only.

### 1.2 상세 구성

좌측 목록 / 우측 상세의 `ListDetailPanel` 구조. 목록은 Golden Path 3종 (`gitlab-allinone-v1`, `gitlab-argocd-v1`, `github-argocd-v1`) + 사용자가 추후 추가할 커스텀 매트릭스를 모두 노출.

목록 행에 표시할 정보:

- 매트릭스 이름 (`name`) + ID 모노스페이스 표기
- 상태 배지 (`status`): `verified` / `untested` / `unsupported` — 기존 `STATUS_BADGE` (`stack-version-page.tsx`) 스타일 재사용
- K8s 범위 (`kubernetes.min`-`kubernetes.max`, `recommended` 이 있으면 별표)
- 도구 개수 (`tools.length`)

우측 상세:

- Kubernetes 범위 + recommended (highlight)
- 툴 테이블: category / name / helm version / app version / **arch support** (badge list, `amd64` / `arm64`) / **min k8s** / **tier** (`stable`/`beta`/`deprecated` 배지).
- arch / tier 는 Task 1 에서 추가된 JSONB 필드. 기존 `normalizeCompatibilityMatrix` 가 이들을 propagate 하지 않고 있을 수 있으니 `stack-api.ts` 의 `RawCompatibilityTool` / `normalizeCompatibilityTool` / `CompatibilityMatrix` 타입을 모두 **확장**할 것 (아래 § 3 참고).
- 하단 메타: `updated_at` 표시. (매트릭스 응답에 포함되지 않으면 이 작업 범위에선 생략 가능 — 후속 이슈로 메모 남기기.)

### 1.3 Cluster 섹션 통합

관리자가 한 페이지에서 "클러스터 arch 상태 ↔ 매트릭스 호환성" 을 교차 확인할 수 있도록, 상세 패널 하단에 클러스터 테이블 영역 추가:

- `useClusters()` (admin-api) 로 조직의 cluster 목록 로드.
- 각 cluster 의 `node_architectures` 를 뱃지로 표시. 빈 슬라이스면 `-` + "refresh discovery" 안내.
- 행별 **"재판독"** 버튼 → `POST /admin/clusters/:id/refresh-discovery` 호출 (`useRefreshDiscovery` 훅 신설). 성공 시 React-Query invalidation.
- 각 클러스터 × 현재 선택된 매트릭스의 교차 평가 열: 매트릭스의 모든 tool 이 해당 클러스터 arch 를 지원하면 ✓, 하나라도 미지원이면 ✗ (tooltip 으로 "X 이 arm64 를 지원하지 않습니다").

### 1.4 국제화

`web/src/i18n/ko.json` / `en.json` 에 네임스페이스 `stackVersionsAdmin.*` 신설. 최소 키:

- `title`, `subtitle`, `breadcrumb.*`, `status.verified|untested|unsupported`, `tier.stable|beta|deprecated`, `archBadge.amd64|arm64`, `archBadge.unknown`, `refreshDiscovery.button|success|failure|notRegistered`, `crossEval.compatible|incompatible|unknown`.

기존 `stackVersionPage.*` 네임스페이스와 혼동하지 말 것 (그건 사용자 뷰, 이것은 어드민 뷰).

### 1.5 테스트

- `stack-versions-page.test.tsx` (신규).
- 케이스: (a) 목록 렌더 — 3개 매트릭스 로드, (b) verified 배지 / untested 배지 구분, (c) tier 배지 렌더 (`stable`/`beta` 모두 노출되는 매트릭스 존재), (d) Refresh 버튼 클릭 시 `/refresh-discovery` 호출되는지 (MSW handler), (e) 교차 평가 ✗ 케이스 — `ArchSupport=["amd64"]` 도구 + `node_architectures=["amd64","arm64"]` 클러스터 매트릭스.
- MSW: 필요하면 `web/src/__tests__/msw/` 또는 기존 test-setup 규칙에 따라 추가.

---

## 2. Task 5 — Install Wizard Auto Select UI & Gate 확장

### 2.1 대상 파일

- `web/src/features/stack/pages/stack-install-page.tsx` — 기존 Pre-Deploy Gate 로직 (`compatibilityGate` useMemo, line ~2100 부근) 확장.
- `web/src/features/stack/api/stack-api.ts` — `validateCompatibility` 호출 / 타입 확장 / 신규 Auto Select 관련 훅.

### 2.2 Auto Select 버튼

위저드 Step 2 (도구 선택) 상단 또는 우측 사이드바 영역에 **"Golden Path Quick Start"** 카드 섹션 추가. Golden Path 3종을 각각 버튼으로 렌더:

- 버튼 레이블: 매트릭스 `name` + 상태 배지.
- 클릭 시: 해당 매트릭스의 `tools[]` 를 폼 상태 (`draft.tools`) 에 일괄 주입 + `selectedTemplateId` 를 매트릭스 ID 로 세팅. Wizard 가 자동으로 Step 3 로 점프 (기존 "Quick Start — Select a Template" 동작 참고).
- **회색 처리 규칙**: `selectedCluster` (useWatch) 의 `node_architectures` 를 확인해, 매트릭스 내 어떤 tool 이든 해당 클러스터 arch 를 지원하지 못하면 해당 버튼을 `disabled` + 부제 "클러스터 arm64 노드와 비호환" 표시. 클러스터 미선택이면 경고색 (yellow) + "클러스터 선택 후 확인 가능".
- 판별 로직은 Task 1 에서 추가된 `tool.arch_support` 를 그대로 활용. React 쪽에 `isMatrixCompatibleWithCluster(matrix, cluster)` 순수 함수 하나 추출해 단위 테스트 가능하게 구성.

### 2.3 Gate 호출 확장

기존 `PreDeployCompatibilityGate` 는 현재 `useCompatibilityMatrix()` 로 매트릭스를 가져와 **클라이언트 사이드** 에서 매칭만 하고 있음. 다음과 같이 변경:

- 유저가 form 상에서 `cluster_id` 를 선택하면 백엔드 `POST /stacks/:stackId/validate` (또는 draft 생성 전이면 별도 endpoint — 확인 후 경로 결정) 에 `cluster_id` 또는 `node_architectures` 를 포함해 호출.
- 응답의 `overall.state` 로 최종 verdict 설정 (클라이언트 사이드 추정은 fallback 으로만 유지).
- `issues[]` 를 조회해 `code === 'TOOL_ARCH_UNSUPPORTED'` 이면 "arm64 노드가 Harbor 를 지원하지 않습니다" 유형 메시지 (i18n key) 노출. `code === 'CLUSTER_ARCH_UNKNOWN'` 이면 "클러스터 아키텍처 미상 — Refresh Discovery 가 필요합니다" 경고 + 해당 클러스터 admin 페이지로 가는 링크 (`/admin/stack-versions?clusterId=xxx` 또는 `/admin/clusters?highlight=xxx`).
- `fail` 일 때는 기존처럼 하드 블록. `warn` + ack 미체크일 때 Next 버튼 disabled.

### 2.4 국제화

`stackInstallPage.compatibility.*` 하위에 추가:

- `auto_select.title`, `auto_select.subtitle`, `auto_select.selectCluster`, `auto_select.clusterArchMismatch`
- `issue.toolArchUnsupported`, `issue.clusterArchUnknown`, `issue.kubeconfigNotRegistered`
- `refreshDiscoveryCta`

### 2.5 테스트

- `stack-install-page.test.tsx` 확장.
- 케이스: (a) Auto Select 버튼 3개 렌더, (b) arm64 클러스터 선택 시 GitLab All-in-One 버튼 disabled (툴팁 포함), (c) 클릭 시 draft.tools 갱신 + Step 3 점프, (d) gate 응답 `code=TOOL_ARCH_UNSUPPORTED` 수신 시 fail UI, (e) `code=CLUSTER_ARCH_UNKNOWN` 수신 시 warn + Refresh 링크, (f) `code=KUBECONFIG_NOT_REGISTERED` 수신 시 별도 메시지.
- `isMatrixCompatibleWithCluster` 순수 함수 단위 테스트를 별도 파일 (`stack-install-compatibility.test.ts`) 로 분리.

---

## 3. 공유 변경 — stack-api.ts / admin-api.ts

Task 4 와 Task 5 가 공유하는 타입 확장. **한 PR 안에서 먼저 정리한 뒤** 각 페이지 구현에 사용할 것.

### 3.1 `web/src/features/stack/api/stack-api.ts`

- `RawCompatibilityTool` / `normalizeCompatibilityTool` / `CompatibilityMatrix['tools'][number]` 에 필드 추가:
  - `archSupport: string[]` (from `arch_support` / `ArchSupport`)
  - `minK8sVersion: string` (from `min_k8s_version` / `MinK8sVersion`)
  - `tier: 'stable' | 'beta' | 'deprecated'` (from `tier` / `Tier`, 기본 `'stable'`)
- `RawCompatibilityIssue` 에 `tool`, `code`, `severity` 타입을 `string` 그대로 유지하되 TS enum 이 필요하면 union 정의.
- `RawCompatibilityValidationResult` 에 `node_architectures?: string[]`, `matrix?: RawCompatibilityMatrix`, `message?: string` 추가.
- `normalizeCompatibilityValidationResult` 에 위 필드 매핑 추가.
- `validateCompatibility` 함수 시그니처 확장: `(stackId: string, input?: { clusterId?: string; nodeArchitectures?: string[] })`. 호출부에서 body 에 JSON 으로 전송. React-Query mutation 훅으로 노출 (`useValidateCompatibility`).

### 3.2 `web/src/features/admin/api/admin-api.ts`

- `Cluster` 타입에 `node_architectures: string[]` 추가 + raw 정규화 (`node_architectures`, `NodeArchitectures`, camel/snake 모두 수용).
- `refreshDiscovery(id: string)` 함수 + `useRefreshDiscovery` mutation 훅 추가. 성공 시 `useClusters()` / `useCluster(id)` invalidation.
- 에러 응답의 `code` 가 `KUBECONFIG_NOT_REGISTERED` 인 경우를 `AppError` 로 변환해 호출부에서 분기 가능하도록.

---

## 4. 스타일 / UX 원칙

1. **기존 디자인 토큰 재사용**: `var(--color-text-secondary)`, `STATUS_BADGE`, `Button`, `Modal`, `ListDetailPanel`, `Breadcrumb` 등 기존 컴포넌트를 우선. 신규 컴포넌트는 필요 시 `web/src/components/shared/` 에만 추가.
2. **접근성**: 모든 배지 / 버튼에 `aria-label` / tooltip text 추가. Auto Select disabled 버튼은 `aria-disabled` + `title` 이유 노출.
3. **i18n**: 하드코딩 금지. 한국어 기본, 영어 fallback. `defaultValue` 로 영어 기본값 같이 제공 (`t('key', 'English default')` 패턴).
4. **로딩 / 에러**: React-Query 기본 `isLoading` / `isError` 분기를 표기. `useRefreshDiscovery` 는 mutation 이므로 버튼 내부 스피너 + `toast` 사용 (프로젝트에 이미 toast 시스템 있으면 그것 사용).
5. **라우터 네비게이션**: CLUSTER_ARCH_UNKNOWN 안내 링크는 `react-router` `<Link>` 사용. 쿼리 파라미터 파싱은 기존 `useSearchParams` 패턴 따를 것.

---

## 5. 제약사항 (중요)

1. **백엔드 수정 금지**: 이 Task 범위는 오직 프론트엔드 + 공유 타입. 백엔드 handler / usecase 를 손봐야 한다면 작업을 중단하고 TODO 로 남긴 후 사용자에게 보고.
2. **Golden Path ID 하드코딩 금지**: `'gitlab-allinone-v1'` 같은 ID 를 어디에도 직접 하드코딩하지 말 것. 매트릭스 목록을 서버에서 받고, `status === 'verified'` + `id` 접두 규칙으로 판정하거나 별도 `recommended: true` 플래그가 들어오면 그것을 사용.
3. **모듈 경계**: `stack` feature 가 `admin` feature 의 내부 컴포넌트를 직접 import 하지 않는다. 공유가 필요하면 `web/src/components/shared/` 로 승격. `admin-api` 의 `useClusters` / `useRefreshDiscovery` 는 stack 쪽에서도 import 가능하지만, cluster 도메인 type 은 `web/src/features/admin/api/admin-api.ts` 에서만 정의.
4. **결정론적 렌더**: 매트릭스 / 클러스터 배열은 서버 응답 순서에 의존하지 말고 `id` 기준 정렬 후 렌더. `node_architectures` 도 정렬된 값으로 가정 (Task 3 에서 보장됨).
5. **CHANGELOG / plan 업데이트**:
   - `CHANGELOG.md` `## [Unreleased] > ### Added` 맨 위에 Task 4 항목 + 그 아래 Task 5 항목 (두 개 분리 entry).
   - `docs/plans/compatibility_matrix_plan.md` Task 4 / Task 5 체크박스 `[x]` + 하위 불릿으로 구현 요약 (Task 3 스타일).
6. **커밋 금지**: 사용자가 명시적으로 요청하기 전에는 `git commit` / `git push` 금지. 작업 완료 후 리뷰만.

---

## 6. 산출물 체크리스트

- [ ] `web/src/features/admin/pages/stack-versions-page.tsx` 신규 + test
- [ ] `web/src/app/routes.tsx` 라우트 추가 (admin only)
- [ ] 사이드바 네비게이션 항목 추가
- [ ] `web/src/features/admin/api/admin-api.ts` — `Cluster.node_architectures` + `useRefreshDiscovery`
- [ ] `web/src/features/stack/api/stack-api.ts` — tool `archSupport` / `minK8sVersion` / `tier`, `validateCompatibility` 확장, 응답 타입 확장
- [ ] `web/src/features/stack/pages/stack-install-page.tsx` — Auto Select 섹션 + gate 호출 확장 + error code 분기
- [ ] `isMatrixCompatibleWithCluster` 순수 함수 + 단위 테스트 (`stack-install-compatibility.test.ts` 또는 utils)
- [ ] `web/src/i18n/ko.json` / `en.json` — 네임스페이스 `stackVersionsAdmin.*`, `stackInstallPage.compatibility.auto_select.*`, `stackInstallPage.compatibility.issue.*` 추가
- [ ] MSW handler 업데이트 (있다면) — `/admin/clusters/:id/refresh-discovery`, `/stacks/:stackId/validate` 신규 응답 shape
- [ ] 테스트 실행: `pnpm -C web test` 전 패키지 통과. `pnpm -C web lint` 통과. (레포 실제 npm/pnpm 확인: `cat web/package.json | grep scripts -A20`.)
- [ ] `CHANGELOG.md`, `docs/plans/compatibility_matrix_plan.md` 업데이트

---

## 7. 작업 순서 제안

1. `stack-api.ts` + `admin-api.ts` 타입/훅 먼저 정리 (Red: 관련 테스트만 먼저 실패하게 작성).
2. `isMatrixCompatibleWithCluster` 순수 함수 + 단위 테스트 (Green).
3. Task 4 어드민 페이지 + 라우트 + 네비 + i18n.
4. Task 4 페이지 테스트.
5. Task 5 Install Wizard 에 Auto Select 섹션 붙이기.
6. Task 5 Gate 확장 (서버 호출 포함).
7. Task 5 페이지 테스트.
8. i18n 키 중복/누락 점검.
9. `pnpm -C web test` / `lint` 수행.
10. CHANGELOG + plan 문서 갱신.

---

## 8. 하지 말아야 할 것

- `ToolVersion` / `CompatibilityMatrix` 서버 스키마 수정 (Task 1 확정분).
- Golden Path 매트릭스 3종의 ID 하드코딩 (위 § 5.2).
- 기존 `stack-version-page.tsx` (사용자 뷰) 로직 변경 — 어드민 뷰는 별개로 신설.
- 매트릭스 "생성 / 수정 / 삭제" (CRUD) 기능 구현 — 계획 문서 Task 4 의 scope 은 목록/상세 그리드까지. CRUD 는 follow-up.
- 사용자 acknowledgment 체크박스 UX 대규모 변경 — 기존 동의 체크 플로우 유지하고 arch 메시지만 추가.
- 커밋/푸시 (사용자 승인 없이).

---

## 9. 완료 시 보고 형식 (한국어, 간결)

1. 변경된 파일 목록 (카테고리별: routing / pages / api / i18n / test / docs).
2. 새 테스트와 의도 (1~2 문장씩).
3. `pnpm test` / `pnpm lint` 결과 요약 (실패 0 확인).
4. 스크린샷 대신 캡처 가능한 주요 UI 변화를 말로 서술 (Auto Select 버튼 위치, 어드민 페이지 상/하 구성 등).
5. 후속 이슈 제안 (매트릭스 CRUD, `updated_at` 표시, 클러스터 nightly refresh 스케줄러 등).
