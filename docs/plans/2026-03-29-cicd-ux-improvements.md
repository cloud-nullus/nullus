# CI/CD UX 개선 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** CI/CD List/Deploy 워크플로우를 Stack과 동일한 수준으로 개선 — 클러스터 필터, 실시간 로그, 매니페스트 편집, WebSocket 배포 진행

**Architecture:** CI/CD List에 클러스터 필터링 추가, Pipeline Logs 전용 페이지 신규 생성(Stack Logs 패턴), Deploy 위저드 6단계로 재구성(템플릿 제거, Stack Git URL 연동, 실제 네임스페이스, 리소스 Input, 매니페스트 편집), 배포 진행 UI를 WebSocket 기반 Stack Deploy 페이지 스타일로 전환

**Tech Stack:** React 19, TypeScript, TanStack Query, react-hook-form, Zod, WebSocket (gorilla/websocket), Go Echo v4, K8s client-go

---

## Group A: CI/CD List 개선

### Task 1: CI/CD List — 클러스터 필터 드롭다운

**Files:**
- Modify: `web/src/features/cicd/pages/cicd-list-page.tsx`
- Modify: `web/src/features/cicd/api/cicd-api.ts`

**What:**
1. `cicd-list-page.tsx`에 `clusterFilter` state 추가
2. `useClusters()` 훅 import (`features/admin/api/admin-api.ts`에 이미 존재)
3. 툴바에 NativeSelect 클러스터 드롭다운 추가 (status 필터 옆)
4. client-side 필터에 `matchesCluster` 조건 추가

**Pattern (Stack List 참조):**
```tsx
// cicd-list-page.tsx 상단
import { useClusters } from '../../admin/api/admin-api'

// state
const [clusterFilter, setClusterFilter] = useState('')
const { data: clustersData } = useClusters()
const clusterOptions = (clustersData?.items ?? []).map((c) => ({ id: c.id, name: c.name }))

// 필터
const matchesCluster = !clusterFilter || p.clusterId === clusterFilter

// toolbar에 NativeSelect 추가
<NativeSelect value={clusterFilter} onChange={(e) => setClusterFilter(e.target.value)}>
  <option value="">All Clusters</option>
  {clusterOptions.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
</NativeSelect>
```

**Verify:** `npm run build` 통과, 브라우저에서 클러스터 필터 작동 확인

---

### Task 2: CI/CD Pipeline Logs 페이지 생성

**Files:**
- Create: `web/src/features/cicd/pages/cicd-pipeline-logs-page.tsx`
- Modify: `web/src/app/routes.tsx` — 라우트 추가
- Modify: `web/src/features/cicd/pages/cicd-list-page.tsx` — Logs 버튼 경로 변경

**What:**
Stack의 `stack-deployment-logs-page.tsx` 패턴을 따르는 CI/CD 전용 로그 페이지 생성.
- 파이프라인 이름 + 메타데이터 헤더
- 해당 파이프라인의 최근 배포 이력 (usePipelineDeployments)
- 각 배포의 로그를 터미널 콘솔 스타일로 표시 (DeployStep.Logs 데이터 활용)
- 단계 타임라인 (Namespace → Deployment → Service)

**라우트:** `cicd/pipelines/:id/logs` → `CicdPipelineLogsPage`

**Logs 버튼 변경:**
```tsx
// cicd-list-page.tsx — 기존
onOpenLogs={() => navigate(`/cicd/history?pipeline=${pipeline.id}`)}
// 변경
onOpenLogs={() => navigate(`/cicd/pipelines/${pipeline.id}/logs`)}
```

**Page 구조:**
```
CicdPipelineLogsPage
├── Breadcrumb: [{ label: 'CI/CD List', path: '/cicd/list' }, { label: '<pipelineName> Logs' }]
├── Header (파이프라인 이름, 클러스터, 네임스페이스, 상태)
├── 최근 배포 선택 (드롭다운 또는 목록)
└── 터미널 콘솔 (선택된 배포의 steps[].logs 표시)
    ├── 단계 타임라인 (각 step의 status → ✓/✗/○)
    └── 로그 라인 (kubectl apply 명령어 + 결과, 색상 구분)
```

**Verify:** `npm run build` 통과, `/cicd/pipelines/<id>/logs` 접근 가능

---

### Task 3: CI/CD List — RUN 버튼 + Breadcrumb 개선

**Files:**
- Modify: `web/src/features/cicd/pages/cicd-list-page.tsx` — RUN 버튼 경로에 pipeline 정보 전달
- Modify: `web/src/features/cicd/pages/developer-deploy-page.tsx` — URL 파라미터 수신, Breadcrumb 동적화
- Modify: `web/src/features/cicd/pages/cicd-history-page.tsx` — Breadcrumb 동적화

**What:**
1. RUN 버튼: pipeline 정보(clusterId, namespace, name)를 query param으로 전달
2. developer-deploy-page: URL params 수신 시 해당 필드 프리필 + Breadcrumb 업데이트
3. 모든 CI/CD 페이지에 상위 네비게이션 Breadcrumb 추가

**RUN 버튼:**
```tsx
onRun={() => navigate(`/cicd/developer-deploy?pipelineId=${pipeline.id}&cluster=${pipeline.clusterId}&namespace=${pipeline.namespace}&name=${pipeline.name}`)}
```

**Breadcrumb 패턴:**
```tsx
// developer-deploy-page.tsx
<Breadcrumb items={[
  { label: 'CI/CD List', path: '/cicd/list' },
  { label: 'Pipeline Setup & Deploy' },
]} />

// cicd-history-page.tsx
<Breadcrumb items={[
  { label: 'CI/CD List', path: '/cicd/list' },
  { label: 'Deployment History' },
]} />

// cicd-pipeline-logs-page.tsx (Task 2에서 이미 추가)
<Breadcrumb items={[
  { label: 'CI/CD List', path: '/cicd/list' },
  { label: `${pipelineName} Logs` },
]} />
```

**Verify:** RUN 버튼 클릭 시 위저드에 정보 프리필 확인, 뒤로가기 Breadcrumb 작동

---

## Group B: Pipeline Setup & Deploy 위저드 개선

### Task 4: 앱 템플릿 그리드 제거 + CI/CD Template app_type 사용

**Files:**
- Modify: `web/src/features/cicd/pages/developer-deploy-page.tsx`

**What:**
1. 앱 템플릿 그리드 섹션 전체 제거 (TEMPLATE_GIT_REPOS, useAppTemplates, 템플릿 카드 UI)
2. CI/CD Template 페이지에서 선택한 template의 `app_type`을 query param으로 수신
3. `DEFAULT_FORM.template` 제거, `appType`은 URL param에서 결정
4. 매니페스트 생성(`generateYaml`) 시 app_type 기반으로 이미지/포트 결정

**app_type → 기본값 매핑:**
```tsx
const APP_TYPE_DEFAULTS: Record<string, { image: string; port: number }> = {
  backend: { image: 'nginx:alpine', port: 8080 },
  web: { image: 'nginx:alpine', port: 80 },
  batch: { image: 'busybox:latest', port: 8080 },
}
```

**URL param 수신:**
```tsx
const [searchParams] = useSearchParams()
const appType = searchParams.get('appType') ?? 'backend'
```

**Verify:** 템플릿 그리드가 표시되지 않음, app_type에 따라 매니페스트가 정상 생성

---

### Task 5: Git Repository — Stack Git 서비스 URL 연동

**Files:**
- Modify: `web/src/features/cicd/pages/developer-deploy-page.tsx`
- Modify: `web/src/features/cicd/api/cicd-api.ts` (필요 시)

**What:**
Step 2를 두 부분으로 분리:
1. Stack 선택 드롭다운 (선택 사항) — 설치된 Stack 중 Git 서비스(GitLab, Gitea 등) 포함된 Stack 목록
2. Git URL 입력 — Stack 선택 시 base URL 자동 입력 + repo 이름 입력 필드 분리

**Stack에서 Git URL 추출:**
Stack의 config에서 Git 서비스 도메인을 추출해야 함. 현재 Stack 구조를 확인하여 GitLab URL을 가져오는 방법 결정.

**간소화된 접근:** Stack 목록에서 Git 서비스가 포함된 Stack을 표시하고, 선택 시 `http://<stack-domain>/` 형태로 base URL 제공. repo 이름은 사용자가 입력.

**UI 구조:**
```tsx
<StepSection title="Git Repository">
  <div className="flex flex-col gap-3">
    <div>
      <label>Stack (선택)</label>
      <NativeSelect value={selectedStackId} onChange={...}>
        <option value="">직접 입력</option>
        {stacks.filter(hasGitService).map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
      </NativeSelect>
    </div>
    {selectedStackId ? (
      <div className="flex gap-2">
        <Input value={stackGitBaseUrl} disabled className="flex-1" />
        <Input placeholder="repo-name" value={repoName} onChange={...} className="flex-1" />
      </div>
    ) : (
      <Input placeholder="https://github.com/org/repo" value={form.gitUrl} onChange={...} />
    )}
  </div>
</StepSection>
```

**gitUrl 합성:** `${stackGitBaseUrl}${repoName}` → form.gitUrl에 저장

**Zod 검증 변경:** `gitUrl`을 `.url()` 대신 `.min(1)` 로 완화 (repo 이름만 입력 시 URL 형식이 아닐 수 있음)
→ 또는 Stack base URL + repo name을 합쳐서 항상 full URL 생성

**Verify:** Stack 선택 시 base URL 표시, repo 이름 입력 시 full URL 합성, Stack 미선택 시 직접 입력

---

### Task 6: 네임스페이스 — 실제 K8s API 조회

**Files:**
- Modify: `web/src/features/cicd/pages/developer-deploy-page.tsx`

**What:**
하드코딩된 `['default', 'production', 'staging']` 대신 `useClusterNamespaces(clusterId)` 훅 사용.

**변경:**
```tsx
// 기존 (하드코딩)
const clusters = (clustersData?.items ?? []).map((c) => ({
  id: c.id, name: c.name,
  namespaces: ['default', 'production', 'staging'],  // ❌
}))

// 변경
import { useClusterNamespaces } from '../../admin/api/admin-api'

const { data: namespacesData } = useClusterNamespaces(form.clusterId)
const namespaces = (namespacesData ?? []).map((ns) => ns.name)

// clusters 매핑에서 namespaces 제거
const clusters = (clustersData?.items ?? []).map((c) => ({
  id: c.id, name: c.name,
}))
```

**Step 3 네임스페이스 드롭다운 변경:**
```tsx
<NativeSelect value={form.namespace} onChange={...}>
  {namespaces.map((ns) => <option key={ns} value={ns}>{ns}</option>)}
</NativeSelect>
```

**클러스터 변경 시:** namespace를 첫 번째 값으로 리셋

**Verify:** 클러스터 변경 시 실제 네임스페이스 목록 로드, `npm run build` 통과

---

### Task 7: 리소스 설정 — 슬라이더 + Input 동시 지원

**Files:**
- Modify: `web/src/features/cicd/pages/developer-deploy-page.tsx` (ResourceSlider 컴포넌트)

**What:**
`ResourceSlider` 컴포넌트에 editable Input 추가. 슬라이더와 Input이 양방향 동기화.

**변경된 ResourceSlider:**
```tsx
function ResourceSlider({ label, value, options, onChange }: {
  label: string; value: string; options: string[]; onChange: (v: string) => void
}) {
  const idx = options.indexOf(value)
  const isCustom = idx === -1
  const sliderId = `resource-${label.toLowerCase().replace(/\s+/g, '-')}`
  
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <label htmlFor={sliderId} className={cn(labelStyleClass, 'mb-0')}>{label}</label>
        <Input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-24 text-right font-mono text-[13px]"
        />
      </div>
      <input
        id={sliderId}
        type="range"
        min={0}
        max={options.length - 1}
        value={isCustom ? 0 : idx}
        onChange={(e) => onChange(options[Number(e.target.value)])}
        className="w-full accent-[#6366f1]"
      />
      <div className="mt-1 flex justify-between">
        {options.map((o) => (
          <span key={o} className="font-mono text-[10px] text-[var(--color-text-secondary)]">{o}</span>
        ))}
      </div>
    </div>
  )
}
```

**Replicas도 동일 패턴:** `value={String(form.replicas)}`, Input에 숫자 입력 허용

**Verify:** 슬라이더 조작 → Input 반영, Input 직접 입력 → 슬라이더 반영(가능한 경우), `npm run build` 통과

---

### Task 8: Step 6 — 매니페스트 편집 단계 추가

**Files:**
- Modify: `web/src/features/cicd/pages/developer-deploy-page.tsx`

**What:**
1. `Step` 타입을 `1 | 2 | 3 | 4 | 5 | 6` 으로 변경
2. `STEP_LABELS`에 `6: '매니페스트 확인'` 추가
3. Step 6 컨텐츠: 생성된 YAML을 `<textarea>`로 표시, 수정 가능
4. 수정된 매니페스트는 별도 state에 저장 (`customManifest`)
5. Deploy 시 `customManifest`가 있으면 해당 값 사용, 없으면 자동 생성 YAML 사용
6. 우측 YAML 미리보기는 Step 6에서 숨김 (본문이 이미 에디터이므로)

**Step 6 UI:**
```tsx
{step === 6 && (
  <StepSection title="매니페스트 확인 및 편집">
    <p className="mb-3 text-xs text-[var(--color-text-secondary)]">
      생성된 YAML 매니페스트를 확인하고, 필요 시 수정하세요.
    </p>
    <textarea
      value={customManifest ?? generateYaml(form)}
      onChange={(e) => setCustomManifest(e.target.value)}
      className="h-[400px] w-full rounded-lg border border-[var(--color-border-default)] bg-[#0d1117] p-4 font-mono text-xs text-[#c9d1d9] focus:outline-none focus:ring-1 focus:ring-[#6366f1]"
      spellCheck={false}
    />
    <Button variant="ghost" size="sm" onClick={() => setCustomManifest(null)} className="mt-2">
      기본값으로 초기화
    </Button>
  </StepSection>
)}
```

**canNext 업데이트:** `6: true` (항상 배포 가능)

**Deploy 버튼:** Step 6에서 표시 (기존 Step 5 → Step 6)

**Verify:** 6단계까지 진행 가능, YAML 편집 후 Deploy 시 수정된 매니페스트 사용, `npm run build` 통과

---

## Group C: WebSocket 배포 진행 UI

### Task 9: 백엔드 — CI/CD WebSocket 로그 핸들러

**Files:**
- Create: `internal/cicd/adapter/handler/deploy_ws_handler.go`
- Modify: `internal/cicd/adapter/kube/step_tracker.go` — Subscribe/Unsubscribe 패턴 추가
- Modify: `internal/cicd/adapter/kube/applier.go` — 로그 이벤트 발행
- Modify: `internal/cicd/adapter/handler/pipeline_handler.go` — WS 라우트 등록
- Modify: `cmd/api/main.go` — WS 라우트 등록

**What:**
Stack의 `deploy_handler.go` StreamLogs 패턴을 CI/CD에 적용.
StepTracker에 pub/sub 채널 추가하여 실시간 로그 이벤트 스트리밍.

**StepTracker 확장:**
```go
// step_tracker.go에 추가
type LogEvent struct {
    DeploymentID string
    StepIndex    int
    Level        string // info, success, error
    Message      string
    Progress     int
    Status       string // "", "success", "failed"
    Timestamp    time.Time
}

type StepTracker struct {
    mu          sync.RWMutex
    steps       map[string][]domain.DeployStep
    subscribers map[string][]chan LogEvent  // 추가
}

func (t *StepTracker) Subscribe(deploymentID string) chan LogEvent {
    t.mu.Lock()
    defer t.mu.Unlock()
    ch := make(chan LogEvent, 64)
    t.subscribers[deploymentID] = append(t.subscribers[deploymentID], ch)
    return ch
}

func (t *StepTracker) Unsubscribe(deploymentID string, ch chan LogEvent) {
    t.mu.Lock()
    defer t.mu.Unlock()
    subs := t.subscribers[deploymentID]
    for i, s := range subs {
        if s == ch {
            t.subscribers[deploymentID] = append(subs[:i], subs[i+1:]...)
            close(ch)
            break
        }
    }
}

func (t *StepTracker) publish(deploymentID string, event LogEvent) {
    // mu already locked by caller
    for _, ch := range t.subscribers[deploymentID] {
        select {
        case ch <- event:
        default: // drop if slow consumer
        }
    }
}
```

**AppendLog, MarkSuccess, MarkFailed에서 publish 호출:**
각 메서드 끝에 `t.publish(deploymentID, LogEvent{...})` 추가

**WebSocket 핸들러 (`deploy_ws_handler.go`):**
Stack의 `StreamLogs` 패턴과 동일:
- gorilla/websocket Upgrader
- StepTracker.Subscribe로 채널 수신
- LogEvent → JSON 메시지로 변환 후 WS 전송
- Ping/Pong keepalive

**라우트:** `e.GET("/ws/cicd/deployments/:id/logs", handler.StreamCicdLogs)`

**Verify:** `go build ./...` 통과, `go test ./internal/cicd/...` 통과

---

### Task 10: 프론트엔드 — CI/CD WebSocket 로그 훅

**Files:**
- Create: `web/src/features/cicd/hooks/use-cicd-deploy-log.ts`

**What:**
Stack의 `use-deploy-log.ts` 패턴을 CI/CD에 적용.

```tsx
import { useState, useEffect, useRef } from 'react'
import { connect } from '../../../lib/websocket'

export type CicdLogLevel = 'info' | 'success' | 'error'
export type CicdDeployStatus = 'connecting' | 'running' | 'success' | 'failed'

export interface CicdLogEntry {
  id: string
  timestamp: string
  level: CicdLogLevel
  message: string
}

export function useCicdDeployLog(deploymentId: string | null) {
  const [logs, setLogs] = useState<CicdLogEntry[]>([])
  const [status, setStatus] = useState<CicdDeployStatus>('connecting')
  const [progress, setProgress] = useState(0)
  const [isConnected, setIsConnected] = useState(false)
  const counterRef = useRef(0)

  useEffect(() => {
    if (!deploymentId) return

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const wsUrl = `${protocol}://${window.location.host}/ws/cicd/deployments/${deploymentId}/logs`

    const client = connect(wsUrl, {
      onMessage: (data) => {
        const payload = data as any
        if (payload.progress !== undefined) setProgress(payload.progress)
        if (payload.type === 'log' && payload.message) {
          setLogs((prev) => [...prev, {
            id: String(++counterRef.current),
            timestamp: payload.timestamp ?? new Date().toISOString(),
            level: payload.level ?? 'info',
            message: payload.message,
          }])
        } else if (payload.type === 'status' && payload.status) {
          setStatus(payload.status as CicdDeployStatus)
        }
      },
      onStatusChange: (connected) => {
        setIsConnected(connected)
        if (connected) setStatus('running')
      },
    })

    return () => client.close()
  }, [deploymentId])

  return { logs, status, progress, isConnected }
}
```

**Verify:** `npm run build` 통과

---

### Task 11: 프론트엔드 — 배포 진행 UI를 Stack Deploy 스타일로 전환

**Files:**
- Modify: `web/src/features/cicd/pages/developer-deploy-page.tsx`

**What:**
현재 polling 기반 배포 진행 UI를 WebSocket 기반 Stack Deploy 페이지 스타일로 교체.

**기존 코드 교체 범위:** `if (deploymentId) { ... }` 블록 전체

**새 UI 구조 (stack-deploy-page.tsx 패턴):**
```
배포 진행 화면
├── Breadcrumb: [CI/CD List → Pipeline Setup & Deploy → 배포 진행]
├── Header (앱 이름, deployment ID, 연결 상태)
├── Phase Steps (Namespace 생성 → Deployment 생성 → Service 생성)
│   ├── 완료: 초록 ✓
│   ├── 진행: 파란 spinner
│   └── 대기: 회색 숫자
├── Progress Bar (0-100%, 100 세그먼트)
├── Terminal Console (WebSocket 로그 실시간 표시)
│   ├── Mac-style 타이틀바
│   ├── 로그 라인: [HH:MM:SS] [LEVEL] message
│   └── 자동 스크롤
├── 생성된 리소스 목록 (완료 시)
├── kubectl 확인 명령어 (완료 시)
└── 완료/실패 버튼 (새 배포, CI/CD 목록)
```

**핵심 변경:**
1. `useDeploymentStatus` (polling) → `useCicdDeployLog` (WebSocket) 교체
2. Stack Deploy 페이지의 PhaseStep, ProgressBar 컴포넌트 패턴 복제
3. 터미널 콘솔: LOG_LEVEL_STYLE 색상 체계 적용
4. 리소스 목록 + kubectl 명령어는 기존 코드 유지 (WebSocket 완료 후 표시)

**Verify:** Deploy 실행 시 WebSocket 연결, 실시간 로그 스트리밍, 단계 진행바 동작, `npm run build` 통과

---

## 실행 순서 및 의존성

```
Task 1 (클러스터 필터) ─────────────────────────┐
Task 2 (Logs 페이지) ──────────────────────────┤
Task 3 (RUN + Breadcrumb) ─────────────────────┤─→ Group A 완료 → 커밋
                                                │
Task 4 (템플릿 제거) ──────────────────────────┐│
Task 5 (Git + Stack URL) ── depends on T4 ────┤│
Task 6 (실제 네임스페이스) ────────────────────┤├→ Group B 완료 → 커밋
Task 7 (리소스 Input) ────────────────────────┤│
Task 8 (매니페스트 편집) ── depends on T4 ────┘│
                                                │
Task 9 (WS 백엔드) ───────────────────────────┐│
Task 10 (WS 프론트 훅) ── depends on T9 ──────┤├→ Group C 완료 → 커밋
Task 11 (WS 배포 UI) ── depends on T10 ───────┘│
```

**권장 병렬화:**
- Group A (Task 1-3)는 독립적, 병렬 실행 가능
- Group B (Task 4-8)는 Task 4가 선행, 나머지는 독립적
- Group C (Task 9-11)는 순차적이나 Group A/B와 병렬 가능
